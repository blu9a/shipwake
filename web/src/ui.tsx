// Small shared pieces: icons, a popover menu, segmented controls, result counts.
import { type ReactNode, useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { useT } from './settings';
import s from './ui.module.css';

/** One-line names: ellipsis at rest, the complete text scrolls on hover or focus. */
export function ScrollName({ children, className = '' }: { children: string; className?: string }) {
  const box = useRef<HTMLSpanElement>(null);
  const text = useRef<HTMLSpanElement>(null);
  const [distance, setDistance] = useState(0);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [scrolling, setScrolling] = useState(false);
  const active = hovered || focused;
  useLayoutEffect(() => {
    const el = box.current;
    const label = text.current;
    if (!el || !label) return;
    const measure = () => setDistance(children ? Math.max(0, label.scrollWidth - el.clientWidth) : 0);
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    ro.observe(label);
    measure();
    return () => ro.disconnect();
  }, [children]);
  useLayoutEffect(() => {
    const label = text.current;
    if (!label) return;
    const media = matchMedia('(prefers-reduced-motion: reduce)');
    let motion: Animation | undefined;
    const update = () => {
      motion?.cancel();
      const run = active && distance > 1 && !media.matches;
      setScrolling(run);
      if (!run) return;
      const pause = 400;
      const duration = (distance / 36) * 1000 + pause * 2;
      motion = label.animate(
        [
          { transform: 'translateX(0)', offset: 0 },
          { transform: 'translateX(0)', offset: pause / duration },
          { transform: `translateX(${-distance}px)`, offset: 1 - pause / duration },
          { transform: `translateX(${-distance}px)`, offset: 1 },
        ],
        { duration, iterations: Infinity, direction: 'alternate', easing: 'linear' },
      );
    };
    update();
    media.addEventListener('change', update);
    return () => {
      motion?.cancel();
      media.removeEventListener('change', update);
    };
  }, [active, distance]);
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: hover/focus reveal text; the enclosing summary owns disclosure.
    <span
      ref={box}
      className={`${s.scrollName} ${className}`}
      title={children}
      data-scroll-name=""
      data-scrolling={scrolling || undefined}
      tabIndex={distance > 1 || focused ? 0 : undefined}
      onPointerEnter={(e) => {
        if (e.pointerType !== 'touch') setHovered(true);
      }}
      onPointerLeave={() => setHovered(false)}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
    >
      <span ref={text}>{children}</span>
    </span>
  );
}

const paths = {
  branch: (
    <>
      <circle cx="6" cy="5" r="2.2" />
      <circle cx="6" cy="19" r="2.2" />
      <circle cx="18" cy="7" r="2.2" />
      <path d="M6 7.2v9.6M18 9.2c0 5-12 3-12 7.6" />
    </>
  ),
  down: <path d="M6 9l6 6 6-6" />,
  tag: (
    <>
      <path d="M3 12V4h8l10 10-8 8L3 12z" />
      <circle cx="7.5" cy="8.5" r="1.4" fill="currentColor" />
    </>
  ),
  beaker: (
    <>
      <path d="M9 3h6M10 3v6l-5.5 9.5A1.7 1.7 0 0 0 6 21h12a1.7 1.7 0 0 0 1.5-2.5L14 9V3" />
      <path d="M7.5 15h9" />
    </>
  ),
  bolt: <path d="M13 2.5L4.5 13.5H11l-1 8 8.5-11H12z" />,
  gauge: (
    <>
      <path d="M4.5 18a9 9 0 1 1 15 0" />
      <path d="M12 13l4-5" />
    </>
  ),
  timer: (
    <>
      <circle cx="12" cy="13.5" r="7.5" />
      <path d="M12 13.5V9.5M9.5 2.5h5" />
    </>
  ),
  shield: <path d="M12 3l7.5 3v5.5c0 4.5-3.2 8.2-7.5 9.5-4.3-1.3-7.5-5-7.5-9.5V6z" />,
  chart: <path d="M3.5 18.5l5.5-6 4 3.5 7.5-9" />,
  panel: (
    <>
      <rect x="3.5" y="4.5" width="17" height="15" rx="3" />
      <path d="M9.5 4.5v15" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6" />
      <path d="M20 20l-4.5-4.5" />
    </>
  ),
  check: <path d="M5 12.5l4.5 4.5L19 7.5" />,
  agent: (
    <>
      <path d="M12 3.5l1.9 5 5 1.9-5 1.9-1.9 5-1.9-5-5-1.9 5-1.9z" />
      <path d="M18.5 15.5l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z" />
    </>
  ),
  plus: <path d="M12 5v14M5 12h14" />,
  copy: (
    <>
      <rect x="8.5" y="8.5" width="11" height="11" rx="2" />
      <path d="M15.5 8.5V6.5a2 2 0 0 0-2-2h-7a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h2" />
    </>
  ),
  x: <path d="M6 6l12 12M18 6L6 18" />,
  play: <path d="M7 4.5v15l12-7.5z" />,
  clip: <path d="M20 11.5l-8.2 8.2a5 5 0 0 1-7.1-7.1l8.6-8.6a3.4 3.4 0 0 1 4.8 4.8l-8.5 8.5a1.7 1.7 0 0 1-2.4-2.4l7.8-7.8" />,
  sub: (
    <>
      <path d="M5 4v7a3 3 0 0 0 3 3h11" />
      <path d="M15 10l4 4-4 4" />
    </>
  ),
  film: (
    <>
      <rect x="3.5" y="5.5" width="17" height="13" rx="3" />
      <path d="M10 9.5v5l4.5-2.5z" fill="currentColor" />
    </>
  ),
  file: <path d="M6 2.5h8l4 4v15H6z" />,
  expand: <path d="M14 4h6v6M20 4l-7 7M10 20H4v-6M4 20l7-7" />,
  download: <path d="M12 4v11M7.5 10.5L12 15l4.5-4.5M5 19.5h14" />,
  bang: (
    <>
      <path d="M12 5v8" />
      <circle cx="12" cy="18.5" r="2" fill="currentColor" stroke="none" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3h0a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5h0a1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8v0a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
    </>
  ),
  image: (
    <>
      <rect x="3.5" y="3.5" width="17" height="17" rx="3" />
      <circle cx="9" cy="9" r="1.8" />
      <path d="M20.5 15.5l-5-5L5 21" />
    </>
  ),
} as const;

export type IconName = keyof typeof paths;

const kindIcons: Record<string, IconName> = {
  test: 'beaker',
  eval: 'gauge',
  'load test': 'bolt',
  perf: 'bolt',
  benchmark: 'timer',
  bench: 'timer',
  security: 'shield',
};

/** A run kind's icon: the project's choice, else a known kind's, else by what the row shows. */
export function kindIcon(configured: string | undefined, kind: string, shows: 'tests' | 'metrics'): IconName {
  if (configured === 'test') return 'beaker';
  if (configured && configured in paths) return configured as IconName;
  return kindIcons[kind.toLowerCase()] ?? (shows === 'tests' ? 'beaker' : 'chart');
}

export function Icon({ name, size = 16, weight = 2 }: { name: IconName; size?: number; weight?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={weight}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  );
}

/** A button that opens a popover; closes on outside click, Escape, or choosing. */
export function Menu({
  label,
  button,
  children,
  align = 'start',
}: {
  label: string;
  button: ReactNode;
  children: (close: () => void) => ReactNode;
  align?: 'start' | 'end';
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const id = useId();
  useLayoutEffect(() => {
    const anchor = ref.current;
    const menu = menuRef.current;
    if (!open || !anchor || !menu) return;
    // Keep the intrinsic width and trigger alignment, shifting only when an edge
    // would leave the viewport. Recheck after resize, label or language changes.
    const place = () => {
      const rect = anchor.getBoundingClientRect();
      const width = menu.getBoundingClientRect().width;
      const start = align === 'end' ? rect.right - width : rect.left;
      const left = Math.max(16, Math.min(start, document.documentElement.clientWidth - 16 - width));
      menu.style.setProperty('--menu-shift', `${left - start}px`);
    };
    place();
    const observer = new ResizeObserver(place);
    observer.observe(anchor);
    observer.observe(menu);
    window.addEventListener('resize', place);
    return () => {
      observer.disconnect();
      window.removeEventListener('resize', place);
    };
  }, [open, align]);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('pointerdown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);
  return (
    <div className={s.menuWrap} ref={ref}>
      <button
        type="button"
        className={s.menuButton}
        aria-label={label}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen(!open)}
      >
        {button}
      </button>
      {open && (
        <div id={id} ref={menuRef} className={`${s.menu} ${align === 'end' ? s.end : ''}`}>
          {children(() => setOpen(false))}
        </div>
      )}
    </div>
  );
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { value: T; label: ReactNode; disabled?: boolean; title?: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className={s.segmented}>
      {options.map((o) => (
        <button
          type="button"
          key={o.value}
          className={o.value === value ? s.on : ''}
          aria-pressed={o.value === value}
          disabled={o.disabled}
          title={o.title}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** Counts as coloured bars and numbers: failed (or blocking), warnings, passed. Zero counts are left out. */
export function Tally({
  failed = 0,
  warn = 0,
  passed = 0,
  measured = false,
}: {
  failed?: number;
  warn?: number;
  passed?: number;
  measured?: boolean; // counts of measurements: blocking, warning, within limits
}) {
  const t = useT();
  const count = (n: number, cls: string | undefined, label: string) =>
    n > 0 && (
      <span className={`${s.cnt} ${cls}`} role="img" aria-label={label} title={label}>
        <i />
        {n}
      </span>
    );
  return (
    <span className={s.tally}>
      {count(failed, s.f, measured ? t.counts.block(failed) : t.counts.failed(failed))}
      {count(warn, s.w, t.counts.warn(warn))}
      {count(passed, s.p, measured ? t.counts.ok(passed) : t.counts.passed(passed))}
    </span>
  );
}

export function StatusIcon({ status }: { status: string }) {
  return (
    <span className={`${s.status} ${s[status] ?? ''}`}>
      {/* one style for all three: a white mark in a circle colored by status */}
      <Icon name={status === 'warn' ? 'bang' : status === 'failed' ? 'x' : 'check'} size={status === 'failed' ? 10 : 12} weight={3.2} />
    </span>
  );
}

export function Loading() {
  return <div className={s.loading} aria-busy="true" />;
}

export { s as ui };
