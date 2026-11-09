'use client';

import { useEffect, useRef } from 'react';
import { useReducedMotion } from '@/lib/landing/use-reduced';

/**
 * Runs `frame(dt, t)` on rAF only while `ref`'s element is on screen and the tab is visible.
 * `frame` is called once immediately (static render) when motion is reduced.
 * Returns nothing; draw state lives in your closure. `setup` runs once on mount and may return cleanup.
 */
export function useScene<T extends HTMLElement>(
  setup: (el: T, ctx: { reduced: boolean; invalidate: () => void }) => {
    frame: (dt: number, t: number) => void;
    cleanup?: () => void;
  } | void,
) {
  const ref = useRef<T>(null);
  const reduced = useReducedMotion() === true;
  const setupRef = useRef(setup);
  setupRef.current = setup;

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    let raf = 0;
    let visible = false;
    let last = 0;
    let t = 0;
    let loop: ((dt: number, t: number) => void) | undefined;
    const tick = (now: number) => {
      const dt = Math.min(64, now - (last || now));
      last = now;
      t += dt;
      loop?.(dt, t);
      raf = visible && !reduced ? requestAnimationFrame(tick) : 0;
    };
    const run = () => {
      if (!raf && visible && !reduced) {
        last = 0;
        raf = requestAnimationFrame(tick);
      }
    };
    const invalidate = () => loop?.(0, t);
    const made = setupRef.current(el, { reduced, invalidate });
    if (made) {
      loop = made.frame;
      loop(0, 0);
    }
    const io = new IntersectionObserver(([e]) => {
      visible = e.isIntersecting;
      if (visible) run();
    });
    io.observe(el);
    const vis = () => (document.hidden ? (cancelAnimationFrame(raf), (raf = 0)) : run());
    document.addEventListener('visibilitychange', vis);
    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      document.removeEventListener('visibilitychange', vis);
      made?.cleanup?.();
    };
  }, [reduced]);

  return ref;
}

/** Fit a canvas to its CSS box at a capped device pixel ratio; returns the context and CSS size. */
export function fitCanvas(canvas: HTMLCanvasElement, maxDpr = 2) {
  const dpr = Math.min(window.devicePixelRatio || 1, maxDpr);
  const w = canvas.clientWidth;
  const h = canvas.clientHeight;
  canvas.width = Math.max(1, Math.round(w * dpr));
  canvas.height = Math.max(1, Math.round(h * dpr));
  const ctx = canvas.getContext('2d')!;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.imageSmoothingEnabled = false;
  return { ctx, w, h, dpr };
}
