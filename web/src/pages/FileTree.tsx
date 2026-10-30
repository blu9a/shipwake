// The changed files as a folder tree. Folders fold; a chain of folders holding only one folder
// shows as one row ("internal/site/dist"), as on GitHub.
import { useState } from 'react';
import type { FileChange } from '../model.gen';
import { Icon } from '../ui';
import s from './CommitPage.module.css';

interface Dir {
  name: string;
  path: string;
  dirs: Dir[];
  files: FileChange[];
}

function build(files: FileChange[]): Dir {
  const root: Dir = { name: '', path: '', dirs: [], files: [] };
  for (const f of files) {
    const parts = f.path.split('/');
    let d = root;
    for (const name of parts.slice(0, -1)) {
      const path = d.path ? `${d.path}/${name}` : name;
      let next = d.dirs.find((x) => x.name === name);
      if (!next) {
        next = { name, path, dirs: [], files: [] };
        d.dirs.push(next);
      }
      d = next;
    }
    d.files.push(f);
  }
  const compact = (d: Dir): Dir => {
    let c = d;
    while (c.dirs.length === 1 && c.files.length === 0 && c !== root) {
      const only = c.dirs[0] as Dir;
      c = { ...only, name: `${c.name}/${only.name}` };
    }
    return { ...c, dirs: c.dirs.map(compact).sort((a, b) => a.name.localeCompare(b.name)) };
  };
  return compact(root);
}

export function FileTree({ files, active, onPick }: { files: FileChange[]; active?: string; onPick: (path: string) => void }) {
  const [shut, setShut] = useState<Set<string>>(() => new Set());
  const toggle = (path: string) =>
    setShut((prev) => {
      const next = new Set(prev);
      if (!next.delete(path)) next.add(path);
      return next;
    });
  const walk = (d: Dir) => (
    <>
      {d.dirs.map((x) => {
        const closed = shut.has(x.path);
        return (
          <div key={x.path}>
            <button type="button" className={s.tr} aria-expanded={!closed} onClick={() => toggle(x.path)}>
              <span className={`${s.chev} ${closed ? s.chevShut : ''}`}>
                <Icon name="down" size={14} />
              </span>
              <FolderIcon />
              <span className={s.nm} title={x.path}>
                {x.name}
              </span>
            </button>
            {!closed && <div className={s.kids}>{walk(x)}</div>}
          </div>
        );
      })}
      {d.files.map((f) => (
        <button
          type="button"
          key={f.path}
          aria-current={f.path === active ? 'true' : undefined}
          className={`${s.tr} ${f.path === active ? s.on : ''}`}
          onClick={() => onPick(f.path)}
        >
          <span className={s.chev} />
          <FileIcon status={f.status} />
          <span className={s.nm} title={f.path}>
            {f.path.split('/').at(-1)}
          </span>
        </button>
      ))}
    </>
  );
  return <div className={s.treeList}>{walk(build(files))}</div>;
}

function FolderIcon() {
  return (
    <svg className={s.dirIc} width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
      <path d="M1.5 3.5a1 1 0 0 1 1-1h3.6l1.4 1.5h6a1 1 0 0 1 1 1v7.5a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1z" fill="currentColor" />
    </svg>
  );
}

/** A page with a mark for what happened to the file: added, deleted, renamed or modified. */
function FileIcon({ status }: { status: string }) {
  const st = status[0] ?? 'M';
  return (
    <svg
      className={`${s.fileIc} ${s[`st${st}`] ?? ''}`}
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.3"
      strokeLinejoin="round"
      strokeLinecap="round"
      aria-hidden="true"
    >
      <path d="M3.5 1.5h6l3 3v10h-9z" />
      {st === 'A' && <path d="M8 6.5v5M5.5 9h5" />}
      {st === 'D' && <path d="M5.5 9h5" />}
      {st === 'R' && <path d="M5.5 9h5M8.5 7l2 2-2 2" />}
      {st !== 'A' && st !== 'D' && st !== 'R' && <circle cx="8" cy="9" r="1.6" fill="currentColor" stroke="none" />}
    </svg>
  );
}
