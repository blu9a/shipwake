// Preparing a release: a version, the checks on the newest trunk commit, public notes, and the command
// that records the approval as an annotated tag. The site is static, so approving gives the command.
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { use, useState } from 'react';
import { changelog, type RequirementChange, withChangelog } from '../changelog';
import { forget, loadMetricStories, loadProject, loadSession, loadStories, type MetricStory, type StoryFeature } from '../data';
import { when } from '../format';
import { decisiveOn } from '../measure';
import { type Lang, messages } from '../messages';
import type { Project, Row, RunSummary } from '../model.gen';
import { useSettings, useT } from '../settings';
import { Segmented, StatusIcon } from '../ui';
import { NotesText } from './notes';
import s from './ReleasePage.module.css';
import tl from './Timeline.module.css';

type Status = 'ok' | 'warn' | 'bad';
interface CheckRow {
  status: Status;
  text: string;
  to?: { run: string };
}

const langName = (l: string) => {
  try {
    return new Intl.DisplayNames([l], { type: 'language' }).of(l) ?? l;
  } catch {
    return l;
  }
};
// Versions are SemVer tags: v1.2.3, or a pre-release such as v1.2.3-rc.1 or v1.2.3-beta.
const versionRe = /^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/;
const semver = (v: string) => {
  const m = versionRe.exec(v);
  return m ? { core: [Number(m[1]), Number(m[2]), Number(m[3])], pre: m[4]?.split('.') ?? [] } : undefined;
};
/** SemVer precedence: the core numbers, then a pre-release ranks before its release (1.0.0-rc.1 < 1.0.0),
 *  and pre-releases compare field by field, numbers numerically and below words. */
const compare = (x: NonNullable<ReturnType<typeof semver>>, y: NonNullable<ReturnType<typeof semver>>): number => {
  for (let i = 0; i < 3; i++) if (x.core[i] !== y.core[i]) return (x.core[i] ?? 0) - (y.core[i] ?? 0);
  if (!x.pre.length || !y.pre.length) return y.pre.length - x.pre.length;
  for (let i = 0; i < Math.max(x.pre.length, y.pre.length); i++) {
    const a = x.pre[i];
    const b = y.pre[i];
    if (a === undefined || b === undefined) return a === undefined ? -1 : 1;
    if (a === b) continue;
    const na = /^\d+$/.test(a);
    const nb = /^\d+$/.test(b);
    if (na && nb) return Number(a) - Number(b);
    if (na !== nb) return na ? -1 : 1;
    return a < b ? -1 : 1;
  }
  return 0;
};

export function ReleasePage() {
  const { project } = useParams({ from: '/$project/release' });
  const p = use(loadProject(project));
  const session = use(loadSession());
  const t = useT();
  const { lang: viewer } = useSettings();
  const router = useRouter();
  const tagged = new Set(p.tags.map((x) => x.sha));
  const cut = p.rows.findIndex((r) => tagged.has(r.sha));
  const unreleased = cut < 0 ? p.rows : p.rows.slice(0, cut);
  const last = p.tags[0];
  const head = p.rows[0];
  const prev = last ? semver(last.name) : undefined;
  const [a = 0, b = 0, c = 0] = prev?.core ?? [];
  // After a pre-release, the release it leads to; otherwise the next minor for features, else the next patch.
  const suggested = !prev
    ? 'v0.1.0'
    : prev.pre.length
      ? `v${a}.${b}.${c}`
      : unreleased.some((r) => r.kind === 'feat')
        ? `v${a}.${b + 1}.0`
        : `v${a}.${b}.${c + 1}`;
  const [version, setVersion] = useState(suggested);
  const [notes, setNotes] = useState<Record<string, string>>(() => {
    try {
      return JSON.parse(localStorage.getItem(`notes:${project}`) ?? '{}');
    } catch {
      return {};
    }
  });
  const [lang, setLang] = useState(p.languages[0] ?? 'en');
  const [approved, setApproved] = useState(false);
  const save = (next: Record<string, string>) => {
    setNotes(next);
    setApproved(false);
    try {
      localStorage.setItem(`notes:${project}`, JSON.stringify(next));
    } catch {
      // private mode
    }
  };
  if (!head) return <p className="muted">{t.notFound}</p>;
  // Everything on the trunk is released: there is nothing to release again.
  if (last && unreleased.length === 0)
    return (
      <>
        <button type="button" className="back" onClick={() => router.history.back()}>
          ‹ {t.back}
        </button>
        <h1 className={s.title}>{t.newRelease}</h1>
        <p className="muted">{t.nothingNew(last.name)}</p>
      </>
    );
  const checks = [...runChecks(p, head.sha, t), ...versionChecks(p, version, t), ...p.languages.map((l, i) => noteCheck(l, i, notes, t))];
  const blocked = checks.some((ch) => ch.status === 'bad');
  return (
    <>
      <button type="button" className="back" onClick={() => router.history.back()}>
        ‹ {t.back}
      </button>
      <h1 className={s.title}>{t.newRelease}</h1>

      <section className={s.sec}>
        <h2>{t.version}</h2>
        <div className={s.row}>
          <input
            className={`field mono ${s.version}`}
            value={version}
            spellCheck={false}
            onChange={(e) => {
              setVersion(e.target.value.trim());
              setApproved(false);
            }}
          />
          {last && <span className="muted small">{t.lastRelease(last.name)}</span>}
        </div>
      </section>

      <section className={s.sec}>
        <h2>{t.commit}</h2>
        <div className={s.commit}>
          <Link className="lnk" to="/$project/commit/$sha" params={{ project, sha: head.sha }}>
            {p.commits[head.sha]?.subject}
          </Link>
          <div className="meta">
            <code>{p.commits[head.sha]?.short}</code>
            <span>{when(p.commits[head.sha]?.date ?? '', viewer)}</span>
          </div>
        </div>
      </section>

      <section className={s.sec}>
        <h2>{t.checks}</h2>
        <div className="list">
          {checks.map((ch) => (
            <div key={ch.text} className={s.check}>
              <StatusIcon status={ch.status === 'ok' ? 'passed' : ch.status === 'bad' ? 'failed' : 'warn'} />
              {ch.to ? (
                <Link className="lnk" to="/$project/run/$run" params={{ project, run: ch.to.run }}>
                  {ch.text}
                </Link>
              ) : (
                <span>{ch.text}</span>
              )}
            </div>
          ))}
        </div>
      </section>

      <section className={s.sec}>
        <div className={s.secHead}>
          <h2>{t.notes}</h2>
          <Assemble p={p} rows={unreleased} notes={notes} onDraft={save} />
        </div>
        {p.languages.length > 1 && (
          <Segmented value={lang} onChange={setLang} options={p.languages.map((l) => ({ value: l, label: langName(l) }))} />
        )}
        <textarea
          className={`field ${s.notes}`}
          rows={7}
          placeholder={t.notesPlaceholder}
          value={notes[lang] ?? ''}
          onChange={(e) => save({ ...notes, [lang]: e.target.value })}
        />
      </section>

      <section className={s.sec}>
        <h2>{t.preview}</h2>
        <div className={tl.release}>
          <div className={tl.cardHead}>
            <b className={tl.version}>{version}</b>
            <span className={tl.when}>{t.merges(unreleased.filter((r) => r.merge).length)}</span>
          </div>
          {notes[lang]?.trim() ? <NotesText text={notes[lang] ?? ''} /> : <p className="muted">{t.notesPlaceholder}</p>}
        </div>
      </section>

      <section className={s.sec}>
        {session.edit ? (
          // Served locally: approving creates the tag, as the person.
          <Tag key={version} p={p} version={version} sha={head.sha} notes={notes} blocked={blocked} />
        ) : (
          <>
            <button type="button" className={s.primary} disabled={blocked} onClick={() => setApproved(true)}>
              {t.approve}
            </button>
            {approved && !blocked && <Command p={p} version={version} sha={head.sha} notes={notes} />}
          </>
        )}
      </section>
    </>
  );
}

/** The latest test run per suite and the measurement verdicts of each series on the commit. */
function runChecks(p: Project, sha: string, t: ReturnType<typeof useT>): CheckRow[] {
  const on = p.runs.filter((r) => r.commit === sha);
  const out: CheckRow[] = [];
  const byName = new Map<string, RunSummary[]>();
  const hasTests = (r: RunSummary) => r.passed + r.failed + r.skipped > 0;
  for (const r of on) byName.set(r.name, [...(byName.get(r.name) ?? []), r]);
  const tests = [...byName.values()].map((rs) => rs.filter(hasTests)).filter((rs) => rs.length);
  if (!tests.length && p.runs.some(hasTests)) out.push({ status: 'bad', text: t.noTestsOn });
  for (const rs of tests) {
    const r = [...rs].sort((a, b) => b.start.localeCompare(a.start))[0] as RunSummary;
    if (r.failed) out.push({ status: 'bad', text: t.testsFailedOn(r.name, r.failed, r.failed + r.passed), to: { run: r.id } });
    else out.push({ status: 'ok', text: t.testsPassedOn(r.name, r.passed), to: { run: r.id } });
  }
  const measured = new Set(p.runs.filter((r) => (r.metrics ?? 0) > 0).map((r) => r.name));
  for (const name of measured) {
    const r = decisiveOn((byName.get(name) ?? []).filter((x) => (x.metrics ?? 0) > 0));
    if (!r) {
      out.push({ status: 'warn', text: t.noRunOf(name) });
      continue;
    }
    const tally = p.tags.length ? r.release : r.previous;
    if (tally?.block) out.push({ status: 'bad', text: `${name}: ${t.status.block} ${tally.block}`, to: { run: r.id } });
    if (tally?.warn) out.push({ status: 'warn', text: `${name}: ${t.status.warn} ${tally.warn}`, to: { run: r.id } });
    if (tally?.ok) out.push({ status: 'ok', text: t.withinLimits(name, tally.ok), to: { run: r.id } });
  }
  return out;
}

function versionChecks(p: Project, v: string, t: ReturnType<typeof useT>): CheckRow[] {
  const now = semver(v);
  const last = p.tags.find((tg) => semver(tg.name));
  const prev = last && semver(last.name);
  const text = p.tags.some((tg) => tg.name === v)
    ? t.versionTaken(v)
    : !now
      ? t.versionForm(v)
      : prev && last && compare(now, prev) <= 0
        ? t.versionOlder(v, last.name)
        : t.versionOk(v);
  return [{ status: text === t.versionOk(v) ? 'ok' : 'bad', text }];
}

function noteCheck(l: string, i: number, notes: Record<string, string>, t: ReturnType<typeof useT>): CheckRow {
  return { status: notes[l]?.trim() ? 'ok' : i === 0 ? 'bad' : 'warn', text: t.notesIn(langName(l)) };
}

/** What the release changed in its requirements: per feature, its scenarios approved, changed or removed
 *  in the release's commits; per metric, the same. Proposals that weren't approved don't count. */
function requirementChanges(p: Project, features: StoryFeature[], metrics: MetricStory[], inRange: Set<string>): RequirementChange[] {
  const out: RequirementChange[] = [];
  const short = (sha: string) => p.commits[sha]?.short ?? sha.slice(0, 7);
  const judge = (events: { kind: string; commit: string }[]) => {
    const before = events.filter((e) => !inRange.has(e.commit));
    const within = events.filter(
      (e) =>
        inRange.has(e.commit) &&
        e.kind !== 'proposed' &&
        e.kind !== 'passed' &&
        e.kind !== 'failed' &&
        e.kind !== 'met' &&
        e.kind !== 'missed',
    );
    if (!within.length) return undefined;
    const agreed = before.some((e) => e.kind === 'approved' || e.kind === 'added');
    const last = within.at(-1)?.commit ?? '';
    if (within.some((e) => e.kind === 'removed')) return agreed ? { what: 'removed' as const, last } : undefined;
    if (!agreed) return within.some((e) => e.kind === 'approved' || e.kind === 'added') ? { what: 'added' as const, last } : undefined;
    return { what: 'changed' as const, last };
  };
  for (const f of features) {
    const n = { added: 0, changed: 0, removed: 0 };
    let last = '';
    for (const x of f.scenarios) {
      const j = judge(x.events);
      if (!j) continue;
      n[j.what]++;
      last = j.last;
    }
    if (n.added + n.changed + n.removed) out.push({ name: f.name, kind: 'scenarios', ...n, short: short(last) });
  }
  for (const m of metrics) {
    const j = judge(m.events);
    if (j)
      out.push({
        name: m.name || m.id,
        kind: 'metric',
        added: +(j.what === 'added'),
        changed: +(j.what === 'changed'),
        removed: +(j.what === 'removed'),
        short: short(j.last),
      });
  }
  return out;
}

/** Drafts the notes' changelog from the release's commits and requirements (changelog.ts); the person's
 *  own text above it stays. What is drafted is what the tag will hold, and what the release shows. */
function Assemble({
  p,
  rows,
  notes,
  onDraft,
}: {
  p: Project;
  rows: Row[];
  notes: Record<string, string>;
  onDraft: (n: Record<string, string>) => void;
}) {
  const features = use(loadStories(p.id));
  const metrics = use(loadMetricStories(p.id));
  const [info, setInfo] = useState('');
  const run = () => {
    const primary = p.languages[0] ?? 'en';
    // A merge's own commits are the changes; the merge commit itself is not one.
    const shas = rows.flatMap((r) => (r.merge ? (r.commits ?? []) : [r.sha]));
    const commits = shas.flatMap((sha) => {
      const c = p.commits[sha];
      return c ? [{ short: c.short, subject: c.subject, body: c.body }] : [];
    });
    const reqs = requirementChanges(p, features, metrics, new Set(rows.flatMap((r) => [r.sha, ...(r.commits ?? [])])));
    const next = { ...notes };
    let any = false;
    for (const l of p.languages) {
      const w = messages[l as Lang] ?? messages.en;
      const sections = changelog(commits, reqs, { heading: w.changelog, requirement: w.requirementChange }, l, primary);
      if (sections) any = true;
      next[l] = withChangelog(next[l] ?? '', sections);
    }
    if (!any) return setInfo(t.assembleNone);
    onDraft(next);
    setInfo('');
  };
  const t = useT();
  return (
    <span className={s.assemble}>
      {info && <span className="muted small">{info}</span>}
      <button type="button" className={s.textBtn} onClick={run}>
        ✦ {t.assemble}
      </button>
    </span>
  );
}

/** The tag's message: the notes in each language, the first one unmarked. */
const notesBody = (p: Project, notes: Record<string, string>) =>
  p.languages
    .map((l, i) => (notes[l]?.trim() ? (i ? `--- ${l}\n\n` : '') + (notes[l] ?? '').trim() : ''))
    .filter(Boolean)
    .join('\n\n');

/** Served by shipwake serve: the approval creates the tag itself, as the person. Pushing is left to them. */
function Tag({
  p,
  version,
  sha,
  notes,
  blocked,
}: {
  p: Project;
  version: string;
  sha: string;
  notes: Record<string, string>;
  blocked: boolean;
}) {
  const t = useT();
  const [state, setState] = useState<{ done?: string; error?: string }>({});
  const [busy, setBusy] = useState(false);
  const create = async () => {
    setBusy(true);
    try {
      const r = await fetch(`api/${p.id}/release`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        // the tag's message, and each language's section for its changelog file
        body: JSON.stringify({ version, commit: sha, notes: notesBody(p, notes), sections: notes }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error ?? r.statusText);
      forget(`${p.id}/index.json`);
      setState({ done: d.commit });
    } catch (e) {
      setState({ error: String((e as Error).message) });
    } finally {
      setBusy(false);
    }
  };
  if (state.done)
    return (
      <p className={s.tagged}>
        {t.tagged(version, state.done.slice(0, 7))}{' '}
        <Link to="/$project" params={{ project: p.id }}>
          {t.timeline}
        </Link>
      </p>
    );
  return (
    <div className={s.tagRow}>
      <button type="button" className={s.primary} disabled={busy || blocked} onClick={create}>
        {t.approve}
      </button>
      {state.error && <span className={s.error}>{state.error}</span>}
    </div>
  );
}

function Command({ p, version, sha, notes }: { p: Project; version: string; sha: string; notes: Record<string, string> }) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  const body = notesBody(p, notes);
  // verbatim: Git would otherwise drop lines starting with # (Markdown headings) from the notes
  const cmd = `{ cat <<'NOTES'\n${body}\nNOTES\nprintf '\\nApproved-by: %s <%s>\\n' "$(git config user.name)" "$(git config user.email)"; } | git tag -a ${version} ${sha.slice(0, 12)} --cleanup=verbatim -F -`;
  return (
    <div className={s.cmd}>
      <div className={s.cmdHead}>
        <span>{t.runInRepo}</span>
        <button type="button" onClick={() => navigator.clipboard?.writeText(cmd).then(() => setCopied(true))}>
          {copied ? t.copied : t.copy}
        </button>
      </div>
      <pre>{cmd}</pre>
    </div>
  );
}
