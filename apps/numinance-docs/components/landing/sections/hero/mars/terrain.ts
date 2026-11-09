import { mulberry32 } from '@/lib/landing/rng';
import { TH, TW } from '../texture';
import { TAU, clamp, smooth } from './dynamics';

/**
 * Seeded Mars-like relief, computed ONCE in sphere space (never per frame, never per rotation):
 * a texel (x, y) is the unit vector of its latitude / longitude, so a crater is a true spherical
 * cap (angular distance d = acos(dot(n, c))) at every latitude, and the 3-D value noise has no seam.
 * Same equirectangular grid as the halftone surface, so one (row, col) lookup serves both.
 */
export interface Terrain {
  /** east / north slope of the height field (rad^-1, ~±1) */
  gE: Float32Array;
  gN: Float32Array;
  /** 0..255: how much of the cell may take the warm accent (low basins), 0 on the highlands and caps */
  tint: Uint8Array;
  /** 0..255: 0 ochre ... 255 rust */
  warm: Uint8Array;
  /** 0/1: topographic contour line */
  contour: Uint8Array;
  /** 0..255: canyon floor / etching */
  canyon: Uint8Array;
  /** 0..255: crater bowl depth (darkens the floor) */
  pit: Uint8Array;
  /** 0..255: polar light */
  polar: Uint8Array;
}

const hash3 = (ix: number, iy: number, iz: number, seed: number) => {
  let h = Math.imul(ix, 374761393) ^ Math.imul(iy, 668265263) ^ Math.imul(iz, 2147483647) ^ Math.imul(seed, 1274126177);
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  h ^= h >>> 16;
  return (h >>> 0) / 4294967296;
};

/** trilinear value noise with a smooth fade, 0..1 */
function noise3(x: number, y: number, z: number, seed: number) {
  const ix = Math.floor(x),
    iy = Math.floor(y),
    iz = Math.floor(z);
  const fx = x - ix,
    fy = y - iy,
    fz = z - iz;
  const u = fx * fx * (3 - 2 * fx),
    v = fy * fy * (3 - 2 * fy),
    w = fz * fz * (3 - 2 * fz);
  const a = (dz: number) => {
    const c00 = hash3(ix, iy, iz + dz, seed) + (hash3(ix + 1, iy, iz + dz, seed) - hash3(ix, iy, iz + dz, seed)) * u;
    const c10 = hash3(ix, iy + 1, iz + dz, seed) + (hash3(ix + 1, iy + 1, iz + dz, seed) - hash3(ix, iy + 1, iz + dz, seed)) * u;
    return c00 + (c10 - c00) * v;
  };
  const l = a(0);
  return l + (a(1) - l) * w;
}

function fbm(x: number, y: number, z: number, seed: number, oct: number) {
  let s = 0,
    amp = 1,
    norm = 0,
    f = 1;
  for (let i = 0; i < oct; i++) {
    s += amp * noise3(x * f, y * f, z * f, seed + i * 17);
    norm += amp;
    amp *= 0.5;
    f *= 2.07;
  }
  return s / norm;
}

interface Crater {
  c: [number, number, number];
  rho: number;
  depth: number;
}

/** incremental builder: `step(msBudget)` returns true when done (so init never blocks a frame) */
export function createTerrainBuilder(seed = 4817) {
  const rnd = mulberry32(seed);
  const n = TW * TH;
  const height = new Float32Array(n);
  const tintF = new Float32Array(n);
  const canyonF = new Float32Array(n);
  const pitF = new Float32Array(n);
  const out: Terrain = {
    gE: new Float32Array(n),
    gN: new Float32Array(n),
    tint: new Uint8Array(n),
    warm: new Uint8Array(n),
    contour: new Uint8Array(n),
    canyon: new Uint8Array(n),
    pit: new Uint8Array(n),
    polar: new Uint8Array(n),
  };

  // sparse, bounded crater field: many small, a few large basins
  const craters: Crater[] = [];
  for (let i = 0; i < 44; i++) {
    const z = rnd() * 2 - 1;
    const lon = rnd() * TAU;
    const r = Math.sqrt(1 - z * z);
    const rho = 0.035 + 0.19 * Math.pow(rnd(), 2.6);
    craters.push({ c: [r * Math.sin(lon), z, r * Math.cos(lon)], rho, depth: 0.32 * rho });
  }
  // two canyon systems along great-circle arcs
  const canyons = [0, 1].map((k) => {
    const lon = rnd() * TAU;
    const lat = (k === 0 ? -0.1 : 0.32) + (rnd() - 0.5) * 0.2;
    const c0: [number, number, number] = [Math.cos(lat) * Math.sin(lon), Math.sin(lat), Math.cos(lat) * Math.cos(lon)];
    const t0: [number, number, number] = [Math.cos(lon), 0, -Math.sin(lon)]; // east
    const ang = (k === 0 ? 0.12 : -0.55) + rnd() * 0.2;
    // direction along the canyon at c0, rotated from east by `ang`
    const nrm: [number, number, number] = [-Math.sin(lat) * Math.sin(lon), Math.cos(lat), -Math.sin(lat) * Math.cos(lon)];
    const dir: [number, number, number] = [
      t0[0] * Math.cos(ang) + nrm[0] * Math.sin(ang),
      t0[1] * Math.cos(ang) + nrm[1] * Math.sin(ang),
      t0[2] * Math.cos(ang) + nrm[2] * Math.sin(ang),
    ];
    const axis: [number, number, number] = [
      c0[1] * dir[2] - c0[2] * dir[1],
      c0[2] * dir[0] - c0[0] * dir[2],
      c0[0] * dir[1] - c0[1] * dir[0],
    ];
    return { c0, dir, axis, half: k === 0 ? 0.95 : 0.6, w: k === 0 ? 0.05 : 0.034, ph: rnd() * TAU };
  });

  let row = 0;
  let stage = 0; // 0 field, 1 craters, 2 derive
  let crater = 0;
  let min = 1e9,
    max = -1e9;

  const fieldRow = (y: number) => {
    const lat = (0.5 - (y + 0.5) / TH) * Math.PI;
    const cl = Math.cos(lat),
      sl = Math.sin(lat);
    for (let x = 0; x < TW; x++) {
      const lon = ((x + 0.5) / TW - 0.5) * TAU;
      const nx = cl * Math.sin(lon),
        ny = sl,
        nz = cl * Math.cos(lon);
      // low-frequency basins and highlands
      const base = fbm(nx * 1.25 + 3.1, ny * 1.25 + 1.7, nz * 1.25 - 2.3, seed, 5);
      let h = (base - 0.5) * 0.2;
      // canyons: signed great-circle distance with a meandering centreline
      let cany = 0;
      for (const cn of canyons) {
        const d = nx * cn.axis[0] + ny * cn.axis[1] + nz * cn.axis[2];
        const s = Math.atan2(nx * cn.dir[0] + ny * cn.dir[1] + nz * cn.dir[2], nx * cn.c0[0] + ny * cn.c0[1] + nz * cn.c0[2]);
        const along = Math.abs(s) / cn.half;
        if (along > 1) continue;
        const taper = 1 - along * along;
        const meander = 0.022 * Math.sin(s * 9 + cn.ph) + 0.012 * (noise3(nx * 14, ny * 14, nz * 14, seed + 9) - 0.5);
        const w = cn.w * (0.55 + 0.45 * taper);
        const q = Math.abs(Math.asin(clamp(d, -1, 1)) - meander) / w;
        if (q < 1) {
          const prof = 1 - q * q;
          h -= 0.085 * prof * taper;
          cany = Math.max(cany, prof * taper);
        } else if (q < 1.7) {
          h += 0.018 * (1 - (q - 1) / 0.7) * taper; // lip
        }
      }
      const i = y * TW + x;
      height[i] = h;
      canyonF[i] = cany;
      // warm basin mask from an independent low-frequency field, biased by elevation
      tintF[i] = fbm(nx * 0.9 - 5.2, ny * 0.9 + 4.4, nz * 0.9 + 0.6, seed + 101, 3) - h * 1.2;
      const pc = smooth((Math.abs(lat) - 1.18) / 0.22);
      out.polar[i] = Math.round(pc * 255);
    }
  };

  const applyCrater = (cr: Crater) => {
    const reach = cr.rho * 2.2;
    const lat0 = Math.asin(clamp(cr.c[1], -1, 1));
    const y0 = Math.max(0, Math.floor((0.5 - (lat0 + reach) / Math.PI) * TH));
    const y1 = Math.min(TH - 1, Math.ceil((0.5 - (lat0 - reach) / Math.PI) * TH));
    for (let y = y0; y <= y1; y++) {
      const lat = (0.5 - (y + 0.5) / TH) * Math.PI;
      const cl = Math.cos(lat),
        sl = Math.sin(lat);
      for (let x = 0; x < TW; x++) {
        const lon = ((x + 0.5) / TW - 0.5) * TAU;
        const d = Math.acos(clamp(cl * Math.sin(lon) * cr.c[0] + sl * cr.c[1] + cl * Math.cos(lon) * cr.c[2], -1, 1));
        if (d > reach) continue;
        const u = d / cr.rho;
        const rim = Math.exp(-(((u - 1) / 0.17) ** 2));
        let h = 0.38 * cr.depth * rim;
        if (u < 1) {
          const bowl = 1 - u * u;
          h -= cr.depth * bowl;
          pitF[y * TW + x] = Math.max(pitF[y * TW + x], bowl * (cr.rho > 0.07 ? 1 : 0.6));
        } else if (u < 2.2) h += 0.1 * cr.depth * (1 - (u - 1) / 1.2) * Math.exp(-(u - 1) * 2);
        height[y * TW + x] += h;
      }
    }
  };

  const derive = () => {
    for (let i = 0; i < n; i++) {
      const h = height[i];
      if (h < min) min = h;
      if (h > max) max = h;
    }
    const span = max - min || 1;
    const levels = 9;
    const dx = TAU / TW;
    const dy = Math.PI / TH;
    for (let y = 0; y < TH; y++) {
      const lat = (0.5 - (y + 0.5) / TH) * Math.PI;
      const cl = Math.max(0.12, Math.cos(lat));
      for (let x = 0; x < TW; x++) {
        const i = y * TW + x;
        const xl = y * TW + ((x + TW - 1) % TW);
        const xr = y * TW + ((x + 1) % TW);
        const yu = Math.max(0, y - 1) * TW + x;
        const yd = Math.min(TH - 1, y + 1) * TW + x;
        out.gE[i] = (height[xr] - height[xl]) / (2 * dx * cl);
        out.gN[i] = (height[yu] - height[yd]) / (2 * dy);
        const q = ((height[i] - min) / span) * levels;
        const qr = ((height[xr] - min) / span) * levels;
        const qd = ((height[yd] - min) / span) * levels;
        out.contour[i] = Math.floor(q) !== Math.floor(qr) || Math.floor(q) !== Math.floor(qd) ? 1 : 0;
        out.canyon[i] = Math.round(clamp(canyonF[i]) * 255);
        out.pit[i] = Math.round(clamp(pitF[i]) * 255);
        // basins take the accent, ~35% of the map; polar caps and canyon floors stay unlit by it
        const t = smooth((tintF[i] - 0.5) / 0.14 + 0.35);
        out.tint[i] = Math.round(t * (1 - out.polar[i] / 255) * 255);
        out.warm[i] = Math.round(smooth(((max - height[i]) / span - 0.45) / 0.35 + 0.25) * 255);
      }
    }
  };

  return {
    out,
    /** returns true once the whole terrain is built */
    step(msBudget: number): boolean {
      const t0 = performance.now();
      while (performance.now() - t0 < msBudget) {
        if (stage === 0) {
          fieldRow(row++);
          if (row >= TH) (stage = 1), (row = 0);
        } else if (stage === 1) {
          applyCrater(craters[crater++]);
          if (crater >= craters.length) stage = 2;
        } else {
          derive();
          return true;
        }
      }
      return false;
    },
  };
}

export type TerrainBuilder = ReturnType<typeof createTerrainBuilder>;
