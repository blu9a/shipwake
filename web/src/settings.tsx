// Viewer settings: language and appearance. Remembered in the browser; nothing else is stored.
import { createContext, type ReactNode, useContext, useEffect, useState } from 'react';
import { type Lang, messages } from './messages';

export type Mode = 'auto' | 'light' | 'dark';

export type Baseline = 'previous' | 'release';

interface Settings {
  lang: Lang;
  mode: Mode;
  baseline: Baseline;
  setLang: (l: Lang) => void;
  setMode: (m: Mode) => void;
  setBaseline: (b: Baseline) => void;
}

const read = (key: string) => {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
};
const write = (key: string, value: string) => {
  try {
    localStorage.setItem(key, value);
  } catch {
    // private mode: settings last for this visit only
  }
};

const Ctx = createContext<Settings | null>(null);

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>(() => {
    const saved = read('lang');
    if (saved === 'en' || saved === 'zh') return saved;
    return navigator.language.startsWith('zh') ? 'zh' : 'en';
  });
  const [mode, setMode] = useState<Mode>(() => (read('mode') as Mode) || 'auto');
  // The last release by default; a run without one falls back to the previous run.
  const [baseline, setBaseline] = useState<Baseline>(() => (read('baseline') === 'previous' ? 'previous' : 'release'));
  useEffect(() => write('baseline', baseline), [baseline]);
  useEffect(() => {
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
    write('lang', lang);
  }, [lang]);
  useEffect(() => {
    if (mode === 'auto') delete document.documentElement.dataset.mode;
    else document.documentElement.dataset.mode = mode;
    write('mode', mode);
  }, [mode]);
  return <Ctx value={{ lang, mode, baseline, setLang, setMode, setBaseline }}>{children}</Ctx>;
}

export function useSettings() {
  const s = useContext(Ctx);
  if (!s) throw new Error('useSettings outside SettingsProvider');
  return s;
}

/** The tool's own text in the viewer's language. */
export function useT() {
  return messages[useSettings().lang];
}
