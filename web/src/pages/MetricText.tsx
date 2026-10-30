// A .metric file as sentences: what varies (a path, "100%") stands out, the fixed words of each
// pattern stay quiet. The editor is plain text with the same highlighting underneath, and a line that
// Shipwake can't read is marked where it is.
import { Fragment, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { MetricLine } from '../data';
import { useT } from '../settings';
import { Icon } from '../ui';
import s from './MetricText.module.css';

const settings = new Set(['dataset', 'direction', 'block', 'warn']);

/** One line, highlighted: the title bold, settings quiet with their value emphasized, errors marked. */
function lineContent(l: MetricLine): ReactNode {
  const t = l.text;
  if (l.kind === 'title') {
    const at = t.search(/[:：]/);
    return (
      <>
        <span className={s.kw}>{t.slice(0, at + 1)}</span>
        <span className={s.title}>{t.slice(at + 1)}</span>
      </>
    );
  }
  if (l.kind === 'tag') return <span className={s.tag}>{t}</span>;
  if (l.kind === 'comment') return <span className={s.comment}>{t}</span>;
  if (l.kind === 'error') return <span className={s.error}>{t}</span>;
  if (settings.has(l.kind) && l.value) {
    const at = t.lastIndexOf(l.value);
    if (at >= 0)
      return (
        <span className={s.quiet}>
          {t.slice(0, at)}
          <span className={`${s.value} ${s[l.kind] ?? ''}`}>{l.value}</span>
          {t.slice(at + l.value.length)}
        </span>
      );
    return <span className={s.quiet}>{t}</span>;
  }
  return t;
}

/** The highlighted text, line for line: the editor's backdrop, or the view where nothing is edited. */
function Highlight({ lines, text }: { lines: MetricLine[]; text: string }) {
  // The text decides the lines; the last check decides their meaning, matched by line number.
  const byN = new Map(lines.map((l) => [l.n, l]));
  return (
    <>
      {text.split('\n').map((raw, i) => {
        const l = byN.get(i + 1);
        const shown = l && l.text === raw ? l : { n: i + 1, text: raw, kind: 'description' as const };
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: lines are their position
          <Fragment key={i}>
            {lineContent(shown)}
            {'\n'}
          </Fragment>
        );
      })}
    </>
  );
}

/** A metric to read: its description, then its settings one per line. The name is the heading
 *  around it, so its title line is left out. */
export function MetricView({ lines }: { lines: MetricLine[] }) {
  const description = lines.filter((l) => l.kind === 'description');
  const rest = lines.filter((l) => !['tag', 'comment', 'blank', 'title', 'description'].includes(l.kind));
  return (
    <div className={s.view}>
      {description.length > 0 && <p className={s.text}>{description.map((l) => l.text.trim()).join(' ')}</p>}
      {rest.length > 0 && (
        <div className={s.settings}>
          {rest.map((l) => (
            <div key={l.n} className={s.text}>
              {lineContent({ ...l, text: l.text.trim() })}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** A metric to edit: plain text over its highlighting; each change is checked as Shipwake reads it. */
export function MetricEditor({
  project,
  path,
  value,
  onChange,
  onCheck,
}: {
  project: string;
  path: string;
  value: string;
  onChange: (v: string) => void;
  onCheck: (ok: boolean) => void;
}) {
  const t = useT();
  const [lines, setLines] = useState<MetricLine[]>([]);
  const ref = useRef<HTMLTextAreaElement>(null);
  const back = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const ctl = new AbortController();
    const tm = setTimeout(() => {
      fetch(`api/${project}/check-metric`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, text: value }),
        signal: ctl.signal,
      })
        .then((r) => r.json())
        .then((d: { lines?: MetricLine[]; name?: string }) => {
          const ls = d.lines ?? [];
          setLines(ls);
          onCheck(!!d.name && !ls.some((l) => l.kind === 'error'));
        })
        .catch(() => {});
    }, 200);
    return () => {
      clearTimeout(tm);
      ctl.abort();
    };
  }, [project, path, value, onCheck]);
  // The backdrop decides the height; the text area lies on it, scrolled with it.
  useLayoutEffect(() => {
    if (ref.current && back.current) ref.current.style.height = `${back.current.offsetHeight}px`;
  });
  const errors = lines.filter((l) => l.kind === 'error');
  return (
    <>
      <div className={s.editor}>
        <div ref={back} className={`${s.text} ${s.backdrop}`} aria-hidden="true">
          <Highlight lines={lines} text={value} />
        </div>
        <textarea
          ref={ref}
          className={`${s.text} ${s.input}`}
          value={value}
          spellCheck={false}
          aria-label={t.metric}
          onChange={(e) => onChange(e.target.value)}
        />
      </div>
      <div className={`${s.check} ${errors.length ? s.bad : ''}`} role="status">
        {lines.length === 0 ? null : errors.length ? (
          errors.map((l) => (
            <div key={l.n}>
              <Icon name="x" size={14} />
              {t.metricLineError(l.n, l.error ?? '')}
            </div>
          ))
        ) : (
          <div>
            <Icon name="check" size={14} />
            {t.metricUnderstood}
          </div>
        )}
      </div>
    </>
  );
}
