'use client';

import type { ComponentProps, ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { registerFigure } from '@/lib/landing/status';
import { Shadow } from './window';

/** A "sheet" of the drawing set: one chapter with its seam, registration marks and status line. */
export function Figure({
  status,
  chapter,
  className,
  children,
  ...props
}: ComponentProps<'section'> & {
  id: string;
  /** Short chapter label shown in the status bar index. */
  chapter: string;
  /** Resting status-bar line while this figure is active. */
  status?: string;
}) {
  return (
    <section
      {...props}
      ref={registerFigure}
      data-figure=""
      data-chapter={chapter}
      data-status={status}
      className={cn('relative scroll-mt-(--lp-menu-h) border-t border-lp-line', className)}
    >
      <Mark className="left-0 -translate-x-1/2" />
      <Mark className="right-0 translate-x-1/2" />
      {children}
    </section>
  );
}

function Mark({ className }: { className: string }) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 11 11"
      className={cn('absolute top-0 z-10 size-2.75 -translate-y-1/2 text-lp-line-strong max-sheet:hidden', className)}
    >
      <path d="M5.5 0v11M0 5.5h11" stroke="currentColor" />
    </svg>
  );
}

/** Pixel-type status plate set over a heading: a lit lamp in a square cell + one short line. */
export function StatusPlate({ children }: { children: ReactNode }) {
  return (
    <span className="relative isolate mb-[0.5em] flex h-lh w-fit font-pixel text-mark font-normal tracking-normal text-lp-ink">
      <Shadow />
      <span aria-hidden className="relative flex aspect-square h-full items-center justify-center border border-lp-ink bg-lp-paper">
        <span className="size-[0.55em] rounded-full border border-lp-ink bg-lp-sun" />
      </span>
      <span className="relative flex items-center border border-s-0 border-lp-ink bg-lp-paper px-[0.75em]">
        <span className="[text-box:trim-both_cap_alphabetic]">{children}</span>
      </span>
    </span>
  );
}

export type Run = string | { term: string };

/** Heading runs: plain text in the light voice, `{term}` runs one step up in medium + orange. `\n` breaks lines. */
export function Terms({ runs }: { runs: readonly Run[] }) {
  const out: ReactNode[] = [];
  for (const run of runs) {
    if (typeof run !== 'string') {
      out.push(
        <span key={out.length} className="font-medium whitespace-nowrap text-lp-orange">
          {run.term}
        </span>,
      );
      continue;
    }
    const lines = run.split('\n');
    lines.forEach((line, i) => {
      if (i > 0) out.push(<br key={out.length} />);
      out.push(line);
    });
  }
  return out;
}

/** A figure's title: one sentence, light weight, key terms in orange. Optionally led by a status plate. */
export function FigureHeader({
  lead,
  title,
  className,
  ...props
}: Omit<ComponentProps<'h2'>, 'title' | 'children'> & { lead?: ReactNode; title: readonly Run[] }) {
  return (
    <h2
      {...props}
      className={cn(
        'max-w-[24ch] text-[clamp(2rem,1rem+3.6vw,4.25rem)] leading-[1.02] font-light tracking-[-0.035em] text-balance',
        className,
      )}
    >
      {lead} <Terms runs={title} />
    </h2>
  );
}

/** Body copy under a figure title. */
export function FigureLede({ children, className }: { children: ReactNode; className?: string }) {
  return <p className={cn('max-w-[52ch] text-base leading-relaxed text-lp-ink-2 md:text-lg', className)}>{children}</p>;
}
