import { expect } from '@playwright/test';
import { Given, me, prove, Then, When } from './project.mjs';

const feature = `Feature: Splitting a bill

  @proposed
  Scenario: Exact totals
    Given a bill of 100.00
    When we split it between 3 people
    Then the shares add up to 100.00
`;

/** The proposal of a feature or metric on the page, by its name. */
const proposal = (page, name) => page.locator('main details').filter({ has: page.locator('summary', { hasText: name }) });

Given('scenarios proposed for a feature', async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  repo.commit('spec: propose exact totals', { 'features/bill.feature': feature });
});

Then('the proposal is shown above everything else', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const first = proposal(page, 'Splitting a bill');
  await expect(first).toBeVisible();
  const top = (await first.boundingBox()).y;
  for (const link of await page.locator('main a.lnk').all()) {
    expect((await link.boundingBox()).y).toBeGreaterThan(top);
  }
  await prove(f, first, ['Splitting a bill']);
});

Then('it can be opened to read its scenarios', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const p = proposal(page, 'Splitting a bill');
  await p.locator(':scope > summary').click();
  await expect(p.getByRole('textbox')).toHaveValue(/Scenario: Exact totals[\s\S]*Then the shares add up to 100\.00/);
  await prove(f, p.getByRole('textbox'), ['Scenario: Exact totals']);
});

When('I approve them in Shipwake', async ({ page, served }) => {
  await served.open(page);
  const p = proposal(page, 'Splitting a bill');
  await p.locator(':scope > summary').click();
  await p.getByRole('button', { name: 'Approve', exact: true }).click();
  await expect(p).toBeHidden();
});

Then('they are no longer proposed', async ({ page, repo }) => {
  expect(repo.read('features/bill.feature')).not.toContain('@proposed');
  expect(repo.git('status', '--porcelain')).toBe('');
  await page.reload();
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
  await expect(proposal(page, 'Splitting a bill')).toHaveCount(0);
});

Then('the commit that approved them names me as the approver', async ({ repo }) => {
  expect(repo.git('log', '-1', '--format=%B')).toContain(`Approved-by: ${me.name} <${me.email}>`);
  expect(repo.git('log', '-1', '--format=%an <%ae>')).toBe(`${me.name} <${me.email}>`);
});

Given("a proposed metric with a line Shipwake doesn't understand", async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  repo.commit('spec: propose exact totals', {
    'metrics/totals.exact.metric': '@proposed\nMetric: Exact totals\n  Higher is better\n  Blocks a release when it feels wrong\n',
  });
});

When('I open the proposal', async ({ page, served }) => {
  await served.open(page);
  await proposal(page, 'Exact totals').locator(':scope > summary').click();
});

Then('that line is marked with the reason', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const p = proposal(page, 'Exact totals');
  // numbered as the editor shows the text
  const text = await p.getByRole('textbox').inputValue();
  const n = text.split('\n').findIndex((l) => l.includes('when it feels wrong')) + 1;
  await expect(p.getByRole('status')).toHaveText(new RegExp(`^Line ${n}: \\S`));
  await prove(f, p.getByRole('status'), [`Line ${n}:`]);
});

Then('approving it is not possible', async ({ page, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const approve = proposal(page, 'Exact totals').getByRole('button', { name: 'Approve', exact: true });
  await expect(approve).toBeDisabled();
  await prove(f, approve, ['Approve']);
});

Given('scenarios proposed on a branch that is not merged', async ({ repo }) => {
  repo.commit('feat: split a bill between friends');
  repo.git('switch', '-q', '-c', 'spec/exact-totals');
  repo.commit('spec: propose exact totals', { 'features/bill.feature': feature });
});

When('I open the timeline of the trunk', async ({ page, served }) => {
  await served.open(page);
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
});

Then('nothing is shown as proposed', async ({ page }) => {
  await expect(page.locator('main a.lnk').first()).toBeVisible();
  await expect(proposal(page, 'Splitting a bill')).toHaveCount(0);
});

Then("the branch's timeline shows the proposal", async ({ page, served, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  await served.open(page, '/branch/spec/exact-totals');
  await expect(page.getByRole('heading', { name: 'spec/exact-totals', exact: true })).toBeVisible();
  await expect(proposal(page, 'Splitting a bill')).toBeVisible();
  await prove(f, proposal(page, 'Splitting a bill'), ['Splitting a bill']);
});

Then('it can be approved only while that branch is checked out', async ({ page, repo, served, $testInfo, $step }) => {
  const f = { page, $testInfo, $step };
  const approve = () => proposal(page, 'Splitting a bill').getByRole('button', { name: 'Approve', exact: true });
  await proposal(page, 'Splitting a bill').locator(':scope > summary').click();
  await expect(approve()).toBeEnabled();

  repo.git('switch', '-q', 'main');
  await served.restart();
  await served.open(page, '/branch/spec/exact-totals');
  await proposal(page, 'Splitting a bill').locator(':scope > summary').click();
  const note = proposal(page, 'Splitting a bill').getByText('Check out spec/exact-totals');
  await expect(note).toBeVisible();
  await expect(approve()).toHaveCount(0);
  await prove(f, note, ['Check out spec/exact-totals']);
});
