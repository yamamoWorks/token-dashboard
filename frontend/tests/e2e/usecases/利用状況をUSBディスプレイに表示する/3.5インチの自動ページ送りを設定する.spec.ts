import { test, expect, type Page } from '@playwright/test';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { startServer } from '../../support/server';
import { startHub } from '../../support/hub';
import { shortIntervals } from '../../support/local';

const token = 'compact-paging-settings-token';

function stats(providerCount = 3) {
  const provider = (index: number) => ({
    provider: `provider-${index + 1}`,
    planLabel: 'Pro',
    windows: [{
      kind: 'weekly',
      label: 'Weekly',
      showMeter: true,
      remainingPercent: 10 + index * 10,
      usedPercent: 90 - index * 10,
    }],
  });
  return {
    periods: {
      today: { totalTokens: 1234567, costUsd: 1.23 },
      month: { totalTokens: 1, costUsd: 1 },
      allTime: { totalTokens: 1, costUsd: 1 },
    },
    limits: { providers: Array.from({ length: providerCount }, (_, index) => provider(index)) },
  };
}

async function preview(page: Page) {
  const image = page.getByRole('img', { name: 'Display preview' });
  await expect(image).toBeVisible();
  return (await image.getAttribute('src')) ?? '';
}

test('3.5インチの自動ページ送りを設定する', async ({ page, context }) => {
  test.setTimeout(180_000);
  const dataDir = mkdtempSync(join(tmpdir(), 'turzx-compact-paging-settings-e2e-'));
  const settingsFile = join(dataDir, 'settings.json');
  const hub = await startHub();
  let server = await startServer(dataDir, 34128, shortIntervals);
  const file = () => JSON.parse(readFileSync(settingsFile, 'utf8'));
  const autoPage = () => page.getByRole('checkbox', { name: 'Auto page' });
  const interval = () => page.getByRole('textbox', { name: 'Page interval' });

  try {
    await test.step('分岐条件', async () => {
      await page.goto(server.url);
      await page.getByRole('link', { name: 'Connection' }).click();
      await page.getByRole('textbox', { name: 'Data source' }).click();
      await page.getByRole('option', { name: 'Hub' }).click();
      await page.getByLabel('Hub URL').fill(hub.url);
      await page.getByLabel(/Access token/).fill(token);
      await page.getByRole('button', { name: 'Save' }).click();
      await expect(page.getByText('Saved.')).toBeVisible();
      await expect.poll(() => hub.streams()).toBe(1);
      hub.send('snapshot', stats());

      await page.getByRole('link', { name: 'Display' }).click();
      await page.getByRole('textbox', { name: 'Display profile' }).click();
      await page.getByRole('option', { name: 'TURZX 3.5 Inch' }).click();
      await expect(page.getByRole('textbox', { name: 'Display profile' })).toHaveValue('TURZX 3.5 Inch');
      await expect.poll(async () => (await preview(page)).length).toBeGreaterThan(1000);
    });

    const requests = hub.requests.length;
    await test.step('手順1', async () => {
      await expect(autoPage()).toBeVisible();
      await expect(autoPage()).toBeChecked();
      await expect(interval()).toHaveValue('10 sec');
      await expect(interval()).toBeEnabled();

      const image = await page.getByRole('img', { name: 'Display preview' }).boundingBox();
      const switchBox = await autoPage().boundingBox();
      const intervalBox = await interval().boundingBox();
      expect(switchBox!.x).toBeGreaterThan(image!.x + image!.width);
      expect(intervalBox!.x).toBeGreaterThan(image!.x + image!.width);

      // Legacy files have no paging keys; the UI still presents the backward-compatible defaults.
      expect(file().compactAutoPage).toBeUndefined();
      expect(file().compactPageIntervalSeconds).toBeUndefined();
    });

    await test.step('手順2', async () => {
      // Keep the preview on page 2 to prove the physical paging setting does not reset preview state.
      const secondPage = page.getByRole('button', { name: 'Preview page 2' });
      await secondPage.click();
      await expect(secondPage).toHaveAttribute('aria-current', 'page');

      await autoPage().click();
      await expect.poll(() => file().compactAutoPage).toBe(false);
      expect(file().compactPageIntervalSeconds).toBe(10);
      await expect(autoPage()).not.toBeChecked();
      await expect(interval()).toBeDisabled();
      await expect(secondPage).toHaveAttribute('aria-current', 'page');
      expect(hub.requests.length).toBe(requests);
      expect(hub.streams()).toBe(1);
    });

    await test.step('手順3', async () => {
      await autoPage().click();
      await expect.poll(() => file().compactAutoPage).toBe(true);
      await expect(autoPage()).toBeChecked();
      await expect(interval()).toBeEnabled();
      await expect(interval()).toHaveValue('10 sec');
      expect(file().compactPageIntervalSeconds).toBe(10);
    });

    await test.step('手順4', async () => {
      await interval().click();
      await page.getByRole('option', { name: '30 sec' }).click();
      await expect.poll(() => file().compactPageIntervalSeconds).toBe(30);
      expect(file().compactAutoPage).toBe(true);
      await expect(interval()).toHaveValue('30 sec');
      expect(hub.requests.length).toBe(requests);
      expect(hub.streams()).toBe(1);
    });

    await test.step('手順5', async () => {
      const firstPage = page.getByRole('button', { name: 'Preview page 1' });
      const secondPage = page.getByRole('button', { name: 'Preview page 2' });
      await expect(secondPage).toHaveAttribute('aria-current', 'page');
      const secondImage = await preview(page);

      await firstPage.click();
      await expect(firstPage).toHaveAttribute('aria-current', 'page');
      await expect(secondPage).not.toHaveAttribute('aria-current', 'page');
      expect(await preview(page)).not.toBe(secondImage);

      expect(file().compactAutoPage).toBe(true);
      expect(file().compactPageIntervalSeconds).toBe(30);
    });

    await test.step('受け入れ条件', async () => {
      // Automatic paging does not move the window preview even while redraw events continue.
      const firstPage = page.getByRole('button', { name: 'Preview page 1' });
      await expect(firstPage).toHaveAttribute('aria-current', 'page');
      await page.waitForTimeout(1500);
      await expect(firstPage).toHaveAttribute('aria-current', 'page');

      // All and only the agreed intervals are offered.
      await interval().click();
      await expect(page.getByRole('option')).toHaveText(['5 sec', '10 sec', '15 sec', '30 sec', '60 sec']);
      await page.keyboard.press('Escape');

      // The controls remain available even when there is only one compact preview page.
      hub.send('stats', stats(2));
      await expect(page.getByRole('button', { name: 'Preview page 2' })).toHaveCount(0);
      await expect(autoPage()).toBeVisible();
      await expect(interval()).toHaveValue('30 sec');

      // They are hidden for the 9.2-inch profile.
      await page.getByRole('textbox', { name: 'Display profile' }).click();
      await page.getByRole('option', { name: 'TURZX 9.2 Inch' }).click();
      await expect(autoPage()).toHaveCount(0);
      await expect(interval()).toHaveCount(0);

      // Returning to compact restores the saved values.
      await page.getByRole('textbox', { name: 'Display profile' }).click();
      await page.getByRole('option', { name: 'TURZX 3.5 Inch' }).click();
      await expect(autoPage()).toBeChecked();
      await expect(interval()).toHaveValue('30 sec');

      // The saved values survive restart.
      await page.close();
      await server.stop();
      await expect.poll(() => hub.streams()).toBe(0);
      server = await startServer(dataDir, 34128, shortIntervals);
      await expect.poll(() => hub.streams()).toBe(1);
      hub.send('snapshot', stats(3));
      page = await context.newPage();
      await page.goto(server.url);
      await expect(autoPage()).toBeChecked();
      await expect(interval()).toHaveValue('30 sec');
      expect(file().compactAutoPage).toBe(true);
      expect(file().compactPageIntervalSeconds).toBe(30);
      await expect(page.getByText(token)).toHaveCount(0);
    });
  } finally {
    await server.stop();
    await hub.close();
    rmSync(dataDir, { recursive: true, force: true });
  }
});
