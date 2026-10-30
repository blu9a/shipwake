// Shipwake's Playwright reporter. It writes one run in Shipwake's own format (`shipwake add -f json`):
// every scenario step, including Background steps that run inside hooks; attachments saved as files
// next to the report; and the "observed" evidence recorded by `decisive()`.
//
//   reporter: [['list'], ['@shipwake/playwright/reporter', { outputFile: 'shipwake/run.json', name: 'Web tests' }]]
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestResult, TestStep } from '@playwright/test/reporter';

interface Options {
  outputFile?: string;
  name?: string;
}
interface Attachment {
  name: string;
  type: string;
  path: string;
  start?: string;
}
interface Step {
  name: string;
  status: string;
  start: string;
  duration: number;
  attachments?: Attachment[];
}
interface Test {
  feature?: string;
  name: string;
  target?: string;
  status: string;
  start: string;
  duration: number;
  attempts: number;
  retried?: boolean;
  error?: string;
  steps?: Step[];
  attachments?: Attachment[];
  evidence?: { step: string; expected: string[]; observed: string }[];
}

const ext: Record<string, string> = { 'image/png': '.png', 'image/jpeg': '.jpg', 'video/webm': '.webm', 'application/json': '.json', 'text/plain': '.txt' };

export default class ShipwakeReporter implements Reporter {
  private out: string;
  private files: string;
  private name: string;
  private start = new Date();
  private tests = new Map<string, Test>();
  private n = 0;

  constructor(options: Options = {}) {
    this.out = resolve(options.outputFile ?? process.env.SHIPWAKE_OUTPUT ?? 'shipwake-run.json');
    this.files = `${this.out.replace(/\.json$/, '')}-files`;
    this.name = options.name ?? process.env.SHIPWAKE_NAME ?? '';
  }

  onBegin(_config: FullConfig, _suite: Suite) {
    this.start = new Date();
  }

  onTestEnd(test: TestCase, result: TestResult) {
    const evidence: Test['evidence'] = [];
    const save = (a: TestResult['attachments'][number]): Attachment | undefined => {
      if (a.name.startsWith('observed: ') && a.contentType === 'application/json' && a.body) {
        try {
          evidence.push(JSON.parse(a.body.toString()));
        } catch {
          // not ours
        }
        return undefined;
      }
      let path = a.path;
      if (!path && a.body) {
        mkdirSync(this.files, { recursive: true });
        path = join(this.files, `${++this.n}${ext[a.contentType] ?? ''}`);
        writeFileSync(path, a.body);
      }
      if (!path) return undefined;
      return { name: a.name, type: a.contentType, path: relative(dirname(this.out), path) };
    };
    // Scenario steps, wherever they ran: in the test, or in hooks (Gherkin Background).
    const steps: Step[] = [];
    const used = new Set<TestResult['attachments'][number]>();
    const walk = (list: TestStep[]) => {
      for (const s of list) {
        if (s.category === 'test.step' && !/\bhook$/i.test(s.title)) {
          const own = (s.attachments ?? []).map((a: TestResult['attachments'][number]) => (used.add(a), save(a))).filter((a): a is Attachment => !!a);
          steps.push({
            name: s.title,
            status: s.error ? 'failed' : 'passed',
            start: s.startTime.toISOString(),
            duration: s.duration,
            ...(own.length ? { attachments: own } : {}),
          });
        } else walk(s.steps);
      }
    };
    walk(result.steps);
    // A recording begins when the page opens: the page fixture's start, so subtitles can follow it.
    const opened = (list: TestStep[]): Date | undefined => {
      for (const s of list) {
        if (s.category === 'fixture' && /\bpage\b/.test(s.title)) return s.startTime;
        const inner = opened(s.steps);
        if (inner) return inner;
      }
      return undefined;
    };
    const recorded = opened(result.steps);
    // Screenshots named after a step ("after: …", "decisive: …") belong to that step.
    const keep = (a: TestResult['attachments'][number]) => /^(image|video|text)\//.test(a.contentType) || a.contentType === 'application/zip' || a.name.startsWith('observed: ');
    const rest: Attachment[] = [];
    for (const a of result.attachments) {
      if (used.has(a) || !keep(a)) continue;
      // A recording is the demo of a passing scenario; a failure has its screenshot and error.
      if (a.contentType.startsWith('video/') && result.status !== 'passed') continue;
      const m = a.name.match(/^(?:after|decisive): (.*)$/);
      const owner = m ? steps.find((st) => st.name.replace(/^(Given|When|Then|And|But) /, '') === m[1]) : undefined;
      const saved = save(a);
      if (saved && recorded && a.contentType.startsWith('video/')) saved.start = recorded.toISOString();
      if (!saved) continue;
      if (owner) owner.attachments = [...(owner.attachments ?? []), saved];
      else rest.push(saved);
    }
    const describe = test.titlePath().slice(3, -1); // ['', project, file, ...describe, title]
    const t: Test = {
      feature: describe[0] ?? test.titlePath()[2],
      name: test.title,
      target: test.parent.project()?.name || undefined,
      status: result.status === 'passed' ? 'passed' : result.status === 'skipped' ? 'skipped' : 'failed',
      start: result.startTime.toISOString(),
      duration: result.duration,
      attempts: result.retry + 1,
      retried: result.status === 'passed' && result.retry > 0,
      error: result.error?.message,
      steps: steps.length ? steps : undefined,
      attachments: rest.length ? rest : undefined,
      evidence: evidence.length ? evidence : undefined,
    };
    this.tests.set(test.id, t); // a retry replaces the earlier attempt
  }

  onEnd(result: FullResult) {
    mkdirSync(dirname(this.out), { recursive: true });
    const run = {
      kind: 'test',
      name: this.name,
      start: this.start.toISOString(),
      duration: result.duration,
      tests: [...this.tests.values()],
    };
    writeFileSync(this.out, JSON.stringify(run, null, 2));
  }

  printsToStdio() {
    return false;
  }
}

