// Opening a repository by its path, when served by `shipwake serve`. A repository not set up for
// Shipwake opens in local mode: nothing is written into it.
import { useNavigate } from '@tanstack/react-router';
import { use, useState } from 'react';
import { forgetSite, loadSession } from '../data';
import { useT } from '../settings';
import s from './OpenPage.module.css';

export function OpenPage() {
  const session = use(loadSession());
  const t = useT();
  const navigate = useNavigate();
  const [path, setPath] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  if (!session.edit) return <p className="muted">{t.notFound}</p>;
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy || !path.trim()) return;
    setBusy(true);
    setError('');
    try {
      const r = await fetch('api/open', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error ?? r.statusText);
      forgetSite();
      await navigate({ to: '/$project', params: { project: d.project } });
    } catch (err) {
      setError(String((err as Error).message));
    } finally {
      setBusy(false);
    }
  };
  return (
    <form className={s.form} onSubmit={submit}>
      <h1 className={s.title}>{t.openRepo}</h1>
      <label className={s.label}>
        <span>{t.repoPath}</span>
        <input
          className="field mono"
          value={path}
          spellCheck={false}
          autoComplete="off"
          placeholder="~/code/app"
          onChange={(e) => setPath(e.target.value)}
        />
      </label>
      <div>
        <button type="submit" className={s.primary} disabled={busy || !path.trim()}>
          {t.open}
        </button>
      </div>
      {error && (
        <div className={s.error} role="alert">
          {error}
        </div>
      )}
    </form>
  );
}
