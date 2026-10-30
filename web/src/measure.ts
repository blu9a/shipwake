// Measurement helpers shared by the run, measurement and release pages. The judging itself happens in the
// build (Go), so the page only reads the comparisons; these helpers find series and thresholds.
import type { Comparison, Metric, Project, Rule, RunSummary } from './model.gen';

/** Runs compare within a series: the same name on the same runner. The kind is only a label. */
export const seriesKey = (r: Pick<RunSummary, 'name' | 'context'>) => `${r.name}\u0000${r.context?.runner ?? ''}`;

/** The runs of one series in history order, oldest first. */
export function series(p: Project, r: RunSummary) {
  const key = seriesKey(r);
  return p.runs.filter((x) => seriesKey(x) === key && x.order >= 0).sort((a, b) => a.order - b.order || a.start.localeCompare(b.start));
}

/** How many runs a measurement's trend shows: in its row, and on its own page. More crowds the
 *  chart; fewer says little. */
export const trendInRow = 5;
export const trendOnPage = 20;

/** The last n items up to and including index i: a trend ending at the run being looked at. */
export function upTo<T>(items: T[], i: number, n: number): T[] {
  const end = i < 0 ? items.length : i + 1;
  return items.slice(Math.max(0, end - n), end);
}

/** A metric's rule (direction, thresholds): an exact ID, else the first pattern where * is one dot-separated part. */
export function ruleFor(p: Project, id: string): Rule | undefined {
  const all = p.rules ?? {};
  if (all[id]) return all[id];
  for (const k of Object.keys(all).sort()) {
    if (!k.includes('*')) continue;
    const re = new RegExp(
      `^${k
        .split('*')
        .map((x) => x.replace(/[.+?^${}()|[\]\\]/g, '\\$&'))
        .join('[^.]+')}$`,
    );
    if (re.test(id)) return all[k];
  }
  return undefined;
}

/** Whether a run has a run at the last release to compare with. */
export const hasRelease = (run: { metrics?: Metric[] }) => (run.metrics ?? []).some((m) => !!m.release?.run);

/** The baseline used for a run: the chosen one when the run has it; the last release falls back to the
 *  previous run, and with neither the comparisons are empty (levels may still judge). */
export const effectiveFor = (run: { metrics?: Metric[] }, chosen: 'previous' | 'release'): 'previous' | 'release' =>
  chosen === 'release' && hasRelease(run) ? 'release' : 'previous';

export const comparison = (m: Metric, baseline: 'previous' | 'release'): Comparison | undefined =>
  baseline === 'release' ? m.release : m.previous;

/** The newest run of a series on a commit that has something comparable; else the newest. */
export function decisiveOn(runs: RunSummary[]) {
  const sorted = [...runs].sort((a, b) => b.start.localeCompare(a.start));
  return sorted.find((r) => r.comparable) ?? sorted[0];
}

const unitScale: Record<string, number>[] = [
  { ns: 1, µs: 1e3, us: 1e3, ms: 1e6, s: 1e9, min: 60e9, h: 3600e9 },
  { B: 1, KiB: 1024, MiB: 1024 ** 2, GiB: 1024 ** 3, TiB: 1024 ** 4, KB: 1e3, MB: 1e6, GB: 1e9, TB: 1e12 },
];
const convert = (x: number, from: string, to: string) => {
  if (!from || from === to) return x;
  for (const fam of unitScale) if (fam[from] && fam[to]) return (x * (fam[from] as number)) / (fam[to] as number);
  return undefined;
};

/** Where a threshold sits on a chart, in the metric's unit. Only a single condition is one line: a level,
 *  or a change from the release baseline. Combined thresholds ("+15% and +20ms") draw none. */
export function thresholdLine(th: string | undefined, unit: string, releaseValue: number | undefined): number | undefined {
  const t = th?.trim();
  if (!t || /\s(and|or)\s/i.test(t)) return undefined;
  const m = t.match(/^(<=?|>=?)?\s*([+\-±]?)\s*(\d+(?:\.\d*)?|\.\d+)\s*(%|[A-Za-zµ/]+)?$/);
  if (!m) return undefined;
  const [, cmp, sign, num, u = ''] = m;
  const v = Number(num);
  if (cmp || !sign) return convert(v, u === '%' ? '' : u, unit);
  if (sign === '±' || releaseValue === undefined) return undefined;
  const dir = sign === '+' ? 1 : -1;
  if (u === '%') return releaseValue * (1 + (dir * v) / 100);
  const amount = convert(v, u, unit);
  return amount === undefined ? undefined : releaseValue + dir * amount;
}
