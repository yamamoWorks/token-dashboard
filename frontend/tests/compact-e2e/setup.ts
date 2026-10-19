import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

export default function setup() {
  const binary = resolve(import.meta.dirname, '../../../bin/token-monitor-turzx-server' + (process.platform === 'win32' ? '.exe' : ''));
  const content = readFileSync(binary);
  for (const marker of ['compact-preview', 'Compact 3.5-inch is the fixed display style for Rev.A.', '3.5-inch compact display']) {
    if (!content.includes(Buffer.from(marker))) {
      throw new Error(`Server lacks Compact assets (${marker}). Run node scripts/compact-verify.mjs build-server first.`);
    }
  }
}
