// An evaluation's cases, as in the draft: each item of its dataset with its input, what it gave before
// (in the baseline run) and now, and the grader's reason. Worse first.
import { use, useState } from 'react';
import { loadRun } from '../data';
import { comparison, effectiveFor } from '../measure';
import type { Case, Project, Run } from '../model.gen';
import { useSettings, useT } from '../settings';
import { Segmented, StatusIcon } from '../ui';
import s from './Cases.module.css';

type Filter = 'worse' | 'better' | 'failing' | 'all';

export function Cases({ p, run }: { p: Project; run: Run }) {
  const { baseline } = useSettings();
  const cases = run.cases ?? [];
  // The baseline of the metric the cases count toward, as for its measurement.
  const m = run.metrics?.find((x) => x.id === cases[0]?.metric) ?? run.metrics?.[0];
  // The same baseline as the measurements above: the last release when there is one, else the previous run.
  const cmp = m && comparison(m, effectiveFor(run, baseline));
  if (!cmp?.run) return <CaseList cases={cases} />;
  return <WithBaseline p={p} cases={cases} baseRun={cmp.run} label={cmp.label} />;
}

function WithBaseline({ p, cases, baseRun, label }: { p: Project; cases: Case[]; baseRun: string; label: string }) {
  const base = use(loadRun(p.id, baseRun));
  const before = new Map((base.cases ?? []).map((c) => [c.id, c]));
  return <CaseList cases={cases} before={before} label={label} />;
}

function CaseList({ cases, before, label }: { cases: Case[]; before?: Map<string, Case>; label?: string }) {
  const t = useT();
  const delta = (x: Case) => x.score - (before?.get(x.id)?.score ?? x.score);
  const worse = cases.filter((x) => delta(x) < 0);
  const better = cases.filter((x) => delta(x) > 0);
  const failing = cases.filter((x) => x.score < 1);
  // With a baseline, what changed; without one, what fails.
  const [filter, setFilter] = useState<Filter>(
    before ? (worse.length || !better.length ? 'worse' : 'better') : failing.length ? 'failing' : 'all',
  );
  const options: { value: Filter; label: string }[] = before
    ? [
        { value: 'worse', label: `${t.worse} ${worse.length}` },
        { value: 'better', label: `${t.better} ${better.length}` },
        { value: 'all', label: `${t.all} ${cases.length}` },
      ]
    : [
        { value: 'failing', label: `${t.failed} ${failing.length}` },
        { value: 'all', label: `${t.all} ${cases.length}` },
      ];
  const shown = { worse, better, failing, all: cases }[filter].toSorted((a, b) => delta(a) - delta(b) || a.score - b.score);
  return (
    <>
      <div className={s.sec}>
        <h2>{t.cases}</h2>
        <Segmented value={filter} onChange={setFilter} options={options} />
      </div>
      <div className="list">
        {shown.length === 0 && (
          <div className={s.none}>
            <span className="muted">{t.noCases}</span>
          </div>
        )}
        {shown.map((x) => (
          <CaseRow key={x.id} x={x} was={before?.get(x.id)} label={label} />
        ))}
      </div>
    </>
  );
}

function CaseRow({ x, was, label }: { x: Case; was?: Case; label?: string }) {
  const t = useT();
  const d = was ? x.score - was.score : 0;
  const good = was ? d >= 0 : x.score >= 1;
  return (
    <details className="row">
      <summary>
        <StatusIcon status={good ? 'passed' : 'failed'} />
        <span className="grow">{x.title || x.input}</span>
        <span className="num">{was ? `${was.score.toFixed(2)} → ${x.score.toFixed(2)}` : x.score.toFixed(2)}</span>
      </summary>
      <div className={s.body}>
        <div>
          <div className={s.lab}>{t.input}</div>
          <div className={s.text}>{x.input}</div>
        </div>
        <div className={was ? s.pair : undefined}>
          {was && (
            <div className={s.out}>
              <div className={s.lab}>
                {t.before(label ?? '')} · {was.score.toFixed(2)}
              </div>
              <div className={s.text}>{was.output}</div>
            </div>
          )}
          <div className={`${s.out} ${d < 0 ? s.worse : d > 0 ? s.better : ''}`}>
            <div className={s.lab}>
              {t.now} · {x.score.toFixed(2)}
            </div>
            <div className={s.text}>{x.output}</div>
          </div>
        </div>
        {x.reason && (
          <div>
            <span className={s.lab}>{t.grader}</span> {x.reason}
          </div>
        )}
      </div>
    </details>
  );
}
