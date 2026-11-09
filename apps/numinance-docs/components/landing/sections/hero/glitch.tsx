'use client';
import { useCallback, useRef } from 'react';
import { useReducedMotion } from '@/lib/landing/use-reduced';

const slice = 'invisible absolute inset-y-0 -inset-x-[0.12em] bg-lp-paper px-[0.12em]';

function frame(top: number, bottom: number, shift: number, inverted = false): Keyframe {
  return {
    visibility: 'visible',
    clipPath: `inset(${top}% 0 ${bottom}% 0)`,
    transform: `translateX(${shift}em)`,
    backgroundColor: inverted ? 'var(--lp-orange)' : 'var(--lp-paper)',
    color: inverted ? 'var(--lp-paper)' : 'var(--lp-orange)',
    easing: 'step-end',
  };
}

/** three-step slice offsets per layer: a restrained pixel shimmer, deterministic */
const KEYFRAMES = [
  [frame(14, 58, 0.07), frame(46, 38, -0.1, true), frame(70, 10, 0.08), { visibility: 'hidden' }],
  [frame(62, 20, -0.05), frame(20, 70, 0.04), frame(0, 100, 0), { visibility: 'hidden' }],
];

/** the accent word, glitched once on load and on hover/focus (never with reduced motion) */
export function Glitch({ children }: { children: string }) {
  const reduced = useReducedMotion();
  const root = useRef<HTMLSpanElement | null>(null);
  const busy = useRef(false);
  const play = useCallback(() => {
    const el = root.current;
    if (!el || busy.current) return;
    busy.current = true;
    el.querySelectorAll('span').forEach((l, i) => l.animate(KEYFRAMES[i], { duration: 150 }));
    setTimeout(() => (busy.current = false), 900);
  }, []);
  const ref = useCallback(
    (node: HTMLSpanElement | null) => {
      root.current = node;
      if (!node || reduced) return;
      const t = setTimeout(play, 1500);
      return () => clearTimeout(t);
    },
    [reduced, play],
  );
  return (
    <span ref={ref} onPointerEnter={reduced ? undefined : play} className="relative inline-block">
      {children}
      <span aria-hidden className={slice}>
        {children}
      </span>
      <span aria-hidden className={slice}>
        {children}
      </span>
    </span>
  );
}
