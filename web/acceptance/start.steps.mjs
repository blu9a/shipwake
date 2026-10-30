import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect } from '@playwright/test';
import { Then, When, shipwake } from './project.mjs';

/** Runs a shipwake command line on the scenario's repository, as typed in it. */
const run = (home, repo, line) => shipwake(home, ...line.replace(/^shipwake\s+/, '').split(/\s+/), repo.dir);

/** The workspace projects.yaml names for this repository, if any. */
const workspaceOf = (home, repo) => {
  const list = existsSync(join(home, 'projects.yaml')) ? readFileSync(join(home, 'projects.yaml'), 'utf8') : '';
  return list.match(new RegExp(`repo: ${repo.dir}\\s+workspace: (\\S+)`))?.[1];
};

When('I run {string} in it', async ({ home, repo, world }, line) => {
  world.output = run(home, repo, line);
});

When('I run {string} in its repository', async ({ home, repo, world }, line) => {
  world.output = run(home, repo, line);
});

Then('it is in local mode, its settings written and committed in its workspace', async ({ home, repo, world }) => {
  const ws = workspaceOf(home, repo);
  expect(ws, 'listed in local mode').toBeTruthy();
  expect(readFileSync(join(ws, '.shipwake.yaml'), 'utf8')).toContain('name: bills');
  const git = (...a) => repo.git('-C', ws, ...a);
  expect(git('status', '--porcelain')).toBe('');
  expect(git('log', '-1', '--format=%s')).toBe("chore: add Shipwake's settings");
  expect(world.output).toContain('Local mode: nothing is written into this repository.');
});

Then('the repository is as it was', async ({ repo, world }) => {
  expect(repo.snapshot()).toEqual(world.before);
});

Then('its settings are written in its workspace, not in the repository', async ({ repo, world }) => {
  expect(existsSync(join(repo.dir, '.shipwake.yaml'))).toBe(false);
  expect(repo.git('status', '--porcelain', '--ignored')).toBe('');
  expect(world.ws.git('log', '-1', '--name-only', '--format=%s')).toBe("chore: add Shipwake's settings\n\n.shipwake.yaml");
});
