// The approved scenarios in ../features, run in a browser against the real shipwake binary (make
// test-acceptance). Proposed scenarios wait for approval and don't run.
import { defineConfig } from '@playwright/test';
import { defineBddConfig } from 'playwright-bdd';

const testDir = defineBddConfig({
  featuresRoot: '../features',
  features: '../features/*.feature',
  steps: 'acceptance/*.mjs',
  tags: process.env.SHIPWAKE_TAGS ?? 'not @proposed', // SHIPWAKE_TAGS=@proposed tries proposals' tests
  outputDir: '.features-gen',
});

export default defineConfig({
  testDir,
  timeout: 60000,
  fullyParallel: true,
  workers: process.env.CI ? 2 : 4,
  retries: 0,
  reporter: 'list',
  use: { browserName: 'chromium', video: 'on', screenshot: 'only-on-failure', trace: 'retain-on-failure' },
});
