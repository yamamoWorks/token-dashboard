import { test, expect } from '@playwright/test';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { startServer } from '../../support/server';

test('Rev.A の Settings は離席時間だけを編集し、既存 Style/Cycle 設定を保持する', async ({ page, context }) => {
  test.setTimeout(120_000);
  const dataDir = mkdtempSync(join(tmpdir(), 'turzx-settings-e2e-'));
  const settingsFile = join(dataDir, 'settings.json');
  // A legacy settings file must not lose inactive keys when Away time is saved.
  writeFileSync(settingsFile, JSON.stringify({
    source: 'Local', limitStyle: 'Bars', cycle: true, cycleSeconds: 30, awayMinutes: 10,
  }));
  const saved = () => JSON.parse(readFileSync(settingsFile, 'utf8'));
  let server = await startServer(dataDir, 34152);
  const away = () => page.getByRole('textbox', { name: 'Away time' });
  try {
    await test.step('開始条件', async () => {
      await page.goto(server.url);
      await page.getByRole('link', { name: 'Settings' }).click();
    });
    await test.step('手順1', async () => {
      await expect(away()).toHaveValue('10 min');
      await expect(page.getByRole('switch', { name: 'Cycle' })).toHaveCount(0);
      await expect(page.getByRole('textbox', { name: 'Cycle time' })).toHaveCount(0);
      await expect(page.getByRole('button', { name: 'Save' })).toHaveCount(0);
    });
    await test.step('手順2', async () => {
      await away().fill('2');
      await away().press('Enter');
      await expect.poll(() => saved().awayMinutes).toBe(2);
      await expect(away()).toHaveValue('2 min');
      expect(saved()).toMatchObject({ source: 'Local', limitStyle: 'Bars', cycle: true, cycleSeconds: 30 });
    });
    await test.step('手順3', async () => {
      await away().fill('0');
      await away().press('Enter');
      await expect(away()).toHaveValue('2 min');
      expect(saved().awayMinutes).toBe(2);
    });
    await test.step('手順4', async () => {
      await page.getByRole('link', { name: 'Display' }).click();
      await expect(page.getByRole('heading', { name: 'Preview' })).toBeVisible();
      await expect(page.getByRole('textbox', { name: 'Display style' })).toHaveCount(0);
      await expect(page.getByRole('switch', { name: 'Cycle' })).toHaveCount(0);
      await expect(page.getByRole('textbox', { name: 'Cycle time' })).toHaveCount(0);
    });
    await test.step('手順5', async () => {
      await page.close();
      await server.stop();
      server = await startServer(dataDir, 34152);
      page = await context.newPage();
      await page.goto(server.url);
      await page.getByRole('link', { name: 'Settings' }).click();
      await expect(away()).toHaveValue('2 min');
    });
    await test.step('受け入れ条件', async () => {
      expect(saved()).toMatchObject({ limitStyle: 'Bars', cycle: true, cycleSeconds: 30, awayMinutes: 2 });
      await expect(page.getByRole('switch', { name: 'Cycle' })).toHaveCount(0);
      await expect(page.getByRole('textbox', { name: 'Cycle time' })).toHaveCount(0);
    });
  } finally {
    await page.close().catch(() => undefined);
    await server.stop();
    rmSync(dataDir, { recursive: true, force: true });
  }
});
