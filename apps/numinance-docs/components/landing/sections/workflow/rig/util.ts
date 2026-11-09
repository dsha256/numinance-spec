/** Small motion helpers the rig and its planner share (adapted from Fumadocs' lib/motion.ts, MIT). */

export function clamp(value: number, min = 0, max = 1): number {
  return Math.min(max, Math.max(min, value));
}
export function mix(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

/** a CSS-style cubic bezier easing, solved by Newton's method with a bisection fallback */
function cubicBezier(x1: number, y1: number, x2: number, y2: number): (t: number) => number {
  const cx = 3 * x1;
  const bx = 3 * (x2 - x1) - cx;
  const ax = 1 - cx - bx;
  const cy = 3 * y1;
  const by = 3 * (y2 - y1) - cy;
  const ay = 1 - cy - by;
  const X = (t: number) => ((ax * t + bx) * t + cx) * t;
  const dX = (t: number) => (3 * ax * t + 2 * bx) * t + cx;
  const Y = (t: number) => ((ay * t + by) * t + cy) * t;
  return (x) => {
    if (x <= 0) return 0;
    if (x >= 1) return 1;
    let t = x;
    for (let i = 0; i < 6; i++) {
      const e = X(t) - x;
      if (Math.abs(e) < 1e-6) return Y(t);
      const d = dX(t);
      if (Math.abs(d) < 1e-6) break;
      t -= e / d;
    }
    let lo = 0;
    let hi = 1;
    t = x;
    for (let i = 0; i < 24; i++) {
      const e = X(t) - x;
      if (Math.abs(e) < 1e-6) break;
      if (e > 0) hi = t;
      else lo = t;
      t = (lo + hi) / 2;
    }
    return Y(t);
  };
}

/** the one easing for things that settle */
export const easeOut = cubicBezier(0.16, 1, 0.3, 1);
/** symmetric, for travel */
export const easeInOut = cubicBezier(0.65, 0, 0.35, 1);

/** 120 bpm sixteenths: the beat the operator's frames step on */
export const TICK = 125;

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** `el`'s box from `origin`'s top left */
export function rectIn(el: Element, origin: DOMRect): Rect {
  const r = el.getBoundingClientRect();
  return { x: r.left - origin.left, y: r.top - origin.top, w: r.width, h: r.height };
}

/**
 * Lets a script steer `el` by its `transform` and `opacity` each frame as cheaply as an animation, till the
 * returned release (a paused additive animation keeps the element on its own compositor layer).
 */
export function steer(el: Element): () => void {
  const animation = el.animate({ transform: ['none', 'none'], opacity: [0, 0] }, { duration: 1, composite: 'add' });
  animation.pause();
  return () => animation.cancel();
}
