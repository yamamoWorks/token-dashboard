import { test, expect, chromium } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Opt in against the running desktop app, real Hub and selected USB device.
// LOCK_E2E_PID, LOCK_E2E_PORT, LOCK_E2E_LOG, LOCK_E2E_CDP are required.
test('Windows session lock resets USB and unlock resumes @lock-desktop', async () => {
  test.skip(process.platform !== 'win32' || !process.env.LOCK_E2E_PID, 'Requires the real desktop and USB device');
  test.setTimeout(90_000);
  const pid = process.env.LOCK_E2E_PID!;
  const port = process.env.LOCK_E2E_PORT!;
  const log = process.env.LOCK_E2E_LOG!;
  expect(port).toMatch(/^COM\d+$/);
  expect(log).toBeTruthy();
  const root = resolve('..');
  const driver = resolve(root, '.test-data/session-notify.exe');
  mkdirSync(resolve(root, '.test-data'), { recursive: true });
  execFileSync('go', ['build', '-o', driver, './frontend/tests/support/session-notify'], { cwd: root });
  const notify = (event: string) => execFileSync(driver, [pid, port, event], { encoding: 'utf8' }).trim();
  const browser = await chromium.connectOverCDP(process.env.LOCK_E2E_CDP ?? 'http://127.0.0.1:9347');
  const page = browser.contexts()[0].pages().find(p => p.url().includes('wails.localhost'))!;
  expect(page).toBeTruthy();
  const start = readFileSync(log, 'utf8').length;
  const transitions = () => readFileSync(log, 'utf8').slice(start).match(/msg=display_transfer_state/g)?.length ?? 0;
  const choice = page.getByRole('checkbox', { name: / session$/ }).first();
  let original: string | null = null;
  try {
    await test.step('分岐条件', async () => {
      await page.getByRole('link', { name: 'Connection', exact: true }).click();
      await expect(page.getByRole('textbox', { name: 'Data source', exact: true })).toHaveValue('Hub');
      await page.getByRole('link', { name: 'Display', exact: true }).click();
      await expect(page.getByAltText('Display preview')).toBeVisible();
      await expect(choice).toBeVisible();
      expect(notify('probe')).toBe('busy');
    });
    await test.step('手順1', async () => {
      notify('lock');
      notify('lock');
      await expect.poll(() => notify('probe'), { timeout: 15_000 }).toBe('free');
      expect(transitions()).toBe(1);
    });
    await test.step('手順2', async () => {
      original = await choice.getAttribute('aria-checked');
      const before = await page.getByAltText('Display preview').getAttribute('src');
      await choice.click();
      await expect(choice).not.toHaveAttribute('aria-checked', original!);
      await expect(page.getByAltText('Display preview')).not.toHaveAttribute('src', before!);
      await page.waitForTimeout(1500); // Beyond Output's reconnect ticker.
      expect(notify('probe')).toBe('free');
      expect(transitions()).toBe(1);
    });
    await test.step('手順3', async () => {
      notify('unlock');
      notify('unlock');
      await expect.poll(() => notify('probe'), { timeout: 15_000 }).toBe('busy');
      expect(transitions()).toBe(2);
    });
    await test.step('受け入れ条件', async () => {
      expect(readFileSync(log, 'utf8').slice(start)).not.toMatch(/turzx_(exit|close|send)_failed/);
      await expect(page.getByAltText('Display preview')).toBeVisible();
    });
  } finally {
    // Restore the running app even if an assertion fails.
    try {
      if (original !== null && await choice.getAttribute('aria-checked') !== original) {
        await choice.click();
        await expect(choice).toHaveAttribute('aria-checked', original);
      }
    } finally {
      notify('unlock');
      await browser.close();
    }
  }
});
