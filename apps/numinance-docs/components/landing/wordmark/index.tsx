'use client';

/**
 * Animated character-grid NUMINANCE wordmark.
 *
 * The idea (glyphs fall on a character grid at a fixed step rate, settle only inside the letter
 * mask, hold, drain, repeat, and the pointer pushes them away) was informed by the Fumadocs landing
 * wordmark simulation. This is an independent implementation with a different mask source,
 * seeded RNG, per-column speeds and phase machine.
 *
 * Portions of the approach: Copyright (c) 2023 Fuma, MIT License.
 */

import { onThemeChange, readPalette, type Palette } from '@/lib/landing/palette';
import { mulberry32 } from '@/lib/landing/rng';
import { useScene } from '@/lib/landing/use-scene';

const GLYPHS = '#-*>[]{}/=+01:;';
const RATE = 40; // simulation steps per second
const HOLD_STEPS = Math.round(RATE * 4.5);
const GAP_STEPS = Math.round(RATE * 0.8);
const MIN_COLS = 80; // keeps strokes >= ~2 cells even at 320px
const CELL_H = 1.25; // cell height / width
const HEAD = 0.7; // extra rows above the word, as a fraction of the word's rows
const SVG_URL = '/brand/numinance-wordmark-ink.svg';
const SVG_ASPECT = 461.3 / 88.9;

const FILL = 0;
const HOLD = 1;
const DRAIN = 2;
const GAP = 3;

/** looks: falling, resting, pushed, orange, sun, indigo */
const LOOKS = 6;
const TINT_LOOK = [1, 3, 4, 5];

interface MaskSource {
  aspect: number;
  /** two-line layout (NUMI / NANCE) for narrow screens */
  stackedAspect: number;
  paint: (ctx: CanvasRenderingContext2D, w: number, h: number, stacked: boolean) => void;
}

const LINE_GAP = 0.3; // stacked line gap, in line heights

let maskPromise: Promise<MaskSource> | null = null;

/** 5x7 bitmap fallback if the official artwork cannot be loaded */
const BITMAP: Record<string, string[]> = {
  N: ['10001', '11001', '10101', '10011', '10001', '10001', '10001'],
  U: ['10001', '10001', '10001', '10001', '10001', '10001', '01110'],
  M: ['10001', '11011', '10101', '10101', '10001', '10001', '10001'],
  I: ['01110', '00100', '00100', '00100', '00100', '00100', '01110'],
  A: ['01110', '10001', '10001', '11111', '10001', '10001', '10001'],
  C: ['01110', '10001', '10000', '10000', '10000', '10001', '01110'],
  E: ['11111', '10000', '10000', '11110', '10000', '10000', '11111'],
};
const BITMAP_W = 9 * 5 + 8;

function bitmapSource(): MaskSource {
  return {
    aspect: BITMAP_W / 7,
    stackedAspect: 29 / (7 * (2 + LINE_GAP)),
    paint(ctx, w, h, stacked) {
      ctx.fillStyle = '#000';
      const lines = stacked ? ['NUMI', 'NANCE'] : ['NUMINANCE'];
      const sx = w / (stacked ? 29 : BITMAP_W);
      const sy = h / (stacked ? 7 * (2 + LINE_GAP) : 7);
      lines.forEach((line, li) =>
        [...line].forEach((ch, n) => {
          BITMAP[ch].forEach((row, y) =>
            [...row].forEach((bit, x) => {
              if (bit === '1')
                ctx.fillRect((n * 6 + x) * sx, (li * 7 * (1 + LINE_GAP) + y) * sy, sx + 0.5, sy + 0.5);
            }),
          );
        }),
      );
    },
  };
}

/** Rasterises the official wordmark artwork (alpha only); falls back to a bitmap font. */
function loadMask(): Promise<MaskSource> {
  if (maskPromise) return maskPromise;
  maskPromise = (async () => {
    try {
      const res = await fetch(SVG_URL);
      if (!res.ok) throw new Error('svg');
      let text = await res.text();
      // some engines need explicit size to rasterise a viewBox-only SVG
      text = text.replace('<svg ', `<svg width="${SVG_ASPECT * 200}" height="200" `);
      const url = URL.createObjectURL(new Blob([text], { type: 'image/svg+xml' }));
      const img = new Image();
      await new Promise<void>((ok, bad) => {
        img.onload = () => ok();
        img.onerror = () => bad(new Error('img'));
        img.src = url;
      });
      URL.revokeObjectURL(url);
      // crop to the artwork's ink bounds so the mask has no dead rows
      const iw = Math.round(SVG_ASPECT * 400);
      const probe = document.createElement('canvas');
      probe.width = iw;
      probe.height = 400;
      const pc = probe.getContext('2d', { willReadFrequently: true })!;
      pc.drawImage(img, 0, 0, iw, 400);
      const d = pc.getImageData(0, 0, iw, 400).data;
      let x0 = iw, x1 = 0, y0 = 400, y1 = 0;
      for (let y = 0; y < 400; y++)
        for (let x = 0; x < iw; x++)
          if (d[(y * iw + x) * 4 + 3] > 20) {
            if (x < x0) x0 = x;
            if (x > x1) x1 = x;
            if (y < y0) y0 = y;
            if (y > y1) y1 = y;
          }
      if (x1 <= x0 || y1 <= y0) throw new Error('empty');
      const sw = x1 - x0 + 1;
      const sh = y1 - y0 + 1;
      const k = (SVG_ASPECT * 200) / iw; // probe px -> image natural px
      // stacked split: the empty column run closest to 43% of the ink width ("Numi" | "nance")
      let gap = x0 + Math.round(sw * 0.43);
      let bestD = Infinity;
      for (let x = x0 + 1; x < x1; x++) {
        let empty = true;
        for (let y = y0; y <= y1 && empty; y++) if (d[(y * iw + x) * 4 + 3] > 20) empty = false;
        if (empty && Math.abs(x - (x0 + sw * 0.43)) < bestD) {
          bestD = Math.abs(x - (x0 + sw * 0.43));
          gap = x;
        }
      }
      const w1 = gap - x0;
      const w2 = x1 - gap + 1;
      const wide = Math.max(w1, w2);
      return {
        aspect: sw / sh,
        stackedAspect: wide / (sh * (2 + LINE_GAP)),
        paint: (ctx, w, h, stacked) => {
          if (!stacked) {
            ctx.drawImage(img, x0 * k, y0 * k, sw * k, sh * k, 0, 0, w, h);
            return;
          }
          const sx = w / wide;
          const sy = h / (sh * (2 + LINE_GAP)); // the box is stretched to the cell grid's aspect
          ctx.drawImage(img, x0 * k, y0 * k, w1 * k, sh * k, 0, 0, w1 * sx, sh * sy);
          ctx.drawImage(img, gap * k, y0 * k, w2 * k, sh * k, 0, sh * sy * (1 + LINE_GAP), w2 * sx, sh * sy);
        },
      } satisfies MaskSource;
    } catch {
      return bitmapSource();
    }
  })();
  return maskPromise;
}

function hash(n: number) {
  return Math.imul(n + 0x9e3779b9, 2654435761) >>> 0;
}

function setup(host: HTMLDivElement, { reduced, invalidate }: { reduced: boolean; invalidate: () => void }) {
  const canvas = host.querySelector('canvas')!;
  const ctx = canvas.getContext('2d')!;
  const atlas = document.createElement('canvas');
  const ac = new AbortController();

  let disposed = false;
  let ready = false;
  let mask: MaskSource | null = null;
  let pal: Palette | null = null;
  let dpr = 1;
  let cols = 0;
  let rows = 0;
  let cellW = 0;
  let cellH = 0;
  let lastW = 0;
  let stacked = false;

  let inside = new Uint8Array(0);
  let grain = new Uint8Array(0);
  let tint = new Uint8Array(0);
  let rest = new Uint8Array(0);
  let hot = new Uint8Array(0);
  let origin = new Int32Array(0);
  let hole = new Int16Array(0);
  let period = new Uint8Array(0);
  let releaseAt = new Int32Array(0);
  let insideList: number[] = [];
  let spawn: number[] = [];
  let moved: number[] = [];
  let capacity = 0;
  let filled = 0;

  let rng = mulberry32(1);
  let cycle = 0;
  let phase = FILL;
  let phaseAt = 0;
  let stepNo = 0;
  let best = 0;
  let bestAt = 0;
  let holdLevel = 0;
  let parity = 0;
  let acc = 0;
  let streams: { x: number; left: number }[] = [];

  // pointer in css px relative to the canvas, null when away
  let ptr: { x: number; y: number } | null = null;

  const mono = () =>
    getComputedStyle(host).getPropertyValue('--font-mono').trim() ||
    "'Geist Mono', ui-monospace, monospace";

  function paintAtlas() {
    pal = readPalette();
    const w = Math.round(cellW * dpr);
    const h = Math.round(cellH * dpr);
    atlas.width = w * GLYPHS.length;
    atlas.height = h * LOOKS;
    const a = atlas.getContext('2d')!;
    a.clearRect(0, 0, atlas.width, atlas.height);
    a.font = `${Math.max(6, Math.round(cellH * 0.86 * dpr))}px ${mono()}`;
    a.textAlign = 'center';
    a.textBaseline = 'middle';
    const colors = [
      pal.ink2,
      pal.ink,
      pal.dark ? pal.yellow : pal.orange,
      pal.orange,
      pal.dark ? pal.sun : pal.orange,
      pal.indigo,
    ];
    for (let look = 0; look < LOOKS; look++) {
      a.globalAlpha = look === 0 ? 0.7 : 1;
      a.fillStyle = colors[look];
      for (let g = 0; g < GLYPHS.length; g++) a.fillText(GLYPHS[g], g * w + w / 2, look * h + h / 2 + dpr * 0.5);
    }
    a.globalAlpha = 1;
  }

  function roll(): number {
    const r = rng();
    return r < 0.012 ? 3 : r < 0.065 ? 1 : r < 0.1 ? 2 : 0;
  }

  function newCycle() {
    cycle++;
    rng = mulberry32(0x4e554d49 + cycle * 7919);
    phase = FILL;
    phaseAt = stepNo;
    best = 0;
    bestAt = stepNo;
    streams = [];
  }

  function layout() {
    const W = host.clientWidth;
    if (!mask || W === 0) return;
    lastW = W;
    dpr = Math.min(window.devicePixelRatio || 1, 2);
    stacked = W < 600;
    const aspect = stacked ? mask.stackedAspect : mask.aspect;
    cols = stacked ? Math.max(48, Math.round(W / 7)) : Math.max(MIN_COLS, Math.round(W / 10));
    cellW = W / cols;
    const wordRows = Math.max(6, Math.round(W / aspect / (cellW * CELL_H)));
    cellH = W / aspect / wordRows;
    const head = Math.round(wordRows * (stacked ? 0.45 : HEAD));
    rows = head + wordRows + 1;

    canvas.width = Math.round(cols * cellW * dpr);
    canvas.height = Math.round(rows * cellH * dpr);
    canvas.style.width = `${cols * cellW}px`;
    canvas.style.height = `${rows * cellH}px`;
    host.style.aspectRatio = 'auto';

    // sample the mask at 4x4 per cell, coverage-thresholded
    const S = 4;
    const m = document.createElement('canvas');
    m.width = cols * S;
    m.height = wordRows * S;
    const mc = m.getContext('2d', { willReadFrequently: true })!;
    mask.paint(mc, m.width, m.height, stacked);
    const px = mc.getImageData(0, 0, m.width, m.height).data;

    const n = cols * rows;
    inside = new Uint8Array(n);
    grain = new Uint8Array(n);
    tint = new Uint8Array(n);
    rest = new Uint8Array(n).fill(255);
    hot = new Uint8Array(n);
    origin = new Int32Array(n).fill(-1);
    hole = new Int16Array(cols);
    releaseAt = new Int32Array(cols);
    period = new Uint8Array(cols);
    const pr = mulberry32(0x6d1e);
    for (let x = 0; x < cols; x++) {
      const r = pr();
      period[x] = r < (stacked ? 0.7 : 0.5) ? 1 : r < 0.92 ? 2 : 3;
    }
    moved = [];
    insideList = [];
    spawn = [];
    capacity = 0;
    filled = 0;
    for (let x = 0; x < cols; x++) {
      let any = false;
      for (let y = 0; y < wordRows; y++) {
        let cov = 0;
        for (let sy = 0; sy < S; sy++)
          for (let sx = 0; sx < S; sx++) cov += px[((y * S + sy) * m.width + x * S + sx) * 4 + 3];
        if (cov / (S * S * 255) > 0.3) {
          const i = (head + y) * cols + x;
          inside[i] = 1;
          insideList.push(i);
          capacity++;
          any = true;
        }
      }
      if (any) spawn.push(x);
    }

    stepNo = 0;
    acc = 0;
    cycle = 0;
    rng = mulberry32(0x4e554d49);
    phase = FILL;
    phaseAt = 0;
    best = 0;
    bestAt = 0;
    streams = [];

    if (reduced) {
      for (const i of insideList) {
        const h = hash(i);
        grain[i] = 1 + (h >>> 8) % GLYPHS.length;
        const r = ((h >>> 3) & 255) / 255;
        tint[i] = r < 0.012 ? 3 : r < 0.065 ? 1 : r < 0.1 ? 2 : 0;
      }
      filled = capacity;
    }
    paintAtlas();
    ready = true;
    invalidate();
  }

  function free(to: number, x: number, y: number, held: boolean) {
    return x >= 0 && x < cols && !grain[to] && (inside[to] === 1 || !held || hole[x] > y);
  }

  function move(from: number, to: number) {
    origin[to] = origin[from] < 0 ? from : origin[from];
    origin[from] = -1;
    moved.push(to);
    grain[to] = grain[from];
    grain[from] = 0;
    tint[to] = tint[from];
    hot[to] = hot[from];
    hot[from] = 0;
    rest[to] = 0;
    if (inside[to] && !inside[from]) filled++;
    else if (!inside[to] && inside[from]) filled--;
  }

  function emit(x: number) {
    if (grain[x]) return;
    grain[x] = 1 + Math.floor(rng() * GLYPHS.length);
    tint[x] = roll();
    rest[x] = 0;
    hot[x] = 0;
  }

  function push() {
    if (!ptr) return;
    const R = cellW * 5;
    const c0 = Math.max(0, Math.floor((ptr.x - R) / cellW));
    const c1 = Math.min(cols - 1, Math.floor((ptr.x + R) / cellW));
    const r0 = Math.max(0, Math.floor((ptr.y - R) / cellH));
    const r1 = Math.min(rows - 1, Math.floor((ptr.y + R) / cellH));
    const hits: number[] = [];
    for (let y = r0; y <= r1; y++)
      for (let x = c0; x <= c1; x++) if (grain[y * cols + x]) hits.push(y * cols + x);
    for (const i of hits) {
      if (!grain[i]) continue;
      const x = i % cols;
      const y = (i / cols) | 0;
      const dx = (x + 0.5) * cellW - ptr.x;
      const dy = (y + 0.5) * cellH - ptr.y;
      const d = Math.hypot(dx, dy);
      if (d >= R) continue;
      const s = 1 - d / R; // impulse falls off with distance
      if (rng() > 0.25 + 0.75 * s) continue;
      const n = 1 + (s > 0.45 ? 1 : 0) + (s > 0.8 ? 1 : 0);
      const sx = dx === 0 ? (rng() < 0.5 ? -1 : 1) : Math.sign(dx);
      const sy = Math.abs(dy) < cellH * 0.35 ? 0 : Math.sign(dy);
      let cur = i;
      for (let k = 0; k < n; k++) {
        const cx = cur % cols;
        const cy = (cur / cols) | 0;
        const tries: [number, number][] = [
          [sx, sy],
          [sx, 0],
          [0, sy],
        ];
        let moved1 = false;
        for (const [ox, oy] of tries) {
          if (!ox && !oy) continue;
          const nx = cx + ox;
          const ny = cy + oy;
          if (nx < 0 || nx >= cols || ny < 0 || ny >= rows - 1) continue;
          const to = ny * cols + nx;
          if (grain[to]) continue;
          move(cur, to);
          hot[to] = 12;
          cur = to;
          moved1 = true;
          break;
        }
        if (!moved1) break;
      }
    }
  }

  function step() {
    for (const i of moved) origin[i] = -1;
    moved = [];
    stepNo++;

    // phase machine
    if (phase === FILL) {
      const maxStreams = Math.max(6, Math.round(cols / 4));
      if (streams.length < maxStreams && spawn.length && rng() < 0.8)
        streams.push({ x: spawn[Math.floor(rng() * spawn.length)], left: 4 + Math.floor(rng() * 26) });
      for (let k = streams.length - 1; k >= 0; k--) {
        const s = streams[k];
        if (rng() < 0.7) emit(s.x);
        if (--s.left <= 0) streams.splice(k, 1);
      }
      if (filled > best) {
        best = filled;
        bestAt = stepNo;
      }
      const stalled = filled > capacity * 0.88 && stepNo - bestAt > RATE * 1.4;
      const late = filled > capacity * 0.6 && stepNo - phaseAt > RATE * 26;
      if (stalled || late) {
        phase = HOLD;
        phaseAt = stepNo;
        holdLevel = filled;
        streams = [];
      }
    } else if (phase === HOLD) {
      // pushed glyphs leave holes: a thin rain repairs them
      if (filled < holdLevel) {
        for (let n = 0; n < 2; n++) {
          const x = spawn[Math.floor(rng() * spawn.length)];
          if (hole[x] >= 0) emit(x);
        }
      }
      if (stepNo - phaseAt >= HOLD_STEPS) {
        phase = DRAIN;
        phaseAt = stepNo;
        for (let x = 0; x < cols; x++) releaseAt[x] = stepNo + Math.floor((x / cols) * 56 + rng() * 14);
      }
    } else if (phase === DRAIN) {
      let any = false;
      for (let i = 0; i < grain.length; i++)
        if (grain[i]) {
          any = true;
          break;
        }
      if (!any) {
        phase = GAP;
        phaseAt = stepNo;
        filled = 0;
      }
    } else if (stepNo - phaseAt >= GAP_STEPS) {
      newCycle();
    }

    if (ptr) push();

    // shimmer: a couple of settled glyphs change, sparsely
    if (phase === HOLD || (phase === FILL && filled > capacity * 0.5)) {
      for (let n = 0; n < 2; n++) {
        const i = insideList[Math.floor(rng() * insideList.length)];
        if (grain[i] && rest[i] > 6 && hot[i] === 0) {
          grain[i] = 1 + Math.floor(rng() * GLYPHS.length);
          if (rng() < 0.15) tint[i] = roll();
        }
      }
    }

    for (let x = 0; x < cols; x++) {
      let y = rows - 1;
      while (y >= 0 && !(inside[y * cols + x] && !grain[y * cols + x])) y--;
      hole[x] = y;
    }

    parity ^= 1;
    for (let y = rows - 1; y >= 0; y--) {
      for (let k = 0; k < cols; k++) {
        const x = parity ? k : cols - 1 - k;
        const i = y * cols + x;
        if (!grain[i]) continue;
        if (hot[i] > 0) hot[i]--;
        if (rest[i] < 255) rest[i]++;
        if (y === rows - 1) {
          grain[i] = 0;
          continue;
        }
        const released = phase === DRAIN && stepNo >= releaseAt[x];
        const isIn = inside[i] === 1;
        if (released && rng() < 0.012) {
          grain[i] = 0;
          if (isIn) filled--;
          continue;
        }
        if (stepNo % period[x] !== 0) continue;
        const held = isIn && !released && phase !== GAP;
        const below = i + cols;
        if (free(below, x, y, held)) {
          move(i, below);
          continue;
        }
        const dx = rng() < 0.5 ? -1 : 1;
        if (free(below + dx, x + dx, y, held)) move(i, below + dx);
        else if (free(below - dx, x - dx, y, held)) move(i, below - dx);
        else if (!isIn) grain[i] = 0; // blocked in the air: dissolve
      }
    }
  }

  function draw(t: number) {
    if (!ready || !pal) return;
    const w = Math.round(cellW * dpr);
    const h = Math.round(cellH * dpr);
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    // ghost dots mark the letter cells
    const dot = Math.max(1, Math.round(dpr));
    ctx.globalAlpha = pal.dark ? 0.35 : 0.5;
    ctx.fillStyle = pal.lineStrong;
    for (const i of insideList) {
      if (grain[i]) continue;
      const x = Math.round((i % cols) * cellW * dpr);
      const y = ((i / cols) | 0) * h;
      ctx.fillRect(x + ((w - dot) >> 1), y + ((h - dot) >> 1), dot, dot);
    }
    ctx.globalAlpha = 1;
    for (let i = 0; i < grain.length; i++) {
      const g = grain[i];
      if (!g) continue;
      const cx = i % cols;
      const cy = (i / cols) | 0;
      let fx = cx;
      let fy = cy;
      const o = origin[i];
      if (o >= 0) {
        const ox = o % cols;
        const oy = (o / cols) | 0;
        fx = ox + (cx - ox) * t;
        fy = oy + (cy - oy) * t;
      }
      const look = hot[i] > 0 ? 2 : rest[i] > 1 ? TINT_LOOK[tint[i]] : 0;
      ctx.drawImage(
        atlas,
        (g - 1) * w,
        look * h,
        w,
        h,
        Math.round(fx * cellW * dpr),
        Math.round(fy * cellH * dpr),
        w,
        h,
      );
    }
  }

  // --- wiring ---
  let debounce = 0;
  let first = true;
  const ro = new ResizeObserver(() => {
    if (!mask) return;
    if (Math.abs(host.clientWidth - lastW) < 1 && !first) return;
    if (first) {
      first = false;
      layout();
      return;
    }
    window.clearTimeout(debounce);
    debounce = window.setTimeout(layout, 140);
  });

  const stopTheme = onThemeChange(() => {
    if (!ready) return;
    paintAtlas();
    draw(0);
  });

  function onPointer(e: PointerEvent) {
    if (!ready || reduced) return;
    const box = canvas.getBoundingClientRect();
    const x = e.clientX - box.left;
    const y = e.clientY - box.top;
    ptr = x >= 0 && y >= 0 && x < box.width && y < box.height ? { x, y } : null;
  }
  const onLeave = () => {
    ptr = null;
  };
  host.addEventListener('pointermove', onPointer, { signal: ac.signal });
  host.addEventListener('pointerdown', onPointer, { signal: ac.signal });
  host.addEventListener('pointerleave', onLeave, { signal: ac.signal });
  host.addEventListener('pointercancel', onLeave, { signal: ac.signal });
  host.addEventListener('pointerup', (e) => e.pointerType !== 'mouse' && onLeave(), { signal: ac.signal });

  void Promise.all([
    loadMask(),
    document.fonts ? document.fonts.load(`12px ${mono()}`).catch(() => undefined) : undefined,
  ]).then(([m]) => {
    if (disposed) return;
    mask = m;
    ro.observe(host);
  });

  return {
    frame(dt: number) {
      if (!ready) return;
      if (!reduced) {
        acc += (Math.min(250, dt) * RATE) / 1000;
        for (; acc >= 1; acc--) step();
      }
      draw(reduced ? 0 : acc);
    },
    cleanup() {
      disposed = true;
      window.clearTimeout(debounce);
      ro.disconnect();
      stopTheme();
      ac.abort();
    },
  };
}

/** Footer wordmark: falling glyphs settle into the official NUMINANCE letterforms. */
export function Wordmark({ className = '' }: { className?: string }) {
  const ref = useScene<HTMLDivElement>(setup);
  return (
    <div className={`w-full px-(--lp-gutter) pt-16 pb-10 sm:pt-24 sm:pb-16 ${className}`}>
      <div
        ref={ref}
        className="relative mx-auto w-full max-w-(--lp-max) overflow-hidden"
        style={{ touchAction: 'pan-y', aspectRatio: `1 / ${1 / 6.8 + 0.7 / 6.8 + 0.02}` }}
      >
        <canvas aria-hidden className="block" />
        <span className="sr-only">Numinance</span>
      </div>
    </div>
  );
}

/** Static, non-animated wordmark in the current ink colour (official artwork as a CSS mask). */
export function WordmarkStatic({ className = '' }: { className?: string }) {
  return (
    <div className={`w-full px-(--lp-gutter) ${className}`}>
      <span
        role="img"
        aria-label="Numinance"
        className="mx-auto block w-full max-w-(--lp-max) bg-lp-ink"
        style={{
          aspectRatio: `${SVG_ASPECT}`,
          maskImage: `url(${SVG_URL})`,
          WebkitMaskImage: `url(${SVG_URL})`,
          maskRepeat: 'no-repeat',
          WebkitMaskRepeat: 'no-repeat',
          maskSize: '100% 100%',
          WebkitMaskSize: '100% 100%',
        }}
      />
    </div>
  );
}
