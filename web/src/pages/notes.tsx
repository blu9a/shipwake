// Release notes live in the annotated tag: the first language, then "--- <lang>" sections; trailers last.
import type { ReactNode } from 'react';
import type { Project, Tag } from '../model.gen';
import { useSettings } from '../settings';
import s from './Timeline.module.css';

export function parseNotes(p: Project, tag: Tag) {
  const lines = `${tag.subject}\n\n${tag.body ?? ''}`.trim().split('\n');
  const trailers: Record<string, string> = {};
  while (lines.length && /^[\w-]+: /.test(lines.at(-1) ?? '')) {
    const [k, ...v] = (lines.pop() as string).split(': ');
    trailers[k as string] = v.join(': ');
  }
  const parts = lines
    .join('\n')
    .trim()
    .split(/\n-{3,} ?([\w-]*)\n/);
  const byLang: Record<string, string> = { [p.languages[0] ?? 'en']: (parts[0] ?? '').trim() };
  for (let i = 1; i < parts.length; i += 2) byLang[parts[i] || p.languages[(i + 1) / 2] || 'x'] = (parts[i + 1] ?? '').trim();
  return { byLang, trailers };
}

// Inline Markdown as changelog tools write it: links (web addresses only), bold and code.
const inlineRe = /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)|\*\*([^*]+)\*\*|`([^`]+)`/g;

/** A line's text with its links, bold and code. */
export function Inline({ text }: { text: string }) {
  const out: ReactNode[] = [];
  let at = 0;
  for (const m of text.matchAll(inlineRe)) {
    if (m.index > at) out.push(text.slice(at, m.index));
    const [, label, href, bold, code] = m;
    if (href)
      out.push(
        <a key={m.index} className="lnk" href={href} target="_blank" rel="noopener noreferrer">
          <Inline text={label ?? ''} />
        </a>,
      );
    else if (bold) out.push(<b key={m.index}>{bold}</b>);
    else out.push(<code key={m.index}>{code}</code>);
    at = m.index + m[0].length;
  }
  if (at < text.length) out.push(text.slice(at));
  return out;
}

/** Light Markdown: "### " lines are section headings, "- " lines a list, a single first line the title,
 *  the rest paragraphs; links, bold and code inside them. A list item ending in a commit, "(7d5611a)",
 *  shows it as code. */
export function NotesText({ text }: { text: string }) {
  const blocks = text
    .trim()
    .split(/\n\s*\n/)
    .map((b) => b.trim())
    .filter(Boolean);
  const bullet = (l: string) => /^[-*•]\s+/.test(l);
  const heading = (l: string) => /^#{1,6}\s+/.test(l);
  const item = (l: string) => {
    const text = l.replace(/^[-*•]\s+/, '');
    const m = text.match(/^(.*\S)\s+\(([0-9a-f]{7,40})\)$/);
    return m ? (
      <>
        <Inline text={m[1] ?? ''} /> <code>{m[2]}</code>
      </>
    ) : (
      <Inline text={text} />
    );
  };
  return blocks.map((blk, i) => {
    const lines = blk.split('\n');
    if (lines.length === 1 && heading(blk))
      return (
        <h4 key={blk} className={s.notesHeading}>
          <Inline text={blk.replace(/^#+\s*/, '')} />
        </h4>
      );
    if (lines.every(bullet))
      return (
        <ul key={blk} className={s.notesList}>
          {lines.map((l) => (
            <li key={l}>{item(l)}</li>
          ))}
        </ul>
      );
    if (i === 0 && lines.length === 1 && blocks.length > 1)
      return (
        <p key={blk} className={s.notesTitle}>
          <Inline text={lines[0]?.replace(/^#+\s*/, '') ?? ''} />
        </p>
      );
    return (
      <p key={blk} className={s.notesBody}>
        <Inline text={lines.join(' ')} />
      </p>
    );
  });
}

/** A release's notes: its section in the changelog file when the file has one, else its tag's message. */
export function ReleaseNotes({ p, tag }: { p: Project; tag: Tag }) {
  const { lang } = useSettings();
  const n = tag.notes && Object.keys(tag.notes).length ? { byLang: tag.notes } : parseNotes(p, tag);
  const text = n.byLang[lang] || n.byLang[p.languages[0] ?? 'en'] || '';
  return <NotesText text={text} />;
}
