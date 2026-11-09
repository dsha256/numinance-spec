/**
 * Numinance landing motion language. Two families on purpose:
 *  - MECH: machines (arms, rails, carriages) — weighty ease-in-out, no overshoot.
 *  - UI:   panels, text, selection — fast ease-out.
 * Numeric helpers are for rAF/canvas code; CSS strings are for WAAPI / Motion.
 */
export const EASE_MECH_CSS = 'cubic-bezier(0.45, 0, 0.2, 1)';
export const EASE_UI_CSS = 'cubic-bezier(0.22, 1, 0.36, 1)';
export const EASE_MECH = [0.45, 0, 0.2, 1] as const;
export const EASE_UI = [0.22, 1, 0.36, 1] as const;

/** Durations in seconds (Motion) — `ms` variants below for timers. */
export const DUR = { fast: 0.15, base: 0.28, panel: 0.45, scene: 0.8 } as const;
export const MS = { fast: 150, base: 280, panel: 450, scene: 800, char: 32 } as const;

export const clamp01 = (x: number) => (x < 0 ? 0 : x > 1 ? 1 : x);
export const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
export const smoothstep = (t: number) => {
  const x = clamp01(t);
  return x * x * (3 - 2 * x);
};
/** Ease-in-out (mechanical): slow start, slow stop. */
export const easeMech = (t: number) => {
  const x = clamp01(t);
  return x < 0.5 ? 4 * x * x * x : 1 - Math.pow(-2 * x + 2, 3) / 2;
};
/** Ease-out (UI). */
export const easeUi = (t: number) => 1 - Math.pow(1 - clamp01(t), 3);
/** Frame-rate independent exponential smoothing toward a target. */
export const damp = (current: number, target: number, lambda: number, dtMs: number) =>
  lerp(current, target, 1 - Math.exp(-lambda * (dtMs / 1000)));
/** Map global t into a sub-range [a,b] → 0..1. */
export const seg = (t: number, a: number, b: number) => clamp01((t - a) / (b - a));
