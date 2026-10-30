import { expect } from '@playwright/test';
import { Given, me, Then, When } from './project.mjs';

const notes = (page) => page.getByRole('textbox').last();

Given('commits since the last release: a feature, a fix and a test', async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  repo.release('v0.1.0', { Features: ['Split a bill between friends'] });
  repo.commit('feat: add a tip before splitting');
  repo.commit('fix: shares add up to the total');
  repo.commit('test: cover a bill of zero');
});

When('I prepare a new release and assemble the draft', async ({ page, served }) => {
  await served.open(page, '/release');
  await expect(page.getByRole('heading', { name: 'New release', exact: true })).toBeVisible();
  await page.getByRole('button', { name: /Assemble draft/ }).click();
});

Then('the notes list the feature and the fix', async ({ page }) => {
  await expect(notes(page)).toHaveValue(/add a tip before splitting/i);
  await expect(notes(page)).toHaveValue(/shares add up to the total/i);
});

Then("they don't list the test", async ({ page }) => {
  await expect(notes(page)).not.toHaveValue(/bill of zero/i);
});

Given('checks that pass on the newest commit', async ({ repo }) => {
  const head = repo.commit('feat: split a bill between friends');
  repo.record(head, 'Acceptance', [{ feature: 'Splitting a bill', name: 'Even split', status: 'passed', duration: 12 }]);
});

When('I approve a release with its notes', async ({ page, served }) => {
  await served.open(page, '/release');
  await expect(page.getByRole('heading', { name: 'New release', exact: true })).toBeVisible();
  await notes(page).fill('### Features\n\n- Split a bill between friends');
  await page.getByRole('button', { name: 'Approve release', exact: true }).click();
  await expect(page.getByText(/^Tagged v0\.1\.0 on [0-9a-f]{7}\./)).toBeVisible();
});

Then('the changelog has a section for the new version', async ({ repo }) => {
  const changelog = repo.git('show', 'HEAD:CHANGELOG.md');
  expect(changelog).toMatch(/^## v0\.1\.0\b/m);
  expect(changelog).toContain('- Split a bill between friends');
});

Then('the version is tagged with me as the approver', async ({ repo }) => {
  const tag = repo.git('cat-file', 'tag', 'v0.1.0');
  expect(tag).toContain(`tagger ${me.name} <${me.email}>`);
  expect(tag).toContain(`Approved-by: ${me.name} <${me.email}>`);
});

Given('every change is already released', async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  repo.release('v0.1.0', { Features: ['Split a bill between friends'] });
});

When('I open the new release page', async ({ page, served }) => {
  await served.open(page, '/release');
});

Then('it says there is nothing new since the last release', async ({ page }) => {
  await expect(page.getByText('Nothing new since v0.1.0.', { exact: true })).toBeVisible();
});
