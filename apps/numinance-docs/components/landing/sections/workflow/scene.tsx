'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { cn } from '@/lib/cn';
import { PixelButton, REPLAY_ICON } from '@/components/landing/machine/pixel-button';
import { Window } from '@/components/landing/ui/window';
import { scopedStatus } from '@/lib/landing/status';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { FINAL_STAGE, createPress, type Press } from './rig/engine';
import { RailBed, RailColumn } from './rig/rail';
import { BED } from './rig/scene';
import { STAGES, paneState } from './model';
import { Define, Execute, Govern, Region, Verify } from './panes';

const setStatus = scopedStatus('workflow');

/**
 * The workflow stage: four regions (intent, authority, execution, evidence) in two columns round a rail band; on a
 * phone, one column beside a rail. An inspection rig rides the rail: a carriage with the pixel operator aboard, a
 * boom, an elbow and a long probe. The rig is a WebGL mesh and an engine (rig/engine.ts) that owns everything that
 * moves on one clock; React sees only discrete changes:
 *
 *   - the stage on show (the engine tells us when a scan brings one up), which sets the regions' states;
 *   - a chip press (the engine plans the arm to that region, from wherever it is) or Replay.
 *
 * Lifecycle: the engine plans the introduction as the stage nears, plays it once when the stage crosses the middle of
 * the viewport, then hands the arm to the pointer (springs started from the plan's pose and velocity). Everything
 * shown is illustrative.
 */
export function Scene() {
  const reduced = useReducedMotion();
  const stageEl = useRef<HTMLDivElement>(null);
  const press = useRef<Press | null>(null);
  const chips = useRef<(HTMLButtonElement | null)[]>([]);
  const [stage, setStage] = useState(0);
  const [sel, setSel] = useState(0);

  useEffect(() => {
    const el = stageEl.current;
    if (!el) return;
    const p = createPress(
      el,
      (s) => {
        setStage(s);
        setSel(s);
        setStatus(STAGES[s].status);
      },
      reduced,
    );
    press.current = p;
    return () => {
      p.destroy();
      press.current = null;
    };
  }, [reduced]);

  const choose = (i: number) => {
    setSel(i);
    press.current?.press(i);
  };

  const onKey = (e: KeyboardEvent) => {
    const n = STAGES.length;
    let to = -1;
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') to = (sel + 1) % n;
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') to = (sel + n - 1) % n;
    else if (e.key === 'Home') to = 0;
    else if (e.key === 'End') to = n - 1;
    if (to < 0) return;
    e.preventDefault();
    setSel(to);
    chips.current[to]?.focus();
  };

  const current = STAGES[stage] ?? STAGES[FINAL_STAGE];

  return (
    <div className="relative px-(--lp-gutter) py-2">
      <div ref={stageEl} data-stage className="relative flex overflow-clip">
        <Window
          data-doc
          title="engineering-run.rig"
          className="min-w-0 flex-1"
          actions={<span className="relative bg-lp-paper px-2 font-pixel text-[13px] leading-4 text-lp-ink-2">pre-alpha</span>}
        >
          {/* toolbar */}
          <div className="z-20 flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-lp-line bg-lp-paper px-3 py-1.5 max-lg:sticky max-lg:top-(--lp-menu-h)">
            <span
              className="inline-flex h-7 min-w-0 items-center gap-2 border border-lp-ink bg-lp-paper px-2 font-pixel text-[13px] leading-4 lg:w-[17.5rem] lg:shrink-0"
              role="status"
              aria-live="off"
            >
              <span aria-hidden className="size-2.5 rounded-full border border-lp-ink bg-lp-sun" />
              {current.status}
            </span>
            <div
              role="group"
              aria-label="Workflow stages"
              onKeyDown={onKey}
              className="flex flex-wrap items-center gap-1.5 max-lg:order-3 max-lg:w-full max-lg:flex-nowrap max-lg:overflow-x-auto max-lg:pb-0.5 max-lg:[scrollbar-width:none]"
            >
              {STAGES.map((s, i) => (
                <button
                  key={s.id}
                  ref={(el) => {
                    chips.current[i] = el;
                  }}
                  type="button"
                  tabIndex={i === sel ? 0 : -1}
                  aria-pressed={i === sel}
                  aria-current={i === stage ? 'step' : undefined}
                  aria-label={`${i + 1} ${s.chip}: ${s.status}`}
                  onClick={() => choose(i)}
                  className={cn(
                    'inline-flex min-h-11 shrink-0 items-center gap-1.5 border px-2.5 font-pixel text-[13px] leading-4 whitespace-nowrap antialiased transition-colors md:min-h-7',
                    i === sel
                      ? cn('border-lp-ink bg-lp-yellow text-lp-on-yellow', i !== stage && 'border-dashed')
                      : 'border-lp-line-strong bg-lp-paper text-lp-ink hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper',
                  )}
                >
                  <span aria-hidden className={cn('size-1.5 rounded-full', i < stage ? 'bg-lp-orange' : 'border border-current')} />
                  {i + 1} {s.chip}
                </button>
              ))}
            </div>
            {!reduced && (
              <span className="ml-auto flex items-center gap-2 max-lg:order-2">
                <PixelButton onClick={() => press.current?.replay()} icon={REPLAY_ICON}>
                  <span className="max-lg:sr-only">Replay</span>
                </PixelButton>
              </span>
            )}
          </div>

          {/* the regions: two columns round the rail band; one column on a phone */}
          <div
            data-grid
            className={cn(
              'relative grid min-h-0 flex-1 grid-cols-1 bg-lp-paper-2 lg:grid-cols-2',
              'lg:grid-rows-[auto_96px_auto]',
            )}
          >
            <Region i={0} state={paneState(0, stage)} right={<span className="max-lg:hidden">schemas/examples/AutonomyPolicy/org-ladder.yaml</span>} className="max-lg:border-b border-lp-line-strong lg:border-r">
              <Define state={paneState(0, stage)} />
            </Region>
            <Region i={1} state={paneState(1, stage)} right="decided by the host" className="max-lg:border-b border-lp-line-strong">
              <Govern state={paneState(1, stage)} />
            </Region>
            {/* the rail band the carriage rides; its construction is drawn here, the carriage by the mesh */}
            <div data-bay aria-hidden style={{ height: BED.height }} className="relative hidden border-y border-lp-ink bg-lp-paper-2 lg:col-span-2 lg:block">
              <RailBed stops={4} />
            </div>
            <Region i={2} state={paneState(2, stage)} right="numinance.plugin.v1" className="max-lg:border-b border-lp-line-strong lg:border-r">
              <Execute state={paneState(2, stage)} stage={stage} />
            </Region>
            <Region i={3} state={paneState(3, stage)} right="evidence/v1">
              <Verify stage={stage} />
            </Region>
          </div>
        </Window>
        {/* a phone's rail runs down this column */}
        <div data-bay-phone className="relative w-18 shrink-0 lg:hidden">
          <RailColumn id="workflow" />
        </div>
        {/* paper dithered over the regions before they are brought up again; the line the probe drags */}
        <div data-cover aria-hidden className="pointer-events-none invisible absolute z-1 bg-lp-paper" />
        <span data-scan aria-hidden className="pointer-events-none absolute top-0 left-0 z-1 h-0.5 bg-lp-orange" />
      </div>
    </div>
  );
}
