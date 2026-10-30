// One measurement over its series: a chart with thresholds and the release, then every run.
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { use } from 'react';
import { loadMetricStories, loadProject, loadRuns, type MetricStory } from '../data';
import { avg, commitLabel, fmtValue, tr, when } from '../format';
import { comparison, effectiveFor, ruleFor, series, thresholdLine, trendOnPage, upTo } from '../measure';
import type { Metric, Project, Run } from '../model.gen';
import { useSettings, useT } from '../settings';
import { ui } from '../ui';
import s from './MeasurePage.module.css';
import { MetricView } from './MetricText';

export function MeasurePage() {
  const { project, run: id, metric } = useParams({ from: '/$project/measure/$run/$metric' });
  const p = use(loadProject(project));
  const { lang, baseline } = useSettings();
  const t = useT();
  const router = useRouter();
  const summary = p.runs.find((r) => r.id === id);
  const hist = summary ? series(p, summary) : [];
  const runs = use(
    loadRuns(
      project,
      hist.map((r) => r.id),
    ),
  );
  const pts = runs.map((r) => ({ r, m: r.metrics?.find((x) => x.id === metric) })).filter((x): x is { r: Run; m: Metric } => !!x.m);
  const here = pts.find((x) => x.r.id === id) ?? pts.at(-1);
  // the chart: the runs ending at this one, as many as it can show legibly; the list below has them all
  const charted = upTo(pts, here ? pts.indexOf(here) : -1, trendOnPage);
  if (!here) return <p className="muted">{t.notFound}</p>;
  const b = effectiveFor(here.r, baseline);
  return (
    <>
      <button type="button" className="back" onClick={() => router.history.back()}>
        ‹ {t.back}
      </button>
      <div className={s.head}>
        <h1>
          {here.m.suite ? `${tr(p, here.m.suite, lang)} · ` : ''}
          {here.m.name}
        </h1>
        <span className="muted">{tr(p, here.r.name, lang)}</span>
      </div>
      <Requirement project={project} metric={metric} />
      <Chart p={p} pts={charted} current={here.r.id} baseline={b} />
      <div className={s.sec}>
        <h2>{t.runsHeading}</h2>
      </div>
      <div className="list">
        {[...pts].reverse().map(({ r, m }) => {
          // each run against its own baseline: an older run has no comparison with today's last release
          const c = comparison(m, effectiveFor(r, baseline));
          const d = c?.delta;
          const worse = d !== undefined && (m.direction === 'higher' ? d < 0 : d > 0);
          return (
            <Link
              key={r.id}
              className={ui.resultRow}
              aria-current={r.id === here.r.id ? 'true' : undefined}
              to="/$project/run/$run"
              params={{ project, run: r.id }}
            >
              <span className={ui.resultName}>
                <code>{commitLabel(p, r.commit)}</code>
                <span className="muted">{when(r.start, lang)}</span>
              </span>
              <span className={ui.resultValue}>
                <span>{fmtValue(avg(m.values), m.unit)}</span>
                {m.values.length > 1 && <small>± {fmtValue((Math.max(...m.values) - Math.min(...m.values)) / 2, m.unit)}</small>}
              </span>
              <span className={`${ui.resultDelta} ${d === undefined || Math.abs(d) < 0.5 ? 'muted' : worse ? 'del' : 'add'}`}>
                {d === undefined ? '' : Math.abs(d) < 0.05 ? '0%' : `${d > 0 ? '▲' : '▼'} ${Math.abs(d).toFixed(1)}%`}
              </span>
              <span className={ui.resultStatus}>
                {c && c.status !== 'none' && <span className={`stChip ${c.status}`}>{t.status[c.status]}</span>}
              </span>
            </Link>
          );
        })}
      </div>
    </>
  );
}

function Chart({
  p,
  pts,
  current,
  baseline,
}: {
  p: Project;
  pts: { r: Run; m: Metric }[];
  current: string;
  baseline: 'previous' | 'release';
}) {
  const t = useT();
  const W = 900;
  const H = 260;
  const L = 70;
  const R = 14;
  const T = 16;
  const B = 34;
  const metric = pts[0]?.m as Metric;
  const limit = ruleFor(p, metric.id);
  const tag = p.tags[0];
  // Runs newer than the release are the ones compared with it.
  const relIdx = tag ? pts.findIndex(({ m }) => !!m.release?.run) : -1;
  const relBase = relIdx >= 0 ? pts[relIdx]?.m.release?.value : undefined;
  const line = (th?: string) => thresholdLine(th, metric.unit, baseline === 'release' ? relBase : undefined);
  const warn = line(limit?.warn);
  const block = line(limit?.block);
  const all = pts.flatMap(({ m }) => m.values).concat([warn, block].filter((x): x is number => x !== undefined && Number.isFinite(x)));
  let lo = Math.min(...all);
  let hi = Math.max(...all);
  const pad = (hi - lo || Math.abs(hi) || 1) * 0.12;
  lo -= pad;
  hi += pad;
  const x = (i: number) => (pts.length === 1 ? (L + W - R) / 2 : L + (i * (W - L - R)) / (pts.length - 1));
  const y = (v: number) => T + ((hi - v) / (hi - lo)) * (H - T - B);
  const band = [
    ...pts.map(({ m }, i) => `${x(i)},${y(Math.max(...m.values))}`),
    ...pts.map(({ m }, i) => `${x(i)},${y(Math.min(...m.values))}`).reverse(),
  ].join(' ');
  const ticks = [lo + pad, (lo + hi) / 2, hi - pad];
  const hline = (v: number | undefined, cls: string, label: string) =>
    v === undefined || v < lo || v > hi ? null : (
      <g className={s[cls]}>
        <line x1={L} x2={W - R} y1={y(v)} y2={y(v)} className={s.th} />
        <text x={W - R} y={y(v) - 4} textAnchor="end" className={s.thl}>
          {label}
        </text>
      </g>
    );
  return (
    <section className={s.chart}>
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={metric.name}>
        {ticks.map((v) => (
          <g key={v}>
            <line className={s.grid} x1={L} x2={W - R} y1={y(v)} y2={y(v)} />
            <text className={s.axl} x={L - 8} y={y(v) + 4} textAnchor="end">
              {fmtValue(v, metric.unit)}
            </text>
          </g>
        ))}
        {hline(warn, 'warn', t.status.warn ?? '')}
        {hline(block, 'block', t.status.block ?? '')}
        {relIdx > 0 && tag && (
          <g className={s.rel}>
            <line x1={(x(relIdx - 1) + x(relIdx)) / 2} x2={(x(relIdx - 1) + x(relIdx)) / 2} y1={T} y2={H - B} />
            <text x={(x(relIdx - 1) + x(relIdx)) / 2 + 4} y={T + 10}>
              {tag.name}
            </text>
          </g>
        )}
        <polygon className={s.band} points={band} />
        <polyline className={s.ln} points={pts.map(({ m }, i) => `${x(i)},${y(avg(m.values))}`).join(' ')} />
        {pts.map(({ r, m }, i) => {
          const st = comparison(m, effectiveFor(r, baseline))?.status ?? 'none';
          return (
            <g key={r.id}>
              <Link to="/$project/run/$run" params={{ project: p.id, run: r.id }}>
                <circle
                  className={`${s.pt} ${s[st] ?? ''} ${r.id === current ? s.cur : ''}`}
                  cx={x(i)}
                  cy={y(avg(m.values))}
                  r={r.id === current ? 6 : 4.5}
                >
                  <title>{`${commitLabel(p, r.commit)} · ${fmtValue(avg(m.values), m.unit)}`}</title>
                </circle>
              </Link>
              <text
                className={s.axl}
                x={x(i)}
                y={H - 12}
                textAnchor={pts.length === 1 ? 'middle' : i === 0 ? 'start' : i === pts.length - 1 ? 'end' : 'middle'}
              >
                {commitLabel(p, r.commit)}
              </text>
            </g>
          );
        })}
      </svg>
    </section>
  );
}

/** When the metric is a requirement, what was agreed and its story: proposed, approved by
 *  whom, then the runs that met or missed it. */
function Requirement({ project, metric }: { project: string; metric: string }) {
  const stories = use(loadMetricStories(project));
  const m: MetricStory | undefined = stories.find((x) => x.id === metric);
  const { lang } = useSettings();
  const t = useT();
  if (!m) return null;
  return (
    <section className={s.requirement}>
      <MetricView lines={m.lines} />
      <ol className={s.story}>
        {m.events.map((e) => (
          <li key={`${e.kind}${e.at}${e.run ?? e.commit}`}>
            <span className={`${s.dot} ${s[e.kind] ?? ''}`} />
            <span>
              {t.storyEvent[e.kind]}
              {e.version && e.kind !== 'approved' && e.kind !== 'met' && e.kind !== 'missed' ? ` · v${e.version}` : ''}
              {e.by && e.kind !== 'met' && e.kind !== 'missed' && <span className="muted"> · {e.by}</span>}
            </span>
            <span className="muted small">
              {when(e.at, lang)}
              {' · '}
              {e.run ? (
                <Link to="/$project/run/$run" params={{ project, run: e.run }}>
                  {t.theRun}
                </Link>
              ) : (
                <Link to="/$project/commit/$sha" params={{ project, sha: e.commit }}>
                  <code>{e.commit.slice(0, 7)}</code>
                </Link>
              )}
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}
