import { test, expect, type Page } from '@playwright/test';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { startServer } from '../../support/server';
import { startHub } from '../../support/hub';
import { shortIntervals } from '../../support/local';

const token = 'compact-preview-token';

function compactStats(offset = 0) {
  const provider = (name: string, remaining: number) => ({
    provider: name,
    planLabel: 'Pro',
    windows: [{ kind: 'weekly', label: 'Weekly', showMeter: true, remainingPercent: remaining, usedPercent: 100 - remaining }],
  });
  return {
    periods: {
      today: { totalTokens: 1234567 + offset, costUsd: 1.23 },
      month: { totalTokens: 1, costUsd: 1 },
      allTime: { totalTokens: 1, costUsd: 1 },
    },
    limits: {
      providers: [
        provider('first', 10 + offset),
        provider('second', 20 + offset),
        provider('third', 30 + offset),
      ],
    },
  };
}

async function preview(page: Page) {
  const image = page.getByRole('img', { name: 'Display preview' });
  await expect(image).toBeVisible();
  return (await image.getAttribute('src')) ?? '';
}

test('3.5インチのプレビューはページドットをクリックしたときだけ切り替わる', async ({ page }) => {
  test.setTimeout(120_000);
  const dataDir = mkdtempSync(join(tmpdir(), 'turzx-compact-preview-e2e-'));
  const hub = await startHub();
  const server = await startServer(dataDir, 34127, shortIntervals);
  try {
    await page.goto(server.url);
    await page.getByRole('link', { name: 'Connection' }).click();
    await page.getByRole('textbox', { name: 'Data source' }).click();
    await page.getByRole('option', { name: 'Hub' }).click();
    await page.getByLabel('Hub URL').fill(hub.url);
    await page.getByLabel(/Access token/).fill(token);
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page.getByText('Saved.')).toBeVisible();

    await page.getByRole('link', { name: 'Display' }).click();
    await page.getByRole('textbox', { name: 'Display profile' }).click();
    await page.getByRole('option', { name: 'TURZX 3.5 Inch' }).click();
    await expect(page.getByRole('textbox', { name: 'Display profile' })).toHaveValue('TURZX 3.5 Inch');

    await expect.poll(() => hub.streams()).toBe(1);
    hub.send('snapshot', compactStats());

    const firstDot = page.getByRole('button', { name: 'Preview page 1' });
    const secondDot = page.getByRole('button', { name: 'Preview page 2' });
    await expect(firstDot).toHaveAttribute('aria-current', 'page');
    await expect(secondDot).not.toHaveAttribute('aria-current', 'page');
    const firstPage = await preview(page);

    // The server build redraws every second. Timer redraws may refresh the image data, but they
    // must not change the manually selected preview page.
    await page.waitForTimeout(1500);
    await expect(firstDot).toHaveAttribute('aria-current', 'page');
    await expect(secondDot).not.toHaveAttribute('aria-current', 'page');

    await secondDot.click();
    await expect(secondDot).toHaveAttribute('aria-current', 'page');
    const secondPage = await preview(page);
    expect(secondPage).not.toBe(firstPage);

    // A real usage update also keeps the selected page while replacing that page's latest content.
    hub.send('stats', compactStats(5));
    await expect.poll(() => preview(page)).not.toBe(secondPage);
    await expect(secondDot).toHaveAttribute('aria-current', 'page');
    await expect(firstDot).not.toHaveAttribute('aria-current', 'page');

    await firstDot.click();
    await expect(firstDot).toHaveAttribute('aria-current', 'page');
    await expect(secondDot).not.toHaveAttribute('aria-current', 'page');
  } finally {
    await server.stop();
    await hub.close();
    rmSync(dataDir, { recursive: true, force: true });
  }
});
