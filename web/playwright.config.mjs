import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 60000,
  fullyParallel: true,
  workers: process.env.CI ? 2 : 4,
  retries: 0,
  reporter: 'list',
  use: {
    browserName: 'chromium', baseURL: 'http://127.0.0.1:5191',
    screenshot: 'only-on-failure', trace: 'retain-on-failure',
  },
  webServer: { command: 'node tests/server.mjs', url: 'http://127.0.0.1:5191', reuseExistingServer: false },
});
