'use client';
import { useEffect, useRef, useSyncExternalStore } from 'react';
import { cn } from '@/lib/cn';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { actionsFor, announceFor, factsFor, isDormantLike, labelFor, RAIL_LABEL, SCENARIO_LABEL, type Action } from './copy';
import { LEVELS, levelIndex, RAIL, railOf, type Scenario, type Store } from './machine';

const SHORT = ['OBS', 'SUG', 'IMP', 'PR', 'MRG', 'STG', 'REL', 'OPS'] as const;
const NEXT_SCENARIO: Record<Scenario, Scenario> = { approval: 'preauthorized', preauthorized: 'policy-deny', 'policy-deny': 'approval' };

const btn = 'inline-flex h-8 items-center justify-center border px-3 font-pixel text-[12px] leading-4 whitespace-nowrap transition-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-orange';
const kinds: Record<Action['kind'], string> = {
  primary: 'border-lp-ink bg-lp-yellow text-[#121210] hover:bg-lp-ink hover:text-lp-paper',
  secondary: 'border-lp-line-strong bg-lp-paper text-lp-ink hover:bg-lp-ink hover:text-lp-paper',
  quiet: 'border-transparent bg-transparent text-lp-ink-2 underline decoration-dotted underline-offset-4 hover:text-lp-ink',
};
const dot: Record<string, string> = {
  idle: 'bg-lp-ink-2',
  working: 'bg-lp-yellow',
  hold: 'bg-lp-orange',
  ok: 'bg-lp-yellow',
  auto: 'bg-lp-orange',
};

/** The compact in-hero control strip: state label, requested vs effective ladder, and the gate controls. */
export function Dock({ store }: { store: Store }) {
  const snap = useSyncExternalStore(store.subscribe, store.get, store.get);
  const reduced = useReducedMotion();
  const label = labelFor(snap);
  const actions = actionsFor(snap, reduced);
  const v = snap.vars;
  const root = useRef<HTMLDivElement>(null);
  const acted = useRef(false);
  const showLadder = v.policyDecision !== 'none';
  const facts = factsFor(snap);
  const cur = railOf(snap.state);
  const curI = cur ? RAIL.indexOf(cur) : -1;

  // after a click replaces the button that was focused, keyboard focus moves to the next control
  useEffect(() => {
    if (!acted.current) return;
    acted.current = false;
    root.current?.querySelector<HTMLElement>('[data-action]')?.focus({ preventScroll: true });
  }, [snap.rev]);

  const run = (a: Action) => {
    acted.current = true;
    store.dispatch(a.event);
  };

  const req = levelIndex(v.requestedAutonomy);
  const eff = levelIndex(v.effectiveAutonomy);
  const cap = levelIndex(v.policyCap);

  return (
    <div
      ref={root}
      data-dock
      data-state={snap.state}
      className="absolute inset-x-3 bottom-3 z-10 border border-lp-line-strong bg-lp-paper/95 p-2.5 font-pixel text-[12px] leading-4 antialiased backdrop-blur-[2px]"
    >
      <p role="status" aria-live="polite" className="sr-only">
        {announceFor(snap)}
      </p>
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
        <p className="flex min-w-0 items-center gap-2 text-lp-ink" aria-hidden>
          <span className={cn('inline-block size-2 shrink-0', dot[label.tone])} />
          <span className="truncate">{label.head}</span>
        </p>
        <p className="text-lp-ink-2" aria-hidden>
          ILLUSTRATIVE / PRE-ALPHA
        </p>
      </div>
      <p className="mt-0.5 text-lp-ink-2" aria-hidden>
        {label.sub}
      </p>

      {facts && (
        <p className="mt-1 flex flex-wrap gap-x-3 text-lp-ink-2" aria-hidden>
          <span>
            CONTEXT <b className="font-normal text-lp-ink">{facts.context}</b>
          </span>
          <span>
            AUTHORITY <b className="font-normal text-lp-ink">{facts.authority}</b>
          </span>
        </p>
      )}

      {showLadder && (
        <div className="mt-2" aria-hidden>
          <ol className="flex gap-0.5">
            {LEVELS.map((l, i) => (
              <li
                key={l}
                title={l}
                className={cn(
                  'min-w-0 flex-1 truncate border px-0.5 py-px text-center text-[11px] leading-4',
                  i <= eff ? 'border-lp-ink bg-lp-ink text-lp-paper' : i <= cap ? 'border-dashed border-lp-line-strong text-lp-ink-2' : 'border-lp-line text-lp-ink-2/70',
                  i === req && 'outline-1 outline-offset-1 outline-lp-orange',
                )}
              >
                <span className="max-sm:hidden">{l}</span>
                <span className="sm:hidden">{SHORT[i]}</span>
              </li>
            ))}
          </ol>
          <p className="mt-1 flex flex-wrap gap-x-3 text-lp-ink-2">
            <span>
              REQUESTED <b className="font-normal text-lp-ink">{v.requestedAutonomy}</b>
            </span>
            <span>
              EFFECTIVE <b className="font-normal text-lp-ink">{v.policyDecision === 'pending' ? '...' : v.effectiveAutonomy}</b>
            </span>
            <span className="text-lp-ink">Policy decides outside the model.</span>
          </p>
        </div>
      )}

      <nav aria-label="Browse the simulated run by stage" className="mt-2">
        <ol className="grid grid-cols-6 gap-0.5">
          {RAIL.map((r, i) => {
            const active = cur === r;
            // the human gate is not part of a policy-permitted run: shown as not required, still browsable
            const skipped = r === 'approval' && v.scenario === 'preauthorized' && v.policyDecision === 'permitted';
            return (
              <li key={r} className="min-w-0">
                <button
                  type="button"
                  data-rail={r}
                  aria-current={active ? 'step' : undefined}
                  title={RAIL_LABEL[r].title}
                  onClick={() => store.dispatch({ type: 'JUMP', to: r })}
                  className={cn(
                    'inline-flex h-7 w-full items-center justify-center border px-0 font-pixel text-[11px] leading-4 transition-none focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-lp-orange',
                    active
                      ? 'border-lp-ink bg-lp-ink text-lp-paper'
                      : i < curI && !skipped
                        ? 'border-lp-ink text-lp-ink hover:bg-lp-ink hover:text-lp-paper'
                        : 'border-lp-line-strong text-lp-ink-2 hover:bg-lp-ink hover:text-lp-paper',
                    skipped && !active && 'border-dashed',
                  )}
                >
                  <span className="max-sm:hidden">{RAIL_LABEL[r].full}</span>
                  <span className="sm:hidden" aria-hidden>
                    {RAIL_LABEL[r].short}
                  </span>
                  <span className="sr-only sm:hidden">{RAIL_LABEL[r].full}</span>
                </button>
              </li>
            );
          })}
        </ol>
      </nav>

      <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1.5">
        {actions.map((a) => (
          <button key={a.id} type="button" data-action={a.id} onClick={() => run(a)} className={cn(btn, kinds[a.kind])}>
            {a.label}
          </button>
        ))}
        {isDormantLike(snap.state) && (
          <button
            type="button"
            data-action="scenario"
            onClick={() => store.dispatch({ type: 'SET_SCENARIO', scenario: NEXT_SCENARIO[v.scenario] })}
            className={cn(btn, kinds.quiet, 'px-1')}
            title="Choose which simulated policy the run is evaluated against"
          >
            {SCENARIO_LABEL[v.scenario]}
          </button>
        )}
      </div>
    </div>
  );
}
