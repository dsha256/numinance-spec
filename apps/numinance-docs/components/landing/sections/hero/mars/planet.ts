import { BAYER4 } from '@/lib/landing/dither';
import { TH, TW, type Surface } from '../texture';
import { TAU, clamp, smooth } from './dynamics';
import type { Terrain } from './terrain';

/**
 * The halftone sphere, shared by the original look and the Mars look. ONE set of per-pixel
 * tables (normal, tangent frame, texture row / column) feeds both materials, so the morph
 * interpolates the SAME surface samples: the lit value `v` is lerped, then re-dithered with the
 * same ordered Bayer cells. No second canvas, no re-rasterisation, no seam.
 */
export const TILT_A = 0.38; // tilt of the polar axis in the screen plane (same constant as the original)
export const BASE_LIGHT: readonly [number, number, number] = (() => {
  const l = [-0.52, -0.6, 0.6];
  const n = Math.hypot(l[0], l[1], l[2]);
  return [l[0] / n, l[1] / n, l[2] / n];
})();

type RGB = readonly [number, number, number];
export interface Colors {
  dark: boolean;
  base: RGB;
  dot: RGB;
  indigo: RGB;
  yellow: RGB;
  paper: RGB;
  rust: RGB;
  ochre: RGB;
  baseWarm: RGB;
  canyon: RGB;
  polar: RGB;
}

/** Mars accents: a MINORITY colour over the black / cream halftone */
export const marsColors = (dark: boolean) =>
  dark
    ? {
        rust: [204, 86, 42] as RGB,
        ochre: [218, 156, 82] as RGB,
        baseWarm: [48, 24, 17] as RGB,
        canyon: [22, 10, 7] as RGB,
        polar: [246, 236, 220] as RGB,
      }
    : {
        rust: [160, 62, 28] as RGB,
        ochre: [198, 136, 68] as RGB,
        baseWarm: [229, 201, 166] as RGB,
        canyon: [92, 40, 22] as RGB,
        polar: [250, 247, 240] as RGB,
      };

export interface SphereParams {
  /** spin in turns */
  rot: number;
  /** boot print 0..1 (the original dither-in) */
  print: number;
  /** Mars morph 0..1 */
  mars: number;
  /** validated-region strength 0..1 and its centre (view-space unit vector) */
  region: number;
  hub: readonly [number, number, number];
  /** scan meridian: strength 0..1 and angle */
  scan: number;
  scanAngle: number;
  light: readonly [number, number, number];
  lightMoved: boolean;
}

const MAX_TINT = 0.9;
const K_RELIEF = 1.7;

export function createPlanet() {
  let N = 0;
  let count = 0;
  let pIdx = new Int32Array(0);
  let pX = new Uint16Array(0);
  let pY = new Uint16Array(0);
  let pLam = new Float32Array(0);
  let pRow = new Uint16Array(0);
  let pCol = new Float32Array(0);
  let pRim = new Float32Array(0);
  let pN = new Float32Array(0); // nx ny nz
  let pE = new Float32Array(0); // east tangent, view frame
  let pT = new Float32Array(0); // north tangent, view frame
  let img: ImageData | null = null;

  /** same sampling as the original `buildTables`, plus the normal and tangent frame */
  function build(Nn: number, R: number, cell: number) {
    N = Nn;
    const Rb = R / cell;
    const half = N / 2;
    const [lx, ly, lz] = BASE_LIGHT;
    const ca = Math.cos(TILT_A),
      sa = Math.sin(TILT_A);
    const total = N * N;
    pX = new Uint16Array(total);
    pY = new Uint16Array(total);
    pLam = new Float32Array(total);
    pRow = new Uint16Array(total);
    pCol = new Float32Array(total);
    pRim = new Float32Array(total);
    pN = new Float32Array(total * 3);
    pE = new Float32Array(total * 3);
    pT = new Float32Array(total * 3);
    const idx: number[] = [];
    let k = 0;
    for (let by = 0; by < N; by++)
      for (let bx = 0; bx < N; bx++) {
        const nx = (bx + 0.5 - half) / Rb;
        const ny = (by + 0.5 - half) / Rb;
        const r2 = nx * nx + ny * ny;
        if (r2 >= 1) continue;
        const nz = Math.sqrt(1 - r2);
        const lam = Math.max(0, nx * lx + ny * ly + nz * lz);
        const mx = nx * ca + ny * sa;
        const my = -nx * sa + ny * ca;
        const lat = Math.asin(clamp(-my, -1, 1));
        const lon = Math.atan2(mx, nz);
        idx.push(by * N + bx);
        pX[k] = bx;
        pY[k] = by;
        pLam[k] = lam;
        pRow[k] = Math.min(TH - 1, Math.floor((0.5 - lat / Math.PI) * TH));
        pCol[k] = (lon / TAU + 0.5) * TW;
        const edge = clamp((0.2 - nz) / 0.2);
        const lit = clamp(0.35 + 0.65 * (-(nx / Math.sqrt(r2 + 1e-6)) * lx - (ny / Math.sqrt(r2 + 1e-6)) * ly) / Math.hypot(lx, ly));
        pRim[k] = edge * edge * lit;
        pN[k * 3] = nx;
        pN[k * 3 + 1] = ny;
        pN[k * 3 + 2] = nz;
        // tangent frame of the body at this point, expressed in the view frame
        const sl = Math.sin(lon),
          cl = Math.cos(lon),
          sp = -my,
          cp = Math.sqrt(Math.max(0, 1 - my * my));
        pE[k * 3] = cl * ca;
        pE[k * 3 + 1] = cl * sa;
        pE[k * 3 + 2] = -sl;
        const tx = -sp * sl,
          ty = -cp,
          tz = -sp * cl;
        pT[k * 3] = tx * ca - ty * sa;
        pT[k * 3 + 1] = tx * sa + ty * ca;
        pT[k * 3 + 2] = tz;
        k++;
      }
    count = k;
    pIdx = Int32Array.from(idx);
    img = new ImageData(N, N);
  }

  /** writes into `img`; returns it for putImageData */
  function render(surface: Surface, terr: Terrain | null, c: Colors, p: SphereParams): ImageData | null {
    if (!img) return null;
    const d = img.data;
    d.fill(0);
    const dark = c.dark;
    const rotCols = p.rot * TW;
    const { tone, groove, accent } = surface;
    const full = p.print >= 1;
    const [lx, ly, lz] = p.light;
    const useBase = !p.lightMoved;
    const marsOn = p.mars > 0.001 && terr !== null;
    const regOn = p.region > 0.001 && terr !== null;
    const scanOn = p.scan > 0.001;
    const hx = p.hub[0],
      hy = p.hub[1],
      hz = p.hub[2];
    const sCos = Math.cos(p.scanAngle),
      sSin = Math.sin(p.scanAngle);
    for (let k = 0; k < count; k++) {
      const bx = pX[k],
        by = pY[k];
      const th = (BAYER4[by & 3][bx & 3] + 0.5) / 16;
      if (!full) {
        const reveal = clamp(p.print * 1.8 - (by / N) * 0.8);
        if (reveal <= th) continue;
      }
      let col = Math.floor(pCol[k] - rotCols) % TW;
      if (col < 0) col += TW;
      const ti = pRow[k] * TW + col;
      const k3 = k * 3;
      const lam = useBase ? pLam[k] : Math.max(0, pN[k3] * lx + pN[k3 + 1] * ly + pN[k3 + 2] * lz);
      const g = groove[ti];
      let v = 0.07 + 0.85 * lam ** 1.15 * tone[ti] * (1 - 0.85 * g) + pRim[k] * 0.9;

      // how much of this cell is Mars: a left-to-right front across the disk, plus the validated region
      let rm = 0;
      if (marsOn) rm = clamp(p.mars * 1.7 - (bx / N) * 0.7);
      let regW = 0;
      if (regOn) {
        const dd = pN[k3] * hx + pN[k3 + 1] * hy + pN[k3 + 2] * hz;
        regW = smooth((dd - 0.74) / 0.2) * p.region;
      }
      const relief = Math.max(rm, regW * 0.7);
      let cell = 0; // 0 plain, 1 contour, 2 canyon
      if (relief > 0.001 && terr) {
        const gE = terr.gE[ti],
          gN = terr.gN[ti];
        const sl =
          K_RELIEF *
          (gE * (lx * pE[k3] + ly * pE[k3 + 1] + lz * pE[k3 + 2]) + gN * (lx * pT[k3] + ly * pT[k3 + 1] + lz * pT[k3 + 2]));
        const lamR = Math.max(0, lam - sl);
        const pit = terr.pit[ti] / 255,
          cany = terr.canyon[ti] / 255,
          pol = terr.polar[ti] / 255;
        let vm = 0.07 + 0.85 * lamR ** 1.15 * (1 - 0.3 * pit) * (1 - 0.55 * cany) * (0.72 + 0.28 * tone[ti]) + pRim[k] * 0.9;
        vm += 0.28 * pol * (lam > 0.02 ? 1 : 0);
        v += (vm - v) * relief;
        if (terr.contour[ti] && ((bx + by) & 3) < 3) cell = 1;
        if (cany > 0.35) cell = 2;
      }
      if (regW > 0) v += 0.1 * regW;
      v = clamp(v);

      const dens = dark ? v : (1 - v * 0.96) * 0.9;
      const th2 = (BAYER4[(by + 2) & 3][(bx + 1) & 3] + 0.5) / 16;
      let col3: RGB = c.base;
      const dotOn = dens > th2;
      if (accent[ti] && v > 0.14 && pRim[k] < 0.1 && rm < 0.5) col3 = accent[ti] === 1 ? c.indigo : c.yellow;
      else if (dotOn) col3 = c.dot;

      if (relief > 0.001 && terr) {
        const thT = (BAYER4[(by + 3) & 3][(bx + 2) & 3] + 0.5) / 16;
        const thW = (BAYER4[(by + 1) & 3][(bx + 3) & 3] + 0.5) / 16;
        const tint = (terr.tint[ti] / 255) * MAX_TINT * relief * (regW > rm ? 0.0 : 1);
        const warmRust = terr.warm[ti] / 255 > thW;
        if (cell === 2 && relief > thT) col3 = c.canyon;
        else if (cell === 1 && relief > 0.4) col3 = warmRust ? c.rust : c.ochre;
        else if (tint > thT) col3 = dotOn ? (warmRust ? c.rust : c.ochre) : c.baseWarm;
        else if (terr.polar[ti] > 120 && dotOn && relief > thT) col3 = c.polar;
      }
      if (scanOn) {
        // a bright meridian sweeping across the disk
        const dd = Math.abs(pN[k3] * sCos + pN[k3 + 2] * sSin);
        if (dd < 0.045 * p.scan && dens > 0.12 && dd < 0.02 + 0.025 * th) col3 = c.yellow;
      }
      const o = pIdx[k] * 4;
      d[o] = col3[0];
      d[o + 1] = col3[1];
      d[o + 2] = col3[2];
      d[o + 3] = 255;
    }
    return img;
  }

  return { build, render, get size() { return N; } };
}

export type Planet = ReturnType<typeof createPlanet>;
