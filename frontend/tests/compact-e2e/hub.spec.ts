import { test, expect, connectHub, drawing, preview, provider, stats, token, recordFrames } from './support';

test('Hub取得・描画更新・PNGプレビュー追従と再接続', async ({ page, app }) => {
  const frames = recordFrames(page);
  await test.step('開始条件', async () => { await connectHub(page, app.hub); });
  await test.step('手順1: snapshot前はWaiting', async () => {
    await expect.poll(async () => (await drawing(page)).waiting).toEqual(['Waiting for Hub']);
    expect((await drawing(page)).tokens).toEqual([]);
    const request = app.hub.requests.at(-1)!;
    expect(request.path).toBe('/api/stats/stream');
    expect(request.headers.authorization).toBe(`Bearer ${token}`);
    expect(request.headers['x-token-monitor-stream']).toBe('2');
  });
  await test.step('手順2: Hubの値を描画', async () => {
    app.hub.send('snapshot', stats());
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['1,234,567']);
    expect((await drawing(page)).cost).toEqual(['$12.34']);
    expect((await drawing(page)).providers).toEqual(['alpha']);
  });
  // Equality against successful completions proves that Preview returns the renderer's PNG.
  let awaitPreview = '';
  await test.step('手順3: statsで更新しプレビューへ反映', async () => {
    const before = await preview(page);
    app.hub.send('stats', stats([provider('alpha', [10])], 7654321));
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['7,654,321']);
    await expect.poll(() => preview(page)).not.toBe(before);
    awaitPreview = await preview(page);
    expect(frames.some(f => !f.error && `data:image/png;base64,${f.png}` === awaitPreview)).toBe(true);
  });
  await test.step('手順4: 受信断の間は最新表示を保持', async () => {
    const before = (await drawing(page)).tokens;
    // Malformed input is rejected at the Hub boundary and must not clear the prior snapshot.
    app.hub.send('stats', { periods: 'invalid', limits: null });
    await page.waitForTimeout(1500);
    expect((await drawing(page)).tokens).toEqual(before);
    const requests = app.hub.requests.length;
    app.hub.disconnect();
    await expect.poll(() => app.hub.requests.length).toBeGreaterThan(requests);
    await expect.poll(() => app.hub.streams()).toBe(1);
    expect((await drawing(page)).tokens).toEqual(before);
    app.hub.send('snapshot', stats([provider('alpha', [50])], 8765));
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['8,765']);
  });
  await test.step('受け入れ条件: 再起動して新規snapshotを受信', async () => {
    await app.restart();
    await expect.poll(() => app.hub.streams()).toBe(1);
    await expect.poll(async () => (await drawing(page)).waiting).toEqual(['Waiting for Hub']);
    app.hub.send('snapshot', stats([provider('beta', [60])], 999));
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['999']);
    await expect(page.getByText(token, { exact: true })).toHaveCount(0);
  });
});
