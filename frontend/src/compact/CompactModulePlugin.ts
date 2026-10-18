import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { normalizePath, type Plugin } from 'vite';

// Match Vite's actual, absolute module IDs. No suffix, substring, or query match is allowed.
const modules = [
  {
    original: '../routes/index.tsx',
    replacement: './CompactDisplayRoute.tsx',
    code: "export { Route } from '../compact/CompactDisplayRoute';\n",
  },
  {
    original: '../routes/settings.tsx',
    replacement: './CompactSettingsRoute.tsx',
    code: "export { Route } from '../compact/CompactSettingsRoute';\n",
  },
  {
    original: '../usecases/browse-styles/StyleList.tsx',
    replacement: './CompactStyleList.tsx',
    code: "export { StyleList } from '../../compact/CompactStyleList';\n",
  },
] as const;

export const compactModuleTargets = modules.map(({ original, replacement, code }) => ({
  original: normalizePath(fileURLToPath(new URL(original, import.meta.url))),
  replacement: normalizePath(fileURLToPath(new URL(replacement, import.meta.url))),
  code,
}));

export function compactReplacementForId(id: string): string | null {
  return compactModuleTargets.find(target => target.original === normalizePath(id))?.code ?? null;
}

export function missingCompactModules(seen: ReadonlySet<string>): string[] {
  return compactModuleTargets.filter(target => !seen.has(target.original)).map(target => target.original);
}

export function compactModulePlugin(): Plugin {
  const seen = new Set<string>();
  return {
    name: 'compact-module-replacement',
    enforce: 'pre',
    buildStart() {
      seen.clear();
      for (const target of compactModuleTargets) {
        if (!existsSync(target.original) || !existsSync(target.replacement)) {
          this.error(`Compact module replacement path missing: ${target.original} -> ${target.replacement}`);
        }
      }
    },
    transform(_source, id) {
      const replacement = compactReplacementForId(id);
      if (replacement === null) return null;
      seen.add(normalizePath(id));
      return { code: replacement, map: null };
    },
    // Production builds must not silently ship the upstream UI when a route or import changes.
    generateBundle(_options, bundle) {
      const missing = missingCompactModules(seen);
      if (missing.length > 0) this.error(`Compact module replacement incomplete: ${missing.join(', ')}`);
      const chunks = Object.values(bundle).filter(item => item.type === 'chunk').map(item => item.code).join('\n');
      // Check what is emitted, not merely what TypeScript or the transform hook accepted.
      for (const marker of ['compact-preview', 'Compact 3.5-inch is the fixed display style for Rev.A.']) {
        if (!chunks.includes(marker)) this.error(`Compact component absent from production bundle: ${marker}`);
      }
    },
  };
}
