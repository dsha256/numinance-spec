import { mulberry32 } from '@/lib/landing/rng';

/** equirectangular texture of the core's machined surface */
export const TW = 512;
export const TH = 256;

export interface Surface {
  /** panel tone 0..1 */
  tone: Float32Array;
  /** engraved depth 0..1 (seams, traces, pits) */
  groove: Float32Array;
  /** 0 none, 1 indigo pad, 2 yellow pad */
  accent: Uint8Array;
}

/**
 * Seeded procedural plating: recursively split panels, hatch vents, longitude/latitude machining
 * lines, etched circuit traces ending in pads, and a few impact pits.
 */
export function buildSurface(seed = 20260): Surface {
  const rnd = mulberry32(seed);
  const tone = new Float32Array(TW * TH).fill(0.8);
  const groove = new Float32Array(TW * TH);
  const accent = new Uint8Array(TW * TH);
  const at = (x: number, y: number) => ((y + TH) % TH) * TW + ((x + TW) % TW);
  const cut = (x: number, y: number, d: number) => {
    const i = at(x, y);
    if (groove[i] < d) groove[i] = d;
  };

  // panels
  const leaves: [number, number, number, number][] = [];
  const split = (x: number, y: number, w: number, h: number, depth: number) => {
    if ((w <= 44 && h <= 30) || depth > 7 || (depth > 3 && rnd() < 0.18)) {
      leaves.push([x, y, w, h]);
      return;
    }
    if (w / 1.6 >= h) {
      const c = Math.round(w * (0.34 + rnd() * 0.32));
      split(x, y, c, h, depth + 1);
      split(x + c, y, w - c, h, depth + 1);
    } else {
      const c = Math.round(h * (0.34 + rnd() * 0.32));
      split(x, y, w, c, depth + 1);
      split(x, y + c, w, h - c, depth + 1);
    }
  };
  split(0, 0, TW, TH, 0);
  for (const [x, y, w, h] of leaves) {
    const t = 0.7 + rnd() * 0.3;
    const vent = rnd() < 0.09 && w > 14 && h > 14;
    for (let j = 0; j < h; j++)
      for (let i = 0; i < w; i++) {
        tone[at(x + i, y + j)] = t;
        if (vent && i > 2 && j > 2 && i < w - 3 && j < h - 3 && j % 3 === 0) cut(x + i, y + j, 0.7);
      }
    for (let i = 0; i < w; i++) {
      cut(x + i, y, 1);
      cut(x + i, y + 1, 0.8);
      cut(x + i, y + h - 1, 0.55);
    }
    for (let j = 0; j < h; j++) {
      cut(x, y + j, 1);
      cut(x + 1, y + j, 0.8);
      cut(x + w - 1, y + j, 0.55);
    }
  }

  // machining lines (every 22.5 degrees), dashed
  for (let k = 0; k < TW; k += 32) for (let y = 0; y < TH; y++) if ((y >> 2) % 2 === 0) cut(k, y, 0.45);
  for (let k = 16; k < TH; k += 32) for (let x = 0; x < TW; x++) if ((x >> 2) % 2 === 0) cut(x, k, 0.45);

  // etched circuit traces ending in pads
  const dirs = [
    [1, 0],
    [1, 1],
    [0, 1],
    [-1, 1],
    [-1, 0],
    [-1, -1],
    [0, -1],
    [1, -1],
  ] as const;
  for (let n = 0; n < 34; n++) {
    let x = Math.floor(rnd() * TW);
    let y = 24 + Math.floor(rnd() * (TH - 48));
    let d = Math.floor(rnd() * 4) * 2;
    const segs = 3 + Math.floor(rnd() * 4);
    for (let s = 0; s < segs; s++) {
      const len = 10 + Math.floor(rnd() * 34);
      for (let k = 0; k < len; k++) {
        cut(x, y, 0.85);
        cut(x + (dirs[d][1] !== 0 ? 1 : 0), y + (dirs[d][0] !== 0 ? 1 : 0), 0.85);
        x += dirs[d][0];
        y += dirs[d][1];
      }
      d = (d + (rnd() < 0.5 ? 1 : 7) * (rnd() < 0.7 ? 1 : 2)) % 8;
    }
    // pad
    const pr = 3;
    for (let j = -pr; j <= pr; j++)
      for (let i = -pr; i <= pr; i++) {
        const q = Math.hypot(i, j);
        if (q <= pr && q > pr - 1.2) cut(x + i, y + j, 1);
        else if (q <= pr - 1.2) groove[at(x + i, y + j)] = 0;
      }
    const roll = rnd();
    if (roll < 0.2) accent[at(x, y)] = accent[at(x + 1, y)] = accent[at(x, y + 1)] = accent[at(x + 1, y + 1)] = 1;
    else if (roll < 0.27) accent[at(x, y)] = accent[at(x + 1, y)] = accent[at(x, y + 1)] = accent[at(x + 1, y + 1)] = 2;
  }

  // impact pits
  for (let n = 0; n < 15; n++) {
    const cx = Math.floor(rnd() * TW);
    const cy = 30 + Math.floor(rnd() * (TH - 60));
    const r = 6 + Math.floor(rnd() * 11);
    for (let j = -r - 2; j <= r + 2; j++)
      for (let i = -r - 2; i <= r + 2; i++) {
        const q = Math.hypot(i, j);
        const idx = at(cx + i, cy + j);
        if (q < r - 1.5) {
          tone[idx] *= 0.5;
          groove[idx] = Math.max(groove[idx] * 0.4, i + j < 0 ? 0.45 : 0.2);
        } else if (q < r) {
          tone[idx] = 1;
          groove[idx] = 0;
        }
      }
  }
  return { tone, groove, accent };
}
