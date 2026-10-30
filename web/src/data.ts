// Loads JSON once each and shares its promise. Workspace rebuilds reuse URLs, so a forgotten file
// must come from the server rather than the browser's HTTP cache.
import type { CommitDetail, FileDiff, Project, Run, Site } from './model.gen';

const cache = new Map<string, Promise<unknown>>();

function load<T>(path: string): Promise<T> {
  let p = cache.get(path) as Promise<T> | undefined;
  if (!p) {
    p = fetch(`data/${path}`, { cache: 'no-store' }).then((r) => {
      if (!r.ok) throw new Error(`${path}: ${r.status}`);
      return r.json() as Promise<T>;
    });
    cache.set(path, p);
  }
  return p;
}

export const loadSite = () => load<Site>('site.json');
export const loadProject = (id: string) => load<Project>(`${id}/index.json`);
export const loadCommit = (id: string, sha: string) => load<CommitDetail>(`${id}/commits/${sha}.json`);
export const loadDiffs = (id: string, sha: string) => load<Record<string, FileDiff>>(`${id}/diffs/${sha}.json`);
export const loadRun = (id: string, run: string) => load<Run>(`${id}/runs/${run}.json`);
/** Forgets a loaded file, so the next use() fetches it again (after the workspace changed it). */
export const forget = (path: string) => cache.delete(path);

/** Like load, but a missing or unreadable file gives the fallback. */
function loadOr<T>(path: string, fallback: T): Promise<T> {
  let p = cache.get(path) as Promise<T> | undefined;
  if (!p) {
    p = fetch(`data/${path}`, { cache: 'no-store' })
      .then((r) => (r.ok ? (r.json() as Promise<T>) : fallback))
      .catch(() => fallback);
    cache.set(path, p);
  }
  return p;
}

// Each feature's requirements loop (internal/story).
export interface StoryEvent {
  kind: 'proposed' | 'added' | 'changed' | 'approved' | 'removed' | 'passed' | 'failed';
  at: string;
  commit: string;
  by?: string;
  steps?: string[];
  was?: string[];
  version?: number;
  run?: string;
  video?: string;
  subject?: string;
}
export interface StoryScenario {
  title: string;
  state: 'proposed' | 'approved' | 'passing' | 'failing' | 'removed';
  steps: string[];
  events: StoryEvent[];
}
export interface StoryFeature {
  path: string;
  name: string;
  scenarios: StoryScenario[];
}
/** The trunk's requirements, or a branch's still waiting for approval (its key from the index). */
const storiesDir = (id: string, branch?: string) => (branch ? `${id}/branches/${branch}` : id);
export const loadStories = (id: string, branch?: string) => load<StoryFeature[]>(`${storiesDir(id, branch)}/stories.json`);

// Measured requirements (internal/story, metrics.go).
export interface MetricEvent {
  kind: 'proposed' | 'added' | 'changed' | 'approved' | 'removed' | 'met' | 'missed';
  at: string;
  commit: string;
  by?: string;
  steps?: string[];
  was?: string[];
  version?: number;
  run?: string;
}
/** A .metric line and what Shipwake read it as (internal/metricspec). */
export interface MetricLine {
  n: number;
  text: string;
  kind: 'tag' | 'title' | 'description' | 'dataset' | 'grader' | 'direction' | 'block' | 'warn' | 'blank' | 'comment' | 'error';
  value?: string;
  error?: string;
}
export interface MetricStory {
  id: string;
  path: string;
  name?: string;
  description?: string;
  dataset?: string;
  grader?: string;
  definition: string[];
  lines: MetricLine[];
  state: 'proposed' | 'approved' | 'met' | 'missed' | 'removed';
  events: MetricEvent[];
}
/** Sites built before measured requirements have no such file: none, then. The fallback is cached
 *  with the file, so every render gets the same promise and forget() refreshes it. */
export const loadMetricStories = (id: string, branch?: string) =>
  loadOr<MetricStory[]>(`${storiesDir(id, branch)}/metric-stories.json`, []);

/** The local workspace: present only when the site is served by `shipwake serve`. */
export interface Session {
  edit: boolean;
  user: string;
  checkedOut?: Record<string, string>; // project → its checked-out branch, where approvals are committed
  projects?: Record<string, { repo: string; workspace: string; local: boolean }>; // where each is on this machine
  skill?: string; // the agent skill's file
}
let session: Promise<Session> | undefined;
/** After opening a repository: the site and the session list one more project. */
export const forgetSite = () => {
  session = undefined;
  forget('site.json');
};
export const loadSession = () => {
  session ??= fetch('api/session')
    .then((r) => (r.ok ? (r.json() as Promise<Session>) : { edit: false, user: '' }))
    .catch(() => ({ edit: false, user: '' }));
  return session;
};

export const fileUrl = (id: string, run: string, path: string) => `data/${id}/files/${run}/${path}`;

const many = new Map<string, Promise<Run[]>>();
/** Several runs at once, cached by their IDs so components can `use()` the result. */
export function loadRuns(id: string, runs: string[]) {
  const key = `${id}:${runs.join(',')}`;
  let p = many.get(key);
  if (!p) {
    p = Promise.all(runs.map((r) => loadRun(id, r)));
    many.set(key, p);
  }
  return p;
}

const details = new Map<string, Promise<CommitDetail[]>>();
/** Several commits' details at once, cached so components can `use()` the result. */
export function loadCommits(id: string, shas: string[]) {
  const key = `${id}:${shas.join(',')}`;
  let p = details.get(key);
  if (!p) {
    p = Promise.all(shas.map((s) => loadCommit(id, s)));
    details.set(key, p);
  }
  return p;
}
