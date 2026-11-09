'use client';

import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { Lock, RotateCcw, Check } from 'lucide-react';
import { Shadow, Window } from '@/components/landing/ui/window';
import { cn } from '@/lib/cn';
import { DITHER_MASKS } from '@/lib/landing/dither';
import { scopedStatus } from '@/lib/landing/status';
const setStatus = scopedStatus('governance');
import { Ladder } from './ladder';
import { Clerk, type ClerkMode } from './clerk';
import {
  DELAYS,
  HOLD,
  LAST,
  PASS,
  PASS_M,
  B_LOW,
  LADDERS,
  ROW_LABELS,
  SCENARIOS,
  resolveLevel,
  START,
  START_M,
  statusFor,
  type LevelId,
  type ScenarioId,
} from './data';
import './governance.css';

const pad = (n: number) => String(n).padStart(2, '0');

export function Machine({ onScenario }: { onScenario?: (s: ScenarioId) => void }) {
  const reduced = useReducedMotion() === true;
  const [sid, setSid] = useState<ScenarioId>('A');
  // the phase belongs to the run that produced it: a stale phase from another run can never show for one frame
  const [clock, setClock] = useState({ key: '', p: 0 });
  const [run, setRun] = useState(0);
  const [level, setLevel] = useState<LevelId | null>(null);
  const [started, setStarted] = useState(false);
  const [nudge, setNudge] = useState(0);
  const root = useRef<HTMLDivElement>(null);
  const inView = useRef(false);
  const resume = useRef<(() => void) | null>(null);

  // auto-play once when scrolled into view
  useEffect(() => {
    const el = root.current;
    if (!el) return;
    const io = new IntersectionObserver(
      ([e]) => {
        inView.current = e.isIntersecting;
        if (e.isIntersecting) {
          setStarted(true);
          resume.current?.();
        }
      },
      { threshold: 0.1 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);

  const requested = level ?? LADDERS[sid].configured;
  const { effective, permitted } = resolveLevel(sid, requested);
  const key = `${sid}|${run}|${permitted ? 1 : 0}`;

  // the phase clock: one timeout chain per run. Off screen or in a hidden tab it simply stops (no polling) and is
  // resumed by the IntersectionObserver / visibilitychange.
  useEffect(() => {
    if (!started || reduced) return;
    let p = 0;
    let t: ReturnType<typeof setTimeout> | undefined;
    const live = () => inView.current && !document.hidden;
    const next = () => {
      resume.current = null;
      if (p >= LAST) return;
      t = setTimeout(() => {
        if (!live()) {
          resume.current = next;
          return;
        }
        p += 1;
        setClock({ key, p });
        next();
      }, DELAYS[p]);
    };
    const vis = () => {
      if (live()) resume.current?.();
    };
    document.addEventListener('visibilitychange', vis);
    next();
    return () => {
      clearTimeout(t);
      resume.current = null;
      document.removeEventListener('visibilitychange', vis);
    };
  }, [started, reduced, key]);

  const shown = reduced ? LAST : clock.key === key ? clock.p : 0;
  const s = sid === 'B' && !permitted ? B_LOW : SCENARIOS[sid];
  const held = !permitted;
  const interacted = level !== null;
  const verdict = shown >= 8;
  const open = !held && shown >= 8;
  const passed = !held && shown >= 9;
  const printed = held ? shown >= 9 : shown >= 10;
  const bay: 'standby' | 'running' | 'done' | 'locked' = held
    ? verdict
      ? 'locked'
      : 'standby'
    : shown >= 10
      ? 'done'
      : shown >= 9
        ? 'running'
        : 'standby';
  const mode: ClerkMode = shown >= 8 ? (held ? 'hold' : 'ok') : shown >= 1 ? 'read' : 'idle';
  const gateLamp = open ? 'ok' : verdict ? 'hold' : shown >= 1 ? 'eval' : 'idle';

  useEffect(() => {
    if (!inView.current) return;
    const line = statusFor(sid, shown);
    if (line) setStatus(line);
  }, [sid, shown]);

  const pick = (id: ScenarioId) => {
    if (id !== sid) {
      setSid(id);
      setLevel(null);
      onScenario?.(id);
    } else setRun((r) => r + 1);
    setStarted(true);
  };
  const pickLevel = (id: LevelId) => {
    setLevel(id);
    setStatus(`policy: requested ${id} → effective ${resolveLevel(sid, id).effective}`);
  };

  const ticket = {
    '--pd': `${(passed ? PASS : shown >= 1 ? HOLD : START) * 100}%`,
    '--pm': `${(passed ? PASS_M : shown >= 1 ? HOLD : START_M) * 100}%`,
  } as CSSProperties;

  return (
    <div ref={root} className="relative isolate">
      <Shadow />
      <Window title="policy-gate · concept demonstration" className="relative">
        {/* controls */}
        <div className="flex flex-col gap-2 border-b border-lp-line-strong p-2 md:flex-row md:items-stretch">
          <div role="group" aria-label="Scenario" className="grid flex-1 gap-2 md:grid-cols-2">
            {(['A', 'B'] as const).map((id) => {
              const sc = SCENARIOS[id];
              const on = id === sid;
              return (
                <button
                  key={id}
                  type="button"
                  aria-pressed={on}
                  onClick={() => pick(id)}
                  className={cn(
                    'flex min-h-11 items-center gap-3 border px-2.5 py-1.5 text-start transition-colors',
                    on
                      ? 'border-lp-ink bg-lp-yellow text-lp-on-yellow'
                      : 'border-lp-line-strong bg-lp-paper hover:bg-lp-paper-2',
                  )}
                >
                  <span className="grid size-6 shrink-0 place-items-center border border-current font-pixel text-[13px]">{id}</span>
                  <span className="min-w-0 flex-1 font-mono text-[12.5px] leading-snug">{sc.label}</span>
                  <span className="shrink-0 border border-current px-1.5 font-pixel text-[11px] leading-5">{sc.risk}</span>
                </button>
              );
            })}
          </div>
          <button
            type="button"
            onClick={() => pick(sid)}
            className="flex min-h-11 items-center justify-center gap-2 border border-lp-line-strong bg-lp-paper px-3 font-pixel text-[13px] hover:bg-lp-paper-2"
          >
            <RotateCcw aria-hidden className="size-3.5" />
            Replay
          </button>
        </div>

        <div className="grid md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_380px]">
          {/* stage */}
          <div
            aria-hidden
            className="relative isolate h-[600px] overflow-hidden border-b border-lp-line-strong md:col-span-2 md:h-[380px] xl:col-span-1 xl:col-start-1 xl:row-start-1 xl:h-[470px] xl:border-r"
          >
            <div className="lp-dots absolute inset-0 opacity-30" />
            <span className="absolute top-2 left-2 font-pixel text-[11px] text-lp-ink-2">INTAKE · proposed action</span>
            <div
              aria-hidden
              className="absolute inset-x-0 bottom-0 h-2 opacity-60"
              style={{ backgroundImage: 'repeating-linear-gradient(90deg, var(--lp-line-strong) 0 1px, transparent 1px 12px)' }}
            />
            <div className="gov-rail" />

            {/* plate */}
            <div className="absolute z-30 max-md:top-[calc(44%+24px)] max-md:left-3 md:top-2.5 md:left-[44%] md:-translate-x-1/2">
              <div className="relative isolate">
                <Shadow />
                <div className="relative flex items-center gap-2 border border-lp-ink bg-lp-paper px-2.5 py-1 font-pixel text-[12px] whitespace-nowrap">
                  <span className="gov-lamp" data-s={gateLamp} />
                  POLICY · OUTSIDE THE MODEL
                </div>
              </div>
            </div>

            {/* gate */}
            <div
              data-open={open || undefined}
              data-shake={held && shown === 8 ? '' : undefined}
              className="gov-gate absolute z-10 flex max-md:inset-x-3 max-md:top-[44%] max-md:h-8 max-md:-translate-y-1/2 md:inset-y-14 md:left-[44%] md:w-8 md:-translate-x-1/2 md:flex-col"
            >
              <Post lamp={gateLamp} />
              <div className="relative flex-1 overflow-hidden border border-lp-line-strong max-md:border-x-0 md:border-y-0">
                <div
                  className="gov-field absolute inset-0 bg-lp-ink-2"
                  style={{ maskImage: DITHER_MASKS[7], maskSize: '4px 4px' }}
                />
                <div className="gov-arm lp-hazard absolute border-lp-ink max-md:inset-x-0 max-md:inset-y-1 max-md:border-y md:inset-x-1 md:inset-y-0 md:border-x" />
              </div>
              <Post lamp={gateLamp} />
            </div>

            {/* clerk */}
            <Clerk
              mode={mode}
              className="absolute z-20 h-16 w-14 max-md:top-[calc(44%+22px)] max-md:right-3 md:bottom-3 md:left-[calc(44%+26px)]"
            />

            {/* tool bay */}
            <div
              data-s={bay}
              className="gov-bay absolute z-10 flex flex-col overflow-hidden border border-lp-line-strong bg-lp-paper max-md:inset-x-3 max-md:top-[58%] max-md:bottom-3 md:inset-y-3 md:right-3 md:left-[58%]"
            >
              <div className="relative flex h-7 shrink-0 items-center gap-2 border-b border-lp-line-strong px-2 font-pixel text-[11px]">
                <span className="gov-lamp" data-s={bay === 'running' ? 'eval' : bay === 'done' ? 'ok' : 'idle'} />
                TOOL EXECUTION BAY
              </div>
              <div className="gov-cells absolute inset-x-2 top-9 hidden gap-1 md:flex">
                {Array.from({ length: 18 }, (_, i) => (
                  <i key={i} style={{ '--i': i } as CSSProperties} className="h-3 flex-1 border border-lp-line-strong" />
                ))}
              </div>
              <div className="relative min-h-0 flex-1 md:absolute md:inset-x-0 md:top-1/2 md:h-32 md:flex-none md:-translate-y-1/2">
                <div
                  className={cn(
                    'absolute inset-2 border border-dashed transition-colors',
                    bay === 'running' || bay === 'done' ? 'border-lp-ink bg-lp-yellow/15' : 'border-lp-line-strong',
                  )}
                />
                {bay === 'locked' && (
                  <div className="lp-hazard absolute inset-2 flex items-center justify-center opacity-90">
                    <span className="bg-lp-paper px-2 py-0.5 font-pixel text-[11px] text-lp-ink">NOT REACHED</span>
                  </div>
                )}
                {bay === 'running' && <div className="gov-sweep absolute inset-x-2 top-0 h-6 bg-lp-yellow/40" />}
              </div>
              <div className="shrink-0 space-y-1.5 border-t border-lp-line-strong p-2 font-mono text-[11.5px] md:absolute md:inset-x-0 md:bottom-0">
                <div className="flex justify-between gap-2">
                  <span className="text-lp-ink-2">tool</span>
                  <span>type check · workspace</span>
                </div>
                <div className="gov-bar h-2 border border-lp-line-strong">
                  <i />
                </div>
                <div className="flex justify-between gap-2">
                  <span className="text-lp-ink-2">result</span>
                  <span>
                    {bay === 'done' ? 'recorded · ok (illustr.)' : bay === 'running' ? 'running…' : bay === 'locked' ? 'no execution' : 'standby'}
                  </span>
                </div>
              </div>
            </div>

            {/* the ticket */}
            <div
              style={ticket}
              className="absolute z-20 w-[200px] transition-[left,top] duration-1000 ease-in-out max-md:top-(--pm) max-md:left-1/2 max-md:-translate-x-1/2 max-md:-translate-y-full motion-reduce:transition-none md:top-1/2 md:left-(--pd) md:w-[clamp(150px,19%,220px)] md:-translate-x-full md:-translate-y-1/2"
            >
              <div className="relative isolate">
                <Shadow />
                <div className="relative border border-lp-ink bg-lp-paper">
                  <div className="flex items-center justify-between gap-2 border-b border-lp-line-strong px-2 py-1 font-pixel text-[11px]">
                    <span>ACTION REQUEST</span>
                    <span className="text-lp-ink-2">proposed</span>
                  </div>
                  <p className="px-2 pt-2 pb-1 font-mono text-[12.5px] leading-snug">{s.action}</p>
                  <div
                    className="mx-2 mt-1 mb-8 h-3.5 opacity-80"
                    style={{
                      backgroundImage:
                        'repeating-linear-gradient(90deg, var(--lp-ink) 0 1px, transparent 1px 3px, var(--lp-ink) 3px 5px, transparent 5px 6px, var(--lp-ink) 6px 7px, transparent 7px 10px)',
                    }}
                  />
                  <span
                    data-on={verdict || undefined}
                    className={cn(
                      'gov-stamp absolute right-1.5 bottom-1.5 -rotate-4 border-2 bg-lp-paper px-1.5 py-0.5 font-pixel text-[11px] leading-4 whitespace-nowrap',
                      held ? 'border-lp-sun text-lp-orange' : 'border-lp-indigo text-lp-indigo',
                    )}
                  >
                    {s.result}
                  </span>
                </div>
              </div>
            </div>
          </div>

          {/* readouts + printer */}
          <div className="xl:col-start-2 xl:row-span-2 xl:row-start-1 xl:border-b-0">
            <div>
              <div className="flex items-center justify-between border-b border-lp-line px-3 py-1.5 font-pixel text-[11px] text-lp-ink-2">
                <span>GATE READOUTS</span>
                <span>illustrative · risk row: RiskClass member</span>
              </div>
              <ol>
                {ROW_LABELS.map((label, i) => {
                  const st = shown < 2 + i ? 'pending' : shown === 2 + i ? 'active' : 'done';
                  const row = s.rows[i];
                  return (
                    <li
                      key={label}
                      data-s={st}
                      className={cn(
                        'flex min-h-11 items-center gap-2.5 border-b border-lp-line px-3 py-1.5',
                        st === 'active' && 'bg-lp-yellow text-lp-on-yellow',
                        st === 'pending' && 'text-lp-ink-2',
                      )}
                    >
                      <span className="font-pixel text-[11px] tabular-nums opacity-70">{pad(i + 1)}</span>
                      <div className="min-w-0 flex-1">
                        <div className="font-pixel text-[11px] tracking-wide uppercase opacity-75">{label}</div>
                        <div className="font-mono text-[12.5px] leading-snug">
                          {st === 'pending' ? '· · · · · ·' : row.v}
                          {st === 'active' && <span className="gov-caret ms-0.5">▌</span>}
                        </div>
                      </div>
                      {st === 'done' && row.f === 'flag' && (
                        <span aria-label="flagged" className="lp-hazard size-3 shrink-0 border border-lp-ink" />
                      )}
                      {st === 'done' && row.f === 'ok' && <Check aria-label="ok" className="size-3.5 shrink-0 text-lp-indigo" />}
                    </li>
                  );
                })}
              </ol>
              <div className="flex min-h-14 items-stretch border-b border-lp-line-strong md:border-b-0 xl:border-b">
                <span className={cn('w-2 shrink-0', verdict && held && 'lp-hazard', verdict && !held && 'bg-lp-indigo')} />
                <div className="flex-1 px-3 py-2">
                  <div className="font-pixel text-[11px] text-lp-ink-2">RESULT</div>
                  <div className={cn('font-pixel text-[17px] leading-6', verdict && (held ? 'text-lp-orange' : 'text-lp-indigo'))}>
                    {verdict ? s.result : '— — —'}
                  </div>
                </div>
              </div>
            </div>
            <Ladder sid={sid} shown={shown} requested={requested} effective={effective} interacted={interacted} onPick={pickLevel} />
          </div>
          <div className="flex flex-col gap-3 p-3 md:border-l md:border-lp-line xl:col-start-1 xl:row-start-2 xl:grid xl:grid-cols-[1fr_340px] xl:border-t xl:border-r xl:border-l-0 xl:border-lp-line-strong">
              <div className="relative min-h-40 border border-dashed border-lp-line-strong bg-lp-paper-2 px-3 pt-4 pb-3">
                <span className="absolute -top-2 left-3 bg-lp-paper px-1.5 font-pixel text-[11px] text-lp-ink-2">GATE PRINTER</span>
                <ul className="space-y-1 font-mono text-[12px] leading-snug">
                  {printed ? (
                    <>
                      <li key={sid + run + 'h'} className="gov-line font-semibold" style={{ '--i': 0 } as CSSProperties}>
                        DECISION · {s.result}
                      </li>
                      {s.why.map((w, i) => (
                        <li key={sid + run + i} className="gov-line" style={{ '--i': i + 1 } as CSSProperties}>
                          ▸ {w}
                        </li>
                      ))}
                      {!held && (
                        <li key={sid + run + 'l'} className="gov-line text-lp-ink-2" style={{ '--i': 5 } as CSSProperties}>
                          log › {s.log}
                        </li>
                      )}
                    </>
                  ) : (
                    <li className="text-lp-ink-2">{shown === 0 ? 'waiting for a request…' : 'evaluating…'}</li>
                  )}
                </ul>
              </div>

              <div className="flex items-center gap-3 border border-lp-line-strong px-2.5 py-2">
                <AgentGlyph />
                <div className="min-w-0 flex-1">
                  <div className="font-pixel text-[11px] text-lp-ink-2">MODEL / AGENT</div>
                  <div className="text-[12px] leading-snug text-lp-ink-2">No override path. The decision sits outside the model.</div>
                </div>
                <button
                  type="button"
                  aria-disabled="true"
                  key={nudge}
                  data-nudge={nudge ? '' : undefined}
                  onClick={() => {
                    setNudge((n) => n + 1);
                    setStatus('override: not available to the model');
                  }}
                  className="gov-override flex min-h-11 shrink-0 cursor-not-allowed items-center gap-1.5 border border-dashed border-lp-line-strong px-2.5 font-pixel text-[12px] text-lp-ink-2 opacity-70"
                >
                  <Lock aria-hidden className="size-3.5" />
                  override
                </button>
              </div>
            </div>
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-lp-line-strong px-3 py-2 font-pixel text-[11px] text-lp-ink-2">
          <span className="text-lp-ink">concept demonstration — not a production run</span>
          <span>stamps are display text · outcomes map to DECISION_OUTCOME_REQUIRE_APPROVAL / _ALLOW</span>
        </div>
      </Window>
      <p className="sr-only" aria-live="polite">
        {verdict
          ? sid === 'A'
            ? 'Scenario A: deploy payments-api to production. Risk HIGH. The gate stays closed and the result is APPROVAL REQUIRED, because the scope exceeds the grant and the risk requires a human approver. In the ladder, the requested level is RELEASE, but the HIGH-risk floor caps the effective level at SUGGEST, and no scoped grant covers a production deploy. The model cannot override the policy decision.'
            : held
              ? `Scenario B: run repository type check. Risk LOW. The requested level ${requested} resolves to the effective level ${effective}, which is below IMPLEMENT, so the gate stays closed and nothing runs. A requested level never lifts a floor.`
              : `Scenario B: run repository type check. Risk LOW. The gate lifts, the ticket enters the tool execution bay and the result PERMITTED is recorded to the log. In the ladder, the requested level ${requested} resolves to the effective level ${effective}, which covers the check, and a matching scoped grant exists.`
          : 'Policy gate demonstration in progress.'}
      </p>
    </div>
  );
}

function Post({ lamp }: { lamp: string }) {
  return (
    <div className="relative grid shrink-0 place-items-center border border-lp-ink bg-lp-paper max-md:w-9 md:h-9">
      <span className="gov-lamp" data-s={lamp} />
    </div>
  );
}

const AGENT = ['.#.#.#.', '#######', '#.....#', '#.#.#.#', '#.....#', '#######', '.#.#.#.'];
function AgentGlyph() {
  return (
    <svg aria-hidden viewBox="0 0 7 7" shapeRendering="crispEdges" className="size-8 shrink-0 fill-lp-ink-2">
      {AGENT.flatMap((r, y) =>
        [...r].map((c, x) => (c === '#' ? <rect key={`${x}${y}`} x={x} y={y} width={1} height={1} /> : null)),
      )}
    </svg>
  );
}
