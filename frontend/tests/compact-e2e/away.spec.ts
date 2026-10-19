import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { signal } from '../e2e/support/presence';
import { test, expect, connectHub, drawing, preview, recordFrames, stats } from './support';

test('Away timeの自動保存・再起動保持・復帰表示', async ({ page, app }) => {
  const frames = recordFrames(page);
  const away = page.getByRole('textbox', { name: 'Away time', exact: true });
  await test.step('開始条件', async () => {
    expect(process.platform, 'This suite requires the Windows presence watcher').toBe('win32');
    await connectHub(page, app.hub);
    app.hub.send('snapshot', stats());
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['1,234,567']);
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
  });
  await test.step('手順1: 1〜240分の値を自動保存', async () => {
    await expect(away).toHaveValue('10 min');
    for (const value of [1, 240, 3]) {
      await away.fill(String(value)); await away.press('Enter');
      await expect.poll(() => JSON.parse(readFileSync(join(app.dataDir, 'settings.json'), 'utf8')).awayMinutes).toBe(value);
    }
    await away.fill('241'); await away.blur();
    await expect(away).toHaveValue('3 min');
    await away.fill('0'); await away.press('Enter');
    await expect(away).toHaveValue('3 min');
    await expect(page.getByRole('button', { name: 'Save', exact: true })).toHaveCount(0);
  });
  await test.step('手順2: 再起動後も保存した値を表示', async () => {
    await app.restart();
    await expect.poll(() => app.hub.streams()).toBe(1);
    app.hub.send('snapshot', stats());
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['1,234,567']);
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await expect(away).toHaveValue('3 min');
  });
  await test.step('手順3: 離席中はプレビューを保ち描画しない', async () => {
    // Synthetic messages target only this test server's hidden window; never lock the user's session.
    signal(app.server.pid, 'suspend');
    await page.waitForTimeout(1500);
    await page.getByRole('link', { name: 'Display', exact: true }).click();
    const before = await preview(page);
    const count = frames.length;
    app.hub.send('stats', stats(undefined, 8888));
    await page.waitForTimeout(2200);
    expect(frames).toHaveLength(count);
    expect(await preview(page)).toBe(before);
  });
  await test.step('受け入れ条件: 復帰時は最新値を表示', async () => {
    signal(app.server.pid, 'resume');
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['8,888']);
    await expect.poll(async () => {
      const src = await preview(page);
      return frames.some(f => !f.error && `data:image/png;base64,${f.png}` === src);
    }).toBe(true);
  });
});
