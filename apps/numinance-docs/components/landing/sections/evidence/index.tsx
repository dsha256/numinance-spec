'use client';

import { Fragment, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { RotateCcw } from 'lucide-react';
import { useInView } from 'motion/react';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { cn } from '@/lib/cn';
import { Command } from '@/components/landing/ui/command';
import { Figure, FigureHeader, FigureLede, StatusPlate } from '@/components/landing/ui/figure';
import { Shadow, Window } from '@/components/landing/ui/window';
import { scopedStatus } from '@/lib/landing/status';
const setStatus = scopedStatus('evidence');
import {
  COMMAND, FINAL, RECORDS, TAMPERED, TICK_MS, TYPE_TICKS, lines, recState, statusFor, vaultState, view, type Scenario,
} from './model';
import { Comparator, Coupler, EmbeddedKey, Inspector, Plate, ResultReadout, TerminalBody, Vault } from './panes';

const SCENARIOS: { id: Scenario; label: string; hint: string }[] = [
  { id: 'valid', label: 'Consistent record', hint: 'Record structure and links are consistent.' },
  { id: 'altered', label: 'Altered record', hint: 'A modification breaks the expected integrity relationship.' },
  { id: 'untrusted', label: 'Trust not established', hint: 'Consistency alone cannot establish trusted provenance.' },
];

export function Evidence() {
  const reduced = useReducedMotion() === true;
  const ref = useRef<HTMLDivElement>(null);
  // 'some' + a bottom margin: a fractional `amount` can never be met by this tall figure on a phone
  const inView = useInView(ref, { amount: 'some', margin: '0px 0px -20% 0px' });
  const started = useRef(false);

  const [scenario, setScenario] = useState<Scenario>('valid');
  const [pinned, setPinned] = useState(true);
  const [pick, setPick] = useState<number | null>(null);
  const [run, setRun] = useState(false);
  // One clock. `tf` is the run's position in 55 ms ticks; it is advanced by a single rAF loop that only exists while
  // the figure is on screen and the tab is visible. Typed characters go straight to the DOM; React is told only when
  // the discrete state (step / typing finished / done) changes, never once per tick.
  const typedRef = useRef<HTMLSpanElement>(null);
  const tf = useRef(0);
  const [epoch, setEpoch] = useState(0);
  const [pos, setPos] = useState({ step: 0, typedDone: false, done: false });

  const sync = (t: number) => {
    const { typed, step } = view(t);
    const node = typedRef.current;
    const text = COMMAND.slice(0, typed);
    if (node && node.textContent !== text) node.textContent = text;
    const next = { step, typedDone: typed >= COMMAND.length, done: t >= FINAL };
    setPos((p) => (p.step === next.step && p.typedDone === next.typedDone && p.done === next.done ? p : next));
  };

  useEffect(() => {
    if (inView && !started.current) {
      started.current = true;
      setRun(true);
    }
  }, [inView]);

  useEffect(() => {
    if (reduced) {
      tf.current = FINAL;
      sync(FINAL);
      return;
    }
    if (!run) return;
    sync(Math.floor(tf.current));
    if (!inView || tf.current >= FINAL) return;
    let raf = 0;
    let last = 0;
    const frame = (now: number) => {
      const dt = Math.min(64, now - (last || now));
      last = now;
      tf.current = Math.min(FINAL, tf.current + dt / TICK_MS);
      sync(Math.floor(tf.current));
      raf = tf.current < FINAL && !document.hidden ? requestAnimationFrame(frame) : 0;
    };
    raf = requestAnimationFrame(frame);
    const vis = () => {
      if (!document.hidden && !raf && tf.current < FINAL) {
        last = 0;
        raf = requestAnimationFrame(frame);
      }
    };
    document.addEventListener('visibilitychange', vis);
    return () => {
      cancelAnimationFrame(raf);
      document.removeEventListener('visibilitychange', vis);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [run, reduced, inView, epoch]);

  const { step, typedDone, done } = pos;
  const effPinned = scenario === 'valid' ? pinned : scenario === 'untrusted';

  /** a control press restarts the run from `from` and is visible in the same commit */
  const restart = (from: number) => {
    setPick(null);
    tf.current = from;
    sync(from);
    setRun(true);
    started.current = true;
    setEpoch((e) => e + 1);
  };

  useEffect(() => {
    if (inView && run) setStatus(statusFor(scenario, effPinned, step));
  }, [inView, run, scenario, effPinned, step]);

  const activeIdx = step >= 1 && step <= 5 ? step - 1 : scenario === 'altered' && step > 5 ? TAMPERED : 4;
  const shown = pick ?? activeIdx;
  const vs = vaultState(scenario, effPinned, step);
  const tl = lines(scenario, effPinned, step);

  return (
    <Figure id="evidence" chapter="EVIDENCE" status="evidence" aria-label="Evidence" className="px-(--lp-gutter) py-(--lp-section)">
      <div className="flex flex-col gap-5">
        <FigureHeader lead={<StatusPlate>verifier idle</StatusPlate>} title={["Don't trust the agent.\nCheck the ", { term: 'evidence' }, '.']} />
        <FigureLede>
          An explanation isn&apos;t proof. Numinance&apos;s open evidence model is designed to make engineering records independently inspectable, with cryptographic trust established using keys the verifier chooses—not keys the engine asks you to trust.
        </FigureLede>
        <p className="font-pixel text-[15px] leading-5 text-lp-ink">The result should stand on its own.</p>
      </div>

      <div ref={ref} className="relative isolate mt-10 md:mt-14">
        <Shadow />
        <Window title="evidence-verifier · simulated evidence · concept demonstration" className="relative" aria-label="Evidence verification machine">
          {/* controls */}
          <div className="flex flex-col gap-3 border-b border-lp-line-strong p-3 lg:flex-row lg:items-center lg:justify-between">
            <div role="group" aria-label="Scenario" className="flex flex-wrap gap-1.5">
              {SCENARIOS.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  aria-pressed={scenario === s.id}
                  onClick={() => {
                    setScenario(s.id);
                    restart(TYPE_TICKS);
                  }}
                  className={cn(
                    'flex min-h-11 max-w-[30ch] flex-col items-start justify-center border px-3 py-1 text-left outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-ink',
                    scenario === s.id ? 'border-lp-ink bg-lp-yellow text-lp-on-yellow' : 'border-lp-line-strong hover:bg-lp-paper-2',
                  )}
                >
                  <span className="font-pixel text-[13px] leading-4 uppercase">{s.label}</span>
                  <span className={cn('font-pixel text-[11px] leading-3 text-pretty', scenario === s.id ? 'opacity-70' : 'text-lp-ink-2')}>{s.hint}</span>
                </button>
              ))}
            </div>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
              <div className="flex items-center gap-2">
                <span id="ev-roots" className="font-pixel text-[13px] text-lp-ink-2">Trust roots:</span>
                <div role="group" aria-labelledby="ev-roots" className="flex">
                  {[
                    [false, 'Unpinned'],
                    [true, 'Caller-pinned'],
                  ].map(([v, l]) => {
                    const on = scenario === 'valid' ? pinned === v : false;
                    return (
                      <button
                        key={String(v)}
                        type="button"
                        aria-pressed={on}
                        disabled={scenario !== 'valid'}
                        title={scenario !== 'valid' ? 'The trust-roots toggle applies to the consistent-record scenario' : undefined}
                        onClick={() => {
                          setPinned(v as boolean);
                          restart(TYPE_TICKS);
                        }}
                        className={cn(
                          'min-h-11 border px-3 font-pixel text-[13px] outline-none first:border-e-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-ink',
                          on ? 'border-lp-ink bg-lp-ink text-lp-paper' : 'border-lp-line-strong hover:bg-lp-paper-2',
                          scenario !== 'valid' && 'cursor-not-allowed opacity-40 hover:bg-transparent',
                        )}
                      >
                        [{l}]
                      </button>
                    );
                  })}
                </div>
              </div>
              <button
                type="button"
                onClick={() => restart(0)}
                className="flex min-h-11 items-center gap-2 border border-lp-line-strong px-3 font-pixel text-[13px] outline-none hover:bg-lp-ink hover:text-lp-paper focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-ink"
              >
                <RotateCcw className="size-3.5" aria-hidden /> Replay
              </button>
            </div>
          </div>

          <div className="grid gap-4 p-3 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
            {/* left: terminal + result */}
            <div className="flex min-w-0 flex-col gap-4">
              <div className="relative isolate">
                <Shadow />
                <Window title="terminal · illustrative output" className="relative">
                  <TerminalBody typedRef={typedRef} typedDone={typedDone} lines={tl} done={done} live={run} />
                </Window>
              </div>
              <div className="flex flex-col gap-1.5">
                <Command command={COMMAND} />
                <p className="text-xs leading-snug text-lp-ink-2">
                  The verifier operates on an <code className="font-mono">evidence/v1</code> export directory, not an arbitrary JSON file, and it needs a key mode (without one it exits 2). Output above is simulated.
                </p>
              </div>
              <div className="relative isolate">
                <Shadow />
                <Window title="verification result" className="relative">
                  <ResultReadout scenario={scenario} pinned={effPinned} step={step} />
                </Window>
              </div>
            </div>

            {/* right: bundle, boundary, vault, inspector */}
            <div className="flex min-w-0 flex-col gap-1">
              <div className="flex flex-1 flex-col gap-3 border border-dashed border-lp-line-strong p-3">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 font-pixel text-[13px] leading-4">
                  <span>EVIDENCE-EXPORT/ <span className="text-lp-ink-2">evidence/v1 directory</span></span>
                  <span className="text-lp-ink-2">untrusted input</span>
                </div>
                <div className="flex flex-1 flex-col pt-3 xl:flex-row">
                  {RECORDS.map((r, i) => (
                    <Fragment key={r.id}>
                      <Plate i={i} scenario={scenario} step={step} selected={shown === i} onPick={() => setPick(i)} />
                      {i < RECORDS.length - 1 && <Coupler i={i} scenario={scenario} step={step} />}
                    </Fragment>
                  ))}
                </div>
                <EmbeddedKey scenario={scenario} />
              </div>
              <Comparator state={vs} />
              <div className="grid gap-4 xl:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
                <Vault state={vs} scenario={scenario} pinnedToggle={pinned} />
                <Inspector i={shown} scenario={scenario} step={step} />
              </div>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-1 border-t border-lp-line-strong px-3 py-2 font-pixel text-[13px] text-lp-ink-2">
            <span>simulated evidence · concept demonstration · digests are placeholders</span>
            <span>{step === 0 ? 'ready' : done ? 'run complete' : `step ${Math.min(step, 7)}/7`}</span>
          </div>
        </Window>
      </div>

      <p className="sr-only" aria-live="polite">
        Simulated evidence verification, scenario {SCENARIOS.find((s) => s.id === scenario)?.label}. {statusFor(scenario, effPinned, step)}. Hash links prove structure only and never establish authenticity; trust comes from caller-pinned material. Records:{' '}
        {RECORDS.map((r, i) => `${r.name.join(' ')} ${recState(scenario, i, step)}`).join(', ')}.
      </p>

      <div className="mt-6 flex flex-wrap gap-x-6 gap-y-2 font-pixel text-[13px]">
        <Link href="/evidence" className="underline decoration-lp-line-strong underline-offset-4 hover:text-lp-orange">Evidence overview →</Link>
        <Link href="/docs/evidence" className="underline decoration-lp-line-strong underline-offset-4 hover:text-lp-orange">/docs/evidence →</Link>
      </div>
    </Figure>
  );
}
