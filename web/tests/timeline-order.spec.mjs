import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '@playwright/test';
import { inHistory } from '../src/order.ts';
import { base, files, head, project } from './fixture.mjs';

// The timeline's order: commits in history order, newest first, and above each commit the release that tags it
// and the runs that tested it, newest run first, however long after the commit they ran.

const run = (commit, at, id) => ({ commit, kind: 'run', at, id });
const commit = (sha) => ({ commit: sha, kind: 'commit', at: 0, id: sha });
const release = (sha, name) => ({ commit: sha, kind: 'release', at: 0, id: name });

test('runs sit with the commit they tested, not when they ran', () => {
  const history = ['c3', 'c2', 'c1'];
  const items = [
    commit('c1'), commit('c2'), commit('c3'), release('c2', 'v2'), release('c1', 'v1'),
    run('c3', 100, 'ci on c3'),
    run('c1', 900, 'backfilled on c1'), // recorded long after c3
    run('c2', 950, 'backfilled on c2'),
    run('c2', 960, 're-run on c2'),
    run('zz', 999, 'on a commit outside this history'),
  ];
  expect(inHistory(history, items).map((x) => x.id)).toEqual([
    'ci on c3', 'c3', 'v2', 're-run on c2', 'backfilled on c2', 'c2', 'v1', 'backfilled on c1', 'c1',
  ]);
});

test('the page puts a backfilled run with its commit', async ({ page }) => {
  const late = { ...files['/data/fixture/index.json'].runs[0], id: 'late', name: 'Backfilled benchmarks', commit: base, start: '2099-01-01T00:00:00Z' };
  await page.route('**/data/fixture/index.json', (route) => route.fulfill({ json: { ...project, runs: [...project.runs, late] } }));
  await page.goto('/#/fixture');
  await expect(page.getByText('Backfilled benchmarks').first()).toBeVisible();
  const text = await page.locator('main').innerText();
  const at = (s) => text.indexOf(s);
  expect(at('Backfilled benchmarks')).toBeGreaterThan(-1);
  // below the newer commit, above the one it tested
  expect(at('Backfilled benchmarks')).toBeGreaterThan(at(project.rows[0].title));
  expect(at('Backfilled benchmarks')).toBeLessThan(text.lastIndexOf('Initial release'));
  expect(head).not.toBe(base);
});

// Real sites, built by shipwake build: SHIPWAKE_CHECK_SITES lists their folders (make test-history builds
// Shipwake's own). Every timeline in them must keep the order above.
const sites = (process.env.SHIPWAKE_CHECK_SITES ?? '').split(',').filter(Boolean);

for (const site of sites) {
  for (const id of readdirSync(join(site, 'data'), { withFileTypes: true }).filter((d) => d.isDirectory()).map((d) => d.name)) {
    test(`the timeline of ${id} in ${site} is in history order`, () => {
      const p = JSON.parse(readFileSync(join(site, 'data', id, 'index.json'), 'utf8'));
      const rows = p.rows.map((r) => r.sha);
      const trunk = new Set(rows);
      const items = inHistory(rows, [
        ...rows.map(commit),
        ...p.tags.filter((t) => trunk.has(t.sha)).map((t) => release(t.sha, t.name)),
        ...p.runs.filter((r) => trunk.has(r.commit)).map((r) => run(r.commit, Date.parse(r.start), r.id)),
      ]);
      // Nothing on the trunk is lost.
      expect(items.filter((x) => x.kind === 'run').length).toBe(p.runs.filter((r) => trunk.has(r.commit)).length);
      for (let i = 0; i < items.length; i++) {
        const x = items[i];
        if (x.kind === 'commit') continue;
        // A release or run is followed by its own commit before any other commit.
        const next = items.slice(i + 1).find((y) => y.kind === 'commit');
        expect(next?.commit, `${x.kind} ${x.id}`).toBe(x.commit);
        // Runs of one commit: newest first.
        const after = items[i + 1];
        if (x.kind === 'run' && after?.kind === 'run' && after.commit === x.commit) expect(x.at).toBeGreaterThanOrEqual(after.at);
      }
      // Releases follow history, not version numbers: projects do release older lines from the trunk
      // (go-chi/chi tagged v1.5.1 there months after v4.1.2), and the timeline shows them where they happened.
    });
  }
}
