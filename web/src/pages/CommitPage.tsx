// A commit: what it is, which scenarios it changed, then its files.
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { type CSSProperties, type PointerEvent, type ReactNode, Suspense, use, useRef, useState } from 'react';
import { loadCommit, loadDiffs, loadProject } from '../data';
import { full, tr } from '../format';
import type { FileChange, FileDiff, ScenarioChange } from '../model.gen';
import { useSettings, useT } from '../settings';
import { Icon, Segmented } from '../ui';
import s from './CommitPage.module.css';
import { FileTree } from './FileTree';

export function CommitPage() {
  const { project, sha } = useParams({ from: '/$project/commit/$sha' });
  const p = use(loadProject(project));
  const d = use(loadCommit(project, sha));
  const { lang } = useSettings();
  const t = useT();
  const router = useRouter();
  const c = p.commits[sha];
  if (!c) return <p className="muted">{t.notFound}</p>;
  const added = d.files.reduce((a, f) => a + (f.added ?? 0), 0);
  const deleted = d.files.reduce((a, f) => a + (f.deleted ?? 0), 0);
  return (
    <>
      <button type="button" className="back" onClick={() => router.history.back()}>
        ‹ {t.back}
      </button>
      <h1 className={s.title}>{c.subject}</h1>
      {c.body && <p className={s.body}>{c.body}</p>}
      <dl className="facts">
        <dt>{t.commit}</dt>
        <dd>
          <code>{c.sha}</code>
        </dd>
        <dt>{t.author}</dt>
        <dd>
          {c.author} · {full(c.date, lang)}
        </dd>
        {c.parents.length > 0 && (
          <>
            <dt>{t.parents}</dt>
            <dd className={s.parents}>
              {c.parents.map((x) =>
                p.commits[x] ? (
                  <Link key={x} to="/$project/commit/$sha" params={{ project, sha: x }}>
                    <code>{x.slice(0, 7)}</code>
                  </Link>
                ) : (
                  <code key={x}>{x.slice(0, 7)}</code>
                ),
              )}
            </dd>
          </>
        )}
        <dt>{t.releasedIn}</dt>
        <dd>{d.tags.length ? d.tags.join(', ') : <span className="muted">{t.notReleased}</span>}</dd>
      </dl>
      {d.scenarios.length > 0 && <Scenarios list={d.scenarios} />}
      <Files key={sha} project={project} sha={sha} files={d.files} added={added} deleted={deleted} />
    </>
  );
}

const stored = (key: string) => {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
};
const store = (key: string, value: string | null) => {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // private mode: the choice lasts for this page only
  }
};

/** The changed files: a tree to find them, beside their diffs. Small commits open every diff. */
function Files({
  project,
  sha,
  files,
  added,
  deleted,
}: {
  project: string;
  sha: string;
  files: FileChange[];
  added: number;
  deleted: number;
}) {
  const t = useT();
  const [query, setQuery] = useState('');
  const [active, setActive] = useState<string>();
  const [pane, setPane] = useState<'files' | 'changes'>('changes');
  const [treeOff, setTreeOff] = useState(() => stored('treeOff') === '1');
  const [width, setWidth] = useState(() => Number(stored('treeWidth')) || 0);
  const [open, setOpen] = useState<Set<string>>(
    () => new Set(files.length <= 40 && added + deleted <= 3000 ? files.map((f) => f.path) : []),
  );
  const layout = useRef<HTMLDivElement>(null);
  const q = query.trim().toLowerCase();
  const shown = q ? files.filter((f) => f.path.toLowerCase().includes(q)) : files;

  const pick = (path: string) => {
    setActive(path);
    setPane('changes');
    setOpen((prev) => new Set(prev).add(path));
    requestAnimationFrame(() => document.getElementById(fileId(files, path))?.scrollIntoView({ block: 'start' }));
  };
  const toggleOpen = (path: string, isOpen: boolean) =>
    setOpen((prev) => {
      if (prev.has(path) === isOpen) return prev;
      const next = new Set(prev);
      if (isOpen) next.add(path);
      else next.delete(path);
      return next;
    });
  const resize = (w: number) => {
    const max = (layout.current?.clientWidth ?? 1000) * 0.6;
    const next = Math.round(Math.max(180, Math.min(w, max)));
    setWidth(next);
    store('treeWidth', String(next));
  };
  const drag = (e: PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const el = e.currentTarget;
    el.setPointerCapture(e.pointerId);
    const left = layout.current?.getBoundingClientRect().left ?? 0;
    layout.current?.classList.add(s.dragging as string);
    el.onpointermove = (m) => resize(m.clientX - left - 6);
    el.onpointerup = el.onpointercancel = () => {
      el.onpointermove = null;
      layout.current?.classList.remove(s.dragging as string);
    };
  };
  const style = width ? ({ '--tw': `${width}px` } as CSSProperties) : undefined;

  return (
    <>
      <div className={s.bar}>
        <button
          type="button"
          className={s.treeBtn}
          aria-pressed={!treeOff}
          title={t.toggleTree}
          aria-label={t.toggleTree}
          onClick={() => {
            setTreeOff(!treeOff);
            store('treeOff', treeOff ? null : '1');
          }}
        >
          <Icon name="panel" />
        </button>
        <b>{t.filesChanged}</b>
        <span className="pill">{files.length}</span>
        <span className={s.sum}>
          <span className="add">+{added.toLocaleString()}</span>
          <span className="del">-{deleted.toLocaleString()}</span>
          <Blocks add={added} del={deleted} />
        </span>
      </div>
      <div className={s.paneSwitch}>
        <Segmented
          value={pane}
          onChange={setPane}
          options={[
            { value: 'files', label: t.paneFiles },
            { value: 'changes', label: t.paneChanges },
          ]}
        />
      </div>
      <div ref={layout} className={`${s.layout} ${treeOff ? s.treeOff : ''}`} data-pane={pane} style={style}>
        <aside className={s.tree}>
          <label className={s.filter}>
            <Icon name="search" size={15} />
            <input
              value={query}
              placeholder={t.filterFiles}
              aria-label={t.filterFiles}
              autoComplete="off"
              onChange={(e) => setQuery(e.target.value)}
            />
          </label>
          {shown.length ? <FileTree files={shown} active={active} onPick={pick} /> : <p className="muted small">{t.noFileMatch}</p>}
        </aside>
        {/* biome-ignore lint/a11y/useSemanticElements: a draggable splitter has no native element */}
        <div
          className={s.divider}
          role="separator"
          aria-orientation="vertical"
          aria-label={t.resizeTree}
          aria-valuenow={width || undefined}
          tabIndex={0}
          title={t.resizeTree}
          onPointerDown={drag}
          onDoubleClick={() => {
            setWidth(0);
            store('treeWidth', null);
          }}
          onKeyDown={(e) => {
            const w = width || 300;
            if (e.key === 'ArrowLeft') resize(w - 20);
            if (e.key === 'ArrowRight') resize(w + 20);
          }}
        />
        <div className={`list ${s.files}`}>
          {shown.map((f) => (
            <FileRow
              key={f.path}
              id={fileId(files, f.path)}
              project={project}
              sha={sha}
              f={f}
              open={open.has(f.path)}
              onToggle={toggleOpen}
            />
          ))}
        </div>
      </div>
    </>
  );
}

const fileId = (files: FileChange[], path: string) => `file-${files.findIndex((f) => f.path === path)}`;

function Scenarios({ list }: { list: ScenarioChange[] }) {
  const { project } = useParams({ from: '/$project/commit/$sha' });
  const p = use(loadProject(project));
  const { lang } = useSettings();
  const t = useT();
  const features = [...new Set(list.map((x) => x.feature))];
  const n = (k: string) => list.filter((x) => x.change === k).length;
  return (
    <>
      <div className={s.bar}>
        <Icon name="beaker" />
        <b>{t.scenariosChanged}</b>
        <span className="pill">{list.length}</span>
        <span className={s.sum}>
          {n('added') > 0 && <span className="add">+{n('added')}</span>}
          {n('changed') > 0 && <span className="mod">~{n('changed')}</span>}
          {n('removed') > 0 && <span className="del">−{n('removed')}</span>}
        </span>
      </div>
      <div className={`list ${s.scenarios}`}>
        {features.map((f) => (
          <div key={f}>
            <div className="groupHead">{tr(p, f, lang)}</div>
            {list
              .filter((x) => x.feature === f)
              .map((x) => (
                <details key={x.title} className="row">
                  <summary>
                    <span className={`change ${x.change}`}>{t.change[x.change]}</span>
                    <span className="grow">{tr(p, x.title, lang)}</span>
                    <span className="num">
                      {x.steps.length} {t.steps}
                    </span>
                  </summary>
                  <div className={s.steps}>
                    {x.steps.map((st) => (
                      <div key={st} className={x.was && !x.was.includes(st) ? s.new : ''}>
                        {stepText(st, (k) => tr(p, k, lang))}
                      </div>
                    ))}
                    {x.was
                      ?.filter((st) => !x.steps.includes(st))
                      .map((st) => (
                        <div key={`was-${st}`} className={s.old}>
                          {stepText(st, (k) => tr(p, k, lang))}
                        </div>
                      ))}
                  </div>
                </details>
              ))}
          </div>
        ))}
      </div>
    </>
  );
}

/** A step in the viewer's language: the keyword and the step text, each translated when the project provides it. */
export function stepText(step: string, tx: (s: string) => string) {
  const m = step.match(/^(Given|When|Then|And|But|\*) (.*)$/);
  if (!m) return tx(step);
  const body = tx(m[2] ?? '');
  return body === m[2] ? step : `${tx(m[1] ?? '')} ${body}`;
}

function FileRow({
  id,
  project,
  sha,
  f,
  open,
  onToggle,
}: {
  id: string;
  project: string;
  sha: string;
  f: FileChange;
  open: boolean;
  onToggle: (path: string, open: boolean) => void;
}) {
  return (
    <details id={id} className={`row ${s.file}`} open={open} onToggle={(e) => onToggle(f.path, (e.target as HTMLDetailsElement).open)}>
      <summary>
        <span className={`fst ${f.status}`}>{f.status}</span>
        <span className={`grow mono ${s.path}`}>
          {f.old && <span className="muted">{f.old} → </span>}
          {f.path}
        </span>
        {f.added != null && (
          <span className={s.sum}>
            {f.added > 0 && <span className="add">+{f.added}</span>}
            {(f.deleted ?? 0) > 0 && <span className="del">-{f.deleted}</span>}
            <Blocks add={f.added} del={f.deleted ?? 0} />
          </span>
        )}
      </summary>
      {open && (
        <Suspense fallback={<div className={s.note}>…</div>}>
          <Diff project={project} sha={sha} path={f.path} />
        </Suspense>
      )}
    </details>
  );
}

function Diff({ project, sha, path }: { project: string; sha: string; path: string }) {
  const diffs = use(loadDiffs(project, sha));
  const t = useT();
  const d: FileDiff | undefined = diffs[path];
  if (!d) return null;
  if (d.binary) return <div className={s.note}>{t.binary}</div>;
  if (d.skipped) return <div className={s.note}>{t.skipped(d.skipped)}</div>;
  let o = 0;
  let n = 0;
  const hl = d.hl;
  const rows = (d.lines ?? []).map((l, i) => {
    const key = `${i}`;
    if (l.startsWith('@@')) {
      const m = l.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)/);
      if (m) [o, n] = [Number(m[1]), Number(m[2])];
      return (
        <tr key={key} className={s.hunk}>
          <td className={s.ln} colSpan={2}>
            ⋯
          </td>
          <td className={s.cd}>{l}</td>
        </tr>
      );
    }
    if (l.startsWith('\\')) return null;
    const k = l[0];
    const text = <Code text={l.slice(1)} hl={hl?.[i]} />;
    if (k === '+')
      return (
        <tr key={key} className={s.add}>
          <td className={s.ln} />
          <td className={s.ln}>{n++}</td>
          <td className={s.cd}>
            <b>+</b>
            {text}
          </td>
        </tr>
      );
    if (k === '-')
      return (
        <tr key={key} className={s.del}>
          <td className={s.ln}>{o++}</td>
          <td className={s.ln} />
          <td className={s.cd}>
            <b>-</b>
            {text}
          </td>
        </tr>
      );
    return (
      <tr key={key}>
        <td className={s.ln}>{o++}</td>
        <td className={s.ln}>{n++}</td>
        <td className={s.cd}>
          <b> </b>
          {text}
        </td>
      </tr>
    );
  });
  return (
    <div className={s.diffWrap} data-horizontal-scroll>
      {/* line-number columns as wide as this file's largest number */}
      <table className={s.diff} style={{ '--ln': `${String(Math.max(o, n, 1)).length}ch` } as CSSProperties}>
        <tbody>{rows}</tbody>
      </table>
      {!!d.more && <div className={s.note}>{t.moreLines(d.more)}</div>}
    </div>
  );
}

/** One line of code, colored by its token classes and lengths ("k3_1f4"), worked out at build time. */
function Code({ text, hl }: { text: string; hl?: string }) {
  if (!hl) return text;
  const parts: ReactNode[] = [];
  let at = 0;
  for (const m of hl.matchAll(/([a-z_])(\d+)/g)) {
    const len = Number(m[2]);
    const piece = text.slice(at, at + len);
    parts.push(
      m[1] === '_' ? (
        piece
      ) : (
        <span key={at} className={`tk-${m[1]}`}>
          {piece}
        </span>
      ),
    );
    at += len;
  }
  if (at < text.length) parts.push(text.slice(at));
  return parts;
}

/** GitHub-style five blocks: green for additions, red for deletions, grey for the rest. */
function Blocks({ add, del }: { add: number; del: number }) {
  const total = add + del;
  const g = total ? Math.floor((5 * add) / total) : 0;
  const r = total ? Math.floor((5 * del) / total) : 0;
  return (
    <span className={s.blocks}>
      {['a', 'b', 'c', 'd', 'e'].map((k, i) => (
        <i key={k} className={i < g ? s.g : i < g + r ? s.r : ''} />
      ))}
    </span>
  );
}
