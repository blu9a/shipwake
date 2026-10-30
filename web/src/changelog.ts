// A release's changelog, assembled from its commits the way generated changelogs usually are: Conventional
// Commits types in sections, breaking changes first, internal types left out. What a
// release changed in its requirements (approved scenarios and metrics) has no such convention, so it
// gets its own section. A commit's Release-note line replaces its subject; Release-note: none drops it.
// The result is Markdown text: the release's notes, edited by the person and stored in the tag as is.

export type Section = 'breaking' | 'requirements' | 'feat' | 'fix' | 'perf' | 'revert' | 'other';
export const sectionOrder: Section[] = ['breaking', 'requirements', 'feat', 'fix', 'perf', 'revert', 'other'];

/** Types a generated changelog usually leaves out: work for the team. */
const shown = new Set(['feat', 'fix', 'perf', 'revert']);
const internal = new Set(['chore', 'test', 'tests', 'ci', 'build', 'style', 'refactor', 'docs', 'spec']);

export interface ChangeCommit {
  short: string;
  subject: string;
  body?: string;
}

/** A requirement changed in the release: a feature's scenarios, or a metric. */
export interface RequirementChange {
  name: string;
  kind: 'scenarios' | 'metric';
  added: number;
  changed: number;
  removed: number;
  short: string; // the last commit that changed it
}

export interface Words {
  heading: Record<Section, string>;
  requirement: (r: RequirementChange) => string;
}

const conventional = /^(\w+)(?:\(([^)]*)\))?(!)?:\s*(.+)$/;

/** The text a commit contributes in a language, or null when it asked to be left out. */
function noteOf(c: ChangeCommit, lang: string, primary: string): string | null | undefined {
  const lines = (c.body ?? '').split('\n');
  const own = lines.map((l) => l.match(new RegExp(`^Release-note-${lang}:\\s*(.+)$`))?.[1]).find(Boolean);
  const main = lines.map((l) => l.match(/^Release-note:\s*(.+)$/)?.[1]).find(Boolean);
  if (main?.trim().toLowerCase() === 'none') return null;
  if (lang !== primary && own) return own.trim();
  return main?.trim();
}

/** The changelog's sections for one language, in order; empty sections are left out. */
export function changelog(commits: ChangeCommit[], requirements: RequirementChange[], words: Words, lang: string, primary: string): string {
  const items: Record<Section, string[]> = { breaking: [], requirements: [], feat: [], fix: [], perf: [], revert: [], other: [] };
  for (const r of requirements) items.requirements.push(`${words.requirement(r)} (${r.short})`);
  for (const c of commits) {
    const note = noteOf(c, lang, primary);
    if (note === null) continue;
    const m = c.subject.match(conventional);
    const breaking = (c.body ?? '').match(/^BREAKING[ -]CHANGE:\s*(.+)$/m)?.[1];
    // Only the types Conventional Commits defines count; "middleware:" or "README:" names a component,
    // so such a commit keeps its whole subject.
    const type = m?.[1]?.toLowerCase() ?? '';
    if (!m || !(shown.has(type) || internal.has(type))) {
      items.other.push(`${note ?? c.subject} (${c.short})`);
      continue;
    }
    const [, , scope = '', bang, description = ''] = m;
    const text = note ?? (scope && scope !== 'spec' ? `${scope}: ${description}` : description);
    if (bang || breaking) items.breaking.push(`${breaking?.trim() ?? text} (${c.short})`);
    if (type === 'spec' || scope === 'spec') continue; // its requirements speak for it
    if (shown.has(type)) items[type as Section].push(`${text} (${c.short})`);
    else if (!internal.has(type)) items.other.push(`${text} (${c.short})`); // a type of the project's own
  }
  return sectionOrder
    .filter((s) => items[s].length)
    .map((s) => `### ${words.heading[s]}\n\n${items[s].map((i) => `- ${i}`).join('\n')}`)
    .join('\n\n');
}

/** The notes with fresh sections: the person's own text before the first section stays as written. */
export function withChangelog(notes: string, sections: string): string {
  const own = notes.split(/^###\s/m)[0]?.trim() ?? '';
  return [own, sections].filter(Boolean).join('\n\n');
}
