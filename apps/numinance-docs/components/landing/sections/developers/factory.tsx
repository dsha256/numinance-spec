'use client';

import { useEffect, useRef } from 'react';
import { motion } from 'motion/react';
import { FileText, Folder } from 'lucide-react';
import { cn } from '@/lib/cn';
import { BAYER4 } from '@/lib/landing/dither';
import { SYMBOL_PIXELS } from '@/lib/landing/operator';
import { Pixels } from '@/components/landing/machine/pixels';
import { Shadow, Window } from '@/components/landing/ui/window';
import type { CliClock } from './clock';
import { RAM, TRAVEL, type Derived, type Frame } from './timeline';

/* ---------- geometry (viewBox 400 × 236) ---------- */

const VB_W = 400;
const VB_H = 236;

function gearPath(cx: number, cy: number, rOut: number, rIn: number, teeth: number, phase = 0): string {
  const pts: string[] = [];
  const step = (Math.PI * 2) / teeth;
  for (let i = 0; i < teeth; i++) {
    const a = phase + i * step;
    const q = step / 4;
    const p = (r: number, ang: number) => `${(cx + Math.cos(ang) * r).toFixed(1)},${(cy + Math.sin(ang) * r).toFixed(1)}`;
    pts.push(p(rIn, a - q * 1.6), p(rOut, a - q * 0.8), p(rOut, a + q * 0.8), p(rIn, a + q * 1.6));
  }
  return `M${pts.join('L')}Z`;
}

/** pixel threshold pattern from the shared 4×4 Bayer matrix: `n` of 16 pixels filled */
function DitherPattern({ id, n }: { id: string; n: number }) {
  const cells: string[] = [];
  for (let y = 0; y < 4; y++) for (let x = 0; x < 4; x++) if (BAYER4[y][x] < n) cells.push(`M${x} ${y}h1v1h-1z`);
  return (
    <pattern id={id} width="4" height="4" patternUnits="userSpaceOnUse">
      <path d={cells.join('')} className="fill-lp-ink-2" shapeRendering="crispEdges" />
    </pattern>
  );
}

const GEAR_A = { cx: 176, cy: 112, ro: 25, ri: 19, n: 10 };
const GEAR_B = { cx: 209, cy: 86, ro: 17, ri: 12.5, n: 7 };

/** chip path in percent of the machine box: crate outlet → along the belt → down the chute into the tree */
const X0 = 35;
const X1 = 73;
const Y_BELT = 74;
const Y_END = 112;

export function Factory({ d, reduced, clock }: { d: Derived; reduced: boolean; clock: CliClock }) {
  const { w, landed, launched } = d;
  const n = w.files.length;
  const box = useRef<HTMLDivElement>(null);
  const gearA = useRef<SVGGElement>(null);
  const gearB = useRef<SVGGElement>(null);
  const ram = useRef<SVGRectElement>(null);
  const chips = useRef<(HTMLSpanElement | null)[]>([]);
  const size = useRef({ w: 0, h: 0 });
  const angle = useRef(0);

  // cached box size: the per-frame writer never reads layout
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => {
      size.current = { w: e.contentRect.width, h: e.contentRect.height };
    });
    ro.observe(el);
    size.current = { w: el.clientWidth, h: el.clientHeight };
    return () => ro.disconnect();
  }, []);

  // everything continuous is a function of the clock: spin, ram stroke, files in transit
  useEffect(() => {
    if (reduced) return;
    const spin = (f: Frame, dt: number) => {
      // gear A turns once per 3.6s at full spin; B meshes at the tooth ratio. Spin ramps with the machine's power.
      angle.current = (angle.current + (f.spin * dt * 360) / 3600) % 36000;
      gearA.current?.setAttribute('transform', `rotate(${angle.current.toFixed(2)} ${GEAR_A.cx} ${GEAR_A.cy})`);
      gearB.current?.setAttribute('transform', `rotate(${((-angle.current * GEAR_A.n) / GEAR_B.n).toFixed(2)} ${GEAR_B.cx} ${GEAR_B.cy})`);
      // ram: out and back for each launch
      let r = 0;
      for (let i = 0; i < f.p.launch.length; i++) {
        const u = (f.t - f.p.launch[i]) / RAM;
        if (u > 0 && u < 1) r = Math.max(r, Math.sin(Math.PI * u));
      }
      ram.current?.setAttribute('x', (107 + 20 * r).toFixed(2));
      // files in transit
      const { w: W, h: H } = size.current;
      for (let i = 0; i < f.p.launch.length; i++) {
        const el = chips.current[i];
        if (!el) continue;
        const u = (f.t - f.p.launch[i]) / TRAVEL;
        if (u <= 0 || u >= 1) {
          if (el.style.visibility !== 'hidden') el.style.visibility = 'hidden';
          continue;
        }
        // out of the crate, along the belt (linear), then down the chute (gravity)
        let x: number;
        let y: number;
        let o: number;
        if (u < 0.7) {
          const k = u / 0.7;
          x = k < 0.11 ? X0 + (4 * k) / 0.11 : X0 + 4 + (X1 - X0 - 4) * ((k - 0.11) / 0.89);
          y = Y_BELT;
          o = Math.min(1, u / 0.056);
        } else {
          const k = (u - 0.7) / 0.3;
          x = X1;
          y = Y_BELT + (Y_END - Y_BELT) * k * k;
          o = 1 - 0.1 * k;
        }
        el.style.visibility = 'visible';
        el.style.opacity = o.toFixed(2);
        // alternate files ride two lanes so close-following labels do not sit on top of each other
        const lane = i % 2 ? -22 : 0;
        el.style.transform = `translate3d(${((x / 100) * W).toFixed(1)}px, ${((y / 100) * H + lane * (1 - Math.max(0, (u - 0.7) / 0.3))).toFixed(1)}px, 0)`;
      }
    };
    spin(clock.frame, 0);
    return clock.subscribe(spin);
  }, [clock, reduced, w.id]);

  const lit = d.power > 0;
  const belt = d.power === 2 && !reduced;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div ref={box} className="relative w-full" style={{ aspectRatio: `${VB_W} / ${VB_H}` }}>
        <svg viewBox={`0 0 ${VB_W} ${VB_H}`} aria-hidden className="absolute inset-0 size-full overflow-visible text-lp-ink" fill="none" strokeWidth={1.25}>
          <defs>
            <DitherPattern id="cli-d6" n={6} />
            <DitherPattern id="cli-d10" n={10} />
            <pattern id="cli-hazard" width="12" height="12" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
              <rect width="12" height="12" className="fill-lp-yellow" />
              <rect width="6" height="12" className="fill-lp-ink" />
            </pattern>
          </defs>

          {/* floor + frame ticks */}
          <path d="M0 216H400" className="stroke-lp-line-strong" strokeDasharray="2 4" />
          <path d="M0 233H400" className="stroke-lp-line" />

          {/* registry cable: source plate → crate */}
          <path d="M46 40V78" className="stroke-lp-ink-2" strokeDasharray="3 3" style={lit && !reduced ? { animation: 'lp-dash 0.5s linear infinite' } : undefined} />
          <path d="M42 70l4 6 4-6z" className={lit ? 'fill-lp-sun stroke-lp-ink' : 'fill-lp-paper stroke-lp-ink-2'} />

          {/* crate */}
          <rect x="9" y="84" width="124" height="106" className="fill-lp-paper-2 stroke-lp-ink" />
          <rect x="125" y="86" width="6" height="102" fill="url(#cli-d6)" stroke="none" />
          <rect x="11" y="182" width="114" height="6" fill="url(#cli-d6)" stroke="none" />
          <rect x="9" y="84" width="124" height="11" fill="url(#cli-hazard)" className="stroke-lp-ink" />
          {/* slats */}
          {[112, 128, 144].map((y) => (
            <path key={y} d={`M17 ${y}H117`} className="stroke-lp-line-strong" />
          ))}
          {/* symbol stamp on a hatch plate */}
          <rect x="18" y="146" width="30" height="30" className="fill-lp-paper stroke-lp-ink" />
          <g transform="translate(24 152)">
            <Pixels rows={SYMBOL_PIXELS} s={3} />
          </g>
          {/* lamp */}
          <circle cx="112" cy="104" r="5.5" className={cn('stroke-lp-ink transition-colors', lit ? 'fill-lp-sun' : 'fill-lp-paper')} />
          {/* outlet slot + ram */}
          <rect x="125" y="156" width="20" height="26" className="fill-lp-paper stroke-lp-ink" />
          <rect ref={ram} x="107" y="161" width="16" height="16" className="fill-lp-orange stroke-lp-ink" />

          {/* drive shaft crate → gear */}
          <path d="M133 114H151" className="stroke-lp-ink" strokeWidth={3} />

          {/* gears */}
          <g ref={gearB}>
            <path d={gearPath(GEAR_B.cx, GEAR_B.cy, GEAR_B.ro, GEAR_B.ri, GEAR_B.n, 0.25)} className="fill-lp-paper stroke-lp-ink" />
            <circle cx={GEAR_B.cx} cy={GEAR_B.cy} r="4" className="fill-lp-paper-2 stroke-lp-ink" />
            <path d={`M${GEAR_B.cx - 9} ${GEAR_B.cy}H${GEAR_B.cx + 9}M${GEAR_B.cx} ${GEAR_B.cy - 9}V${GEAR_B.cy + 9}`} className="stroke-lp-ink-2" />
          </g>
          <g ref={gearA}>
            <path d={gearPath(GEAR_A.cx, GEAR_A.cy, GEAR_A.ro, GEAR_A.ri, GEAR_A.n)} className="fill-lp-paper stroke-lp-ink" />
            <circle cx={GEAR_A.cx} cy={GEAR_A.cy} r="13" className="fill-lp-paper-2 stroke-lp-ink-2" />
            <path
              d={[0, 1, 2, 3].map((k) => `M${GEAR_A.cx} ${GEAR_A.cy}L${(GEAR_A.cx + Math.cos((k * Math.PI) / 2) * 13).toFixed(1)} ${(GEAR_A.cy + Math.sin((k * Math.PI) / 2) * 13).toFixed(1)}`).join('')}
              className="stroke-lp-ink-2"
            />
            <circle cx={GEAR_A.cx} cy={GEAR_A.cy} r="5" className="fill-lp-sun stroke-lp-ink" />
          </g>
          <path d="M167 140l-8 8h34l-8-8z" fill="url(#cli-d6)" stroke="none" />
          <rect x="151" y="145" width="50" height="3" className="fill-lp-ink" stroke="none" />

          {/* belt */}
          <path d="M146 180H322a9 9 0 0 1 0 18H146a9 9 0 0 1 0-18z" className="fill-lp-paper-2 stroke-lp-ink" />
          <path d="M152 189H316" className="stroke-lp-ink-2" strokeDasharray="6 6" style={belt ? { animation: 'lp-dash 0.55s linear infinite' } : undefined} />
          <circle cx="146" cy="189" r="3" className="fill-lp-ink" stroke="none" />
          <circle cx="322" cy="189" r="3" className="fill-lp-ink" stroke="none" />
          {[168, 236, 300].map((x) => (
            <path key={x} d={`M${x} 198V216M${x - 8} 216H${x + 8}`} className="stroke-lp-ink" />
          ))}
          <rect x="152" y="199" width="164" height="5" fill="url(#cli-d6)" stroke="none" />

          {/* checking gantry */}
          <path d="M236 180V138M272 180V138" className="stroke-lp-ink" strokeWidth={2} />
          <rect x="230" y="128" width="48" height="12" className="fill-lp-paper stroke-lp-ink" />
          <circle cx="254" cy="134" r="3.5" className={cn('stroke-lp-ink', d.power === 2 ? 'fill-lp-sun' : 'fill-lp-paper')} />
          {d.power === 2 && !reduced && <path d="M240 140H268" className="stroke-lp-sun" strokeWidth={2} style={{ animation: 'lp-caret 0.8s steps(1,end) infinite' }} />}

          {/* drop chute */}
          <path d="M306 198l-6 20h46l-6-20z" className="fill-lp-paper stroke-lp-ink" />
          <path d="M300 218v8M346 218v8" className="stroke-lp-ink" />
          <path d="M320 228l4 5 4-5z" className={d.power === 2 ? 'fill-lp-sun' : 'fill-lp-ink-2'} stroke="none" />
        </svg>

        {/* source / registry plate */}
        <p className="absolute top-0 left-0 z-2 flex max-w-[64%] flex-col gap-0.5 border border-lp-line-strong bg-lp-paper px-2.5 py-1.5 leading-none">
          <span className="font-pixel text-[11px] tracking-wide text-lp-ink-2">
            SOURCE
          </span>
          <code className="truncate text-xs leading-4">{w.source.name}</code>
          <span className="truncate font-mono text-[11px] leading-4 text-lp-orange">{w.source.note}</span>
        </p>

        {/* file counter */}
        <Counter count={landed} />

        {/* machine readout */}
        <div className="absolute top-[24%] right-0 z-2 flex w-[35%] min-w-32 flex-col gap-1.5 border border-lp-line-strong bg-lp-paper px-2.5 py-2">
          <p className="flex items-center gap-1.5 font-pixel text-[13px] leading-none">
            <span aria-hidden className={cn('size-2 shrink-0 border border-lp-ink', lit ? 'bg-lp-sun' : 'bg-lp-paper')} />
            <span className="truncate">{d.label.toUpperCase()}</span>
          </p>
          <div aria-hidden className="flex gap-0.5">
            {Array.from({ length: n }, (_, i) => (
              <span key={i} className={cn('h-2 flex-1 border border-lp-ink transition-colors', landed > i ? 'bg-lp-sun' : launched > i ? 'bg-lp-yellow' : 'bg-lp-paper')} />
            ))}
          </div>
          <p className="truncate font-mono text-[11px] leading-none text-lp-ink-2">
            {landed}/{n} landed
          </p>
        </div>

        {/* files in transit: positioned by the clock, one element per file */}
        {!reduced &&
          w.files.map((f, i) => (
            <span
              key={`${w.id}-${i}`}
              ref={(el) => {
                chips.current[i] = el;
              }}
              aria-hidden
              className="pointer-events-none absolute top-0 left-0 z-3 flex h-5 items-center gap-1 border border-lp-ink bg-lp-paper px-1.5 font-mono text-[11px] leading-none whitespace-nowrap shadow-[2px_2px_0_var(--lp-orange)] will-change-transform"
              style={{ translate: '-50% -50%', visibility: 'hidden' }}
            >
              <FileText aria-hidden className="size-3 text-lp-orange" />
              {f.path[f.path.length - 1]}
            </span>
          ))}
      </div>

      <TreeWindow d={d} className="mt-1 flex-1" />
    </div>
  );
}

function Counter({ count }: { count: number }) {
  const digits = String(count).split('');
  return (
    <p className="absolute top-0 right-0 z-2 flex items-center gap-2 border border-lp-line-strong bg-lp-paper px-2.5 py-1.5" data-counter>
      <span aria-hidden className="flex gap-px">
        {digits.map((c, i) => (
          <motion.span
            key={`${i}-${c}`}
            initial={{ rotateX: -90 }}
            animate={{ rotateX: 0 }}
            transition={{ duration: 0.22 }}
            className="relative grid h-6 w-4 place-items-center overflow-hidden bg-lp-ink font-pixel text-base leading-none text-lp-sun after:absolute after:inset-x-0 after:top-1/2 after:h-px after:bg-lp-paper dark:bg-lp-paper-2"
          >
            {c}
          </motion.span>
        ))}
      </span>
      <span className="font-pixel text-xs text-lp-ink-2">
        <span className="sr-only">{count} </span>
        {count === 1 ? 'file' : 'files'}
      </span>
    </p>
  );
}

function TreeWindow({ d, className }: { d: Derived; className?: string }) {
  const { w, landed } = d;
  const rows = w.tree.filter((r) => landed > r.file);
  return (
    <div className={cn('relative isolate', className)}>
      <Shadow />
      <Window className="relative h-full" title={<>{w.root}/ <span className="text-lp-ink-2">· illustrative layout</span></>}>
        <ul className="min-h-[9.5rem] flex-1 overflow-y-auto py-1.5 pe-2 font-mono text-xs leading-[22px]" aria-label={`Project tree, ${landed} of ${w.files.length} files`}>
          {rows.map((r) => {
            const fresh = !r.dir && r.file === landed - 1 && d.edit === 'initial';
            return (
              <motion.li
                key={w.id + r.name + r.depth}
                initial={{ opacity: 0, x: -8 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.25 }}
                className={cn('flex items-center gap-1.5 whitespace-nowrap', fresh && 'bg-lp-yellow/30')}
                style={{ paddingInlineStart: 10 + r.depth * 14 }}
              >
                {r.dir ? <Folder aria-hidden className="size-3.5 shrink-0 text-lp-ink-2" /> : <FileText aria-hidden className="size-3.5 shrink-0 text-lp-ink-2" />}
                <span className={r.dir ? 'text-lp-ink' : 'text-lp-ink'}>{r.name}{r.dir ? '/' : ''}</span>
                {!r.dir && <span aria-hidden className={cn('ms-auto pe-1 font-pixel text-[11px]', fresh ? 'text-lp-orange' : 'text-lp-ink-2')}>+</span>}
              </motion.li>
            );
          })}
          {rows.length === 0 && <li className="px-3 font-pixel text-[13px] text-lp-ink-2">empty — nothing dispensed yet</li>}
        </ul>
      </Window>
    </div>
  );
}
