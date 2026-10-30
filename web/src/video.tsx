// Recordings: speed and subtitles are viewer settings, remembered; subtitles start off. Subtitles are the
// scenario's steps, timed from when each began, so nothing beyond the recording is stored. The page draws
// them in one place; browsers place their own by their controls, so theirs show only in full screen.
import { type RefObject, useEffect, useMemo, useState } from 'react';
import { tr } from './format';
import type { Lang } from './messages';
import type { Attachment, Project, Test } from './model.gen';
import { useSettings, useT } from './settings';
import { Segmented } from './ui';
import s from './video.module.css';

const speeds = ['0.5', '0.75', '1', '2'] as const;
type Speed = (typeof speeds)[number];

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
    // private mode: for this visit only
  }
};

let shared = {
  speed: (speeds as readonly string[]).includes(read('vspeed') ?? '') ? (read('vspeed') as Speed) : '0.5',
  subs: read('vsubs') === '1',
};
const listeners = new Set<() => void>();

/** The viewer's speed and subtitles, the same for every video on the page. */
export function useVideoSettings() {
  const [v, setV] = useState(shared);
  useEffect(() => {
    const on = () => setV(shared);
    listeners.add(on);
    return () => {
      listeners.delete(on);
    };
  }, []);
  const set = (next: Partial<typeof shared>) => {
    shared = { ...shared, ...next };
    write('vspeed', shared.speed);
    write('vsubs', shared.subs ? '1' : '');
    for (const l of listeners) l();
  };
  return { ...v, set };
}

/** Applies the viewer's settings to a video, now and whenever it loads another clip. */
export function useApplySettings(ref: RefObject<HTMLVideoElement | null>) {
  const { speed, subs } = useVideoSettings();
  useEffect(() => {
    const v = ref.current;
    if (!v) return;
    const apply = () => {
      v.playbackRate = Number(speed);
      v.defaultPlaybackRate = Number(speed);
      const own = fullScreen(v) ? 'showing' : 'hidden'; // hidden still reports its cues to the page
      for (const track of Array.from(v.textTracks)) track.mode = subs ? own : 'disabled';
    };
    apply();
    const events = ['loadedmetadata', 'play', 'webkitbeginfullscreen', 'webkitendfullscreen'];
    for (const e of events) v.addEventListener(e, apply);
    v.textTracks.addEventListener('addtrack', apply);
    document.addEventListener('fullscreenchange', apply);
    return () => {
      for (const e of events) v.removeEventListener(e, apply);
      v.textTracks.removeEventListener('addtrack', apply);
      document.removeEventListener('fullscreenchange', apply);
    };
  }, [ref, speed, subs]);
}

const fullScreen = (v: HTMLVideoElement) =>
  document.fullscreenElement === v || !!(v as HTMLVideoElement & { webkitDisplayingFullscreen?: boolean }).webkitDisplayingFullscreen;

/** The subtitle for this moment, drawn by the page over the video's lower part. */
export function Caption({ video }: { video: RefObject<HTMLVideoElement | null> }) {
  const { subs } = useVideoSettings();
  const [text, setText] = useState('');
  useEffect(() => {
    const v = video.current;
    if (!v) return;
    const update = () => {
      const track = v.textTracks[0];
      const cues = track && track.mode !== 'disabled' && !fullScreen(v) ? Array.from(track.activeCues ?? []) : [];
      setText(cues.map((c) => (c as VTTCue).text).join('\n'));
    };
    const watch = () => {
      for (const track of Array.from(v.textTracks)) track.oncuechange = update;
      update();
    };
    watch();
    v.textTracks.addEventListener('addtrack', watch);
    v.addEventListener('seeked', update);
    document.addEventListener('fullscreenchange', update);
    return () => {
      v.textTracks.removeEventListener('addtrack', watch);
      v.removeEventListener('seeked', update);
      document.removeEventListener('fullscreenchange', update);
    };
  }, [video]);
  if (!subs || !text) return null;
  return (
    <div className={s.caption} aria-hidden="true">
      <span>{text}</span>
    </div>
  );
}

export function VideoControls({ dark }: { dark?: boolean }) {
  const t = useT();
  const { speed, subs, set } = useVideoSettings();
  return (
    <div className={`${s.controls} ${dark ? s.dark : ''}`}>
      <span className={s.pair}>
        <span className={s.label}>{t.speed}</span>
        <Segmented value={speed} onChange={(v) => set({ speed: v })} options={speeds.map((v) => ({ value: v, label: `${v}×` }))} />
      </span>
      <span className={s.pair}>
        <span className={s.label}>{t.subtitles}</span>
        <Segmented
          value={subs ? 'on' : 'off'}
          onChange={(v) => set({ subs: v === 'on' })}
          options={[
            { value: 'off', label: t.off },
            { value: 'on', label: t.on },
          ]}
        />
      </span>
    </div>
  );
}

const stamp = (sec: number) => {
  const ms = Math.max(0, Math.round(sec * 1000));
  const h = Math.floor(ms / 3600000);
  const m = Math.floor(ms / 60000) % 60;
  const x = (ms / 1000) % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${x.toFixed(3).padStart(6, '0')}`;
};

/** WebVTT from the test's steps: each step from when it began, relative to when the recording began.
 *  Runs recorded before steps had start times are estimated: steps back to back, ending with the test. */
export function subtitles(p: Project, x: Test, lang: Lang, recording?: Attachment) {
  const steps = x.steps ?? [];
  if (!steps.length) return '';
  const began = recording?.start ? Date.parse(recording.start) : Number.NaN;
  const timed = !Number.isNaN(began) && steps.every((st) => st.start);
  let at = Math.max(0, x.duration - steps.reduce((a, st) => a + st.duration, 0)) / 1000;
  const cues = steps.map((st) => {
    const from = timed ? (Date.parse(st.start ?? '') - began) / 1000 : at;
    at = from + st.duration / 1000;
    return `${stamp(from)} --> ${stamp(at)}\n${tr(p, st.name, lang)}`;
  });
  return `WEBVTT\n\n${cues.join('\n\n')}\n`;
}

/** A blob URL for the test's subtitles, released when no longer used. */
export function useSubtitles(p: Project, x: Test | undefined, recording?: Attachment) {
  const { lang } = useSettings();
  const url = useMemo(() => {
    const text = x ? subtitles(p, x, lang, recording) : '';
    return text ? URL.createObjectURL(new Blob([text], { type: 'text/vtt' })) : undefined;
  }, [p, x, lang, recording]);
  useEffect(
    () => () => {
      if (url) URL.revokeObjectURL(url);
    },
    [url],
  );
  return { url, lang };
}
