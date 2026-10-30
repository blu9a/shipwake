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

// Blank stretches: a test's recording starts when its browser page opens, often long before the page shows
// anything (the test is still setting up), and goes blank whenever the page loads again. A blank screen
// looks frozen or broken; the page it was about to show, or the one it just showed, doesn't.

/** A stretch where the recording shows a single color, and the picture to show instead. */
interface Blank {
  start: number;
  end: number;
  still?: string; // the last picture before it; none before the page first shows something
}

/** Whether a small sample of a frame is one color: hardly any pixel differs from the commonest. */
const isBlank = (data: Uint8ClampedArray) => {
  const counts = new Map<number, number>();
  const key = (i: number) => ((data[i] ?? 0) >> 4) * 256 + ((data[i + 1] ?? 0) >> 4) * 16 + ((data[i + 2] ?? 0) >> 4);
  for (let i = 0; i < data.length; i += 4) counts.set(key(i), (counts.get(key(i)) ?? 0) + 1);
  const most = Math.max(...counts.values());
  return most >= (data.length / 4) * 0.997;
};

const once = (el: EventTarget, event: string) => new Promise<void>((ok) => el.addEventListener(event, () => ok(), { once: true }));

/** Finds the blank stretches of a recording by sampling a hidden copy of it, about ten times a second. */
async function findBlanks(src: string, signal: AbortSignal): Promise<Blank[]> {
  const v = document.createElement('video');
  v.muted = true;
  v.preload = 'auto';
  v.src = src;
  await once(v, 'loadeddata');
  if (!Number.isFinite(v.duration)) {
    // recordings written as they're made may not state their length until read to the end
    v.currentTime = 1e7;
    await once(v, 'seeked');
  }
  const duration = v.duration;
  const step = Math.max(0.1, duration / 400);
  const small = document.createElement('canvas');
  small.width = 48;
  small.height = 27;
  const ctx = small.getContext('2d', { willReadFrequently: true });
  if (!ctx || !Number.isFinite(duration)) return [];
  const picture = () => {
    const c = document.createElement('canvas');
    c.width = v.videoWidth;
    c.height = v.videoHeight;
    c.getContext('2d')?.drawImage(v, 0, 0);
    return c.toDataURL('image/jpeg', 0.9);
  };
  const seek = async (t: number) => {
    v.currentTime = t;
    await once(v, 'seeked');
  };
  const out: Blank[] = [];
  let good: number | undefined; // the last sample that showed something
  let open: Blank | undefined;
  for (let t = 0; t < duration && !signal.aborted; t += step) {
    await seek(t);
    ctx.drawImage(v, 0, 0, small.width, small.height);
    const blank = isBlank(ctx.getImageData(0, 0, small.width, small.height).data);
    if (blank && !open) {
      open = { start: t, end: duration };
      if (good !== undefined) {
        await seek(good); // the picture it holds: the last one before it
        open.still = picture();
        await seek(t);
      }
    }
    if (!blank) {
      good = t;
      if (open) {
        open.end = t;
        out.push(open);
        open = undefined;
      }
    }
  }
  if (open) out.push(open);
  v.removeAttribute('src');
  v.load();
  return out;
}

/** Keeps a recording from showing blank screens: it starts where the page first shows something, and a
 *  blank stretch later on shows the last picture instead, held until the page is back. Subtitles stay in
 *  step, since the recording itself plays on underneath. The picture to show now, if any. */
export function useNoBlanks(ref: RefObject<HTMLVideoElement | null>, src: string) {
  const [blanks, setBlanks] = useState<Blank[]>([]);
  const [still, setStill] = useState<string>();
  useEffect(() => {
    const ctl = new AbortController();
    setBlanks([]);
    setStill(undefined);
    if (!src) return;
    findBlanks(src, ctl.signal)
      .then((b) => !ctl.signal.aborted && setBlanks(b))
      .catch(() => {}); // unreadable: shown as recorded
    return () => ctl.abort();
  }, [src]);
  useEffect(() => {
    const v = ref.current;
    if (!v || blanks.length === 0) return;
    let frame = 0;
    const check = () => {
      const t = v.currentTime;
      const b = blanks.find((x) => t >= x.start - 0.05 && t < x.end);
      if (b && !b.still && !(v.duration <= b.end)) {
        // (a length not known yet doesn't rule it out)
        v.currentTime = b.end; // before the page showed anything: start where it does
        return;
      }
      setStill(b?.still);
    };
    const tick = () => {
      check();
      frame = requestAnimationFrame(tick);
    };
    const events = ['seeked', 'loadedmetadata', 'loadeddata', 'timeupdate'];
    for (const e of events) v.addEventListener(e, check);
    const play = () => {
      cancelAnimationFrame(frame);
      tick();
    };
    const stop = () => cancelAnimationFrame(frame);
    v.addEventListener('play', play);
    v.addEventListener('pause', stop);
    v.addEventListener('ended', stop);
    check();
    if (!v.paused) play();
    return () => {
      stop();
      for (const e of events) v.removeEventListener(e, check);
      v.removeEventListener('play', play);
      v.removeEventListener('pause', stop);
      v.removeEventListener('ended', stop);
    };
  }, [ref, blanks]);
  return still;
}

/** The picture held over a blank stretch, in the video's place. */
export function Still({ src }: { src?: string }) {
  if (!src) return null;
  return <img className={s.still} src={src} alt="" aria-hidden="true" data-still="" />;
}
