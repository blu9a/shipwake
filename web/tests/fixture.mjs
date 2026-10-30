// Deterministic browser data: no personal repository, backend, network, or Git history required.
export const head = 'b'.repeat(40);
export const base = 'a'.repeat(40);
export const branch = 'feat/responsive-components-with-a-very-long-unbroken-branch-name';
export const path = `web/src/${'deeply-nested-component/'.repeat(5)}ResponsiveComponent.tsx`;
const start = '2026-10-28T14:00:00Z';
const previousStart = '2026-10-27T14:00:00Z';
const subject = `Keep long content readable: ${'LongUnbrokenCommitSubject'.repeat(5)}`;

const metric = (id, name, values, status = 'ok') => ({
  id, name, suite: 'API load', values, unit: 'ms', direction: 'lower',
  previous: { run: 'previous', label: 'aaaaaaa', value: 1000, delta: 12.5, status },
  release: { run: 'previous', label: 'v1.0.0', value: 1000, delta: 25, status: 'block' },
});
const metrics = [
  metric('latency', 'RequestLatencyWithoutAnyWordBreaks'.repeat(3), [12345, 12455], 'warn'),
  metric('render', 'Rendering latency', [81, 83]),
  metric('parse', 'Parsing latency', [10, 12]),
  metric('total', 'End-to-end latency', [1000, 1100]),
];
const context = {
  runner: 'browser-ci',
  model: 'LongUnbrokenModelIdentifier'.repeat(5),
  ['environment'.repeat(8)]: 'A long context value that must wrap within its own column',
};
const targets = ['desktop', 'phone', 'tablet', 'large-text', 'touch', 'long-target'];
export const runs = {
  previous: {
    id: 'previous', kind: 'perf', name: 'API load', commit: base, branch: 'main',
    start: previousStart, duration: 30, format: 'shipwake', context,
    metrics: metrics.map((m) => ({ ...m, values: [1000], previous: undefined, release: undefined })),
  },
  current: {
    id: 'current', kind: 'perf', name: 'API load', commit: head, branch: 'main',
    start, duration: 30, format: 'shipwake', context, metrics,
  },
  tests: {
    id: 'tests', kind: 'test', name: 'Browser acceptance', commit: head, branch,
    start, duration: 120, format: 'playwright', context,
    tests: targets.map((target, i) => ({
      feature: 'Responsive components', name: `A readable scenario ${'LongScenarioName'.repeat(5)}`,
      target, status: i === 0 ? 'failed' : 'passed', retried: i === 1,
      duration: 123.456, attempts: i === 1 ? 2 : 1,
      error: i === 0 ? 'FailureWithNoWordBreaks'.repeat(15) : undefined,
      steps: [
        { name: 'Given a narrow screen', status: 'passed', duration: 1 },
        { name: 'When the window resizes', status: 'passed', duration: 1 },
        { name: 'Then the complete result stays readable', status: i === 0 ? 'failed' : 'passed', duration: 1 },
      ],
      evidence: [{ step: 'the complete result stays readable', expected: ['complete'], observed: 'The complete value stays visible.' }],
    })),
  },
};
const summary = (run, order) => ({
  ...run, tests: undefined, metrics: run.metrics?.length,
  passed: run.kind === 'test' ? 5 : 0, failed: run.kind === 'test' ? 1 : 0, skipped: 0,
  order, comparable: true, targets: run.kind === 'test' ? targets : undefined,
  previous: { ok: 3, warn: 1 }, release: { block: 4 },
});
export const project = {
  id: 'fixture', name: 'Shipwake responsive project', trunk: 'main', languages: ['en', 'zh'],
  targets: Object.fromEntries(targets.map((t) => [t, { en: t === 'long-target' ? 'VeryLongUnbrokenTargetName'.repeat(4) : t, zh: `设备 ${t}` }])),
  rows: [
    { sha: head, merge: true, branch, title: subject, kind: 'fix', commits: [head] },
    { sha: base, merge: false, title: 'Initial release', kind: 'feat' },
  ],
  tags: [{ name: 'v1.0.0', sha: base, date: previousStart, subject: 'A readable release', body: 'Release notes remain readable on every screen.' }],
  branches: [{ name: branch, base, ahead: [head], behind: 1, updated: start }],
  runs: [summary(runs.previous, 0), summary(runs.current, 1), summary(runs.tests, 1)],
  commits: {
    [head]: { sha: head, short: 'bbbbbbb', parents: [base, 'c'.repeat(40)], author: 'Responsive test', date: start, subject,
      body: 'Release-note: Menus, swipes and hover buttons now work the same way with touch, mouse and trackpad.\nRelease-note-zh: 菜单、滑动和悬停按钮在触屏、鼠标和触控板上的用法现在一致。' },
    [base]: { sha: base, short: 'aaaaaaa', parents: [], author: 'Responsive test', date: previousStart, subject: 'Initial release' },
  },
};
const detail = (sha) => ({
  sha, tags: [], files: [{ path, status: 'M', added: 123456, deleted: 12345 }],
  scenarios: [{ feature: 'Responsive components', title: 'A scenario with readable results on narrow screens', change: 'added',
    steps: ['Given a narrow screen', `Then ${'ReadableContent'.repeat(15)}`] }],
});
export const files = {
  '/data/site.json': { version: 1, generatedAt: start, projects: [{ id: project.id, name: project.name }, { id: 'empty', name: 'Empty project' }] },
  '/data/fixture/index.json': project,
  '/data/fixture/stories.json': [],
  '/data/empty/stories.json': [],
  '/data/empty/index.json': { ...project, id: 'empty', name: 'Empty project', rows: [], tags: [], runs: [], branches: [], commits: {} },
  ...Object.fromEntries(Object.entries(runs).map(([id, r]) => [`/data/fixture/runs/${id}.json`, r])),
  ...Object.fromEntries([head, base].flatMap((sha) => [
    [`/data/fixture/commits/${sha}.json`, detail(sha)],
    [`/data/fixture/diffs/${sha}.json`, { [path]: { lines: ['@@ -1 +1 @@', `-${'oldCode'.repeat(80)}`, `+${'newCode'.repeat(80)}`] } }],
  ])),
};
export const routes = [
  '/fixture', `/fixture/branch/${branch}`, `/fixture/commit/${head}`,
  '/fixture/run/current', '/fixture/run/tests', '/fixture/measure/current/latency', '/fixture/release', '/empty',
];
