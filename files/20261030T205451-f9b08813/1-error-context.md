# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: recordings.feature.spec.js >> Watching what a scenario did >> A blank moment holds the last picture
- Location: .features-gen/recordings.feature.spec.js:12:3

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: locator('main img[data-still]')
Expected: visible
Timeout: 5000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" locator('main img[data-still]') with timeout 5000ms
  - waiting for locator('main img[data-still]')

```

```yaml
- banner:
  - link "Shipwake":
    - /url: /#/
  - button "Projects": bills
  - button "Branch": main
  - button "Prompt for an agent"
  - button "Settings"
- main:
  - button "‹ Back"
  - heading "Acceptance" [level=1]
  - text: Oct 30, 2026, 8:55 PM
  - term: Branch
  - definition:
    - code: main
  - term: Commit
  - definition:
    - link "a617283":
      - /url: /#/bills/commit/a6172830e107c89688d02717a639a1cfb4278645
      - code: a617283
    - text: "feat: split a bill between friends"
  - term: Stable tag
  - definition: —
  - text: Failed 0 Passed 1
  - heading "Timeline" [level=2]
  - button "Reloading"
  - text: 0 500 ms 1 s 1.5 s 2 s 2.5 s 3 s
  - heading "Tests" [level=2]
  - text: Splitting a bill
  - img "1 passed": "1"
  - group:
    - text: Reloading
    - button "Copy the name"
    - img "Attachments"
    - text: 3.0 s Speed
    - button "0.5×" [pressed]
    - button "0.75×"
    - button "1×"
    - button "2×"
    - text: Subtitles
    - button "Off" [pressed]
    - button "On"
    - text: Attempts 0
```

# Test source

```ts
  1   | import { copyFileSync, mkdirSync } from 'node:fs';
  2   | import { join } from 'node:path';
  3   | import { expect } from '@playwright/test';
  4   | import { Given, Then, When } from './project.mjs';
  5   | 
  6   | const page = (color, text) =>
  7   |   `<body style="margin:0;background:${color};font:48px sans-serif;display:grid;place-items:center;height:100vh"><h1>${text}</h1></body>`;
  8   | 
  9   | /** Records a browser page through these moments ([html, seconds]; null html is a blank page) and keeps the
  10  |  *  file next to the scenario's report, as a test runner would. Its name in the report. */
  11  | async function recording(browser, repo, name, moments) {
  12  |   const dir = join(repo.dir, '..', 'report', `${name}-video`);
  13  |   const context = await browser.newContext({ recordVideo: { dir, size: { width: 640, height: 360 } }, viewport: { width: 640, height: 360 } });
  14  |   const p = await context.newPage();
  15  |   for (const [html, seconds] of moments) {
  16  |     await p.setContent(html ?? '');
  17  |     await p.waitForTimeout(seconds * 1000);
  18  |   }
  19  |   const path = await p.video().path();
  20  |   await context.close();
  21  |   mkdirSync(join(repo.dir, '..', 'report'), { recursive: true });
  22  |   copyFileSync(path, join(repo.dir, '..', 'report', `${name}.webm`));
  23  |   return `${name}.webm`;
  24  | }
  25  | 
  26  | const scenario = (name, video) => ({
  27  |   feature: 'Splitting a bill',
  28  |   name,
  29  |   status: 'passed',
  30  |   duration: 3000,
  31  |   attachments: [{ name: 'Recording', path: video, type: 'video/webm' }],
  32  | });
  33  | 
  34  | /** The scenario's video on its run page, opened. */
  35  | async function openRecording(page, served, name) {
  36  |   await served.open(page);
  37  |   await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  38  |   await page.getByText(name, { exact: true }).click();
  39  |   const video = page.locator('main video').first();
  40  |   await expect(video).toBeVisible();
  41  |   return video;
  42  | }
  43  | 
  44  | const at = (video) => video.evaluate((v) => v.currentTime);
  45  | 
  46  | Given('a scenario recorded while its page was still blank for the first second', async ({ browser, repo }) => {
  47  |   const tested = repo.commit('feat: split a bill between friends');
  48  |   const video = await recording(browser, repo, 'late', [[null, 1.2], [page('#4f5bd5', 'Bills'), 1.5]]);
  49  |   repo.record(tested, 'Acceptance', [scenario('Late page', video)]);
  50  | });
  51  | 
  52  | When("I open the scenario's recording", async ({ page, served, world }) => {
  53  |   world.video = await openRecording(page, served, 'Late page');
  54  | });
  55  | 
  56  | Then('it shows the page from the start, not a blank screen', async ({ world }) => {
  57  |   // once the player has found the blank start, it stands, still paused, where the page first shows something
  58  |   await expect(world.video).toHaveAttribute('data-scan', /samples/, { timeout: 30000 });
  59  |   const scan = await world.video.evaluate((v) => `${v.dataset.scan}; blank: ${v.dataset.blanks || 'none'}`);
  60  |   expect(scan, 'the scan should find the blank first second').toMatch(/blank: 0\.0-1\.\d/);
  61  |   await expect.poll(() => at(world.video), { timeout: 5000 }).toBeGreaterThan(0.9);
  62  |   expect(await world.video.evaluate((v) => v.paused)).toBe(true);
  63  | });
  64  | 
  65  | Given('a scenario recorded while its page went blank for a second in the middle', async ({ browser, repo }) => {
  66  |   const tested = repo.commit('feat: split a bill between friends');
  67  |   const video = await recording(browser, repo, 'reload', [[page('#4f5bd5', 'Bills'), 1.5], [null, 1.2], [page('#2a9d5c', 'Shares'), 1.5]]);
  68  |   repo.record(tested, 'Acceptance', [scenario('Reloading', video)]);
  69  | });
  70  | 
  71  | When('I play the recording through that second', async ({ page, served, world }) => {
  72  |   world.page = page;
  73  |   world.video = await openRecording(page, served, 'Reloading');
  74  |   await world.video.evaluate((v) => {
  75  |     v.muted = true;
  76  |     v.playbackRate = 1;
  77  |     v.currentTime = 0.5;
  78  |     return v.play();
  79  |   });
  80  | });
  81  | 
  82  | Then("the page's last picture stays on screen until the page is back", async ({ world }) => {
  83  |   const still = world.page.locator('main img[data-still]');
  84  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(1.9);
  85  |   expect(await at(world.video)).toBeLessThan(2.6);
> 86  |   await expect(still).toBeVisible();
      |                       ^ Error: expect(locator).toBeVisible() failed
  87  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(3.1);
  88  |   await expect(still).toBeHidden();
  89  | });
  90  | 
  91  | Given('a feature with two recorded scenarios', async ({ browser, repo }) => {
  92  |   const tested = repo.commit('feat: split a bill between friends');
  93  |   const first = await recording(browser, repo, 'first', [[page('#4f5bd5', 'First'), 1]]);
  94  |   const second = await recording(browser, repo, 'second', [[page('#2a9d5c', 'Second'), 1]]);
  95  |   repo.record(tested, 'Acceptance', [scenario('First scenario', first), scenario('Second scenario', second)]);
  96  | });
  97  | 
  98  | When('I watch the feature', async ({ page, served, world }) => {
  99  |   world.page = page;
  100 |   await served.open(page);
  101 |   await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  102 |   await page.getByRole('link', { name: /Watch the feature/ }).click();
  103 | });
  104 | 
  105 | Then('the end of the first scenario stays on screen a moment before the second begins', async ({ world }) => {
  106 |   const video = world.page.locator('video');
  107 |   // the page notes when the first scenario ended and when the next began: no polling delay in between
  108 |   await video.evaluate((v) => {
  109 |     const w = window;
  110 |     v.addEventListener('ended', () => {
  111 |       w.endedAt ??= performance.now();
  112 |     });
  113 |     v.addEventListener('loadstart', () => {
  114 |       if (w.endedAt !== undefined) w.nextAt ??= performance.now();
  115 |     });
  116 |     v.playbackRate = 2;
  117 |   });
  118 |   await expect(world.page.getByText('2 / 2 · Second scenario')).toBeAttached({ timeout: 15000 });
  119 |   const held = await world.page.evaluate(() => window.nextAt - window.endedAt);
  120 |   expect(held).toBeGreaterThanOrEqual(1000);
  121 | });
  122 | 
```