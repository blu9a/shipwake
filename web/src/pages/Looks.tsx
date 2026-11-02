// Visual review: a commit's screenshots compared with the looks approved for them. On the commit, the ones
// that look different come first and the rest wait folded; a look's page compares the approved look and the
// new one side by side or one over the other, the changed pixels highlighted, and approves it.
import { Link, useParams, useRouter } from '@tanstack/react-router';
import {
  type CSSProperties,
  type PointerEvent,
  type ReactNode,
  Suspense,
  startTransition,
  use,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { diffUrl, fileUrl, forget, loadLooks, loadProject, loadSession } from '../data';
import { full, tr } from '../format';
import type { Look, Project } from '../model.gen';
import { useSettings, useT } from '../settings';
import { Icon, Segmented } from '../ui';
import cs from './CommitPage.module.css';
import s from './Looks.module.css';
import rs from './ReleasePage.module.css';

/** Looks of one subject, in the order they were recorded. */
function bySubject(looks: Look[]) {
  const groups = new Map<string, Look[]>();
  for (const l of looks) groups.set(l.subject, [...(groups.get(l.subject) ?? []), l]);
  return [...groups.values()];
}
const pending = (l: Look) => l.status !== 'same';
/** A subject's state: changed when one of its variants changed, else new when one is new. */
const groupStatus = (g: Look[]) => (g.some((l) => l.status === 'changed') ? 'changed' : g.some((l) => l.status === 'new') ? 'new' : 'same');
const chipClass: Record<string, string> = { changed: 'changed', new: 'added', same: 'same' };

/** The commit's looks, when its runs recorded any. */
export function CommitLooks({ project, sha }: { project: string; sha: string }) {
  const p = use(loadProject(project));
  if (!p.looks?.[sha]) return null;
  return (
    <Suspense fallback={null}>
      <LookList project={project} sha={sha} />
    </Suspense>
  );
}

function LookList({ project, sha }: { project: string; sha: string }) {
  const ls = use(loadLooks(project, sha));
  const t = useT();
  const groups = bySubject(ls.looks);
  const review = groups.filter((g) => g.some(pending));
  const same = groups.filter((g) => !g.some(pending));
  const n = (st: string) => ls.looks.filter((l) => l.status === st).length;
  return (
    <section aria-label={t.looks}>
      <div className={cs.bar}>
        <Icon name="image" />
        <b>{t.looks}</b>
        <span className="pill">{ls.looks.length}</span>
        <span className={cs.sum}>
          {n('changed') > 0 && <span className="mod">~{n('changed')}</span>}
          {n('new') > 0 && <span className="add">+{n('new')}</span>}
        </span>
      </div>
      <div className={`list ${s.looks}`}>
        {review.map((g) => (
          <Subject key={g[0]?.subject} project={project} sha={sha} looks={g} />
        ))}
        {same.length > 0 && (
          <details className={`row ${s.same}`}>
            <summary>
              <span className="grow muted">{t.looksSame(same.reduce((a, g) => a + g.length, 0))}</span>
              <span className={s.chev} aria-hidden="true">
                <Icon name="down" size={14} />
              </span>
            </summary>
            {same.map((g) => (
              <Subject key={g[0]?.subject} project={project} sha={sha} looks={g} />
            ))}
          </details>
        )}
      </div>
    </section>
  );
}

/** One subject: its state and name, then its variants together, the changed ones marked. */
function Subject({ project, sha, looks }: { project: string; sha: string; looks: Look[] }) {
  const p = use(loadProject(project));
  const { lang } = useSettings();
  const t = useT();
  const first = looks.find(pending) ?? looks[0];
  if (!first) return null;
  const st = groupStatus(looks);
  const count = (st: string) => looks.filter((l) => l.status === st).length;
  return (
    <div className={s.subject} data-look-subject={first.subject}>
      <div className={s.subjectHead}>
        {st !== 'same' && <span className={`change ${chipClass[st]}`}>{t.lookStatus[st]}</span>}
        <Link className={`lnk grow ${s.name}`} to="/$project/look/$sha/$look" params={{ project, sha, look: first.id }}>
          {tr(p, first.subject, lang)}
        </Link>
        {looks.length > 1 && st !== 'same' && (
          <span className="muted small">
            {[
              count('changed') ? t.variantsChanged(count('changed'), looks.length) : '',
              count('new') ? t.variantsNew(count('new'), looks.length) : '',
            ]
              .filter(Boolean)
              .join(' · ')}
          </span>
        )}
      </div>
      <Strip project={project} sha={sha} looks={looks} />
    </div>
  );
}

/** A subject's variants side by side: each one's screenshot and name, marked when it changed or is new. */
function Strip({ project, sha, looks, current }: { project: string; sha: string; looks: Look[]; current?: string }) {
  const p = use(loadProject(project));
  const { lang } = useSettings();
  const t = useT();
  const strip = useRef<HTMLDivElement>(null);
  // The current variant is in view, without moving the page.
  useLayoutEffect(() => {
    const box = strip.current;
    const on = current ? box?.querySelector<HTMLElement>(`[data-look-id="${current}"]`) : undefined;
    if (!box || !on) return;
    const left = on.offsetLeft - box.offsetLeft;
    if (left < box.scrollLeft || left + on.offsetWidth > box.scrollLeft + box.clientWidth)
      box.scrollLeft = left - (box.clientWidth - on.offsetWidth) / 2;
  }, [current]);
  return (
    <div ref={strip} className={s.strip} data-horizontal-scroll>
      {looks.map((l) => (
        <Link
          key={l.id}
          className={`${s.shot} ${s[l.status] ?? ''} ${l.id === current ? s.current : ''}`}
          to="/$project/look/$sha/$look"
          params={{ project, sha, look: l.id }}
          replace={current !== undefined}
          aria-current={l.id === current ? 'page' : undefined}
          title={`${l.name} · ${t.lookStatus[l.status]}`}
          data-look={l.name}
          data-look-id={l.id}
          data-look-status={l.status}
        >
          <span className={s.thumb}>
            <img loading="lazy" src={fileUrl(project, l.run, l.path)} alt={l.name} />
          </span>
          <span className={s.cap}>
            <i className={s.dot} />
            {l.variant && <span className={s.capText}>{tr(p, l.variant, lang)}</span>}
            {(pending(l) || !l.variant) && <span className={s.capStatus}>{t.lookStatus[l.status]}</span>}
          </span>
        </Link>
      ))}
    </div>
  );
}

export function LookPage() {
  const { project, sha } = useParams({ from: '/$project/look/$sha/$look' });
  return <LookView key={`${project}/${sha}`} project={project} sha={sha} />;
}

/** The page's files as state, so an approval can replace them with the rebuilt ones (see DESIGN.md). */
function useLookFiles(project: string, sha: string) {
  const [files, setFiles] = useState(() => ({ p: loadProject(project), looks: loadLooks(project, sha) }));
  const changed = async () => {
    forget(`${project}/index.json`);
    forget(`${project}/looks/${sha}.json`);
    const next = { p: loadProject(project), looks: loadLooks(project, sha) };
    await Promise.all([next.p, next.looks]);
    startTransition(() => setFiles(next));
  };
  return { p: use(files.p), ls: use(files.looks), changed };
}

function LookView({ project, sha }: { project: string; sha: string }) {
  const { look } = useParams({ from: '/$project/look/$sha/$look' });
  const { p, ls, changed } = useLookFiles(project, sha);
  const { lang } = useSettings();
  const t = useT();
  const router = useRouter();
  const lk = ls.looks.find((l) => l.id === look);
  if (!lk) return <p className="muted">{t.notFound}</p>;
  const variants = ls.looks.filter((l) => l.subject === lk.subject);
  const c = p.commits[sha];
  const run = p.runs.find((r) => r.id === lk.run);
  return (
    <>
      <button type="button" className="back" onClick={() => router.history.back()}>
        ‹ {t.back}
      </button>
      <h1 className={s.title}>{tr(p, lk.subject, lang)}</h1>
      <dl className="facts">
        <dt>{t.commit}</dt>
        <dd>
          <Link to="/$project/commit/$sha" params={{ project, sha }}>
            <code>{c?.short ?? sha.slice(0, 7)}</code>
          </Link>{' '}
          {c?.subject}
        </dd>
        {run && (
          <>
            <dt>{t.recordedBy}</dt>
            <dd>
              <Link className="lnk" to="/$project/run/$run" params={{ project, run: run.id }}>
                {tr(p, run.name, lang)}
              </Link>
            </dd>
          </>
        )}
      </dl>
      {variants.length > 1 && (
        <nav className={s.variants} aria-label={t.variants}>
          <Strip project={project} sha={sha} looks={variants} current={lk.id} />
        </nav>
      )}
      <Verdict p={p} lk={lk} />
      <Compare key={lk.id} project={project} lk={lk} />
      <Approve key={`approve-${lk.id}`} p={p} sha={sha} lk={lk} variants={variants} onApproved={changed} />
    </>
  );
}

/** What the comparison found, and the approval it was compared with. */
function Verdict({ p, lk }: { p: Project; lk: Look }) {
  const { lang } = useSettings();
  const t = useT();
  const a = lk.approved;
  return (
    <div className={s.verdict} data-look-status={lk.status}>
      <div className={s.verdictLine}>
        <span className={lk.status === 'same' ? 'stChip ok' : `change ${chipClass[lk.status]}`}>{t.lookStatus[lk.status]}</span>
        {lk.variant && <span className={s.variant}>{tr(p, lk.variant, lang)}</span>}
        <span className="grow">
          {lk.status === 'changed' ? t.pixelsDiffer(lk.pixels ?? 0) : lk.status === 'new' ? t.nothingToCompare : t.asApproved}
        </span>
      </div>
      {a && (
        <div className={`muted small ${s.approvedBy}`}>
          <Icon name="check" size={14} />
          {t.approvedOn(a.by.replace(/\s*<[^>]*>$/, ''), p.commits[a.commit]?.short ?? a.commit.slice(0, 7), full(a.at, lang))}
        </div>
      )}
    </div>
  );
}

type Mode = 'side' | 'slider' | 'onion';
const readMode = (): Mode => {
  try {
    const m = localStorage.getItem('lookMode');
    return m === 'slider' || m === 'onion' ? m : 'side';
  } catch {
    return 'side';
  }
};

/** The approved look and the new one: side by side, or one over the other with a slider or as an onion
 *  skin. The changed pixels can be highlighted in each. */
function Compare({ project, lk }: { project: string; lk: Look }) {
  const t = useT();
  const [mode, setMode] = useState<Mode>(readMode);
  const [highlight, setHighlight] = useState(true);
  const [pos, setPos] = useState(50);
  const [fade, setFade] = useState(50);
  const a = lk.approved;
  const now = fileUrl(project, lk.run, lk.path);
  if (!a || lk.status === 'new')
    return (
      <div className={s.stage}>
        <figure className={s.fig}>
          <figcaption>{t.newLook}</figcaption>
          <Frame w={lk.width} h={lk.height}>
            <Shot src={now} w={lk.width} h={lk.height} W={lk.width} H={lk.height} alt={lk.name} />
          </Frame>
        </figure>
      </div>
    );
  const was = fileUrl(project, a.run, a.path);
  const W = Math.max(lk.width, a.width || lk.width);
  const H = Math.max(lk.height, a.height || lk.height);
  const diff = highlight && lk.diff ? <img className={s.diff} src={diffUrl(project, lk.diff)} alt="" data-diff /> : null;
  const approvedShot = (
    <Shot src={was} w={a.width || lk.width} h={a.height || lk.height} W={W} H={H} alt={`${t.approvedLook}: ${lk.name}`} />
  );
  const newShot = <Shot src={now} w={lk.width} h={lk.height} W={W} H={H} alt={`${t.newLook}: ${lk.name}`} />;
  const choose = (m: Mode) => {
    setMode(m);
    try {
      localStorage.setItem('lookMode', m);
    } catch {
      // private mode: the choice lasts for this page
    }
  };
  const drag = (e: PointerEvent<HTMLDivElement>) => {
    const box = e.currentTarget.getBoundingClientRect();
    const at = (x: number) => setPos(Math.round(Math.max(0, Math.min(100, ((x - box.left) / box.width) * 100))));
    at(e.clientX);
    const el = e.currentTarget;
    el.setPointerCapture(e.pointerId);
    el.onpointermove = (m) => at(m.clientX);
    el.onpointerup = el.onpointercancel = () => {
      el.onpointermove = null;
    };
  };
  return (
    <>
      <div className={s.controls}>
        <Segmented
          value={mode}
          onChange={choose}
          options={[
            { value: 'side', label: t.sideBySide },
            { value: 'slider', label: t.slider },
            { value: 'onion', label: t.onion },
          ]}
        />
        {lk.diff && (
          <label className={s.toggle}>
            <input type="checkbox" checked={highlight} onChange={(e) => setHighlight(e.target.checked)} />
            <span className={s.swatch} />
            {t.highlight}
          </label>
        )}
      </div>
      {mode === 'side' && (
        <div className={`${s.stage} ${s.side}`}>
          <figure className={s.fig}>
            <figcaption>{t.approvedLook}</figcaption>
            <Frame w={W} h={H}>
              {approvedShot}
              {diff}
            </Frame>
          </figure>
          <figure className={s.fig}>
            <figcaption>{t.newLook}</figcaption>
            <Frame w={W} h={H}>
              {newShot}
              {diff}
            </Frame>
          </figure>
        </div>
      )}
      {mode === 'slider' && (
        <div className={s.stage}>
          <figure className={s.fig} style={{ maxWidth: `${W}px` }}>
            <figcaption className={s.ends}>
              <span>{t.approvedLook}</span>
              <span>{t.newLook}</span>
            </figcaption>
            <div className={s.slideArea} onPointerDown={drag} style={{ '--pos': `${pos}%` } as CSSProperties}>
              <Frame w={W} h={H}>
                {approvedShot}
                <div className={s.clip}>{newShot}</div>
                {diff}
                <span className={s.handle} aria-hidden="true" />
              </Frame>
            </div>
            <input
              className={s.range}
              type="range"
              min={0}
              max={100}
              value={pos}
              aria-label={t.position}
              onChange={(e) => setPos(Number(e.target.value))}
            />
          </figure>
        </div>
      )}
      {mode === 'onion' && (
        <div className={s.stage}>
          <figure className={s.fig} style={{ maxWidth: `${W}px` }}>
            <figcaption className={s.ends}>
              <span>{t.approvedLook}</span>
              <span>{t.newLook}</span>
            </figcaption>
            <Frame w={W} h={H}>
              {approvedShot}
              <div className={s.fade} style={{ opacity: fade / 100 }}>
                {newShot}
              </div>
              {diff}
            </Frame>
            <input
              className={s.range}
              type="range"
              min={0}
              max={100}
              value={fade}
              aria-label={t.opacity}
              onChange={(e) => setFade(Number(e.target.value))}
            />
          </figure>
        </div>
      )}
    </>
  );
}

/** A box with the larger image's proportions, never wider than its pixels; screenshots sit in its corner. */
function Frame({ w, h, children }: { w: number; h: number; children: ReactNode }) {
  return (
    <div className={s.frameWrap} style={{ maxWidth: `${w}px` }}>
      <div className={s.frame} style={{ aspectRatio: `${w} / ${h}` }}>
        {children}
      </div>
    </div>
  );
}

function Shot({ src, w, h, W, H, alt }: { src: string; w: number; h: number; W: number; H: number; alt: string }) {
  return <img className={s.img} src={src} alt={alt} style={{ width: `${(w / W) * 100}%`, height: `${(h / H) * 100}%` }} />;
}

/** Approving a look, as the person when served locally; otherwise the command that does it. */
function Approve({
  p,
  sha,
  lk,
  variants,
  onApproved,
}: {
  p: Project;
  sha: string;
  lk: Look;
  variants: Look[];
  onApproved: () => Promise<void>;
}) {
  const session = use(loadSession());
  const t = useT();
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const open = variants.filter(pending);
  if (done)
    return (
      <p className={s.done} role="status">
        <Icon name="check" size={16} />
        {t.lookApproved}
      </p>
    );
  if (!pending(lk)) return null;
  const approve = async (ids: string[]) => {
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      const r = await fetch(`api/${p.id}/approve-looks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ commit: sha, looks: ids }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error ?? r.statusText);
      setDone(true); // the actions go at once; the rebuilt files follow
      await onApproved();
    } catch (e) {
      setError(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  };
  if (!session.edit) {
    const quote = (x: string) => `'${x.replace(/'/g, `'\\''`)}'`;
    const cmd = `shipwake looks approve --commit ${sha.slice(0, 12)} ${quote(lk.name)}`;
    return (
      <div className={rs.cmd}>
        <div className={rs.cmdHead}>
          <span>{t.runInRepo}</span>
          <button type="button" onClick={() => navigator.clipboard?.writeText(cmd).then(() => setCopied(true))}>
            {copied ? t.copied : t.copy}
          </button>
        </div>
        <pre>{cmd}</pre>
      </div>
    );
  }
  return (
    <div className={s.actions}>
      <button type="button" className={rs.primary} disabled={busy} onClick={() => approve([lk.id])}>
        {busy ? t.approving : t.approveLook}
      </button>
      {open.length > 1 && (
        <button type="button" className={s.secondary} disabled={busy} onClick={() => approve(open.map((l) => l.id))}>
          {t.approveVariants(open.length)}
        </button>
      )}
      {error && (
        <span className={rs.error} role="alert">
          {error}
        </span>
      )}
    </div>
  );
}

/** For the release page and check: the commit's looks nobody approved, by name. */
export function lookChecks(p: Project, sha: string, t: ReturnType<typeof useT>) {
  const tally = p.looks?.[sha];
  if (!tally) return [];
  return [
    ...(tally.changed ?? []).map((name) => ({ status: 'bad' as const, text: t.lookCheckChanged(name), look: true })),
    ...(tally.new ?? []).map((name) => ({ status: 'bad' as const, text: t.lookCheckNew(name), look: true })),
    ...(tally.same ? [{ status: 'ok' as const, text: t.lookCheckOk(tally.same), look: true }] : []),
  ];
}
