import Link from 'next/link';
import { Figure, FigureHeader, FigureLede, StatusPlate } from '@/components/landing/ui/figure';
import { Scene } from './scene';

const LIFECYCLE = [
  { k: 'UNDERSTAND', h: 'Know the system first.', t: 'Resolve relevant repository structure, dependencies, documentation, and constraints.' },
  { k: 'PLAN', h: 'Turn intent into a plan.', t: 'Define scoped work, risk, execution steps, and checks.' },
  { k: 'ACT', h: 'Work within the authority granted.', t: 'Dispatch permitted work through scoped capabilities and adapters.' },
  { k: 'VERIFY', h: 'Check the work, not the explanation.', t: 'Run deterministic checks and independent reviews, with bounded repair where permitted.' },
  { k: 'EVIDENCE', h: 'Leave a record others can inspect.', t: 'Record what was attempted, allowed, denied, and observed.' },
] as const;

/**
 * WORKFLOW: an inspection rig rides a rail between four regions (define, govern, execute, verify), a pixel operator
 * aboard its carriage, and scans each in turn. A conceptual visualization built from real contract shapes in the
 * public numinance-spec; every result shown is illustrative or SIMULATED.
 */
export function Workflow() {
  return (
    <Figure id="workflow" chapter="WORKFLOW" status="workflow" aria-label="Workflow: a task goes in, accountable work comes out">
      <div className="grid gap-6 px-(--lp-gutter) pt-(--lp-section) pb-8 min-[900px]:grid-cols-12 min-[900px]:items-end">
        <FigureHeader
          className="min-[900px]:col-span-7"
          lead={<StatusPlate>intent received</StatusPlate>}
          title={['A task goes in.\n', { term: 'Accountable work' }, '\ncomes out.']}
        />
        <FigureLede className="min-[900px]:col-span-5">
          From declared intent to governed tool execution, typed contracts, and evidence, Numinance is designed to carry
          engineering work through a controlled process—not bounce responsibility back to the next prompt.
          <span className="mt-3 block font-pixel text-[13px] text-lp-ink">Less prompting. More engineering.</span>
        </FigureLede>
      </div>
      <Scene />
      <ol aria-label="Engineering lifecycle" className="mx-(--lp-gutter) grid border-t border-lp-line-strong md:grid-cols-5">
        {LIFECYCLE.map((l, i) => (
          <li key={l.k} className="flex gap-3 border-b border-lp-line py-4 md:border-b-0 md:border-s md:px-4 md:first:border-s-0 md:first:ps-0">
            <span aria-hidden className="font-pixel text-3xl leading-none text-lp-orange">{String(i + 1).padStart(2, '0')}</span>
            <div className="min-w-0">
              <h3 className="font-pixel text-[13px] leading-4 text-lp-ink">{l.k}</h3>
              <p className="mt-1 font-pixel text-[12px] leading-4 text-lp-ink">{l.h}</p>
              <p className="mt-1.5 text-[13px] leading-relaxed text-lp-ink-2">{l.t}</p>
            </div>
          </li>
        ))}
      </ol>
      <p className="mx-(--lp-gutter) mt-3 max-w-[78ch] text-[13px] leading-relaxed text-lp-ink-2">
        Where checks fail, repair is bounded and recovery is explicit; rollback is part of the lifecycle, not an afterthought. Everything shown is illustrative.
      </p>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 px-(--lp-gutter) pt-3 pb-(--lp-section) font-pixel text-[13px] text-lp-ink-2">
        <span>conceptual visualization · pre-alpha</span>
        <Link href="/docs/protocol" className="underline underline-offset-4 hover:text-lp-ink">
          Protocol docs
        </Link>
        <Link href="/docs/evidence" className="underline underline-offset-4 hover:text-lp-ink">
          Evidence docs
        </Link>
      </div>
      <p className="sr-only">
        Six stages shown across four regions: intent, authority, execution and evidence. An inspection rig rides a rail between
        the regions and scans each in turn, then rests; use the stage buttons to bring any stage up. Intake: intent is received. Inspection:
        constraints from an AutonomyPolicy are evaluated and a decision (allow, deny or require approval) is made outside the model.
        Execution: the host sets the ActionContext and dispatches through PluginService.Invoke. Evidence: a hash-linked record is
        emitted through HostService.EmitEvidence. Verification: the command numi evidence verify with a runner key, log key and log
        origin checks the export; the verified result shown is a simulation. Reset clears the stages.
      </p>
    </Figure>
  );
}
