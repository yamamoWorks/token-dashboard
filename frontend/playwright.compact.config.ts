import { defineConfig, devices } from '@playwright/test';

// Deliberately independent of the upstream suite and its webServer.
export default defineConfig({
  testDir: './tests/compact-e2e',
  globalSetup: './tests/compact-e2e/setup.ts',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 20_000 },
  use: {
    ...devices['Desktop Chrome'], headless: true,
    trace: 'retain-on-failure', screenshot: 'only-on-failure',
    launchOptions: {
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
      ignoreDefaultArgs: process.env.PLAYWRIGHT_IGNORE_DISABLE_EXTENSIONS === '1' ? ['--disable-extensions'] : undefined,
    },
  },
  reporter: [['list'], ['html', { outputFolder: 'playwright-report/compact', open: 'never' }]],
  outputDir: 'test-results/compact',
});
