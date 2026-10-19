import { test as base, expect, type Page } from '@playwright/test';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createServer, get, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { startServer } from '../e2e/support/server';
import { startHub } from '../e2e/support/hub';
import { localHome } from '../e2e/support/local';

export const token = 'compact-test-token';

// Reuse upstream Hub fixtures through a transport proxy that can break SSE connections.
async function reconnectableHub() {
  const hub = await startHub();
  const streams = new Set<ServerResponse>();
  const proxy = createServer((req, res) => {
    const upstream = get(hub.url + req.url, { headers: req.headers }, incoming => {
      res.writeHead(incoming.statusCode ?? 502, incoming.headers);
      streams.add(res);
      incoming.pipe(res);
    });
    upstream.on('error', () => res.destroy());
    res.on('close', () => { streams.delete(res); upstream.destroy(); });
  });
  await new Promise<void>(done => proxy.listen(0, '127.0.0.1', done));
  return { ...hub,
    url: `http://127.0.0.1:${(proxy.address() as AddressInfo).port}`,
    disconnect: () => { for (const stream of streams) stream.destroy(); },
    close: async () => {
      for (const stream of streams) stream.destroy();
      await new Promise<void>(done => proxy.close(() => done()));
      await hub.close();
    },
  };
}
export const provider = (name: string, remaining: (number | null)[] = [25]) => ({
  provider: name, planLabel: 'Pro', windows: remaining.map((value, i) => ({
    kind: i === 0 ? 'session' : 'weekly', label: i === 0 ? 'Session' : 'Weekly',
    showMeter: true, remainingPercent: value, usedPercent: value === null ? null : 100 - value,
    windowMinutes: i === 0 ? 300 : 10080,
    resetsAt: new Date(Date.now() + 2 * 3_600_000 + 30 * 60_000).toISOString(),
  })),
});
export const stats = (providers = [provider('alpha', [25, 70])], tokens = 1234567) => ({
  periods: {
    today: { totalTokens: tokens, costUsd: 12.34 },
    month: { totalTokens: 23456789, costUsd: 56.78 },
    allTime: { totalTokens: 345678901, costUsd: 90.12 },
  },
  limits: { providers },
});

export type Completion = { id: number; png: string; jpeg: string; error: string };
export function recordFrames(page: Page) {
  const frames: Completion[] = [];
  page.on('request', request => {
    if (!request.url().endsWith('/wails/runtime') || request.method() !== 'POST') return;
    const payload = request.postDataJSON();
    if (payload?.args?.methodName !== 'token-monitor-turzx/internal/display.Service.CompleteFrame') return;
    const [id, png, jpeg, error] = payload.args.args as [number, string, string, string];
    frames.push({ id, png, jpeg, error });
  });
  return frames;
}

// Observe the actual serialized SVG used by Canvas; never replace the renderer or its result.
export async function observeDrawing(page: Page) {
  await page.addInitScript(() => {
    const serialize = XMLSerializer.prototype.serializeToString;
    XMLSerializer.prototype.serializeToString = function(node) {
      const result = serialize.call(this, node);
      if (result.includes('3.5-inch compact display')) {
        (window as typeof window & { compactSVG?: string }).compactSVG = result;
      }
      return result;
    };
  });
}
export async function drawing(page: Page) {
  return page.evaluate(() => {
    const svg = (window as typeof window & { compactSVG?: string }).compactSVG ?? '';
    const doc = new DOMParser().parseFromString(svg, 'image/svg+xml');
    const texts = (selector: string) => [...doc.querySelectorAll(selector)].map(e => e.textContent?.trim());
    return {
      providers: texts('.provider-name'), percentages: texts('.meter-value strong'),
      labels: texts('.original-label'), resets: texts('.reset-time span'),
      tokens: texts('.token-count'), cost: texts('.token-cost'), waiting: texts('.waiting'),
      rings: [...doc.querySelectorAll('.window-ring')].map(e => ({
        label: e.getAttribute('aria-label'), fills: e.querySelectorAll('.ring-fill').length,
      })),
    };
  });
}
export async function preview(page: Page) {
  const image = page.getByRole('img', { name: 'Display preview' });
  await expect(image).toBeVisible();
  return await image.getAttribute('src') ?? '';
}
export async function openApp(page: Page, url: string) {
  await page.goto(url);
  await expect(page.getByTestId('compact-preview')).toBeVisible();
  await expect.poll(() => drawing(page)).toMatchObject({ waiting: expect.any(Array) });
}
export async function connectHub(page: Page, hub: Awaited<ReturnType<typeof startHub>>) {
  await page.getByRole('link', { name: 'Connection', exact: true }).click();
  await page.getByRole('textbox', { name: 'Data source' }).click();
  await page.getByRole('option', { name: 'Hub', exact: true }).click();
  await page.getByLabel('Hub URL').fill(hub.url);
  await page.getByLabel(/Access token/).fill(token);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Saved.')).toBeVisible();
  await expect.poll(() => hub.streams()).toBe(1);
  await page.getByRole('link', { name: 'Display', exact: true }).click();
}

type App = {
  dataDir: string; home: string; hub: Awaited<ReturnType<typeof reconnectableHub>>;
  server: Awaited<ReturnType<typeof startServer>>;
  restart: () => Promise<void>;
};
export const test = base.extend<{ app: App }>({
  app: async ({ page }, runTest) => {
    const dataDir = mkdtempSync(join(tmpdir(), 'compact-e2e-'));
    const { home, env } = localHome();
    const hub = await reconnectableHub();
    let server: Awaited<ReturnType<typeof startServer>> | undefined;
    try {
      await observeDrawing(page);
      server = await startServer(dataDir, 34215, env);
      const app: App = { dataDir, home, hub, server, restart: async () => {
        await page.goto('about:blank');
        await app.server.stop();
        await expect.poll(() => hub.streams()).toBe(0);
        server = await startServer(dataDir, 34215, env);
        app.server = server;
        await openApp(page, server.url);
      } };
      await openApp(page, server.url);
      await runTest(app);
    } finally {
      await page.goto('about:blank').catch(() => undefined);
      await server?.stop();
      await hub.close();
      // These are the exact absolute directories returned by mkdtempSync.
      rmSync(dataDir, { recursive: true, force: true });
      rmSync(home, { recursive: true, force: true });
    }
  },
});
export { expect };
