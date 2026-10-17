import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import Handlebars from 'handlebars';
import { expect, test } from '@playwright/test';

const root = join(import.meta.dirname, '../../../../themes/compact');
const template = Handlebars.compile(readFileSync(join(root, 'template.hbs'), 'utf8'), { strict: true });
const css = readFileSync(join(root, 'style.css'), 'utf8');

function window(id: string, percent: number | null, label: string, duration: string) {
  return {
    id, label, durationLabel: duration, percentText: percent === null ? '—' : `${percent}%`,
    remainingPercent: percent, hasRemaining: percent !== null, tone: 'normal', resetText: '1h 30m',
  };
}
function panel(provider: string, circles: { windows: ReturnType<typeof window>[] }[]) {
  return { provider, plan: 'Pro', icon: '', span: circles.length, circles };
}
function data(panels: ReturnType<typeof panel>[], available = true, waitingMessage = 'Waiting for Hub') {
  return {
    available, waitingMessage,
    tokens: [{ label: 'Today', tokensText: '12,345,678', costText: '$48.32', tokensFontSize: 42 }],
    gaugePanels: panels, barColumns: [],
  };
}
async function load(page: import('@playwright/test').Page, value: ReturnType<typeof data>) {
  await page.setViewportSize({ width: 1920, height: 462 });
  await page.setContent(`<!doctype html><html><head><meta charset="utf-8"><style>${css}</style></head><body>${template(value)}</body></html>`);
}

// Execute the same HTML -> SVG foreignObject -> Canvas steps used by theme-renderer.ts.
async function renderCanvas(page: import('@playwright/test').Page) {
  return page.evaluate(async () => {
    const namespace = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(namespace, 'svg');
    svg.setAttribute('width', '1920');
    svg.setAttribute('height', '462');
    svg.setAttribute('viewBox', '0 0 1920 462');
    const style = document.createElementNS(namespace, 'style');
    style.textContent = document.querySelector('style')!.textContent;
    const foreignObject = document.createElementNS(namespace, 'foreignObject');
    foreignObject.setAttribute('width', '1920');
    foreignObject.setAttribute('height', '462');
    foreignObject.append(document.importNode(document.body, true));
    svg.append(style, foreignObject);
    const image = new Image();
    image.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(new XMLSerializer().serializeToString(svg));
    await image.decode();
    const canvas = document.createElement('canvas');
    canvas.width = 1920;
    canvas.height = 462;
    const context = canvas.getContext('2d', { alpha: false })!;
    context.fillStyle = '#000';
    context.fillRect(0, 0, 1920, 462);
    context.drawImage(image, 0, 0);
    const pixel = (x: number, y: number) => [...context.getImageData(x, y, 1, 1).data].slice(0, 3);
    return {
      width: canvas.width, height: canvas.height,
      outside: [pixel(600, 100), pixel(1850, 450), pixel(300, 400)],
      background: pixel(479, 319),
      panel: pixel(20, 80),
      png: canvas.toDataURL('image/png').startsWith('data:image/png;base64,'),
    };
  });
}

for (const grouping of ['one group', 'two groups'] as const) {
  test(`compact draws two concentric rings for ${grouping}`, async ({ page }) => {
    const a = window('session', 35, 'Session', '5h');
    const b = window('weekly', 88, 'Weekly', '7d');
    const circles = grouping === 'one group' ? [{ windows: [a, b] }] : [{ windows: [a] }, { windows: [b] }];
    await load(page, data([panel('claude', circles), panel('codex', [{ windows: [window('daily', 49, 'Daily', '1d')] }])]));
    await expect(page.locator('.provider-card')).toHaveCount(2);
    await expect(page.locator('.provider-card').first().locator('.window-ring')).toHaveCount(2);
    const rings = await page.locator('.provider-card').first().locator('.window-ring').evaluateAll(groups =>
      groups.map(group => ({
        outer: getComputedStyle(group.querySelector('.ring-outer.ring-fill')!).display,
        inner: getComputedStyle(group.querySelector('.ring-inner.ring-fill')!).display,
      })),
    );
    expect(rings[0].outer).not.toBe('none');
    expect(rings[0].inner).toBe('none');
    expect(rings[1].outer).toBe('none');
    expect(rings[1].inner).not.toBe('none');
    const viewport = page.locator('.compact-screen');
    expect(await viewport.boundingBox()).toMatchObject({ x: 0, y: 0, width: 480, height: 320 });
    for (const item of await page.locator('.provider-card, .window-detail, .token-strip').all()) {
      const rect = await item.boundingBox();
      expect(rect, 'visible content must fit inside 480x320').not.toBeNull();
      expect(rect!.x).toBeGreaterThanOrEqual(0);
      expect(rect!.y).toBeGreaterThanOrEqual(0);
      expect(rect!.x + rect!.width).toBeLessThanOrEqual(480);
      expect(rect!.y + rect!.height).toBeLessThanOrEqual(320);
    }
    const pixels = await renderCanvas(page);
    expect(pixels).toMatchObject({ width: 1920, height: 462, png: true });
    expect(pixels.outside).toEqual([pixels.background, pixels.background, pixels.background]);
    expect(pixels.panel).not.toEqual(pixels.background);
  });
}

test('compact keeps waiting state within the 3.5-inch area', async ({ page }) => {
  await load(page, data([], false, 'Waiting for local usage'));
  await expect(page.locator('.waiting')).toHaveText('Waiting for local usage');
  await expect(page.locator('.provider-card')).toHaveCount(0);
  const rect = await page.locator('.waiting').boundingBox();
  expect(rect).toMatchObject({ x: 0, y: 0, width: 480, height: 320 });
  const pixels = await renderCanvas(page);
  expect(pixels.outside).toEqual([pixels.background, pixels.background, pixels.background]);
});
