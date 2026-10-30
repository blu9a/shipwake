// Proposed requirements at the top of the timeline: a feature's
// scenarios, or a metric in sentences. Served by `shipwake serve`, it is the person's workspace:
// change the proposal, approve it.
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { MetricStory, StoryFeature, StoryScenario } from '../data';
import { when } from '../format';
import { useSettings, useT } from '../settings';
import { MetricEditor, MetricView } from './MetricText';
import s from './Proposal.module.css';

/** A feature's scenarios that aren't approved yet, in a dashed box. On the timeline it folds and
 *  names its feature. */
export function Proposal({
  project,
  f,
  proposed,
  edit,
  onChange,
  folding,
}: {
  project: string;
  f: StoryFeature;
  proposed: StoryScenario[];
  edit: boolean;
  onChange: () => Promise<void>;
  folding?: boolean;
}) {
  const t = useT();
  const { lang } = useSettings();
  const [text, setText] = useState<string>();
  const [saved, setSaved] = useState<string>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [approved, setApproved] = useState(false);
  const editing = edit && !approved;
  useEffect(() => {
    if (!edit) return;
    const ctl = new AbortController();
    fetch(`api/${project}/proposal?path=${encodeURIComponent(f.path)}`, { cache: 'no-store', signal: ctl.signal })
      .then((r) => {
        if (!r.ok) throw new Error(r.statusText);
        return r.json();
      })
      .then((d) => {
        setText(d.text ?? '');
        setSaved(d.text ?? '');
      })
      .catch((e) => {
        if (!ctl.signal.aborted) setError(String((e as Error).message));
      });
    return () => ctl.abort();
  }, [edit, project, f.path]);
  const post = async (what: string, body: object) => {
    const r = await fetch(`api/${project}/${what}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error ?? r.statusText);
  };
  const run = async (approve: boolean) => {
    if (busy || approved) return;
    setBusy(true);
    setError('');
    try {
      if (text !== saved) await post('proposal', { path: f.path, text });
      if (approve) {
        const titles = [...(text ?? '').matchAll(/^\s*(?:Scenario|Scenario Outline|Example):\s*(.+)$/gm)].map((m) => (m[1] ?? '').trim());
        await post('approve', { path: f.path, scenarios: titles });
        setApproved(true);
      }
      setSaved(text);
      await onChange();
    } catch (e) {
      setError(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  };
  // When it was proposed and changed: one line, newest last.
  const history = [...new Map(proposed.flatMap((x) => x.events).map((e) => [e.commit, e])).values()].sort((a, b) =>
    a.at.localeCompare(b.at),
  );
  const head = (
    <>
      <span className={`${s.state} ${approved ? '' : s.proposed}`}>{t.storyState[approved ? 'approved' : 'proposed']}</span>
      {folding && (
        <>
          <b className={s.proposalName}>{f.name}</b>
          <span className="muted small">{t.scenarios(proposed.length)}</span>
        </>
      )}
      <span className="muted small">{history.map((e) => `${t.storyEvent[e.kind]} ${when(e.at, lang)}`).join(' · ')}</span>
    </>
  );
  const body = (
    <>
      {editing ? (
        <Editor value={text ?? ''} label={t.proposal} onChange={setText} />
      ) : (
        <div className={s.readonly}>
          {proposed.map((x) => (
            <div key={x.title} className={s.block}>
              <b>{x.title}</b>
              {x.steps.map((st, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: steps can repeat
                <div key={i}>{st}</div>
              ))}
            </div>
          ))}
        </div>
      )}
      {editing && (
        <div className={s.actions}>
          <button type="button" className={s.primary} disabled={busy || !text?.trim()} onClick={() => run(true)}>
            {t.approveProposal}
          </button>
          <button type="button" className={s.secondary} disabled={busy || text === saved} onClick={() => run(false)}>
            {t.saveChanges}
          </button>
        </div>
      )}
      {error && (
        <div className={s.error} role="alert">
          {error}
        </div>
      )}
    </>
  );
  if (folding)
    return (
      <details className={`${s.proposal} ${s.folding}`}>
        <summary className={s.proposalHead}>{head}</summary>
        {body}
      </details>
    );
  return (
    <section className={s.proposal}>
      <div className={s.proposalHead}>{head}</div>
      {body}
    </section>
  );
}

/** A proposed metric: its sentences in a dashed box like proposed scenarios. Served by shipwake serve,
 *  they are edited as text, checked line by line as Shipwake reads them, and approved. */
export function MetricProposal({
  project,
  m,
  edit,
  onChange,
}: {
  project: string;
  m: MetricStory;
  edit: boolean;
  onChange: () => Promise<void>;
}) {
  const t = useT();
  const { lang } = useSettings();
  const [text, setText] = useState<string>();
  const [saved, setSaved] = useState<string>();
  const [ok, setOk] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [approved, setApproved] = useState(false);
  const editing = edit && !approved;
  const check = useCallback((v: boolean) => setOk(v), []);
  useEffect(() => {
    if (!edit) return;
    const ctl = new AbortController();
    fetch(`api/${project}/metric?path=${encodeURIComponent(m.path)}`, { cache: 'no-store', signal: ctl.signal })
      .then((r) => {
        if (!r.ok) throw new Error(r.statusText);
        return r.json();
      })
      .then((d) => {
        setText(d.text ?? '');
        setSaved(d.text ?? '');
      })
      .catch((e) => {
        if (!ctl.signal.aborted) setError(String((e as Error).message));
      });
    return () => ctl.abort();
  }, [edit, project, m.path]);
  const post = async (what: string, body: object) => {
    const r = await fetch(`api/${project}/${what}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error ?? r.statusText);
  };
  const run = async (approve: boolean) => {
    if (busy || approved) return;
    setBusy(true);
    setError('');
    try {
      if (text !== saved) await post('metric', { path: m.path, text });
      if (approve) {
        await post('approve-metric', { path: m.path });
        setApproved(true);
      }
      setSaved(text);
      await onChange();
    } catch (e) {
      setError(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  };
  const history = m.events.filter((e) => e.kind === 'proposed' || e.kind === 'changed');
  return (
    <details className={`${s.proposal} ${s.folding}`}>
      <summary className={s.proposalHead}>
        <span className={`${s.state} ${approved ? '' : s.proposed}`}>{t.storyState[approved ? 'approved' : 'proposed']}</span>
        <b className={s.proposalName}>{m.name || m.id}</b>
        <span className="muted small">{t.metric}</span>
        <span className="muted small">{history.map((e) => `${t.storyEvent[e.kind]} ${when(e.at, lang)}`).join(' · ')}</span>
      </summary>
      {editing && text !== undefined ? (
        <MetricEditor project={project} path={m.path} value={text} onChange={setText} onCheck={check} />
      ) : (
        <div className={s.metricView}>
          <MetricView lines={m.lines} />
        </div>
      )}
      {editing && (
        <div className={s.actions}>
          <button type="button" className={s.primary} disabled={busy || !ok} onClick={() => run(true)}>
            {t.approveProposal}
          </button>
          <button type="button" className={s.secondary} disabled={busy || !ok || text === saved} onClick={() => run(false)}>
            {t.saveChanges}
          </button>
        </div>
      )}
      {error && (
        <div className={s.error} role="alert">
          {error}
        </div>
      )}
    </details>
  );
}

const fit = (el: HTMLTextAreaElement | null) => {
  if (!el) return;
  el.style.height = 'auto';
  el.style.height = `${el.scrollHeight + 2}px`;
};

/** The proposal's text, as tall as its wrapped lines: nothing hides below a scrollbar. */
function Editor({ value, label, onChange }: { value: string; label: string; onChange: (v: string) => void }) {
  const ref = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => fit(ref.current)); // after each change of the text
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => fit(el));
    ro.observe(el.parentElement ?? el); // the width decides the wrapping
    return () => ro.disconnect();
  }, []);
  return (
    <textarea
      ref={ref}
      className={`mono ${s.editor}`}
      value={value}
      spellCheck={false}
      aria-label={label}
      rows={4}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}
