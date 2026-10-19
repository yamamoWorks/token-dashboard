import { test, expect, connectHub, drawing, provider, stats } from './support';

test('2プロバイダー単位で10秒固定巡回し設定操作を表示しない', async ({ page, app }) => {
  test.setTimeout(60_000);
  await test.step('開始条件', async () => {
    await connectHub(page, app.hub);
    app.hub.send('snapshot', stats([provider('alpha', [10]), provider('beta', [20]), provider('gamma', [30])]));
  });
  const names = async () => (await drawing(page)).providers;
  await test.step('手順1: 最初のページはリスク順の2件', async () => {
    await expect.poll(names).toEqual(['alpha', 'beta']);
    await expect(page.getByRole('checkbox')).toHaveCount(6);
  });
  await test.step('手順2: 10秒周期で第2ページと第1ページへ', async () => {
    await expect.poll(names, { timeout: 12_000, intervals: [100] }).toEqual(['gamma']);
    const changedAt = Date.now();
    // Start timing at a real page transition: the timer is owned by Mirror, not snapshot arrival.
    await page.waitForTimeout(8000);
    expect(await names()).toEqual(['gamma']);
    await expect.poll(names, { timeout: 4000, intervals: [100] }).toEqual(['alpha', 'beta']);
    expect(Date.now() - changedAt).toBeGreaterThanOrEqual(9000);
    expect(Date.now() - changedAt).toBeLessThan(11_500);
  });
  await test.step('受け入れ条件', async () => {
    await expect(page.locator('[class*="page-dot"], [aria-label*="page"]')).toHaveCount(0);
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await expect(page.getByRole('textbox', { name: /Cycle/ })).toHaveCount(0);
    await expect(page.getByRole('switch', { name: /Cycle/ })).toHaveCount(0);
  });
});
