import { test, expect, connectHub, drawing, preview, provider, stats } from './support';

test('Compactの1/2窓・未知割合・リング順・480×320の視覚検証', async ({ page, app }) => {
  await test.step('開始条件', async () => { await connectHub(page, app.hub); });
  await test.step('手順1: Waitingと1プロバイダー1窓', async () => {
    await expect.poll(async () => (await drawing(page)).waiting).toEqual(['Waiting for Hub']);
    app.hub.send('snapshot', stats([provider('alpha', [60])]));
    await expect.poll(async () => (await drawing(page)).percentages).toEqual(['60%']);
    expect((await drawing(page)).providers).toEqual(['alpha']);
    expect((await drawing(page)).labels).toEqual(['Session']);
    expect((await drawing(page)).resets[0]).toMatch(/2h/);
  });
  await test.step('手順2: 2プロバイダー・短い窓が外周・未知割合は塗らない', async () => {
    // Weekly has lower remaining and is selected first by risk, then reordered by duration.
    app.hub.send('stats', stats([provider('alpha', [70, 10]), provider('unknown', [null])]));
    await expect.poll(async () => (await drawing(page)).percentages).toEqual(['70%', '10%', '—']);
    const view = await drawing(page);
    expect(view.providers).toEqual(['alpha', 'unknown']);
    expect(view.labels).toEqual(['Session', 'Weekly', 'Session']);
    expect(view.rings.map(r => r.fills)).toEqual([2, 2, 0]);
    expect(view.tokens).toEqual(['1,234,567']);
    expect(view.cost).toEqual(['$12.34']);
    // Inspect real raster pixels in Compact coordinates, including the blank virtual canvas.
    const pixels = await page.evaluate(async src => {
      const img = new Image(); img.src = src; await img.decode();
      const c = document.createElement('canvas'); c.width = 1920; c.height = 462;
      const ctx = c.getContext('2d')!; ctx.drawImage(img, 0, 0);
      const count = (area: number[], color: number[]) => {
        const [x, y, w, h] = area; const d = ctx.getImageData(x, y, w, h).data;
        let result = 0;
        for (let i = 0; i < d.length; i += 4) if (color.every((v, j) => d[i + j] === v)) result++;
        return result;
      };
      const radius = (color: number[]) => {
        const d = ctx.getImageData(8, 96, 228, 145).data;
        let sum = 0; let n = 0;
        for (let i = 0; i < d.length; i += 4) {
          if (!color.every((v, j) => d[i + j] === v)) continue;
          const x = 8 + (i / 4) % 228; const y = 96 + Math.floor(i / 4 / 228);
          sum += Math.hypot(x - 122, y - 168.5); n++;
        }
        return sum / n;
      };
      return {
        size: [img.naturalWidth, img.naturalHeight],
        outside: count([480, 0, 1440, 462], [15, 17, 23]),
        below: count([0, 320, 480, 142], [15, 17, 23]),
        firstNormal: count([8, 59, 228, 253], [116, 102, 224]),
        firstDanger: count([8, 59, 228, 253], [240, 97, 109]),
        unknownNormal: count([244, 59, 228, 253], [116, 102, 224]),
        outerRadius: radius([116, 102, 224]), innerRadius: radius([240, 97, 109]),
      };
    }, await preview(page));
    expect(pixels.size).toEqual([1920, 462]);
    expect(pixels.outside).toBe(1440 * 462);
    expect(pixels.below).toBe(480 * 142);
    expect(pixels.firstNormal).toBeGreaterThan(100);
    expect(pixels.firstDanger).toBeGreaterThan(10);
    expect(pixels.unknownNormal).toBe(0);
    expect(pixels.outerRadius).toBeCloseTo(64.8, 0);
    expect(pixels.innerRadius).toBeCloseTo(51.84, 0);
  });
  await test.step('受け入れ条件: 左上クリップと狭い画面への縮小', async () => {
    for (const width of [1280, 540]) {
      await page.setViewportSize({ width, height: 900 });
      const viewport = page.getByTestId('compact-preview');
      const geometry = await viewport.evaluate(element => {
        const box = element.getBoundingClientRect();
        const img = element.querySelector('img')!; const rect = img.getBoundingClientRect();
        return { width: box.width, height: box.height, imageWidth: rect.width, imageHeight: rect.height,
          left: rect.left - box.left, top: rect.top - box.top, overflow: getComputedStyle(element).overflow };
      });
      expect(geometry.width).toBeLessThanOrEqual(480);
      expect(geometry.height).toBeCloseTo(geometry.width * 320 / 480, 0);
      expect(geometry.imageWidth).toBeCloseTo(geometry.width * 4, 0);
      expect(geometry.imageHeight).toBeCloseTo(geometry.width * 462 / 480, 0);
      expect([geometry.left, geometry.top]).toEqual([0, 0]);
      expect(geometry.overflow).toBe('hidden');
      if (width === 540) expect(geometry.width).toBeLessThan(480);
      await test.info().attach(`compact-${width}px`, { body: await viewport.screenshot(), contentType: 'image/png' });
    }
  });
});
