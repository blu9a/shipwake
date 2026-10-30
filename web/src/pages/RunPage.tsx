// A run: tests grouped by feature, or measurements grouped by suite with their status and trend.
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { type CSSProperties, createContext, type ReactElement, type ReactNode, use, useContext, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { fileUrl, loadProject, loadRun, loadRuns } from '../data';
import { avg, fmtValue, full, targetName, tr } from '../format';
import { comparison, effectiveFor, hasRelease, series, trendInRow, upTo } from '../measure';
import type { Attachment, Evidence, Metric, Project, Run, Step, Test } from '../model.gen';
import { useSettings, useT } from '../settings';
import { Icon, ScrollName, Segmented, StatusIcon, Tally, ui } from '../ui';
import { Caption, useApplySettings, useSubtitles, VideoControls } from '../video';
import { Cases } from './Cases';
import s from './RunPage.module.css';
import { featureClips } from './WatchPage';

export function RunPage() {
  const { project, run: id } = useParams({ from: '/$project/run/$run' });
  const p = use(loadProject(project));
  const run = use(loadRun(project, id));
  const { lang } = useSettings();
  const t = useT();
  const router = useRouter();
  const c = p.commits[run.commit];
  const summary = p.runs.find((r) => r.id === id);
  const measured = (run.metrics?.length ?? 0) > 0;
  return (
    <>
      <button type="button" className="back" onClick={() => router.history.back()}>
        ‹ {t.back}
      </button>
      <div className={s.head}>
        <h1>{tr(p, run.name, lang)}</h1>
        <span className={s.when}>
          {full(run.start, lang)}
          {run.duration > 0 && ` (${t.secs(run.duration)})`}
        </span>
      </div>
      <section className={s.overview}>
        <dl className="facts plain">
          {run.branch && (
            <>
              <dt>{t.branch}</dt>
              <dd>
                <code>{run.branch}</code>
              </dd>
            </>
          )}
          <dt>{t.commit}</dt>
          <dd>
            <Link to="/$project/commit/$sha" params={{ project, sha: run.commit }}>
              <code>{c?.short ?? run.commit.slice(0, 7)}</code>
            </Link>{' '}
            {c?.subject}
          </dd>
          <dt>{t.stableTag}</dt>
          <dd>{p.tags.find((x) => x.sha === run.commit)?.name ?? t.none}</dd>
          {[...new Set([...Object.keys(run.context ?? {}), ...(summary?.changed ?? [])])].map((k) => (
            <Ctx key={k} k={k} v={run.context?.[k] ?? ''} changed={!!summary?.changed?.includes(k)} was={summary?.was?.[k]} />
          ))}
        </dl>
        {summary && !measured && (
          <div className={s.stats}>
            <Stat label={t.failed} value={summary.failed} tone="f" />
            <Stat label={t.passed} value={summary.passed} tone="p" />
          </div>
        )}
        {summary && measured && <MeasureStats run={run} />}
      </section>
      {(run.tests?.length ?? 0) > 0 && (
        <FullView>
          <Tests p={p} run={run} />
        </FullView>
      )}
      {measured && <Measurements p={p} run={run} />}
      {(run.cases?.length ?? 0) > 0 && <Cases p={p} run={run} />}
    </>
  );
}

/** A context value; when it differs from the previous run of the series, what it was. */
function Ctx({ k, v, changed, was }: { k: string; v: string; changed: boolean; was?: string }) {
  const t = useT();
  return (
    <>
      <dt>{k}</dt>
      <dd>
        {v || '—'}
        {changed && <span className={s.was}>{was ? t.was(was) : t.wasUnset}</span>}
      </dd>
    </>
  );
}

function Stat({ label, value, tone }: { label: string; value: number; tone: 'f' | 'w' | 'p' }) {
  return (
    <div className={`${s.stat} ${s[tone]} ${tone === 'f' && value ? s.has : ''}`}>
      <div className={s.k}>
        <i />
        {label}
      </div>
      <div className={s.v}>{value}</div>
    </div>
  );
}

function MeasureStats({ run }: { run: Run }) {
  const { baseline } = useSettings();
  const t = useT();
  const n: Record<string, number> = { ok: 0, warn: 0, block: 0 };
  for (const m of run.metrics ?? []) {
    const st = comparison(m, effectiveFor(run, baseline))?.status;
    if (st && st in n) n[st] = (n[st] ?? 0) + 1;
  }
  if (!n.block && !n.warn && !n.ok) return null;
  return (
    <div className={s.stats}>
      <Stat label={t.status.block ?? ''} value={n.block ?? 0} tone="f" />
      <Stat label={t.status.warn ?? ''} value={n.warn ?? 0} tone="w" />
      <Stat label={t.status.ok ?? ''} value={n.ok ?? 0} tone="p" />
    </div>
  );
}

function Tests({ p, run }: { p: Project; run: Run }) {
  const { lang } = useSettings();
  const t = useT();
  const tests = run.tests ?? [];
  const targets = [...new Set(tests.map((x) => x.target).filter((x): x is string => !!x))];
  const [target, setTarget] = useState('all');
  const shown = tests.filter((x) => target === 'all' || x.target === target);
  const features = [...new Set(shown.map((x) => x.feature ?? ''))];
  // A bar on the timeline opens its test in the list below and brings it into view.
  const [pick, setPick] = useState<number>();
  useEffect(() => {
    if (pick === undefined) return;
    const row = document.getElementById(`t${pick}`) as HTMLDetailsElement | null;
    if (!row) return;
    row.open = true;
    row.scrollIntoView({ block: 'center', behavior: 'smooth' });
    row.classList.add(s.hit ?? '');
    const tm = setTimeout(() => row.classList.remove(s.hit ?? ''), 1600);
    return () => clearTimeout(tm);
  }, [pick]);
  return (
    <>
      <div className={s.sec}>
        <h2>{t.timeline}</h2>
      </div>
      <Gantt
        p={p}
        run={run}
        onPick={(i) => {
          setTarget('all');
          setPick(undefined);
          setTimeout(() => setPick(i));
        }}
      />
      <div className={s.sec}>
        <h2>{t.tests}</h2>
        {targets.length > 1 && (
          <Segmented
            value={target}
            onChange={setTarget}
            options={[{ value: 'all', label: t.all }, ...targets.map((x) => ({ value: x, label: targetName(p, x, lang) }))]}
          />
        )}
      </div>
      <div className="list">
        {features.map((f) => (
          <Group key={f} p={p} run={run} name={f} tests={shown.filter((x) => (x.feature ?? '') === f)} showTarget={target === 'all'} />
        ))}
      </div>
    </>
  );
}

/** A feature's tests. When several have recordings, the header links to them played as one video. */
function Group({ p, run, name, tests, showTarget }: { p: Project; run: Run; name: string; tests: Test[]; showTarget: boolean }) {
  const { lang } = useSettings();
  const t = useT();
  const ts = [...tests].sort((a, b) => Number(b.status === 'failed') - Number(a.status === 'failed'));
  const clips = featureClips(run, name);
  return (
    <div>
      <div className="groupHead">
        <span className={s.groupName}>{tr(p, name, lang) || '—'}</span>
        {clips.length > 1 && (
          <Link className={s.watch} to="/$project/run/$run/watch/$" params={{ project: p.id, run: run.id, _splat: name }}>
            <Icon name="play" size={12} />
            {t.watchFeature(clips.length, t.secs(clips.reduce((a, c) => a + c.x.duration, 0)))}
          </Link>
        )}
        <span className="grow" />
        <Tally failed={ts.filter((x) => x.status === 'failed').length} passed={ts.filter((x) => x.status === 'passed').length} />
      </div>
      {ts.map((x) => (
        <TestRow key={`${x.name}|${x.target ?? ''}|${x.start ?? ''}`} p={p} run={run} x={x} showTarget={showTarget} />
      ))}
    </div>
  );
}

function Recording({ p, run, x, video }: { p: Project; run: Run; x: Test; video: Attachment }) {
  const ref = useRef<HTMLVideoElement>(null);
  useApplySettings(ref);
  const subs = useSubtitles(p, x, video);
  return (
    <div>
      <div className={s.screen}>
        {/* biome-ignore lint/a11y/useMediaCaption: the subtitles track is added when there are steps */}
        <video ref={ref} className={s.video} controls preload="metadata" src={fileUrl(p.id, run.id, video.path)}>
          {subs.url && <track key={subs.url} kind="subtitles" srcLang={subs.lang} label={subs.lang} src={subs.url} />}
        </video>
        <Caption video={ref} />
      </div>
      <VideoControls />
    </div>
  );
}

function TestRow({ p, run, x, showTarget }: { p: Project; run: Run; x: Test; showTarget: boolean }) {
  const { lang } = useSettings();
  const t = useT();
  const gherkin = (x.steps ?? []).some((st) => /^(Given|When|Then) /.test(st.name));
  const video = (x.attachments ?? []).find((a) => a.type.startsWith('video/'));
  const files = (x.attachments ?? []).filter((a) => !a.type.startsWith('video/'));
  return (
    <details className="row" id={`t${(run.tests ?? []).indexOf(x)}`}>
      <summary>
        <StatusIcon status={x.status} />
        <span className="grow">{tr(p, x.name, lang)}</span>
        {x.retried && <span className="chip warn">{t.passedOnRetry}</span>}
        {hasAttachments(x) && <AttachmentMark />}
        {showTarget && x.target && <span className="muted small">{targetName(p, x.target, lang)}</span>}
        <span className="num">{t.secs(x.duration)}</span>
      </summary>
      <div className={s.detail}>
        {gherkin ? (
          <Passage p={p} steps={x.steps ?? []} evidence={x.evidence ?? []} error={x.error} />
        ) : (
          x.error && <pre className={s.err}>{x.error}</pre>
        )}
        {video && <Recording p={p} run={run} x={x} video={video} />}
        <div className="meta">
          {x.start && <span>{`${t.started} ${full(x.start, lang)}`}</span>}
          <span>{`${t.attempts} ${x.attempts}`}</span>
        </div>
        {(x.steps ?? []).length > 0 ? (
          <details className={s.more}>
            <summary>
              <span>{t.stepsFiles}</span>
              <Icon name="down" size={14} />
            </summary>
            <div className={s.tree}>
              {(x.steps ?? []).map((st, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: steps can repeat; their position is their identity
                <StepRow key={i} p={p} run={run} st={st} n={i + 1} level={0} />
              ))}
              {files.map((a, i) => (
                <FileRow key={a.path} p={p} run={run} a={a} n={i + 1} level={0} />
              ))}
            </div>
          </details>
        ) : (
          files.length > 0 && (
            <div className={s.tree}>
              {files.map((a, i) => (
                <FileRow key={a.path} p={p} run={run} a={a} n={i + 1} level={0} />
              ))}
            </div>
          )
        )}
      </div>
    </details>
  );
}

/** The run on a time axis: one row of bars per target, a bar per test from its start to its end. */
function Gantt({ p, run, onPick }: { p: Project; run: Run; onPick: (i: number) => void }) {
  const { lang } = useSettings();
  const t = useT();
  const t0 = new Date(run.start).getTime();
  const bars = (run.tests ?? []).map((x, i) => ({
    i,
    x,
    from: x.start ? new Date(x.start).getTime() - t0 : 0,
    target: x.target ?? '',
  }));
  const span = Math.max(1, ...bars.map((b) => b.from + b.x.duration));
  const targets = [...new Set(bars.map((b) => b.target))];
  const solo = targets.length === 1 && targets[0] === '';
  const step = [1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 15000, 30000, 60000].find((x) => span / x <= 8) ?? 120000;
  const ticks: number[] = [];
  for (let x = 0; x <= span; x += step) ticks.push(x);
  const pct = (ms: number) => `${(ms / span) * 100}%`;
  return (
    <div className={`${s.gantt} ${solo ? s.solo : ''}`}>
      {targets.map((tg) => {
        // Tests that overlap go on separate lanes.
        const lanes: { end: number; bars: typeof bars }[] = [];
        for (const b of bars.filter((x) => x.target === tg).sort((a, c) => a.from - c.from)) {
          let lane = lanes.find((l) => l.end <= b.from);
          if (!lane) {
            lane = { end: 0, bars: [] };
            lanes.push(lane);
          }
          lane.bars.push(b);
          lane.end = b.from + b.x.duration;
        }
        return (
          <div key={tg} className={s.gGroup}>
            {!solo && <div className={s.gLabel}>{targetName(p, tg, lang)}</div>}
            <div>
              {lanes.map((l) => (
                <div key={l.bars[0]?.i} className={s.gLane}>
                  {l.bars.map((b) => (
                    <button
                      type="button"
                      key={b.i}
                      className={`${s.gBar} ${b.x.status === 'failed' ? s.failed : ''}`}
                      style={{ left: pct(b.from), width: pct(b.x.duration) }}
                      title={`${tr(p, b.x.name, lang)} · ${t.secs(b.x.duration)}`}
                      aria-label={tr(p, b.x.name, lang)}
                      onClick={() => onPick(b.i)}
                    />
                  ))}
                </div>
              ))}
            </div>
          </div>
        );
      })}
      <div className={s.gAxis}>
        {ticks.map((x) => (
          <span key={x} style={{ left: pct(x) }}>
            {x === 0 ? '0' : t.secs(x).replace(/\.0(?= )/, '')}
          </span>
        ))}
      </div>
    </div>
  );
}

/** Presence only: no attachment totals to store or keep in sync. */
function hasAttachments(x: { attachments?: Attachment[]; steps?: Step[] }): boolean {
  return !!x.attachments?.length || !!x.steps?.some(hasAttachments);
}

function AttachmentMark() {
  const t = useT();
  return (
    <span className={s.attached} role="img" aria-label={t.attachments} title={t.attachments}>
      <Icon name="clip" size={16} />
    </span>
  );
}

/** One step in "Steps and files": its status, name, what it holds, how long it took; open to see inside. */
function StepRow({ p, run, st, n, level }: { p: Project; run: Run; st: Step; n: number; level: number }) {
  const { lang } = useSettings();
  const t = useT();
  const inner = st.steps ?? [];
  const atts = st.attachments ?? [];
  const head = (
    <>
      <StatusIcon status={st.status} />
      <span className={s.n}>{n}</span>
      <ScrollName className={s.name}>{tr(p, st.name, lang)}</ScrollName>
      {(hasAttachments(st) || inner.length > 0) && (
        <span className={s.badges}>
          {hasAttachments(st) && <AttachmentMark />}
          {inner.length > 0 && (
            <span title={t.substeps}>
              <Icon name="sub" size={16} />
              {inner.length}
            </span>
          )}
        </span>
      )}
      <span className={s.du}>{t.secs(st.duration)}</span>
    </>
  );
  const pad = { '--lv': level } as CSSProperties;
  if (!inner.length && !atts.length)
    return (
      <div className={`listRow ${s.srow}`} style={pad}>
        {head}
      </div>
    );
  return (
    <details open>
      <summary className={`listRow ${s.srow}`} style={pad}>
        {head}
      </summary>
      {inner.map((c, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: steps can repeat; their position is their identity
        <StepRow key={i} p={p} run={run} st={c} n={i + 1} level={level + 1} />
      ))}
      {atts.map((a, i) => (
        <FileRow key={a.path} p={p} run={run} a={a} n={i + 1} level={level + 1} />
      ))}
    </details>
  );
}

const kib = (n: number) => (n < 1024 ? `${n} B` : n < 1048576 ? `${(n / 1024).toFixed(1)} KiB` : `${(n / 1048576).toFixed(1)} MiB`);

/** An attached file: its kind, name, type and size, full screen and download; open to see it. */
function FileRow({ p, run, a, n, level }: { p: Project; run: Run; a: Attachment; n: number; level: number }) {
  const t = useT();
  const show = useContext(Full);
  const url = fileUrl(p.id, run.id, a.path);
  const image = a.type.startsWith('image/');
  const video = a.type.startsWith('video/');
  const pad = { '--lv': level } as CSSProperties;
  return (
    <details style={pad}>
      <summary className={`listRow ${s.srow}`} style={pad}>
        <span className={s.aIc}>
          <Icon name={image ? 'image' : video ? 'film' : 'file'} size={18} />
        </span>
        <span className={s.n}>{n}</span>
        <ScrollName className={s.name}>{a.name}</ScrollName>
        <span className={s.mime}>{a.type}</span>
        <span className={s.mime}>{kib(a.size)}</span>
        <span className={s.acts}>
          {(image || video) && (
            <button
              type="button"
              className={s.plain}
              title={t.fullscreen}
              aria-label={t.fullscreen}
              onClick={(e) => {
                e.preventDefault();
                show({ url, kind: image ? 'image' : 'video' });
              }}
            >
              <Icon name="expand" size={18} />
            </button>
          )}
          <a className={s.plain} href={url} download title={t.download} aria-label={t.download} onClick={(e) => e.stopPropagation()}>
            <Icon name="download" size={18} />
          </a>
        </span>
      </summary>
      <div className={s.abody}>
        {image ? (
          <img loading="lazy" src={url} alt={a.name} />
        ) : video ? (
          // biome-ignore lint/a11y/useMediaCaption: a recording of the app
          <video controls preload="none" src={url} />
        ) : null}
      </div>
    </details>
  );
}

type Shown = { url: string; kind: 'image' | 'video' };
const Full = createContext<(f: Shown) => void>(() => {});

/** Screenshots and recordings at full size over the page; a click outside or Escape closes them. */
function FullView({ children }: { children: ReactNode }) {
  const t = useT();
  const [shown, setShown] = useState<Shown>();
  useEffect(() => {
    if (!shown) return;
    const key = (e: KeyboardEvent) => e.key === 'Escape' && setShown(undefined);
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  }, [shown]);
  return (
    <Full value={setShown}>
      {children}
      {/* at the body: the page's container queries would otherwise confine a fixed layer to the content */}
      {shown &&
        createPortal(
          // biome-ignore lint/a11y/useKeyWithClickEvents: Escape closes it too
          // biome-ignore lint/a11y/noStaticElementInteractions: the backdrop closes the viewer
          <div className={s.viewer} onClick={(e) => e.target === e.currentTarget && setShown(undefined)}>
            <button type="button" className={s.vClose} aria-label={t.close} onClick={() => setShown(undefined)}>
              <Icon name="x" size={16} />
            </button>
            {shown.kind === 'image' ? (
              <img src={shown.url} alt="" />
            ) : (
              // biome-ignore lint/a11y/useMediaCaption: a recording of the app
              <video src={shown.url} controls autoPlay />
            )}
          </div>,
          document.body,
        )}
    </Full>
  );
}

/** A scenario as three short sentences: Given → before, When → action, Then → result; "And" joins the one above. */
function Passage({ p, steps, evidence, error }: { p: Project; steps: Step[]; evidence: Evidence[]; error?: string }) {
  const { lang } = useSettings();
  const t = useT();
  const role: Record<string, string> = { Given: 'before', When: 'action', Then: 'result' };
  const groups: { role: string; steps: { text: string; status: string }[] }[] = [];
  for (const st of steps) {
    const m = st.name.match(/^(Given|When|Then|And|But) (.*)$/);
    if (!m) continue;
    const r = role[m[1] ?? ''] ?? groups.at(-1)?.role ?? 'before';
    const last = groups.at(-1);
    if (last?.role === r) last.steps.push({ text: m[2] ?? '', status: st.status });
    else groups.push({ role: r, steps: [{ text: m[2] ?? '', status: st.status }] });
  }
  const tx = (k: string) => tr(p, k, lang);
  const joined = (parts: string[]) =>
    lang === 'zh' ? `${parts.join('；')}。` : `${parts.length > 1 ? `${parts.slice(0, -1).join(', ')} and ${parts.at(-1)}` : parts[0]}.`;
  return (
    <div className={s.passage}>
      {groups.map((g) => {
        const failed = g.steps.some((x) => x.status === 'failed');
        const proofs = g.role === 'result' ? evidence.filter((ev) => g.steps.some((x) => x.text === ev.step)) : [];
        return (
          <div key={g.role + (g.steps[0]?.text ?? '')} className={`${s.pline} ${failed ? s.bad : ''}`}>
            <span className={s.prole}>{t.roles[g.role]}</span>
            <div>
              {joined(g.steps.map((x) => tx(x.text)))}
              {failed && error && <pre className={s.err}>{error}</pre>}
              {proofs.map((ev) => (
                <div key={ev.step} className={s.proof}>
                  <span className="muted">{t.observed}</span> <Excerpt text={ev.observed} expected={ev.expected} />
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/** Only the parts of the observed text around what was expected, with the expected part marked. */
function Excerpt({ text, expected }: { text: string; expected: string[] }) {
  const pad = 28;
  const spans: [number, number][] = [];
  for (const e of expected) {
    const i = e ? text.indexOf(e) : -1;
    if (i >= 0) spans.push([Math.max(0, i - pad), Math.min(text.length, i + e.length + pad)]);
  }
  if (!spans.length) return <>“{text.length > 120 ? `${text.slice(0, 120)}…` : text}”</>;
  spans.sort((a, b) => a[0] - b[0]);
  const merged: [number, number][] = [];
  for (const sp of spans) {
    const last = merged.at(-1);
    if (last && sp[0] <= last[1]) last[1] = Math.max(last[1], sp[1]);
    else merged.push([sp[0], sp[1]]);
  }
  const marked = (part: string) => {
    const out: (string | ReactElement)[] = [];
    let rest = part;
    for (;;) {
      const hit = expected
        .filter(Boolean)
        .map((e) => ({ e, i: rest.indexOf(e) }))
        .filter((h) => h.i >= 0)
        .sort((a, b) => a.i - b.i)[0];
      if (!hit) break;
      out.push(rest.slice(0, hit.i), <mark key={out.length}>{hit.e}</mark>);
      rest = rest.slice(hit.i + hit.e.length);
    }
    out.push(rest);
    return out;
  };
  return (
    <>
      {merged.map(([a, b]) => (
        <span key={a}>
          {a > 0 ? '…' : '“'}
          {marked(text.slice(a, b))}
          {b < text.length ? '…' : '”'}{' '}
        </span>
      ))}
    </>
  );
}

/** Measurements, judged against the chosen baseline. */
function Measurements({ p, run }: { p: Project; run: Run }) {
  const { lang, baseline, setBaseline } = useSettings();
  const t = useT();
  const summary = p.runs.find((r) => r.id === run.id);
  const hist = summary ? series(p, summary) : [];
  const runs = use(
    loadRuns(
      p.id,
      hist.map((r) => r.id),
    ),
  );
  // the run and those before it in its series, as many as the row's trend shows
  const upto = upTo(
    runs,
    runs.findIndex((r) => r.id === run.id),
    trendInRow,
  );
  const b = effectiveFor(run, baseline);
  const base = (run.metrics ?? []).map((m) => comparison(m, b)).find((c) => c?.run);
  const suites = [...new Set((run.metrics ?? []).map((m) => m.suite ?? ''))];
  return (
    <>
      <div className={s.sec}>
        <h2>{t.measurements(run.metrics?.length ?? 0)}</h2>
        <div className={s.comparison}>
          {base ? (
            <span className="muted small">
              {t.baseline}{' '}
              <Link to="/$project/run/$run" params={{ project: p.id, run: base.run }}>
                {b === 'release' ? base.label : t.previousRun(base.label)}
              </Link>
            </span>
          ) : (
            summary?.changed?.includes('runner') && <span className="muted small">{t.notComparable}</span>
          )}
          <Segmented
            value={b}
            onChange={setBaseline}
            options={[
              {
                value: 'release',
                label: p.tags[0]?.name ?? t.lastReleaseShort,
                disabled: !hasRelease(run),
                title: hasRelease(run) ? undefined : t.noReleaseBase,
              },
              { value: 'previous', label: t.previousRunShort },
            ]}
          />
        </div>
      </div>
      <div className="list">
        {suites.map((su) => (
          <div key={su}>
            {su && <div className="groupHead">{tr(p, su, lang)}</div>}
            {(run.metrics ?? [])
              .filter((m) => (m.suite ?? '') === su)
              .map((m) => (
                <MetricRow key={m.id} p={p} run={run} m={m} trend={upto.map((r) => r.metrics?.find((x) => x.id === m.id))} />
              ))}
          </div>
        ))}
      </div>
    </>
  );
}

function MetricRow({ p, run, m, trend }: { p: Project; run: Run; m: Metric; trend: (Metric | undefined)[] }) {
  const { baseline } = useSettings();
  const t = useT();
  const v = avg(m.values);
  const spread = (Math.max(...m.values) - Math.min(...m.values)) / 2;
  const c = comparison(m, effectiveFor(run, baseline));
  const delta = c?.delta;
  const worse = delta !== undefined && (m.direction === 'higher' ? delta < 0 : delta > 0);
  const pts = trend.filter((x): x is Metric => !!x).map((x) => avg(x.values));
  return (
    <Link className={ui.resultRow} to="/$project/measure/$run/$metric" params={{ project: p.id, run: run.id, metric: m.id }}>
      <span className={ui.resultName}>{m.name}</span>
      <span className={ui.resultValue}>
        <span>{fmtValue(v, m.unit)}</span>
        {m.values.length > 1 && <small>± {fmtValue(spread, m.unit)}</small>}
      </span>
      <span className={`${ui.resultDelta} ${delta === undefined || Math.abs(delta) < 0.5 ? 'muted' : worse ? s.worse : s.better}`}>
        {delta === undefined ? '' : `${delta > 0 ? '▲' : delta < 0 ? '▼' : ''} ${Math.abs(delta).toFixed(1)}%`}
      </span>
      <Spark pts={pts} status={c?.status} />
      <span className={ui.resultStatus}>
        {c && c.status !== 'none' && <span className={`stChip ${c.status}`}>{t.status[c.status]}</span>}
      </span>
    </Link>
  );
}

/** The series so far, ending at this run; a first run is a single point. */
function Spark({ pts, status }: { pts: number[]; status?: string }) {
  if (pts.length === 0) return <svg className={`${s.spark} ${ui.resultTrend}`} aria-hidden="true" />;
  const lo = Math.min(...pts);
  const hi = Math.max(...pts);
  const W = 96;
  const H = 24;
  const y = (v: number) => (hi === lo ? H / 2 : 3 + ((hi - v) / (hi - lo)) * (H - 6));
  const x = (i: number) => (pts.length === 1 ? W - 4 : 2 + (i * (W - 4)) / (pts.length - 1));
  const last = pts.length - 1;
  return (
    <svg className={`${s.spark} ${ui.resultTrend}`} viewBox={`0 0 ${W} ${H}`} aria-hidden="true">
      <polyline points={pts.map((v, i) => `${x(i)},${y(v)}`).join(' ')} />
      <circle className={status ? (s[status] ?? '') : ''} cx={x(last)} cy={y(pts[last] ?? 0)} r="2.6" />
    </svg>
  );
}
