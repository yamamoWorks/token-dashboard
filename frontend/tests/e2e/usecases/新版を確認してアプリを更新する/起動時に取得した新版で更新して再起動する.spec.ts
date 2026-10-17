import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { createHash, generateKeyPairSync, sign } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { appProcesses, launch, powershell, waitFor, windowState } from '../../support/desktop';

// Installs the real desktop build, so it runs only through `node scripts/run.mjs test:desktop`.
// Rev.A must not auto-update from the original fork, even when linker flags
// supply a signed local update feed. The test installs real desktop builds.
const root = resolve(import.meta.dirname, '../../../../..');
const appID = 'io.github.nuitsjp.token-monitor-turzx';
const installDir = join(process.env.LOCALAPPDATA ?? '', 'Programs', appID);
const installedExe = join(installDir, 'token-monitor-turzx.exe');
const installer = (version: string) => `token-monitor-turzx-${version}-amd64-setup.exe`;
const hubURL = 'http://127.0.0.1:9';
const registry = (key: string, value: string) => powershell(`(Get-ItemProperty -Path 'HKCU:\\${key}' -ErrorAction SilentlyContinue).'${value}'`);
const runKey = () => registry('Software\\Microsoft\\Windows\\CurrentVersion\\Run', appID);
const installedVersion = () => registry(`Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${appID}`, 'DisplayVersion');

function build(version: string, source: string, publicKey: string) {
  const result = spawnSync('node', ['scripts/run.mjs', 'package'], { cwd: root, stdio: 'inherit',
    env: { ...process.env, BUILD_APP_VERSION: version, BUILD_UPDATE_SOURCE: source, BUILD_UPDATE_PUBLIC_KEY: publicKey } });
  if (result.status !== 0) throw new Error(`package ${version} failed`);
  return join(root, 'bin', installer(version));
}

// The same envelope as cmd/release: the signature covers the exact payload bytes.
function publishRelease(source: string, file: string, version: string, key: ReturnType<typeof generateKeyPairSync>['privateKey']) {
  const data = readFileSync(file);
  const payload = Buffer.from(JSON.stringify({ appID, version, os: 'windows', arch: 'amd64', filename: installer(version),
    size: data.length, sha256: createHash('sha256').update(data).digest('hex'), notes: '' }));
  copyFileSync(file, join(source, installer(version)));
  writeFileSync(join(source, 'update.json'), JSON.stringify({ payload: payload.toString('base64'), signature: sign(null, payload, key).toString('base64') }));
  return createHash('sha256').update(data).digest('hex');
}

// A saved connection in the same form as internal/settings (DPAPI, current user, app ID as entropy).
function saveSettings(dataDir: string) {
  const plain = JSON.stringify({ url: hubURL, token: 'e2e-hub-token' });
  const sealed = powershell(`Add-Type -AssemblyName System.Security
[Convert]::ToBase64String([Security.Cryptography.ProtectedData]::Protect([Text.Encoding]::UTF8.GetBytes('${plain}'), [Text.Encoding]::UTF8.GetBytes('${appID}'), 'CurrentUser'))`);
  mkdirSync(dataDir, { recursive: true });
  writeFileSync(join(dataDir, 'settings.json'), JSON.stringify({ source: 'Hub', connection: sealed, displayID: '', displayName: '' }, null, 2));
}

function uninstall() {
  for (const pid of appProcesses(installedExe)) process.kill(pid);
  if (!existsSync(join(installDir, 'uninstall.exe'))) return;
  // A copy runs the uninstaller in place, so the call returns after the files are removed.
  const copy = join(tmpdir(), 'token-monitor-turzx-e2e-uninstall.exe');
  copyFileSync(join(installDir, 'uninstall.exe'), copy);
  spawnSync(copy, ['/S', `_?=${installDir}`]);
  rmSync(copy, { force: true });
  rmSync(installDir, { recursive: true, force: true });
}

test('Rev.Aではビルド時の更新元指定があっても更新せずUIも表示しない @desktop', async () => {
  test.skip(process.env.DESKTOP_E2E !== '1', 'installs the desktop app; run node scripts/run.mjs test:desktop');
  test.setTimeout(15 * 60_000);
  expect(powershell('@(Get-Process token-monitor-turzx -ErrorAction SilentlyContinue).Count'), 'stop every running Token Dashboard first').toBe('0');
  expect(existsSync(installedExe), 'uninstall Token Dashboard first').toBe(false);
  const work = mkdtempSync(join(tmpdir(), 'turzx-update-disabled-e2e-'));
  const source = join(work, 'release');
  const dataDir = join(work, 'data');
  const env = { ...process.env, WAILS_DATA_DIR: dataDir };
  mkdirSync(source);
  try {
    const { publicKey, privateKey } = generateKeyPairSync('ed25519');
    const rawKey = publicKey.export({ format: 'der', type: 'spki' }).subarray(-32).toString('base64');
    const current = join(work, installer('0.1.0'));
    copyFileSync(build('0.1.0', source, rawKey), current);
    publishRelease(source, build('0.2.0', source, rawKey), '0.2.0', privateKey);
    saveSettings(dataDir);
    expect(spawnSync(current, ['/S']).status).toBe(0);
    expect(installedVersion()).toBe('0.1.0');
    expect(runKey()).toBe(`"${installedExe}"`);

    launch(installedExe, env);
    const pid = (await waitFor('the app to start', () => appProcesses(installedExe)))[0];
    launch(installedExe, env); // second instance opens the settings window

    // Allow an incorrectly enabled startup updater time to stage the signed release.
    await new Promise(resolve => setTimeout(resolve, 5_000));
    const window = windowState(pid);
    expect(window.title).toBe('Token Dashboard v0.1.0');
    expect(window.names).not.toContain('Update and restart');
    expect(window.text).not.toContain('Version 0.2.0 is ready to install.');
    expect(installedVersion()).toBe('0.1.0');
    const updates = join(dataDir, 'updates');
    expect(existsSync(updates) ? readdirSync(updates) : []).toEqual([]);
    expect(window.hubURL).toBe(hubURL);
  } finally {
    uninstall();
    rmSync(work, { recursive: true, force: true });
  }
  expect(existsSync(installedExe)).toBe(false);
  expect(runKey()).toBe('');
});
