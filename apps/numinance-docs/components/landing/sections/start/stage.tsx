'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { onThemeChange, readPalette } from '@/lib/landing/palette';
import { scopedStatus } from '@/lib/landing/status';
const setStatus = scopedStatus('start');
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { Crt } from './crt';
import { paintScene } from './scene';
import { visible } from './script';

const clamp = (v: number, a = 0, b = 1) => Math.min(b, Math.max(a, v));
const smooth = (v: number, a: number, b: number) => {
  const t = clamp((v - a) / (b - a));
  return t * t * (3 - 2 * t);
};
const inOut = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);

const CELL = 4;

/**
 * The pinned stage. Everything is a pure function of one scroll progress `p` in 0..1, so scrolling back
 * retraces the same path: the camera starts in the glass, pulls out to the full machine, the landscape
 * rises behind it and the call to action slides in beside it.
 * Below `lg` there is no pin: the machine is stacked over the copy and only the session types on scroll.
 */
export function Stage({ children }: { children: ReactNode }) {
  const reduced = useReducedMotion();
  const track = useRef<HTMLDivElement>(null);
  const stage = useRef<HTMLDivElement>(null);
  const slot = useRef<HTMLDivElement>(null);
  const cam = useRef<HTMLDivElement>(null);
  const screen = useRef<HTMLDivElement>(null);
  const band = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const [type, setType] = useState(1);

  const session = useMemo(() => visible(type), [type]);

  useEffect(() => {
    const trackEl = track.current!;
    const stageEl = stage.current!;
    const camEl = cam.current!;
    const cv = canvas.current!;
    const ctx = cv.getContext('2d')!;
    const cache: { c: [number, number, number][] | null; k: string } = { c: null, k: '' };
    const lg = matchMedia('(min-width: 1024px)');
    let pal = readPalette();
    let img: ImageData | null = null;
    let geo = { k0: 1, dx: 0, dy: 0, ox: 0, oy: 0, bw: 0, bh: 0 };
    let raf = 0;
    let last = -1;
    let lastType = -1;
    let lastStatus = '';
    let p = 1;
    // the track's page-space box and the sticky offset, measured on layout, never per scroll frame
    let trackTop = 0;
    let trackH = 0;
    let range = 0;
    let stick = 0;
    let slotTop = 0;

    const measure = () => {
      trackTop = trackEl.getBoundingClientRect().top + scrollY;
      trackH = trackEl.offsetHeight;
      range = trackH - stageEl.offsetHeight;
      stick = parseFloat(getComputedStyle(stageEl).top) || 0;
      slotTop = slot.current!.getBoundingClientRect().top + scrollY;
    };

    const layout = () => {
      measure();
      camEl.style.transform = 'none';
      const s = screen.current!.getBoundingClientRect();
      const sl = slot.current!.getBoundingClientRect();
      const st = stageEl.getBoundingClientRect();
      const ox = s.left + s.width / 2 - sl.left;
      const oy = s.top + s.height / 2 - sl.top;
      const k0 = lg.matches ? clamp(Math.min((st.width * 0.84) / s.width, (st.height * 0.78) / s.height), 1, 4) : 1;
      geo = {
        k0,
        ox,
        oy,
        dx: st.left + st.width / 2 - (s.left + s.width / 2),
        dy: st.top + st.height / 2 - (s.top + s.height / 2),
        bw: 0,
        bh: 0,
      };
      camEl.style.transformOrigin = `${ox}px ${oy}px`;
      const b = band.current!.getBoundingClientRect();
      const w = Math.max(1, Math.ceil(b.width / CELL));
      const h = Math.max(1, Math.ceil(b.height / CELL));
      cv.width = w;
      cv.height = h;
      img = ctx.createImageData(w, h);
      last = -1;
      apply(true);
    };

    const progress = () => {
      if (reduced) return 1;
      const vh = innerHeight;
      if (lg.matches) {
        return range > 0 ? clamp((stick - (trackTop - scrollY)) / range) : 1;
      }
      const q = clamp((vh * 0.9 - (slotTop - scrollY)) / (vh * 0.55));
      return 0.1 + 0.9 * q;
    };

    const apply = (force = false) => {
      raf = 0;
      p = progress();
      if (!force && Math.abs(p - last) < 0.0005) return;
      last = p;
      const wide = lg.matches && !reduced;
      const on = smooth(p, 0, 0.1);
      const e = wide ? inOut(smooth(p, 0.3, 0.78)) : 1;
      const k = geo.k0 + (1 - geo.k0) * e;
      const tilt = 5 * e;
      camEl.style.transform = `translate(${geo.dx * (1 - e)}px, ${geo.dy * (1 - e)}px) scale(${k}) perspective(1800px) rotateY(${-tilt}deg)`;
      const st = stageEl.style;
      st.setProperty('--on', String(0.015 + 0.985 * smooth(on, 0.25, 1)));
      st.setProperty('--on-o', String(on > 0.02 ? 1 : 0));
      st.setProperty('--lamp', on > 0.3 ? '#7bd88f' : '#e0442b');
      const c = wide ? smooth(p, 0.72, 0.94) : 1;
      st.setProperty('--cta', String(c));
      st.setProperty('--bg', String(wide ? smooth(p, 0.4, 0.62) : 1));

      const tp = smooth(p, 0.1, 0.64);
      const q = Math.round(tp * 400) / 400;
      if (q !== lastType) {
        lastType = q;
        setType(q);
      }

      if (img) {
        paintScene(
          img,
          pal,
          {
            terrain: wide ? smooth(p, 0.44, 0.8) : 1,
            sun: wide ? smooth(p, 0.54, 0.88) : 1,
            drift: wide ? p : 0,
            horizon: lg.matches ? 0.76 : 0.55,
            sunX: lg.matches ? 0.88 : 0.5,
            sunR: lg.matches ? 0.072 : 0.22,
            sunY: lg.matches ? 0.07 : 0.28,
          },
          cache,
        );
        ctx.putImageData(img, 0, 0);
      }

      // status line follows the scroll state while this stage crosses mid-screen
      const top = trackTop - scrollY;
      if (top < innerHeight / 2 && top + trackH > innerHeight / 2) {
        const s = p < 0.1 ? 'powering on' : p < 0.72 ? 'session: verify' : 'ready';
        if (s !== lastStatus) {
          lastStatus = s;
          setStatus(s);
        }
      } else lastStatus = '';
    };
    const queue = () => {
      if (!raf) raf = requestAnimationFrame(() => apply());
    };

    layout();
    addEventListener('scroll', queue, { passive: true });
    const ro = new ResizeObserver(layout);
    ro.observe(stageEl);
    ro.observe(slot.current!);
    // content above can grow or shrink without the stage resizing: re-measure the track, cheaply
    const above = new ResizeObserver(() => {
      measure();
      queue();
    });
    above.observe(document.body);
    const stopTheme = onThemeChange(() => {
      pal = readPalette();
      cache.c = null;
      apply(true);
    });
    lg.addEventListener('change', layout);
    return () => {
      cancelAnimationFrame(raf);
      removeEventListener('scroll', queue);
      ro.disconnect();
      above.disconnect();
      stopTheme();
      lg.removeEventListener('change', layout);
    };
  }, [reduced]);

  return (
    <div ref={track} className="relative lg:h-[230svh] lg:motion-reduce:h-auto">
      <div
        ref={stage}
        style={{ '--sh': 'max(640px, calc(100svh - var(--lp-menu-h) - var(--lp-status-h)))' } as React.CSSProperties}
        className="relative grid grid-cols-1 gap-8 px-(--lp-gutter) pt-(--lp-section) pb-10 lg:sticky lg:top-(--lp-menu-h) lg:h-(--sh) lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:grid-rows-[minmax(0,1fr)] lg:items-center lg:gap-8 lg:pt-[56px] lg:pb-[130px] lg:motion-reduce:static"
      >
        {/* backdrop: sun, terrain (dithered canvas), fine waveforms and baseline marks */}
        <div ref={band} aria-hidden className="relative z-0 -mx-(--lp-gutter) order-2 h-44 overflow-hidden lg:absolute lg:inset-0 lg:mx-0 lg:h-auto">
          <div className="absolute inset-0" style={{ opacity: 'var(--bg, 1)' }}>
            <canvas ref={canvas} className="absolute inset-0 size-full [image-rendering:pixelated]" />
            <Waves />
            <Baseline />
          </div>
        </div>

        <div ref={slot} className="relative z-10 mx-auto w-full max-w-[560px] aspect-[1/0.92] sm:aspect-[1.1] lg:order-1 lg:max-w-none lg:w-[min(100%,calc((var(--sh)-140px)*1.1))] lg:justify-self-start">
          <Crt lines={session.lines} typing={session.typing} screenRef={screen} camRef={cam} />
        </div>

        <div
          style={{ opacity: 'var(--cta, 1)', transform: 'translateX(calc((1 - var(--cta, 1)) * 24px))' }}
          className="relative z-10 order-3 flex flex-col gap-6 lg:order-2 lg:focus-within:opacity-100!"
        >
          {children}
        </div>
      </div>
    </div>
  );
}

/** Fine waveform curves across the sky, drifting with the scroll. */
function Waves() {
  const path = (amp: number, freq: number, phase: number, y: number) => {
    let d = '';
    for (let x = 0; x <= 1440; x += 16) d += `${x ? 'L' : 'M'}${x} ${(y + Math.sin((x / 1440) * Math.PI * 2 * freq + phase) * amp).toFixed(1)} `;
    return d;
  };
  return (
    <svg aria-hidden viewBox="0 0 1440 800" preserveAspectRatio="none" className="absolute inset-0 size-full text-lp-line-strong max-lg:hidden">
      {[0, 1, 2, 3].map((i) => (
        <path key={i} d={path(14 + i * 5, 1.5 + i * 0.5, i * 1.3, 130 + i * 22)} fill="none" stroke="currentColor" strokeWidth="1" vectorEffect="non-scaling-stroke" opacity={0.75 - i * 0.12} />
      ))}
    </svg>
  );
}

/** Technical baseline: a rule with ticks and pixel-type annotations at the foot of the scene. */
function Baseline() {
  return (
    <div className="absolute inset-x-(--lp-gutter) bottom-3 text-lp-ink-2 lg:bottom-4">
      <div
        className="h-2 border-t border-lp-line-strong bg-[repeating-linear-gradient(to_right,var(--lp-line-strong)_0_1px,transparent_1px_16px)]"
      />
      <div className="flex justify-between font-pixel text-[11px] leading-4">
        <span className="bg-lp-paper pe-2">BASELINE 0.00</span>
        <span className="bg-lp-paper px-2 max-sm:hidden">HORIZON · ILLUSTRATIVE SCENE</span>
        <span className="bg-lp-paper ps-2">FIG. 11</span>
      </div>
    </div>
  );
}
