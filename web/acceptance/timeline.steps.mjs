import { expect } from '@playwright/test';
import { Given, prove, Then, When } from './project.mjs';

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

Then('the run appears directly above the older commit', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const list = await entries(page);
  const run = list.indexOf('Acceptance');
  expect(run, `entries: ${list.join(' | ')}`).toBeGreaterThan(list.indexOf('feat: round shares to the cent'));
  expect(list[run + 1]).toBe('feat: split a bill between friends');
  const both = page.locator('main a.lnk').filter({ hasText: /^(Acceptance|feat: split a bill between friends)$/ });
  await prove(f, both.first().locator('xpath=ancestor::div[.//code][1]/..'), ['Acceptance', 'feat: split a bill between friends']);
});

Then('it names the commit it tested', async ({ page, world, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  // the run's entry: the nearest block around its name that also holds its details
  const run = page.getByRole('link', { name: 'Acceptance', exact: true }).locator('xpath=ancestor::div[.//code][1]');
  const tested = run.locator(`a[href$="/commit/${world.older}"]`);
  await expect(tested).toHaveText(world.older.slice(0, 7));
  await prove(f, tested, [world.older.slice(0, 7)]);
});

Given('a test run with one failing test on a commit', async ({ repo, world }) => {
  world.commit = repo.commit('feat: split a bill between friends');
  repo.record(world.commit, 'Acceptance', [
    passed('Even split'),
    { feature: 'Splitting a bill', name: 'Uneven split', status: 'failed', duration: 30, error: 'expected 33.34 to be 33.33' },
  ]);
});

Then("that commit's run shows one failure", async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  await expect(page.getByRole('img', { name: '1 failed', exact: true })).toBeVisible();
  await expect(page.getByRole('img', { name: '1 passed', exact: true })).toBeVisible();
  await prove(f, page.getByRole('img', { name: '1 failed', exact: true }), ['1']);
});

Then('opening the run shows the failing test with its error', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  const failing = page.getByText('Uneven split', { exact: true });
  await expect(failing).toBeVisible();
  await failing.click();
  await expect(page.getByText('expected 33.34 to be 33.33')).toBeVisible();
  await prove(f, page.getByText('expected 33.34 to be 33.33'), ['expected 33.34 to be 33.33']);
});

Given('a release whose changelog lists a new feature and a fix', async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  // written the way changelog tools write them: links, bold, code
  repo.release('v0.1.0', {
    Features: ['Split a bill between friends'],
    Fixes: ['**totals:** Shares add up to the `total` ([#12](https://example.com/bills/pull/12))'],
  });
});

Then('the release shows the feature and the fix as its changelog says', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const release = page.locator('main').getByText('v0.1.0', { exact: true }).first();
  await expect(release).toBeVisible();
  await expect(page.locator('main').getByText('Split a bill between friends', { exact: true })).toBeVisible();
  const fix = page.locator('main li').filter({ hasText: 'Shares add up to the' });
  await expect(fix).toHaveText('totals: Shares add up to the total (#12)');
  await expect(fix.getByRole('link', { name: '#12' })).toHaveAttribute('href', 'https://example.com/bills/pull/12');
  await prove(f, fix.locator('xpath=ancestor::*[.//h4][1]'), ['Split a bill between friends', 'totals: Shares add up to the total (#12)']);
});

// Results without evidence

const step = (name) => ({ name, status: 'passed', duration: 5 });

Given('a run in which some scenarios recorded what their result steps saw', async ({ world }) => {
  world.tests = [
    {
      feature: 'Splitting a bill',
      name: 'Even split',
      status: 'passed',
      duration: 12,
      steps: [step('Given a bill of 90.00'), step('Then each person pays 30.00')],
      evidence: [{ step: 'each person pays 30.00', expected: ['30.00'], observed: 'Each person pays 30.00' }],
    },
  ];
});

Given('one scenario whose result step recorded nothing', async ({ repo, world }) => {
  world.tests.push({
    feature: 'Splitting a bill',
    name: 'Uneven split',
    status: 'passed',
    duration: 12,
    steps: [step('Given a bill of 100.00'), step('Then the shares add up to 100.00')],
  });
  repo.record(repo.commit('feat: split a bill between friends'), 'Acceptance', world.tests);
});

When('I open that scenario in the run', async ({ page, served }) => {
  await served.open(page);
  await page.getByRole('link', { name: 'Acceptance', exact: true }).click();
  await page.getByText('Uneven split', { exact: true }).click();
});

Then('its result says no evidence was recorded', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const none = page.getByText('No evidence recorded', { exact: true });
  await expect(none).toHaveCount(1);
  await expect(none).toBeVisible();
  await prove(f, none, ['No evidence recorded']);
});

Then('the scenarios with evidence show what was observed', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  await page.getByText('Even split', { exact: true }).click();
  const observed = page.getByText('Observed', { exact: true }).locator('..');
  await expect(observed).toContainText('Each person pays 30.00');
  await prove(f, observed, ['30.00']);
});
