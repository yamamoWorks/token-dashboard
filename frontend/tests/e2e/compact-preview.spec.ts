import { test, expect, type Page } from '@playwright/test';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { startServer } from './support/server';
import { startHub } from './support/hub';
import { shortIntervals } from './support/local';

function stats() {
  const provider = (name: string, remaining: number) => ({
    provider: name, planLabel: 'Pro',
    windows: [{ kind: 'weekly', label: 'Weekly', showMeter: true,
      remainingPercent: remaining, usedPercent: 100 - remaining }],
  });
  return {
    periods: {
      today: { totalTokens: 1234567, costUsd: 1.23 },
      month: { totalTokens: 1, costUsd: 1 },
      allTime: { totalTokens: 1, costUsd: 1 },
    },
    limits: { providers: [provider('first', 10), provider('second', 20), provider('third', 30)] },
  };
}

async function pixel(page: Page, x: number, y: number) {
  return page.evaluate(async ({ x, y }) => {
    const preview = document.querySelector<HTMLImageElement>('img[alt="Display preview"]')!;
    const img = new Image();
    img.src = preview.src;
    await img.decode();
    const canvas = document.createElement('canvas');
    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;
    const context = canvas.getContext('2d')!;
    context.drawImage(img, 0, 0);
    return [...context.getImageData(x, y, 1, 1).data].slice(0, 3).join(',');
  }, { x, y });
}

test('Rev.A プレビューは左上480×320を縮小表示し10秒ページ巡回に追従する', async ({ page }) => {
  test.setTimeout(100_000);
  const dataDir = mkdtempSync(join(tmpdir(), 'turzx-compact-preview-e2e-'));
  const hub = await startHub();
  const server = await startServer(dataDir, 34162, shortIntervals);
  try {
    await page.goto(server.url);
    await page.getByRole('link', { name: 'Connection' }).click();
    await page.getByRole('textbox', { name: 'Data source' }).click();
    await page.getByRole('option', { name: 'Hub' }).click();
    await page.getByLabel('Hub URL').fill(hub.url);
    await page.getByLabel(/Access token/).fill('compact-preview-token');
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page.getByText('Saved.')).toBeVisible();
    await expect.poll(() => hub.streams()).toBe(1);
    hub.send('snapshot', stats());
    await page.getByRole('link', { name: 'Display' }).click();

    const clip = page.getByTestId('compact-preview');
    const img = page.getByRole('img', { name: 'Display preview' });
    await expect(clip).toBeVisible();
    await expect(img).toBeVisible();
    const dimensions = await img.evaluate(image => {
      const node = image as HTMLImageElement;
      return { width: node.naturalWidth, height: node.naturalHeight, png: node.src.startsWith('data:image/png;base64,') };
    });
    expect(dimensions).toEqual({ width: 1920, height: 462, png: true });

    const geometry = async () => {
      const crop = (await clip.boundingBox())!;
      const image = (await img.boundingBox())!;
      return { crop, image };
    };
    const checkGeometry = async (narrow: boolean) => {
      const { crop, image } = await geometry();
      expect(crop.width).toBeGreaterThan(0);
      expect(crop.width).toBeLessThanOrEqual(480);
      if (narrow) expect(crop.width).toBeLessThan(480);
      expect(Math.abs(crop.height - crop.width * 320 / 480)).toBeLessThan(1);
      expect(Math.abs(image.width - crop.width * 4)).toBeLessThan(1);
      expect(Math.abs(image.height - crop.width * 462 / 480)).toBeLessThan(1);
      expect(Math.abs(image.x - crop.x)).toBeLessThan(1);
      expect(Math.abs(image.y - crop.y)).toBeLessThan(1);
      expect(await clip.evaluate(node => getComputedStyle(node).overflow)).toBe('hidden');
    };
    await checkGeometry(false);
    await page.setViewportSize({ width: 400, height: 900 });
    await checkGeometry(true);

    await expect(page.getByRole('button', { name: /Preview page/ })).toHaveCount(0);
    await expect(page.getByRole('textbox', { name: 'Display style' })).toHaveCount(0);
    // Pixel in the first card must become the panel colour after the Hub snapshot arrives.
    await expect.poll(() => pixel(page, 20, 110), { timeout: 20_000 }).toBe('21,24,33');

    // On a 3-provider snapshot, the right card exists on one page and is absent on the next.
    // Compare a panel-background pixel, unaffected by the test server's burn-in position shift.
    const right = () => pixel(page, 260, 110);
    const first = await right();
    expect(['21,24,33', '15,17,23']).toContain(first);
    const next = first === '21,24,33' ? '15,17,23' : '21,24,33';
    await expect.poll(right, { timeout: 22_000, intervals: [300, 600] }).toBe(next);
    await expect.poll(right, { timeout: 22_000, intervals: [300, 600] }).toBe(first);
  } finally {
    await server.stop();
    await hub.close();
    rmSync(dataDir, { recursive: true, force: true });
  }
});
