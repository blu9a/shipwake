import { test, expect } from '@playwright/test';
import { project, head } from './fixture.mjs';

const scenarioText = '  Scenario: Exact totals\n    Given a bill of 100.00\n    Then the shares add up\n';
const metricText = '@eval\nMetric: Exact totals\n  Higher is better\n  Blocks a release below 100%';
const metricLines = metricText.split('\n').map((text, i) => ({ n: i + 1, text, kind: ['tag', 'title', 'direction', 'block'][i] }));

// Model the writable workspace, including delayed rebuild reads and rejected duplicate approval.
async function workspace(page, kind, withOther = false) {
  let approved = false;
  let posts = 0;
  let fail = false;
  let releaseRefresh;
  let refreshGate = Promise.resolve();
  const snapshot = structuredClone(project);
  const commit = 'd'.repeat(40);
  await page.addInitScript(() => { localStorage.setItem('lang', 'en'); localStorage.setItem('mode', 'light'); });
  await page.route('**/api/**', async (route) => {
    const { pathname } = new URL(route.request().url());
    const action = pathname.split('/').at(-1);
    if (action === 'session') return route.fulfill({ json: { edit: true, user: 'Fixture reviewer', checkedOut: { fixture: 'main' } } });
    if (action === 'proposal' || action === 'metric') return route.fulfill({ json: { text: action === 'proposal' ? scenarioText : withOther ? metricText.replace('Exact totals', 'Unrelated requirement') : metricText } });
    if (action === 'check-metric') return route.fulfill({ json: { name: 'Exact totals', lines: metricLines } });
    if (action === 'approve' || action === 'approve-metric') {
      posts++;
      if (fail || approved) return route.fulfill({ status: 400, json: { error: fail ? 'Please try again' : 'Exact totals is not proposed' } });
      approved = true;
      snapshot.rows.unshift({ sha: commit, merge: false, title: 'Approve exact totals', kind: 'spec' });
      snapshot.commits[commit] = { ...snapshot.commits[head], sha: commit, short: 'ddddddd', parents: [head], subject: 'Approve exact totals', date: '2026-10-29T15:00:00Z' };
      return route.fulfill({ json: { commit } });
    }
    return route.fulfill({ status: 404 });
  });
  await page.route('**/data/fixture/*.json', async (route) => {
    const name = new URL(route.request().url()).pathname.split('/').at(-1);
    if (approved) await refreshGate;
    if (name === 'index.json') return route.fulfill({ json: snapshot });
    const state = approved ? 'approved' : 'proposed';
    if (name === 'stories.json') return route.fulfill({ json: kind === 'scenario' ? [{ path: 'specs/bill.feature', name: 'Splitting a bill', scenarios: [{ title: 'Exact totals', state, steps: ['Given a bill of 100.00', 'Then the shares add up'], events: [] }] }] : [] });
    if (name === 'metric-stories.json') return route.fulfill({ json: kind === 'metric' || withOther ? [{ id: 'totals.exact', path: 'metrics/totals.exact.metric', name: withOther ? 'Unrelated requirement' : 'Exact totals', state: withOther ? 'proposed' : state, definition: metricText.split('\n'), lines: metricLines, events: [] }] : [] });
    return route.continue();
  });
  await page.goto('/#/fixture');
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
  const proposal = page.locator('details').filter({ has: page.locator('textarea') }).filter({ has: page.locator('summary').filter({ hasText: kind === 'scenario' ? 'Splitting a bill' : 'Exact totals' }) });
  await proposal.locator(':scope > summary').click();
  const button = proposal.getByRole('button', { name: 'Approve', exact: true });
  await expect(button).toBeEnabled();
  return {
    proposal, button, posts: () => posts, fail: (value) => { fail = value; },
    pauseRefresh: () => { refreshGate = new Promise((resolve) => { releaseRefresh = resolve; }); },
    resumeRefresh: () => releaseRefresh?.(),
  };
}

for (const kind of ['scenario', 'metric']) {
  test(`${kind}: approval refreshes the production timeline without reloading`, async ({ page }) => {
    const w = await workspace(page, kind);
    await w.button.click();
    await expect(w.proposal).toHaveCount(0);
    await expect(page.getByText('Approve exact totals', { exact: true })).toBeVisible();
    expect(w.posts()).toBe(1);
    await page.reload();
    await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Approve', exact: true })).toHaveCount(0);
  });

  test(`${kind}: completed approval cannot be submitted again while data refreshes`, async ({ page }) => {
    const w = await workspace(page, kind);
    w.pauseRefresh();
    try {
      await w.button.dblclick();
      await expect.poll(w.posts).toBe(1);
      await expect(page.getByRole('button', { name: 'Approve', exact: true })).toHaveCount(0);
    } finally {
      w.resumeRefresh();
    }
    await expect(w.proposal).toHaveCount(0);
    await expect(page.getByText('Approve exact totals', { exact: true })).toBeVisible();
  });

  test(`${kind}: rejected approval keeps the proposal available for retry`, async ({ page }) => {
    const w = await workspace(page, kind);
    w.fail(true);
    await w.button.click();
    await expect(w.proposal.getByText('Please try again', { exact: true })).toBeVisible();
    await expect(w.button).toBeEnabled();
    w.fail(false);
    await w.button.click();
    await expect(w.proposal).toHaveCount(0);
    expect(w.posts()).toBe(2);
  });
}

test("approving one proposal preserves another proposal's open editor and unsaved text", async ({ page }) => {
  const w = await workspace(page, 'scenario', true);
  const other = page.locator('details').filter({ has: page.locator('summary', { hasText: 'Unrelated requirement' }) });
  await other.locator(':scope > summary').click();
  const draft = 'Metric: Unrelated requirement\n  My unsaved description';
  await other.getByRole('textbox').fill(draft);
  await w.button.click();
  await expect(page.getByText('Approve exact totals', { exact: true })).toBeVisible();
  await expect(other).toHaveAttribute('open', '');
  await expect(other.getByRole('textbox')).toHaveValue(draft);
});
