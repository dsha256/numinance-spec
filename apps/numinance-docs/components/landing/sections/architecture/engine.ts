/**
 * Portions adapted from Fumadocs (apps/docs/app/(landing)/sections/parts/engine.ts, lib/cable.ts)
 * https://github.com/fuma-nama/fumadocs
 * MIT License, Copyright (c) 2022 Fuma Nama. See ../workflow/rig/LICENSE-fumadocs.txt.
 *
 * Adapted: the mesh-driven module stack with anchored DOM plates and displays, the CRT open and swap
 * blink, the inverse-kinematics arm, picking by unprojection and the chamfered cable. Written for
 * Numinance: one spring-followed pose with velocity continuity, a latest-wins request queue, fixed
 * modules the arm may only touch, cartridge fetch and dock, and pointer-driven inspection.
 */
import { ditherKeyframes } from '../workflow/rig/dither';
import {
  between,
  createMesh,
  identity,
  type Mat4,
  MATERIAL,
  multiply,
  type PartSpec,
  transform,
  type Vec3,
} from '../workflow/rig/mesh';
import { clamp, easeInOut, mix, TICK } from '../workflow/rig/util';
import {
  BASE,
  BOOM,
  BOOM_PIECES,
  BOUNDS,
  CARTRIDGE_PIECES,
  D,
  DEPTH,
  DISPLAYS,
  FLOOR,
  FOREARM,
  FOREARM_PIECES,
  GRIP,
  HELD,
  HOME,
  LIGHT,
  type Piece,
  PLACES,
  PLATE,
  RAM_FOOT,
  RAM_HEAD,
  RAM_PIECES,
  reach,
  REST,
  SHOULDER,
  SLOT_OF,
  STACK,
  STACKED,
  TOOL_PIECES,
  VIEWS,
  W,
} from './geometry';
import {
  cableOf,
  type Flow,
  MODULES,
  type ModuleId,
  ORDER,
  type Pick,
  readoutOf,
  type Selection,
} from './model';

type Point = [number, number];

export interface EngineCallbacks {
  /** the figure itself asked for this (a click on the machine, a key on a plate, the intro's demo): mirror it in the panel */
  sync: (pick: Pick) => void;
  status: (text: string) => void;
  /** the monitor shows this many lines of a readout at most, and whether every readout fits whole */
  layout: (fits: boolean) => void;
  /** the context was lost or cannot be made */
  fail: () => void;
}

export interface Engine {
  request: (pick: Pick) => void;
  destroy: () => void;
}

export interface EngineOptions {
  reduced: boolean;
  selection: Selection;
  focus: Pick;
}

const CLEAR = 34;
const HOVER = 24;
const TIMING = {
  crt: { line: TICK, dot: TICK / 2, hold: TICK / 2 },
  jolt: 2 * TICK,
  lead: 4 * TICK,
  /** a readout line each */
  line: 1.1 * TICK,
};
const CRT_MS = 2 * (TIMING.crt.line + TIMING.crt.dot) + TIMING.crt.hold;
/** the visible change happens a dot into the blink, half its hold gone */
const CHANGE = TIMING.crt.line + TIMING.crt.dot + TIMING.crt.hold / 2;
const FULL: Point = [1, 1];
const LINE = 16;
/** each cable's jog off halfway, so lifted lanes never cross */
const LANE = 11;
const SVG = 'http://www.w3.org/2000/svg';
/** the arm's pad rests this far from a module's face while it reads or inserts */
const FACE = 2;

interface Step {
  to?: Point;
  tol?: number;
  vmax?: number;
  dwell?: number;
  start?: () => void;
  arrive?: () => void;
}

interface Job {
  kind: 'build' | 'swap' | 'touch';
  mod: ModuleId;
  pick?: Pick;
  steps: Step[];
  i: number;
  started: boolean;
  arrived: boolean;
  until: number;
  /** its physical event has happened: the cartridge is in, or the face was touched */
  committed: boolean;
  gripped: boolean;
}

type Request = { kind: 'build'; mod: ModuleId } | { kind: 'pick'; pick: Pick; demo?: boolean };

interface Leader {
  g: SVGGElement | null;
  outer: SVGPathElement;
  core: SVGPathElement;
  arrow: SVGPolygonElement;
  a: SVGGElement;
  b: SVGGElement;
  /** NaN till its module lands, -Infinity for one drawn whole */
  at: number;
  len: number;
  look: 'solid' | 'dashed';
  route: string;
}

interface Slot {
  id: ModuleId;
  parts: number[];
  locals: Float32Array[];
  pieces: Piece[];
  matrix: Mat4;
  scale: Point;
  plate: HTMLElement;
  plateY: number;
  name: HTMLElement;
  row: HTMLElement;
  rowName: HTMLElement;
  state: 'off' | 'held' | 'placed';
  crt: { at: number; kind: 'open' | 'swap' } | null;
  /** what the module wears: the selection committed, per group */
  have: number[];
  /** the group last changed or inspected */
  last: number;
  leader: Leader;
  /** a signature of everything its parts were last posed on, to skip the pass when nothing moved */
  sig: string;
  /** the row's port on the panel, in holder px */
  end: Point;
}

const el = <T extends Element>(root: ParentNode, selector: string): T => root.querySelector<T>(selector)!;

export function createArchitecture(root: HTMLElement, cb: EngineCallbacks, opts: EngineOptions): Engine | null {
  const found = root.querySelector<HTMLElement>('[data-holder]');
  const panel = root.querySelector<HTMLElement>('[data-window]');
  const cables = root.querySelector<SVGSVGElement>('[data-cables]');
  if (!found || !panel || !cables) return null;
  const holder: HTMLElement = found;
  // the figure's own canvas, made here and removed with it: a lost or released context never outlives its element
  const canvas = document.createElement('canvas');
  canvas.setAttribute('aria-hidden', 'true');
  canvas.className = 'absolute inset-0 size-full';
  holder.prepend(canvas);

  const specs: PartSpec[] = [];
  let ids = 0;
  // only what rests on the floor casts a shadow: never the arm, a module once it's down
  function add(pieces: Piece[], shadow = false): number[] {
    const out: number[] = [];
    const body = ++ids;
    for (const piece of pieces) {
      out.push(specs.length);
      specs.push({ shape: piece.shape, id: piece.solid ? ++ids : body, material: piece.material, shadow });
    }
    return out;
  }
  const local = (piece: Piece): Float32Array => transform(identity(), piece.at, piece.turn, piece.size);
  const floor = specs.length;
  specs.push({ shape: FLOOR.shape, id: 0, material: FLOOR.material, shadow: false });
  const base = add(BASE, true);
  const boom = add(BOOM_PIECES);
  const forearm = add(FOREARM_PIECES);
  const tool = add(TOOL_PIECES);
  const ram = add(RAM_PIECES);
  const cartridge = add(CARTRIDGE_PIECES);
  const links = {
    boom: BOOM_PIECES.map(local),
    forearm: FOREARM_PIECES.map(local),
    tool: TOOL_PIECES.map(local),
    cartridge: CARTRIDGE_PIECES.map(local),
  };

  const slots: Slot[] = [];
  for (const module of STACKED) {
    const id = module.id;
    const plate = el<HTMLElement>(root, `[data-plate="${id}"]`);
    const row = el<HTMLElement>(panel, `[data-row="${id}"]`);
    const svgG = document.createElementNS(SVG, 'g');
    slots.push({
      id,
      parts: add(module.pieces),
      locals: module.pieces.map(local),
      pieces: module.pieces,
      matrix: identity(),
      scale: [1, 1],
      plate,
      plateY: module.pieces.find((piece) => piece.plate)!.at[1],
      name: el<HTMLElement>(plate, '[data-name]'),
      row,
      rowName: el<HTMLElement>(row, '[data-rowname]'),
      state: 'off',
      crt: null,
      have: opts.selection[id].slice(),
      last: 0,
      leader: makeLeader(svgG),
      sig: '',
      end: [0, 0],
    });
  }
  const slotById = (id: ModuleId): Slot => slots[SLOT_OF[id]];
  const monitor = el<HTMLElement>(root, '[data-display="engine"]');
  const monitorTitle = el<HTMLElement>(monitor, '[data-title]');
  const monitorLines = [...monitor.querySelectorAll<HTMLElement>('[data-line]')];
  const listing = el<HTMLElement>(root, '[data-display="kernel"]');
  const listed = [...listing.querySelectorAll<HTMLElement>('[data-line]')];

  function makeLeader(g: SVGGElement): Leader {
    const path = (cls: string, width: number) => {
      const p = document.createElementNS(SVG, 'path');
      p.setAttribute('class', cls);
      p.setAttribute('fill', 'none');
      p.setAttribute('stroke-width', String(width));
      p.setAttribute('stroke-linejoin', 'miter');
      p.setAttribute('shape-rendering', 'crispEdges');
      return p;
    };
    const port = () => {
      const p = document.createElementNS(SVG, 'g');
      p.setAttribute('shape-rendering', 'crispEdges');
      p.innerHTML =
        '<rect x="-6" y="-6" width="12" height="12" class="fill-lp-paper stroke-lp-ink" stroke-width="1"/><rect x="-3" y="-3" width="6" height="6" class="fill-lp-ink"/>';
      return p;
    };
    const outer = path('stroke-lp-ink', 4);
    const core = path('stroke-lp-paper', 2);
    const arrow = document.createElementNS(SVG, 'polygon');
    arrow.setAttribute('class', 'fill-lp-ink');
    const a = port();
    const b = port();
    g.append(outer, core, arrow, a, b);
    g.setAttribute('visibility', 'hidden');
    cables!.append(g);
    return { g, outer, core, arrow, a, b, at: Number.NaN, len: 0, look: 'solid', route: '' };
  }

  const mesh = createMesh(canvas, {
    parts: specs,
    onFrame: frame,
    onResize: layout,
    onRest: () => undefined,
  });
  if (!mesh) {
    cables.replaceChildren();
    canvas.remove();
    return null;
  }

  // a lost context cannot be drawn on: the page keeps its panel and a static figure
  const onLost = (event: Event) => {
    event.preventDefault();
    cb.fail();
  };
  canvas.addEventListener('webglcontextlost', onLost);

  const plates = slots.map((slot) =>
    mesh.anchor(slot.plate, slot.matrix, [PLATE.x - PLATE.w / 2, slot.plateY + PLATE.h / 2, PLATE.z]),
  );
  const screenAt = (which: 'engine' | 'kernel', target: HTMLElement) => {
    const { at, size } = DISPLAYS[which];
    const corner: Vec3 = [at[0] - size[0] / 2, at[1] + size[1] / 2, at[2]];
    return mesh.anchor(target, slotById(which).matrix, corner);
  };
  const monitorAnchor = screenAt('engine', monitor);
  const listingAnchor = screenAt('kernel', listing);

  mesh.place(floor, local(FLOOR));
  for (let i = 0; i < BASE.length; i++) mesh.place(base[i], local(BASE[i]));
  mesh.light(LIGHT, BOUNDS[0], BOUNDS[1]);

  // ---------------------------------------------------------------- layout (resize only)
  const whole = matchMedia('(width >= 48rem)');
  const beside = matchMedia('(width >= 64rem)');
  let scale = 1;
  let maxLines = 6;
  let started = false;
  let width = 0;

  function layout(w: number, h: number) {
    width = w;
    const view = whole.matches ? VIEWS.wide : VIEWS.phone;
    scale = w / (view.right - view.left);
    mesh!.camera({
      width: w,
      height: h,
      scale,
      origin: [-view.left * scale, h + view.bottom * scale],
      depth: DEPTH,
      z: [400, -400],
    });
    fit();
    mesh!.invalidate();
  }

  function fit() {
    const w = Math.round(PLATE.w * scale);
    const h = Math.round(PLATE.h * scale);
    const inner = h - 2;
    const badge = h < 20 ? 0 : inner;
    const icon = inner >= 36 ? 16 : 8;
    const two = h >= 34;
    const space = h < 24 ? 6 : 10;
    const style = root.style;
    style.setProperty('--arch-plate-w', `${w}px`);
    style.setProperty('--arch-plate-h', `${h}px`);
    style.setProperty('--arch-icon', `${icon}px`);
    style.setProperty('--arch-icon-at', `${Math.max(0, Math.floor((inner - icon) / 2))}px`);
    style.setProperty('--arch-pad', `${space}px`);
    // Geist Pixel at 13px in a 16px line: its caps from 2.5px down, its baseline at 12px
    const inset = Math.min(8, (inner - 2 * (12 - 2.5) - 3) / 2);
    const role = Math.max(0, Math.floor(inset - 2.5));
    const name = two ? Math.max(0, Math.round(inner - inset - 12)) : Math.max(0, Math.round((inner - 12 - 2.5) / 2));
    style.setProperty('--arch-text-at', `${two ? role : name}px`);
    style.setProperty('--arch-name-at', `${two ? name - role - LINE : 0}px`);
    for (const slot of slots) {
      el<HTMLElement>(slot.plate, '[data-badge]').hidden = badge === 0;
      el<HTMLElement>(slot.plate, '[data-role]').hidden = !two;
    }
    for (const which of ['engine', 'kernel'] as const) {
      const [dw, dh] = DISPLAYS[which].size;
      const target = which === 'engine' ? monitor : listing;
      target.style.width = `${Math.round(dw * scale)}px`;
      target.style.height = `${Math.round(dh * scale)}px`;
    }
    const room = Math.floor((DISPLAYS.engine.size[1] * scale - 2) / LINE);
    maxLines = Math.max(0, Math.min(7, room) - 1);
    const kernelRows = Math.floor((DISPLAYS.kernel.size[1] * scale - 2) / LINE);
    listed.forEach((line, i) => (line.hidden = i >= kernelRows));
    cb.layout(maxLines >= 6);
    drawn.length = 0;
    measureRows();
    for (const slot of slots) slot.leader.route = '';
    cableDirty = true;
  }

  /** the panel's rows, in the holder's px: measured here only, never in a frame */
  function measureRows() {
    if (!beside.matches) return;
    const box = holder.getBoundingClientRect();
    const panelBox = panel!.getBoundingClientRect();
    for (const slot of slots) {
      const r = slot.rowName.getBoundingClientRect();
      slot.end = [Math.round(panelBox.left - box.left), Math.round(r.top - box.top + r.height / 2)];
    }
  }

  const refit = new ResizeObserver(() => {
    if (width > 0) fit();
    mesh!.invalidate();
  });
  refit.observe(panel);
  void document.fonts.ready.then(() => {
    if (width > 0) fit();
    mesh!.invalidate();
  });

  // ---------------------------------------------------------------- the one pose
  let clock = 0;
  /** the pad's pose: position and velocity, followed to a carrot that slews at a bounded speed to the goal */
  const arm = { x: HOME[0], y: HOME[1], vx: 0, vy: 0, cx: HOME[0], cy: HOME[1], gx: HOME[0], gy: HOME[1], vmax: 600, tol: 2 };
  let armBusy = false;
  const angles: Point = [0, 0];
  const pad: Point = [0, 0];
  const linkM = identity();
  const partM = identity();
  const slotM = identity();

  function setGoal(to: Point, vmax = 700, tol = 2) {
    arm.gx = to[0];
    arm.gy = to[1];
    arm.vmax = vmax;
    arm.tol = tol;
  }

  /** critically damped spring on the pad, the carrot ahead of it limited in speed: a start and a stop that ease,
   * and a retarget that keeps both position and velocity */
  function follow(dt: number) {
    const dx = arm.gx - arm.cx;
    const dy = arm.gy - arm.cy;
    const dist = Math.hypot(dx, dy);
    const step = arm.vmax * dt;
    if (dist <= step) {
      arm.cx = arm.gx;
      arm.cy = arm.gy;
    } else {
      arm.cx += (dx / dist) * step;
      arm.cy += (dy / dist) * step;
    }
    const w = 13;
    const n = Math.max(1, Math.ceil(dt / 0.008));
    const h = dt / n;
    for (let i = 0; i < n; i++) {
      arm.vx += (w * w * (arm.cx - arm.x) - 2 * w * arm.vx) * h;
      arm.vy += (w * w * (arm.cy - arm.y) - 2 * w * arm.vy) * h;
      arm.x += arm.vx * h;
      arm.y += arm.vy * h;
    }
    armBusy = Math.hypot(arm.gx - arm.x, arm.gy - arm.y) > 0.25 || Math.hypot(arm.vx, arm.vy) > 1 || arm.cx !== arm.gx || arm.cy !== arm.gy;
  }

  const arrived = () =>
    arm.cx === arm.gx &&
    arm.cy === arm.gy &&
    Math.hypot(arm.gx - arm.x, arm.gy - arm.y) <= arm.tol &&
    Math.hypot(arm.vx, arm.vy) < (arm.tol >= 6 ? 60 : 8);

  // ---------------------------------------------------------------- state
  const want: Selection = JSON.parse(JSON.stringify(opts.selection));
  let shown: Pick = opts.focus;
  /** the readout being typed: when it began */
  let typeAt = -Infinity;
  const drawn: string[] = [];
  const queue: Request[] = [];
  let job: Job | null = null;
  let pending: { at: number; pick: Pick } | null = null;
  let intro = false;
  /** the visitor has chosen something: the intro's demo no longer overrides it */
  let touched = false;
  let aim: Point | null = null;
  let aimMod = -1;
  let jolt = { at: -1e9, depth: 0 };
  let cableDirty = true;
  let lit = -1;
  const cart = { state: 'shelf' as 'shelf' | 'held' | 'docked' };

  const nextOption = (mod: ModuleId, group: number, from: number) => (from + 1) % MODULES[mod].groups[group].options.length;

  function report(text: string) {
    cb.status(text);
  }

  // ---------------------------------------------------------------- requests: latest wins, none dropped
  function request(pick: Pick) {
    if (intro) skipIntro();
    const { mod, group, option } = pick;
    want[mod][group] = option;
    touched = true;
    if (opts.reduced || !started) {
      applyNow(pick);
      return;
    }
    // the same port being retargeted while its job has not happened yet: stop that job safely
    if (job && !job.committed && job.pick && job.pick.mod === mod && job.pick.group === group) abort();
    else if (job && !job.committed && job.kind === 'touch' && job.pick) abort();
    const key = (r: Request) => (r.kind === 'pick' ? `${r.pick.mod}:${r.pick.group}` : '');
    const at = queue.findIndex((r) => key(r) === `${mod}:${group}`);
    if (at >= 0) queue.splice(at, 1);
    queue.push({ kind: 'pick', pick });
    mesh!.invalidate();
  }

  /** a change with no travel: reduced motion, or a figure not yet shown */
  function applyNow(pick: Pick) {
    const slot = slotById(pick.mod);
    slot.have[pick.group] = pick.option;
    slot.last = pick.group;
    shown = pick;
    typeAt = -Infinity;
    cableDirty = true;
    dress(slot);
    mesh!.invalidate();
  }

  /** commit a pick: the module wears it, the monitor reads it out, the cable takes its state */
  function commit(pick: Pick) {
    const slot = slotById(pick.mod);
    slot.have[pick.group] = pick.option;
    slot.last = pick.group;
    shown = pick;
    typeAt = clock;
    cableDirty = true;
    dress(slot);
    const o = MODULES[pick.mod].groups[pick.group].options[pick.option];
    report(`${MODULES[pick.mod].name}: ${o.title}`);
  }

  function dress(slot: Slot) {
    const name = MODULES[slot.id].plate!(slot.have, slot.last);
    setText(slot.name, name);
    slot.plate.setAttribute('aria-label', `${MODULES[slot.id].name}: ${name}`);
    slot.sig = '';
  }

  function abort() {
    const j = job;
    if (!j) return;
    if (j.kind === 'build') return;
    if (j.kind === 'touch') {
      // an inspection that never reached the face still lands its state
      if (j.pick) commit(j.pick);
      job = null;
      return;
    }
    if (!j.gripped) {
      job = null;
      return;
    }
    // carrying a cartridge for the old choice: return it to the plinth, then take the next
    j.steps = [
      { to: [GRIP[0] - 4, GRIP[1] + 40], tol: 8, vmax: 800 },
      { to: GRIP, tol: 2, vmax: 260, arrive: () => (cart.state = 'shelf') },
      { to: [GRIP[0] - 40, GRIP[1]], tol: 10 },
    ];
    j.i = 0;
    j.started = false;
    j.arrived = false;
    j.pick = undefined;
    j.committed = true;
  }

  function pump() {
    while (!job && queue.length > 0) {
      const r = queue.shift()!;
      if (r.kind === 'build') {
        job = buildJob(r.mod);
        break;
      }
      const { mod, group, option } = r.pick;
      if (r.demo) {
        want[mod][group] = option;
        cb.sync(r.pick);
      }
      const slot = slotById(mod);
      const same = slot.have[group] === option;
      if (same && shown.mod === mod && shown.group === group && shown.option === option) continue;
      job = MODULES[mod].swap && !same ? swapJob(r.pick) : touchJob(r.pick);
    }
  }

  const blank = (kind: Job['kind'], mod: ModuleId, steps: Step[], pick?: Pick): Job => ({
    kind,
    mod,
    pick,
    steps,
    i: 0,
    started: false,
    arrived: false,
    until: 0,
    committed: false,
    gripped: false,
  });

  function buildJob(mod: ModuleId): Job {
    const slot = slotById(mod);
    const i = SLOT_OF[mod];
    const y = PLACES[i];
    const j = blank('build', mod, []);
    j.steps = [
      {
        to: REST,
        tol: 40,
        vmax: 900,
        arrive: () => {
          slot.state = 'held';
          slot.crt = { at: clock, kind: 'open' };
          report(`building ${MODULES[mod].name}`);
        },
      },
      { to: [-W / 2, y + HOVER], tol: 16, vmax: 820 },
      {
        to: [-W / 2, y],
        tol: 3,
        vmax: 520,
        arrive: () => {
          slot.state = 'placed';
          jolt = { at: clock, depth: i };
          slot.leader.at = clock;
          cableDirty = true;
          j.committed = true;
          reveal(slot);
        },
      },
      // straight back to fetch the next: the return and the next carry overlap
      { to: REST, tol: 40, vmax: 900 },
    ];
    return j;
  }

  function yOf(mod: ModuleId, group: number): number {
    const i = SLOT_OF[mod];
    return PLACES[i] + (mod === 'adapters' ? (1 - group) * 16 : 0);
  }

  function swapJob(pick: Pick): Job {
    const y = yOf(pick.mod, pick.group);
    const j = blank('swap', pick.mod, [], pick);
    const slot = slotById(pick.mod);
    j.steps = [
      { to: [GRIP[0] - 44, GRIP[1] + 6], tol: 10, vmax: 900 },
      {
        to: GRIP,
        tol: 2,
        vmax: 280,
        arrive: () => {
          cart.state = 'held';
          j.gripped = true;
        },
        dwell: TICK,
      },
      { to: [GRIP[0], GRIP[1] + 44], tol: 8, vmax: 560 },
      { to: [-W / 2 - 30 - CLEAR, y], tol: 6, vmax: 880 },
      {
        to: [-W / 2 - 30, y],
        tol: 1.5,
        vmax: 260,
        arrive: () => {
          cart.state = 'docked';
          j.gripped = false;
          j.committed = true;
          slot.crt = { at: clock, kind: 'swap' };
          pending = { at: clock + CHANGE, pick };
        },
        dwell: CRT_MS,
      },
      {
        to: [-W / 2 - 30 - CLEAR, y],
        tol: 14,
        vmax: 700,
        start: () => {
          cart.state = 'shelf';
        },
      },
    ];
    return j;
  }

  function touchJob(pick: Pick): Job {
    const y = PLACES[SLOT_OF[pick.mod]];
    const j = blank('touch', pick.mod, [], pick);
    j.steps = [
      { to: [-W / 2 - 30 - CLEAR, y], tol: 10, vmax: 900 },
      {
        to: [-W / 2 - FACE, y],
        tol: 1.5,
        vmax: 260,
        arrive: () => {
          j.committed = true;
          commit(pick);
          jolt = { at: clock, depth: SLOT_OF[pick.mod] };
        },
        dwell: 2 * TICK,
      },
      { to: [-W / 2 - 30 - CLEAR, y], tol: 14, vmax: 700 },
    ];
    return j;
  }

  function runJob() {
    for (let guard = 0; guard < 6; guard++) {
      if (!job) {
        pump();
        if (!job) return;
      }
      const j = job;
      const s = j.steps[j.i];
      if (!s) {
        job = null;
        continue;
      }
      if (!j.started) {
        j.started = true;
        s.start?.();
        // with more waiting behind it, the arm hurries: every request still happens, in order
        if (s.to) setGoal(s.to, (s.vmax ?? 700) * (intro ? 2.4 : queue.length > 0 ? 1.7 : 1), s.tol);
      }
      if (!j.arrived) {
        if (s.to && !arrived()) return;
        j.arrived = true;
        j.until = clock + (s.dwell ?? 0);
        s.arrive?.();
      }
      if (clock < j.until) return;
      j.i++;
      j.started = false;
      j.arrived = false;
    }
  }

  /** every module placed, every cable drawn, the arm at home: the end of the intro, instantly */
  function settleAll() {
    for (const slot of slots) {
      slot.state = 'placed';
      slot.crt = null;
      slot.leader.at = -Infinity;
      reveal(slot, false);
      dress(slot);
    }
    cart.state = 'shelf';
    job = null;
    queue.length = 0;
    pending = null;
    arm.x = arm.cx = arm.gx = HOME[0];
    arm.y = arm.cy = arm.gy = HOME[1];
    arm.vx = arm.vy = 0;
    typeAt = -Infinity;
    cableDirty = true;
  }

  function skipIntro() {
    intro = false;
    settleAll();
    report('architecture · customer boundary');
  }

  function reveal(slot: Slot, animate = true) {
    slot.row.dataset.state = 'on';
    if (animate) slot.row.animate(ditherKeyframes(2), { duration: TIMING.lead / 2, easing: 'steps(16, start)' });
  }

  function start() {
    if (started) return;
    started = true;
    if (opts.reduced) {
      settleAll();
      mesh!.invalidate();
      return;
    }
    intro = true;
    for (const id of [...ORDER].reverse()) queue.push({ kind: 'build', mod: id });
    // the demo: swap the executor once, to show what the arm does
    if (!touched) queue.push({ kind: 'pick', pick: { mod: 'adapters', group: 0, option: 1 }, demo: true });
    mesh!.invalidate();
  }

  // ---------------------------------------------------------------- input
  const press = (mod: ModuleId) => {
    const slot = slotById(mod);
    const group = shown.mod === mod ? shown.group : slot.last;
    const pick: Pick = { mod, group, option: nextOption(mod, group, want[mod][group]) };
    cb.sync(pick);
    request(pick);
  };

  let selected = '';
  const onDown = () => {
    selected = getSelection()?.toString() ?? '';
  };
  const onClick = (event: MouseEvent) => {
    const now = getSelection()?.toString() ?? '';
    // a click that ends a text selection isn't a press
    if (event.detail > 0 && now !== '' && now !== selected) return;
    const target = event.target as Element;
    const face = target.closest<HTMLElement>('[data-plate], [data-display]');
    let mod: ModuleId | undefined;
    if (face) mod = (face.dataset.plate ?? face.dataset.display) as ModuleId;
    else if (target === canvas) mod = pick(event.offsetX, event.offsetY);
    if (mod) press(mod);
  };
  const onKey = (event: KeyboardEvent) => {
    const target = event.target as HTMLElement;
    if (target.dataset.plate && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault();
      press(target.dataset.plate as ModuleId);
    }
  };
  root.addEventListener('pointerdown', onDown);
  root.addEventListener('click', onClick);
  root.addEventListener('keydown', onKey);

  /** the module whose front face the point's ray strikes first, nearest wins */
  function pick(px: number, py: number): ModuleId | undefined {
    const [x0, y0] = mesh!.unproject(px, py);
    let found = -1;
    let nearest = -Infinity;
    for (let i = 0; i < slots.length; i++) {
      if (slots[i].state !== 'placed') continue;
      const half = STACKED[i].height / 2;
      const near = Math.min(D / 2, (W / 2 - x0) / DEPTH[0], (PLACES[i] + half - y0) / DEPTH[1]);
      const far = Math.max(-D / 2, (-W / 2 - x0) / DEPTH[0], (PLACES[i] - half - y0) / DEPTH[1]);
      if (near >= far && near > nearest) {
        nearest = near;
        found = i;
      }
    }
    return found < 0 ? undefined : STACKED[found].id;
  }

  /** the arm follows a mouse over the figure when nothing else drives it; touch and keys never depend on it */
  const onMove = (event: PointerEvent) => {
    if (event.pointerType !== 'mouse' || intro || !started) return;
    const target = event.target as Element;
    const face = target.closest<HTMLElement>('[data-plate], [data-display]');
    let mod: ModuleId | undefined;
    let at: Point | null = null;
    if (face) mod = (face.dataset.plate ?? face.dataset.display) as ModuleId;
    else if (target === canvas) {
      mod = pick(event.offsetX, event.offsetY);
      if (!mod) {
        const [x, y] = mesh!.unproject(event.offsetX, event.offsetY);
        at = [clamp(x, -W / 2 - 140, -W / 2 - 40), clamp(y, 40, STACK - 30)];
      }
    }
    if (mod) at = [-W / 2 - 26, PLACES[SLOT_OF[mod]]];
    const index = mod ? SLOT_OF[mod] : -1;
    if (index !== aimMod) {
      if (aimMod >= 0) delete slots[aimMod].row.dataset.aim;
      if (index >= 0) slots[index].row.dataset.aim = '';
      aimMod = index;
    }
    const cursor = mod ? 'pointer' : '';
    if (canvas.style.cursor !== cursor) canvas.style.cursor = cursor;
    aim = at;
    mesh!.invalidate();
  };
  const onLeave = () => {
    aim = null;
    if (aimMod >= 0) delete slots[aimMod].row.dataset.aim;
    aimMod = -1;
    mesh!.invalidate();
  };
  holder.addEventListener('pointermove', onMove, { passive: true });
  holder.addEventListener('pointerleave', onLeave, { passive: true });

  // the intro plays once, when the figure is half in view
  const shownIo = new IntersectionObserver(
    (entries) => {
      if (entries[entries.length - 1].intersectionRatio >= 0.5) {
        start();
        mesh!.invalidate();
      }
    },
    { threshold: [0, 0.5] },
  );
  shownIo.observe(canvas);

  // ---------------------------------------------------------------- the frame
  const setText = (target: HTMLElement, text: string) => {
    if (target.textContent !== text) target.textContent = text;
  };

  function crt(at: number, kind: 'open' | 'swap', now: number): Point {
    const phase = TIMING.crt;
    let t = now - at;
    if (kind === 'open') t += phase.line + phase.dot + phase.hold;
    if (t < phase.line) return [1, mix(1, 0.02, easeInOut(t / phase.line))];
    t -= phase.line;
    if (t < phase.dot) return [mix(1, 0.02, easeInOut(t / phase.dot)), 0.02];
    t -= phase.dot;
    if (t < phase.hold) return [0.02, 0.02];
    t -= phase.hold;
    if (t < phase.dot) return [mix(0.02, 1, easeInOut(t / phase.dot)), 0.02];
    t -= phase.dot;
    if (t < phase.line) return [1, mix(0.02, 1, easeInOut(t / phase.line))];
    return FULL;
  }

  function frame(dt: number) {
    clock += dt;
    if (pending && clock >= pending.at) {
      const p = pending.pick;
      pending = null;
      commit(p);
    }
    runJob();
    // idle: follow the mouse, else park
    if (!job) {
      if (aim && !intro && started) setGoal(aim, 900, 6);
      else setGoal(HOME, 520, 6);
    }
    follow(Math.min(dt, 50) / 1000);
    poseArm();
    if (intro && !job && queue.length === 0) {
      intro = false;
      report('architecture · customer boundary');
    }
    let moving = armBusy || job !== null || queue.length > 0 || pending !== null;
    moving = poseSlots() || moving;
    if (showReadout()) moving = true;
    if (cableDirty || cablesDrawing()) {
      if (drawCables()) moving = true;
    }
    if (moving) mesh!.invalidate();
  }

  function poseArm() {
    pad[0] = arm.x;
    pad[1] = arm.y;
    reach(pad, angles);
    const [b, f] = angles;
    const elbow: Point = [SHOULDER[0] + Math.cos(b) * BOOM, SHOULDER[1] + Math.sin(b) * BOOM];
    const wrist: Point = [elbow[0] + Math.cos(f) * FOREARM, elbow[1] + Math.sin(f) * FOREARM];
    placeLink(boom, links.boom, [SHOULDER[0], SHOULDER[1], 0], b);
    placeLink(forearm, links.forearm, [elbow[0], elbow[1], 0], f);
    placeLink(tool, links.tool, [wrist[0], wrist[1], 0], 0);
    const head: Vec3 = [
      SHOULDER[0] + Math.cos(b) * RAM_HEAD[0] - Math.sin(b) * RAM_HEAD[1],
      SHOULDER[1] + Math.sin(b) * RAM_HEAD[0] + Math.cos(b) * RAM_HEAD[1],
      RAM_FOOT[2],
    ];
    const split: Vec3 = [mix(RAM_FOOT[0], head[0], 0.55), mix(RAM_FOOT[1], head[1], 0.55), RAM_FOOT[2]];
    mesh!.place(ram[0], between(partM, RAM_FOOT, split, 18));
    mesh!.place(ram[1], between(partM, split, head, 9));
    // the cartridge, on its plinth or in the pad's grip; none while it is docked
    const held = cart.state === 'held';
    const at: Vec3 = held ? [pad[0] + HELD, pad[1], 0] : [GRIP[0] + HELD, GRIP[1], 0];
    transform(slotM, at);
    for (let k = 0; k < cartridge.length; k++) {
      mesh!.show(cartridge[k], cart.state !== 'docked');
      mesh!.place(cartridge[k], multiply(partM, slotM, links.cartridge[k]));
    }
  }

  function placeLink(parts: number[], locals: Float32Array[], at: Vec3, angle: number) {
    transform(linkM, at, [0, 0, angle]);
    for (let i = 0; i < locals.length; i++) mesh!.place(parts[i], multiply(partM, linkM, locals[i]));
  }

  /** the modules: where each is, how squashed, what it wears; true while anything on them is moving */
  function poseSlots(): boolean {
    let moving = false;
    for (let i = 0; i < slots.length; i++) {
      const slot = slots[i];
      const scale = slot.crt ? crt(slot.crt.at, slot.crt.kind, clock) : FULL;
      if (slot.crt) {
        moving = true;
        if (clock - slot.crt.at > CRT_MS) slot.crt = null;
      }
      let y = PLACES[i];
      let x = 0;
      if (slot.state === 'held') {
        x = pad[0] + W / 2;
        y = pad[1];
      }
      const since = clock - jolt.at;
      if (slot.state === 'placed' && i <= jolt.depth && since < TIMING.jolt) {
        y -= 2 * Math.sin((since / TIMING.jolt) * Math.PI);
        moving = true;
      }
      const pivot = slot.crt?.kind === 'open' ? -W / 2 : 0;
      const sig = `${x.toFixed(2)}|${y.toFixed(2)}|${scale[0].toFixed(3)}|${scale[1].toFixed(3)}|${slot.state}|${pivot}|${slot.have.join(',')}`;
      slot.scale = scale;
      if (sig !== slot.sig) {
        slot.sig = sig;
        cableDirty = true;
        transform(slot.matrix, [x + pivot, y, 0], undefined, [scale[0], scale[1], scale[1]]);
        multiply(slot.matrix, slot.matrix, transform(linkM, [-pivot, 0, 0]));
        const visible = slot.state !== 'off';
        const line = scale[1] < 0.5;
        for (let k = 0; k < slot.parts.length; k++) {
          const piece = slot.pieces[k];
          const part = slot.parts[k];
          const on = !piece.when || piece.when[1].includes(slot.have[piece.when[0]]);
          mesh!.show(part, visible && on);
          mesh!.shadow(part, slot.state === 'placed');
          const material = piece.dress ? piece.dress[1][slot.have[piece.dress[0]]] : piece.material;
          mesh!.material(part, line ? MATERIAL.accent : material);
          mesh!.place(part, multiply(partM, slot.matrix, slot.locals[k]));
        }
      }
      plates[i].shown = slot.state !== 'off';
    }
    const engine = slotById('engine');
    monitorAnchor.shown = engine.state !== 'off';
    const kernel = slotById('kernel');
    listingAnchor.shown = kernel.state === 'placed' && kernel.scale[0] === 1 && kernel.scale[1] === 1;
    return moving;
  }

  /** the monitor types the committed readout line by line; true while it still has lines to type */
  function showReadout(): boolean {
    const readout = readoutOf(shown);
    const count = typeAt === -Infinity ? 1 + readout.lines.length : 1 + Math.max(0, Math.floor((clock - typeAt) / TIMING.line));
    const upto = Math.min(count, 1 + readout.lines.length);
    const lines = [readout.title, ...readout.lines];
    for (let n = 0; n < monitorLines.length + 1; n++) {
      const target = n === 0 ? monitorTitle : monitorLines[n - 1];
      const text = n < upto && n <= maxLines ? lines[n] : '';
      if (drawn[n] !== text) {
        drawn[n] = text;
        setText(target, text);
      }
    }
    const kernelSlot = slotById('kernel');
    const hit = kernelSlot.have[0];
    if (hit !== lit) {
      lit = hit;
      listed.forEach((line, i) => line.toggleAttribute('data-hit', i === hit));
    }
    return upto < 1 + readout.lines.length;
  }

  // ---------------------------------------------------------------- cables
  const cablesDrawing = () => slots.some((s) => !Number.isNaN(s.leader.at) && clock - s.leader.at < TIMING.lead);

  function pathOf(points: Point[]): { d: string; len: number; mid: { at: Point; dir: Point } } {
    const CH = 8;
    const line: Point[] = [points[0]];
    const toward = (a: Point, b: Point, d: number): Point => {
      const l = Math.hypot(b[0] - a[0], b[1] - a[1]) || 1;
      return [a[0] + ((b[0] - a[0]) / l) * d, a[1] + ((b[1] - a[1]) / l) * d];
    };
    const run = (i: number) => Math.hypot(points[i + 1][0] - points[i][0], points[i + 1][1] - points[i][1]);
    for (let i = 1; i < points.length - 1; i++) {
      const cut = Math.min(CH, run(i - 1) / 2, run(i) / 2);
      line.push(toward(points[i], points[i - 1], cut), toward(points[i], points[i + 1], cut));
    }
    line.push(points[points.length - 1]);
    let len = 0;
    let best = -1;
    let bestAt: Point = line[0];
    let bestDir: Point = [1, 0];
    for (let i = 1; i < line.length; i++) {
      const l = Math.hypot(line[i][0] - line[i - 1][0], line[i][1] - line[i - 1][1]);
      // the arrow rides the longest straight run
      if (l > best && (line[i][0] === line[i - 1][0] || line[i][1] === line[i - 1][1])) {
        best = l;
        bestAt = [(line[i][0] + line[i - 1][0]) / 2, (line[i][1] + line[i - 1][1]) / 2];
        bestDir = [Math.sign(line[i][0] - line[i - 1][0]), Math.sign(line[i][1] - line[i - 1][1])];
      }
      len += l;
    }
    return { d: `M${line.map(([x, y]) => `${x} ${y}`).join('L')}`, len, mid: { at: bestAt, dir: bestDir } };
  }

  function arrowPoints(at: Point, dir: Point, flow: Flow): string {
    const [x, y] = at;
    const [dx, dy] = flow === 'in' ? [-dir[0], -dir[1]] : dir;
    // a 5px triangle pointing along the flow
    const nx = -dy;
    const ny = dx;
    const tip: Point = [x + dx * 5, y + dy * 5];
    const l: Point = [x - dx * 3 + nx * 4, y - dy * 3 + ny * 4];
    const r: Point = [x - dx * 3 - nx * 4, y - dy * 3 - ny * 4];
    return `${tip} ${l} ${r}`;
  }

  /** every placed module's cable, from a port on its face edge to its row's: straight where they meet, else across,
   * along a lane of its own, and across. Written only when its route or look changed. */
  function drawCables(): boolean {
    const on = beside.matches;
    let moving = false;
    const down: number[] = [];
    const up: number[] = [];
    const starts: Point[] = [];
    for (let i = 0; i < slots.length; i++) {
      const slot = slots[i];
      const [x, y] = mesh!.project([W / 2, 0, -D / 2], slot.matrix);
      starts.push([Math.round(x), Math.round(y)]);
      if (slot.end[1] > Math.round(y) + 1) down.push(i);
      else if (slot.end[1] < Math.round(y) - 1) up.push(i);
    }
    for (let i = 0; i < slots.length; i++) {
      const slot = slots[i];
      const leader = slot.leader;
      const g = leader.g!;
      const visible = on && slot.state === 'placed' && !Number.isNaN(leader.at);
      if (!visible) {
        if (g.getAttribute('visibility') !== 'hidden') g.setAttribute('visibility', 'hidden');
        continue;
      }
      const [sx, sy] = starts[i];
      const [ex, ey] = slot.end;
      let points: Point[];
      if (Math.abs(ey - sy) <= 1) points = [[sx, ey], [ex, ey]];
      else {
        // lower modules take the lanes nearer the module on the way down, upper ones on the way up, so they never cross
        const group = ey > sy ? down : up;
        const rank = ey > sy ? group.indexOf(i) : group.length - 1 - group.indexOf(i);
        const jog = Math.round((sx + ex) / 2) + (group.length - 1 - rank) * LANE - ((group.length - 1) * LANE) / 2;
        points = [[sx, sy], [jog, sy], [jog, ey], [ex, ey]];
      }
      const m = MODULES[slot.id];
      const look = cableOf(Object.fromEntries(slots.map((s) => [s.id, s.have])) as Selection, slot.id);
      const route = `${points.join(' ')}|${look}`;
      if (route !== leader.route) {
        leader.route = route;
        const p = pathOf(points);
        leader.len = p.len;
        leader.look = look;
        leader.outer.setAttribute('d', p.d);
        leader.core.setAttribute('d', p.d);
        leader.core.setAttribute('class', slot.id === 'adapters' ? 'stroke-lp-indigo' : 'stroke-lp-paper');
        leader.a.setAttribute('transform', `translate(${points[0][0]} ${points[0][1]})`);
        leader.b.setAttribute('transform', `translate(${ex} ${ey})`);
        leader.arrow.setAttribute('points', arrowPoints(p.mid.at, p.mid.dir, m.cable.flow));
        leader.arrow.setAttribute('visibility', m.cable.flow === 'none' ? 'hidden' : 'visible');
      }
      g.setAttribute('visibility', 'visible');
      const p = leader.at === -Infinity ? 1 : clamp((clock - leader.at) / TIMING.lead);
      const eased = 1 - Math.pow(1 - p, 3);
      if (p < 1) moving = true;
      // drawn on, then at rest: whole, or dashed where the design is not built
      const dash = p < 1 ? `${leader.len}` : leader.look === 'dashed' ? '7 5' : 'none';
      const offset = p < 1 ? `${leader.len * (1 - eased)}` : '0';
      for (const path of [leader.outer, leader.core]) {
        path.style.strokeDasharray = dash;
        path.style.strokeDashoffset = offset;
      }
      leader.arrow.setAttribute('opacity', p < 1 ? '0' : '1');
    }
    cableDirty = moving;
    return moving;
  }

  let dead = false;
  // ---------------------------------------------------------------- start
  slots.forEach((s) => dress(s));
  if (opts.reduced) {
    // finished at once, with working controls
    for (const slot of slots) slot.state = 'placed';
    settleAll();
  }

  return {
    request(pick: Pick) {
      request(pick);
    },
    destroy() {
      if (dead) return;
      dead = true;
      shownIo.disconnect();
      refit.disconnect();
      root.removeEventListener('pointerdown', onDown);
      root.removeEventListener('click', onClick);
      root.removeEventListener('keydown', onKey);
      holder.removeEventListener('pointermove', onMove);
      holder.removeEventListener('pointerleave', onLeave);
      canvas.removeEventListener('webglcontextlost', onLost);
      canvas.style.cursor = '';
      cables.replaceChildren();
      mesh.destroy();
      // a remount makes its own canvas, so this one's context goes now
      canvas.getContext('webgl2')?.getExtension('WEBGL_lose_context')?.loseContext();
      canvas.remove();
    },
  };
}
