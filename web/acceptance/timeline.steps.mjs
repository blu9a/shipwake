import { expect } from '@playwright/test';
import { Given, Then, When } from './project.mjs';

const passed = (name) => ({ feature: 'Splitting a bill', name, status: 'passed', duration: 12 });

/** The timeline's entries, top to bottom, by the text of their main link. */
const entries = (page) => page.locator('main a.lnk').allInnerTexts();

Given('a project whose newest commit came after an older one', async ({ repo, world }) => {
  world.older = repo.commit('feat: split a bill between friends');
  world.newest = repo.commit('feat: round shares to the cent');
});

Given('a test run on the older commit, recorded after the newest commit', async ({ repo, world }) => {
  repo.record(world.older, 'Acceptance', [passed('Even split')]);
});

When("I open the project's timeline", async ({ page, served }) => {
  await served.open(page);
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
});

Then('the run appears directly above the older commit', async ({ page }) => {
  const list = await entries(page);
  const run = list.indexOf('Acceptance');
  expect(run, `entries: ${list.join(' | ')}`).toBeGreaterThan(list.indexOf('feat: round shares to the cent'));
  expect(list[run + 1]).toBe('feat: split a bill between friends');
});

Then('it names the commit it tested', async ({ page, world }) => {
  // the run's entry: the nearest block around its name that also holds its details
  const run = page.getByRole('link', { name: 'Acceptance', exact: true }).locator('xpath=ancestor::div[.//code][1]');
  await expect(run.locator(`a[href$="/commit/${world.older}"]`)).toHaveText(world.older.slice(0, 7));
});

Given('a test run with one failing test on a commit', async ({ repo, world }) => {
  world.commit = repo.commit('feat: split a bill between friends');
  repo.record(world.commit, 'Acceptance', [
    passed('Even split'),
    { feature: 'Splitting a bill', name: 'Uneven split', status: 'failed', duration: 30, error: 'expected 33.34 to be 33.33' },
  ]);
});

Then("that commit's run shows one failure", async ({ page }) => {
  await expect(page.getByRole('img', { name: '1 failed', exact: true })).toBeVisible();
  await expect(page.getByRole('img', { name: '1 passed', exact: true })).toBeVisible();
});

Then('opening the run shows the failing test with its error', async ({ page }) => {
  await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  const failing = page.getByText('Uneven split', { exact: true });
  await expect(failing).toBeVisible();
  await failing.click();
  await expect(page.getByText('expected 33.34 to be 33.33')).toBeVisible();
});

Given('a release whose changelog lists a new feature and a fix', async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  repo.release('v0.1.0', { Features: ['Split a bill between friends'], Fixes: ['Shares add up to the total'] });
});

Then('the release shows the feature and the fix as its changelog says', async ({ page }) => {
  const release = page.locator('main').getByText('v0.1.0', { exact: true }).first();
  await expect(release).toBeVisible();
  await expect(page.locator('main').getByText('Split a bill between friends', { exact: true })).toBeVisible();
  await expect(page.locator('main').getByText('Shares add up to the total', { exact: true })).toBeVisible();
});
