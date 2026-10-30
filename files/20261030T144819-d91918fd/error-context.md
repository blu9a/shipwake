# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: recordings.feature.spec.js >> Watching what a scenario did >> A feature's video ends each scenario on its result
- Location: .features-gen/recordings.feature.spec.js:18:3

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: true
Received: false
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
    - generic [ref=e36]:
      - button "Close" [ref=e37] [cursor=pointer]
      - generic [ref=e40]:
        - generic [ref=e41]:
          - generic [ref=e42]: 2 / 2 · Second scenario
          - generic [ref=e44]:
            - button "First scenario" [ref=e45] [cursor=pointer]
            - button "Second scenario" [ref=e47] [cursor=pointer]
          - generic [ref=e49]:
            - generic [ref=e50]:
              - generic [ref=e51]: Speed
              - generic [ref=e52]:
                - button "0.5×" [pressed] [ref=e53] [cursor=pointer]
                - button "0.75×" [ref=e54] [cursor=pointer]
                - button "1×" [ref=e55] [cursor=pointer]
                - button "2×" [ref=e56] [cursor=pointer]
            - generic [ref=e57]:
              - generic [ref=e58]: Subtitles
              - generic [ref=e59]:
                - button "Off" [pressed] [ref=e60] [cursor=pointer]
                - button "On" [ref=e61] [cursor=pointer]
        - generic [ref=e62]:
          - generic [ref=e63]: Splitting a bill
          - list [ref=e64]:
            - listitem [ref=e65]:
              - button "First scenario" [ref=e66] [cursor=pointer]
            - listitem [ref=e71]:
              - button "Second scenario" [ref=e72] [cursor=pointer]
```

# Test source

```ts
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
  57  |   // still paused, it already stands where the page first shows something
  58  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(0.9);
  59  |   expect(await world.video.evaluate((v) => v.paused)).toBe(true);
  60  | });
  61  | 
  62  | Given('a scenario recorded while its page went blank for a second in the middle', async ({ browser, repo }) => {
  63  |   const tested = repo.commit('feat: split a bill between friends');
  64  |   const video = await recording(browser, repo, 'reload', [[page('#4f5bd5', 'Bills'), 1.5], [null, 1.2], [page('#2a9d5c', 'Shares'), 1.5]]);
  65  |   repo.record(tested, 'Acceptance', [scenario('Reloading', video)]);
  66  | });
  67  | 
  68  | When('I play the recording through that second', async ({ page, served, world }) => {
  69  |   world.page = page;
  70  |   world.video = await openRecording(page, served, 'Reloading');
  71  |   await world.video.evaluate((v) => {
  72  |     v.muted = true;
  73  |     v.playbackRate = 1;
  74  |     v.currentTime = 0.5;
  75  |     return v.play();
  76  |   });
  77  | });
  78  | 
  79  | Then("the page's last picture stays on screen until the page is back", async ({ world }) => {
  80  |   const still = world.page.locator('main img[data-still]');
  81  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(1.9);
  82  |   expect(await at(world.video)).toBeLessThan(2.6);
  83  |   await expect(still).toBeVisible();
  84  |   await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(3.1);
  85  |   await expect(still).toBeHidden();
  86  | });
  87  | 
  88  | Given('a feature with two recorded scenarios', async ({ browser, repo }) => {
  89  |   const tested = repo.commit('feat: split a bill between friends');
  90  |   const first = await recording(browser, repo, 'first', [[page('#4f5bd5', 'First'), 1]]);
  91  |   const second = await recording(browser, repo, 'second', [[page('#2a9d5c', 'Second'), 1]]);
  92  |   repo.record(tested, 'Acceptance', [scenario('First scenario', first), scenario('Second scenario', second)]);
  93  | });
  94  | 
  95  | When('I watch the feature', async ({ page, served, world }) => {
  96  |   world.page = page;
  97  |   await served.open(page);
  98  |   await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  99  |   await page.getByRole('link', { name: /Watch the feature/ }).click();
  100 | });
  101 | 
  102 | Then('the end of the first scenario stays on screen a moment before the second begins', async ({ world }) => {
  103 |   const video = world.page.locator('video');
  104 |   await video.evaluate((v) => {
  105 |     v.playbackRate = 2;
  106 |   });
  107 |   await expect.poll(() => video.evaluate((v) => v.ended), { timeout: 15000 }).toBe(true);
  108 |   await world.page.waitForTimeout(600);
  109 |   await expect(world.page.getByText('1 / 2 · First scenario')).toBeAttached();
> 110 |   expect(await video.evaluate((v) => v.ended)).toBe(true);
      |                                                ^ Error: expect(received).toBe(expected) // Object.is equality
  111 |   await expect(world.page.getByText('2 / 2 · Second scenario')).toBeAttached({ timeout: 5000 });
  112 | });
  113 | 
```