import { test, expect, connectHub, drawing, preview, recordFrames, stats } from './support';

test('Canvas PNG生成・応答・取得失敗からの復旧', async ({ page, app }) => {
  const frames = recordFrames(page);
  await test.step('開始条件', async () => {
    await connectHub(page, app.hub);
    app.hub.send('snapshot', stats());
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['1,234,567']);
  });
  await test.step('手順1: PNGと従来JPEGの両応答を生成', async () => {
    await expect.poll(() => frames.filter(f => !f.error && f.png && f.jpeg).length).toBeGreaterThan(0);
    const frame = frames.filter(f => !f.error && f.png && f.jpeg).at(-1)!;
    expect(Buffer.from(frame.png, 'base64').subarray(0, 8)).toEqual(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
    expect(Buffer.from(frame.jpeg, 'base64').subarray(0, 2)).toEqual(Buffer.from([255, 216]));
    const dimensions = await page.evaluate(async ({ png, jpeg }) => {
      const size = async (type: string, value: string) => {
        const image = new Image(); image.src = `data:image/${type};base64,${value}`;
        await image.decode(); return [image.naturalWidth, image.naturalHeight];
      };
      return [await size('png', png), await size('jpeg', jpeg)];
    }, frame);
    // Go Bridge tests prove that this ordinary JPEG is discarded before Serial output.
    expect(dimensions).toEqual([[1920, 462], [462, 1920]]);
    const src = await preview(page);
    expect(frames.some(f => `data:image/png;base64,${f.png}` === src)).toBe(true);
  });
  let current = await preview(page);
  await test.step('手順2: Preview取得の一時障害から復旧', async () => {
    let blocked = 0;
    await page.route('**/wails/runtime', async route => {
      const body = route.request().postDataJSON();
      if (body?.args?.methodName === 'token-monitor-turzx/internal/display.Service.Preview') {
        blocked++;
        await route.abort('failed');
      } else await route.continue();
    });
    app.hub.send('stats', stats(undefined, 2222));
    await expect.poll(() => blocked).toBeGreaterThan(0);
    await page.unroute('**/wails/runtime');
    app.hub.send('stats', stats(undefined, 3333));
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['3,333']);
    await expect.poll(() => preview(page)).not.toBe(current);
    current = await preview(page);
    expect(frames.some(f => `data:image/png;base64,${f.png}` === current)).toBe(true);
  });
  await test.step('手順3: Canvasエンコード失敗後に次の描画で復旧', async () => {
    await page.evaluate(() => {
      const original = HTMLCanvasElement.prototype.toDataURL;
      HTMLCanvasElement.prototype.toDataURL = function(...args) {
        if (args[0] !== 'image/png') return original.apply(this, args);
        HTMLCanvasElement.prototype.toDataURL = original;
        throw new Error('compact-test-encode-failure: ' + args[0]);
      };
    });
    app.hub.send('stats', stats(undefined, 4444));
    await expect.poll(() => frames.some(f => f.error.includes('compact-test-encode-failure'))).toBe(true);
    app.hub.send('stats', stats(undefined, 5555));
    await expect.poll(async () => (await drawing(page)).tokens).toEqual(['5,555']);
  });
  await test.step('受け入れ条件', async () => {
    await expect.poll(() => preview(page)).not.toBe(current);
    const src = await preview(page);
    expect(frames.some(f => !f.error && `data:image/png;base64,${f.png}` === src)).toBe(true);
  });
});
