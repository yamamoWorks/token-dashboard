import { test, expect, connectHub, drawing, stats } from './support';

test('Compact固定スタイルと不要なGauges/Bars操作の非表示', async ({ page, app }) => {
  await test.step('開始条件', async () => {
    await connectHub(page, app.hub);
    app.hub.send('snapshot', stats());
    await expect.poll(async () => (await drawing(page)).providers).toEqual(['alpha']);
  });
  await test.step('手順1: Displayにスタイル切替なし', async () => {
    await expect(page.getByRole('textbox', { name: /Display style|Usage limit style/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Gauges|Bars/ })).toHaveCount(0);
  });
  await test.step('手順2: Style一覧でCompactだけがIn use', async () => {
    await page.getByRole('link', { name: 'Styles', exact: true }).click();
    const compact = page.getByRole('article', { name: 'Compact 3.5-inch', exact: true });
    await expect(compact.getByText('Built-in', { exact: true })).toBeVisible();
    await expect(compact.getByText('In use', { exact: true })).toBeVisible();
    await expect(compact.getByRole('button', { name: /Delete/ })).toHaveCount(0);
    for (const name of ['Gauges', 'Bars']) {
      await expect(page.getByRole('article', { name, exact: true }).getByText('In use')).toHaveCount(0);
    }
  });
  await test.step('受け入れ条件', async () => {
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await expect(page.getByRole('textbox', { name: /Style|Cycle/ })).toHaveCount(0);
    await expect(page.getByRole('switch')).toHaveCount(0);
    // Inspect the actual registered Wails service, rather than only source constants.
    const infoResponse = page.waitForResponse(response => {
      if (!response.url().endsWith('/wails/runtime') || response.request().method() !== 'POST') return false;
      return response.request().postDataJSON()?.args?.methodName === 'token-monitor-turzx/internal/desktop.Service.GetInfo';
    });
    await page.reload();
    const info = await (await infoResponse).json();
    expect(info.updateConfigured).toBe(false);
    await expect(page.getByRole('button', { name: /Check for updates|Update now|Install update/ })).toHaveCount(0);
  });
});
