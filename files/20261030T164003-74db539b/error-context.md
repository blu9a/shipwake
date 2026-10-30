# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: recordings.feature.spec.js >> Watching what a scenario did >> A recording starts where the page shows something
- Location: .features-gen/recordings.feature.spec.js:6:3

# Error details

```
Error: expect(locator).toHaveAttribute(expected) failed

Locator: locator('main video').first()
Expected pattern: /^0\.0-1\.\d/
Received string:  ""
Timeout: 30000ms

Call log:
  - Expect "toHaveAttribute" locator('main video').first() with timeout 30000ms
  - waiting for locator('main video').first()
    2 × locator resolved to <video controls="" preload="metadata" class="_video_162t2_150" src="data/bills/files/20261030T164021-f15a3149/late.webm"></video>
      - unexpected value "null"
    62 × locator resolved to <video controls="" data-blanks="" preload="metadata" class="_video_162t2_150" src="data/bills/files/20261030T164021-f15a3149/late.webm"></video>
       - unexpected value ""

```

# Page snapshot

```yaml
- generic [ref=e3]:
  - banner [ref=e4]:
    - link "Shipwake" [ref=e5] [cursor=pointer]:
      - /url: /#/
    - button "Projects" [ref=e11] [cursor=pointer]:
      - generic "bills" [ref=e12]
    - button "Branch" [ref=e16] [cursor=pointer]:
      - generic "main" [ref=e22]
    - button "Prompt for an agent" [ref=e26] [cursor=pointer]
    - button "Settings" [ref=e31] [cursor=pointer]
  - main [ref=e35]:
    - button "‹ Back" [ref=e36] [cursor=pointer]
    - generic [ref=e37]:
      - heading "Acceptance" [level=1] [ref=e38]
      - generic [ref=e39]: Oct 30, 2026, 4:40 PM
    - generic [ref=e40]:
      - generic [ref=e41]:
        - term [ref=e42]: Branch
        - definition [ref=e43]:
          - code [ref=e44]: main
        - term [ref=e45]: Commit
        - definition [ref=e46]:
          - link [ref=e47] [cursor=pointer]:
            - /url: /#/bills/commit/a6172830e107c89688d02717a639a1cfb4278645
            - code [ref=e48]: a617283
          - text: "feat: split a bill between friends"
        - term [ref=e49]: Stable tag
        - definition [ref=e50]: —
      - generic [ref=e51]:
        - generic [ref=e52]:
          - generic [ref=e53]: Failed
          - generic [ref=e55]: "0"
        - generic [ref=e56]:
          - generic [ref=e57]: Passed
          - generic [ref=e59]: "1"
    - heading "Timeline" [level=2] [ref=e61]
    - generic [ref=e62]:
      - button "Late page" [ref=e66] [cursor=pointer]
      - generic [ref=e67]:
        - generic [ref=e68]: "0"
        - generic [ref=e69]: 500 ms
        - generic [ref=e70]: 1 s
        - generic [ref=e71]: 1.5 s
        - generic [ref=e72]: 2 s
        - generic [ref=e73]: 2.5 s
        - generic [ref=e74]: 3 s
    - heading "Tests" [level=2] [ref=e76]
    - generic [ref=e78]:
      - generic [ref=e79]:
        - generic [ref=e80]: Splitting a bill
        - img "1 passed" [ref=e82]: "1"
      - group [ref=e84]:
        - generic "Late page Copy the name Attachments 3.0 s" [active] [ref=e85] [cursor=pointer]:
          - generic [ref=e89]: Late page
          - button "Copy the name" [ref=e90]
          - img "Attachments" [ref=e94]
          - generic [ref=e97]: 3.0 s
        - generic [ref=e98]:
          - generic [ref=e102]:
            - generic [ref=e103]:
              - generic [ref=e104]: Speed
              - generic [ref=e105]:
                - button "0.5×" [pressed] [ref=e106] [cursor=pointer]
                - button "0.75×" [ref=e107] [cursor=pointer]
                - button "1×" [ref=e108] [cursor=pointer]
                - button "2×" [ref=e109] [cursor=pointer]
            - generic [ref=e110]:
              - generic [ref=e111]: Subtitles
              - generic [ref=e112]:
                - button "Off" [pressed] [ref=e113] [cursor=pointer]
                - button "On" [ref=e114] [cursor=pointer]
          - generic [ref=e115]: Attempts 0
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
> 58  |   await expect(world.video).toHaveAttribute('data-blanks', /^0\.0-1\.\d/, { timeout: 30000 });
      |                             ^ Error: expect(locator).toHaveAttribute(expected) failed
  59  |   await expect.poll(() => at(world.video), { timeout: 5000 }).toBeGreaterThan(0.9);
  60  |   expect(await world.video.evaluate((v) => v.paused)).toBe(true);
  61  | });
  62  | 
  63  | Given('a scenario recorded while its page went blank for a second in the middle', async ({ browser, repo }) => {
  64  |   const tested = repo.commit('feat: split a bill between friends');
  65  |   const video = await recording(browser, repo, 'reload', [[page('#4f5bd5', 'Bills'), 1.5], [null, 1.2], [page('#2a9d5c', 'Shares'), 1.5]]);
  66  |   repo.record(tested, 'Acceptance', [scenario('Reloading', video)]);
  67  | });
  68  | 
  69  | When('I play the recording through that second', async ({ page, served, world }) => {
  70  |   world.page = page;
  71  |   world.video = await openRecording(page, served, 'Reloading');
  72  |   await world.video.evaluate((v) => {
  73  |     v.muted = true;
  74  |     v.playbackRate = 1;
  75  |     v.currentTime = 0.5;
  76  |     return v.play();
  77  |   });
  78  | });
  79  | 
  80  | Then("the page's last picture stays on screen until the page is back", async ({ world }) => {
  81  |   const still = world.page.locator('main img[data-still]');
  82  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(1.9);
  83  |   expect(await at(world.video)).toBeLessThan(2.6);
  84  |   await expect(still).toBeVisible();
  85  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(3.1);
  86  |   await expect(still).toBeHidden();
  87  | });
  88  | 
  89  | Given('a feature with two recorded scenarios', async ({ browser, repo }) => {
  90  |   const tested = repo.commit('feat: split a bill between friends');
  91  |   const first = await recording(browser, repo, 'first', [[page('#4f5bd5', 'First'), 1]]);
  92  |   const second = await recording(browser, repo, 'second', [[page('#2a9d5c', 'Second'), 1]]);
  93  |   repo.record(tested, 'Acceptance', [scenario('First scenario', first), scenario('Second scenario', second)]);
  94  | });
  95  | 
  96  | When('I watch the feature', async ({ page, served, world }) => {
  97  |   world.page = page;
  98  |   await served.open(page);
  99  |   await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  100 |   await page.getByRole('link', { name: /Watch the feature/ }).click();
  101 | });
  102 | 
  103 | Then('the end of the first scenario stays on screen a moment before the second begins', async ({ world }) => {
  104 |   const video = world.page.locator('video');
  105 |   // the page notes when the first scenario ended and when the next began: no polling delay in between
  106 |   await video.evaluate((v) => {
  107 |     const w = window;
  108 |     v.addEventListener('ended', () => {
  109 |       w.endedAt ??= performance.now();
  110 |     });
  111 |     v.addEventListener('loadstart', () => {
  112 |       if (w.endedAt !== undefined) w.nextAt ??= performance.now();
  113 |     });
  114 |     v.playbackRate = 2;
  115 |   });
  116 |   await expect(world.page.getByText('2 / 2 · Second scenario')).toBeAttached({ timeout: 15000 });
  117 |   const held = await world.page.evaluate(() => window.nextAt - window.endedAt);
  118 |   expect(held).toBeGreaterThanOrEqual(1000);
  119 | });
  120 | 
```