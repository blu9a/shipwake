import { copyFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect } from '@playwright/test';
import { Given, Then, When } from './project.mjs';

const page = (color, text) =>
  `<body style="margin:0;background:${color};font:48px sans-serif;display:grid;place-items:center;height:100vh"><h1>${text}</h1></body>`;

/** Records a browser page through these moments ([html, seconds]; null html is a blank page) and keeps the
 *  file next to the scenario's report, as a test runner would. Its name in the report. */
async function recording(browser, repo, name, moments) {
  const dir = join(repo.dir, '..', 'report', `${name}-video`);
  const context = await browser.newContext({ recordVideo: { dir, size: { width: 640, height: 360 } }, viewport: { width: 640, height: 360 } });
  const p = await context.newPage();
  for (const [html, seconds] of moments) {
    await p.setContent(html ?? '');
    await p.waitForTimeout(seconds * 1000);
  }
  const path = await p.video().path();
  await context.close();
  mkdirSync(join(repo.dir, '..', 'report'), { recursive: true });
  copyFileSync(path, join(repo.dir, '..', 'report', `${name}.webm`));
  return `${name}.webm`;
}

const scenario = (name, video) => ({
  feature: 'Splitting a bill',
  name,
  status: 'passed',
  duration: 3000,
  attachments: [{ name: 'Recording', path: video, type: 'video/webm' }],
});

/** The scenario's video on its run page, opened. */
async function openRecording(page, served, name) {
  await served.open(page);
  await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  await page.getByText(name, { exact: true }).click();
  const video = page.locator('main video').first();
  await expect(video).toBeVisible();
  return video;
}

const at = (video) => video.evaluate((v) => v.currentTime);

Given('a scenario recorded while its page was still blank for the first second', async ({ browser, repo }) => {
  const tested = repo.commit('feat: split a bill between friends');
  const video = await recording(browser, repo, 'late', [[null, 1.2], [page('#4f5bd5', 'Bills'), 1.5]]);
  repo.record(tested, 'Acceptance', [scenario('Late page', video)]);
});

When("I open the scenario's recording", async ({ page, served, world }) => {
  world.video = await openRecording(page, served, 'Late page');
});

Then('it shows the page from the start, not a blank screen', async ({ world }) => {
  // still paused, it already stands where the page first shows something
  await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(0.9);
  expect(await world.video.evaluate((v) => v.paused)).toBe(true);
});

Given('a scenario recorded while its page went blank for a second in the middle', async ({ browser, repo }) => {
  const tested = repo.commit('feat: split a bill between friends');
  const video = await recording(browser, repo, 'reload', [[page('#4f5bd5', 'Bills'), 1.5], [null, 1.2], [page('#2a9d5c', 'Shares'), 1.5]]);
  repo.record(tested, 'Acceptance', [scenario('Reloading', video)]);
});

When('I play the recording through that second', async ({ page, served, world }) => {
  world.page = page;
  world.video = await openRecording(page, served, 'Reloading');
  await world.video.evaluate((v) => {
    v.muted = true;
    v.playbackRate = 1;
    v.currentTime = 0.5;
    return v.play();
  });
});

Then("the page's last picture stays on screen until the page is back", async ({ world }) => {
  const still = world.page.locator('main img[data-still]');
  await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(1.9);
  expect(await at(world.video)).toBeLessThan(2.6);
  await expect(still).toBeVisible();
  await expect.poll(() => at(world.video), { timeout: 15000 }).toBeGreaterThan(3.1);
  await expect(still).toBeHidden();
});

Given('a feature with two recorded scenarios', async ({ browser, repo }) => {
  const tested = repo.commit('feat: split a bill between friends');
  const first = await recording(browser, repo, 'first', [[page('#4f5bd5', 'First'), 1]]);
  const second = await recording(browser, repo, 'second', [[page('#2a9d5c', 'Second'), 1]]);
  repo.record(tested, 'Acceptance', [scenario('First scenario', first), scenario('Second scenario', second)]);
});

When('I watch the feature', async ({ page, served, world }) => {
  world.page = page;
  await served.open(page);
  await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  await page.getByRole('link', { name: /Watch the feature/ }).click();
});

Then('the end of the first scenario stays on screen a moment before the second begins', async ({ world }) => {
  const video = world.page.locator('video');
  await video.evaluate((v) => {
    v.playbackRate = 2;
  });
  await expect.poll(() => video.evaluate((v) => v.ended), { timeout: 15000 }).toBe(true);
  await world.page.waitForTimeout(600);
  await expect(world.page.getByText('1 / 2 · First scenario')).toBeAttached();
  expect(await video.evaluate((v) => v.ended)).toBe(true);
  await expect(world.page.getByText('2 / 2 · Second scenario')).toBeAttached({ timeout: 5000 });
});
