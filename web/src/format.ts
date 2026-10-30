import type { Lang } from './messages';
import type { Names, Project } from './model.gen';

const locale = (lang: Lang) => (lang === 'zh' ? 'zh-CN' : 'en-US');

export const when = (iso: string, lang: Lang) =>
  new Intl.DateTimeFormat(locale(lang), { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(
    new Date(iso),
  );

export const day = (iso: string, lang: Lang) =>
  new Intl.DateTimeFormat(locale(lang), { month: 'short', day: 'numeric' }).format(new Date(iso));

export const full = (iso: string, lang: Lang) =>
  new Intl.DateTimeFormat(locale(lang), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso));

/** A project's own text in the viewer's language, when the project provides it. */
export function tr(p: Project, text: string, lang: Lang) {
  return (p.text?.[text] as Names | undefined)?.[lang] ?? text;
}

/** A target's display name: from the project config, or as recorded. */
export function targetName(p: Project, t: string, lang: Lang) {
  const n = p.targets?.[t];
  return n?.[lang] ?? n?.[p.languages[0] ?? 'en'] ?? t;
}

export const avg = (v: number[]) => v.reduce((a, b) => a + b, 0) / Math.max(v.length, 1);

const binaryUnits = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB'];

export function fmtValue(v: number, unit: string) {
  if (Number.isNaN(v)) return '—';
  let i = binaryUnits.indexOf(unit);
  if (i >= 0) {
    while (Math.abs(v) >= 1024 && i < binaryUnits.length - 1) {
      v /= 1024;
      i++;
    }
    return `${i === 0 ? Math.round(v) : v.toFixed(1)} ${binaryUnits[i]}`;
  }
  if (unit === 'ns') return v >= 1e6 ? `${(v / 1e6).toFixed(2)} ms` : v >= 1000 ? `${(v / 1000).toFixed(1)} µs` : `${Math.round(v)} ns`;
  if (unit === 'ms') return `${v >= 100 ? Math.round(v).toLocaleString() : +v.toFixed(1)} ms`;
  if (unit === 'allocs') return `${Math.round(v)}`;
  if (unit === 'score') return v.toFixed(2);
  if (unit === '%') return `${+v.toFixed(2)}%`;
  return `${(+v.toFixed(2)).toLocaleString()} ${unit}`;
}

/** How a commit is named to people: its release's version when it is one, else its short hash. */
export function commitLabel(p: Project, sha: string): string {
  return p.tags.find((t) => t.sha === sha)?.name ?? p.commits[sha]?.short ?? sha.slice(0, 7);
}
