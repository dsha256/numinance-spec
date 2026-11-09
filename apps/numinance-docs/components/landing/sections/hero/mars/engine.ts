import { drawOperator, OPERATOR_H, OPERATOR_W, type OperatorFrame } from '@/lib/landing/operator';
import { onThemeChange, readPalette, rgb, type Palette } from '@/lib/landing/palette';
import { mulberry32 } from '@/lib/landing/rng';
import { scopedStatus } from '@/lib/landing/status';
import { fitCanvas } from '@/lib/landing/use-scene';
import { STAGES } from '../engine';
import { buildSurface, type Surface } from '../texture';
import {
  TAU,
  angle,
  bodyToView,
  camera,
  clamp,
  lonLat,
  mix,
  orbitVec,
  project,
  slerp,
  smooth,
  solveKepler,
  spring,
  stepAngle,
  stepSpring,
  wrapPi,
  type Orbit,
  type Proj,
} from './dynamics';
import { announceFor } from './copy';
import { isAutonomous, levelIndex, LEVELS, type HeroEvent, type HeroState, type Snapshot, type Store } from './machine';
import {
  acceptedAt,
  AUTO_REFETCH_MS,
  BOOT_MS,
  BUNDLE_DIGEST,
  CAPTURE_AT,
  CAPTURE_MS,
  CHECKS,
  checkWindow,
  DIGESTS,
  FRESH,
  NODE_TIPS,
  PERMIT_MS,
  POLICY_MS,
  POLICY_RING,
  POLICY_RING_MS,
  REFETCH_DIGEST,
  READY_MS,
  RETRIEVE_MS,
  REVIEW_ORBIT,
  SEQ,
  SOURCES,
  SPAWN_STEP,
  STALE_AT,
  STALE_INDEX,
  STALE_NOTE,
  validateEnd,
} from './model';
import { BASE_LIGHT, createPlanet, marsColors, TILT_A, type Colors } from './planet';
import { createTerrainBuilder, type Terrain, type TerrainBuilder } from './terrain';

const setStatus = scopedStatus('numinance');

/** original dormant script, kept as-is so the first screen IS the approved hero */
const INSPECT = [-0.3, -0.7, -0.45, -0.65, -0.25];
const NODE_STEP = 0.2;
const T0 = 1700;
const BEAT = 4200;
const ALIGN = 1100;
const FIN = 2600;
const PROBE_FIRST = 800;
const PROBE_AFTER = ALIGN + 400;
const CYCLE = 5 * BEAT + 2 * FIN;
const TYPE_MS = 38;
const FINALE = ['verified? not by me.', 'check it yourself.'] as const;
const TILT = -0.2;
const PITCH = Math.asin(0.4 / 1.34);
const RING_N = 7; // 5 public waypoints + review + release chips

const ease = (v: number) => smooth(v);
const nodeTarget = (i: number) => -NODE_STEP * i + (INSPECT[i % 5] - INSPECT[0]) / TAU;

export interface MarsEngine {
  frame: (dt: number, t: number) => void;
  cleanup: () => void;
}
export interface MarsOptions {
  reduced: boolean;
  invalidate: () => void;
  store: Store;
  /** start in the completed post-gate pose (no morph, no sequence) */
  settled: boolean;
  onFail: () => void;
}

interface Star {
  x: number;
  y: number;
  s: number;
  a: number;
  tw: number;
  plus: boolean;
}
interface SrcPos extends Proj {
  c: number; // capture progress 0..1
  on: boolean; // drawn this frame
  sx: number; // surface-pin screen position
  sy: number;
  sz: number;
}

export function createMarsHero(root: HTMLElement, opts: MarsOptions): MarsEngine {
  const reduced = opts.reduced;
  const store = opts.store;
  const canvas = root.querySelector('canvas') as HTMLCanvasElement;
  const chips = Array.from(root.querySelectorAll<HTMLElement>('[data-node]'));
  const bubble = root.querySelector<HTMLElement>('[data-bubble]')!;
  const sizer = bubble.querySelector<HTMLElement>('[data-sizer]')!;
  const said = bubble.querySelector<HTMLElement>('[data-said]')!;
  const tipEl = root.querySelector<HTMLElement>('[data-tip]')!;
  const section = root.closest('section');

  let ctx0 = canvas.getContext('2d');
  if (!ctx0) {
    opts.onFail();
    return { frame: () => {}, cleanup: () => {} };
  }

  const surface: Surface = buildSurface();
  const planet = createPlanet();
  const buf = document.createElement('canvas');
  const bctx = buf.getContext('2d')!;

  let lastSphere = -1e9;
  // Mars terrain is built lazily, in small slices, never inside a frame's budget
  let terrain: Terrain | null = null;
  let builder: TerrainBuilder | null = createTerrainBuilder();
  let buildTimer = 0;
  let buildCpu = 0;
  const pump = (ms: number) => {
    if (!builder) return;
    const t0 = performance.now();
    const done = builder.step(ms);
    buildCpu += performance.now() - t0;
    if (done) {
      // profiling aid: cumulative main-thread time spent generating the cached terrain (read via performance.getEntriesByName)
      performance.mark('mars:terrain-cpu', { detail: { ms: +buildCpu.toFixed(1) } });
      terrain = builder.out;
      builder = null;
      lastSphere = -1e9;
      opts.invalidate();
    }
  };
  const schedule = () => {
    if (!builder) return;
    buildTimer = window.setTimeout(() => {
      pump(5);
      schedule();
    }, 16);
  };
  schedule(); // sliced across idle gaps; `ensureTerrain` finishes it synchronously if Mars is needed sooner
  const ensureTerrain = () => {
    if (builder) pump(1e6);
  };

  let pal: Palette = readPalette();
  let colors!: Colors;
  const buildColors = () => {
    pal = readPalette();
    const m = marsColors(pal.dark);
    colors = {
      dark: pal.dark,
      base: rgb(pal.paper2),
      dot: rgb(pal.ink),
      indigo: rgb(pal.indigo),
      yellow: rgb(pal.yellow),
      paper: rgb(pal.paper),
      ...m,
    };
  };
  buildColors();
  const refreshPalette = () => {
    buildColors();
    lastSphere = -1e9;
    opts.invalidate();
  };
  const stopTheme = onThemeChange(refreshPalette);

  // ---- geometry ----
  let W = 0,
    H = 0,
    cx = 0,
    cy = 0,
    R = 0,
    rx = 0,
    ry = 0,
    cell = 3,
    N = 0,
    narrow = false,
    dpr = 1;
  let ctx = ctx0;
  let font = '12px monospace';
  let stars: Star[] = [];
  const cam = camera(PITCH, TILT);
  const camPolicy = camera(PITCH, 0.32);

  const resize = () => {
    const box = root.getBoundingClientRect();
    if (box.width < 10 || box.height < 10) return;
    W = Math.round(box.width);
    H = Math.round(box.height);
    narrow = W < 560;
    const fit = fitCanvas(canvas, narrow ? 1.5 : 2);
    ctx = fit.ctx;
    dpr = fit.dpr;
    const fam = getComputedStyle(canvas).fontFamily || 'monospace';
    font = `12px ${fam}`;
    // same layout as the original, with a dock-sized band reserved below the sphere
    // the dock (state, context/authority, ladder, stage rail, controls) is ~215px: the whole sphere sits above it
    const dockH = narrow ? 222 : 218;
    R = Math.min(W * 0.355, H * 0.335, (H - dockH - 104) / 2);
    R = Math.max(70, R);
    cell = 3;
    cx = W / 2;
    cy = Math.max(H * 0.55, R + (narrow ? 104 : 112));
    if (cy + R + R * 0.45 + 34 > H) cy = Math.max(R + 90, H - R * 1.45 - 34);
    // on a phone the dock (state + ladder + buttons) is ~170px tall: keep the whole sphere above it
    cy = Math.max(R + (narrow ? 78 : 100), Math.min(cy, H - dockH - R - 6));
    rx = R * 1.34;
    ry = R * 0.4;
    N = Math.ceil((2 * R) / cell) + 2;
    buf.width = N;
    buf.height = N;
    const tb = performance.now();
    planet.build(N, R, cell);
    performance.mark('mars:tables-cpu', { detail: { ms: +(performance.now() - tb).toFixed(1), cells: N * N } });
    const rnd = mulberry32(99);
    const n = narrow ? 70 : 150;
    stars = [];
    for (let i = 0; i < n; i++)
      stars.push({ x: rnd(), y: rnd(), s: rnd() < 0.82 ? 2 : 4, a: 0.25 + rnd() * 0.6, tw: rnd() < 0.16 ? rnd() * TAU : -1, plus: false });
    for (let i = 0; i < 6; i++) stars.push({ x: rnd(), y: rnd(), s: 4, a: 0.7, tw: rnd() * TAU, plus: true });
    for (const c of chips) delete c.dataset.k;
    lastSphere = -1e9;
    opts.invalidate();
  };

  // ---- simulation state ----
  let snap: Snapshot = store.get();
  let seenRev = -1;
  // autoplay is one-shot per page load: it starts on the first frame the scene is actually running (visible tab, on screen)
  let autoTried = false;
  // the original intro ramps (print, ring, nodes, operator) run faster when the story starts at once
  let fastIntro = false;
  let sClock = 0; // ms on the current state's timeline (accelerated by "Speed up")
  let stateT = 0;
  let accDt = 0;
  let lastDraw = -1e9;

  const rotSpring = spring(0, 0.008); // x = rot (turns), v = rate; critically damped toward the target rate
  const rate = spring(0.008, 0);
  const marsS = spring(0, 0);
  const policyS = spring(0, 0);
  const regionS = spring(0, 0);
  const lightAz = spring(0, 0);
  const phase = spring(0, 0);
  const par = { x: spring(0, 0), y: spring(0, 0), tx: 0, ty: 0 };
  const focusA = spring(0, 0); // reticle angle on the workflow ring
  const focusR = spring(0, 0); // radial reach 0..1
  const focusAlpha = spring(0, 0);
  const ptr = { x: 0, y: 0, on: false, t: 0 };
  let fine = false;
  try {
    fine = matchMedia('(pointer: fine)').matches;
  } catch {}

  let rot = 0; // body spin, turns
  let opX = 0;
  let opFlip = false;
  let walkClock = 0;
  let moving = false;
  let lineStart = 0;
  let lineText = '';
  let shown = -1;
  let beatKey = '';
  let bubbleVisible = false;
  let bw = 0,
    bh = 0;
  let ringDormant = true; // the ring follows the original's script directly
  let hover = -1;
  let hoverSrc = -1;
  let scanAngle = 0;

  // anchors on the body (set when the experience wakes, so they land on the visible hemisphere)
  const anchors: [number, number, number][] = SOURCES.map(() => [0, 0, 1]);
  const hubB: [number, number, number] = [0, 0, 1];
  const frameS = { rot: 0, ca: Math.cos(TILT_A), sa: Math.sin(TILT_A) };
  const tmp: [number, number, number] = [0, 0, 0];
  const tmp2: [number, number, number] = [0, 0, 0];
  const pr: Proj = { x: 0, y: 0, z: 0 };
  const srcPos: SrcPos[] = SOURCES.map(() => ({ x: 0, y: 0, z: 0, c: 0, on: false, sx: 0, sy: 0, sz: 0 }));
  const nodeScr = Array.from({ length: RING_N }, () => ({ x: 0, y: 0, front: true, a: 0, on: false }));
  const hubView: [number, number, number] = [0, 0, 1];
  const wakeAnchors = () => {
    for (let i = 0; i < SOURCES.length; i++) {
      // view-frame site -> body frame at the current spin
      lonLat(SOURCES[i].lon - rot * TAU, SOURCES[i].lat, anchors[i]);
    }
    let sx = 0,
      sy = 0,
      sz = 0;
    for (const a of anchors) (sx += a[0]), (sy += a[1]), (sz += a[2]);
    const l = Math.hypot(sx, sy, sz);
    hubB[0] = sx / l;
    hubB[1] = sy / l;
    hubB[2] = sz / l;
  };
  wakeAnchors();

  const onMove = (e: PointerEvent) => {
    if (reduced) return;
    if (e.pointerType === 'touch') return;
    if (!fine) return;
    ptr.x = e.clientX;
    ptr.y = e.clientY;
    ptr.on = true;
    ptr.t = performance.now();
  };
  const onDown = (e: PointerEvent) => {
    if (reduced || e.pointerType !== 'touch') return;
    const b = root.getBoundingClientRect();
    if (e.clientX < b.left || e.clientX > b.right || e.clientY < b.top || e.clientY > b.bottom) return;
    ptr.x = e.clientX;
    ptr.y = e.clientY;
    ptr.on = true;
    opts.invalidate();
  };
  const onLeave = () => {
    ptr.on = false;
  };
  window.addEventListener('pointermove', onMove, { passive: true });
  window.addEventListener('pointerdown', onDown, { passive: true });
  document.documentElement.addEventListener('pointerleave', onLeave);

  // canvas hover: sources and the chips' own pointer / focus events share one tooltip
  const tipText = (i: number, isSrc: boolean) => {
    const v = snap.vars;
    if (isSrc) {
      const s = SOURCES[i];
      const stale = i === STALE_INDEX && v.staleSource === 'flagged';
      const d = i === STALE_INDEX && v.staleSource === 'refetched' ? REFETCH_DIGEST : DIGESTS[i];
      if (stale) return `${s.label.toLowerCase()} / stale ${STALE_NOTE}`;
      return `${s.receipt} / ref ${d} / ${i === STALE_INDEX && v.staleSource === 'refetched' ? 'v5' : s.version} / sim`;
    }
    return NODE_TIPS[i].tip(BUNDLE_DIGEST);
  };
  const showTip = (text: string, x: number, y: number) => {
    if (tipEl.textContent !== text) tipEl.textContent = text;
    tipEl.dataset.state = 'on';
    const w = tipEl.offsetWidth || 120;
    tipEl.style.transform = `translate(${Math.round(clamp(x - w / 2, 6, W - w - 6))}px, ${Math.round(clamp(y, 6, H - 28))}px)`;
  };
  const hideTip = () => {
    if (tipEl.dataset.state !== 'off') tipEl.dataset.state = 'off';
  };
  const chipHandlers: (() => void)[] = [];
  chips.forEach((chip, i) => {
    const on = () => (hover = i);
    const off = () => hover === i && (hover = -1);
    chip.addEventListener('pointerenter', on);
    chip.addEventListener('pointerleave', off);
    chip.addEventListener('focus', on);
    chip.addEventListener('blur', off);
    // touch / click toggles the receipt label; arrows move between waypoints
    const click = () => ((hover = hover === i ? -1 : i), opts.invalidate());
    const key = (e: KeyboardEvent) => {
      const dir = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
      if (!dir) return;
      e.preventDefault();
      const live = chips.filter((c) => c.tabIndex === 0);
      const at = live.indexOf(chip);
      live[(at + dir + live.length) % live.length]?.focus();
    };
    chip.addEventListener('click', click);
    chip.addEventListener('keydown', key);
    chipHandlers.push(() => {
      chip.removeEventListener('pointerenter', on);
      chip.removeEventListener('pointerleave', off);
      chip.removeEventListener('focus', on);
      chip.removeEventListener('blur', off);
      chip.removeEventListener('click', click);
      chip.removeEventListener('keydown', key);
    });
  });
  const onCanvasMove = (e: PointerEvent) => {
    if (reduced) return;
    const b = root.getBoundingClientRect();
    const px = e.clientX - b.left,
      py = e.clientY - b.top;
    let best = -1,
      bd = 14;
    for (let i = 0; i < SOURCES.length; i++) {
      const s = srcPos[i];
      if (!s.on || s.c < 1) continue;
      const d = Math.hypot(s.sx - px, s.sy - py);
      if (d < bd) (bd = d), (best = i);
    }
    hoverSrc = best;
  };
  root.addEventListener('pointermove', onCanvasMove, { passive: true });
  const onCtxLost = (e: Event) => e.preventDefault();
  const onCtxBack = () => resize();
  canvas.addEventListener('contextlost', onCtxLost);
  canvas.addEventListener('contextrestored', onCtxBack);
  if (reduced) {
    // no autoplay motion: land on the completed, coherent state at once (the rail and controls then browse it)
    const s0 = store.get();
    if (s0.rev < 6 && s0.vars.stop === null && s0.state !== 'AUTONOMOUS_INTERACTIVE') store.dispatch({ type: 'JUMP', to: 'evidence' });
  }
  const unsubStore = store.subscribe(() => opts.invalidate());

  const ro = new ResizeObserver(resize);
  ro.observe(root);
  resize();

  // ---- helpers ----
  const ringPoint = (a: number) => {
    const c = Math.cos(TILT),
      s = Math.sin(TILT);
    const ex = rx * Math.cos(a),
      ey = ry * Math.sin(a);
    return { x: cx + ex * c - ey * s, y: cy + ex * s + ey * c, front: Math.sin(a) > 0 };
  };
  const hiddenBehind = (x: number, y: number, front: boolean) => !front && Math.hypot(x - cx, y - cy) < R + 6;

  function pixelText(text: string, x: number, y: number, color: string, align: CanvasTextAlign = 'left') {
    ctx.font = font;
    ctx.textAlign = align;
    ctx.textBaseline = 'alphabetic';
    ctx.fillStyle = color;
    ctx.fillText(text, Math.round(x), Math.round(y));
  }
  /** boxed technical label */
  function tag(text: string, x: number, y: number, align: 'left' | 'right' | 'center', fg: string, bg: string, line: string) {
    ctx.font = font;
    const w = Math.ceil(ctx.measureText(text).width) + 8;
    let x0 = align === 'left' ? x : align === 'right' ? x - w : x - w / 2;
    x0 = clamp(x0, 4, W - w - 4);
    const y0 = clamp(y, 4, H - 22);
    ctx.fillStyle = bg;
    ctx.fillRect(Math.round(x0), Math.round(y0), w, 16);
    ctx.strokeStyle = line;
    ctx.lineWidth = 1;
    ctx.strokeRect(Math.round(x0) + 0.5, Math.round(y0) + 0.5, w - 1, 15);
    pixelText(text, x0 + 4, y0 + 12, fg);
  }
  const crosshair = (x: number, y: number, s: number) => {
    ctx.beginPath();
    ctx.moveTo(Math.round(x) - s + 0.5, Math.round(y) + 0.5);
    ctx.lineTo(Math.round(x) + s + 0.5, Math.round(y) + 0.5);
    ctx.moveTo(Math.round(x) + 0.5, Math.round(y) - s + 0.5);
    ctx.lineTo(Math.round(x) + 0.5, Math.round(y) + s + 0.5);
    ctx.stroke();
  };
  const dashRing = (from: number, to: number, front: boolean, ox: number, oy: number) => {
    const steps = 180;
    ctx.beginPath();
    let pen = false;
    for (let i = 0; i <= steps; i++) {
      const a = from + ((to - from) * i) / steps;
      const p = ringPoint(a);
      if (p.front === front) {
        if (!pen) ctx.moveTo(p.x + ox + 0.5, p.y + oy + 0.5);
        else ctx.lineTo(p.x + ox + 0.5, p.y + oy + 0.5);
        pen = true;
      } else pen = false;
    }
    ctx.stroke();
  };
  /** polyline of an orbit, split into the near / far side; `pass` 0 far, 1 near; sweep 0..1 draws it on */
  const orbitPath = (o: Orbit, c: typeof cam, ox: number, oy: number, near: boolean, sweep = 1, steps = 96) => {
    ctx.beginPath();
    let pen = false;
    for (let i = 0; i <= steps * sweep; i++) {
      orbitVec(o, (i / steps) * TAU, tmp);
      project(tmp, c, cx + ox, cy + oy, R, pr);
      if (pr.z >= 0 === near) {
        if (!pen) ctx.moveTo(pr.x + 0.5, pr.y + 0.5);
        else ctx.lineTo(pr.x + 0.5, pr.y + 0.5);
        pen = true;
      } else pen = false;
    }
    ctx.stroke();
  };
  const policyOrbit: Orbit = { a: POLICY_RING.a, e: 0, incl: POLICY_RING.incl, node: POLICY_RING.node, peri: 0, m0: 0, n: 0 };
  const detentT = (k: number) => 0.95 - k * (1.9 / 7);

  function setBubble(text: string, visible: boolean) {
    if (text !== lineText) {
      lineText = text;
      sizer.textContent = text;
      said.textContent = '';
      shown = 0;
      bw = bubble.offsetWidth;
      bh = bubble.offsetHeight;
    }
    if (visible !== bubbleVisible) {
      bubbleVisible = visible;
      bubble.dataset.state = visible ? 'on' : 'off';
    }
  }
  function announce(text: string) {
    if (!section) return;
    const r = section.getBoundingClientRect();
    if (r.top < innerHeight / 2 && r.bottom > innerHeight / 2) setStatus(text);
  }

  // ---- state entry ----
  const targets = (s: HeroState) => ({
    mars: isAutonomous(s) ? 1 : 0,
    policy: s === 'POLICY_EVALUATING' || s === 'POLICY_PERMITTED' || s === 'NEEDS_APPROVAL' || s === 'BLOCKED_POLICY' || s === 'APPROVED' || isAutonomous(s) ? 1 : 0,
    dormant: s === 'DORMANT' || s === 'REST',
  });
  let prevState: HeroState | null = null;
  let seenEpoch = -1;
  function onEnter(prev: HeroState | null, now: HeroState) {
    sClock = 0;
    // the sources land on the visible hemisphere at the moment the scene wakes (never moved once awake)
    if (now === 'BOOT' || prev === null || prev === 'DORMANT' || prev === 'REST') wakeAnchors();
    if (now === 'BOOT') scanAngle = Math.PI * 0.5;
    if (now === 'AUTONOMOUS_INTERACTIVE') {
      focusA.x = nodeAngle(4);
      focusAlpha.x = 0;
    }
    if (targets(now).dormant) hover = -1;
    if (prev !== null) announce(announceFor(snap));
  }
  const nodeAngle = (j: number) => TAU * (phase.x + j * NODE_STEP) + INSPECT[0];

  // settled preview: jump to the end pose
  if (opts.settled) {
    snap = store.get();
    marsS.x = 1;
    policyS.x = 1;
    regionS.x = 0.6;
    rate.x = 0.0015;
    rot = 0.02;
    wakeAnchors();
    phase.x = nodeTarget(4);
    ringDormant = false;
    opX = 0;
  }

  // ---- the frame ----
  const frame = (dtIn: number, tIn: number) => {
    if (!W || !H) return;
    if (!autoTried && !reduced && dtIn > 0) {
      // first real animation frame (the scene is visible): play the story once, by itself
      autoTried = true;
      const s0 = store.get();
      if (s0.state === 'DORMANT' && s0.rev === 0) {
        fastIntro = true;
        store.dispatch({ type: 'AUTOPLAY' });
      }
    }
    const T = reduced ? 1e6 : tIn;
    // intro ramps of the original hero, time-warped (never beyond their own end) when the story starts immediately
    const Ti = reduced ? 1e6 : fastIntro && T < 2400 ? T * 1.8 + 500 : T;
    snap = store.get();
    const st = snap.state;
    if (snap.rev !== seenRev) {
      seenRev = snap.rev;
      stateT = T;
      // only a new state or an explicit restart / jump restarts the state clock (a speed toggle does not)
      if (st !== prevState || snap.vars.epoch !== seenEpoch) onEnter(prevState, st);
      seenEpoch = snap.vars.epoch;
      prevState = st;
    }
    // calm states idle at ~30 fps; the springs still integrate the elapsed time on the next draw
    accDt += dtIn;
    const calm = !reduced && ((st === 'BLOCKED_CONTEXT' && snap.vars.drive === 'manual') || st === 'NEEDS_APPROVAL' || st === 'BLOCKED_POLICY') && !(ptr.on && performance.now() - ptr.t < 1500);
    if (calm && tIn - lastDraw < 30 && lastDraw > 0) return;
    lastDraw = tIn;
    const dt = Math.min(accDt, 64);
    accDt = 0;
    const dtS = dt / 1000;
    const v = snap.vars;
    const tg = targets(st);

    // ---- state clocks and completion events ----
    if (reduced) sClock = 1e6;
    else sClock += dt * (v.accelerate ? 2.6 : 1);
    {
      // completion events of the timed states. A dispatch re-enters this function (the store's listener
      // invalidates the scene), so stop here and let that nested frame draw the new state.
      const resumed = v.staleSource === 'refetched';
      const auto = v.drive === 'auto';
      const fire = (e: HeroEvent) => {
        accDt = dt;
        const before = store.get();
        store.dispatch(e);
        return store.get() !== before;
      };
      let fired = false;
      if (st === 'BOOT' && sClock >= BOOT_MS) fired = fire({ type: 'BOOTED' });
      else if (st === 'CONTEXT_RETRIEVING' && (!reduced || auto) && sClock >= RETRIEVE_MS) fired = fire({ type: 'RETRIEVED' });
      else if (st === 'CONTEXT_VALIDATING') {
        if (!resumed && v.staleEnabled && sClock >= STALE_AT) fired = fire({ type: 'STALE_FOUND' });
        else if (sClock >= validateEnd(resumed)) fired = fire({ type: 'VALIDATED' });
      } else if (st === 'BLOCKED_CONTEXT' && auto && sClock >= AUTO_REFETCH_MS) fired = fire({ type: 'REFETCH' });
      else if (st === 'CONTEXT_READY' && sClock >= READY_MS) fired = fire({ type: 'READY_DONE' });
      else if (st === 'POLICY_EVALUATING' && sClock >= POLICY_MS) fired = fire({ type: 'POLICY_RESULT' });
      else if (st === 'POLICY_PERMITTED' && sClock >= PERMIT_MS) fired = fire({ type: 'BEGIN' });
      else if (st === 'APPROVED') fired = fire({ type: 'BEGIN' });
      else if (st === 'AUTONOMOUS_SEQUENCE' && sClock >= SEQ.total) fired = fire({ type: 'SEQUENCE_DONE' });
      if (fired) return;
    }
    const u = sClock;
    const inDormant = tg.dormant;
    const wasDormantLoop = ringDormant;

    // ---- timeline of the original dormant script (the approved intro) ----
    const print = reduced ? 1 : ease((Ti - 500) / 1000);
    const ringQ = reduced ? 1 : ease((Ti - 650) / 1000);
    const starQ = reduced ? 1 : ease((Ti - 100) / 700);
    const opQ = reduced ? 1 : clamp((Ti - 1250) / 450);

    let dStage = -1;
    let finaleLine = -1;
    let lt = 0;
    let dPhase = 0;
    let probe = 0;
    let idx = 0;
    if (reduced) {
      finaleLine = 1;
      dPhase = -NODE_STEP * 4 + (INSPECT[4] - INSPECT[0]) / TAU;
    } else if (T < T0) {
      dPhase = 0.07 * (1 - ease((T - 1000) / 700));
    } else {
      const uu = T - T0;
      const cyc = Math.floor(uu / CYCLE);
      let r = uu - cyc * CYCLE;
      let k = 0;
      if (r < 5 * BEAT) {
        k = Math.floor(r / BEAT);
        lt = r - k * BEAT;
        dStage = k;
      } else {
        r -= 5 * BEAT;
        finaleLine = Math.floor(r / FIN);
        lt = r - finaleLine * FIN;
        k = 4;
      }
      idx = cyc * 5 + k;
      const target = (i: number) => -NODE_STEP * i + (INSPECT[i % 5] - INSPECT[0]) / TAU;
      if (dStage >= 0 && !(idx === 0)) dPhase = target(idx - 1) + (target(idx) - target(idx - 1)) * ease(lt / ALIGN);
      else if (dStage >= 0) dPhase = target(0);
      else dPhase = target(idx);
      if (dStage >= 0) probe = ease((lt - (idx === 0 ? PROBE_FIRST : PROBE_AFTER)) / 500) * (1 - ease((lt - (BEAT - 420)) / 380));
    }

    // ---- ring phase: the script while dormant; critically damped (velocity-continuous) otherwise ----
    let stage = -1; // active waypoint node 0..4
    let release = 0; // release-identity handoff 0..1
    let seqP = 0;
    if (inDormant) {
      if (wasDormantLoop) {
        phase.v = (dPhase - phase.x) / Math.max(dtS, 1e-3);
        phase.x = dPhase;
      } else {
        stepSpring(phase, dPhase, 4.2, dtS);
        if (Math.abs(phase.x - dPhase) < 4e-4 && Math.abs(phase.v) < 4e-3) ringDormant = true;
      }
      stage = dStage;
    } else {
      if (ringDormant) {
        ringDormant = false; // keep the velocity of the script at the hand-off
      }
      if (st === 'AUTONOMOUS_SEQUENCE') {
        seqP = u / SEQ.total;
        for (let i = 0; i < SEQ.steps.length; i++) if (u >= SEQ.steps[i].at && u < SEQ.steps[i].to) idx = i;
        const stepI = SEQ.steps.findIndex((s) => u >= s.at && u < s.to);
        // waypoint 0..3 map to nodes 0..3, the release handoff keeps node 3 -> 4, evidence is node 4
        const nodeFor = [0, 1, 2, 3, 3, 4];
        stage = stepI < 0 ? (u < SEQ.steps[0].at ? -1 : 4) : nodeFor[stepI];
        if (stage >= 0) {
          const want = nodeTarget(stage);
          const k = Math.round(phase.x - want);
          stepSpring(phase, want + k, 4.6, dtS);
        }
        if (stepI === 4) release = ease((u - SEQ.steps[4].at) / (SEQ.steps[4].to - SEQ.steps[4].at));
        if (stepI === 5) release = 1 - ease((u - SEQ.steps[5].at) / 500);
      } else {
        // calm rhythm: the nodes keep orbiting slowly; speed eases between contexts
        const want = isAutonomous(st) ? -0.012 : -0.007; // turns/s (the script steps backwards too)
        // phase.v is the ring's angular velocity, so hand-offs between position and rate control stay continuous
        phase.v += (want - phase.v) * (1 - Math.exp(-2.5 * dtS));
        phase.x += phase.v * dtS;
        stage = -1;
      }
    }

    // spin of the body: the original rate while dormant, a slow, steady one once awake (anchors stay put)
    {
      const wantRate = inDormant ? 0.008 : 0.0015;
      rotSpring.v += (wantRate - rotSpring.v) * (1 - Math.exp(-3 * dtS));
      rot += rotSpring.v * dtS;
    }
    // morph and presence springs
    if (reduced) {
      marsS.x = tg.mars;
      policyS.x = tg.policy;
    } else {
      stepSpring(marsS, tg.mars, 3.3, dtS);
      stepSpring(policyS, tg.policy, 6, dtS);
    }
    const regTarget = st === 'CONTEXT_VALIDATING' || st === 'BLOCKED_CONTEXT' || st === 'CONTEXT_READY' ? 1 : st === 'POLICY_EVALUATING' || st === 'POLICY_PERMITTED' || st === 'NEEDS_APPROVAL' || st === 'BLOCKED_POLICY' || st === 'APPROVED' ? 0.8 : 0;
    if (reduced) regionS.x = regTarget;
    else stepSpring(regionS, regTarget, 4, dtS);
    const mars = clamp(marsS.x);
    const policyQ = clamp(policyS.x);
    // terminator drift once the sequence has finished
    stepSpring(lightAz, st === 'AUTONOMOUS_INTERACTIVE' && !reduced ? Math.sin(T / 6500) * 0.38 : 0, 1.4, dtS);
    const lAz = lightAz.x;
    const lcs = Math.cos(lAz),
      lsn = Math.sin(lAz);
    const light: [number, number, number] = [BASE_LIGHT[0] * lcs + BASE_LIGHT[2] * lsn, BASE_LIGHT[1], -BASE_LIGHT[0] * lsn + BASE_LIGHT[2] * lcs];
    frameS.rot = rot;

    // ---- pointer: critically damped parallax and a reachable focal reticle ----
    const b = root.getBoundingClientRect();
    let localX = 0,
      localY = 0;
    if (ptr.on) {
      par.tx = clamp(((ptr.x - b.left) / b.width) * 2 - 1, -1, 1);
      par.ty = clamp(((ptr.y - b.top) / b.height) * 2 - 1, -1, 1);
      localX = ptr.x - b.left;
      localY = ptr.y - b.top;
    } else {
      par.tx = 0;
      par.ty = 0;
    }
    stepSpring(par.x, par.tx, 7, dtS);
    stepSpring(par.y, par.ty, 7, dtS);
    const off = (amt: number) => [Math.round(par.x.x * amt), Math.round(par.y.x * amt * 0.7)] as const;
    const [sox, soy] = off(2.5);
    const [rox, roy] = off(5);
    const [cox, coy] = off(7);
    const [oox, ooy] = off(10);
    // focal target: the nearest point of the workflow ring to the pointer, reached by a damped step
    const interactive = st === 'AUTONOMOUS_INTERACTIVE';
    if (interactive) {
      let bestA = focusA.x,
        bd = 1e12;
      if (ptr.on && localX >= -20 && localX <= W + 20 && localY >= -20 && localY <= H + 20) {
        for (let i = 0; i < 72; i++) {
          const a = (i / 72) * TAU;
          const p = ringPoint(a);
          const d = (p.x - localX) ** 2 + (p.y - localY) ** 2;
          if (d < bd) (bd = d), (bestA = a);
        }
        stepAngle(focusA, bestA, 7.5, dtS);
        const reach = clamp(1 - Math.sqrt(bd) / (R * 1.6));
        stepSpring(focusR, reach, 7, dtS);
        stepSpring(focusAlpha, 1, 8, dtS);
      } else {
        stepSpring(focusAlpha, 0, 6, dtS);
      }
    }

    // ---- node geometry ----
    const nodes = STAGES.map((_, j) => {
      const a = nodeAngle(j);
      return { a, ...ringPoint(a) };
    });
    const active = stage;
    const nodeDone = (j: number) => (isAutonomous(st) ? (st === 'AUTONOMOUS_INTERACTIVE' ? true : stage > j || (stage === 4 && u >= SEQ.steps[5].to - 200)) : false);
    // review + release identities occupy the two extra chip slots
    const reviewOn = isAutonomous(st) && (st === 'AUTONOMOUS_INTERACTIVE' || u >= SEQ.steps[3].at);
    const reviewQ = reviewOn ? (st === 'AUTONOMOUS_INTERACTIVE' ? 1 : ease((u - SEQ.steps[3].at) / 400)) : 0;
    const releaseQ = isAutonomous(st) ? policyQ : 0;

    // ---- operator ----
    const crownY = (dx: number) => cy - Math.sqrt(Math.max(0, R * R - dx * dx));
    let targetX = 0;
    let lookX = cx;
    if (inDormant) {
      if (dStage >= 0) {
        const n = nodes[dStage];
        targetX = clamp((n.x - cx) * 0.36, R * 0.18, R * 0.4);
      } else if (finaleLine === 1) targetX = reduced ? 0 : -R * 0.2;
      if (T < T0 && !reduced) targetX = 0;
    } else if (st === 'CONTEXT_RETRIEVING' || st === 'BOOT') {
      let latest = -1;
      for (let i = 0; i < SOURCES.length; i++) if (u >= i * SPAWN_STEP) latest = i;
      if (latest >= 0 && srcPos[latest].on) {
        lookX = srcPos[latest].x;
        targetX = clamp((lookX - cx) * 0.3, -R * 0.3, R * 0.3);
      }
    } else if (st === 'AUTONOMOUS_SEQUENCE' && stage >= 0) {
      const n = nodes[stage];
      lookX = n.x;
      targetX = clamp((n.x - cx) * 0.36, R * 0.18, R * 0.4);
      if (stage === 4 || stage === 1) targetX = clamp((n.x - cx) * 0.36, -R * 0.4, R * 0.4);
    } else if (interactive) {
      const fp = ringPoint(focusA.x);
      lookX = fp.x;
      targetX = focusAlpha.x > 0.15 ? clamp((fp.x - cx) * 0.34, -R * 0.4, R * 0.4) : opX;
    }
    if (reduced) opX = reduced && inDormant ? 0 : targetX;
    else {
      const dx = targetX - opX;
      const step = (dt / 1000) * 64;
      moving = Math.abs(dx) > 0.6 && opQ >= 1;
      if (moving) {
        opX += Math.sign(dx) * Math.min(Math.abs(dx), step);
        opFlip = dx < 0;
        walkClock += dt;
      } else {
        walkClock = 0;
        if (inDormant) {
          if (dStage >= 0) opFlip = nodes[dStage].x < cx + opX;
          else if (finaleLine === 1) opFlip = true;
          else opFlip = false;
        } else opFlip = lookX < cx + opX;
      }
    }
    const opCell = narrow ? 2 : R > 190 ? 4 : 3;
    const opW = OPERATOR_W * opCell;
    const opH = OPERATOR_H * opCell;
    const feetX = cx + opX;
    const feetY = crownY(opX) + 2;
    const opLeft = Math.round(feetX - opW / 2);
    const opTop = Math.round(feetY - opH);
    let opFrame: OperatorFrame = 'idle0';
    if (moving) opFrame = Math.floor(walkClock / 170) % 2 === 0 ? 'walkA' : 'walkB';
    else if (!reduced) {
      const blinkPhase = T % 3600;
      if (blinkPhase > 3380) opFrame = 'blink';
      else opFrame = Math.floor(T / 650) % 2 === 0 ? 'idle0' : 'idle1';
    }

    // ---- speech ----
    let text = '';
    let on = false;
    let start = 350;
    let bkey = '';
    if (inDormant) {
      if (dStage >= 0) {
        text = STAGES[dStage].say;
        on = lt > (idx === 0 ? 350 : 500) && lt < BEAT - 500;
        start = idx === 0 ? 350 : 500;
        bkey = `d${idx}`;
      } else if (finaleLine >= 0) {
        text = FINALE[finaleLine];
        on = reduced || (lt > 350 && lt < FIN - 250);
        bkey = `f${idx}${finaleLine}`;
      }
    } else {
      bkey = `${st}${v.staleSource}${stage}`;
      on = true;
      start = 0;
      if (st === 'BOOT') text = 'context request.';
      else if (st === 'CONTEXT_RETRIEVING') text = 'fetching: scoped sources.';
      else if (st === 'CONTEXT_VALIDATING') text = v.staleSource === 'refetched' ? 'refetch: v5.' : 'checking provenance...';
      else if (st === 'BLOCKED_CONTEXT') text = v.drive === 'auto' ? 'stale source. re-fetching.' : 'stale source. blocked.';
      else if (st === 'CONTEXT_READY') text = 'context ready. not authority.';
      else if (st === 'POLICY_EVALUATING') text = 'ready is not authority.';
      else if (st === 'POLICY_PERMITTED') text = 'policy permits this scope.';
      else if (st === 'NEEDS_APPROVAL') text = 'policy decides. not me.';
      else if (st === 'BLOCKED_POLICY') text = v.approvalState === 'declined' ? 'held. staying supervised.' : 'blocked by policy.';
      else if (st === 'AUTONOMOUS_SEQUENCE') {
        const lines = ['reading bundle...', 'plan: scope bounded', 'act: inside grant', 'verify: reviewer checks', 'handoff: release identity', 'evidence: recorded'];
        const si = SEQ.steps.findIndex((s) => u >= s.at && u < s.to);
        text = si >= 0 ? lines[si] : u < SEQ.steps[0].at ? 'policy permits. go.' : lines[5];
        bkey = `${st}${si}`;
      } else if (st === 'AUTONOMOUS_INTERACTIVE') {
        text = 'check it yourself.';
        on = sClock < 4200 || reduced;
      }
    }
    if (bkey !== beatKey) {
      beatKey = bkey;
      lineStart = inDormant ? T : sClock;
      if (inDormant && finaleLine >= 0 && !reduced) announce(`[illustrative] ${FINALE[finaleLine]}`);
      else if (inDormant && dStage >= 0) announce(`[illustrative] ${STAGES[dStage].status}`);
    }
    if (reduced) {
      const rt = text || FINALE[1];
      setBubble(rt, true);
      said.textContent = rt;
      shown = rt.length;
    } else {
      if (text) setBubble(text, on);
      else if (bubbleVisible) setBubble(lineText, false);
      if (on) {
        const t0 = inDormant ? start : lineStart + start;
        const elapsed = inDormant ? lt - start : sClock - t0;
        // awake states type from the first frame: never an empty bubble
        const n = clamp(Math.floor(elapsed / TYPE_MS) + (inDormant ? 0 : 1), 0, lineText.length);
        if (n !== shown) {
          shown = n;
          said.textContent = lineText.slice(0, n);
        }
      }
    }

    // ---- sources: orbit position, capture, surface pins ----
    const sx0 = cx + cox,
      sy0 = cy + coy;
    const retrievingDone = st !== 'BOOT' && st !== 'CONTEXT_RETRIEVING' && !inDormant;
    for (let i = 0; i < SOURCES.length; i++) {
      const s = SOURCES[i];
      const p = srcPos[i];
      bodyToView(anchors[i], frameS, tmp);
      p.sx = sx0 + tmp[0] * R;
      p.sy = sy0 + tmp[1] * R;
      p.sz = tmp[2];
      p.on = false;
      p.c = 0;
      if (inDormant || st === 'BOOT') continue;
      const spawn = i * SPAWN_STEP;
      const tt = retrievingDone ? 1e6 : u - spawn;
      if (tt < 0) continue;
      p.c = retrievingDone ? 1 : ease((tt - CAPTURE_AT) / CAPTURE_MS);
      const M = s.orbit.m0 + (s.orbit.n * Math.min(tt, CAPTURE_AT + CAPTURE_MS)) / 1000;
      const E = solveKepler(M, s.orbit.e);
      orbitVec(s.orbit, E, tmp2);
      project(tmp2, cam, cx + rox, cy + roy, R, pr);
      const pc = p.c;
      p.x = mix(pr.x, p.sx, pc);
      p.y = mix(pr.y, p.sy, pc);
      p.z = mix(pr.z, p.sz, pc);
      p.on = !(p.z < 0 && Math.hypot(p.x - sx0, p.y - sy0) < R);
      if (pc >= 1) p.on = p.sz > 0.02;
    }

    // ================= draw =================
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    ctx.imageSmoothingEnabled = false;
    ctx.lineWidth = 1;

    for (const s of stars) {
      let a = s.a * starQ;
      if (s.tw >= 0 && !reduced) a *= 0.35 + 0.65 * (0.5 + 0.5 * Math.sin(T / 900 + s.tw));
      if (a < 0.05) continue;
      const x = Math.round((s.x * W + sox) / 2) * 2;
      const y = Math.round((s.y * H + soy) / 2) * 2;
      ctx.globalAlpha = a * (pal.dark ? 0.9 : 0.55);
      ctx.fillStyle = pal.ink;
      if (s.plus) {
        ctx.fillRect(x - 3, y, 7, 1);
        ctx.fillRect(x, y - 3, 1, 7);
      } else ctx.fillRect(x, y, 2, 2);
    }
    ctx.globalAlpha = 1;

    ctx.strokeStyle = pal.lineStrong;
    ctx.globalAlpha = 0.55;
    const m = 14;
    for (const [px, py] of [
      [m, m],
      [W - m, m],
      [m, H - m],
      [W - m, H - m],
    ])
      crosshair(px, py, 4);
    ctx.globalAlpha = 1;

    const gR = R * 1.13;
    ctx.strokeStyle = pal.ink2;
    ctx.fillStyle = pal.ink2;
    ctx.globalAlpha = 0.55 * ringQ;
    for (let d = 0; d < 360; d += 6) {
      const a = (d * Math.PI) / 180;
      const l = d % 30 === 0 ? 6 : 2.5;
      const x1 = cx + rox + Math.cos(a) * gR,
        y1 = cy + roy + Math.sin(a) * gR;
      ctx.beginPath();
      ctx.moveTo(Math.round(x1) + 0.5, Math.round(y1) + 0.5);
      ctx.lineTo(Math.round(x1 + Math.cos(a) * l) + 0.5, Math.round(y1 + Math.sin(a) * l) + 0.5);
      ctx.stroke();
    }
    ctx.globalAlpha = 1;

    // workflow ring, far half
    const sweep = ringQ * TAU;
    ctx.strokeStyle = pal.ink;
    ctx.globalAlpha = 0.8;
    ctx.setLineDash([3, 4]);
    if (ringQ > 0) dashRing(0, sweep, false, rox, roy);
    ctx.setLineDash([]);
    ctx.globalAlpha = 1;

    const items = (front: boolean) => {
      if (ringQ < 1) return;
      const speeds = [0.021, -0.013, 0.034];
      for (let i = 0; i < speeds.length; i++) {
        const a = (reduced ? 0.6 + i * 2.1 : T / 1000) * speeds[i] * TAU + i * 2.2;
        const p = ringPoint(a);
        if (p.front !== front) continue;
        for (let tr = 3; tr >= 0; tr--) {
          const q = ringPoint(a - Math.sign(speeds[i]) * tr * 0.035);
          ctx.globalAlpha = 0.18 + (3 - tr) * 0.2;
          ctx.fillStyle = pal.ink;
          ctx.fillRect(Math.round(q.x + rox) - 1, Math.round(q.y + roy) - 1, tr === 0 ? 3 : 2, tr === 0 ? 3 : 2);
        }
        ctx.globalAlpha = 1;
      }
    };
    items(false);

    const drawNodes = (front: boolean) => {
      if (Ti < 1450 && !reduced) return;
      nodes.forEach((n, j) => {
        nodeScr[j].x = n.x + rox;
        nodeScr[j].y = n.y + roy;
        nodeScr[j].front = n.front;
        nodeScr[j].a = n.a;
        nodeScr[j].on = true;
        if (n.front !== front) return;
        const x = Math.round(n.x + rox),
          y = Math.round(n.y + roy);
        const shownQ = reduced ? 1 : clamp((Ti - 1450 - j * 130) / 300);
        if (shownQ <= 0) return;
        const isActive = j === active;
        const done = nodeDone(j);
        const hue = STAGES[j].hue === 'yellow' ? pal.yellow : pal.indigo;
        ctx.globalAlpha = shownQ;
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - 5, y - 5, 11, 11);
        ctx.strokeStyle = pal.ink;
        ctx.strokeRect(x - 4.5, y - 4.5, 10, 10);
        ctx.fillStyle = (isActive && (inDormant ? probe > 0.9 : true)) || done ? hue : pal.ink;
        ctx.fillRect(x - 1, y - 1, 3, 3);
        if (done && !isActive) {
          ctx.fillStyle = hue;
          ctx.fillRect(x - 3, y - 3, 7, 1);
        }
        if (isActive) {
          const pulse = reduced ? 0 : (T % 1400) / 1400;
          const s = 8 + Math.round(pulse * 6);
          ctx.strokeStyle = hue;
          ctx.globalAlpha = shownQ * (1 - pulse * 0.7);
          for (const [sx, sy] of [
            [-1, -1],
            [1, -1],
            [-1, 1],
            [1, 1],
          ]) {
            ctx.beginPath();
            ctx.moveTo(x + sx * s + 0.5, y + sy * s + 0.5 - sy * 4);
            ctx.lineTo(x + sx * s + 0.5, y + sy * s + 0.5);
            ctx.lineTo(x + sx * s + 0.5 - sx * 4, y + sy * s + 0.5);
            ctx.stroke();
          }
          if (inDormant ? probe > 0.9 : true) {
            ctx.globalAlpha = 1;
            ctx.fillStyle = hue;
            ctx.fillRect(x - 4, y - 4, 9, 2);
          }
        }
        ctx.globalAlpha = 1;
      });
    };
    drawNodes(false);

    // ---- policy ring, far half (external: a solid thin ring on its own inclined plane) ----
    const pOx = rox,
      pOy = roy;
    if (policyQ > 0.01) {
      ctx.strokeStyle = pal.ink2;
      ctx.globalAlpha = 0.55 * policyQ;
      orbitPath(policyOrbit, camPolicy, pOx, pOy, false, clamp(policyQ), 120);
      ctx.globalAlpha = 1;
    }
    // source tracks, far side
    const drawSourceTracks = (front: boolean) => {
      if (inDormant || st === 'BOOT') return;
      for (let i = 0; i < SOURCES.length; i++) {
        const p = srcPos[i];
        const tt = retrievingDone ? 1e6 : u - i * SPAWN_STEP;
        if (tt < 0) continue;
        const fade = retrievingDone ? 0 : 1 - p.c;
        if (fade <= 0.02) continue;
        const s = SOURCES[i];
        // the Kepler ellipse (faint) and the node's recent track (fine, fading)
        ctx.strokeStyle = pal.ink2;
        ctx.globalAlpha = 0.5 * fade;
        ctx.setLineDash([2, 4]);
        orbitPath(s.orbit, cam, rox, roy, front, 1, 90);
        ctx.setLineDash([]);
        const M0 = s.orbit.m0 + (s.orbit.n * Math.min(tt, CAPTURE_AT + CAPTURE_MS)) / 1000;
        for (let k = 1; k <= 12; k++) {
          const E = solveKepler(M0 - (k * 0.045) * Math.sign(s.orbit.n), s.orbit.e);
          orbitVec(s.orbit, E, tmp2);
          project(tmp2, cam, cx + rox, cy + roy, R, pr);
          if (pr.z >= 0 !== front) continue;
          const q = mix(pr.x, p.sx, p.c * (1 - k / 14)),
            q2 = mix(pr.y, p.sy, p.c * (1 - k / 14));
          ctx.globalAlpha = 0.7 * fade * (1 - k / 13);
          ctx.fillStyle = pal.ink;
          ctx.fillRect(Math.round(q) - 1, Math.round(q2) - 1, 2, 2);
        }
        ctx.globalAlpha = 1;
      }
    };
    drawSourceTracks(false);

    // review identity: its own small, retrograde orbit (far half)
    const reviewPos = { x: 0, y: 0, z: 0 };
    if (reviewQ > 0) {
      const M = REVIEW_ORBIT.m0 + (REVIEW_ORBIT.n * (reduced ? 2 : T / 1000));
      const E = solveKepler(M, REVIEW_ORBIT.e);
      orbitVec(REVIEW_ORBIT, E, tmp2);
      project(tmp2, cam, cx + rox, cy + roy, R * 1.0, pr);
      reviewPos.x = pr.x;
      reviewPos.y = pr.y;
      reviewPos.z = pr.z;
      ctx.strokeStyle = pal.indigo;
      ctx.globalAlpha = 0.4 * reviewQ;
      ctx.setLineDash([1, 3]);
      orbitPath(REVIEW_ORBIT, cam, rox, roy, false, 1, 72);
      ctx.setLineDash([]);
      ctx.globalAlpha = 1;
    }

    // ---- the sphere ----
    {
      const hv = hubView;
      bodyToView(hubB, frameS, hv);
      const needs =
        T - lastSphere >= 33 ||
        reduced ||
        print < 1 ||
        (marsS.x > 0.002 && marsS.x < 0.998) ||
        regionS.x > 0.002 ||
        (st === 'BOOT') ||
        Math.abs(lAz) > 0.001;
      if (needs) {
        lastSphere = T;
        if (mars > 0.001) ensureTerrain();
        if (regionS.x > 0.002 && !terrain) ensureTerrain();
        const lightMoved = Math.abs(lAz) > 1e-4;
        // the scan meridian sweeps the disk during BOOT
        const scanQ = st === 'BOOT' ? Math.sin(clamp(u / BOOT_MS) * Math.PI) : 0;
        const sc = st === 'BOOT' ? Math.PI * 0.5 - clamp(u / BOOT_MS) * Math.PI : 0;
        const img = planet.render(surface, terrain, colors, {
          rot,
          print,
          mars,
          region: clamp(regionS.x),
          hub: hv,
          scan: scanQ,
          scanAngle: sc,
          light,
          lightMoved,
        });
        if (img) bctx.putImageData(img, 0, 0);
      }
    }
    ctx.drawImage(buf, 0, 0, N, N, Math.round(cx - (N * cell) / 2) + cox, Math.round(cy - (N * cell) / 2) + coy, N * cell, N * cell);
    if (print > 0.2) {
      ctx.strokeStyle = pal.ink;
      ctx.globalAlpha = 0.9 * clamp(print * 1.5);
      ctx.beginPath();
      ctx.arc(cx + cox + 0.5, cy + coy + 0.5, R, Math.PI * 0.95, Math.PI * 1.75);
      ctx.stroke();
      ctx.globalAlpha = 0.35 * clamp(print * 1.5);
      ctx.beginPath();
      ctx.arc(cx + cox + 0.5, cy + coy + 0.5, R, 0, TAU);
      ctx.stroke();
      ctx.globalAlpha = 1;
    }

    // ---- on-surface context graph: great circles (slerp), pins, validated region, pulses ----
    const surfPt = (b: readonly number[], lift: number, out: { x: number; y: number; z: number }) => {
      bodyToView(b, frameS, tmp);
      out.x = sx0 + tmp[0] * R * lift;
      out.y = sy0 + tmp[1] * R * lift;
      out.z = tmp[2];
    };
    const gp = { x: 0, y: 0, z: 0 };
    /** stroke a great circle between two body vectors from t0..t1; only the visible hemisphere */
    const greatArc = (a: readonly number[], bq: readonly number[], t0: number, t1: number) => {
      const steps = Math.max(6, Math.ceil(angle(a, bq) * 16));
      ctx.beginPath();
      let pen = false;
      for (let i = 0; i <= steps; i++) {
        const t = t0 + ((t1 - t0) * i) / steps;
        slerp(a, bq, t, tmp2);
        const lift = 1 + 0.035 * Math.sin(Math.PI * t);
        surfPt(tmp2, lift, gp);
        if (gp.z > 0.03) {
          if (!pen) ctx.moveTo(gp.x + 0.5, gp.y + 0.5);
          else ctx.lineTo(gp.x + 0.5, gp.y + 0.5);
          pen = true;
        } else pen = false;
      }
      ctx.stroke();
    };
    const arcPoint = (a: readonly number[], bq: readonly number[], t: number, out: { x: number; y: number; z: number }) => {
      slerp(a, bq, t, tmp2);
      surfPt(tmp2, 1 + 0.035 * Math.sin(Math.PI * t), out);
    };
    const pulse = (a: readonly number[], bq: readonly number[], t: number, color: string) => {
      if (t <= 0 || t >= 1) return;
      arcPoint(a, bq, t, gp);
      if (gp.z < 0.03) return;
      ctx.fillStyle = color;
      ctx.fillRect(Math.round(gp.x) - 2, Math.round(gp.y) - 2, 5, 5);
      ctx.globalAlpha = 0.4;
      const t2 = t - 0.06;
      if (t2 > 0) {
        arcPoint(a, bq, t2, gp);
        ctx.fillRect(Math.round(gp.x) - 1, Math.round(gp.y) - 1, 3, 3);
      }
      ctx.globalAlpha = 1;
    };
    const afterRetrieve = !inDormant && st !== 'BOOT' && st !== 'CONTEXT_RETRIEVING';
    const resumedV = v.staleSource === 'refetched';
    const checkPass = (i: number): number => {
      // 0 pending, 1 running, 2 pass, 3 flagged
      if (st === 'CONTEXT_VALIDATING') {
        const [a, bq] = checkWindow(i, resumedV);
        if (u >= bq) return 2;
        if (u >= a) return 1;
        return 0;
      }
      if (st === 'BLOCKED_CONTEXT') return i < FRESH ? 2 : i === FRESH ? 3 : 0;
      return afterRetrieve ? 2 : 0;
    };
    if (!inDormant && st !== 'BOOT') {
      const hubLift = { x: 0, y: 0, z: 0 };
      surfPt(hubB, 1, hubLift);
      // spokes: each accepted source converges onto the ContextBundle
      for (let i = 0; i < SOURCES.length; i++) {
        const accT = st === 'CONTEXT_RETRIEVING' ? u - acceptedAt(i) : 1e6;
        if (accT < 0) continue;
        const prog = st === 'CONTEXT_RETRIEVING' ? ease(accT / 520) : 1;
        const stale = i === STALE_INDEX && v.staleSource === 'flagged';
        ctx.strokeStyle = stale ? pal.orange : pal.ink;
        ctx.globalAlpha = st === 'CONTEXT_RETRIEVING' ? 1 : 0.6;
        ctx.setLineDash(stale ? [2, 3] : []);
        greatArc(anchors[i], hubB, 0, prog);
        ctx.setLineDash([]);
      }
      ctx.globalAlpha = 1;
      // the geodesic ring between sources, drawn as the checks pass
      if (st === 'CONTEXT_VALIDATING' || st === 'BLOCKED_CONTEXT' || afterRetrieve) {
        for (let i = 0; i < SOURCES.length; i++) {
          const j = (i + 1) % SOURCES.length;
          const prog = st === 'CONTEXT_VALIDATING' ? ease((u - (resumedV ? -400 : i * 200)) / 500) : 1;
          if (prog <= 0) continue;
          ctx.strokeStyle = pal.paper;
          ctx.globalAlpha = 0.8;
          ctx.lineWidth = 3;
          greatArc(anchors[i], anchors[j], 0, prog);
          ctx.lineWidth = 1;
          ctx.strokeStyle = pal.yellow;
          ctx.globalAlpha = 1;
          greatArc(anchors[i], anchors[j], 0, prog);
        }
        ctx.globalAlpha = 1;
      }
      // validated region: a dotted small circle around the bundle that becomes precise
      if (regionS.x > 0.02) {
        ctx.strokeStyle = pal.yellow;
        ctx.globalAlpha = 0.8 * clamp(regionS.x);
        ctx.setLineDash([2, 3]);
        ctx.beginPath();
        const rho = 0.7;
        // orthonormal basis around the hub (body frame)
        const up: [number, number, number] = Math.abs(hubB[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
        const e1x = hubB[1] * up[2] - hubB[2] * up[1],
          e1y = hubB[2] * up[0] - hubB[0] * up[2],
          e1z = hubB[0] * up[1] - hubB[1] * up[0];
        const l1 = Math.hypot(e1x, e1y, e1z);
        const ax = e1x / l1,
          ay = e1y / l1,
          az = e1z / l1;
        const bxv = hubB[1] * az - hubB[2] * ay,
          byv = hubB[2] * ax - hubB[0] * az,
          bzv = hubB[0] * ay - hubB[1] * ax;
        let pen = false;
        for (let i = 0; i <= 72; i++) {
          const t = (i / 72) * TAU;
          const c = Math.cos(rho),
            s = Math.sin(rho);
          tmp2[0] = hubB[0] * c + (ax * Math.cos(t) + bxv * Math.sin(t)) * s;
          tmp2[1] = hubB[1] * c + (ay * Math.cos(t) + byv * Math.sin(t)) * s;
          tmp2[2] = hubB[2] * c + (az * Math.cos(t) + bzv * Math.sin(t)) * s;
          surfPt(tmp2, 1, gp);
          if (gp.z > 0.03) {
            if (!pen) ctx.moveTo(gp.x + 0.5, gp.y + 0.5);
            else ctx.lineTo(gp.x + 0.5, gp.y + 0.5);
            pen = true;
          } else pen = false;
        }
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.globalAlpha = 1;
      }
      // logical-event pulses only
      if (st === 'CONTEXT_VALIDATING') {
        for (let i = 0; i < CHECKS.length; i++) {
          const [a, bq] = checkWindow(i, resumedV);
          if (u < a || u > bq + 100) continue;
          const k = i % SOURCES.length;
          const kk = (k + 1) % SOURCES.length;
          if (i === FRESH && resumedV) pulse(hubB, anchors[STALE_INDEX], (u - a) / (bq - a), pal.orange);
          else pulse(anchors[k], anchors[kk], (u - a) / (bq - a), pal.yellow);
        }
      }
      if (st === 'CONTEXT_RETRIEVING') {
        for (let i = 0; i < SOURCES.length; i++) {
          const accT = u - acceptedAt(i);
          if (accT >= 0 && accT < 520) pulse(anchors[i], hubB, accT / 520, pal.yellow);
        }
      }
      // pins (accepted sources)
      for (let i = 0; i < SOURCES.length; i++) {
        const p = srcPos[i];
        if (!p.on || p.c < 1) continue;
        const x = Math.round(p.sx),
          y = Math.round(p.sy);
        const stale = i === STALE_INDEX && v.staleSource === 'flagged';
        const isHover = hoverSrc === i;
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - 6, y - 6, 13, 13);
        ctx.strokeStyle = stale ? pal.orange : pal.ink;
        ctx.strokeRect(x - 5.5, y - 5.5, 12, 12);
        ctx.fillStyle = stale ? pal.orange : pal.yellow;
        ctx.fillRect(x - 2, y - 2, 5, 5);
        if (isHover) {
          ctx.strokeStyle = pal.yellow;
          crosshair(x, y, 9);
        }
      }
      // the bundle node
      if (afterRetrieve || (st === 'CONTEXT_RETRIEVING' && u > acceptedAt(SOURCES.length - 1))) {
        surfPt(hubB, 1, gp);
        if (gp.z > 0.03) {
          const x = Math.round(gp.x),
            y = Math.round(gp.y);
          ctx.strokeStyle = pal.ink;
          ctx.fillStyle = pal.yellow;
          ctx.fillRect(x - 3, y - 3, 7, 7);
          ctx.strokeRect(x - 3.5, y - 3.5, 7, 7);
        }
      }
    }

    // ---- ring front, with a paper underlay ----
    ctx.strokeStyle = pal.paper;
    ctx.globalAlpha = 0.9;
    ctx.lineWidth = 3;
    if (ringQ > 0) dashRing(0, sweep, true, rox, roy);
    ctx.lineWidth = 1;
    ctx.strokeStyle = pal.ink;
    ctx.globalAlpha = 1;
    ctx.setLineDash([3, 4]);
    if (ringQ > 0) dashRing(0, sweep, true, rox, roy);
    ctx.setLineDash([]);
    // autonomous progress: the travelled arc of the workflow ring is lit
    if (isAutonomous(st) && mars > 0.05) {
      ctx.strokeStyle = pal.orange;
      ctx.globalAlpha = 0.95 * clamp(mars * 1.3);
      ctx.lineWidth = 1;
      const a0 = nodeAngle(0);
      const upto = st === 'AUTONOMOUS_INTERACTIVE' ? nodeAngle(4) : stage >= 0 ? nodeAngle(stage) : a0;
      const span = Math.max(0, Math.min(TAU, ((a0 - upto) % TAU + TAU) % TAU));
      // the nodes are laid out at decreasing angles, so travel from the last back to the first
      ctx.setLineDash([3, 4]);
      if (span > 0.001) {
        dashRing(upto, upto + span, true, rox, roy);
        dashRing(upto, upto + span, false, rox, roy);
      }
      ctx.setLineDash([]);
      ctx.globalAlpha = 1;
    }
    items(true);
    drawNodes(true);
    drawSourceTracks(true);

    // sources in flight (markers, not yet landed): front + far drawn together; the sphere hides the far ones
    if (!inDormant && st !== 'BOOT')
      for (let i = 0; i < SOURCES.length; i++) {
        const p = srcPos[i];
        if (!p.on || p.c >= 1) continue;
        const x = Math.round(p.x),
          y = Math.round(p.y);
        const sz = p.z < 0 ? 4 : 5 + Math.round(p.z * 1.5);
        const appear = clamp((u - i * SPAWN_STEP) / 160);
        if (p.z < 0 && Math.hypot(p.x - sx0, p.y - sy0) < R) continue;
        ctx.globalAlpha = appear;
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - sz - 1, y - sz - 1, sz * 2 + 3, sz * 2 + 3);
        ctx.strokeStyle = pal.ink;
        ctx.strokeRect(x - sz - 0.5, y - sz - 0.5, sz * 2 + 2, sz * 2 + 2);
        ctx.fillStyle = pal.yellow;
        ctx.fillRect(x - 1, y - 1, 3, 3);
        ctx.globalAlpha = 1;
        if (p.c < 0.5 && st === 'CONTEXT_RETRIEVING') {
          const dx = x - cx,
            dy = y - cy;
          const dl = Math.hypot(dx, dy) || 1;
          tag(SOURCES[i].label, x + (dx / dl) * 16, y + (dy / dl) * 10 - 8, dx > 0 ? 'left' : 'right', pal.ink, pal.paper, pal.lineStrong);
        }
      }

    // ---- policy ring, near half, ladder detents, markers (outside the model) ----
    if (policyQ > 0.01) {
      ctx.strokeStyle = pal.paper;
      ctx.globalAlpha = 0.9 * policyQ;
      ctx.lineWidth = 3;
      orbitPath(policyOrbit, camPolicy, pOx, pOy, true, clamp(policyQ), 120);
      ctx.lineWidth = 1;
      ctx.strokeStyle = pal.ink;
      ctx.globalAlpha = policyQ;
      orbitPath(policyOrbit, camPolicy, pOx, pOy, true, clamp(policyQ), 120);
      // detents: eight ladder levels along the right arc
      const det = (k: number, out: Proj) => {
        orbitVec(policyOrbit, detentT(k), tmp2);
        project(tmp2, camPolicy, cx + pOx, cy + pOy, R, out);
      };
      const dp: Proj = { x: 0, y: 0, z: 0 };
      for (let k = 0; k < LEVELS.length; k++) {
        det(k, dp);
        const x = Math.round(dp.x),
          y = Math.round(dp.y);
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - 3, y - 3, 7, 7);
        ctx.strokeStyle = pal.ink;
        ctx.strokeRect(x - 2.5, y - 2.5, 6, 6);
      }
      const reqI = levelIndex(v.requestedAutonomy);
      const effI = levelIndex(v.effectiveAutonomy);
      const capI = levelIndex(v.policyCap);
      const decided = v.policyDecision !== 'none' && v.policyDecision !== 'pending' && (st !== 'POLICY_EVALUATING' || u > POLICY_MS - 350);
      const showReq = st !== 'POLICY_EVALUATING' || u > 250;
      if (showReq) {
        det(reqI, dp);
        const x = Math.round(dp.x),
          y = Math.round(dp.y);
        ctx.strokeStyle = pal.yellow;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(x, y - 8);
        ctx.lineTo(x + 8, y);
        ctx.lineTo(x, y + 8);
        ctx.lineTo(x - 8, y);
        ctx.closePath();
        ctx.stroke();
        pixelText('REQ', x - 12, y + 4, pal.ink, 'right');
      }
      if (decided) {
        det(capI, dp);
        ctx.fillStyle = pal.yellow;
        ctx.fillRect(Math.round(dp.x) - 6, Math.round(dp.y) - 1, 13, 2);
        det(effI, dp);
        const x = Math.round(dp.x),
          y = Math.round(dp.y);
        ctx.fillStyle = pal.ink;
        ctx.fillRect(x - 4, y - 4, 9, 9);
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - 1, y - 1, 3, 3);
        pixelText('EFF', x - 12, y + 4, pal.ink, 'right');
      }
      // gate bracket: pulsing while the human decision is pending, orange when held
      if (st === 'NEEDS_APPROVAL' || st === 'BLOCKED_POLICY' || st === 'POLICY_EVALUATING' || st === 'POLICY_PERMITTED') {
        det(Math.min(reqI, 7), dp);
        const held = st === 'BLOCKED_POLICY';
        const pu = reduced ? 0.4 : (T % 1200) / 1200;
        const s = 11 + Math.round(pu * 5);
        ctx.strokeStyle = held ? pal.orange : pal.yellow;
        ctx.globalAlpha = held ? 1 : 1 - pu * 0.6;
        for (const [sx, sy] of [
          [-1, -1],
          [1, -1],
          [-1, 1],
          [1, 1],
        ]) {
          ctx.beginPath();
          ctx.moveTo(dp.x + sx * s + 0.5, dp.y + sy * s + 0.5 - sy * 4);
          ctx.lineTo(dp.x + sx * s + 0.5, dp.y + sy * s + 0.5);
          ctx.lineTo(dp.x + sx * s + 0.5 - sx * 4, dp.y + sy * s + 0.5);
          ctx.stroke();
        }
      }
      // grant assessment: a token travels bundle -> gate and the verdict returns
      if (st === 'POLICY_EVALUATING' && !reduced) {
        det(reqI, dp);
        const p1 = clamp((u - 250) / 600),
          p2 = clamp((u - 850) / 500);
        const h = { x: 0, y: 0, z: 0 };
        surfPt(hubB, 1, h);
        const f = p2 > 0 ? 1 - p2 : p1;
        if (p1 > 0 && (p1 < 1 || p2 > 0) && p2 < 1) {
          const x = mix(h.x, dp.x, ease(f)),
            y = mix(h.y, dp.y, ease(f)) - Math.sin(ease(f) * Math.PI) * 18;
          ctx.fillStyle = p2 > 0 ? pal.ink : pal.yellow;
          ctx.fillRect(Math.round(x) - 2, Math.round(y) - 2, 5, 5);
        }
      }
      // external gate label
      det(7, dp);
      ctx.globalAlpha = policyQ;
      ctx.strokeStyle = pal.ink2;
      ctx.beginPath();
      ctx.moveTo(Math.round(dp.x) + 0.5, Math.round(dp.y) + 0.5);
      ctx.lineTo(Math.round(dp.x) + 0.5, Math.round(dp.y) - 14.5);
      ctx.stroke();
      tag(st === 'POLICY_PERMITTED' ? 'POLICY PERMITS / external' : 'POLICY / external', dp.x, dp.y - 31, 'right', pal.ink, pal.paper, pal.lineStrong);
      ctx.globalAlpha = 1;
    }

    // release identity: parked on the external ring, separate from the author orbit; review on its own
    const releasePos = { x: 0, y: 0 };
    if (releaseQ > 0.02) {
      orbitVec(policyOrbit, -2.35, tmp2);
      project(tmp2, camPolicy, cx + pOx, cy + pOy, R, pr);
      releasePos.x = pr.x;
      releasePos.y = pr.y;
      const x = Math.round(pr.x),
        y = Math.round(pr.y);
      ctx.globalAlpha = releaseQ;
      ctx.fillStyle = release > 0.05 ? pal.orange : pal.paper;
      ctx.fillRect(x - 6, y - 6, 13, 13);
      ctx.strokeStyle = pal.ink;
      ctx.strokeRect(x - 5.5, y - 5.5, 12, 12);
      ctx.fillStyle = pal.ink;
      ctx.fillRect(x - 2, y - 2, 5, 5);
      ctx.globalAlpha = 1;
      // handoff: a dashed bridge from the release identity to the bundle on the surface
      if (release > 0.01 && stage >= 3) {
        const h = { x: 0, y: 0, z: 0 };
        surfPt(hubB, 1, h);
        ctx.strokeStyle = pal.orange;
        ctx.setLineDash([3, 3]);
        ctx.lineDashOffset = reduced ? 0 : -(T / 50) % 6;
        ctx.beginPath();
        ctx.moveTo(x + 0.5, y + 0.5);
        ctx.lineTo(Math.round(mix(x, h.x, release)) + 0.5, Math.round(mix(y, h.y, release)) + 0.5);
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.lineDashOffset = 0;
      }
    }
    if (reviewQ > 0) {
      const x = Math.round(reviewPos.x),
        y = Math.round(reviewPos.y);
      if (!(reviewPos.z < 0 && Math.hypot(reviewPos.x - sx0, reviewPos.y - sy0) < R)) {
        ctx.globalAlpha = reviewQ;
        ctx.fillStyle = pal.indigo;
        ctx.beginPath();
        ctx.moveTo(x, y - 6);
        ctx.lineTo(x + 6, y);
        ctx.lineTo(x, y + 6);
        ctx.lineTo(x - 6, y);
        ctx.closePath();
        ctx.fill();
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - 1, y - 1, 3, 3);
        ctx.globalAlpha = 1;
        // independent check on the VERIFY waypoint: a separate probe, not the operator's
        if (st === 'AUTONOMOUS_SEQUENCE' && stage === 3 && u > SEQ.steps[3].at + 450) {
          const n = nodes[3];
          const pp = ease((u - SEQ.steps[3].at - 450) / 500);
          ctx.strokeStyle = pal.indigo;
          ctx.setLineDash([2, 3]);
          ctx.beginPath();
          ctx.moveTo(x + 0.5, y + 0.5);
          ctx.lineTo(Math.round(mix(x, n.x + rox, pp)) + 0.5, Math.round(mix(y, n.y + roy, pp)) + 0.5);
          ctx.stroke();
          ctx.setLineDash([]);
        }
      }
    }
    // evidence: a hash-linked chain from the bundle to the EVIDENCE node
    if (st === 'AUTONOMOUS_SEQUENCE' && u > SEQ.steps[5].at) {
      const h = { x: 0, y: 0, z: 0 };
      surfPt(hubB, 1, h);
      const n = nodes[4];
      const q = ease((u - SEQ.steps[5].at) / 700);
      ctx.fillStyle = pal.yellow;
      for (let k = 0; k < 4; k++) {
        const f = clamp(q * 1.2 - k * 0.12);
        if (f <= 0) continue;
        const x = mix(h.x, n.x + rox, f),
          y = mix(h.y, n.y + roy, f) - Math.sin(f * Math.PI) * 12;
        ctx.fillRect(Math.round(x) - 2, Math.round(y) - 2, 5, 5);
      }
    }

    // ---- focal reticle (interactive) ----
    if (interactive && focusAlpha.x > 0.02) {
      const fp = ringPoint(focusA.x);
      const fx = fp.x + rox,
        fy = fp.y + roy;
      const ox = (localX - fx) * 0.12 * focusR.x,
        oy = (localY - fy) * 0.12 * focusR.x;
      const x = fx + clamp(ox, -R * 0.1, R * 0.1),
        y = fy + clamp(oy, -R * 0.1, R * 0.1);
      ctx.globalAlpha = clamp(focusAlpha.x);
      ctx.strokeStyle = pal.orange;
      ctx.beginPath();
      ctx.arc(Math.round(x) + 0.5, Math.round(y) + 0.5, 9, 0, TAU);
      ctx.stroke();
      crosshair(x, y, 14);
      ctx.globalAlpha = 1;
    }

    // ---- operator ----
    if (opQ > 0) {
      const rows = Math.ceil(OPERATOR_H * opQ);
      ctx.save();
      ctx.beginPath();
      ctx.rect(opLeft + oox - 2, opTop + ooy + opH - rows * opCell, opW + 4, rows * opCell + 4);
      ctx.clip();
      ctx.fillStyle = pal.ink;
      ctx.globalAlpha = 0.25;
      ctx.fillRect(opLeft + oox + opCell, opTop + ooy + opH, opW - opCell * 2, 2);
      ctx.globalAlpha = 1;
      drawOperator(ctx, opLeft + oox, opTop + ooy, opCell, opFrame, pal, opFlip);
      ctx.restore();
    }
    // probe from the operator to the active node
    const probeQ = inDormant ? probe : st === 'AUTONOMOUS_SEQUENCE' && stage >= 0 ? ease((u - (SEQ.steps.find((s) => u >= s.at && u < s.to)?.at ?? 0) - 250) / 450) : 0;
    const probeNode = inDormant ? stage : stage;
    if (probeNode >= 0 && probeQ > 0.01) {
      const n = nodes[probeNode];
      const hx = opLeft + oox + (opFlip ? 0 : opW);
      const hy = opTop + ooy + 9 * opCell;
      const tx = n.x + rox,
        ty = n.y + roy;
      const px = hx + (tx - hx) * probeQ,
        py = hy + (ty - hy) * probeQ;
      const hue = STAGES[probeNode].hue === 'yellow' ? pal.yellow : pal.indigo;
      ctx.strokeStyle = hue;
      ctx.setLineDash([3, 3]);
      ctx.lineDashOffset = reduced ? 0 : -(T / 60) % 6;
      ctx.beginPath();
      ctx.moveTo(Math.round(hx) + 0.5, Math.round(hy) + 0.5);
      ctx.lineTo(Math.round(px) + 0.5, Math.round(py) + 0.5);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.lineDashOffset = 0;
      ctx.fillStyle = hue;
      ctx.fillRect(Math.round(px) - 2, Math.round(py) - 2, 5, 5);
    }

    // ---- readouts and the validation checklist ----
    const ink2 = pal.ink2;
    if (ringQ >= 0.6) {
      ctx.globalAlpha = clamp((ringQ - 0.6) * 3);
      const rd = ((((rot % 1) + 1) % 1) * 360).toFixed(1).padStart(5, '0');
      pixelText(`CORE / ROT ${rd}°`, 28, 33, ink2);
      if (interactive && focusAlpha.x > 0.3) {
        const fp = ringPoint(focusA.x);
        const r = Math.hypot(fp.x - cx, (fp.y - cy) / 0.8) / R;
        const th = (Math.atan2(-(fp.y - cy), fp.x - cx) * 180) / Math.PI;
        pixelText(`FOCUS r=${r.toFixed(2)} / θ=${th.toFixed(1)}°`, W - 28, 33, ink2, 'right');
      } else if (active >= 0) {
        const n = nodes[active];
        const r = Math.hypot(n.x - cx, (n.y - cy) / 0.8) / R;
        const th = (Math.atan2(-(n.y - cy), n.x - cx) * 180) / Math.PI;
        pixelText(`r=${r.toFixed(2)} / θ=${th.toFixed(1)}°`, W - 28, 33, ink2, 'right');
        ctx.strokeStyle = ink2;
        ctx.globalAlpha = 0.5;
        crosshair(n.x + rox, n.y + roy, 14);
        ctx.globalAlpha = 1;
      } else {
        pixelText(`r=1.34 / θ=${(((INSPECT[4] * 180) / Math.PI) % 360).toFixed(1)}°`, W - 28, 33, ink2, 'right');
      }
      ctx.globalAlpha = 1;
    }
    // checklist: provenance / permission / freshness / dedupe / conflicts / blocking questions
    if (st === 'CONTEXT_RETRIEVING') {
      let n = 0;
      for (let i = 0; i < SOURCES.length; i++) if (u >= acceptedAt(i)) n++;
      pixelText(`receipts ${n}/${SOURCES.length}`, narrow ? 14 : 28, 54, ink2);
    }
    if (st === 'CONTEXT_VALIDATING' || st === 'BLOCKED_CONTEXT' || st === 'CONTEXT_READY') {
      const rows = narrow ? 1 : CHECKS.length;
      let first = 0;
      if (narrow) {
        first = 0;
        for (let i = 0; i < CHECKS.length; i++) if (checkPass(i) === 1 || checkPass(i) === 3) first = i;
        if (st !== 'BLOCKED_CONTEXT' && checkPass(CHECKS.length - 1) === 2) first = CHECKS.length - 1;
      }
      const x0 = narrow ? 14 : 28;
      for (let r = 0; r < rows; r++) {
        const i = first + r;
        const pass = checkPass(i);
        const y = 54 + r * 15;
        const col = pass === 3 ? pal.orange : pass === 2 ? pal.ink : ink2;
        const mark = pass === 2 ? '+' : pass === 3 ? '!' : pass === 1 ? '>' : '-';
        pixelText(`${mark} ${CHECKS[i]}${pass === 3 ? ' / STALE' : ''}`, x0, y, col);
      }
      if (st === 'CONTEXT_READY') pixelText('= CONTEXT READY / scoped bundle', x0, 54 + rows * 15, pal.ink);
    }
    // source receipts (ref + version) beside accepted pins
    if (!inDormant && st !== 'BOOT') {
      for (let i = 0; i < SOURCES.length; i++) {
        const p = srcPos[i];
        if (!p.on || p.c < 1) continue;
        const stale = i === STALE_INDEX && v.staleSource === 'flagged';
        const accAge = st === 'CONTEXT_RETRIEVING' ? u - acceptedAt(i) : 1e6;
        const showAlways = !narrow;
        const justNow = accAge < 900;
        if (!(showAlways || justNow || stale || hoverSrc === i)) continue;
        const ver = i === STALE_INDEX && v.staleSource === 'refetched' ? 'v5' : SOURCES[i].version;
        const dig = i === STALE_INDEX && v.staleSource === 'refetched' ? REFETCH_DIGEST : DIGESTS[i];
        const label = stale ? `${SOURCES[i].label} / STALE ${ver}` : `${SOURCES[i].label} / ${ver} ${dig}`;
        const dx = p.sx - sx0,
          dy = p.sy - sy0;
        const dl = Math.hypot(dx, dy) || 1;
        if (!(isAutonomous(st) && !stale && hoverSrc !== i)) {
          ctx.globalAlpha = stale ? 1 : st === 'CONTEXT_RETRIEVING' ? clamp(1 - (accAge - 700) / 400) * 0.95 + 0.05 : 0.7;
          if (accAge > 1100 || st !== 'CONTEXT_RETRIEVING') ctx.globalAlpha = stale ? 1 : hoverSrc === i ? 1 : 0.0;
          if (ctx.globalAlpha > 0.05)
            tag(label, p.sx + (dx / dl) * 16, p.sy + (dy / dl) * 12 - 7, dx > 0 ? 'left' : 'right', stale ? pal.orange : pal.ink, pal.paper, stale ? pal.orange : pal.lineStrong);
          ctx.globalAlpha = 1;
        }
      }
    }

    // ---- DOM: chips, tooltip, bubble ----
    const interactiveChips = interactive;
    chips.forEach((chip, j) => {
      let x: number, y: number, front: boolean, hiddenNode = false, appearQ = 1;
      if (j < 5) {
        const n = nodes[j];
        x = n.x + rox;
        y = n.y + roy;
        front = n.front;
        hiddenNode = hiddenBehind(x, y, n.front);
        appearQ = reduced ? 1 : clamp((Ti - 1550 - j * 130) / 250);
      } else if (j === 5) {
        x = reviewPos.x;
        y = reviewPos.y;
        front = reviewPos.z >= 0;
        hiddenNode = reviewQ < 0.6 || (reviewPos.z < 0 && Math.hypot(x - sx0, y - sy0) < R + 6);
      } else {
        x = releasePos.x;
        y = releasePos.y;
        front = true;
        hiddenNode = releaseQ < 0.6;
      }
      const isAct = j < 5 ? j === active && (inDormant ? probe > 0.05 : true) : j === 5 ? st === 'AUTONOMOUS_SEQUENCE' && stage === 3 && u > SEQ.steps[3].at + 450 : release > 0.05;
      const wkey = `${narrow}${isAct}`;
      if (chip.dataset.k !== wkey) {
        chip.dataset.k = wkey;
        chip.dataset.compact = String(narrow);
        chip.dataset.active = String(isAct);
        chip.dataset.w = String(chip.offsetWidth);
      }
      const w = Number(chip.dataset.w) || 80;
      const half = w / 2;
      const cxp = clamp(x, half + 6, W - half - 6);
      const up = j < 5 ? !front : j === 5 ? true : true;
      chip.style.transform = `translate(${Math.round(cxp)}px, ${Math.round(y + (up ? -16 : 16))}px) translate(-50%, ${up ? '-100%' : '0'})`;
      const vis = appearQ >= 1 && !hiddenNode;
      const ds = vis ? 'on' : 'off';
      if (chip.dataset.state !== ds) chip.dataset.state = ds;
      chip.dataset.hue = j < 5 ? STAGES[j].hue : j === 5 ? 'indigo' : 'yellow';
      const ti = interactiveChips && vis ? 0 : -1;
      if (chip.tabIndex !== ti) chip.tabIndex = ti;
      const pe = interactiveChips && vis ? 'auto' : 'none';
      if (chip.style.pointerEvents !== pe) chip.style.pointerEvents = pe;
    });
    // one tooltip: chip hover / focus, the focal reticle, or a pinned source
    let tipOn = false;
    if (hover >= 0 && chips[hover]?.dataset.state === 'on') {
      const n = hover < 5 ? nodes[hover] : null;
      const hx = hover < 5 ? n!.x + rox : hover === 5 ? reviewPos.x : releasePos.x;
      const hy = hover < 5 ? n!.y + roy : hover === 5 ? reviewPos.y : releasePos.y;
      showTip(tipText(hover, false), hx, hy + (hover < 5 && n!.front ? 40 : -48));
      tipOn = true;
    } else if (hoverSrc >= 0 && srcPos[hoverSrc].on) {
      showTip(tipText(hoverSrc, true), srcPos[hoverSrc].sx, srcPos[hoverSrc].sy + 16);
      tipOn = true;
    } else if (interactive && focusAlpha.x > 0.6 && !reduced) {
      const fp = ringPoint(focusA.x);
      let bj = -1,
        bd2 = 26 * 26;
      for (let j = 0; j < 5; j++) {
        const d = (nodes[j].x + rox - fp.x - rox) ** 2 + (nodes[j].y - fp.y) ** 2;
        if (d < bd2) (bd2 = d), (bj = j);
      }
      if (bj >= 0) {
        showTip(tipText(bj, false), nodes[bj].x + rox, nodes[bj].y + roy + (nodes[bj].front ? 40 : -48));
        tipOn = true;
      }
    }
    if (!tipOn) hideTip();

    if (bubbleVisible || bubble.dataset.state === 'on') {
      const bx = clamp(feetX + oox - bw * 0.35, 6, W - bw - 6);
      const by = opTop + ooy - bh - 6;
      bubble.style.transform = `translate(${Math.round(bx)}px, ${Math.max(4, Math.round(by))}px)`;
      const tail = clamp(feetX + oox - bx, 14, bw - 20);
      bubble.style.setProperty('--tail', `${Math.round(tail)}px`);
    }
    void seqP;
  };

  return {
    frame,
    cleanup: () => {
      clearTimeout(buildTimer);
      ro.disconnect();
      stopTheme();
      unsubStore();
      chipHandlers.forEach((f) => f());
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerdown', onDown);
      document.documentElement.removeEventListener('pointerleave', onLeave);
      root.removeEventListener('pointermove', onCanvasMove);
      canvas.removeEventListener('contextlost', onCtxLost);
      canvas.removeEventListener('contextrestored', onCtxBack);
    },
  };
}
