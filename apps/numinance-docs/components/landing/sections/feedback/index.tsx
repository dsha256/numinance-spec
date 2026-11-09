'use client';

import Link from 'next/link';
import { useLayoutEffect, useRef, useState } from 'react';
import { Chip } from '@/components/landing/ui/chip';
import { Figure, FigureHeader, FigureLede, StatusPlate } from '@/components/landing/ui/figure';
import { Shadow, Window } from '@/components/landing/ui/window';
import { cn } from '@/lib/cn';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { type Author, ENTRIES, type Entry, type Socket, publishedQuote } from './data';
import { createFeedback } from './engine';

/** Where each window sits (desktop: two loose columns; narrow: a column, each inset unevenly),
 *  and the side its cable enters from. */
const PLACE: Record<Entry['id'], { cls: string; entry: 'top' | 'left' | 'right' }> = {
  rfc: { cls: 'w-[92%] min-[900px]:col-start-1 min-[900px]:row-start-1 min-[900px]:w-[86%]', entry: 'top' },
  david: {
    cls: 'ms-[6%] w-[94%] min-[900px]:col-start-2 min-[900px]:row-start-1 min-[900px]:mt-14 min-[900px]:ms-0 min-[900px]:w-full',
    entry: 'top',
  },
  status: {
    cls: 'ms-[3%] w-[92%] min-[900px]:col-start-1 min-[900px]:row-start-2 min-[900px]:mt-12 min-[900px]:ms-[14%] min-[900px]:w-[86%]',
    entry: 'right',
  },
  conformance: {
    cls: 'ms-[9%] w-[91%] min-[900px]:col-start-2 min-[900px]:row-start-2 min-[900px]:mt-16 min-[900px]:ms-[18%] min-[900px]:w-[82%]',
    entry: 'left',
  },
};

const SOCKETS: { id: Socket; left: string; note: string }[] = [
  { id: 'numinance-spec', left: '12%', note: 'source' },
  { id: 'evidence/v1', left: '34%', note: 'format' },
  { id: 'conformance', left: '60%', note: 'checks' },
  { id: 'feedback bus', left: '86%', note: 'open line' },
];

function Initials({ author }: { author: Author }) {
  const initials = author.name
    .split(' ')
    .map((p) => p[0])
    .join('');
  return (
    <span
      aria-hidden
      className="relative isolate flex size-11 shrink-0 items-center justify-center border border-lp-ink bg-lp-yellow font-pixel text-base text-lp-on-yellow"
    >
      {initials}
    </span>
  );
}

/** pixel-art quotation mark, orange: a closing pair, turned half round to open */
function QuoteMark({ close }: { close?: boolean }) {
  const mark = 'M1 0h4v3H1zM0 3h4v1H0zM0 4h3v1H0z';
  return (
    <svg
      aria-hidden
      viewBox="0 0 11 5"
      shapeRendering="crispEdges"
      className={cn('inline-block h-4 w-[35px] fill-lp-orange', close ? 'ms-2 align-top' : 'rotate-180')}
    >
      <path d={mark} />
      <path d={mark} transform="translate(6 0)" />
    </svg>
  );
}

function Quote({ author }: { author: Author }) {
  const text = publishedQuote(author);
  return (
    <blockquote className="relative flex-1 px-6 pt-5 pb-6">
      <QuoteMark />
      {text ? (
        <p className="mt-1 text-lg/6.5 font-light text-pretty md:text-xl/7">
          {text}
          <QuoteMark close />
        </p>
      ) : (
        <div
          role="note"
          className="mt-2 border border-dashed border-lp-line-strong px-4 py-5 text-center"
        >
          <p className="font-pixel text-[13px] leading-4 text-lp-ink">Quote pending approval</p>
          <p className="mt-1.5 text-sm/5 text-lp-ink-2">Not yet published. Nothing is attributed until it is approved.</p>
        </div>
      )}
    </blockquote>
  );
}

function Person({ author }: { author: Author }) {
  return (
    <figure className="flex flex-1 flex-col">
      <Quote author={author} />
      <figcaption className="flex items-center gap-3 border-t border-lp-line px-6 py-4">
        <Initials author={author} />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-sm/5 font-medium">{author.name}</span>
          <span className="text-sm/5 text-lp-ink-2">{author.role}</span>
        </span>
      </figcaption>
      <p className="border-t border-lp-line px-6 py-2.5 font-pixel text-[12px] leading-4 text-lp-ink-2">
        Individual attribution. No company endorsement.
      </p>
    </figure>
  );
}

function Note({ entry }: { entry: Extract<Entry, { kind: 'note' }> }) {
  return (
    <div className="flex flex-1 flex-col gap-3 px-6 pt-5 pb-5">
      <Chip className="w-fit">{entry.tag}</Chip>
      <div className="flex flex-col gap-2 text-base/6 font-light text-pretty md:text-lg/6.5">
        {entry.body.map((p) => (
          <p key={p}>{p}</p>
        ))}
      </div>
      <Link
        href={entry.link.href}
        className="mt-auto w-fit pt-1 text-sm/5 font-medium underline decoration-lp-orange decoration-2 underline-offset-4 hover:text-lp-orange focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-lp-orange"
      >
        {entry.link.label} <span aria-hidden>→</span>
      </Link>
    </div>
  );
}

/** the stub that stands in for a wire on narrow screens: a labelled socket and a short cable */
function Stub({ from }: { from: Socket }) {
  return (
    <div aria-hidden className="absolute bottom-full start-6 flex flex-col items-start min-[900px]:hidden">
      <span className="flex h-6 items-center gap-1.5 border border-lp-line-strong bg-lp-paper px-2 font-pixel text-[12px] whitespace-nowrap">
        <span className="size-2 bg-lp-ink" />
        {from}
      </span>
      <span className="ms-4 block h-5 w-1 border-x border-lp-ink bg-lp-paper" />
    </div>
  );
}

function Port() {
  return (
    <span
      data-port
      aria-hidden
      className="pointer-events-none absolute top-0 left-0 z-10 size-3 border border-lp-ink bg-lp-paper after:absolute after:inset-0.75 after:bg-lp-ink max-[899px]:hidden"
    />
  );
}

function Rack() {
  return (
    <div aria-hidden className="relative mb-24 hidden h-[84px] min-[900px]:block">
      <span className="absolute inset-x-0 top-[22px] border-t border-dashed border-lp-line-strong" />
      {SOCKETS.map((s) => (
        <div
          key={s.id}
          style={{ left: s.left }}
          className="absolute top-0 flex -translate-x-1/2 flex-col items-center"
        >
          <div className="relative isolate">
            <Shadow />
            <div className="relative flex h-11 w-[132px] flex-col items-center justify-center border border-lp-line-strong bg-lp-paper font-pixel">
              <span className="text-[13px] leading-4 whitespace-nowrap text-lp-ink">{s.id}</span>
              <span className="text-[11px] leading-3 text-lp-ink-2">{s.note}</span>
            </div>
          </div>
          <span
            data-jack={s.id}
            className="mt-2 size-3.5 border border-lp-ink bg-lp-paper transition-colors after:m-[2px] after:block after:size-2 after:bg-lp-ink data-[live]:bg-lp-sun"
          />
        </div>
      ))}
    </div>
  );
}

export function Feedback() {
  const desk = useRef<HTMLDivElement>(null);
  const reduced = useReducedMotion();
  const total = ENTRIES.length;
  const [received, setReceived] = useState(total);

  useLayoutEffect(() => {
    const el = desk.current;
    if (!el) return;
    return createFeedback(el, { animate: !reduced, onCount: setReceived });
  }, [reduced]);

  return (
    <Figure
      id="feedback"
      chapter="FEEDBACK"
      status={`messages received ${total}/${total}`}
      aria-label="Feedback"
      className="flex flex-col gap-10 px-(--lp-gutter) py-(--lp-section)"
    >
      <div className="flex flex-col gap-5">
        <FigureHeader
          lead={
            <StatusPlate>
              messages received {received}/{total}
            </StatusPlate>
          }
          title={["Don't just watch the future.\n", { term: 'Challenge' }, ' how it\'s built.']}
        />
        <FigureLede>
          The specification is public. The engineering decisions are open to scrutiny. Read the contracts, question the boundaries, challenge the assumptions, and help shape what accountable autonomy should look like.
        </FigureLede>
        <p className="font-pixel text-[15px] leading-5 text-lp-ink">The best systems survive scrutiny.</p>
      </div>

      <div ref={desk} className="relative">
        <p className="sr-only">
          Four messages: an invitation to review the RFCs and challenge weak assumptions, a featured attribution to David Shainidze, Principal
          Engineer at Fundbox, whose quote is pending approval, a pre-alpha status note with no customers or
          endorsements claimed, and an invitation to help shape the conformance checks.
        </p>
        <Rack />
        <div
          data-layer
          aria-hidden
          className="pointer-events-none absolute inset-0 z-[1] max-[899px]:hidden"
        />
        <div className="relative z-[2] grid grid-cols-1 gap-y-16 min-[900px]:grid-cols-2 min-[900px]:gap-x-[88px] min-[900px]:gap-y-0">
          {ENTRIES.map((entry) => (
            <div
              key={entry.id}
              data-window={entry.id}
              data-from={entry.from}
              data-entry={PLACE[entry.id].entry}
              className={cn('relative isolate mt-6 self-start min-[900px]:mt-0', PLACE[entry.id].cls)}
            >
              <Shadow />
              <Stub from={entry.from} />
              <Window title={entry.title} className="relative min-h-full">
                {entry.kind === 'person' ? <Person author={entry.author} /> : <Note entry={entry} />}
              </Window>
              <Port />
            </div>
          ))}
        </div>
      </div>
    </Figure>
  );
}
