/**
 * The workflow rig's engine: one clock (the mesh's frames), a movement planner, and a hand-over to the pointer.
 *
 * The planner (`move`, `sample`, `plan`, `replan`, the `take` hand-over and the `aimFor`/`aimStep` springs) and the
 * bed/column station geometry are adapted from Fumadocs' write-scene engine (MIT, (c) Fuma Nama; see
 * LICENSE-fumadocs.txt). What it drives is Numinance's: the four stage regions are inspected one at a time by the
 * probe, the stage they show is a function of the plan's clock, and the pixel operator (lib/landing/operator.ts)
 * rides the deck on that same clock.
 *
 *   ready  -> the intro is planned, the arm parked, nothing runs
 *   play   -> the carriage drives, the boom turns, the probe drops and scans a region, lifts, repeats, parks
 *   rest   -> the intro has played once; the rig is the reader's: `take()` hands the planned pose and its velocity
 *             to critically damped springs that follow the pointer; leaving parks it again
 *   fade   -> Replay: the regions dither out under a cover, then the intro plans again from the arm's pose
 *
 * A pose is [carriage, boom angle, probe angle from the boom, probe pitch off the plane].
 */
import { OPERATOR_H, OPERATOR_W, drawOperator, type OperatorFrame } from '@/lib/landing/operator';
import { readPalette } from '@/lib/landing/palette';
import { ditherKeyframes } from './dither';
import {
  type Anchor,
  createMesh,
  housePalette,
  identity,
  type Mat4,
  MATERIAL,
  type Mesh,
  multiply,
  normalize,
  type PartSpec,
  type Shape,
  transform,
} from './mesh';
import { clamp, easeInOut, easeOut, type Rect, rectIn, steer, TICK } from './util';
import {
  BED,
  BED_DECK,
  BOOM,
  COLUMN_DECK,
  HUB,
  PEN,
  PROBE,
  type Pose,
  build,
  FRAMES,
  type Part,
  pose as poseRig,
  shading,
  type Vec2,
} from './scene';

/** a move's beats, ms */
const DRIVE = 460;
const TIP_UP = 140;
const TIP_DOWN = 200;
const DROP = 160;
const LIFT = 160;
const REST = 60;
const PARK = 700;
/** the scan's pace: ms a px of a region, within SLIDE */
const SLIDE_MS = 2;
const SLIDE: Vec2 = [420, 820];
/** ms the intro waits before the first move, and between one region's scan and the next move */
const LEAD_IN = 450;
const GAP = 140;
/** a long move cruises at TOP px/ms between RAMP ms ramps; the drive's curve is at its steepest STEEPEST times its
 * mean speed, so under SHORT px it never passes TOP */
const TOP = 2.8;
const RAMP = 160;
const STEEPEST = 2.857;
const SHORT = (TOP * DRIVE) / STEEPEST;
/** of the drop, the share after the arm has stopped */
const SETTLE = 0.25;
/** ms a radian takes at its gentlest: the boom's turn, the elbow's bend */
const TURN_RATE = 140;
const BEND_RATE = 110;
/** a phone's elbow sweep (px/ms); rad off its aim a boom still swings; windows' overlap share */
const SWING = 1;
const SWUNG = 0.3;
const OVERLAP = 0.35;
/** the elbow's inner angle at both ends of a scan, and pointing at a cursor, rad; folded at rest it leaves TUCK */
const INNER: Vec2 = [(50 * Math.PI) / 180, (150 * Math.PI) / 180];
const AIMED = (100 * Math.PI) / 180;
const TUCK = 0.06;
/** the probe stood up on its trunnion; up a phone's column never further, never end-on */
const UPRIGHT = Math.PI / 2;
const STEEP = Math.PI / 3;
/** ms the regions take to dither out for a replay */
const FADE = (16 * 1000) / 60;
/** world px: how high the camera stands over the plane */
const CAMERA = 4000;
/** high from the upper right, so shadows stay short and fall on the plates or into the bay */
const LIGHT = normalize([0.13, -0.18, 0.975]);
/** the pointer's springs: stiffness (1/s), top speeds (px/s, rad/s), the probe's tip; up a phone's column the arm
 * follows the viewport's middle on a stiffer one */
const AIM = 10;
const AIM_SPEED = 1500;
const AIM_SLEW = 7;
const AIM_TIP = 16;
const SCROLL_AIM = 16;
const AIM_GAP = 10;
/** CSS px one of the operator's cells takes */
const CELL = 5;
const OW = OPERATOR_W * CELL;
const OH = OPERATOR_H * CELL;
/** the pressure gauge: at rest, while the probe scans, as it lifts */
const GAUGE: [number, number, number] = [0.2, 0.75, 1];
/** arm units round the elbow its eye, crank and yoke reach, kept inside the window */
const ELBOW = 50;
const DOWN = Math.PI / 2;
/** the probe's radius to its hexagon's corners, arm units */
const PEN_R = PROBE.radius * 1.16;
/** what casts the operator's shadow, CSS px round the seat: head and body, legs */
const HER: { shape: Shape; m: Mat4 }[] = [
  { shape: 'box', m: transform(identity(), [0, -34, 0], undefined, [34, 36, 1]) },
  { shape: 'box', m: transform(identity(), [0, -8, 0], undefined, [28, 16, 1]) },
];

/** the stage each of the intro's scans brings up: [region, stage, downwards] */
const INTRO: readonly [number, number, boolean][] = [
  [0, 1, true],
  [1, 1, true],
  [2, 2, true],
  [3, 3, true],
  [3, 4, false],
];
/** the scan a chip asks for, by stage; Reset has none */
const SCAN_OF: readonly ([number, boolean] | null)[] = [[0, true], [1, true], [2, true], [3, true], [3, false], null];
/** the stage the finished intro leaves on screen: everything recorded and verified */
export const FINAL_STAGE = 4;

function within(u: number, w: Vec2): number {
  if (w[1] <= w[0]) return u >= w[1] ? 1 : 0;
  return clamp((u - w[0]) / (w[1] - w[0]));
}

/** a long move's progress with ramps of share r: the camera curve's halves round a cruise */
function cruise(r: number): (u: number) => number {
  const v = 1 / (1 - 1.3 * r);
  const d = 0.35 * r * v;
  return (u) => {
    if (u <= r) return 2 * d * easeInOut(u / (2 * r));
    if (u >= 1 - r) return 1 - 2 * d * easeInOut((1 - u) / (2 * r));
    return d + v * (u - r);
  };
}

function inBox(x: number, y: number, r: number, b: Rect): boolean {
  return x > b.x - r && x < b.x + b.w + r && y > b.y - r && y < b.y + b.h + r;
}

/** `angle` turned by whole turns nearest to `near`, so a turn takes the short way round */
function nearest(angle: number, near: number): number {
  const turn = 2 * Math.PI;
  return angle + Math.round((near - angle) / turn) * turn;
}

/** spans in order, the overlapping ones joined */
function merge(spans: Vec2[]): Vec2[] {
  spans.sort((a, b) => a[0] - b[0]);
  const out: Vec2[] = [];
  for (const [from, to] of spans) {
    const last = out.at(-1);
    if (last && from <= last[1]) last[1] = Math.max(last[1], to);
    else out.push([from, to]);
  }
  return out;
}

/** a move from pose `a` to `b` over [t0, t1]; a scan (`task`) runs the probe's point straight */
interface Segment {
  t0: number;
  t1: number;
  a: number[];
  b: number[];
  ease: (u: number, i: number) => number;
  tilt?: (u: number) => number;
  bend?: (u: number) => number;
  task?: { a: number[]; b: number[]; bend: number };
}

interface Leg {
  end: number[];
  time: number;
  ease: (u: number, i: number) => number;
  tilt: (u: number) => number;
  bend?: (u: number) => number;
}

/** a stage region: the probe scans down (or up) its body */
interface Block {
  index: number;
  el: HTMLElement;
  head: HTMLElement;
  cell: Rect;
  tabBox: Rect;
  /** its scan: the line's x, its top and foot, the carriage at each, the elbow's side */
  tipX: number;
  range: Vec2;
  stop: number;
  stopEnd: number;
  bend: number;
}

interface Ask {
  block: Block;
  /** scans downwards (true) or upwards */
  out: boolean;
  /** the stage that comes up when the probe touches */
  stage: number;
  reader: boolean;
}

/** a planned scan: it sets off at `go`, scans from `touch` to `hit`, and is free at `free` */
interface Job {
  ask: Ask;
  go: number;
  touch: number;
  hit: number;
  free: number;
}

export interface Press {
  /** a reader's chip: bring stage `i` up, by the arm when it can */
  press(stage: number): void;
  /** dither the regions out and play the introduction again */
  replay(): void;
  destroy(): void;
}

/**
 * The stage's rig. Everything on screen that moves is a function of the plan's clock, run on the mesh's frames; after
 * the intro the rig is the reader's. The arm's canvas and its context are made as the stage first comes on screen.
 * `onStage` is told each change of the stage on show (discrete: React never sees a frame).
 */
export function createPress(stage: HTMLElement, onStage: (stage: number) => void, reduced: boolean): Press {
  const doc = stage.querySelector<HTMLElement>('[data-doc]')!;
  const grid = stage.querySelector<HTMLElement>('[data-grid]')!;
  const bed = stage.querySelector<HTMLElement>('[data-bay]')!;
  const column = stage.querySelector<HTMLElement>('[data-bay-phone]')!;
  const runs = stage.querySelector<HTMLElement>('[data-chain="runs"]')!;
  const fixedRun = stage.querySelector<HTMLElement>('[data-chain="fixed"]')!;
  const pulledRun = stage.querySelector<HTMLElement>('[data-chain="pulled"]')!;
  const bend = stage.querySelector<SVGElement>('[data-chain="bend"]')!;
  const stations = stage.querySelectorAll<SVGElement>('[data-station]');
  const scan = stage.querySelector<HTMLElement>('[data-scan]')!;
  const cover = stage.querySelector<HTMLElement>('[data-cover]')!;
  // what the frames move, they move by transform, steered while the frames run
  const moved: Element[] = [runs, fixedRun, pulledRun, bend];
  let steered: (() => void)[] = [];

  // the operator: a canvas of its own cells, painted when its frame changes, kept on the deck by the mesh
  const mate = document.createElement('canvas');
  mate.width = OW;
  mate.height = OH;
  mate.setAttribute('aria-hidden', 'true');
  Object.assign(mate.style, {
    position: 'absolute',
    top: '0',
    left: '0',
    zIndex: '3',
    width: `${OW}px`,
    height: `${OH}px`,
    pointerEvents: 'none',
    visibility: 'hidden',
    imageRendering: 'pixelated',
  });
  stage.append(mate);
  const mateCtx = mate.getContext('2d')!;
  let matePalette = readPalette();
  let mateShown = '';

  const blocks: Block[] = [];
  for (const el of stage.querySelectorAll<HTMLElement>('[data-region]')) {
    blocks.push({
      index: blocks.length,
      el,
      head: el.querySelector('header')!,
      cell: { x: 0, y: 0, w: 0, h: 0 },
      tabBox: { x: 0, y: 0, w: 0, h: 0 },
      tipX: 0,
      range: [0, 0],
      stop: 0,
      stopEnd: 0,
      bend: -1,
    });
  }

  let mesh: Mesh | null = null;
  let made = false;
  let canvas: HTMLCanvasElement | null = null;
  /** the stage shows */
  let visible = false;
  /** the stage has crossed the middle of the viewport, so the intro starts once the arm can draw */
  let seen = false;
  /** the canvas's CSS px size */
  const size: Vec2 = [1, 1];

  // metrics, stage px
  let laidOut = false;
  let foot: Vec2 = [0, 0];
  /** the rail: along x (0) on the bed, y (1) up a phone's column, from (x0, y0) to (x1, y1) */
  let along = 0;
  const rail = [0, 0, 0, 0];
  let deck = BED_DECK;
  /** CSS px an arm unit takes on the deck in use; the boom, and the probe from the elbow to its point */
  let scale = deck.scale;
  let boom = BOOM * scale;
  let fore = PROBE.reach * scale;
  let band: Rect = { x: 0, y: 0, w: 0, h: 0 };
  let below = false;
  let inside: Rect = { x: 0, y: 0, w: 0, h: 0 };
  /** carriage spans on the bed where the operator would stand on a tab, and at a stop the hub too */
  let herOff: Vec2[] = [];
  let stopOff: Vec2[] = [];

  // the plan: the asked scans in order from a pose at a time, then the park
  const asks: Ask[] = [];
  const jobs: Job[] = [];
  const track: Segment[] = [];
  const pen: Segment[] = [];
  let park: number[] = [0, 0, TUCK - Math.PI, 0];
  /** up a phone's column, how far the folded probe opens off the boom to the window's border */
  let fold = TUCK;
  let from: number[] = park;
  let since = 0;
  let total = 0;
  let clock = 0;
  /** the stage on show before the plan's first scan touches */
  let base = 0;
  let shown = -1;
  /** rest: nothing to do; ready: the intro is planned, awaiting the view; fade: the regions dither out */
  let phase: 'rest' | 'ready' | 'fade' | 'play' = 'rest';
  /** the intro is planned (till then the stage is the server's), and its sequence still plays */
  let begun = false;
  let playing = false;
  let fading = 0;
  let last = -1;

  let aiming = false;
  let following = false;
  /** the stage's top left in the viewport, read once a frame while it follows */
  const view: Vec2 = [0, 0];
  const pointer: Vec2 = [0, 0];
  const cursorAt: Vec2 = [0, 0];
  const aimAt = [0, 0, 0, 0];
  const aimPose = [0, 0, 0, 0];
  const aimSpeed = [0, 0, 0, 0];
  const targets = [0, 0, 0];
  let lift: number = PEN.up;

  const pose = [0, 0, 0, 0];
  const up: number[] = [PEN.up];
  const sole: number[] = [PEN.up];
  const point: [number, number, number] = [0, 0, 1];
  let covering: Animation | undefined;
  /** the rig's pose as the mesh lays it, the rig's frame, and the operator's: a CSS px a unit at the seat */
  const posed: Pose = { turn: 0, dir: [1, 0], pen: PEN.up, pitch: 0, gauge: GAUGE[0] };
  const root = identity();
  const her = identity();
  const m = identity();
  /** the rig's parts on the deck in use, its frames and theirs on the plane */
  let rig: Part[] = [];
  const frames: Mat4[] = [];
  const placed: Mat4[] = [];
  for (let i = 0; i < FRAMES; i++) {
    frames.push(identity());
    placed.push(identity());
  }
  let proxy = 0;
  let floor = 0;
  const bounds = [0, 0, 0, 0];
  const clip = { x: 0, y: 0, w: 0, h: 0 };
  let scanAnchor: Anchor | undefined;
  let herAnchor: Anchor | undefined;
  let scanned: Block | null = null;
  const seat: [number, number, number] = [0, 0, 0];
  const pivotAt: Vec2 = [0, 0];
  const elbowAt: Vec2 = [0, 0];
  const tipAt: Vec2 = [0, 0];
  let chainAt = -1;
  let steady = false;
  /** the carriage's last x, ms of the last paint, for the operator's stride */
  let prevPivot = Number.NaN;
  let strideAt = 0;
  let strolling = false;
  let facing = false;

  function layout() {
    const s = stage.getBoundingClientRect();
    if (s.width === 0) return;
    laidOut = true;
    const page = rectIn(doc, s);
    inside = rectIn(grid, s);
    for (const b of blocks) {
      b.tabBox = rectIn(b.head, s);
      b.cell = rectIn(b.el, s);
      b.range = [b.tabBox.y + b.tabBox.h, b.cell.y + b.cell.h];
    }
    band = rectIn(bed, s);
    along = band.w > 0 ? 0 : 1;
    deck = along === 0 ? BED_DECK : COLUMN_DECK;
    rig = build(deck, posed);
    scale = deck.scale;
    boom = BOOM * scale;
    fore = PROBE.reach * scale;
    foot = [page.x + page.w / 2, s.height * 0.3];
    chainAt = -1;
    if (along === 0) {
      rail[0] = band.x + BED.start;
      rail[1] = band.y + BED.rail;
      // anywhere short of this the boom turns with its elbow and crank inside the band's end
      rail[2] = band.x + band.w - (BOOM + ELBOW) * scale;
      rail[3] = rail[1];
      below = blocks.some((b) => b.cell.y > rail[1]);
      offTabs();
      park = [clear(rail[0], herOff), 0, TUCK - Math.PI, 0];
      for (const b of blocks) station(b);
      // the detents the carriage stops on, home's first; stops within one's width share it
      const stops = [park[0]];
      for (const b of blocks) stops.push(b.stop);
      const width = stations[0]?.getBoundingClientRect().width ?? 0;
      for (let i = 0; i < stations.length && i < stops.length; i++) {
        let shared = false;
        for (let j = 0; j < i; j++) if (Math.abs(stops[j] - stops[i]) < width) shared = true;
        stations[i].style.left = `${Math.round(stops[i] - band.x - width / 2)}px`;
        stations[i].style.visibility = shared ? 'hidden' : 'visible';
      }
    } else {
      const bay = rectIn(column, s);
      rail[0] = bay.x + bay.w / 2;
      rail[2] = rail[0];
      // the operator's crown, as the camera sees it, at the window's top; the deck's foot at its bottom
      const k = CAMERA / (CAMERA - deck.seat[2] * scale);
      rail[1] = foot[1] + (page.y + OH + 4 - foot[1]) / k;
      rail[3] = page.y + page.h - (deck.box[1] + deck.box[3]) * scale;
      below = false;
      // the probe flat across the plates as far as the window lets it, the boom up the column under a plate it
      // reaches, else down over it
      let left = Infinity;
      for (const b of blocks) left = Math.min(left, b.cell.x);
      const reach = Math.min(fore, rail[0] - left);
      const drop = boom + Math.sqrt(fore * fore - reach * reach);
      for (const b of blocks) {
        b.tipX = rail[0] - reach;
        const up = b.range[1] + drop <= rail[3];
        b.bend = up ? -1 : 1;
        b.stop = clamp(b.range[0] + (up ? drop : -drop), rail[1], rail[3]);
        b.stopEnd = clamp(b.range[1] + (up ? drop : -drop), rail[1], rail[3]);
      }
      // at rest as low as keeps the elbow in the window, the probe open to the window's border
      const lifted = CAMERA / (CAMERA - PEN.up * scale);
      const border = foot[0] + (inside.x + inside.w + 1 - foot[0]) / lifted;
      fold = Math.asin(clamp((rail[0] - border) / fore, 0, 1));
      park = [page.y + page.h - boom - ELBOW * scale, boomAngle(DOWN), Math.PI - fold, 0];
    }
    if (!begun || phase === 'ready') from = park;
    cover.style.left = `${inside.x}px`;
    cover.style.top = `${inside.y}px`;
    cover.style.width = `${inside.w}px`;
    cover.style.height = `${inside.h}px`;
    if (mesh) {
      mesh.camera({
        width: size[0],
        height: size[1],
        eye: [foot[0], foot[1], CAMERA],
        z: [400, -20],
      });
      mesh.light(LIGHT, [0, 0, -1], [s.width, s.height, 320 * scale]);
      // the regions under the rig take its shadow
      transform(m, [inside.x + inside.w / 2, inside.y + inside.h / 2, -0.5], undefined, [inside.w, inside.h, 1]);
      mesh.place(floor, m);
    }
    plan();
    listen();
  }

  /** the carriage spans where the operator's canvas would cover a tab, and at a stop the hub over the boom's root
   * too; on the bed its box moves with the carriage, scaled by the camera */
  function offTabs() {
    const k = CAMERA / (CAMERA - deck.seat[2] * scale);
    const left = foot[0] * (1 - k) + deck.seat[0] * scale * k - OW / 2 - 0.5;
    const width = OW + 1;
    const top = foot[1] + (rail[1] + deck.seat[1] * scale - foot[1]) * k - OH;
    const her: Vec2[] = [];
    const hub: Vec2[] = [];
    for (const { tabBox: t } of blocks) {
      if (t.y < top + OH && t.y + t.h > top) her.push([(t.x - left - width) / k, (t.x + t.w - left) / k]);
      hub.push([t.x - HUB * scale, t.x + t.w + HUB * scale]);
    }
    stopOff = merge(her.concat(hub));
    herOff = merge(her);
  }

  /** c, or the nearest carriage on the rail clear of `off` */
  function clear(c: number, off: Vec2[]): number {
    for (const [start, end] of off) {
      if (c <= start || c >= end) continue;
      const back = start >= rail[0];
      if (end <= rail[2] && (!back || end - c < c - start)) return end;
      return back ? start : c;
    }
    return c;
  }

  /** block b's stop on the bed: the pivot off the scan's line by as much as keeps the elbow's inner angle within
   * INNER at both its ends, on the side the tabs move it least */
  function station(b: Block) {
    const up = b.cell.y < rail[1];
    const near = up ? rail[1] - b.range[1] : b.range[0] - rail[1];
    const far = up ? rail[1] - b.range[0] : b.range[1] - rail[1];
    const d =
      (Math.sqrt(Math.max(0, span(INNER[0]) ** 2 - near * near)) +
        Math.sqrt(Math.max(0, span(INNER[1]) ** 2 - far * far))) /
      2;
    const middle = b.cell.x + b.cell.w / 2;
    let least = Infinity;
    for (const side of [1, -1]) {
      const ideal = middle + side * d;
      const stop = clear(clamp(ideal, rail[0], rail[2]), stopOff);
      if (Math.abs(stop - ideal) >= least) continue;
      least = Math.abs(stop - ideal);
      b.stop = stop;
      b.stopEnd = stop;
      b.tipX = clamp(stop - side * d, b.cell.x, b.cell.x + b.cell.w);
      b.bend = side * (up ? -1 : 1);
    }
  }

  /** the probe's point's distance from the pivot with the elbow's inner angle at `inner` */
  function span(inner: number): number {
    return Math.sqrt(boom * boom + fore * fore - 2 * boom * fore * Math.cos(inner));
  }

  /** whether box [x0, x1) by [y0, y1) lies on a tab */
  function onTab(x0: number, y0: number, x1: number, y1: number): boolean {
    for (const { tabBox: t } of blocks) {
      if (x1 > t.x && x0 < t.x + t.w && y1 > t.y && y0 < t.y + t.h) return true;
    }
    return false;
  }

  function xOf(c: number): number {
    return along === 0 ? c : rail[0];
  }

  function yOf(c: number): number {
    return along === 1 ? c : rail[1];
  }

  function innerAt(r: number, l2 = fore): number {
    return Math.acos(clamp((boom * boom + l2 * l2 - r * r) / (2 * boom * l2), -1, 1));
  }

  /** the joints that put a second link's end at (x, y) from the pivot, into out[1] and out[2] */
  function reachTo(x: number, y: number, bendTo: number, out: number[], l2 = fore) {
    const d = clamp(Math.hypot(x, y), Math.abs(boom - l2) + 0.5, boom + l2 - 0.5);
    const a = Math.acos(clamp((boom * boom + d * d - l2 * l2) / (2 * boom * d), -1, 1));
    out[1] = Math.atan2(y, x) - bendTo * a;
    out[2] = bendTo * (Math.PI - innerAt(d, l2));
  }

  /** the boom on the band's side on the bed, on the plates' side up a phone's column */
  function boomAngle(angle: number): number {
    if (along === 0) return Math.atan2(Math.sin(angle), Math.cos(angle));
    return angle - 2 * Math.PI * Math.ceil((angle + Math.PI / 2 - 0.5) / (2 * Math.PI));
  }

  function solve(task: number[], bendTo: number, near: number[]): number[] {
    const out = [task[0], 0, 0, 0];
    reachTo(task[1], task[2], bendTo, out);
    out[1] = boomAngle(out[1]);
    out[2] = nearest(out[2], near[2]);
    return out;
  }

  function place(p: number[]) {
    pivotAt[0] = xOf(p[0]);
    pivotAt[1] = yOf(p[0]);
    elbowAt[0] = pivotAt[0] + boom * Math.cos(p[1]);
    elbowAt[1] = pivotAt[1] + boom * Math.sin(p[1]);
    tipAt[0] = elbowAt[0] + fore * Math.cos(p[1] + p[2]);
    tipAt[1] = elbowAt[1] + fore * Math.sin(p[1] + p[2]);
  }

  function stowed(p: number[]): boolean {
    return Math.abs(along === 0 ? Math.sin(p[1]) : Math.cos(p[1])) < 0.34;
  }

  function foldAt(angle: number, near: number): number {
    return nearest(Math.PI - fold * Math.sin(angle), near);
  }

  /** a move from pose a to b: the probe tips up, the joints turn while it stands, it tips down; on the bed the trip
   * home first brings the elbow out of the plates, the probe along the band */
  function move(a: number[], b: number[], pace = 1, least = 0): Leg {
    const turn = boomAngle(b[1]);
    // up a phone's column the probe folds along the boom, never end-on
    const flat = along === 1;
    const home = !flat && stowed([b[0], turn]) && !stowed(a);
    const fa = flat ? foldAt(a[1], a[2]) : a[2];
    const fb = flat ? foldAt(turn, fa) : home ? nearest(-turn, a[2]) : a[2];
    const end = [b[0], turn, nearest(b[2], fb), b[3]];
    const run = Math.abs(b[0] - a[0]);
    // a move that goes nowhere only tips the probe across
    const stays = run < 0.5 && Math.abs(end[1] - a[1]) < 1e-3 && Math.abs(end[2] - a[2]) < 1e-3;
    const preT = Math.abs(fa - a[2]) * BEND_RATE * pace;
    const postT = Math.abs(end[2] - fb) * BEND_RATE * pace;
    // a tip takes as long as its drawn length takes to change, so a part tip is no quicker
    const upT = stays ? 0 : flat ? preT : Math.cos(a[3]) * TIP_UP * pace;
    const downT = stays ? 0 : flat ? postT : Math.cos(b[3]) * TIP_DOWN * pace;
    const turnT = Math.abs(end[1] - a[1]) * (flat ? Math.max(TURN_RATE, boom / SWING) : TURN_RATE) * pace;
    const bendT = Math.abs((flat || home ? fb - fa : end[2] - a[2]) * BEND_RATE * pace);
    const jointT = home ? preT + Math.max(turnT, bendT) + postT : Math.max(turnT, bendT);
    const driveT = run < 1 ? 0 : run > SHORT ? Math.max(DRIVE, 1.3 * RAMP + run / TOP) : DRIVE;
    const time = Math.max(
      least,
      pace * driveT + (home ? upT + jointT : 0),
      upT + jointT + downT - OVERLAP * (Math.min(upT, jointT) + Math.min(jointT, downT)),
      160,
    );
    // the joints turn as soon as the probe stands, or out of the stowed pose as late as they can
    const start = stowed(a)
      ? time - downT + OVERLAP * Math.min(jointT, downT) - jointT
      : upT - OVERLAP * Math.min(upT, jointT);
    const joints: Vec2 = [start / time, (start + jointT) / time];
    const upSpan: Vec2 = [0, upT / time];
    const down: Vec2 = [1 - downT / time, 1];
    // the boom's turn: on the trip home between the elbow's, and the carriage only after them
    const turns: Vec2 = home ? [joints[0] + preT / time, joints[1] - postT / time] : joints;
    const drive: Vec2 = home ? [joints[1], 1] : [0, 1];
    const curve = run > SHORT ? cruise(RAMP / (time * (1 - drive[0]))) : easeInOut;
    const before: Vec2 = flat ? upSpan : [joints[0], turns[0]];
    const after: Vec2 = flat ? down : [turns[1], joints[1]];
    return {
      end,
      time,
      ease: (u, i) => (i === 0 ? curve(within(u, drive)) : easeInOut(within(u, turns))),
      bend:
        (flat || home) && !stays
          ? (u) =>
              a[2] +
              (fa - a[2]) * easeInOut(within(u, before)) +
              (fb - fa) * easeInOut(within(u, turns)) +
              (end[2] - fb) * easeInOut(within(u, after))
          : undefined,
      tilt: (u) =>
        stays || flat
          ? a[3] + (b[3] - a[3]) * easeInOut(u)
          : a[3] +
            (UPRIGHT - a[3]) * easeInOut(within(u, upSpan)) -
            (UPRIGHT - b[3]) * easeInOut(within(u, down)),
    };
  }

  function sample(list: Segment[], t: number, first: number[], out: number[]) {
    let at = first;
    for (const s of list) {
      if (t < s.t0) break;
      if (t < s.t1) {
        const u = (t - s.t0) / (s.t1 - s.t0);
        if (s.task) {
          const { a, b } = s.task;
          const k = s.ease(u, 0);
          out[0] = a[0] + (b[0] - a[0]) * k;
          reachTo(a[1] + (b[1] - a[1]) * k, a[2] + (b[2] - a[2]) * k, s.task.bend, out);
          out[1] = boomAngle(out[1]);
          out[2] = nearest(out[2], s.a[2]);
          out[3] = 0;
          return;
        }
        for (let i = 0; i < out.length; i++) {
          out[i] = s.a[i] + (s.b[i] - s.a[i]) * s.ease(u, i);
        }
        if (s.bend) out[2] = s.bend(u);
        if (s.tilt) out[3] = s.tilt(u);
        return;
      }
      at = s.b;
    }
    for (let i = 0; i < out.length; i++) out[i] = at[i];
  }

  function push(leg: Leg, a: number[], t: number) {
    track.push({
      t0: t,
      t1: t + leg.time,
      a,
      b: leg.end,
      ease: leg.ease,
      tilt: leg.tilt,
      bend: leg.bend,
    });
  }

  function plan() {
    if (!laidOut) return;
    track.length = 0;
    pen.length = 0;
    jobs.length = 0;
    let previous = from;
    let free = since;
    let first = true;
    for (const ask of asks) {
      const b = ask.block;
      const c0 = ask.out ? b.stop : b.stopEnd;
      const c1 = ask.out ? b.stopEnd : b.stop;
      const a = [c0, b.tipX - xOf(c0), (ask.out ? b.range[0] : b.range[1]) - yOf(c0)];
      const z = [c1, b.tipX - xOf(c1), (ask.out ? b.range[1] : b.range[0]) - yOf(c1)];
      const leg = move(previous, solve(a, b.bend, previous));
      // the intro's scans follow one another after a breath; a reader's sets off at once
      const go = free + (first || ask.reader ? 0 : GAP);
      first = false;
      push(leg, previous, go);
      const start = leg.end;
      const reached = go + leg.time;
      pen.push({
        t0: reached - DROP * (1 - SETTLE),
        t1: reached + DROP * SETTLE,
        a: [PEN.up],
        b: [PEN.down],
        ease: easeInOut,
      });
      const touch = reached + DROP * SETTLE;
      const hit = touch + clamp((b.range[1] - b.range[0]) * SLIDE_MS, SLIDE[0], SLIDE[1]);
      const done = solve(z, b.bend, start);
      track.push({
        t0: touch,
        t1: hit,
        a: start,
        b: done,
        ease: easeInOut,
        task: { a, b: z, bend: b.bend },
      });
      const stood =
        along === 1 ? [done[0], done[1], foldAt(done[1], done[2]), 0] : [done[0], done[1], done[2], UPRIGHT];
      const away = Math.max(TIP_UP, Math.abs(stood[2] - done[2]) * BEND_RATE);
      track.push({ t0: hit, t1: hit + away, a: done, b: stood, ease: easeInOut });
      pen.push({ t0: hit, t1: hit + LIFT, a: [PEN.down], b: [PEN.up], ease: easeOut });
      jobs.push({ ask, go, touch, hit, free: hit + away });
      previous = stood;
      free = hit + away;
    }
    // the park answers no one, so it takes its time: a press can catch it on the way
    const leg = move(previous, park, 1.5, PARK);
    push(leg, previous, free + REST);
    total = free + REST + leg.time;
  }

  /** the stage on show at time t: the latest scan that has touched, else the base */
  function stageAt(t: number): number {
    let s = base;
    for (const job of jobs) if (job.touch <= t) s = job.ask.stage;
    return s;
  }

  /** a new plan from now and the arm's pose: the scans not yet touching wait their turn, the stage keeps what it
   * shows */
  function replan(t: number) {
    const waiting: Ask[] = [];
    for (let i = 0; i < jobs.length; i++) if (jobs[i].touch > t) waiting.push(jobs[i].ask);
    base = stageAt(t);
    asks.length = 0;
    for (const ask of waiting) asks.push(ask);
    since = t;
    from = [...(aiming ? aimPose : pose)];
    aiming = false;
    plan();
  }

  function wake() {
    if (phase === 'rest' || phase === 'ready') {
      phase = 'play';
      last = -1;
    }
    mesh?.invalidate();
  }

  /** a reader's chip: stage `i` comes up, by the arm when it can; "latest request wins": the intro's remaining
   * scans are dropped and the arm sets off from where it is */
  function press(i: number) {
    if (reduced || !mesh || !laidOut) {
      base = i;
      asks.length = 0;
      jobs.length = 0;
      plan();
      apply();
      return;
    }
    const t = clock;
    replan(t);
    asks.length = 0;
    playing = false;
    const scanOf = SCAN_OF[i];
    if (scanOf) asks.push({ block: blocks[scanOf[0]], out: scanOf[1], stage: i, reader: true });
    else base = i;
    plan();
    // the stage clock restarts its tail at `since`; a reset has nothing to scan, so the arm just parks
    phase = 'rest';
    wake();
  }

  function project(x: number, y: number, z: number) {
    const k = CAMERA / (CAMERA - z);
    point[0] = foot[0] + (x - foot[0]) * k;
    point[1] = foot[1] + (y - foot[1]) * k;
    point[2] = k;
  }

  /** whether the probe at (x, y) lies on a tab or on the operator */
  function blocked(x: number, y: number): boolean {
    const r = PEN_R * scale;
    for (const b of blocks) if (inBox(x, y, r, b.tabBox)) return true;
    const x0 = seat[0] - OW / 2;
    const y0 = seat[1] - OH;
    return x > x0 - r && x < x0 + OW + r && y > y0 - r && y < y0 + OH + r;
  }

  /** the pose pointing the probe at the cursor, AIM_GAP short of it, clear of the tabs */
  function aimFor(out: number[]) {
    if (along === 1) return readFor(out);
    out[3] = 0;
    cursorAt[0] = pointer[0] - view[0];
    cursorAt[1] = pointer[1] - view[1];
    const k = CAMERA / (CAMERA - lift * scale);
    const x = clamp(foot[0] + (cursorAt[0] - foot[0]) / k, inside.x, inside.x + inside.w);
    const y = clamp(foot[1] + (cursorAt[1] - foot[1]) / k, inside.y, inside.y + inside.h);
    const bendTo = below && y > rail[1] ? 1 : -1;
    const r = span(AIMED);
    const h = Math.abs(y - rail[1]);
    const reach = Math.sqrt(Math.max(0, r * r - h * h));
    out[0] = clear(clamp(x + Math.max(60 * scale, reach), rail[0], rail[2]), herOff);
    reachTo(x - out[0], y - rail[1], bendTo, out, fore + AIM_GAP);
    out[1] = boomAngle(out[1]);
    out[2] = nearest(out[2], aimPose[2]);
  }

  /** up a phone's column, the park's fold carried to the line in the viewport's middle */
  function readFor(out: number[]) {
    const k = CAMERA / (CAMERA - PEN.up * scale);
    const top = blocks[0].range[0];
    const bottom = blocks[blocks.length - 1].range[1];
    const y = foot[1] + (clamp(innerHeight / 2 - view[1], top, bottom) - foot[1]) / k;
    out[0] = clamp(y - boom + fore * Math.cos(fold), rail[1], park[0]);
    out[1] = DOWN;
    out[2] = Math.PI - fold;
    out[3] = 0;
  }

  /** how far the probe stands up for its drawn length to stop short of a tab or the operator */
  function tipFor(p: number[]): number {
    const dx = Math.cos(p[1] + p[2]);
    const dy = Math.sin(p[1] + p[2]);
    const wrist = PROBE.wrist * scale;
    const length = fore - wrist;
    for (let l = 0; l <= length; l += 4) {
      if (blocked(elbowAt[0] + dx * (wrist + l), elbowAt[1] + dy * (wrist + l)))
        return Math.acos(clamp((l - 4) / length, 0, 1));
    }
    return 0;
  }

  function aimStep(dt: number): boolean {
    if (following) aimFor(aimAt);
    lift = Math.min(PEN.up, lift + ((PEN.up - PEN.down) / LIFT) * 1000 * dt);
    const turn = boomAngle(aimAt[1]);
    const swinging = Math.abs(turn - aimPose[1]) > SWUNG;
    // up a phone's column the probe folds along the boom while it swings round; on the bed, going home, the carriage
    // waits while the elbow comes out of the plates
    targets[0] = along === 0 && !following && swinging ? aimPose[0] : aimAt[0];
    targets[1] = turn;
    targets[2] = along === 1 && swinging ? foldAt(aimPose[1], aimPose[2]) : nearest(aimAt[2], aimPose[2]);
    const stiff = along === 1 ? SCROLL_AIM : AIM;
    // up a phone's column the elbow sweeps no faster than in a move
    const slew = Math.min(AIM_SLEW, (SWING * 1000) / boom);
    let still = true;
    for (let i = 0; i < 3; i++) {
      const top = i === 0 ? AIM_SPEED : i === 1 && along === 1 ? slew : AIM_SLEW;
      aimSpeed[i] += (stiff * stiff * (targets[i] - aimPose[i]) - 2 * stiff * aimSpeed[i]) * dt;
      aimSpeed[i] = clamp(aimSpeed[i], -top, top);
      aimPose[i] += aimSpeed[i] * dt;
      if (Math.abs(targets[i] - aimPose[i]) > (i === 0 ? 1 : 0.004)) still = false;
      if (Math.abs(aimSpeed[i]) > (i === 0 ? 20 : 0.05)) still = false;
    }
    place(aimPose);
    // up a phone's column it lies folded, never stood up end-on
    const swing = along === 1 ? 0 : 0.25 * (Math.abs(aimSpeed[1]) + Math.abs(aimSpeed[2]));
    const tipped = along === 1 ? 0 : tipFor(aimPose);
    const need = Math.min(along === 1 ? STEEP : UPRIGHT, Math.max(aimAt[3], tipped, swing));
    aimSpeed[3] += (AIM_TIP * AIM_TIP * (need - aimPose[3]) - 2 * AIM_TIP * aimSpeed[3]) * dt;
    aimPose[3] = clamp(aimPose[3] + aimSpeed[3] * dt, 0, UPRIGHT);
    if (Math.abs(need - aimPose[3]) > 0.004 || Math.abs(aimSpeed[3]) > 0.05) still = false;
    return still && lift === PEN.up;
  }

  /** paints the operator's canvas when its frame or facing changed, and places it on the deck */
  function paintMate(frame: OperatorFrame, flip: boolean, hop: number) {
    const key = `${frame}${flip ? 'f' : ''}`;
    if (key !== mateShown) {
      mateShown = key;
      mateCtx.clearRect(0, 0, OW, OH);
      drawOperator(mateCtx, 0, 0, CELL, frame, matePalette, flip);
      mate.style.visibility = 'visible';
    }
    if (herAnchor) herAnchor.offset[1] = -OH - hop * CELL;
  }

  /** the operator on the plan's clock: striding while the carriage runs, at the console while the probe scans or
   * follows, a hop of acknowledgement as a scan completes, idle (beacon, blink) at rest; all off the same frame time */
  function operate(t: number, now: number) {
    const dx = Number.isNaN(prevPivot) ? 0 : pivotAt[0] - prevPivot;
    prevPivot = pivotAt[0];
    // the carriage runs while it moved this frame, and a stride keeps a beat after it stops
    if (Math.abs(dx) > 0.15) strideAt = now;
    strolling = now - strideAt < TICK * 2;
    if (Math.abs(dx) > 0.15) facing = dx < 0;
    let working = false;
    let hop = 0;
    for (const job of jobs) {
      if (t >= job.go - TICK && t < job.free) {
        const aimX = t < job.touch ? job.ask.block.tipX : tipAt[0];
        facing = aimX < seat[0];
        if (t >= job.touch && t < job.hit) {
          working = true;
          hop = Math.floor(t / (TICK * 2)) % 2 ? 0.5 : 0;
        }
      }
      // the acknowledgement: up and down once as the scan completes
      const after = t - job.hit;
      if (after >= 0 && after < TICK * 3) hop = after < TICK * 1.5 ? 1 : 0.5;
    }
    if (aiming && following) {
      working = true;
      facing = cursorAt[0] < seat[0];
    }
    const beat = Math.floor(now / TICK);
    let frame: OperatorFrame;
    if (strolling) frame = (beat >> 1) % 2 ? 'walkB' : 'walkA';
    else if (working) frame = 'idle0';
    else if (beat % 28 === 27) frame = 'blink';
    else frame = (beat >> 3) % 2 ? 'idle1' : 'idle0';
    // a stride bobs the operator a half cell on the beat
    if (strolling && beat % 2) hop = Math.max(hop, 0.5);
    paintMate(frame, facing, reduced ? 0 : hop);
  }

  function apply() {
    const t = clock;
    const now = performance.now();
    let lineY = 0;
    let z: number = PEN.up;
    if (laidOut) {
      if (aiming) for (let i = 0; i < 4; i++) pose[i] = aimPose[i];
      else sample(track, t, from, pose);
      sample(pen, t, up, sole);
      z = aiming ? lift : sole[0];
      place(pose);
      // the line the probe's point drags, on whole canvas pixels
      project(tipAt[0], tipAt[1], z * scale);
      lineY = mesh ? mesh.snap(point[1]) : point[1];
    }

    // the stage on show: a function of the clock
    const st = begun || jobs.length ? stageAt(t) : base;
    if (st !== shown) {
      shown = st;
      onStage(st);
    }

    let gauge = GAUGE[0];
    let sliding: Block | null = null;
    for (const job of jobs) {
      if (t >= job.touch && t < job.hit) {
        sliding = job.ask.block;
        gauge = GAUGE[1];
      } else if (t >= job.hit && t < job.free) gauge = Math.max(gauge, GAUGE[2]);
    }
    if (!laidOut) return;

    const bx = pivotAt[0];
    const by = pivotAt[1];
    project(bx + deck.seat[0] * scale, by + deck.seat[1] * scale, deck.seat[2] * scale);
    seat[0] = point[0];
    seat[1] = point[1];
    seat[2] = deck.seat[2] * scale;
    if (along === 0) chain(bx - band.x);
    operate(t, now);

    if (!mesh) return;
    const k = (CAMERA - seat[2]) / CAMERA;
    transform(her, [bx + deck.seat[0] * scale, by + deck.seat[1] * scale, seat[2]], undefined, [k, k, k]);
    posed.turn = pose[1];
    posed.dir[0] = Math.cos(pose[1] + pose[2]);
    posed.dir[1] = Math.sin(pose[1] + pose[2]);
    posed.pen = z;
    posed.pitch = pose[3];
    posed.gauge = gauge;
    transform(root, [bx, by, 0], undefined, [scale, scale, scale]);
    poseRig(deck, posed, frames, rig);
    for (let i = 0; i < FRAMES; i++) multiply(placed[i], root, frames[i]);
    bounds[0] = bounds[1] = Infinity;
    bounds[2] = bounds[3] = -Infinity;
    for (let i = 0; i < rig.length; i++) lay(i, multiply(m, placed[rig[i].frame], rig[i].local), rig[i].radius * scale);
    for (let i = 0; i < HER.length; i++) lay(proxy + i, multiply(m, her, HER[i].m), 30);
    // the operator's canvas stands inside what the mesh draws
    bounds[0] = Math.min(bounds[0], seat[0] - OW);
    bounds[1] = Math.min(bounds[1], seat[1] - OH - CELL * 2);
    bounds[2] = Math.max(bounds[2], seat[0] + OW);
    bounds[3] = Math.max(bounds[3], seat[1] + CELL);
    // the mesh draws only where the rig and its shadow are, and were
    const x0 = Math.floor(bounds[0]) - 2;
    const y0 = Math.floor(bounds[1]) - 2;
    const w = Math.ceil(bounds[2]) + 2 - x0;
    const h = Math.ceil(bounds[3]) + 2 - y0;
    if (x0 !== clip.x || y0 !== clip.y || w !== clip.w || h !== clip.h) {
      clip.x = x0;
      clip.y = y0;
      clip.w = w;
      clip.h = h;
      mesh.clip(clip);
    }
    // the line the probe's point drags across its region
    if (sliding !== scanned) {
      scanned = sliding;
      if (sliding) scan.style.width = `${sliding.cell.w}px`;
    }
    scanAnchor!.shown = sliding !== null;
    scanAnchor!.at[0] = sliding?.cell.x ?? 0;
    scanAnchor!.at[1] = lineY;
  }

  /** places part i, and grows the bounds by what it covers, `r` round its middle, and its shadow */
  function lay(i: number, w: Mat4, r: number) {
    mesh!.place(i, w);
    project(w[12], w[13], w[14]);
    const shadowX = w[12] - (LIGHT[0] / LIGHT[2]) * w[14];
    const shadowY = w[13] - (LIGHT[1] / LIGHT[2]) * w[14];
    const k = r * point[2];
    bounds[0] = Math.min(bounds[0], point[0] - k, shadowX - r);
    bounds[1] = Math.min(bounds[1], point[1] - k, shadowY - r);
    bounds[2] = Math.max(bounds[2], point[0] + k, shadowX + r);
    bounds[3] = Math.max(bounds[3], point[1] + k, shadowY + r);
  }

  /** the drag chain's bend rolls half the carriage's way from home: both runs show from it, the pulled one up to the
   * carriage */
  function chain(c: number) {
    const at = Math.round(c);
    if (at === chainAt) return;
    chainAt = at;
    const b = Math.round((at + BED.start) / 2);
    runs.style.transform = `translateX(${b}px)`;
    fixedRun.style.transform = `translateX(${-b}px)`;
    pulledRun.style.transform = `translateX(${at - b}px) translateX(-100%)`;
    bend.style.transform = `translateX(${b - BED.bend}px)`;
  }

  function refresh() {
    const was = along;
    layout();
    // a resize inside the stage leaves the arm in the reader's hands, its springs aimed anew; only a move between the
    // bed and the column puts it back on the plan
    if (along !== was) {
      aiming = false;
      following = false;
    } else if (aiming && !following) for (let i = 0; i < 4; i++) aimAt[i] = park[i];
    apply();
    mesh?.invalidate();
  }

  function onFrame(dtMs: number) {
    const dt = last < 0 ? 1000 / 60 : Math.min(48, dtMs || 1000 / 60);
    last = 0;
    if (following) {
      const s = stage.getBoundingClientRect();
      view[0] = s.left;
      view[1] = s.top;
    }
    if (phase === 'fade') {
      fading += dt;
      // the cover dithers over the regions, then the stage is cleared under it
      if (fading >= FADE) {
        covering?.cancel();
        cover.style.visibility = '';
        phase = 'play';
        clock = 0;
        apply();
      }
      mesh!.invalidate();
      return;
    }
    // the clock runs on under the reader's springs, so a scan plays out as in the plan
    if (phase === 'play') {
      clock = Math.min(total, clock + dt);
      if (clock >= total) {
        phase = 'rest';
        playing = false;
      }
    }
    if (following && !aiming && free()) take();
    let still = false;
    if (aiming) {
      still = aimStep(dt / 1000);
      steady = still;
      if (still && !following) {
        // home; stopped short of it, it drives the rest of the way
        const short = Math.abs(aimAt[0] - aimPose[0]) >= 1;
        replan(clock);
        if (short) wake();
      }
    }
    apply();
    const going = (aiming && !still) || phase === 'play' || strolling;
    steering(going);
    if (going) mesh!.invalidate();
    else last = -1;
  }

  /** at rest, what the frames moved leaves its layer, and the page's layers are fewer */
  function steering(on: boolean) {
    if (on === steered.length > 0) return;
    for (const release of steered) release();
    steered = [];
    if (on) for (const el of moved) steered.push(steer(el));
  }

  /** the introduction: each region scanned in turn, then the park; the stage shows what its scans bring up */
  function write() {
    begun = true;
    playing = true;
    base = 0;
    asks.length = 0;
    for (const [region, st, out] of INTRO) asks.push({ block: blocks[region], out, stage: st, reader: false });
    since = LEAD_IN;
    plan();
  }

  /** dither the regions out, then play the introduction again from the arm's pose */
  function replay() {
    if (reduced || !mesh) {
      base = FINAL_STAGE;
      shown = -1;
      apply();
      return;
    }
    from = aiming ? [...aimPose] : begun ? [...pose] : park;
    aiming = false;
    following = false;
    write();
    // the plan starts at once, from the pose the arm is in
    since = LEAD_IN;
    plan();
    phase = 'fade';
    fading = 0;
    clock = 0;
    last = -1;
    cover.style.visibility = 'visible';
    covering?.cancel();
    covering = cover.animate(ditherKeyframes(2), { duration: FADE, fill: 'forwards' });
    mesh.invalidate();
  }

  /** the introduction planned, before the stage comes into view */
  function ready() {
    from = park;
    write();
    phase = 'ready';
    clock = 0;
    apply();
  }

  /** the introduction starts: from the planned, parked arm */
  function begin() {
    if (reduced || !mesh || !laidOut) return;
    if (phase === 'ready') wake();
  }

  function free(): boolean {
    return begun && !playing && (!jobs.length || clock >= jobs[jobs.length - 1].free);
  }

  /** the rig leaves its plan for the reader's springs, from where and as fast as the plan has it */
  function take() {
    sample(track, clock, from, aimPose);
    sample(pen, clock, up, sole);
    lift = sole[0];
    const before = [0, 0, 0, 0];
    sample(track, clock - 16, from, before);
    aimSpeed[0] = (aimPose[0] - before[0]) / 0.016;
    for (let i = 1; i < 4; i++) aimSpeed[i] = (aimPose[i] - nearest(before[i], aimPose[i])) / 0.016;
    aiming = true;
    steady = false;
  }

  /** after the intro the arm follows the reader: the pointer on the bed, the scroll up a phone's column */
  function onScroll() {
    if (!begun || playing || reduced || !mesh) return;
    following = true;
    mesh.invalidate();
  }

  function onMove(event: PointerEvent) {
    if (event.pointerType !== 'mouse') return;
    pointer[0] = event.clientX;
    pointer[1] = event.clientY;
    if (along === 0) onScroll();
  }

  function onLeave(event: PointerEvent) {
    if (event.pointerType !== 'mouse' || !following) return;
    following = false;
    for (let i = 0; i < 4; i++) aimAt[i] = park[i];
    mesh?.invalidate();
  }

  /** the finished stage with no rig: everything recorded and verified */
  function stillStage() {
    begun = false;
    playing = false;
    asks.length = 0;
    jobs.length = 0;
    phase = 'rest';
    aiming = false;
    following = false;
    base = FINAL_STAGE;
    shown = -1;
    apply();
  }

  /** the arm's canvas and context, made as the stage first comes on screen and kept */
  function show() {
    made = true;
    canvas = document.createElement('canvas');
    canvas.className = 'pointer-events-none absolute inset-0 z-2 size-full';
    canvas.setAttribute('aria-hidden', 'true');
    stage.append(canvas);
    const parts: PartSpec[] = [];
    rig = build(deck, posed);
    // shadows fall on the regions only, never across the rig
    for (const { shape, material, id } of rig) parts.push({ shape, material, id, shadow: true, receive: false });
    proxy = parts.length;
    for (const { shape } of HER) parts.push({ shape, material: MATERIAL.clear, id: 0, shadow: true, receive: false });
    floor = parts.length;
    parts.push({ shape: 'box', material: MATERIAL.floor, id: 0, shadow: false });
    mesh = createMesh(canvas, {
      parts,
      shader: { scene: shading },
      onFrame,
      onResize(width, height) {
        size[0] = width;
        size[1] = height;
        refresh();
        if (seen) begin();
      },
      // the rig is painted in the new colours at once; its accent is the page's sun, yellow at night
      theme(page) {
        matePalette = readPalette();
        mateShown = '';
        return { ...housePalette(page), accent: page.night ? page.yellow : page.sun };
      },
    });
    if (!mesh) {
      canvas.remove();
      canvas = null;
      // no WebGL2: the stage shows its finished state, and a chip brings its stage up at once
      stillStage();
      mate.remove();
      return;
    }
    // a lost context (GPU reset, too many contexts) leaves the finished stage and the text, never a frozen rig; if the
    // browser restores it, the rig is built again on the spot, at rest
    const cv = canvas;
    cv.addEventListener(
      'webglcontextlost',
      (event) => {
        event.preventDefault();
        mesh?.destroy();
        mesh = null;
        stillStage();
        cv.style.visibility = 'hidden';
        mate.style.visibility = 'hidden';
      },
      { signal },
    );
    cv.addEventListener(
      'webglcontextrestored',
      () => {
        cv.remove();
        canvas = null;
        made = false;
        if (signal.aborted) return;
        if (visible) show();
        if (mesh) {
          mate.style.visibility = '';
          laidOut = false;
          refresh();
        }
      },
      { signal },
    );
    for (let i = 0; i < rig.length; i++) if (rig[i].point) mesh.taper(i, 0, 0);
    herAnchor = mesh.anchor(mate, her);
    herAnchor.offset[0] = -OW / 2;
    herAnchor.offset[1] = -OH;
    herAnchor.shown = true;
    scanAnchor = mesh.anchor(scan);
  }

  const listeners = new AbortController();
  const { signal } = listeners;
  const passive = { passive: true, signal };

  /** the scroll moves a phone's arm only while its stage shows */
  function listen() {
    if (visible && along === 1) addEventListener('scroll', onScroll, passive);
    else removeEventListener('scroll', onScroll);
  }

  stage.addEventListener('pointermove', onMove, passive);
  stage.addEventListener('pointerleave', onLeave, passive);

  // the arm can draw once the stage is near; the intro starts when it crosses the middle of the viewport
  const near = new IntersectionObserver((entries) => {
    visible = entries[entries.length - 1].isIntersecting;
    listen();
    if (visible && !made) show();
  });
  near.observe(stage);
  const middle = new IntersectionObserver(
    (entries) => {
      if (!entries[entries.length - 1].isIntersecting) return;
      seen = true;
      begin();
    },
    { rootMargin: '-35% 0px -35% 0px' },
  );
  middle.observe(stage);
  // a region that changes size (fonts, a resize) re-lays the stations
  let relayout = 0;
  const watch = new ResizeObserver(() => {
    cancelAnimationFrame(relayout);
    relayout = requestAnimationFrame(() => {
      if (mesh) refresh();
    });
  });
  watch.observe(grid);
  if (document.fonts.status === 'loading')
    void document.fonts.ready.then(() => {
      if (!signal.aborted && mesh) refresh();
    });

  if (reduced) {
    // a composed, still stage: everything recorded and verified, the arm parked
    base = FINAL_STAGE;
    shown = -1;
    from = park;
  } else ready();
  apply();

  return {
    press,
    replay,
    destroy() {
      listeners.abort();
      removeEventListener('scroll', onScroll);
      near.disconnect();
      middle.disconnect();
      watch.disconnect();
      cancelAnimationFrame(relayout);
      steering(false);
      covering?.cancel();
      mesh?.destroy();
      // a remount makes its own canvas, so this one's context goes now
      canvas?.getContext('webgl2')?.getExtension('WEBGL_lose_context')?.loseContext();
      canvas?.remove();
      mate.remove();
    },
  };
}
