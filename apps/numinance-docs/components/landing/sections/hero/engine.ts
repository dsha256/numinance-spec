import { BAYER4 } from '@/lib/landing/dither';
import { drawOperator, OPERATOR_H, OPERATOR_W, type OperatorFrame } from '@/lib/landing/operator';
import { onThemeChange, readPalette, rgb, type Palette } from '@/lib/landing/palette';
import { mulberry32 } from '@/lib/landing/rng';
import { scopedStatus } from '@/lib/landing/status';
const setStatus = scopedStatus('numinance');
import { fitCanvas } from '@/lib/landing/use-scene';
import { buildSurface, TH, TW, type Surface } from './texture';

export const STAGES = [
  { id: 'understand', label: 'UNDERSTAND', say: 'reading repository…', status: 'understand: reading repository…', hue: 'yellow' },
  { id: 'plan', label: 'PLAN', say: 'plan: scope bounded', status: 'plan: scope bounded', hue: 'indigo' },
  { id: 'act', label: 'ACT', say: 'act: policy permits', status: 'act: policy permits', hue: 'yellow' },
  { id: 'verify', label: 'VERIFY', say: 'verify: checks green', status: 'verify: checks green', hue: 'indigo' },
  { id: 'evidence', label: 'EVIDENCE', say: 'evidence: recorded', status: 'evidence: recorded', hue: 'yellow' },
] as const;

const FINALE = ['verified? not by me.', 'check it yourself.'] as const;

/** where each node sits on the ring (radians, 0 = ring's right end, negative = up/back) when inspected */
const INSPECT = [-0.3, -0.7, -0.45, -0.65, -0.25];
const NODE_STEP = 0.2; // turns between nodes
// Boot beats (ms from the scene's first frame): stars 100, sphere 500-1500, ring 650, operator 1250-1700, chips ~1550-2000.
// The first beat starts as the operator lands, with no hold; later beats walk to the next node (ALIGN) and probe
// as soon as the ring has settled. Measured against Fumadocs' hero (first line of speech ~1.9s after boot).
const T0 = 1700; // ms: first beat
const BEAT = 4200;
const ALIGN = 1100;
const FIN = 2600;
const PROBE_FIRST = 800; // first beat: nothing to align, probe at once
const PROBE_AFTER = ALIGN + 400;
const CYCLE = 5 * BEAT + 2 * FIN;
const TYPE_MS = 38;

const TAU = Math.PI * 2;
const clamp = (v: number, a = 0, b = 1) => Math.min(b, Math.max(a, v));
const ease = (v: number) => (v = clamp(v)) * v * (3 - 2 * v);
const wrap = (a: number) => ((a + Math.PI) % TAU + TAU) % TAU - Math.PI;

interface Star {
  x: number;
  y: number;
  s: number;
  a: number;
  tw: number; // twinkle phase or -1
  plus: boolean;
}

export interface HeroEngine {
  frame: (dt: number, t: number) => void;
  cleanup: () => void;
}

export function createHero(root: HTMLElement, opts: { reduced: boolean; invalidate: () => void }): HeroEngine {
  const reduced = opts.reduced;
  const canvas = root.querySelector('canvas') as HTMLCanvasElement;
  const chips = Array.from(root.querySelectorAll<HTMLElement>('[data-node]'));
  const bubble = root.querySelector<HTMLElement>('[data-bubble]')!;
  const sizer = bubble.querySelector<HTMLElement>('[data-sizer]')!;
  const said = bubble.querySelector<HTMLElement>('[data-said]')!;
  const section = root.closest('section');

  const surface: Surface = buildSurface();
  const buf = document.createElement('canvas');
  const bctx = buf.getContext('2d')!;

  let pal: Palette = readPalette();
  let cDot = rgb(pal.ink);
  let cBase = rgb(pal.paper2);
  let cIndigo = rgb(pal.indigo);
  let cYellow = rgb(pal.yellow);
  let cPaper = rgb(pal.paper);
  const refreshPalette = () => {
    pal = readPalette();
    cDot = rgb(pal.ink);
    cBase = rgb(pal.paper2);
    cIndigo = rgb(pal.indigo);
    cYellow = rgb(pal.yellow);
    cPaper = rgb(pal.paper);
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
  const TILT = -0.2;
  let ctx = canvas.getContext('2d')!;
  let font = '12px monospace';

  // per-pixel sphere tables
  let count = 0;
  let pIdx = new Int32Array(0);
  let pX = new Uint16Array(0);
  let pY = new Uint16Array(0);
  let pLam = new Float32Array(0);
  let pRow = new Uint16Array(0);
  let pCol = new Float32Array(0);
  let pRim = new Float32Array(0);
  let img: ImageData | null = null;
  let lastSphere = -1e9;
  let stars: Star[] = [];

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
    R = Math.min(W * 0.355, H * 0.335, (H - 170) / 1.55);
    R = Math.max(70, R);
    cell = 3;
    cx = W / 2;
    cy = Math.max(H * 0.55, R + (narrow ? 104 : 112));
    if (cy + R + R * 0.45 + 34 > H) cy = Math.max(R + 90, H - R * 1.45 - 34);
    rx = R * 1.34;
    ry = R * 0.4;
    N = Math.ceil((2 * R) / cell) + 2;
    buf.width = N;
    buf.height = N;
    img = bctx.createImageData(N, N);
    buildTables();
    // stars
    const rnd = mulberry32(99);
    const n = narrow ? 70 : 150;
    stars = [];
    for (let i = 0; i < n; i++)
      stars.push({
        x: rnd(),
        y: rnd(),
        s: rnd() < 0.82 ? 2 : 4,
        a: 0.25 + rnd() * 0.6,
        tw: rnd() < 0.16 ? rnd() * TAU : -1,
        plus: false,
      });
    for (let i = 0; i < 6; i++) stars.push({ x: rnd(), y: rnd(), s: 4, a: 0.7, tw: rnd() * TAU, plus: true });
    for (const c of chips) delete c.dataset.k;
    lastSphere = -1e9;
    opts.invalidate();
  };

  function buildTables() {
    const Rb = R / cell;
    const half = N / 2;
    const L = [-0.52, -0.6, 0.6];
    const ll = Math.hypot(...L);
    const lx = L[0] / ll,
      ly = L[1] / ll,
      lz = L[2] / ll;
    const a = 0.38;
    const ca = Math.cos(a),
      sa = Math.sin(a);
    const idx: number[] = [];
    pX = new Uint16Array(N * N);
    pY = new Uint16Array(N * N);
    pLam = new Float32Array(N * N);
    pRow = new Uint16Array(N * N);
    pCol = new Float32Array(N * N);
    pRim = new Float32Array(N * N);
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
        // thin bright rim, strongest toward the light
        const edge = clamp((0.2 - nz) / 0.2);
        const lit = clamp(0.35 + 0.65 * (-(nx / Math.sqrt(r2 + 1e-6)) * lx - (ny / Math.sqrt(r2 + 1e-6)) * ly) / Math.hypot(lx, ly));
        pRim[k] = edge * edge * lit;
        k++;
      }
    count = k;
    pIdx = Int32Array.from(idx);
  }

  // ---- simulation state ----
  let opX = 0; // operator offset from centre along the crown, px
  let opFlip = false;
  let walkClock = 0;
  let moving = false;
  let lineStart = 0;
  let lineText = '';
  let shown = -1;
  let beatKey = -2;
  let bubbleVisible = false;
  let bw = 0,
    bh = 0;
  const par = { x: 0, y: 0, tx: 0, ty: 0 };
  let fine = false;

  try {
    fine = matchMedia('(pointer: fine)').matches;
  } catch {}
  const onMove = (e: PointerEvent) => {
    if (!fine || reduced) return;
    ptr.x = e.clientX;
    ptr.y = e.clientY;
    ptr.on = true;
  };
  const ptr = { x: 0, y: 0, on: false };
  window.addEventListener('pointermove', onMove, { passive: true });

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

  function renderSphere(rot: number, print: number) {
    if (!img) return;
    const d = img.data;
    d.fill(0);
    const dark = pal.dark;
    const rotCols = rot * TW;
    const { tone, groove, accent } = surface;
    const full = print >= 1;
    const half = N / 2;
    for (let k = 0; k < count; k++) {
      const bx = pX[k],
        by = pY[k];
      const th = (BAYER4[by & 3][bx & 3] + 0.5) / 16;
      let reveal = 1;
      if (!full) {
        reveal = clamp(print * 1.8 - (by / N) * 0.8);
        if (reveal <= th) continue;
      }
      let col = Math.floor(pCol[k] - rotCols) % TW;
      if (col < 0) col += TW;
      const ti = pRow[k] * TW + col;
      const g = groove[ti];
      let v = 0.07 + 0.85 * pLam[k] ** 1.15 * tone[ti] * (1 - 0.85 * g) + pRim[k] * 0.9;
      v = clamp(v);
      const dens = dark ? v : (1 - v * 0.96) * 0.9;
      const th2 = (BAYER4[(by + 2) & 3][(bx + 1) & 3] + 0.5) / 16;
      let c = cBase;
      if (accent[ti] && v > 0.14 && pRim[k] < 0.1) c = accent[ti] === 1 ? cIndigo : cYellow;
      else if (dens > th2) c = cDot;
      const o = pIdx[k] * 4;
      d[o] = c[0];
      d[o + 1] = c[1];
      d[o + 2] = c[2];
      d[o + 3] = 255;
    }
    bctx.putImageData(img, 0, 0);
  }

  function mix(a: number[], b: number[], t: number) {
    return [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t].map(Math.round);
  }

  const dashRing = (from: number, to: number, front: boolean, ox: number, oy: number) => {
    // stroke the arc between `from`..`to` (radians) that is on the requested side of the sphere
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

  const crosshair = (x: number, y: number, s: number) => {
    ctx.beginPath();
    ctx.moveTo(Math.round(x) - s + 0.5, Math.round(y) + 0.5);
    ctx.lineTo(Math.round(x) + s + 0.5, Math.round(y) + 0.5);
    ctx.moveTo(Math.round(x) + 0.5, Math.round(y) - s + 0.5);
    ctx.lineTo(Math.round(x) + 0.5, Math.round(y) + s + 0.5);
    ctx.stroke();
  };

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
    if (r.top < innerHeight / 2 && r.bottom > innerHeight / 2) setStatus(`[illustrative] ${text}`);
  }

  const frame = (dt: number, tIn: number) => {
    if (!W || !H) return;
    const T = reduced ? 1e6 : tIn;

    // ---- timeline ----
    const rootAppear = T;
    const print = reduced ? 1 : ease((T - 500) / 1000);
    const ringQ = reduced ? 1 : ease((T - 650) / 1000);
    const starQ = reduced ? 1 : ease((T - 100) / 700);
    const opQ = reduced ? 1 : clamp((T - 1250) / 450);

    let stage = -1; // active node
    let finaleLine = -1;
    let lt = 0;
    let phase = 0;
    let probe = 0;
    let idx = 0;
    if (reduced) {
      finaleLine = 1;
      phase = -NODE_STEP * 4 + (INSPECT[4] - INSPECT[0]) / TAU;
    } else if (rootAppear < T0) {
      phase = 0.07 * (1 - ease((T - 1000) / 700));
    } else {
      const u = T - T0;
      const cyc = Math.floor(u / CYCLE);
      let r = u - cyc * CYCLE;
      let k = 0;
      if (r < 5 * BEAT) {
        k = Math.floor(r / BEAT);
        lt = r - k * BEAT;
        stage = k;
      } else {
        r -= 5 * BEAT;
        finaleLine = Math.floor(r / FIN);
        lt = r - finaleLine * FIN;
        k = 4;
      }
      idx = cyc * 5 + k;
      const target = (i: number) => -NODE_STEP * i + (INSPECT[i % 5] - INSPECT[0]) / TAU;
      if (stage >= 0 && !(idx === 0)) phase = target(idx - 1) + (target(idx) - target(idx - 1)) * ease(lt / ALIGN);
      else if (stage >= 0) phase = target(0);
      else phase = target(idx);
      if (stage >= 0) {
        probe = ease((lt - (idx === 0 ? PROBE_FIRST : PROBE_AFTER)) / 500) * (1 - ease((lt - (BEAT - 420)) / 380));
      }
    }
    const rotTurns = (T / 1000) * 0.008;

    // ---- pointer parallax ----
    if (ptr.on) {
      const b = root.getBoundingClientRect(); // once per frame, not per event
      par.tx = clamp(((ptr.x - b.left) / b.width) * 2 - 1, -1, 1);
      par.ty = clamp(((ptr.y - b.top) / b.height) * 2 - 1, -1, 1);
    }
    const k = 1 - Math.exp(-dt / 160);
    par.x += (par.tx - par.x) * k;
    par.y += (par.ty - par.y) * k;
    const off = (amt: number) => [Math.round(par.x * amt), Math.round(par.y * amt * 0.7)] as const;
    const [sox, soy] = off(2.5);
    const [rox, roy] = off(5);
    const [cox, coy] = off(7);
    const [oox, ooy] = off(10);

    // ---- node geometry ----
    const nodes = STAGES.map((_, j) => {
      const a = TAU * (phase + j * NODE_STEP) + INSPECT[0];
      return { a, ...ringPoint(a) };
    });
    const active = stage;

    // ---- operator ----
    const crownY = (dx: number) => cy - Math.sqrt(Math.max(0, R * R - dx * dx));
    let targetX = 0;
    if (stage >= 0) {
      const n = nodes[stage];
      targetX = clamp((n.x - cx) * 0.36, R * 0.18, R * 0.4);
    } else if (finaleLine === 0) targetX = 0;
    else if (finaleLine === 1) targetX = reduced ? 0 : -R * 0.2;
    if (rootAppear < T0 && !reduced) targetX = 0;
    if (reduced) opX = 0;
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
        if (stage >= 0) opFlip = nodes[stage].x < cx + opX;
        else if (finaleLine === 1) opFlip = true;
        else opFlip = false;
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

    // ---- script: bubble, status ----
    const key = reduced ? 100 : stage >= 0 ? idx : finaleLine >= 0 ? 1000 + idx * 2 + finaleLine : -1;
    if (key !== beatKey) {
      beatKey = key;
      lineStart = T;
      if (stage >= 0) announce(STAGES[stage].status);
      else if (finaleLine >= 0 && !reduced) announce(FINALE[finaleLine]);
    }
    if (reduced) {
      setBubble(FINALE[1], true);
      said.textContent = FINALE[1];
      shown = FINALE[1].length;
    } else {
      let text = '';
      let on = false;
      if (stage >= 0) {
        text = STAGES[stage].say;
        on = lt > (idx === 0 ? 350 : 500) && lt < BEAT - 500;
      } else if (finaleLine >= 0) {
        text = FINALE[finaleLine];
        on = lt > 350 && lt < FIN - 250;
      }
      if (text) setBubble(text, on);
      else if (bubbleVisible) setBubble(lineText, false);
      if (on) {
        const start = stage >= 0 ? (idx === 0 ? 350 : 500) : 350;
        const n = clamp(Math.floor((lt - start) / TYPE_MS), 0, lineText.length);
        if (n !== shown) {
          shown = n;
          said.textContent = lineText.slice(0, n);
        }
      }
    }

    // ---- draw ----
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    ctx.imageSmoothingEnabled = false;
    ctx.lineWidth = 1;

    // stars
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
      } else ctx.fillRect(x, y, s.s === 4 ? 2 : 2, s.s === 4 ? 2 : 2);
    }
    ctx.globalAlpha = 1;

    // frame registration marks + angular gauge around the core
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

    // ring (back half)
    const sweep = ringQ * TAU;
    ctx.strokeStyle = pal.ink;
    ctx.globalAlpha = 0.8;
    ctx.setLineDash([3, 4]);
    if (ringQ > 0) dashRing(-Math.PI * 0.5 * 0, sweep, false, rox, roy);
    ctx.setLineDash([]);
    ctx.globalAlpha = 1;

    const items = (front: boolean) => {
      // technical probes drifting on the ring
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
      if (T < 1450 && !reduced) return;
      nodes.forEach((n, j) => {
        if (n.front !== front) return;
        const x = Math.round(n.x + rox),
          y = Math.round(n.y + roy);
        const shownQ = reduced ? 1 : clamp((T - 1450 - j * 130) / 300);
        if (shownQ <= 0) return;
        const isActive = j === active;
        const hue = STAGES[j].hue === 'yellow' ? pal.yellow : pal.indigo;
        ctx.globalAlpha = shownQ;
        // marker: hollow square with a centre pip
        ctx.fillStyle = pal.paper;
        ctx.fillRect(x - 5, y - 5, 11, 11);
        ctx.strokeStyle = pal.ink;
        ctx.strokeRect(x - 4.5, y - 4.5, 10, 10);
        ctx.fillStyle = isActive && probe > 0.9 ? hue : pal.ink;
        ctx.fillRect(x - 1, y - 1, 3, 3);
        if (isActive) {
          // pulsing corner brackets
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
          if (probe > 0.9) {
            ctx.globalAlpha = 1;
            ctx.fillStyle = hue;
            ctx.fillRect(x - 4, y - 4, 9, 2);
          }
        }
        ctx.globalAlpha = 1;
      });
    };
    drawNodes(false);

    // core
    if (T - lastSphere >= 33 || reduced || print < 1) {
      lastSphere = T;
      renderSphere(rotTurns, print);
    }
    ctx.drawImage(buf, 0, 0, N, N, Math.round(cx - (N * cell) / 2) + cox, Math.round(cy - (N * cell) / 2) + coy, N * cell, N * cell);
    // limb: thin bright edge, brightest toward the light
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

    // ring (front half), with a paper underlay so it reads over the dotted surface
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
    ctx.globalAlpha = 1;
    items(true);
    drawNodes(true);

    // operator
    if (opQ > 0) {
      const rows = Math.ceil(OPERATOR_H * opQ);
      ctx.save();
      ctx.beginPath();
      ctx.rect(opLeft + oox - 2, opTop + ooy + opH - rows * opCell, opW + 4, rows * opCell + 4);
      ctx.clip();
      // soft contact shadow
      ctx.fillStyle = pal.ink;
      ctx.globalAlpha = 0.25;
      ctx.fillRect(opLeft + oox + opCell, opTop + ooy + opH, opW - opCell * 2, 2);
      ctx.globalAlpha = 1;
      drawOperator(ctx, opLeft + oox, opTop + ooy, opCell, opFrame, pal, opFlip);
      ctx.restore();
    }

    // probe from the operator to the active node
    if (stage >= 0 && probe > 0.01) {
      const n = nodes[stage];
      const hx = opLeft + oox + (opFlip ? 0 : opW);
      const hy = opTop + ooy + 9 * opCell;
      const tx = n.x + rox,
        ty = n.y + roy;
      const px = hx + (tx - hx) * probe,
        py = hy + (ty - hy) * probe;
      const hue = STAGES[stage].hue === 'yellow' ? pal.yellow : pal.indigo;
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

    // measurement readouts, driven by the live values
    const ink2 = pal.ink2;
    if (ringQ >= 0.6) {
      ctx.globalAlpha = clamp((ringQ - 0.6) * 3);
      const rot = ((rotTurns % 1) * 360).toFixed(1).padStart(5, '0');
      pixelText(`CORE / ROT ${rot}°`, 28, 33, ink2);
      if (active >= 0) {
        const n = nodes[active];
        const r = Math.hypot(n.x - cx, (n.y - cy) / 0.8) / R;
        const th = (Math.atan2(-(n.y - cy), n.x - cx) * 180) / Math.PI;
        pixelText(`r=${r.toFixed(2)} / θ=${th.toFixed(1)}°`, W - 28, 33, ink2, 'right');
        // leader + crosshair at the inspected node
        ctx.strokeStyle = ink2;
        ctx.globalAlpha = 0.5;
        crosshair(n.x + rox, n.y + roy, 14);
        ctx.globalAlpha = 1;
      } else {
        pixelText(`r=1.34 / θ=${(((INSPECT[4] * 180) / Math.PI) % 360).toFixed(1)}°`, W - 28, 33, ink2, 'right');
      }
      ctx.globalAlpha = 1;
    }

    // ---- DOM: chips + bubble ----
    chips.forEach((chip, j) => {
      const n = nodes[j];
      const x = n.x + rox,
        y = n.y + roy;
      const wkey = `${narrow}${j === active && probe > 0.05}`;
      if (chip.dataset.k !== wkey) {
        chip.dataset.k = wkey;
        chip.dataset.compact = String(narrow);
        chip.dataset.active = String(j === active && probe > 0.05);
        chip.dataset.w = String(chip.offsetWidth);
      }
      const w = Number(chip.dataset.w) || 80;
      const half = w / 2;
      const cxp = clamp(x, half + 6, W - half - 6);
      const up = !n.front;
      const hidden = hiddenBehind(x, y, n.front);
      const appear = reduced ? 1 : clamp((T - 1550 - j * 130) / 250);
      chip.style.transform = `translate(${Math.round(cxp)}px, ${Math.round(y + (up ? -16 : 16))}px) translate(-50%, ${up ? '-100%' : '0'})`;
      chip.dataset.state = appear < 1 || hidden ? 'off' : 'on';
      chip.dataset.hue = STAGES[j].hue;
    });

    if (bubbleVisible || bubble.dataset.state === 'on') {
      const bx = clamp(feetX + oox - bw * 0.35, 6, W - bw - 6);
      const by = opTop + ooy - bh - 6;
      bubble.style.transform = `translate(${Math.round(bx)}px, ${Math.max(4, Math.round(by))}px)`;
      const tail = clamp(feetX + oox - bx, 14, bw - 20);
      bubble.style.setProperty('--tail', `${Math.round(tail)}px`);
    }
  };

  return {
    frame,
    cleanup: () => {
      ro.disconnect();
      stopTheme();
      window.removeEventListener('pointermove', onMove);
    },
  };
}
