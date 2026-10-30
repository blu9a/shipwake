// What every scenario stands on: a real Git repository of its own, recorded runs, and the real
// shipwake binary serving it, opened in a browser as the person would.
import { execFileSync, spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { test as base, createBdd } from 'playwright-bdd';

const shipwake = resolve(import.meta.dirname, '../../bin/shipwake');

// Who approves: the repository's own Git identity, as on a person's machine.
export const me = { name: 'Pat Example', email: 'pat@example.com' };

export class Repo {
  constructor(dir) {
    this.dir = dir;
    this.day = 0;
    this.git('init', '-q', '-b', 'main');
    this.git('config', 'user.name', me.name);
    this.git('config', 'user.email', me.email);
  }

  /** Git in the repository, without the machine's own settings. Each change is a day after the last. */
  git(...args) {
    const at = new Date(Date.UTC(2026, 0, 1 + this.day)).toISOString();
    return execFileSync('git', ['-C', this.dir, ...args], {
      encoding: 'utf8',
      env: { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', GIT_AUTHOR_DATE: at, GIT_COMMITTER_DATE: at },
    }).trim();
  }

  write(path, text) {
    mkdirSync(dirname(join(this.dir, path)), { recursive: true });
    writeFileSync(join(this.dir, path), text);
  }

  read(path) {
    return readFileSync(join(this.dir, path), 'utf8');
  }

  /** A commit with these files, or a change to a notes file; its full SHA. */
  commit(subject, files = { 'notes.txt': `${subject}\n` }) {
    this.day++;
    for (const [path, text] of Object.entries(files)) this.write(path, text);
    this.git('add', '-A');
    this.git('commit', '-q', '-m', subject);
    return this.git('rev-parse', 'HEAD');
  }

  /** A release: the changelog's section for it, committed, and its annotated tag. */
  release(version, notes) {
    const sections = Object.entries(notes)
      .map(([heading, items]) => `### ${heading}\n\n${items.map((x) => `- ${x}`).join('\n')}`)
      .join('\n\n');
    const sha = this.commit(`chore(release): ${version}`, { 'CHANGELOG.md': `# Changelog\n\n## ${version} (2026-01-${String(this.day + 1).padStart(2, '0')})\n\n${sections}\n` });
    this.git('tag', '-a', version, '-m', version);
    return sha;
  }

  /** Records a run of tests on a commit, as CI would after running them. */
  record(commit, name, tests) {
    const file = join(this.dir, '..', `${name}.json`);
    writeFileSync(file, JSON.stringify({ name, kind: 'test', tests }));
    execFileSync(shipwake, ['add', '-f', 'json', '--file', file, '--commit', commit, '--repo', this.dir], {
      env: { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' },
      stdio: 'pipe',
    });
  }
}

const freePort = () =>
  new Promise((ok, fail) => {
    const s = createServer();
    s.once('error', fail);
    s.listen(0, '127.0.0.1', () => {
      const { port } = s.address();
      s.close(() => ok(port));
    });
  });

/** shipwake serve on the repository; started when the scenario first opens a page, again after outside changes. */
class Served {
  constructor(repo) {
    this.repo = repo;
  }

  async start() {
    if (this.proc) return;
    const port = await freePort();
    this.url = `http://127.0.0.1:${port}`;
    this.proc = spawn(shipwake, ['serve', this.repo.dir, '--addr', `127.0.0.1:${port}`], {
      env: { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let log = '';
    await new Promise((ok, fail) => {
      const seen = (d) => {
        log += d;
        if (log.includes('Serving on')) ok();
      };
      this.proc.stdout.on('data', seen);
      this.proc.stderr.on('data', seen);
      this.proc.once('exit', (code) => fail(new Error(`shipwake serve exited with ${code}:\n${log}`)));
    });
    const site = await (await fetch(`${this.url}/data/site.json`)).json();
    this.id = site.projects[0].id;
  }

  async stop() {
    if (!this.proc) return;
    const p = this.proc;
    this.proc = undefined;
    await new Promise((ok) => {
      p.once('exit', ok);
      p.kill();
    });
  }

  /** The repository changed outside Shipwake (a checkout, a commit): serve builds it again. */
  async restart() {
    await this.stop();
    await this.start();
  }

  async open(page, path = '') {
    await this.start();
    await page.goto(`${this.url}/#/${this.id}${path}`);
  }
}

export const test = base.extend({
  repo: async ({}, use) => {
    const root = mkdtempSync(join(tmpdir(), 'shipwake-acceptance-'));
    const dir = join(root, 'bills');
    mkdirSync(dir);
    await use(new Repo(dir));
    rmSync(root, { recursive: true, force: true });
  },
  served: async ({ repo }, use) => {
    const s = new Served(repo);
    await use(s);
    await s.stop();
  },
  page: async ({ page }, use) => {
    await page.addInitScript(() => {
      localStorage.setItem('lang', 'en');
      localStorage.setItem('mode', 'light');
    });
    await use(page);
  },
  // What a scenario remembers between its steps: commits by role, names it chose.
  world: async ({}, use) => {
    await use({});
  },
});

export const { Given, When, Then } = createBdd(test);
