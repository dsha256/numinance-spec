'use client';

import Link from 'next/link';
import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Check, Clipboard, RotateCcw } from 'lucide-react';
import { cn } from '@/lib/cn';
import { scopedStatus } from '@/lib/landing/status';
const setStatus = scopedStatus('developers');
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { useInView } from '@/components/landing/machine/use-in-view';
import { Chip } from '@/components/landing/ui/chip';
import { WORKFLOWS } from './data';
import { Editor } from './editor';
import { Factory } from './factory';
import { CliClock, frameKey } from './clock';
import { derive, finalFrame, statusLine, type Frame } from './timeline';

/** server-rendered text of the settled first run; the clock owns this node afterwards */
const FIRST_COMMAND = WORKFLOWS[0].command;

export function Cli() {
  const reduced = useReducedMotion();
  const live = !reduced;
  const [clock] = useState(() => new CliClock());
  // React holds only the discrete frame; typed text, positions and spin are written to the DOM by the clock
  const [frame, setFrame] = useState<Frame>(() => finalFrame(0));
  const [view, setView] = useState(0);
  const root = useRef<HTMLDivElement>(null);
  const typedEl = useRef<HTMLSpanElement>(null);
  const started = useRef(false);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const d = derive(frame);
  const s = { wf: frame.wf, phase: frame.phase };

  useEffect(() => {
    const onFrame = (f: Frame) => {
      setFrame((prev) => (frameKey(prev) === frameKey(f) ? prev : f));
      const node = typedEl.current?.firstChild;
      if (node instanceof Text) {
        const text = f.p.w.command.slice(0, f.typed);
        if (node.data !== text) node.data = text;
      }
    };
    const off = clock.subscribe(onFrame);
    return () => {
      off();
      clock.dispose();
    };
  }, [clock]);

  /** a click is visible at the very next paint: the first frame renders synchronously and replaces any run in progress */
  const run = useCallback(
    (wf: number) => {
      started.current = true;
      setView(0);
      if (live) clock.start(wf);
      else clock.settle(wf);
    },
    [clock, live],
  );

  useInView(
    root,
    () => {
      clock.setVisible(true);
      if (!started.current && live) run(0);
    },
    () => clock.setVisible(false),
    0.15,
  );

  // reduced motion switched on mid-run: settle on the final frame
  useEffect(() => {
    if (!live) clock.settle(clock.frame.wf);
  }, [live, clock]);

  // the editor follows the run: the edited file when the flag appears
  useEffect(() => {
    if (d.edit === 'flag') setView(d.w.edit.file);
  }, [d.edit, d.w]);

  useEffect(() => {
    if (!started.current) return;
    setStatus(statusLine(frame, d));
  }, [frame.phase, d.landed, frame.wf]); // eslint-disable-line react-hooks/exhaustive-deps

  const onTabKey = (e: KeyboardEvent, i: number) => {
    const n = WORKFLOWS.length;
    const next = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? (i + 1) % n : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? (i + n - 1) % n : e.key === 'Home' ? 0 : e.key === 'End' ? n - 1 : -1;
    if (next < 0) return;
    e.preventDefault();
    tabs.current[next]?.focus();
    run(next);
  };

  const w = d.w;

  return (
    <div ref={root}>
      <p className="sr-only">{w.summary}</p>
      <div className="grid gap-x-8 gap-y-4 min-[900px]:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] min-[900px]:grid-rows-[auto_1fr]">
        {/* command controls */}
        <div className="flex min-w-0 flex-col gap-3 min-[900px]:col-start-2 min-[900px]:row-start-1">
          <div role="tablist" aria-label="Workflow" className="grid grid-cols-2 gap-2">
            {WORKFLOWS.map((k, i) => (
              <button
                key={k.id}
                ref={(el) => {
                  tabs.current[i] = el;
                }}
                type="button"
                role="tab"
                aria-selected={s.wf === i}
                tabIndex={s.wf === i ? 0 : -1}
                onClick={() => run(i)}
                onKeyDown={(e) => onTabKey(e, i)}
                className={cn(
                  'flex min-h-11 min-w-0 flex-col items-start justify-center border border-lp-line-strong px-3 py-1.5 text-start outline-none focus-visible:ring-2 focus-visible:ring-lp-ink',
                  s.wf === i ? 'bg-lp-yellow text-lp-on-yellow' : 'bg-lp-paper hover:bg-lp-paper-2',
                )}
              >
                <span className="text-sm leading-5 font-medium">{k.label}</span>
                <span className="font-pixel text-[11px] leading-4 opacity-70">{k.badge}</span>
              </button>
            ))}
          </div>

          <div className="flex items-stretch border border-lp-line-strong bg-lp-paper">
            <div className="flex min-w-0 flex-1 items-baseline gap-2 px-3 py-2.5 font-mono text-xs leading-5 sm:text-[13px]">
              <span aria-hidden className="text-lp-orange select-none">$</span>
              {/* the full command sets the box's size; typing reveals it in place */}
              <code className="relative grid min-w-0 break-all">
                <span className="invisible col-start-1 row-start-1">{w.command}</span>
                <span aria-hidden className="col-start-1 row-start-1">
                  <span ref={typedEl}>{FIRST_COMMAND}</span>
                  {(s.phase === 'typing' || !d.running) && (
                    <span className={cn('ms-px inline-block h-[1.1em] w-[0.55em] translate-y-[0.2em] bg-lp-ink', s.phase !== 'typing' && 'animate-lp-caret')} />
                  )}
                </span>
              </code>
            </div>
            {w.confirmed && <CopyButton command={w.command} />}
            <button
              type="button"
              onClick={() => run(s.wf)}
              aria-label="Run again"
              className="flex w-11 shrink-0 items-center justify-center border-s border-lp-line-strong outline-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper motion-reduce:hidden"
            >
              <RotateCcw aria-hidden className="size-4" />
            </button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Chip className="h-auto min-h-6 whitespace-normal py-0.5">{w.chip}</Chip>
          </div>
        </div>

        {/* the factory + project tree */}
        <div className="flex min-w-0 flex-col min-[900px]:col-start-1 min-[900px]:row-span-2 min-[900px]:row-start-1">
          <Factory d={d} reduced={!live} clock={clock} />
          <Disclosure className="mt-6 hidden min-[900px]:block" />
        </div>

        {/* editor */}
        <div className="min-w-0 min-[900px]:col-start-2 min-[900px]:row-start-2">
          <Editor d={d} rw={frame.rw} view={view} onView={setView} caret />
        </div>
      </div>

      <Disclosure className="mt-8 min-[900px]:hidden" />
    </div>
  );
}

function CopyButton({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      aria-label={copied ? 'Copied' : `Copy "${command}"`}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(command);
          setCopied(true);
          setStatus(`copied: ${command}`);
          setTimeout(() => setCopied(false), 1600);
        } catch {}
      }}
      className="flex w-11 shrink-0 items-center justify-center border-s border-lp-line-strong outline-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper"
    >
      {copied ? <Check aria-hidden className="size-4" /> : <Clipboard aria-hidden className="size-4" />}
    </button>
  );
}

function Disclosure({ className }: { className?: string }) {
  return (
    <p className={cn('max-w-[70ch] border-s-2 border-lp-sun ps-3 text-sm leading-relaxed text-lp-ink-2', className)}>
      <span className="font-pixel text-[13px] text-lp-ink">DISCLOSURE </span>
      File names, file contents and results here are illustrative and use placeholders; nothing was captured from a run. Open-CLI schema
      validation is not private-engine profile resolution, and neither is readiness. Hash links prove structural consistency only; a result is
      verified only against caller-pinned keys (there is no built-in trust root).{' '}
      <Link href="/docs/cli" className="text-lp-ink underline underline-offset-4 hover:text-lp-orange">
        /docs/cli
      </Link>{' '}
      <Link href="/docs/evidence" className="text-lp-ink underline underline-offset-4 hover:text-lp-orange">
        /docs/evidence
      </Link>
    </p>
  );
}
