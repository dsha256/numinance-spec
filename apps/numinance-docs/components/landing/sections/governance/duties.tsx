import { cn } from '@/lib/cn';
import { Window } from '@/components/landing/ui/window';
import { Lock } from 'lucide-react';
import { DUTIES, type ScenarioId } from './data';

/** Separation of duties as a small mechanical interlock: four stations, one bracket that cannot be closed. */
export function Duties({ scenario }: { scenario: ScenarioId }) {
  const A = scenario === 'A';
  const state: Record<string, string> = {
    author: 'proposed the action',
    reviewer: A ? 'independent of author' : 'not required here',
    release: A ? 'holds the release' : 'not required here',
    approver: A ? 'required · pending' : 'not required here',
  };
  return (
    <Window title="separation of duties · interlock" className="relative">
      <div className="p-3 md:p-5">
        <ol className="grid gap-6 md:grid-cols-4 md:gap-8">
          {DUTIES.map((d, i) => {
            const hot = A && d.id === 'approver';
            return (
              <li
                key={d.id}
                className={cn(
                  'relative border bg-lp-paper p-3',
                  hot ? 'border-lp-ink' : 'border-lp-line-strong',
                )}
              >
                <div className="flex items-center gap-2 font-pixel text-[11px] text-lp-ink-2">
                  <span className="gov-lamp" data-s={hot ? 'hold' : d.id === 'author' ? 'ok' : 'idle'} />
                  <span className="tabular-nums">0{i + 1}</span>
                </div>
                <div className="mt-2 text-[15px] leading-tight font-medium">{d.label}</div>
                <div className="mt-0.5 text-xs leading-snug text-lp-ink-2">{d.note}</div>
                <div className="mt-2 border-t border-dashed border-lp-line-strong pt-1.5 font-pixel text-[11px]">{state[d.id]}</div>
                {i < 3 && (
                  <>
                    <span aria-hidden className="absolute top-1/2 -right-8 hidden h-px w-8 bg-lp-line-strong md:block" />
                    <span aria-hidden className="absolute top-1/2 -right-[19px] hidden size-2.5 -translate-y-1/2 border border-lp-ink bg-lp-paper md:block" />
                    <span aria-hidden className="absolute -bottom-6 left-6 h-6 w-px bg-lp-line-strong md:hidden" />
                  </>
                )}
              </li>
            );
          })}
        </ol>
        {/* the bracket that cannot close */}
        <div aria-hidden className="relative mx-[12.5%] mt-5 hidden h-7 border-x border-b border-dashed border-lp-ink md:block">
          <div className="absolute top-full left-1/2 flex -translate-x-1/2 -translate-y-1/2 items-center border border-lp-ink bg-lp-paper">
            <span className="lp-hazard h-6 w-4" />
            <span className="flex items-center gap-1.5 px-2.5 font-pixel text-[11px] whitespace-nowrap">
              <Lock className="size-3" /> author → approve own work · not permitted
            </span>
            <span className="lp-hazard h-6 w-4" />
          </div>
        </div>
        <p className="mt-6 flex items-center gap-2 border border-lp-ink bg-lp-paper p-2 font-pixel text-[11px] md:hidden">
          <span className="lp-hazard h-6 w-3 shrink-0" />
          <Lock aria-hidden className="size-3 shrink-0" /> the author cannot approve its own work
        </p>
        <p className="mt-6 max-w-[70ch] text-sm leading-relaxed text-lp-ink-2 max-md:mt-3 md:mt-8">
          <span className="text-lp-ink">The author cannot approve its own work.</span> The interlock separates who proposes,
          who reviews, who releases and who approves. Illustrative diagram of the design intent.
        </p>
      </div>
    </Window>
  );
}
