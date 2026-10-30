import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { expect } from '@playwright/test';
import { Given, me, shipwake, Then, When } from './project.mjs';

const feature = (state) => `Feature: Splitting a bill

  ${state === 'proposed' ? '@proposed\n  ' : ''}Scenario: Exact totals
    Given a bill of 100.00
    When we split it between 3 people
    Then the shares add up to 100.00
`;

/** A project's workspace, where its requirements are written and committed as an agent or person would. */
class Workspace {
  constructor(dir) {
    this.dir = dir;
  }

  git(...args) {
    return execFileSync('git', ['-C', this.dir, '-c', `user.name=${me.name}`, '-c', `user.email=${me.email}`, ...args], {
      encoding: 'utf8',
      env: { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' },
    }).trim();
  }

  commit(message, files) {
    for (const [path, text] of Object.entries(files)) {
      mkdirSync(dirname(join(this.dir, path)), { recursive: true });
      writeFileSync(join(this.dir, path), text);
    }
    this.git('add', '-A');
    this.git('commit', '-q', '-m', message);
    return this.git('rev-parse', 'HEAD');
  }
}

/** The proposal of a feature on the page, by its name. */
const proposal = (page, name) => page.locator('main details').filter({ has: page.locator('summary', { hasText: name }) });

/** Opens a repository in local mode, as the page does, and returns its workspace. */
async function inLocalMode(served, home, repo) {
  served.alone = true;
  await served.add(repo);
  const entries = readFileSync(join(home, 'projects.yaml'), 'utf8');
  const dir = entries.match(new RegExp(`repo: ${repo.dir}\\s+workspace: (\\S+)`))?.[1];
  expect(dir, entries).toBeTruthy();
  return new Workspace(dir);
}

// Opening a repository by its path

Given("a repository on my machine that Shipwake has never seen", async ({ repo, world }) => {
  repo.commit('feat: split a bill between friends');
  repo.release('v0.1.0', { Features: ['Split a bill between friends'] });
  repo.commit('fix: shares add up to the total');
  repo.git('branch', 'experiment');
  world.before = repo.snapshot();
});

When('I open it in Shipwake by its path', async ({ page, served, repo }) => {
  served.alone = true;
  await served.start();
  await page.goto(served.url);
  await expect(page.getByRole('heading', { name: 'Open a repository', exact: true })).toBeVisible();
  await page.getByRole('textbox', { name: 'Repository path' }).fill(repo.dir);
  await page.getByRole('button', { name: 'Open', exact: true }).click();
});

Then('its timeline is shown', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'fix: shares add up to the total' })).toBeVisible();
});

Then('its files, branches, tags and Git settings are as they were', async ({ repo, world }) => {
  expect(repo.snapshot()).toEqual(world.before);
});

// Approving commits to the workspace

Given('scenarios proposed for a project in local mode', async ({ repo, served, home, world }) => {
  repo.commit('feat: split a bill between friends');
  world.ws = await inLocalMode(served, home, repo);
  world.ws.commit('spec: propose exact totals', { 'features/bill.feature': feature('proposed') });
  world.commits = repo.git('rev-list', '--all', '--count');
  await served.restart();
});

Then("the approval is a commit in the project's workspace that names me", async ({ world }) => {
  expect(world.ws.git('log', '-1', '--format=%B')).toContain(`Approved-by: ${me.name} <${me.email}>`);
  expect(world.ws.git('log', '-1', '--format=%an <%ae>')).toBe(`${me.name} <${me.email}>`);
  expect(world.ws.git('show', 'HEAD:features/bill.feature')).not.toContain('@proposed');
});

Then('the repository has no new commits', async ({ repo, world }) => {
  expect(repo.git('rev-list', '--all', '--count')).toBe(world.commits);
  expect(repo.git('status', '--porcelain')).toBe('');
});

// Runs are kept in the workspace

Given('a project in local mode', async ({ repo, served, home, world }) => {
  world.tested = repo.commit('feat: split a bill between friends');
  world.ws = await inLocalMode(served, home, repo);
  world.branches = repo.git('branch', '--list');
});

When('I record a run of its tests, with a recording', async ({ repo, world }) => {
  const video = { name: 'Recording', path: 'exact-totals.webm', type: 'video/webm' };
  repo.record(world.tested, 'Acceptance', [{ feature: 'Splitting a bill', name: 'Exact totals', status: 'passed', duration: 12, attachments: [video] }], {
    'exact-totals.webm': 'a recording',
  });
});

Then("the run and its recording are kept in the project's workspace", async ({ world }) => {
  const kept = world.ws.git('ls-tree', '-r', '--name-only', 'shipwake-results');
  expect(kept).toMatch(/^runs\/\S+\.json$/m);
  expect(kept).toMatch(/^files\/\S+\/exact-totals\.webm$/m);
});

Then('the repository has no new branch', async ({ repo, world }) => {
  expect(repo.git('branch', '--list')).toBe(world.branches);
});

Then('the run is shown on the repository commit it tested', async ({ page, served, world }) => {
  await served.restart();
  await served.open(page);
  const run = page.getByRole('link', { name: 'Acceptance', exact: true }).locator('xpath=ancestor::div[.//code][1]');
  await expect(run.locator(`a[href$="/commit/${world.tested}"]`)).toHaveText(world.tested.slice(0, 7));
});

// Requirements follow the repository's branch

Given('a project in local mode with scenarios proposed for the branch feature\\/tip', async ({ repo, served, home, world }) => {
  repo.commit('feat: split a bill between friends');
  repo.git('switch', '-q', '-c', 'feature/tip');
  repo.commit('feat: add a tip before splitting');
  repo.git('switch', '-q', 'main');
  world.ws = await inLocalMode(served, home, repo);
  world.ws.git('switch', '-q', '-c', 'feature/tip');
  world.ws.commit('spec: propose exact totals', { 'features/bill.feature': feature('proposed') });
  world.ws.git('switch', '-q', 'main');
});

When('the repository has feature\\/tip checked out', async ({ repo, served }) => {
  repo.git('switch', '-q', 'feature/tip');
  await served.restart();
});

Then('the timeline of feature\\/tip shows the proposal, and it can be approved', async ({ page, served, world }) => {
  await served.open(page, '/branch/feature/tip');
  await expect(page.getByRole('heading', { name: 'feature/tip', exact: true })).toBeVisible();
  const p = proposal(page, 'Splitting a bill');
  await p.locator(':scope > summary').click();
  await p.getByRole('button', { name: 'Approve', exact: true }).click();
  await expect(p).toBeHidden();
  // committed on the workspace's branch of the same name
  expect(world.ws.git('log', '-1', '--format=%s', 'feature/tip')).toMatch(/^spec: approve/);
  expect(world.ws.git('log', '-1', '--format=%s', 'main')).not.toMatch(/^spec: approve/);
});

Then("the trunk's timeline shows nothing proposed", async ({ page, served }) => {
  await served.open(page);
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
  await expect(page.locator('main a.lnk').first()).toBeVisible();
  await expect(proposal(page, 'Splitting a bill')).toHaveCount(0);
});

// Opening every project at once

Given('two projects in local mode and one repository set up the usual way', async ({ repo, another, served }) => {
  served.alone = true;
  const notes = another('notes');
  const shop = another('shop');
  repo.commit('feat: split a bill between friends');
  notes.commit('feat: write a note');
  shop.commit('feat: sell a thing', { '.shipwake.yaml': 'name: Shop\n' });
  for (const r of [repo, notes, shop]) await served.add(r);
  await served.stop();
});

When('I start Shipwake without naming a repository', async ({ page, served }) => {
  served.alone = true;
  await served.start();
  await page.goto(served.url);
  await expect(page.getByRole('heading', { name: 'main', exact: true })).toBeVisible();
});

Then('all three projects are shown', async ({ page }) => {
  await page.getByRole('button', { name: 'Projects' }).click();
  for (const name of ['bills', 'notes', 'Shop']) await expect(page.getByRole('link', { name, exact: true })).toBeVisible();
});

// Giving an agent what it needs

When('I open the agent prompt', async ({ page, served }) => {
  await served.restart();
  await served.open(page);
  await page.getByRole('button', { name: 'Prompt for an agent' }).click();
});

Then('it names the skill, the repository, its workspace and the checked-out branch', async ({ page, repo, home, world }) => {
  const text = await page.locator('.prompt pre').innerText();
  const skill = join(home, 'skill', 'SKILL.md');
  expect(text).toContain(skill);
  expect(existsSync(skill) && readFileSync(skill, 'utf8')).toMatch(/^---|^# /);
  expect(text).toContain(repo.dir);
  expect(text).toContain(world.ws.dir);
  expect(text).toContain('branch main');
  world.prompt = text;
});

Then('I can copy it with one click', async ({ page, context, world }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.getByRole('button', { name: 'Copy', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Copied', exact: true })).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(world.prompt);
});

// Adopting the requirements into the repository

Given('a project in local mode with approved scenarios and recorded runs', async ({ repo, served, home, world }) => {
  world.tested = repo.commit('feat: split a bill between friends');
  world.ws = await inLocalMode(served, home, repo);
  world.ws.commit('spec: propose exact totals', { 'features/bill.feature': feature('proposed'), '.shipwake.yaml': 'name: Bills\n' });
  world.ws.commit(`spec: approve 1 scenario of "Splitting a bill"\n\nApproved-by: ${me.name} <${me.email}>`, {
    'features/bill.feature': feature('approved'),
  });
  repo.record(world.tested, 'Acceptance', [{ feature: 'Splitting a bill', name: 'Exact totals', status: 'passed', duration: 12 }]);
  world.commits = Number(repo.git('rev-list', '--count', 'HEAD'));
  world.kept = world.ws.git('for-each-ref', '--format=%(refname) %(objectname)');
});

When('I adopt it into the repository', async ({ repo, home }) => {
  shipwake(home, 'adopt', repo.dir);
});

Then('the repository has one new commit, made as me, with the requirements and settings', async ({ repo, world }) => {
  expect(Number(repo.git('rev-list', '--count', 'HEAD'))).toBe(world.commits + 1);
  expect(repo.git('log', '-1', '--format=%an <%ae>')).toBe(`${me.name} <${me.email}>`);
  expect(repo.git('show', '--name-only', '--format=', 'HEAD').split('\n').sort()).toEqual(['.shipwake.yaml', 'features/bill.feature']);
  expect(repo.git('status', '--porcelain')).toBe('');
});

Then('its message lists what was approved in local mode, and by whom', async ({ repo }) => {
  expect(repo.git('log', '-1', '--format=%B')).toContain(`approve 1 scenario of "Splitting a bill", by ${me.name} <${me.email}>`);
});

Then('the workspace is kept as it was', async ({ world }) => {
  expect(world.ws.git('for-each-ref', '--format=%(refname) %(objectname)')).toBe(world.kept);
  expect(world.ws.git('status', '--porcelain')).toBe('');
});
