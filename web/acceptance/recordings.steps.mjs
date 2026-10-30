import { copyFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect } from '@playwright/test';
import { Given, Then, When } from './project.mjs';

/** A recording as a test runner keeps it, next to the scenario's report. The recordings are fixed files
 *  (web/acceptance/fixtures), made with known stretches: recording them during the run depended on the
 *  machine's speed, and a busy one dropped the blank second they're about. */
function recording(repo, name) {
  mkdirSync(join(repo.dir, '..', 'report'), { recursive: true });
  copyFileSync(join(import.meta.dirname, 'fixtures', `${name}.webm`), join(repo.dir, '..', 'report', `${name}.webm`));
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

Given('a scenario recorded while its page was still blank for the first second', async ({ repo }) => {
  const tested = repo.commit('feat: split a bill between friends');
  const video = recording(repo, 'late'); // 1.2 s white, then 1.5 s of picture
  repo.record(tested, 'Acceptance', [scenario('Late page', video)]);
});

When("I open the scenario's recording", async ({ page, served, world }) => {
  world.video = await openRecording(page, served, 'Late page');
});

Then('it shows the page from the start, not a blank screen', async ({ world }) => {
  // once the player has found the blank start, it stands, still paused, where the page first shows something
  await expect(world.video).toHaveAttribute('data-scan', /samples/, { timeout: 30000 });
  const scan = await world.video.evaluate((v) => `${v.dataset.scan}; blank: ${v.dataset.blanks || 'none'}`);
  expect(scan, 'the scan should find the blank first second').toMatch(/blank: 0\.0-1\.\d/);
  await expect.poll(() => at(world.video), { timeout: 5000 }).toBeGreaterThan(0.9);
  expect(await world.video.evaluate((v) => v.paused)).toBe(true);
});

Given('a scenario recorded while its page went blank for a second in the middle', async ({ repo }) => {
  const tested = repo.commit('feat: split a bill between friends');
  const video = recording(repo, 'reload'); // 1.5 s of picture, 1.2 s white, 1.5 s of another picture
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

Given('a feature with two recorded scenarios', async ({ repo }) => {
  const tested = repo.commit('feat: split a bill between friends');
  const first = recording(repo, 'first');
  const second = recording(repo, 'second');
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
  // the page notes when the first scenario ended and when the next began: no polling delay in between
  await video.evaluate((v) => {
    const w = window;
    v.addEventListener('ended', () => {
      w.endedAt ??= performance.now();
    });
    v.addEventListener('loadstart', () => {
      if (w.endedAt !== undefined) w.nextAt ??= performance.now();
    });
    v.playbackRate = 2;
  });
  await expect(world.page.getByText('2 / 2 · Second scenario')).toBeAttached({ timeout: 15000 });
  const held = await world.page.evaluate(() => window.nextAt - window.endedAt);
  expect(held).toBeGreaterThanOrEqual(1000);
});
