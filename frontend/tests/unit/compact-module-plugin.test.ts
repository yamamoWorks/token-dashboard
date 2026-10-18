import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  compactModulePlugin, compactModuleTargets, compactReplacementForId, missingCompactModules,
} from '../../src/compact/CompactModulePlugin';

const modules = [
  { original: 'routes/index.tsx', replacement: 'compact/CompactDisplayRoute.tsx', name: 'Route' },
  { original: 'routes/settings.tsx', replacement: 'compact/CompactSettingsRoute.tsx', name: 'Route' },
  { original: 'usecases/browse-styles/StyleList.tsx', replacement: 'compact/CompactStyleList.tsx', name: 'StyleList' },
] as const;

const absolute = (path: string) => fileURLToPath(new URL(`../../src/${path}`, import.meta.url)).replaceAll('\\', '/');

describe('CompactModulePlugin', () => {
  it('maps exactly the original three modules to separately exported Compact modules', () => {
    expect(compactModuleTargets).toHaveLength(modules.length);
    for (const item of modules) {
      const id = absolute(item.original);
      const target = compactModuleTargets.find(value => value.original === id);
      expect(target?.replacement).toBe(absolute(item.replacement));
      expect(target?.code).toContain(`export { ${item.name} } from `);
      expect(readFileSync(absolute(item.replacement), 'utf8')).toMatch(
        new RegExp(`export\\s+const\\s+${item.name}\\s*=`),
      );
      expect(compactReplacementForId(id)).toBe(target?.code);
    }
  });

  it('does not transform substrings, unknown modules, or query variants', () => {
    const original = compactModuleTargets[0].original;
    expect(compactReplacementForId(original + '?raw')).toBeNull();
    expect(compactReplacementForId(original + '/unexpected')).toBeNull();
    expect(compactReplacementForId(original + '.bak')).toBeNull();
    expect(compactReplacementForId(original.replace('index.tsx', 'index-other.tsx'))).toBeNull();
    expect(compactReplacementForId('/unrelated/src/routes/index.tsx')).toBeNull();
  });

  it('applies the replacements as Vite transforms before React processing', () => {
    const plugin = compactModulePlugin();
    expect(plugin.enforce).toBe('pre');
    expect(plugin.name).toBe('compact-module-replacement');
    const transform = plugin.transform;
    if (typeof transform !== 'function') throw new Error('transform hook missing');
    for (const target of compactModuleTargets) {
      const output = transform.call({} as never, 'upstream source', target.original, {} as never);
      expect(output).toEqual({ code: target.code, map: null });
    }
    expect(transform.call({} as never, 'ordinary source', '/unrelated/index.tsx', {} as never)).toBeNull();
  });

  it('fails closed in production when any replacement was not transformed', () => {
    const expected = compactModuleTargets.map(target => target.original);
    expect(missingCompactModules(new Set())).toEqual(expected);
    expect(missingCompactModules(new Set(expected.slice(0, 2)))).toEqual([expected[2]]);
    expect(missingCompactModules(new Set(expected))).toEqual([]);
  });

  it('rejects bundles that omit Compact modules or their emitted UI code', () => {
    const plugin = compactModulePlugin();
    const transform = plugin.transform;
    const generateBundle = plugin.generateBundle;
    if (typeof transform !== 'function' || typeof generateBundle !== 'function') {
      throw new Error('required Vite hooks are missing');
    }
    const context = { error: (message: string) => { throw new Error(message); } } as never;
    const bundle = (code: string) => ({ 'app.js': { type: 'chunk', code } }) as never;
    expect(() => generateBundle.call(context, {} as never, bundle(''), false))
      .toThrow(/replacement incomplete/);
    for (const target of compactModuleTargets) {
      transform.call({} as never, '', target.original, {} as never);
    }
    expect(() => generateBundle.call(context, {} as never, bundle(''), false))
      .toThrow(/compact-preview/);
    expect(() => generateBundle.call(context, {} as never,
      bundle('compact-preview Compact 3.5-inch is the fixed display style for Rev.A.'), false))
      .not.toThrow();
  });

  it('does not modify original TypeScript modules', () => {
    const original = readFileSync(absolute('routes/index.tsx'), 'utf8');
    expect(original).toContain('StyleSelect');
    const originalSettings = readFileSync(absolute('usecases/configure-hub/ConfigureHub.tsx'), 'utf8');
    expect(originalSettings).toContain('Cycle time');
    expect(originalSettings).toContain('export function StyleSelect');
  });
});
