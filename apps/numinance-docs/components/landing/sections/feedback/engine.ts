import { report } from '@/lib/landing/status';
import { type Point, clipsFor, roundedPath } from './cable';
import { createClock } from './clock';

/** px per ms a cable's head travels, and the least / most a growth takes (ms) */
const SPEED = 1.4;
const GROW: [number, number] = [300, 600];
/** ms between one window's pop and the next cable starting */
const NEXT = 130;
const FRAME = 42;
const POP: Keyframe[] = [
  { transform: 'scale(0)' },
  { transform: 'scale(0.3, 0.2)' },
  { transform: 'scale(0.86)' },
  { transform: 'scale(1.03)' },
  { transform: 'none' },
];
const PORT_HALF = 6;
const SVG = 'http://www.w3.org/2000/svg';

interface Wire {
  win: HTMLElement;
  port: HTMLElement;
  jack: HTMLElement;
  svg: SVGSVGElement;
  rail: SVGPathElement;
  core: SVGPathElement;
  clipsG: SVGGElement;
  head: HTMLElement;
  d: string;
  length: number;
  tip: Point;
  clips: { el: SVGRectElement; at: number }[];
  drawn: boolean;
  running: Animation[];
}

/** `el`'s layout box in `desk`'s coordinates (unaffected by a pop's transform) */
function boxIn(el: HTMLElement, desk: HTMLElement) {
  let x = 0;
  let y = 0;
  for (let n: HTMLElement | null = el; n && n !== desk; n = n.offsetParent as HTMLElement | null) {
    x += n.offsetLeft;
    y += n.offsetTop;
  }
  return { x, y, w: el.offsetWidth, h: el.offsetHeight };
}

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

function svgEl<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string | number>) {
  const el = document.createElementNS(SVG, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, String(v));
  return el;
}

export interface FeedbackOptions {
  animate: boolean;
  onCount: (n: number) => void;
}

/**
 * Cable-led entrance. Desktop: for each window in order a cable draws from its source socket along
 * an orthogonal, rounded route to the window's port, the window pops on the cable tip, then the
 * next cable starts. Narrow: no wires, windows pop as they scroll into view. `animate:false`
 * (reduced motion) lays everything out in its final state.
 */
export function createFeedback(desk: HTMLElement, { animate, onCount }: FeedbackOptions): () => void {
  const layer = desk.querySelector<HTMLElement>('[data-layer]');
  if (!layer) return () => {};
  const wins = [...desk.querySelectorAll<HTMLElement>('[data-window]')];
  const total = wins.length;
  const wide = () => layer.getClientRects().length > 0;
  const mq = window.matchMedia('(min-width: 900px)');
  const startedWide = mq.matches;

  const wires: Wire[] = [];
  for (const win of wins) {
    const port = win.querySelector<HTMLElement>('[data-port]');
    const jack = desk.querySelector<HTMLElement>(`[data-jack="${win.dataset.from}"]`);
    if (!port || !jack) continue;
    const svg = svgEl('svg', { class: 'absolute left-0 top-0 overflow-visible' }) as SVGSVGElement;
    svg.style.cssText = 'position:absolute;left:0;top:0;overflow:visible';
    const rail = svgEl('path', { fill: 'none', 'stroke-width': 4, 'stroke-linejoin': 'round', class: 'stroke-lp-ink' }) as SVGPathElement;
    const core = svgEl('path', { fill: 'none', 'stroke-width': 2, 'stroke-linejoin': 'round', class: 'stroke-lp-paper' }) as SVGPathElement;
    const clipsG = svgEl('g', {}) as SVGGElement;
    svg.append(rail, core, clipsG);
    const head = document.createElement('span');
    head.setAttribute('aria-hidden', 'true');
    head.className = 'absolute top-0 left-0 size-2 bg-lp-ink';
    head.style.visibility = 'hidden';
    head.style.offsetRotate = '0deg';
    layer.append(svg, head);
    wires.push({
      win, port, jack, svg, rail, core, clipsG, head,
      d: '', length: 0, tip: [0, 0], clips: [], drawn: !animate, running: [],
    });
  }

  function route(w: Wire, deskBox: DOMRect, index: number): Point[] {
    const j = w.jack.getBoundingClientRect();
    const sx = Math.round(j.left - deskBox.left + j.width / 2);
    const sy = Math.round(j.bottom - deskBox.top);
    const { x: wx, y: wy, w: ww, h: wh } = boxIn(w.win, desk);
    const side = w.win.dataset.entry; // 'top' | 'left' | 'right'
    const lane = Math.round(deskBox.width / 2);
    const corridor = sy + 18 + (index % 2) * 18;
    if (side === 'top') {
      const ex = Math.round(clamp(sx, wx + 28, wx + ww - 28));
      const mid = Math.round(sy + (wy - sy) / 2);
      return sx === ex ? [[sx, sy], [ex, wy]] : [[sx, sy], [sx, mid], [ex, mid], [ex, wy]];
    }
    const ey = Math.round(wy + Math.min(wh * 0.38, 90));
    if (side === 'right') {
      const lx = lane - 9;
      return [[sx, sy], [sx, corridor], [lx, corridor], [lx, ey], [wx + ww, ey]];
    }
    const lx = lane + 9;
    return [[sx, sy], [sx, corridor], [lx, corridor], [lx, ey], [wx, ey]];
  }

  function layout() {
    const on = wide();
    layer!.style.display = '';
    if (!on) return;
    const box = desk.getBoundingClientRect();
    wires.forEach((w, i) => {
      for (const a of w.running) a.finish();
      const pts = route(w, box, i);
      w.tip = pts[pts.length - 1];
      w.d = roundedPath(pts);
      w.svg.setAttribute('width', String(box.width));
      w.svg.setAttribute('height', String(box.height));
      w.rail.setAttribute('d', w.d);
      w.core.setAttribute('d', w.d);
      w.length = w.rail.getTotalLength();
      for (const p of [w.rail, w.core]) {
        p.style.strokeDasharray = `${w.length}`;
        p.style.strokeDashoffset = w.drawn ? '0' : `${w.length}`;
      }
      w.clipsG.replaceChildren();
      w.clips = clipsFor(pts).map((c) => {
        const [cw, ch] = c.vertical ? [13, 5] : [5, 13];
        const el = svgEl('rect', {
          x: c.x - cw / 2, y: c.y - ch / 2, width: cw, height: ch,
          class: 'fill-lp-paper stroke-lp-ink', 'stroke-width': 1, 'shape-rendering': 'crispEdges',
        }) as SVGRectElement;
        el.style.opacity = w.drawn ? '1' : '0';
        w.clipsG.append(el);
        return { el, at: c.at };
      });
      w.head.style.offsetPath = `path("${w.d}")`;
      // the window's port: centred on the cable's tip
      const own = boxIn(w.win, desk);
      w.port.style.left = `${w.tip[0] - own.x - PORT_HALF}px`;
      w.port.style.top = `${w.tip[1] - own.y - PORT_HALF}px`;
    });
  }

  const clock = createClock([desk]);
  let dead = false;
  let received = 0;

  const show = (el: HTMLElement, on: boolean) => (el.style.visibility = on ? '' : 'hidden');
  const count = () => {
    received += 1;
    onCount(received);
    report(desk, `messages received ${received}/${total}`);
  };

  function popWindow(win: HTMLElement, origin?: Point) {
    if (origin) {
      const own = boxIn(win, desk);
      win.style.transformOrigin = `${origin[0] - own.x}px ${origin[1] - own.y}px`;
    }
    else win.style.transformOrigin = '50% 0%';
    show(win, true);
    win.animate(POP, { duration: (POP.length - 1) * FRAME, fill: 'backwards' });
    count();
  }

  async function grow(w: Wire) {
    const duration = Math.round(clamp(w.length / SPEED, ...GROW));
    w.jack.dataset.live = '';
    show(w.head, true);
    const opts: KeyframeAnimationOptions = { duration, easing: 'linear', fill: 'forwards' };
    const anims = [
      w.rail.animate({ strokeDashoffset: [`${w.length}`, '0'] }, opts),
      w.core.animate({ strokeDashoffset: [`${w.length}`, '0'] }, opts),
      w.head.animate({ offsetDistance: ['0%', '100%'] }, opts),
      ...w.clips.map(({ el, at }) =>
        el.animate({ opacity: [0, 1] }, { duration: 1, delay: Math.round(at * duration), fill: 'forwards' }),
      ),
    ];
    w.running = anims;
    if (clock.held) anims.forEach((a) => a.pause());
    await Promise.all(anims.map((a) => a.finished.catch(() => undefined)));
    w.running = [];
    w.drawn = true;
    for (const a of anims) a.cancel();
    for (const p of [w.rail, w.core]) p.style.strokeDashoffset = '0';
    for (const { el } of w.clips) el.style.opacity = '1';
    show(w.head, false);
    delete w.jack.dataset.live;
  }

  async function playWide() {
    for (const win of wins) {
      const w = wires.find((x) => x.win === win);
      if (w) await grow(w);
      if (dead) return;
      popWindow(win, w?.tip);
      await clock.wait(NEXT);
      if (dead) return;
    }
    report(desk, `messages received ${total}/${total}`);
  }

  /** everything in its final state */
  function finalize() {
    dead = true;
    clock.stop();
    for (const w of wires) {
      for (const a of w.running) a.cancel();
      w.drawn = true;
      show(w.head, false);
      delete w.jack.dataset.live;
    }
    wins.forEach((win) => {
      win.getAnimations().forEach((a) => a.cancel());
      show(win, true);
    });
    layout();
    onCount(total);
    report(desk, `messages received ${total}/${total}`);
  }

  layout();
  const resize = new ResizeObserver(() => layout());
  resize.observe(desk);
  const onMq = () => {
    if (mq.matches !== startedWide) finalize();
  };
  mq.addEventListener('change', onMq);

  const stops: (() => void)[] = [];
  if (!animate) {
    onCount(total);
    report(desk, `messages received ${total}/${total}`);
  } else {
    wins.forEach((win) => show(win, false));
    onCount(0);
    report(desk, `messages received 0/${total}`);
    if (startedWide) {
      let started = false;
      const thresholds = Array.from({ length: 21 }, (_, i) => i / 20);
      const io = new IntersectionObserver(
        (entries) => {
          const e = entries[entries.length - 1];
          const seen = Math.max(e.intersectionRatio, e.intersectionRect.height / window.innerHeight);
          if (seen >= 0.22) {
            clock.hold(false);
            if (!started) {
              started = true;
              void playWide();
            }
          } else if (seen < 0.08) clock.hold(true);
        },
        { threshold: thresholds },
      );
      io.observe(desk);
      stops.push(() => io.disconnect());
    } else {
      const io = new IntersectionObserver(
        (entries) => {
          for (const e of entries) {
            if (!e.isIntersecting) continue;
            io.unobserve(e.target);
            const win = e.target as HTMLElement;
            const delay = (wins.indexOf(win) % 2) * 120;
            clock.wait(delay).then(() => {
              if (!dead) popWindow(win);
            });
          }
        },
        { threshold: 0.25 },
      );
      wins.forEach((win) => io.observe(win));
      stops.push(() => io.disconnect());
    }
  }

  return () => {
    dead = true;
    clock.stop();
    resize.disconnect();
    mq.removeEventListener('change', onMq);
    stops.forEach((s) => s());
    for (const w of wires) {
      for (const a of w.running) a.cancel();
      w.svg.remove();
      w.head.remove();
      delete w.jack.dataset.live;
    }
    wins.forEach((win) => {
      win.getAnimations().forEach((a) => a.cancel());
      show(win, true);
    });
  };
}
