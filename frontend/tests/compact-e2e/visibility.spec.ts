import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, connectHub, drawing, provider, stats } from './support';

test('全データから枠を非表示にして再起動後も保持', async ({ page, app }) => {
  const snapshot = stats([provider('alpha', [10, 20]), provider('beta', [30]), provider('gamma', [40])]);
  await test.step('開始条件', async () => {
    await connectHub(page, app.hub);
    app.hub.send('snapshot', snapshot);
    await expect.poll(async () => (await drawing(page)).providers).toEqual(['alpha', 'beta']);
  });
  await test.step('手順1: 描画外のプロバイダーも一覧に残る', async () => {
    await expect(page.getByRole('checkbox')).toHaveCount(7);
    await expect(page.getByRole('checkbox', { name: 'gamma Pro', exact: true })).toBeVisible();
  });
  await test.step('手順2: alphaの枠をすべて非表示', async () => {
    await page.getByRole('checkbox', { name: 'alpha Pro', exact: true }).click();
    await expect(page.getByRole('checkbox', { name: 'alpha Pro', exact: true })).not.toBeChecked();
    await expect.poll(async () => (await drawing(page)).providers).toEqual(['beta', 'gamma']);
    await expect.poll(() => JSON.parse(readFileSync(join(app.dataDir, 'settings.json'), 'utf8')).hiddenLimits.length).toBe(2);
    await expect(page.getByRole('checkbox')).toHaveCount(7);
  });
  await test.step('手順3: 再起動後に全snapshotを再受信', async () => {
    await app.restart();
    await expect.poll(() => app.hub.streams()).toBe(1);
    app.hub.send('snapshot', snapshot);
    await expect.poll(async () => (await drawing(page)).providers).toEqual(['beta', 'gamma']);
  });
  await test.step('受け入れ条件', async () => {
    await expect(page.getByRole('checkbox', { name: 'alpha Pro', exact: true })).not.toBeChecked();
    await expect(page.getByRole('checkbox')).toHaveCount(7);
    await page.getByRole('checkbox', { name: 'alpha Pro', exact: true }).click();
    await expect.poll(async () => (await drawing(page)).providers).toEqual(['alpha', 'beta']);
  });
});
