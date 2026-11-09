/**
 * Portions adapted from Fumadocs (apps/docs/app/(landing)/sections/parts/scene.ts)
 * https://github.com/fuma-nama/fumadocs
 * MIT License, Copyright (c) 2022 Fuma Nama. See ../workflow/rig/LICENSE-fumadocs.txt.
 *
 * The pedestal, the two-link arm and its inverse kinematics, the module case and its plate, display and post
 * construction, and the oblique view are Fumadocs'. The six modules, the cartridge and its shelf are Numinance's.
 */
import { FACING, MATERIAL, type Shape, type Vec3 } from '../workflow/rig/mesh';
import { ORDER, type ModuleId } from './model';

/** units are CSS px at the drawing's full size: y up, z toward the viewer */
export const W = 380;
export const D = 150;
export const GAP = 10;
const FEET = 6;
const DOCK: Vec3 = [W + 40, 8, D + 36];
/** a nameplate's mount: its face is the page's nameplate, laid on it */
export const PLATE = { w: 216, h: 38, x: -W / 2 + 14 + 108, z: D / 2 + 2 } as const;

export interface Screen {
  at: Readonly<Vec3>;
  size: readonly [number, number];
}

export interface Piece {
  shape: Shape;
  material: number;
  at: Vec3;
  size: Vec3;
  /** radians round x, then y, then z */
  turn?: Vec3;
  /** its own solid, inked round; else one with the body it sits on */
  solid?: boolean;
  plate?: boolean;
  /** drawn only while group [0] of its module is on one of [1] */
  when?: [group: number, options: number[]];
  /** its material by group [0]'s option, [1] */
  dress?: [group: number, materials: number[]];
}

export interface ModuleGeometry {
  id: ModuleId;
  height: number;
  pieces: Piece[];
}

function plate(y = 0): Piece {
  return {
    shape: 'box',
    material: MATERIAL.ink,
    at: [PLATE.x, y, PLATE.z - 1],
    size: [PLATE.w, PLATE.h, 2],
    solid: true,
    plate: true,
  };
}

function display({ at, size }: Screen): Piece[] {
  return [
    { shape: 'box', material: MATERIAL.grey, at: [at[0], at[1], at[2] - 1.5], size: [size[0] + 6, size[1] + 6, 2], solid: true },
    { shape: 'box', material: MATERIAL.ink, at: [at[0], at[1], at[2] - 0.25], size: [size[0], size[1], 0.5] },
  ];
}

function posts(height: number): Piece[] {
  const out: Piece[] = [];
  for (const x of [-W / 2 + 22, W / 2 - 22]) {
    for (const z of [-D / 2 + 20, D / 2 - 20]) {
      out.push({ shape: 'cylinder', material: MATERIAL.grey, at: [x, -height / 2 - GAP / 2, z], size: [10, GAP, 10], solid: true });
    }
  }
  return out;
}

/** a row of `n` lamps along a module's face at `y`: lamp k lit while group 0 is on k */
function lamps(n: number, x0: number, step: number, y: number): Piece[] {
  const out: Piece[] = [];
  for (let k = 0; k < n; k++) {
    const at: Vec3 = [x0 + k * step, y, D / 2 + 1];
    out.push({ shape: 'sphere', material: MATERIAL.grey, at, size: [8, 8, 8], solid: true });
    out.push({ shape: 'sphere', material: MATERIAL.accent, at: [at[0], at[1], at[2] + 0.5], size: [9, 9, 9], solid: true, when: [0, [k]] });
  }
  return out;
}

/** what the monitor shows: the engine's readout */
export const DISPLAYS = {
  engine: { at: [0, 180 / 2 - 8 - (117 + 12) / 2, D / 2 + 3], size: [W - 32, 117] },
  kernel: { at: [102, 0, D / 2 + 2.5], size: [118, 86] },
} as const satisfies Partial<Record<ModuleId, Screen>>;

function engine(): ModuleGeometry {
  const height = 180;
  const { at, size } = DISPLAYS.engine;
  const knobs = -height / 2 + 8 + PLATE.h / 2;
  const pieces: Piece[] = [
    { shape: 'box', material: MATERIAL.paper, at: [0, 0, 0], size: [W, height, D] },
    { shape: 'box', material: MATERIAL.ink, at: [0, at[1], D / 2 + 1], size: [size[0] + 12, size[1] + 12, 2], solid: true },
    { shape: 'box', material: MATERIAL.paper, at: [0, at[1], at[2] - 0.5], size: [...size, 1] },
    plate(knobs),
    { shape: 'cylinder', material: MATERIAL.ink, at: [118, knobs, D / 2 + 3], size: [18, 6, 18], turn: FACING, solid: true },
    { shape: 'cylinder', material: MATERIAL.paper, at: [146, knobs, D / 2 + 2.5], size: [13, 5, 13], turn: FACING, solid: true },
    { shape: 'sphere', material: MATERIAL.accent, at: [170, knobs, D / 2 + 1], size: [9, 9, 9], solid: true },
  ];
  for (let i = 0; i < 9; i++) {
    pieces.push({ shape: 'box', material: MATERIAL.ink, at: [-128 + i * 32, height / 2 + 0.5, -6], size: [14, 1, D - 50] });
  }
  return { id: 'engine', height, pieces: [...pieces, ...posts(height)] };
}

function kernel(): ModuleGeometry {
  const height = 100;
  const pieces: Piece[] = [{ shape: 'box', material: MATERIAL.ink, at: [0, 0, 0], size: [W, height, D] }, plate(), ...display(DISPLAYS.kernel)];
  // proud of the case on every face it shows, so neither face wins by a rounding
  for (const x of [-W / 2 + 6, W / 2 - 13.5]) {
    pieces.push({ shape: 'box', material: MATERIAL.hazard, at: [x, 0, 0.5], size: [x < 0 ? 12 : 29, height + 1, D + 1], solid: true });
  }
  return { id: 'kernel', height, pieces: [...pieces, ...posts(height)] };
}

function config(): ModuleGeometry {
  const height = 52;
  const pieces: Piece[] = [{ shape: 'box', material: MATERIAL.paper, at: [0, 0, 0], size: [W, height, D] }, plate()];
  // six tabs, one per kind of configuration; the chosen one is lit
  for (let k = 0; k < 6; k++) {
    const at: Vec3 = [30 + k * 24, 0, D / 2 + 1];
    pieces.push({ shape: 'box', material: MATERIAL.ink, at, size: [18, 22, 2], solid: true });
    pieces.push({ shape: 'box', material: MATERIAL.accent, at: [at[0], at[1], at[2] + 1], size: [12, 6, 1], solid: true, when: [0, [k]] });
  }
  return { id: 'config', height, pieces: [...pieces, ...posts(height)] };
}

function adapters(): ModuleGeometry {
  const height = 62;
  const pieces: Piece[] = [{ shape: 'box', material: MATERIAL.ink, at: [0, 0, 0], size: [W, height, D] }, plate()];
  // three ports, each holding a cartridge dressed for the option in it
  const ports: [number, number[]][] = [
    [0, [MATERIAL.accent, MATERIAL.paper, MATERIAL.grey]],
    [1, [MATERIAL.paper, MATERIAL.grey]],
    [2, [MATERIAL.grey, MATERIAL.paper]],
  ];
  for (const [g, mats] of ports) {
    const x = 62 + g * 46;
    pieces.push({ shape: 'box', material: MATERIAL.grey, at: [x, 0, D / 2 + 0.5], size: [38, 40, 1], solid: true });
    pieces.push({ shape: 'box', material: MATERIAL.paper, at: [x, 0, D / 2 + 3], size: [30, 32, 4], solid: true, dress: [g, mats] });
    pieces.push({ shape: 'box', material: MATERIAL.ink, at: [x, 8, D / 2 + 5.5], size: [18, 1, 1] });
  }
  return { id: 'adapters', height, pieces: [...pieces, ...posts(height)] };
}

function evidence(): ModuleGeometry {
  const height = 46;
  const pieces: Piece[] = [{ shape: 'box', material: MATERIAL.paper, at: [0, 0, 0], size: [W, height, D] }, plate()];
  // a chain of records, linked
  for (let k = 0; k < 5; k++) {
    pieces.push({ shape: 'box', material: MATERIAL.ink, at: [26 + k * 26, 0, D / 2 + 1], size: [16, 16, 2], solid: true });
    if (k < 4) pieces.push({ shape: 'box', material: MATERIAL.ink, at: [39 + k * 26, 0, D / 2 + 0.75], size: [10, 2, 1.5] });
  }
  pieces.push(...lamps(3, 152, 13, -14));
  return { id: 'evidence', height, pieces: [...pieces, ...posts(height)] };
}

function boundary(): ModuleGeometry {
  const height = 52;
  const pieces: Piece[] = [
    { shape: 'box', material: MATERIAL.ink, at: [0, 0, 0], size: [W, height, D], dress: [0, [MATERIAL.ink, MATERIAL.paper, MATERIAL.grey]] },
    plate(),
    { shape: 'cylinder', material: MATERIAL.accent, at: [W / 2 - 24, 12, D / 2 + 2], size: [12, 4, 12], turn: FACING, solid: true },
    { shape: 'box', material: MATERIAL.paper, at: [W / 2 - 24, -9, D / 2 + 2], size: [12, 16, 4], solid: true },
    // managed: an uplink bar to the control plane
    { shape: 'box', material: MATERIAL.paper, at: [70, 0, D / 2 + 1], size: [76, 6, 2], when: [0, [0]] },
    { shape: 'box', material: MATERIAL.accent, at: [110, 0, D / 2 + 2], size: [8, 10, 3], solid: true, when: [0, [0]] },
    // sovereign: a closed bolt, nothing drawn into it
    { shape: 'cylinder', material: MATERIAL.grey, at: [84, 0, D / 2 + 2], size: [22, 5, 22], turn: FACING, solid: true, when: [0, [1]] },
    { shape: 'box', material: MATERIAL.ink, at: [84, 0, D / 2 + 4.5], size: [16, 3, 2], solid: true, when: [0, [1]] },
    // air-gapped: hazard stripes, not built
    { shape: 'box', material: MATERIAL.hazard, at: [84, 0, D / 2 + 1.5], size: [100, 22, 2], solid: true, when: [0, [2]] },
  ];
  for (const x of [-W / 2 + 30, W / 2 - 30]) {
    for (const z of [-D / 2 + 24, D / 2 - 24]) {
      pieces.push({ shape: 'box', material: MATERIAL.ink, at: [x, -height / 2 - FEET / 2, z], size: [36, FEET, 28], solid: true });
    }
  }
  return { id: 'boundary', height, pieces };
}

const BUILDERS: Record<ModuleId, () => ModuleGeometry> = { engine, kernel, config, adapters, evidence, boundary };

/** bottom to top */
export const STACKED: ModuleGeometry[] = [...ORDER].reverse().map((id) => BUILDERS[id]());
export const SLOT_OF: Record<ModuleId, number> = Object.fromEntries(STACKED.map((m, i) => [m.id, i])) as Record<ModuleId, number>;

export const PLACES: number[] = (() => {
  const out: number[] = [];
  let y = DOCK[1] + FEET;
  for (const module of STACKED) {
    out.push(y + module.height / 2);
    y += module.height + GAP;
  }
  return out;
})();

export const STACK = PLACES[PLACES.length - 1] + STACKED[STACKED.length - 1].height / 2;

export const SHOULDER: [number, number] = [-510, 150];
export const BOOM = 236;
export const FOREARM = 200;
export const TOOL = 46;
const FOOT = 80;
export const RAM_FOOT: Vec3 = [SHOULDER[0] - 62, 80, 30];
export const RAM_HEAD: [number, number] = [82, 22];

/** where spare cartridges wait: a small plinth between the arm and the stack */
export const SHELF = { x: -262, top: 44 } as const;
/** a cartridge's size, and where its centre sits off the pad's face while held */
export const CARTRIDGE: Vec3 = [30, 26, 46];
export const HELD = CARTRIDGE[0] / 2;
/** the pad's place when it grips the cartridge on its plinth */
export const GRIP: [number, number] = [SHELF.x - HELD, SHELF.top + CARTRIDGE[1] / 2];

export const BASE: Piece[] = [
  { shape: 'box', material: MATERIAL.paper, at: [0, DOCK[1] / 2, 0], size: DOCK },
  ...[-1, 1].map((side): Piece => ({ shape: 'box', material: MATERIAL.ink, at: [side * (W / 2 + 6), DOCK[1] + 0.5, D / 2 + 6], size: [16, 1, 8] })),
  { shape: 'box', material: MATERIAL.hazard, at: [SHOULDER[0], 7, 0], size: [2 * FOOT, 14, 2 * FOOT] },
  { shape: 'box', material: MATERIAL.paper, at: [SHOULDER[0], 36, 0], size: [150, 44, 150] },
  { shape: 'cylinder', material: MATERIAL.grey, at: [SHOULDER[0], 66, 0], size: [124, 16, 124], solid: true },
  { shape: 'box', material: MATERIAL.paper, at: [SHOULDER[0], 106, -34], size: [68, 70, 10], solid: true },
  { shape: 'box', material: MATERIAL.paper, at: [SHOULDER[0], 106, 34], size: [68, 70, 10], solid: true },
  { shape: 'box', material: MATERIAL.ink, at: [RAM_FOOT[0], RAM_FOOT[1] - 4, RAM_FOOT[2]], size: [16, 14, 14], solid: true },
  ...[54, 18, -18].map((x): Piece => ({ shape: 'prism', material: MATERIAL.ink, at: [SHOULDER[0] + x, 36, 76.5], size: [9, 3, 9], turn: FACING, solid: true })),
  { shape: 'sphere', material: MATERIAL.accent, at: [SHOULDER[0] - 54, 36, 76], size: [10, 10, 10], solid: true },
  // the cartridge plinth
  { shape: 'box', material: MATERIAL.paper, at: [SHELF.x, SHELF.top / 2, 0], size: [64, SHELF.top, 72], solid: true },
  { shape: 'box', material: MATERIAL.hazard, at: [SHELF.x, 5, 0], size: [66, 10, 74], solid: true },
  { shape: 'box', material: MATERIAL.ink, at: [SHELF.x, SHELF.top + 0.5, 0], size: [44, 1, 56] },
];

export const BOOM_PIECES: Piece[] = [
  { shape: 'cylinder', material: MATERIAL.grey, at: [0, 0, 0], size: [62, 88, 62], turn: FACING },
  { shape: 'cylinder', material: MATERIAL.accent, at: [0, 0, 45], size: [32, 3, 32], turn: FACING, solid: true },
  { shape: 'box', material: MATERIAL.paper, at: [BOOM / 2, 0, 0], size: [BOOM, 36, 42] },
  ...[0.36, 0.6, 0.82].map((t): Piece => ({ shape: 'cylinder', material: MATERIAL.ink, at: [BOOM * t, 0, 21.5], size: [15, 1, 15], turn: FACING })),
  { shape: 'box', material: MATERIAL.ink, at: [-50, 0, 0], size: [56, 60, 50], solid: true },
  { shape: 'box', material: MATERIAL.hazard, at: [-50, 0, 25.5], size: [56, 15, 1] },
];

export const FOREARM_PIECES: Piece[] = [
  { shape: 'cylinder', material: MATERIAL.grey, at: [0, 0, 0], size: [48, 60, 48], turn: FACING },
  { shape: 'cylinder', material: MATERIAL.accent, at: [0, 0, 31], size: [26, 3, 26], turn: FACING, solid: true },
  { shape: 'box', material: MATERIAL.paper, at: [FOREARM / 2, 0, 0], size: [FOREARM, 28, 32] },
  { shape: 'box', material: MATERIAL.ink, at: [FOREARM / 2 + 6, 0, 16.5], size: [FOREARM - 64, 6, 1] },
];

export const TOOL_PIECES: Piece[] = [
  { shape: 'cylinder', material: MATERIAL.grey, at: [0, 0, 0], size: [34, 42, 34], turn: FACING },
  { shape: 'box', material: MATERIAL.paper, at: [20, 0, 0], size: [26, 28, 38], solid: true },
  { shape: 'box', material: MATERIAL.ink, at: [TOOL - 3, 0, 0], size: [6, 46, 60], solid: true },
  { shape: 'cylinder', material: MATERIAL.ink, at: [18, 21, 12], size: [5, 18, 5], solid: true },
];

export const RAM_PIECES: Piece[] = [
  { shape: 'cylinder', material: MATERIAL.ink, at: [0, 0, 0], size: [1, 1, 1], solid: true },
  { shape: 'cylinder', material: MATERIAL.ink, at: [0, 0, 0], size: [1, 1, 1], solid: true },
];

/** a cartridge: what the arm carries from the plinth to a port, centred on its own middle */
export const CARTRIDGE_PIECES: Piece[] = [
  { shape: 'box', material: MATERIAL.paper, at: [0, 0, 0], size: CARTRIDGE },
  { shape: 'box', material: MATERIAL.accent, at: [0, CARTRIDGE[1] / 2 + 0.5, 0], size: [14, 1, 26], solid: true },
  { shape: 'box', material: MATERIAL.ink, at: [CARTRIDGE[0] / 2, 0, 0], size: [2, 18, 36], solid: true },
];

/** the boom's and forearm's angles that put the pad's face at `pad`, the elbow up; `pad` itself is brought in reach */
export function reach(pad: [number, number], out: [number, number]): [number, number] {
  let tx = pad[0] - TOOL - SHOULDER[0];
  let ty = pad[1] - SHOULDER[1];
  const raw = Math.hypot(tx, ty);
  const d = Math.min(Math.max(raw, Math.abs(BOOM - FOREARM) + 1), BOOM + FOREARM - 1);
  if (raw > 0 && d !== raw) {
    tx *= d / raw;
    ty *= d / raw;
    pad[0] = tx + TOOL + SHOULDER[0];
    pad[1] = ty + SHOULDER[1];
  }
  const toward = Math.atan2(ty, tx);
  const inner = Math.acos((BOOM * BOOM + d * d - FOREARM * FOREARM) / (2 * BOOM * d));
  const boom = toward + inner;
  const ex = Math.cos(boom) * BOOM;
  const ey = Math.sin(boom) * BOOM;
  out[0] = boom;
  out[1] = Math.atan2(ty - ey, tx - ex);
  return out;
}

export const FLOOR: Piece = { shape: 'box', material: MATERIAL.floor, at: [0, -0.5, 0], size: [2400, 1, 600] };
export const BOUNDS: [Vec3, Vec3] = [
  [SHOULDER[0] - FOOT - 40, -1, -D / 2 - 120],
  [W / 2 + 60, STACK + 20, D / 2 + 40],
];

export const LIGHT: Vec3 = [0.2, 0.86, 0.47];

/** a unit back goes this far right and up: the 2:1 of a pixel line */
export const DEPTH: [number, number] = [0.447, 0.224];

export interface View {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

const FRONT = -DEPTH[1] * (DOCK[2] / 2) - 2;
const TOP = STACK + DEPTH[1] * (D / 2) + 2;

/** from 768px the whole machine; on phones from just past the pedestal, the arm over the edge */
export const VIEWS: Record<'wide' | 'phone', View> = {
  wide: {
    left: SHOULDER[0] - FOOT - DEPTH[0] * FOOT - 2,
    right: DOCK[0] / 2 + DEPTH[0] * (DOCK[2] / 2) + 2,
    top: TOP,
    bottom: FRONT,
  },
  phone: { left: SHELF.x - 56, right: DOCK[0] / 2 + DEPTH[0] * (DOCK[2] / 2) + 2, top: TOP, bottom: FRONT },
};

/** where the pad waits while a new module is carried in, and where it parks */
export const REST: [number, number] = [-250, STACK - 80];
export const HOME: [number, number] = [-W / 2 - 60, 250];
