import { bayer } from '@/lib/landing/dither';
import { rgb, type Palette } from '@/lib/landing/palette';

/** Ridge height for layer `i` at column `x` (in cells), a few deterministic sines. */
function ridge(i: number, x: number, w: number, shift: number) {
  const k = 1 / (w + 1);
  const u = x * k * Math.PI * 2 + shift * (0.6 + i * 0.35);
  const a = 1 + i * 0.25;
  return (Math.sin(u * (1.2 + i * 0.3) + i * 2.1) * 0.6 + Math.sin(u * (3.1 + i) + i) * 0.28 + Math.sin(u * 7.3 + i * 4) * 0.08) * a;
}

export interface SceneState {
  /** 0..1: how far the terrain has risen into the frame */
  terrain: number;
  /** 0..1: how far the sun has risen */
  sun: number;
  /** raw scroll progress, for parallax drift */
  drift: number;
  /** fraction of the band height where the horizon sits */
  horizon: number;
  /** sun centre x as a fraction of the band width */
  sunX: number;
  /** sun radius as a fraction of the band height */
  sunR: number;
  /** sun resting height as a fraction of the band height */
  sunY: number;
}

const LAYERS = 5;

/**
 * Paints the landscape onto the low-resolution canvas `img` (one cell = one chunky pixel):
 * a circular sun with a dithered halo and layered Bayer-dithered ridges in front of it.
 */
export function paintScene(img: ImageData, pal: Palette, s: SceneState, cache: { c: [number, number, number][] | null; k: string }) {
  const { width: w, height: h, data } = img;
  const key = pal.paper + pal.ink + pal.sun + pal.ink2;
  if (!cache.c || cache.k !== key) {
    cache.c = [rgb(pal.paper), rgb(pal.ink), rgb(pal.sun), rgb(pal.ink2), rgb(pal.line)];
    cache.k = key;
  }
  const [paper, inkRaw, sun, ink2Raw] = cache.c;
  const mix = (c: [number, number, number], t: number): [number, number, number] => [0, 1, 2].map((i) => Math.round(paper[i] + (c[i] - paper[i]) * t)) as [number, number, number];
  const ink = mix(inkRaw, 0.75);
  const ink2 = mix(ink2Raw, 0.6);
  data.fill(0);

  const hz = h * s.horizon;
  const rise = (1 - s.terrain) * h * 0.5;
  const r = h * s.sunR;
  const cx = w * s.sunX;
  const rest = h * s.sunY;
  const cy = rest + (1 - s.sun) * (hz - rest + r * 1.3);
  const shift = s.drift * 5;

  // ridge rows per layer: far (0) .. near (LAYERS-1); typed arrays are reused between calls (paint runs per scroll frame)
  if (rowW !== w) {
    rowBuf = Array.from({ length: LAYERS }, () => new Float32Array(w));
    rowW = w;
  }
  const rows = rowBuf;
  const tops = topBuf.length >= w ? topBuf : (topBuf = new Float32Array(w));
  for (let i = 0; i < LAYERS; i++) {
    const row = rows[i];
    const base = hz + (i / (LAYERS - 1)) * h * (1 - s.horizon) * 0.5 - h * 0.012 * (LAYERS - i);
    const amp = h * (0.05 - i * 0.004);
    for (let x = 0; x < w; x++) row[x] = base + ridge(i, x, w, shift) * amp + rise * (0.6 + i * 0.25);
  }
  // highest ridge in each column: above it is sky, where only the sun can draw
  for (let x = 0; x < w; x++) {
    let m = rows[0][x];
    for (let i = 1; i < LAYERS; i++) if (rows[i][x] < m) m = rows[i][x];
    tops[x] = m;
  }
  const dens = [0.05, 0.09, 0.14, 0.2, 0.27];
  const edges = [ink2, ink2, ink2, ink, ink];
  const put = (o: number, c: [number, number, number]) => {
    data[o] = c[0];
    data[o + 1] = c[1];
    data[o + 2] = c[2];
    data[o + 3] = 255;
  };
  const halo = r * 1.5;
  const sunX0 = Math.max(0, Math.floor(cx - halo));
  const sunX1 = Math.min(w - 1, Math.ceil(cx + halo));
  const sunY0 = Math.max(0, Math.floor(cy - halo));
  const sunY1 = Math.min(h - 1, Math.ceil(cy + halo));
  const hasSun = s.sun > 0;
  const fade = 1 / (h * 0.14);

  for (let x = 0; x < w; x++) {
    const top = tops[x];
    const r0 = rows[0][x];
    const r1 = rows[1][x];
    const r2 = rows[2][x];
    const r3 = rows[3][x];
    const r4 = rows[4][x];
    const inSun = hasSun && x >= sunX0 && x <= sunX1;
    // sky: the sun only
    if (inSun) {
      const yEnd = Math.min(sunY1, Math.ceil(top) - 1);
      for (let y = sunY0; y <= yEnd; y++) {
        const dx = x - cx;
        const dy = y - cy;
        const dist = Math.sqrt(dx * dx + dy * dy);
        const o = (y * w + x) * 4;
        if (dist <= r) put(o, sun);
        else if (dist <= halo) {
          const d = (1 - (dist - r) / (r * 0.5)) * 0.42;
          if (bayer(x, y) < d) put(o, sun);
        }
      }
    }
    // ground: the nearest ridge a pixel has passed
    for (let y = Math.max(0, Math.ceil(top)); y < h; y++) {
      let layer = -1;
      let ry = 0;
      if (y >= r4) (layer = 4), (ry = r4);
      else if (y >= r3) (layer = 3), (ry = r3);
      else if (y >= r2) (layer = 2), (ry = r2);
      else if (y >= r1) (layer = 1), (ry = r1);
      else if (y >= r0) (layer = 0), (ry = r0);
      if (layer >= 0) {
        const o = (y * w + x) * 4;
        const dy = y - ry;
        if (dy < 1) put(o, edges[layer]);
        else {
          const depth = Math.min(1, dy * fade);
          if (bayer(x, y) < dens[layer] * (1 - depth * 0.8)) put(o, ink2);
          else put(o, paper);
        }
      }
    }
  }
}

let rowBuf: Float32Array[] = [];
let rowW = 0;
let topBuf = new Float32Array(0);
