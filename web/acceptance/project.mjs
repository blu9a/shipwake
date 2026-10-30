// What every scenario stands on: a real Git repository of its own, recorded runs, and the real
// shipwake binary serving it, opened in a browser as the person would.
import { execFileSync, spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { test as base, createBdd } from 'playwright-bdd';
import { showPointer, slowClicks } from '../../packages/playwright/src/index.ts';


// Who approves: the repository's own Git identity, as on a person's machine.
export const me = { name: 'Pat Example', email: 'pat@example.com' };

/** The environment every shipwake the tests start runs in: no machine settings, a home of its own. */
const env = (home) => ({ ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', SHIPWAKE_HOME: home });

/** Runs the shipwake binary, as a person would in a terminal. */
export const shipwake = (home, ...args) => execFileSync(shipwake_, args, { env: env(home), encoding: 'utf8', stdio: 'pipe' });
const shipwake_ = resolve(import.meta.dirname, '../../bin/shipwake');

export class Repo {
  constructor(dir, home) {
    this.dir = dir;
    this.home = home;
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

  /** Records a run of tests on a commit, as CI would after running them. Attachments are files next to
   *  the report, written here with what they're named. */
  record(commit, name, tests, files = {}) {
    const dir = join(this.dir, '..', 'report');
    mkdirSync(dir, { recursive: true });
    for (const [path, text] of Object.entries(files)) writeFileSync(join(dir, path), text);
    const file = join(dir, `${name}.json`);
    writeFileSync(file, JSON.stringify({ name, kind: 'test', tests }));
    shipwake(this.home, 'add', '-f', 'json', '--file', file, '--commit', commit, '--repo', this.dir);
  }

  /** What a careful person would compare to see nothing was changed: refs, settings, files, status. */
  snapshot() {
    const files = [];
    const walk = (d) => {
      for (const e of readdirSync(join(this.dir, d), { withFileTypes: true })) {
        if (d === '' && e.name === '.git') continue;
        const rel = join(d, e.name);
        if (e.isDirectory()) walk(rel);
        else files.push(`${rel} ${readFileSync(join(this.dir, rel), 'utf8').length}`);
      }
    };
    walk('');
    const inGit = (path) => {
      try {
        return readdirSync(join(this.dir, '.git', path)).sort().join(',');
      } catch {
        return '';
      }
    };
    return {
      refs: this.git('for-each-ref', '--format=%(refname) %(objectname)'),
      config: readFileSync(join(this.dir, '.git', 'config'), 'utf8'),
      exclude: readFileSync(join(this.dir, '.git', 'info', 'exclude'), 'utf8'),
      hooks: inGit('hooks'),
      worktrees: inGit('worktrees'),
      status: this.git('status', '--porcelain', '--ignored'),
      files: files.sort(),
    };
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

/** shipwake serve on the repository, or, named nothing, on every project opened before; started when
 *  the scenario first opens a page, again after outside changes. */
class Served {
  constructor(repo, home) {
    this.repo = repo;
    this.home = home;
    this.alone = false; // serve without naming a repository
  }

  async start() {
    if (this.proc) return;
    const port = await freePort();
    this.url = `http://127.0.0.1:${port}`;
    const repo = this.alone ? [] : [this.repo.dir];
    this.proc = spawn(shipwake_, ['serve', ...repo, '--addr', `127.0.0.1:${port}`], {
      env: env(this.home),
      cwd: this.home, // not in a repository: serving nothing named means the projects opened before
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
    this.id = site.projects[0]?.id;
  }

  /** Opens a repository by its path, as the page does; its project ID. */
  async add(repo) {
    await this.start();
    const r = await fetch(`${this.url}/api/open`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Origin: this.url },
      body: JSON.stringify({ path: repo.dir }),
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    this.id = d.project;
    return d.project;
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
  // Everything a scenario makes lives in one folder: its repositories, and Shipwake's home.
  root: async ({}, use) => {
    const root = mkdtempSync(join(tmpdir(), 'shipwake-acceptance-'));
    await use(root);
    rmSync(root, { recursive: true, force: true });
  },
  home: async ({ root }, use) => {
    const home = join(root, 'home');
    mkdirSync(home);
    await use(home);
  },
  /** Another repository of the scenario's, by its folder name. */
  another: async ({ root, home }, use) => {
    await use((name) => {
      const dir = join(root, name);
      mkdirSync(dir);
      return new Repo(dir, home);
    });
  },
  repo: async ({ another }, use) => {
    await use(another('bills'));
  },
  served: async ({ repo, home }, use) => {
    const s = new Served(repo, home);
    await use(s);
    await s.stop();
  },
  // Recorded as a demo of the feature: a pointer that reaches what it clicks, and the result left on
  // screen a moment before the recording ends.
  page: async ({ page }, use) => {
    await page.addInitScript(() => {
      localStorage.setItem('lang', 'en');
      localStorage.setItem('mode', 'light');
    });
    await showPointer(page);
    slowClicks(page, 250);
    await use(page);
    if (page.url().startsWith('http')) await page.waitForTimeout(800);
  },
  // What a scenario remembers between its steps: commits by role, names it chose.
  world: async ({}, use) => {
    await use({});
  },
});

export const { Given, When, Then } = createBdd(test);
