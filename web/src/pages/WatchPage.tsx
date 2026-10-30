// A feature's recordings in one run, played back to back with a chapter for each scenario.
import { Link, useParams, useRouter } from '@tanstack/react-router';
import { use, useEffect, useRef, useState } from 'react';
import { fileUrl, loadProject, loadRun } from '../data';
import { targetName, tr } from '../format';
import type { Run } from '../model.gen';
import { useSettings, useT } from '../settings';
import { Icon, StatusIcon } from '../ui';
import { Caption, Still, useApplySettings, useNoBlanks, useSubtitles, VideoControls } from '../video';
import s from './WatchPage.module.css';

/** The clips the feature's video is made of: one target's passing recordings, in the order they ran. */
export function featureClips(run: Run, feature: string) {
  const withVideo = (run.tests ?? []).flatMap((x) => {
    const v = (x.attachments ?? []).find((a) => a.type.startsWith('video/'));
    return v && (x.feature ?? '') === feature ? [{ x, path: v.path, video: v }] : [];
  });
  const target = withVideo[0]?.x.target;
  return withVideo.filter((c) => c.x.target === target).sort((a, b) => (a.x.start ?? '').localeCompare(b.x.start ?? ''));
}

export function WatchPage() {
  const { project, run: id, _splat: feature = '' } = useParams({ from: '/$project/run/$run/watch/$' });
  const p = use(loadProject(project));
  const run = use(loadRun(project, id));
  const { lang } = useSettings();
  const t = useT();
  const router = useRouter();
  const clips = featureClips(run, feature);
  const [at, setAt] = useState(0);
  const [progress, setProgress] = useState(0);
  const [card, setCard] = useState(true);
  const ref = useRef<HTMLVideoElement>(null);
  useApplySettings(ref);
  const clip = clips[at];
  const subs = useSubtitles(p, clip?.x, clip?.video);
  const still = useNoBlanks(ref, clip ? fileUrl(project, run.id, clip.path) : '');
  // The scenario's name shows over the video for a moment whenever a clip starts.
  useEffect(() => {
    if (!card) return;
    const tm = setTimeout(() => setCard(false), 2200);
    return () => clearTimeout(tm);
  }, [card]);
  const close = () => router.history.back();
  const next = useRef(0);
  const go = (i: number) => {
    clearTimeout(next.current);
    setAt(i);
    setProgress(0);
    setCard(true);
  };
  return (
    <div className={s.viewer}>
      <button type="button" className={s.close} aria-label={t.close} onClick={close}>
        <Icon name="x" size={18} />
      </button>
      {clip ? (
        <div className={s.player}>
          <div className={s.main}>
            <div className={s.frame}>
              <video
                ref={ref}
                className={s.video}
                src={fileUrl(project, run.id, clip.path)}
                controls
                autoPlay
                muted
                playsInline
                onTimeUpdate={(e) => setProgress(e.currentTarget.duration ? e.currentTarget.currentTime / e.currentTarget.duration : 0)}
                // the result stays on screen a moment before the next scenario
                onEnded={() => {
                  if (at + 1 < clips.length) next.current = window.setTimeout(() => go(at + 1), 1200);
                }}
              >
                {subs.url && <track key={subs.url} kind="subtitles" srcLang={subs.lang} label={subs.lang} src={subs.url} />}
              </video>
              <Still src={still} />
              <Caption video={ref} />
              <div className={`${s.card} ${card ? s.show : ''}`}>
                {at + 1} / {clips.length} · {tr(p, clip.x.name, lang)}
              </div>
            </div>
            <div className={s.bar}>
              {clips.map((c, i) => (
                <button
                  type="button"
                  key={`${c.x.name}|${c.x.start ?? ''}`}
                  className={s.seg}
                  title={tr(p, c.x.name, lang)}
                  aria-label={tr(p, c.x.name, lang)}
                  onClick={() => go(i)}
                >
                  <i style={{ width: `${i < at ? 100 : i === at ? progress * 100 : 0}%` }} />
                </button>
              ))}
            </div>
            <VideoControls dark />
          </div>
          <div className={s.side}>
            <div className={s.title}>{tr(p, feature, lang) || '—'}</div>
            {clip.x.target && <div className={s.sub}>{targetName(p, clip.x.target, lang)}</div>}
            <ol className={s.chapters}>
              {clips.map((c, i) => (
                <li key={`${c.x.name}|${c.x.start ?? ''}`}>
                  <button type="button" className={i === at ? s.on : undefined} onClick={() => go(i)}>
                    <StatusIcon status={c.x.status} />
                    <span>{tr(p, c.x.name, lang)}</span>
                  </button>
                </li>
              ))}
            </ol>
          </div>
        </div>
      ) : (
        <p className={s.sub}>
          <Link to="/$project/run/$run" params={{ project, run: id }}>
            {tr(p, run.name, lang)}
          </Link>
        </p>
      )}
    </div>
  );
}
