// The timeline: everything on the trunk, newest first in history order (order.ts): commits and merges, and
// above each the release that tags it and the runs that tested it, however long after they ran. A merge
// folds its branch's history the same way.
import { Link, useParams } from '@tanstack/react-router';
import { Fragment, type ReactNode, startTransition, use, useState } from 'react';
import { forget, loadMetricStories, loadProject, loadSession, loadStories, type MetricStory, type StoryFeature } from '../data';
import { commitLabel, day, targetName, tr, when } from '../format';
import type { Branch, Commit, Project, Row, RunSummary, Tag } from '../model.gen';
import { inHistory, type Placed } from '../order';
import { useSettings, useT } from '../settings';
import { Icon, kindIcon, Tally } from '../ui';
import { ReleaseNotes } from './notes';
import { MetricProposal, Proposal } from './Proposal';
import s from './Timeline.module.css';

export function TrunkView() {
  const { project } = useParams({ from: '/$project' });
  return <WorkspaceTimeline key={project} project={project} />;
}

/** The files a timeline reads: the project, and the requirements of the trunk or of one branch. */
const timelineFiles = (project: string, branch?: string) => ({
  project: loadProject(project),
  stories: loadStories(project, branch),
  metrics: loadMetricStories(project, branch),
});

/** The timeline's files as state: clearing a module cache cannot invalidate calls memoized by React
 *  Compiler. After a workspace change the promises are replaced, keeping other drafts mounted. */
function useTimelineFiles(project: string, branch?: string) {
  const [files, setFiles] = useState(() => timelineFiles(project, branch));
  const changed = async () => {
    const dir = branch ? `${project}/branches/${branch}` : project;
    forget(`${dir}/stories.json`);
    forget(`${dir}/metric-stories.json`);
    forget(`${project}/index.json`);
    const next = timelineFiles(project, branch);
    await Promise.all(Object.values(next));
    startTransition(() => setFiles(next));
  };
  return { p: use(files.project), stories: use(files.stories), metrics: use(files.metrics), changed };
}

/** What waits for a decision on a branch, for the top of its timeline; nothing when nothing does.
 *  Each branch shows its own requirements, and they can be changed only where they are checked out. */
function pending(
  project: string,
  branch: string,
  stories: StoryFeature[],
  metrics: MetricStory[],
  onChange: () => Promise<void>,
): ReactNode {
  const features = stories.filter((f) => f.scenarios.some((x) => x.state === 'proposed'));
  const proposed = metrics.filter((m) => m.state === 'proposed');
  if (features.length + proposed.length === 0) return undefined;
  return <Pending project={project} branch={branch} features={features} metrics={proposed} onChange={onChange} />;
}

function Pending({
  project,
  branch,
  features,
  metrics: proposed,
  onChange,
}: {
  project: string;
  branch: string;
  features: StoryFeature[];
  metrics: MetricStory[];
  onChange: () => Promise<void>;
}) {
  const session = use(loadSession());
  const here = session.checkedOut?.[project] === branch;
  const edit = session.edit && here;
  const on = { name: branch, elsewhere: session.edit && !here };
  return (
    <>
      {features.map((f) => (
        <Proposal
          key={f.path}
          project={project}
          branch={on}
          f={f}
          proposed={f.scenarios.filter((x) => x.state === 'proposed')}
          edit={edit}
          onChange={onChange}
          folding
        />
      ))}
      {proposed.map((m) => (
        <MetricProposal key={m.id} project={project} branch={on} m={m} edit={edit} onChange={onChange} />
      ))}
    </>
  );
}

function WorkspaceTimeline({ project }: { project: string }) {
  const { p, stories, metrics, changed } = useTimelineFiles(project);
  const t = useT();
  const trunk = new Set(p.rows.map((r) => r.sha));
  const entries = inHistory(
    p.rows.map((r) => r.sha),
    [
      ...p.rows.map((r) => ({
        commit: r.sha,
        kind: 'commit' as const,
        at: time(p.commits[r.sha]?.date),
        key: r.sha,
        show: (place: Place) => <RowItem p={p} r={r} line={place === 'line'} />,
      })),
      ...runEntries(p, trunk),
      ...p.tags
        .filter((tag) => trunk.has(tag.sha))
        .map((tag) => ({
          commit: tag.sha,
          kind: 'release' as const,
          at: time(tag.date),
          key: `tag:${tag.name}`,
          release: true,
          show: () => <ReleaseCard p={p} tag={tag} index={p.rows.findIndex((r) => r.sha === tag.sha)} />,
        })),
    ],
  );
  return (
    <>
      <header className={`${s.head} ${s.headRow}`}>
        <div className="grow">
          <h1 className="mono">{p.trunk}</h1>
          <div className="meta">
            <span>{t.defaultBranch}</span>
            <span>{t.merges(p.rows.filter((r) => r.merge).length)}</span>
            {p.tags[0] && <span>{t.latestRelease(p.tags[0].name)}</span>}
          </div>
        </div>
        <Link className={s.textBtn} to="/$project/release" params={{ project }}>
          <Icon name="tag" />
          {t.newRelease}
        </Link>
      </header>
      {/* What waits for a decision comes first; the latest entry then joins the line like the others. */}
      <Entries entries={entries} top={pending(project, p.trunk, stories, metrics, changed)} />
    </>
  );
}

export function BranchView() {
  const { project, _splat } = useParams({ from: '/$project/branch/$' });
  const p = use(loadProject(project));
  const t = useT();
  const b = p.branches.find((x) => x.name === _splat);
  if (!b) return <p className="muted">{t.notFound}</p>;
  return <BranchTimeline key={`${project}:${b.name}`} project={project} shown={b} />;
}

function BranchTimeline({ project, shown }: { project: string; shown: Branch }) {
  const { p, stories, metrics, changed } = useTimelineFiles(project, shown.key);
  const t = useT();
  // as of the newest files: an approval made here adds a commit to the branch
  const b = p.branches.find((x) => x.name === shown.name) ?? shown;
  const base = p.commits[b.base];
  const entries = inHistory(b.ahead, [
    ...b.ahead.map((sha) => ({
      commit: sha,
      kind: 'commit' as const,
      at: time(p.commits[sha]?.date),
      key: sha,
      show: (place: Place) => <CommitItem p={p} c={p.commits[sha]} line={place === 'line'} />,
    })),
    ...runEntries(p, new Set(b.ahead)),
  ]);
  return (
    <>
      <header className={s.head}>
        <h1 className="mono">{b.name}</h1>
        <div className="meta">
          <span>{t.ahead(b.ahead.length, p.trunk)}</span>
          <span>{t.behind(b.behind, p.trunk)}</span>
        </div>
      </header>
      <Entries
        entries={entries}
        top={pending(project, b.name, stories, metrics, changed)}
        end={
          base && (
            <div className={s.end}>
              <span className={s.endDot} />
              {t.branchedFrom(p.trunk)} <code>{base.short}</code> ·{' '}
              <Link className="lnk" to="/$project/commit/$sha" params={{ project, sha: base.sha }}>
                {base.subject}
              </Link>
            </div>
          )
        }
      />
    </>
  );
}

/** Something that happened: a commit, a run, a release. Shown on the line, or in the top card. */
interface Entry extends Placed {
  key: string;
  release?: boolean;
  show: (place: Place) => ReactNode;
}

/** Where an entry is shown: on the timeline's line, as the top card, or folded in a merge (compact). */
type Place = 'line' | 'card' | 'fold';

const time = (iso?: string) => (iso ? Date.parse(iso) : 0);

/** Runs on these commits; a run with both tests and measurements is two entries, each with its counts. */
function runEntries(p: Project, shas: Set<string>): Entry[] {
  return p.runs
    .filter((r) => shas.has(r.commit))
    .flatMap((r) =>
      runParts(r).map((part) => ({
        commit: r.commit,
        kind: 'run' as const,
        at: time(r.start),
        key: `${r.id}:${part}`,
        show: (place: Place) =>
          place === 'fold' ? <RunLine p={p} r={r} part={part} /> : <RunItem p={p} r={r} part={part} line={place === 'line'} />,
      })),
    );
}

const runParts = (r: RunSummary): ('tests' | 'metrics')[] => {
  const parts: ('tests' | 'metrics')[] = [];
  if (r.passed + r.failed + r.skipped > 0 || !r.metrics) parts.push('tests');
  if (r.metrics) parts.push('metrics');
  return parts;
};

/** The newest entry is the top card, whatever it is; the rest follow on the line, newest first. */
function Entries({ entries, end, top }: { entries: Entry[]; end?: ReactNode; top?: ReactNode }) {
  // Something on top (pending proposals) takes the first entry's place; that entry joins the line.
  const first = top ? undefined : entries[0];
  const rest = top ? entries : entries.slice(1);
  return (
    <>
      {top}
      {first && (first.release ? first.show('card') : <section className={s.latest}>{first.show('card')}</section>)}
      <div className={s.tl}>
        {rest.map((e) => (
          <Fragment key={e.key}>{e.show('line')}</Fragment>
        ))}
        {end}
      </div>
    </>
  );
}

function RowItem({ p, r, line }: { p: Project; r: Row; line: boolean }) {
  const { lang } = useSettings();
  const t = useT();
  const c = p.commits[r.sha];
  if (!c) return null;
  if (!r.merge) return <CommitItem p={p} c={c} line={line} />;
  const own = (r.commits ?? []).map((sha) => p.commits[sha]).filter((x): x is Commit => !!x);
  // The branch's history, folded: its commits and the runs on them, newest first.
  const branch = inHistory(
    own.map((x) => x.sha),
    [
      ...own.map((x) => ({ commit: x.sha, kind: 'commit' as const, at: time(x.date), key: x.sha, show: () => <FoldCommit p={p} c={x} /> })),
      ...runEntries(p, new Set(own.map((x) => x.sha))),
    ],
  );
  return (
    <div className={`${s.item} ${s.merge}`}>
      {line && <span className={s.dot} />}
      <div className={s.title}>
        <Link className="lnk" to="/$project/commit/$sha" params={{ project: p.id, sha: c.sha }}>
          {c.subject}
        </Link>
      </div>
      <div className="meta">
        {r.branch && <code>{r.branch}</code>}
        <code>{c.short}</code>
        <span>{when(c.date, lang)}</span>
        {!!c.scenarios && <span>{t.scenarios(c.scenarios)}</span>}
      </div>
      {own.length > 0 && (
        <details className={`${s.more} ${s.toggle}`}>
          <summary>{t.onBranch(own.filter((x) => x.parents.length < 2).length)}</summary>
          <div className={s.commits}>
            {branch.map((e) => (
              <Fragment key={e.key}>{e.show('fold')}</Fragment>
            ))}
          </div>
        </details>
      )}
    </div>
  );
}

function CommitItem({ p, c, line }: { p: Project; c?: Commit; line: boolean }) {
  const { lang } = useSettings();
  const t = useT();
  if (!c) return null;
  const sync = c.parents.length > 1;
  return (
    <div className={`${s.item} ${sync ? s.sync : ''}`}>
      {line && <span className={s.dot} />}
      <div className={sync ? '' : s.title}>
        <Link className="lnk" to="/$project/commit/$sha" params={{ project: p.id, sha: c.sha }}>
          {c.subject}
        </Link>
      </div>
      <div className="meta">
        <code>{c.short}</code>
        <span>{when(c.date, lang)}</span>
        {!!c.scenarios && <span>{t.scenarios(c.scenarios)}</span>}
      </div>
    </div>
  );
}

/** A run's status for one part: its class and counts. Measurements are judged against the last release when
 *  the run has a run there, else the previous run; without thresholds nothing is judged, and no color shows. */
function verdict(r: RunSummary, part: 'tests' | 'metrics') {
  const measured = part === 'metrics';
  const tally = r.hasRelease ? r.release : r.previous;
  const bad = measured ? !!tally?.block : r.failed > 0;
  const warn = measured && !bad && !!tally?.warn;
  const unjudged = measured && !bad && !warn && !tally?.ok;
  return {
    cls: bad ? s.bad : warn ? s.warnRun : unjudged ? s.unjudged : '',
    unjudged,
    tally: measured ? (
      <Tally failed={tally?.block} warn={tally?.warn} passed={tally?.ok} measured />
    ) : (
      <Tally failed={r.failed} passed={r.passed} />
    ),
  };
}

/** A run: what ran, its counts, and when. On the line its icon is the marker; in the top card it leads the title. */
function RunItem({ p, r, part, line }: { p: Project; r: RunSummary; part: 'tests' | 'metrics'; line: boolean }) {
  const { lang } = useSettings();
  const t = useT();
  const v = verdict(r, part);
  const icon = (
    <span className={line ? s.runDot : s.runIc}>
      <Icon name={kindIcon(p.kinds?.[r.kind]?.icon, r.kind, part)} size={15} />
    </span>
  );
  return (
    <div className={`${s.item} ${s.run} ${v.cls}`}>
      {line && icon}
      <div className={s.title}>
        {!line && icon}
        <Link className="lnk" to="/$project/run/$run" params={{ project: p.id, run: r.id }}>
          <b>{tr(p, r.name, lang)}</b>
        </Link>
        {v.tally}
      </div>
      <div className="meta">
        {/* the commit it tested: runs sit where they ran, which may be long after (nightly, backfilled) */}
        <Link to="/$project/commit/$sha" params={{ project: p.id, sha: r.commit }}>
          <code>{commitLabel(p, r.commit)}</code>
        </Link>
        {r.targets && r.targets.length > 0 && <span>{r.targets.map((x) => targetName(p, x, lang)).join(', ')}</span>}
        {r.context?.model && <span>{r.context.model}</span>}
        {v.unjudged && !!r.metrics && <span>{t.measurements(r.metrics)}</span>}
        <span>{when(r.start, lang)}</span>
        {part === 'tests' && r.duration > 0 && <span>{t.secs(r.duration)}</span>}
      </div>
    </div>
  );
}

/** A run inside a merge's fold: one compact line with its counts and time. */
function RunLine({ p, r, part }: { p: Project; r: RunSummary; part: 'tests' | 'metrics' }) {
  const { lang } = useSettings();
  const t = useT();
  const v = verdict(r, part);
  return (
    <div className={`${s.runLine} ${v.cls}`}>
      <span className={s.lineIc}>
        <Icon name={kindIcon(p.kinds?.[r.kind]?.icon, r.kind, part)} size={14} />
      </span>
      <Link className={`lnk ${s.runName}`} to="/$project/run/$run" params={{ project: p.id, run: r.id }}>
        {tr(p, r.name, lang)}
      </Link>
      {v.tally}
      <span className={s.runMeta}>
        <code>{commitLabel(p, r.commit)}</code>
        {r.context?.model && <span>{r.context.model}</span>}
        {v.unjudged && !!r.metrics && <span>{t.measurements(r.metrics)}</span>}
        <span>{when(r.start, lang)}</span>
        {part === 'tests' && r.duration > 0 && <span>{t.secs(r.duration)}</span>}
      </span>
    </div>
  );
}

/** A branch commit inside a merge's fold: its subject in the regular weight, then its hash and time. */
function FoldCommit({ p, c }: { p: Project; c: Commit }) {
  const { lang } = useSettings();
  return (
    <div className={`${s.foldCommit} ${c.parents.length > 1 ? s.sync : ''}`}>
      <Link className="lnk" to="/$project/commit/$sha" params={{ project: p.id, sha: c.sha }}>
        {c.subject}
      </Link>
      <div className="meta">
        <code>{c.short}</code>
        <span>{when(c.date, lang)}</span>
      </div>
    </div>
  );
}

/** A release: its public notes in the viewer's language, then the changes it contains. */
function ReleaseCard({ p, tag, index }: { p: Project; tag: Tag; index: number }) {
  const { lang } = useSettings();
  const t = useT();
  const tagAt = new Set(p.tags.map((x) => x.sha));
  const next = p.rows.findIndex((x, j) => j > index && tagAt.has(x.sha));
  const span = p.rows.slice(index, next < 0 ? p.rows.length : next);
  // What the release says is what its tag holds: the notes, changelog included, nothing derived here.
  return (
    <section className={s.release}>
      <span className={s.releaseDot}>
        <Icon name="tag" />
      </span>
      <div className={s.cardHead}>
        <b className={s.version}>{tag.name}</b>
        <span className={s.when}>
          {day(tag.date, lang)} · {t.merges(span.filter((r) => r.merge).length)}
        </span>
      </div>
      <ReleaseNotes p={p} tag={tag} />
    </section>
  );
}
