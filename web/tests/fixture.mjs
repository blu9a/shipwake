// Deterministic browser data: no personal repository, backend, network, or Git history required.
import { crc32, deflateSync } from 'node:zlib';

/** A flat PNG of one color: a screenshot's stand-in. */
function png(w, h, [r, g, b, a = 255]) {
  const chunk = (type, data) => {
    const out = Buffer.alloc(12 + data.length);
    out.writeUInt32BE(data.length, 0);
    out.write(type, 4, 'latin1');
    data.copy(out, 8);
    out.writeUInt32BE(crc32(out.subarray(4, 8 + data.length)), 8 + data.length);
    return out;
  };
  const head = Buffer.alloc(13);
  head.writeUInt32BE(w, 0);
  head.writeUInt32BE(h, 4);
  head.set([8, 6, 0, 0, 0], 8);
  const row = Buffer.concat([Buffer.from([0]), Buffer.from(Array.from({ length: w }, () => [r, g, b, a]).flat())]);
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', head), chunk('IDAT', deflateSync(Buffer.concat(Array(h).fill(row)))), chunk('IEND', Buffer.alloc(0))]);
}
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
// Visual review: a subject with a long name in four variants, two of them changed, and a new one.
const longSubject = `Bill row ${'WithAVeryLongUnbrokenComponentName'.repeat(3)}`;
const look = (subject, variant, status, i, size = [640, 120]) => {
  const name = variant ? `${subject} [${variant}]` : subject;
  const approved = status === 'new' ? undefined : {
    id: `a${i}`, subject, variant, commit: base, run: 'tests', path: `approved-${i}.png`, hash: 'a'.repeat(64),
    width: size[0], height: size[1], by: 'Responsive test <test@example.com>', at: previousStart,
  };
  return { id: `look-${i}`, name, subject, variant, status, run: 'tests', path: `look-${i}.png`, hash: 'b'.repeat(64),
    width: size[0], height: size[1], approved, pixels: status === 'changed' ? 1234567 : undefined, diff: status === 'changed' ? 'diff.png' : undefined };
};
export const looks = {
  commit: head,
  looks: [
    look(longSubject, 'light, phone', 'same', 1, [320, 72]),
    look(longSubject, 'light, desktop', 'same', 2),
    look(longSubject, `dark, phone, ${'VariantWithoutBreaks'.repeat(3)}`, 'changed', 3, [320, 72]),
    look(longSubject, 'dark, desktop', 'changed', 4),
    look('Receipt card', '', 'new', 5, [280, 140]),
    look('Each pays', '', 'same', 6, [300, 50]),
  ],
};
project.looks = { [head]: { same: 3, changed: [looks.looks[2].name, looks.looks[3].name], new: ['Receipt card'] } };
const images = Object.fromEntries(looks.looks.flatMap((l, i) => [
  [`/data/fixture/files/tests/${l.path}`, png(l.width, l.height, i % 2 ? [37, 99, 235] : [17, 17, 19])],
  ...(l.approved ? [[`/data/fixture/files/tests/${l.approved.path}`, png(l.width, l.height, [22, 163, 74])]] : []),
]));
images['/data/fixture/looks/diffs/diff.png'] = png(640, 120, [229, 72, 77, 64]);

const detail = (sha) => ({
  sha, tags: [], files: [{ path, status: 'M', added: 123456, deleted: 12345 }],
  scenarios: [{ feature: 'Responsive components', title: 'A scenario with readable results on narrow screens', change: 'added',
    steps: ['Given a narrow screen', `Then ${'ReadableContent'.repeat(15)}`] }],
});
export const files = {
  '/data/site.json': { version: 1, generatedAt: start, projects: [{ id: project.id, name: project.name }, { id: 'empty', name: 'Empty project' }] },
  '/data/fixture/index.json': project,
  '/data/fixture/stories.json': [],
  [`/data/fixture/looks/${head}.json`]: looks,
  ...images,
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
  `/fixture/look/${head}/look-3`, `/fixture/look/${head}/look-5`,
];
