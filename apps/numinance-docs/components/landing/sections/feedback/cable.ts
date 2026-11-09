export type Point = [number, number];

const RADIUS = 11;
const CLIP_EVERY = 110;

/** Rounded orthogonal path through `pts` (quadratic corners). */
export function roundedPath(pts: Point[], radius = RADIUS): string {
  const p = pts.filter((q, i) => i === 0 || q[0] !== pts[i - 1][0] || q[1] !== pts[i - 1][1]);
  if (p.length < 2) return '';
  let d = `M${p[0][0]} ${p[0][1]}`;
  for (let i = 1; i < p.length - 1; i++) {
    const [a, b, c] = [p[i - 1], p[i], p[i + 1]];
    const l1 = Math.hypot(b[0] - a[0], b[1] - a[1]);
    const l2 = Math.hypot(c[0] - b[0], c[1] - b[1]);
    const r = Math.min(radius, l1 / 2, l2 / 2);
    const s: Point = [b[0] + ((a[0] - b[0]) / l1) * r, b[1] + ((a[1] - b[1]) / l1) * r];
    const e: Point = [b[0] + ((c[0] - b[0]) / l2) * r, b[1] + ((c[1] - b[1]) / l2) * r];
    d += `L${s[0]} ${s[1]}Q${b[0]} ${b[1]} ${e[0]} ${e[1]}`;
  }
  const last = p[p.length - 1];
  return `${d}L${last[0]} ${last[1]}`;
}

export interface Clip {
  x: number;
  y: number;
  vertical: boolean;
  /** share (0..1) of the cable's length where its head passes this clip */
  at: number;
}

/** Clip fittings spaced along straight runs (away from corners), with their draw-in share. */
export function clipsFor(pts: Point[]): Clip[] {
  const p = pts.filter((q, i) => i === 0 || q[0] !== pts[i - 1][0] || q[1] !== pts[i - 1][1]);
  const runs = p.slice(1).map((q, i) => Math.hypot(q[0] - p[i][0], q[1] - p[i][1]));
  const total = runs.reduce((a, b) => a + b, 0) || 1;
  const out: Clip[] = [];
  let before = 0;
  runs.forEach((l, i) => {
    const [a, b] = [p[i], p[i + 1]];
    const vertical = a[0] === b[0];
    const usable = l - 2 * (RADIUS + 10);
    if (usable > 6) {
      const n = Math.max(1, Math.round(usable / CLIP_EVERY));
      for (let k = 0; k < n; k++) {
        const d = RADIUS + 10 + (usable * (k + 0.5)) / n;
        const t = d / l;
        out.push({
          x: Math.round(a[0] + (b[0] - a[0]) * t),
          y: Math.round(a[1] + (b[1] - a[1]) * t),
          vertical,
          at: (before + d) / total,
        });
      }
    }
    before += l;
  });
  return out;
}
