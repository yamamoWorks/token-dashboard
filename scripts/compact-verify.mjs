// Compact verification never calls verify/ci or the upstream Playwright config.
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
process.chdir(root);
const nodeVersion = readFileSync('.nvmrc', 'utf8').trim();
if (process.versions.node !== nodeVersion) throw new Error(`Requires Node.js ${nodeVersion}`);
const windows = process.platform === 'win32';
function run(command, args) {
  const result = command === 'npm' && windows
    ? spawnSync(process.env.ComSpec || 'cmd.exe', ['/d', '/s', '/c', `npm ${args.join(' ')}`], { cwd: root, stdio: 'inherit' })
    : spawnSync(command, args, { cwd: root, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
const node = (...args) => run(process.execPath, args);
const npm = (...args) => run('npm', ['--prefix', 'frontend', ...args]);
function assertCompact(binary) {
  const bytes = readFileSync(binary);
  for (const marker of ['compact-preview', 'Compact 3.5-inch is the fixed display style for Rev.A.', '3.5-inch compact display']) {
    if (!bytes.includes(Buffer.from(marker))) throw new Error(`${binary}: missing Compact build marker ${marker}`);
  }
}
function buildServer() {
  node('scripts/run.mjs', 'generate');
  npm('run', 'build'); // Includes TS typecheck, actual Vite plugin and generated Wails bindings.
  node('scripts/build.mjs', 'server');
  assertCompact(`bin/token-monitor-turzx-server${windows ? '.exe' : ''}`);
}
const command = process.argv[2] ?? 'verify';
if (!['build-server', 'verify', 'ci'].includes(command)) throw new Error(`Unknown Compact command: ${command}`);
if (command !== 'build-server') {
  run('go', ['test', '.', './internal/...', './cmd/release/...']);
  run('go', ['test', '-ldflags', '-X main.buildUpdateSource=https://example.invalid/update.json -X main.buildUpdatePublicKey=override-key', '.', '-run', 'TestCompactUpdaterWithEmbeddedMetadataAndLinkerOverrides', '-count=1']);
  run('go', ['vet', './...']);
  run('go', ['vet', '-tags', 'server', './...']);
  run('python', ['scripts/doc_check.py', '.']);
  npm('run', 'lint');
  npm('run', 'test');
}
buildServer();
if (command !== 'build-server') npm('run', 'test:e2e', '--', '--config', 'playwright.compact.config.ts');
if (command === 'ci') {
  if (!windows) throw new Error('Compact ci requires Windows desktop and NSIS');
  node('scripts/build.mjs', 'desktop');
  const app = JSON.parse(readFileSync('build/app.json', 'utf8'));
  assertCompact(resolve('bin', app.executable));
  node('scripts/build.mjs', 'package');
}
