import { appendFileSync, mkdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, connectHub, drawing, stats } from './support';

test('Local設定・更新・再起動保持とHub切替', async ({ page, app }) => {
  const logDir = join(app.home, '.claude/projects/compact');
  const log = join(logDir, 'session.jsonl');
  mkdirSync(logDir, { recursive: true });
  let serial = 0;
  const add = (input: number) => appendFileSync(log, JSON.stringify({
    type: 'assistant', timestamp: new Date().toISOString(), requestId: `request-${++serial}`, sessionId: 'compact',
    message: { id: `message-${serial}`, model: 'claude-sonnet-4-5', usage: {
      input_tokens: input, output_tokens: 0, cache_read_input_tokens: 0, cache_creation_input_tokens: 0,
    } },
  }) + '\n');
  await test.step('開始条件', async () => {
    add(1234);
    // Restart makes discovery include the newly created fixture log directory.
    await app.restart();
    await page.getByRole('link', { name: 'Connection', exact: true }).click();
    await expect(page.getByRole('textbox', { name: 'Data source' })).toHaveValue('Local');
  });
  await test.step('手順1: Localを保存', async () => {
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByText('Saved.')).toBeVisible();
    expect(JSON.parse(readFileSync(join(app.dataDir, 'settings.json'), 'utf8')).source).toBe('Local');
    await page.getByRole('link', { name: 'Display', exact: true }).click();
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['1,234']);
  });
  await test.step('手順2: ローカル記録の更新を描画', async () => {
    add(2000);
    await expect.poll(async () => (await drawing(page)).tokens, { timeout: 30_000 }).toEqual(['3,234']);
  });
  await test.step('手順3: 保存したLocal設定で再起動', async () => {
    await app.restart();
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['3,234']);
  });
  await test.step('手順4: Hubに切り替える', async () => {
    await connectHub(page, app.hub);
    app.hub.send('snapshot', stats());
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['1,234,567']);
  });
  await test.step('受け入れ条件', async () => {
    add(100_000);
    // Observe several local polling intervals: the selected Hub remains authoritative.
    await page.waitForTimeout(3000);
    expect((await drawing(page)).tokens).toEqual(['1,234,567']);
    expect(JSON.parse(readFileSync(join(app.dataDir, 'settings.json'), 'utf8')).source).toBe('Hub');
  });
});
