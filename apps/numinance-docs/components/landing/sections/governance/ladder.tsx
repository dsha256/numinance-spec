import { Check } from 'lucide-react';
import { cn } from '@/lib/cn';
import { LADDERS, LADDER_AT, LEVELS, type LevelId, type ScenarioId } from './data';

const idx = (id: string) => LEVELS.findIndex((l) => l.id === id);

/**
 * Compact autonomy ladder inside the policy-gate figure. Follows the gate's phase until the viewer picks a level;
 * after that every mark responds on the next frame. Four distinct things: the level REQUESTED, the CAP (floor)
 * the effective policy permits, the EFFECTIVE level (never above the cap), and the scoped GRANT, a separate
 * check that a level never replaces. Illustrative; mirrors the AutonomyPolicy resolution rule.
 */
export function Ladder({
  sid,
  shown,
  requested,
  effective,
  interacted,
  onPick,
}: {
  sid: ScenarioId;
  shown: number;
  requested: LevelId;
  effective: LevelId;
  interacted: boolean;
  onPick: (id: LevelId) => void;
}) {
  const L = LADDERS[sid];
  const cfg = idx(requested);
  const cap = idx(L.cap);
  const eff = idx(effective);
  const needs = idx(L.needs);
  const showCfg = interacted || shown >= LADDER_AT.configured;
  const showCap = interacted || shown >= LADDER_AT.cap;
  const showGrant = interacted || shown >= LADDER_AT.grant;
  const showEff = interacted || shown >= LADDER_AT.effective;
  const isA = sid === 'A';
  const covers = eff >= needs;

  return (
    <div className="border-t border-lp-line-strong">
      <div className="flex items-center justify-between gap-2 border-b border-lp-line px-3 py-1.5 font-pixel text-[11px] text-lp-ink-2">
        <span>AUTONOMY LADDER</span>
        <span>illustrative · AutonomyPolicy</span>
      </div>
      <ol className="px-3 pt-2" aria-label="Autonomy levels, lowest to highest. Select one to request it.">
        {LEVELS.map((lv, i) => {
          const above = showCap && i > cap;
          const isCfg = showCfg && i === cfg;
          const isCap = showCap && i === cap;
          const isEff = showEff && i === eff;
          return (
            <li
              key={lv.id}
              data-above-cap={above || undefined}
              className={cn(
                'border-b transition-colors duration-150',
                isCap ? 'border-b-2 border-lp-orange' : 'border-lp-line',
                isEff && 'bg-lp-yellow text-lp-on-yellow',
              )}
            >
             <button
              type="button"
              aria-pressed={i === cfg}
              aria-label={`Request ${lv.id}${above ? ' (above the floor, resolves to ' + effective + ')' : ''}`}
              onClick={() => onPick(lv.id)}
              className="flex min-h-7 w-full items-center gap-2 py-0.5 text-start outline-none hover:bg-lp-paper-2/60 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-lp-ink"
             >
              <span className="w-4 shrink-0 text-center font-pixel text-[11px] tabular-nums opacity-60">{i + 1}</span>
              <span className={cn('w-[72px] shrink-0 font-pixel text-[12px] leading-4', above && 'text-lp-ink-2')}>
                {lv.id}
              </span>
              <span
                aria-hidden
                className={cn('h-2 min-w-3 flex-1 border border-dashed border-lp-line-strong', above ? 'lp-hazard opacity-20' : 'opacity-60')}
              />
              <span className="flex shrink-0 flex-wrap justify-end gap-1">
                {isCfg && (
                  <span className="border border-lp-indigo bg-lp-paper px-1 font-pixel text-[11px] leading-4 text-lp-indigo">
                    REQUESTED
                  </span>
                )}
                {isCap && (
                  <span className="border border-lp-orange bg-lp-paper px-1 font-pixel text-[11px] leading-4 text-lp-orange">
                    CAP
                  </span>
                )}
              </span>
             </button>
            </li>
          );
        })}
      </ol>
      <div className="space-y-1.5 px-3 pt-2 pb-3 font-mono text-[11.5px] leading-snug">
        <div className="min-h-9 text-lp-ink-2">
          {showCfg ? (
            <>
              <span className="text-lp-indigo">requested {requested}</span>
              {requested === L.configured && !interacted ? <> ({L.configuredNote})</> : null} ·{' '}
              {showCap ? (
                <>
                  <span className="text-lp-orange">cap {L.cap}</span> ({L.capNote})
                </>
              ) : (
                'cap …'
              )}
              {showEff && (
                <>
                  {' '}
                  → <span className="text-lp-ink">effective {effective}</span>
                  {'. '}
                  {isA
                    ? `${L.needs} (production deploy) is above it: approval required.`
                    : covers
                      ? `${L.needs} (check) is within it.`
                      : `${L.needs} (check) is above it: held.`}
                </>
              )}
            </>
          ) : (
            'requested level · cap · effective level'
          )}
        </div>
        <div
          className={cn(
            'flex items-center gap-2 border px-2 py-1',
            showGrant ? (L.grantOk ? 'border-lp-indigo' : 'border-lp-ink') : 'border-dashed border-lp-line-strong text-lp-ink-2',
          )}
        >
          {showGrant ? (
            L.grantOk ? (
              <Check aria-hidden className="size-3.5 shrink-0 text-lp-indigo" />
            ) : (
              <span aria-hidden className="lp-hazard size-3 shrink-0 border border-lp-ink" />
            )
          ) : (
            <span aria-hidden className="size-3 shrink-0 border border-dashed border-lp-line-strong" />
          )}
          <span className="font-pixel text-[11px]">SCOPED GRANT</span>
          <span className="min-w-0 flex-1">{showGrant ? L.grant : '· · ·'}</span>
        </div>
        <div className="text-[11px] text-lp-ink-2">
          Select a level to request it. Configuration never lifts a floor or the host sandbox ceiling, and a level is never a grant.
        </div>
      </div>
    </div>
  );
}
