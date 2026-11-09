'use client';

import type { Ref } from 'react';
import { motion } from 'motion/react';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { cn } from '@/lib/cn';
import { mulberry32 } from '@/lib/landing/rng';
import { Shadow } from '@/components/landing/ui/window';
import {
  RECORDS, RESULT, TAMPERED, TAMPERED_DIGEST, linkState, recState, type LinkState, type Outcome, type RecState, type Scenario, type TLine, type VaultState, outcome,
} from './model';

/* ---------- terminal (always dark) ---------- */

const TONE: Record<TLine['tone'], string> = {
  cmd: 'text-[#eceae2]',
  dim: 'text-[#eceae2]/45',
  plain: 'text-[#eceae2]/90',
  ok: 'text-[#f8e875]',
  warn: 'text-[#ffb27a]',
  bad: 'text-[#ff7a33] font-semibold',
};

export function TerminalBody({ typedRef, typedDone, lines, done, live }: { typedRef: Ref<HTMLSpanElement>; typedDone: boolean; lines: TLine[]; done: boolean; live: boolean }) {
  return (
    <div
      aria-hidden
      className="relative h-[25rem] overflow-hidden bg-[#0c0c0b] p-3 font-mono text-[12px] leading-[1.55] text-[#eceae2] sm:h-80 sm:text-[12.5px] lg:h-[21rem]"
    >
      <div className="pointer-events-none absolute inset-0 opacity-[0.07]" style={{ backgroundImage: 'repeating-linear-gradient(to bottom, #fff 0 1px, transparent 1px 3px)' }} />
      <div className="relative flex flex-col justify-end gap-0 break-words whitespace-normal sm:whitespace-pre-wrap">
        <div className="flex">
          <span className="select-none text-[#f8e875]">$&nbsp;</span>
          <span ref={typedRef} />
          {!done && live && <span className="ms-px inline-block h-[1.1em] w-[0.55em] translate-y-[0.15em] animate-lp-caret bg-[#f8e875]" />}
        </div>
        {typedDone &&
          lines.map((l, i) => (
            <div key={i} className={cn(TONE[l.tone])}>
              {l.text}
            </div>
          ))}
      </div>
    </div>
  );
}

/* ---------- record plates + couplers ---------- */

const STATE_LABEL: Record<RecState, string> = { pending: 'QUEUED', inspecting: 'INSPECTING', ok: 'LINK OK', altered: 'ALTERED', flagged: 'UNRELIABLE' };

function Barcode({ seed, alter }: { seed: number; alter?: boolean }) {
  const r = mulberry32(seed * 7919 + 13);
  const cells = Array.from({ length: 24 }, () => r() > 0.5);
  const flip = alter ? new Set([5, 6, 14, 15, 16]) : new Set<number>();
  return (
    <svg aria-hidden viewBox="0 0 24 3" shapeRendering="crispEdges" className="h-3 w-full">
      {cells.map((on, i) => {
        const bad = flip.has(i);
        const fill = bad ? 'var(--lp-sun)' : on ? 'var(--lp-ink)' : 'transparent';
        return <rect key={i} x={i} y={0} width={1} height={3} fill={fill} opacity={on || bad ? 1 : 0} />;
      })}
    </svg>
  );
}

export function Plate({
  i, scenario, step, selected, onPick,
}: { i: number; scenario: Scenario; step: number; selected: boolean; onPick: () => void }) {
  const r = RECORDS[i];
  const st = recState(scenario, i, step);
  const reduced = useReducedMotion();
  const printed = st !== 'pending';
  const bad = st === 'altered';
  const flagged = st === 'flagged';
  return (
    <motion.button
      type="button"
      aria-pressed={selected}
      onClick={onPick}
      initial={false}
      animate={{ y: printed ? 0 : 10, opacity: printed ? (flagged ? 0.6 : 1) : 0.4 }}
      transition={reduced ? { duration: 0 } : { type: 'spring', stiffness: 260, damping: 24 }}
      className={cn(
        'relative isolate flex min-w-0 flex-1 cursor-pointer flex-row items-center gap-3 border bg-lp-paper-2 p-2.5 text-left outline-none xl:flex-col xl:items-stretch xl:gap-2',
        printed ? 'border-lp-line-strong' : 'border-dashed border-lp-line-strong',
        bad && 'border-lp-orange',
        selected && 'outline-2 -outline-offset-4 outline-lp-ink/50',
        'focus-visible:outline-2 focus-visible:outline-lp-ink',
      )}
    >
      {/* link tab: this plate carries its predecessor's digest */}
      <span aria-hidden className={cn('absolute -top-px -left-px hidden size-2.5 border-e border-b border-lp-line-strong xl:block', i === 0 ? 'bg-lp-paper' : 'bg-lp-yellow')} />
      {st === 'inspecting' && (
        <motion.span
          layoutId="ev-scan"
          transition={reduced ? { duration: 0 } : { type: 'spring', stiffness: 300, damping: 30 }}
          className="absolute inset-x-0 -top-3 z-10 flex h-3 items-center justify-center bg-lp-yellow font-pixel text-[11px] leading-none text-lp-on-yellow"
        >
          INSPECT
        </motion.span>
      )}
      {bad && <span aria-hidden className="lp-hazard absolute inset-x-0 top-0 h-1" />}
      {flagged && (
        <span aria-hidden className="pointer-events-none absolute inset-0 -z-10 opacity-50" style={{ backgroundImage: 'repeating-linear-gradient(135deg, var(--lp-line-strong) 0 1px, transparent 1px 6px)' }} />
      )}
      <span className="flex w-36 shrink-0 flex-col gap-1 xl:w-auto">
        <span className="flex items-center justify-between font-pixel text-[11px] leading-4">
          <span className="text-lp-ink-2">{r.n}</span>
          <span className={cn('border px-1', bad ? 'border-lp-orange bg-lp-sun text-black' : st === 'ok' ? 'border-lp-ink bg-lp-yellow text-lp-on-yellow' : 'border-lp-line-strong text-lp-ink-2')}>
            {STATE_LABEL[st]}
          </span>
        </span>
        <span className="font-pixel text-[13px] leading-[1.1]">
          {r.name[0]}
          <br />
          {r.name[1]}
        </span>
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1.5 border-lp-line xl:border-t xl:pt-2">
        <span className="flex flex-col font-mono text-[11px] leading-4">
          <span className="font-pixel text-[11px] text-lp-ink-2">digest</span>
          <span className={cn('whitespace-nowrap', bad && 'text-lp-orange line-through decoration-1')}>{bad ? TAMPERED_DIGEST : r.digest}</span>
        </span>
        <span className="flex flex-col font-mono text-[11px] leading-4">
          <span className="font-pixel text-[11px] text-lp-ink-2">carries prev</span>
          <span className="whitespace-nowrap text-lp-ink-2">{r.prev ?? 'none (chain start)'}</span>
        </span>
      </span>
      <span className="hidden w-16 shrink-0 sm:block xl:w-full">
        <Barcode seed={i + 1} alter={bad} />
      </span>
    </motion.button>
  );
}

const LINK_GLYPH: Record<LinkState, string> = { pending: '', ok: '=', broken: '≠', void: '·' };

export function Coupler({ i, scenario, step }: { i: number; scenario: Scenario; step: number }) {
  const st = linkState(scenario, i, step);
  return (
    <div className="relative flex h-6 shrink-0 items-center justify-center xl:h-auto xl:w-6" role="img" aria-label={`link ${i + 1} to ${i + 2}: ${st}`}>
      <span aria-hidden className={cn('absolute h-full w-px xl:h-px xl:w-full', st === 'broken' ? 'bg-lp-sun' : 'bg-lp-line-strong', st === 'ok' && 'bg-lp-ink')} />
      <span
        aria-hidden
        className={cn(
          'relative z-10 grid size-5 place-items-center border font-pixel text-[13px] leading-none transition-colors',
          st === 'ok' && 'border-lp-ink bg-lp-yellow text-lp-on-yellow',
          st === 'broken' && 'border-lp-ink bg-lp-sun text-black',
          (st === 'pending' || st === 'void') && 'border-dashed border-lp-line-strong bg-lp-paper text-lp-ink-2',
        )}
      >
        {LINK_GLYPH[st]}
      </span>
    </div>
  );
}

/* ---------- trust boundary ---------- */

const KEYS = ['runner key(s)', 'log key', 'origin'] as const;

function Padlock({ closed }: { closed: boolean }) {
  return (
    <svg aria-hidden viewBox="0 0 8 10" shapeRendering="crispEdges" className="size-5 shrink-0 fill-current">
      {closed ? <path d="M2 0h4v1h1v3h-1V1H2v3H1V1h1zM0 4h8v6H0zM3 6h2v2H3z" fillRule="evenodd" /> : <path d="M2 0h4v1h1v1H6V1H2v3h6v6H0V4h1V1h1zM3 6h2v2H3z" fillRule="evenodd" />}
    </svg>
  );
}

export function Vault({ state, scenario, pinnedToggle }: { state: VaultState; scenario: Scenario; pinnedToggle: boolean }) {
  const loaded = state === 'loaded' || state === 'mismatch';
  const label =
    state === 'loaded' ? 'pinned' : state === 'mismatch' ? 'pinned' : state === 'skipped' ? 'not used' : state === 'empty' ? 'not pinned' : scenario === 'valid' && !pinnedToggle ? 'not pinned' : 'standing by';
  return (
    <div className="relative isolate h-full">
      <Shadow />
      <div className={cn('relative flex h-full flex-col border-2 border-lp-ink bg-lp-paper', loaded && 'bg-lp-paper-2')}>
        <div className="flex items-center gap-2 border-b border-lp-ink px-2.5 py-1.5 font-pixel text-[13px] leading-4">
          <Padlock closed={loaded} />
          <span>CALLER-PINNED TRUST MATERIAL</span>
        </div>
        <ul className="flex flex-col gap-1.5 p-2.5">
          {KEYS.map((k) => (
            <li key={k} className="flex items-center gap-2 font-mono text-xs">
              <span
                aria-hidden
                className={cn(
                  'grid size-4 shrink-0 place-items-center border',
                  state === 'loaded' && 'border-lp-ink bg-lp-yellow',
                  state === 'mismatch' && 'border-lp-ink bg-lp-paper',
                  (state === 'empty' || state === 'skipped' || state === 'idle') && 'border-dashed border-lp-line-strong',
                )}
              >
                {state === 'loaded' && <span className="size-1.5 bg-lp-on-yellow" />}
                {state === 'mismatch' && <span className="size-1.5 bg-lp-ink" />}
              </span>
              <span className={cn(!loaded && 'text-lp-ink-2')}>{k}</span>
              <span className="ms-auto font-pixel text-[11px] text-lp-ink-2">{label}</span>
            </li>
          ))}
        </ul>
        <p className="mt-auto border-t border-lp-line px-2.5 py-1.5 text-xs leading-snug text-lp-ink-2">
          Supplied by the caller, outside the bundle. This is the only source of trust.
        </p>
      </div>
    </div>
  );
}

export function Comparator({ state }: { state: VaultState }) {
  const text = state === 'loaded' ? 'KEY MATCH' : state === 'mismatch' ? 'NO MATCH' : state === 'empty' ? 'NOTHING PINNED' : state === 'skipped' ? 'NOT EVALUATED' : 'COMPARE';
  return (
    <div className="relative flex items-center gap-3 py-2">
      <span aria-hidden className="lp-hazard h-1.5 flex-1" />
      <span
        className={cn(
          'relative border px-2.5 py-1 font-pixel text-[13px] leading-4 whitespace-nowrap',
          state === 'loaded' && 'border-lp-ink bg-lp-yellow text-lp-on-yellow',
          state === 'mismatch' && 'border-lp-ink bg-lp-sun text-black',
          state === 'empty' && 'border-lp-ink bg-lp-paper',
          (state === 'skipped' || state === 'idle') && 'border-dashed border-lp-line-strong bg-lp-paper text-lp-ink-2',
        )}
      >
        TRUST BOUNDARY · {text}
      </span>
      <span aria-hidden className="lp-hazard h-1.5 flex-1" />
    </div>
  );
}

export function EmbeddedKey({ scenario }: { scenario: Scenario }) {
  return (
    <div className="relative flex flex-wrap items-center gap-x-3 gap-y-1 border border-dashed border-lp-line-strong px-2.5 py-2">
      <span aria-hidden className="size-3.5 shrink-0 border border-lp-ink bg-[repeating-linear-gradient(135deg,var(--lp-ink)_0_1px,transparent_1px_4px)]" />
      <span className="font-pixel text-[13px] leading-4">EMBEDDED KEY · UNTRUSTED</span>
      <span className="text-xs leading-snug text-lp-ink-2">
        {scenario === 'untrusted' ? 'Signed by a key the caller never pinned.' : 'Travels inside the export. It cannot vouch for the export.'}
      </span>
    </div>
  );
}

/* ---------- inspector ---------- */

export function Inspector({ i, scenario, step }: { i: number; scenario: Scenario; step: number }) {
  const r = RECORDS[i];
  const st = recState(scenario, i, step);
  const altered = st === 'altered';
  const rnd = mulberry32(i * 31 + 5);
  const cells = Array.from({ length: 16 * 6 }, () => rnd());
  const diff = new Set(altered ? [21, 22, 37, 38, 39, 53, 70, 71] : []);
  const rows: [string, string, string?][] = [
    ['recorded digest', r.digest],
    ['recomputed', st === 'pending' ? 'not yet read' : altered ? TAMPERED_DIGEST : st === 'flagged' ? 'not computed' : r.digest, altered ? 'text-lp-orange' : undefined],
    ['predecessor link', r.prev ? (st === 'pending' ? 'not yet read' : st === 'flagged' ? 'predecessor failed' : altered ? 'record content altered' : 'matches predecessor') : 'chain start'],
  ];
  return (
    <div className="relative isolate h-full">
      <Shadow />
      <div className="relative h-full border border-lp-line-strong bg-lp-paper">
        <div className="flex items-center justify-between gap-2 border-b border-lp-line-strong px-2.5 py-1.5 font-pixel text-[13px] leading-4">
          <span>ARTIFACT INSPECTOR</span>
          <span className="text-lp-ink-2">{r.name.join(' ')}</span>
        </div>
        <div className="grid gap-3 p-2.5 sm:grid-cols-[auto_1fr]">
          <svg aria-hidden viewBox="0 0 16 6" shapeRendering="crispEdges" className="h-[4.5rem] w-48 border border-lp-line-strong sm:w-40">
            {cells.map((v, k) => {
              const x = k % 16, y = Math.floor(k / 16);
              const on = v > 0.55 + y * 0.03;
              return <rect key={k} x={x} y={y} width={1} height={1} fill={diff.has(k) ? 'var(--lp-sun)' : 'var(--lp-ink)'} opacity={on || diff.has(k) ? (st === 'pending' ? 0.2 : 1) : 0} />;
            })}
          </svg>
          <dl className="flex min-w-0 flex-col gap-1 font-mono text-[11px] leading-4">
            {rows.map(([k, v, cls]) => (
              <div key={k} className="flex flex-wrap gap-x-2">
                <dt className="w-28 shrink-0 font-pixel text-[11px] text-lp-ink-2">{k}</dt>
                <dd className={cn('min-w-0 break-words', cls)}>{v}</dd>
              </div>
            ))}
          </dl>
        </div>
        <ul className="flex flex-col gap-0.5 border-t border-lp-line px-2.5 py-2 text-xs leading-snug">
          {r.fields.map(([k, v]) => (
            <li key={k} className="flex gap-2">
              <span className="w-28 shrink-0 text-lp-ink-2">{k}</span>
              <span>{v}</span>
            </li>
          ))}
          <li className="mt-1 font-pixel text-[11px] text-lp-ink-2">illustrative fields · not the evidence/v1 schema</li>
        </ul>
      </div>
    </div>
  );
}

/* ---------- result + proof strip ---------- */

export function ResultReadout({ scenario, pinned, step }: { scenario: Scenario; pinned: boolean; step: number }) {
  const done = step >= 7;
  const o: Outcome = outcome(scenario, pinned);
  const info = RESULT[o];
  const reduced = useReducedMotion();
  return (
    <div className="flex flex-col gap-3 p-3" aria-live="polite">
      <div className="min-h-[7.5rem]">
        {done ? (
          <motion.div key={`${scenario}-${pinned}`} initial={reduced ? false : { opacity: 0, scale: 0.96 }} animate={{ opacity: 1, scale: 1 }} transition={{ duration: 0.2 }} className="flex flex-col gap-2">
            <span
              className={cn(
                'w-fit border-2 px-2.5 py-1 font-pixel text-lg leading-6',
                o === 'verified' && 'border-lp-ink bg-lp-yellow text-lp-on-yellow',
                o === 'broken' && 'border-lp-ink bg-lp-sun text-black',
                o === 'consistent' && 'border-lp-ink bg-lp-paper-2 text-lp-ink',
              )}
            >
              {info.label}
            </span>
            <p className="text-sm leading-relaxed">{info.body}</p>
          </motion.div>
        ) : (
          <div className="flex flex-col gap-2">
            <span className="w-fit border-2 border-dashed border-lp-line-strong px-2.5 py-1 font-pixel text-lg leading-6 text-lp-ink-2">
              {step === 0 ? 'AWAITING RUN' : 'VERIFYING …'}
            </span>
            <p className="text-sm leading-relaxed text-lp-ink-2">Records are inspected one by one, then compared against trust material the caller pinned.</p>
          </div>
        )}
      </div>
      <ProofStrip scenario={scenario} pinned={pinned} step={step} />
      <p className="border-t border-lp-line pt-2 text-xs leading-snug text-lp-ink-2">
        <strong className="font-medium text-lp-ink">Hash links prove structure only.</strong> They never establish authenticity. A result is plain-language and illustrative here; real levels: verified, failed, unauthenticated, self-consistent, incomplete, unsigned, unanchored.
      </p>
    </div>
  );
}

function ProofStrip({ scenario, pinned, step }: { scenario: Scenario; pinned: boolean; step: number }) {
  const o = outcome(scenario, pinned);
  const cell = 'flex h-9 flex-col items-center justify-center border font-pixel text-[11px] leading-none';
  return (
    <div className="grid grid-cols-[repeat(5,1fr)_1.5fr_1.5fr] gap-1" role="img" aria-label="Proof strip: five record links, trust, result">
      {RECORDS.map((r, i) => {
        const st = recState(scenario, i, step);
        return (
          <span key={r.id} className={cn(cell, st === 'ok' && 'border-lp-ink bg-lp-yellow text-lp-on-yellow', st === 'altered' && 'border-lp-ink bg-lp-sun text-black', st === 'flagged' && 'border-lp-line-strong bg-lp-paper-2 text-lp-ink-2', (st === 'pending' || st === 'inspecting') && 'border-dashed border-lp-line-strong text-lp-ink-2')}>
            {r.n}
            <span className="mt-0.5">{st === 'ok' ? '=' : st === 'altered' ? '≠' : st === 'flagged' ? '·' : '…'}</span>
          </span>
        );
      })}
      <span className={cn(cell, step >= 6 && (scenario === 'valid' && pinned ? 'border-lp-ink bg-lp-yellow text-lp-on-yellow' : scenario === 'altered' ? 'border-lp-line-strong bg-lp-paper-2 text-lp-ink-2' : 'border-lp-ink bg-lp-sun text-black'), step < 6 && 'border-dashed border-lp-line-strong text-lp-ink-2')}>
        TRUST
        <span className="mt-0.5">{step < 6 ? '…' : scenario === 'valid' && pinned ? 'pinned' : scenario === 'altered' ? 'n/a' : 'not set'}</span>
      </span>
      <span className={cn(cell, step >= 7 ? (o === 'verified' ? 'border-lp-ink bg-lp-ink text-lp-paper' : o === 'broken' ? 'border-lp-ink bg-lp-sun text-black' : 'border-lp-ink bg-lp-paper-2') : 'border-dashed border-lp-line-strong text-lp-ink-2')}>
        RESULT
        <span className="mt-0.5">{step < 7 ? '…' : o === 'verified' ? 'verified*' : o === 'broken' ? 'broken' : 'untrusted'}</span>
      </span>
    </div>
  );
}

export { TAMPERED };
