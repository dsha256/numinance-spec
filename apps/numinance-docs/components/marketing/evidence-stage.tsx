'use client';

import { useId, useRef, useState } from 'react';
import Link from 'next/link';
import { AnimatePresence, motion } from 'motion/react';
import { KeyRound, Plus, RotateCcw } from 'lucide-react';
import { CodeWindow } from '@/components/ui/code-window';
import { cn } from '@/lib/cn';
import { EASE, dur } from '@/lib/motion';

const COMMAND = 'numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR';

/** Placeholder records. Titles and digests are illustrative, not spec values. */
const RECORDS = [
  { title: 'Intent received', digest: '7f3a…c91e' },
  { title: 'Plan recorded', digest: '2b90…14d7' },
  { title: 'Change proposed', digest: 'e5c1…a08b' },
  { title: 'Checks executed', digest: '94d2…3f6c' },
  { title: 'Independent review recorded', digest: '1ac8…d725' },
  { title: 'Approval recorded', digest: 'c07e…58b3' },
  { title: 'Release recorded', digest: '6d4f…e1a9' },
  { title: 'Deployment recorded', digest: 'a3b6…0c42' },
] as const;

const SOURCES = [
  'numinance-spec/docs/evidence/',
  'numinance-spec/schemas/evidence/v1/',
  'numinance-spec/conformance/evidence/v1/',
  'numinance-spec/rfcs/0003-evidence-chain-format-v1.md',
  'numinance-spec/sdk/go/evidenceverify/',
] as const;

type Trust = 'unpinned' | 'pinned';

const OUTCOMES: Record<Trust, { label: string; summary: string; detail: string }> = {
  unpinned: {
    label: 'Limited outcome',
    summary: 'Chain coherent. Trust not established.',
    detail:
      'Each record points at the digest of the record before it, so the export is internally coherent. That does not show who produced it. With no caller-pinned keys, the verifier has no independent basis for trust and cannot report a fully verified result. Real levels: verified, failed, unauthenticated, self-consistent, incomplete, unsigned, unanchored; a key from the export never reaches verified.',
  },
  pinned: {
    label: 'Verified (simulated example)',
    summary: 'Trust established against caller-pinned material.',
    detail:
      'The caller supplied runner key(s), a log key and an origin, so the recorded material can be checked against keys the caller chose to trust, not keys the export carries. This is a simulated example, not live attestation. In a real run the caller must supply the pinned material and the verifier computes the outcome.',
  },
};

export function EvidenceStage() {
  const [count, setCount] = useState(4);
  const [selected, setSelected] = useState(3);
  const [hover, setHover] = useState<number | null>(null);
  const [fresh, setFresh] = useState<number | null>(null);
  const [trust, setTrust] = useState<Trust>('unpinned');
  const [announce, setAnnounce] = useState('');
  const active = Math.min(hover ?? selected, count - 1);
  const full = count >= RECORDS.length;

  const append = () => {
    if (full) return;
    setFresh(count);
    setSelected(count);
    setCount(count + 1);
    setAnnounce(`Record ${count + 1} appended. Digest path highlighted back to record 1.`);
  };
  const reset = () => {
    setCount(4);
    setSelected(3);
    setFresh(null);
    setAnnounce('Sequence reset to four records.');
  };

  return (
    <div className="space-y-6">
      <div className="overflow-hidden rounded-2xl border border-nm-line-strong bg-nm-raised">
        <div className="flex flex-col gap-4 border-b border-nm-line px-4 py-4 sm:flex-row sm:items-center sm:justify-between md:px-6">
          <p className="nm-eyebrow">Interactive architecture preview · Illustrative · Pre-alpha</p>
          <TrustToggle value={trust} onChange={setTrust} />
        </div>

        <div className="grid divide-y divide-nm-line lg:grid-cols-[1.15fr_1fr_1fr] lg:divide-x lg:divide-y-0">
          {/* Records */}
          <Panel index="01" title="Records" caption="Append-only sequence. Digests are placeholders.">
            <ol className="relative" aria-label="Illustrative evidence records">
              {RECORDS.slice(0, count).map((r, i) => {
                const inPath = i <= active;
                const prev = i === 0 ? 'none (first record)' : RECORDS[i - 1].digest;
                return (
                  <motion.li
                    key={r.digest}
                    initial={i === fresh ? { opacity: 0, y: 16 } : false}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: dur.enter, ease: EASE }}
                    className="relative pl-7"
                  >
                    <span
                      aria-hidden
                      className={cn(
                        'absolute left-[7px] top-0 w-px transition-colors duration-200',
                        i === count - 1 ? 'h-5' : 'h-full',
                        i === 0 && 'top-5 h-[calc(100%-1.25rem)]',
                        i > 0 && inPath ? 'bg-nm-accent' : 'bg-nm-line-strong',
                      )}
                    />
                    <span
                      aria-hidden
                      className={cn(
                        'absolute left-0 top-[1.05rem] size-[15px] rounded-full border-2 bg-nm-raised transition-colors duration-200',
                        i === active ? 'border-nm-accent bg-nm-accent' : inPath ? 'border-nm-accent' : 'border-nm-line-strong',
                      )}
                    />
                    <button
                      type="button"
                      aria-pressed={i === selected}
                      onClick={() => setSelected(i)}
                      onPointerEnter={() => setHover(i)}
                      onPointerLeave={() => setHover(null)}
                      onFocus={() => setHover(i)}
                      onBlur={() => setHover(null)}
                      className={cn(
                        'my-1 block min-h-11 w-full rounded-lg border px-3 py-2 text-left transition-colors duration-200',
                        i === active ? 'border-nm-accent bg-nm-accent-soft' : 'border-transparent hover:bg-nm-surface',
                      )}
                    >
                      <span className="flex items-baseline justify-between gap-3">
                        <span className="text-sm font-medium text-nm-fg">{r.title}</span>
                        <span className="font-mono text-[0.7rem] text-nm-faint">#{String(i + 1).padStart(4, '0')}</span>
                      </span>
                      <span className="mt-1 block font-mono text-[0.72rem] leading-5 text-nm-muted">
                        <span className="text-nm-faint">digest </span>
                        <span className={inPath ? 'text-nm-accent' : undefined}>{r.digest}</span>
                        <br />
                        <span className="text-nm-faint">prev&nbsp;&nbsp;&nbsp;</span>
                        <span className={i > 0 && inPath ? 'text-nm-accent' : undefined}>{prev}</span>
                      </span>
                    </button>
                  </motion.li>
                );
              })}
            </ol>
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={append}
                disabled={full}
                className="inline-flex min-h-11 items-center gap-2 rounded-full bg-nm-fg px-4 text-sm font-medium text-nm-bg transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
              >
                <Plus className="size-4" aria-hidden />
                {full ? 'Sequence complete' : 'Append next record'}
              </button>
              {count > 4 && (
                <button
                  type="button"
                  onClick={reset}
                  className="inline-flex min-h-11 items-center gap-2 rounded-full border border-nm-line-strong px-4 text-sm font-medium text-nm-fg hover:bg-nm-surface"
                >
                  <RotateCcw className="size-4" aria-hidden />
                  Reset
                </button>
              )}
            </div>
            <p className="mt-3 text-xs leading-5 text-nm-faint">
              Highlighted path: each record references the digest of the one before it, back to the first record.
            </p>
            <p className="sr-only" role="status">
              {announce}
            </p>
          </Panel>

          {/* Trust */}
          <Panel index="02" title="Trust" caption="Who decides which keys count.">
            <div className="space-y-4">
              <TrustGroup
                heading="Supplied by the caller"
                tone="pinned"
                supplied={trust === 'pinned'}
                items={['Runner key(s)', 'Log key', 'Log origin']}
              />
              <TrustGroup
                heading="Embedded in the export bundle"
                tone="embedded"
                supplied
                items={['Keys carried inside the bundle']}
              />
            </div>
            <p className="mt-4 text-xs leading-5 text-nm-muted">
              {trust === 'pinned'
                ? 'Pinned material is the trust root. Keys inside the bundle are still not trusted on their own.'
                : 'Nothing is pinned. Keys inside the bundle cannot vouch for the bundle that carries them.'}
            </p>
          </Panel>

          {/* Report */}
          <Panel index="03" title="Report" caption="What each outcome means.">
            <div aria-live="polite" className="min-h-[15rem]">
              <AnimatePresence mode="wait" initial={false}>
                <motion.div
                  key={trust}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  transition={{ duration: dur.panel, ease: EASE }}
                >
                  <p className={cn('font-mono text-xs uppercase tracking-[0.12em]', trust === 'pinned' ? 'text-nm-ok' : 'text-nm-warn')}>
                    {OUTCOMES[trust].label}
                  </p>
                  <p className="mt-2 text-lg font-semibold leading-snug tracking-tight">{OUTCOMES[trust].summary}</p>
                  <p className="mt-3 text-sm leading-6 text-nm-muted">{OUTCOMES[trust].detail}</p>
                </motion.div>
              </AnimatePresence>
            </div>
            <dl className="mt-4 space-y-2 border-t border-nm-line pt-4 text-xs leading-5">
              {(['unpinned', 'pinned'] as const).map((k) => (
                <div
                  key={k}
                  className={cn('rounded-lg border px-3 py-2 transition-colors', trust === k ? 'border-nm-accent bg-nm-accent-soft' : 'border-nm-line')}
                >
                  <dt className="font-medium text-nm-fg">{k === 'unpinned' ? 'Coherent, trust not established' : 'Verified against pinned material'}</dt>
                  <dd className="text-nm-muted">
                    {k === 'unpinned' ? 'Internal consistency only. Says nothing about origin.' : 'Requires caller-pinned runner key(s), log key and origin.'}
                  </dd>
                </div>
              ))}
            </dl>
          </Panel>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1.1fr_1fr]">
        <div className="min-w-0">
          <CodeWindow title="terminal" copy={COMMAND} badge="CLI">
            <p className="whitespace-pre-wrap break-all">
              <span className="select-none text-white/40">$ </span>
              {COMMAND}
            </p>
          </CodeWindow>
          <p className="mt-3 text-sm leading-6 text-nm-muted">
            The verifier operates on an <span className="font-mono text-nm-fg">evidence/v1</span> export directory, not on arbitrary JSON files. Exactly one key mode is required
            (pinned keys, <span className="font-mono">--trust-bundle-keys</span> or <span className="font-mono">--skip-signatures</span>); without one it exits 2.
          </p>
        </div>
        <div className="rounded-xl border border-nm-line p-5">
          <p className="nm-eyebrow mb-3">Specification sources</p>
          <ul className="space-y-1">
            {SOURCES.map((s) => (
              <li key={s}>
                <Link
                  href="/docs/evidence"
                  className="block break-all py-1 font-mono text-[0.78rem] text-nm-muted underline-offset-4 transition-colors hover:text-nm-fg hover:underline"
                >
                  {s}
                </Link>
              </li>
            ))}
          </ul>
          <p className="mt-3 text-xs text-nm-faint">Paths refer to the numinance-spec repository. Each link opens the evidence reference.</p>
        </div>
      </div>
    </div>
  );
}

/** Closing statement shared by the section and the standalone page. */
export function EvidenceCallout() {
  return (
    <p className="mt-16 max-w-3xl text-balance text-[clamp(1.6rem,3.4vw,2.6rem)] font-semibold leading-[1.1] tracking-[-0.025em]">
      The engineer can be autonomous. <span className="text-nm-muted">Trust doesn&rsquo;t have to be.</span>
    </p>
  );
}

function Panel({ index, title, caption, children }: { index: string; title: string; caption: string; children: React.ReactNode }) {
  return (
    <div className="p-4 md:p-6">
      <div className="mb-5 flex items-baseline gap-3">
        <span className="font-mono text-xs text-nm-faint">{index}</span>
        <h3 className="text-base font-semibold tracking-tight">{title}</h3>
      </div>
      <p className="-mt-3 mb-5 text-xs text-nm-faint">{caption}</p>
      {children}
    </div>
  );
}

function TrustGroup({
  heading,
  tone,
  supplied,
  items,
}: {
  heading: string;
  tone: 'pinned' | 'embedded';
  supplied: boolean;
  items: string[];
}) {
  const pinned = tone === 'pinned';
  return (
    <div>
      <p className="mb-2 text-xs font-medium text-nm-muted">{heading}</p>
      <ul className="space-y-2">
        {items.map((it) => (
          <li
            key={it}
            className={cn(
              'flex min-h-11 items-center gap-3 rounded-lg border px-3 text-sm transition-colors duration-200',
              pinned && supplied && 'border-nm-accent bg-nm-accent-soft text-nm-fg',
              pinned && !supplied && 'border-dashed border-nm-line-strong text-nm-faint',
              !pinned && 'border-dashed border-nm-line-strong text-nm-muted',
            )}
          >
            <KeyRound className={cn('size-4 shrink-0', pinned && supplied ? 'text-nm-accent' : 'text-nm-faint')} aria-hidden />
            <span className="flex-1">{it}</span>
            <span className="font-mono text-[0.68rem] uppercase tracking-wider text-nm-faint">
              {pinned ? (supplied ? 'pinned' : 'not supplied') : 'untrusted'}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function TrustToggle({ value, onChange }: { value: Trust; onChange: (t: Trust) => void }) {
  const id = useId();
  const refs = useRef<Record<Trust, HTMLButtonElement | null>>({ unpinned: null, pinned: null });
  const opts: { id: Trust; label: string }[] = [
    { id: 'unpinned', label: 'Unpinned' },
    { id: 'pinned', label: 'Caller-pinned' },
  ];
  const onKey = (e: React.KeyboardEvent) => {
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) return;
    e.preventDefault();
    const next: Trust = value === 'unpinned' ? 'pinned' : 'unpinned';
    onChange(next);
    refs.current[next]?.focus();
  };
  return (
    <div className="flex items-center gap-3">
      <span id={`${id}-l`} className="font-mono text-xs text-nm-muted">
        Trust roots:
      </span>
      <div
        role="radiogroup"
        aria-labelledby={`${id}-l`}
        onKeyDown={onKey}
        className="inline-flex rounded-full border border-nm-line bg-nm-bg p-1"
      >
        {opts.map((o) => {
          const on = o.id === value;
          return (
            <button
              key={o.id}
              ref={(el) => {
                refs.current[o.id] = el;
              }}
              type="button"
              role="radio"
              aria-checked={on}
              tabIndex={on ? 0 : -1}
              onClick={() => onChange(o.id)}
              className={cn(
                'relative min-h-10 rounded-full px-4 text-sm font-medium transition-colors duration-200',
                on ? 'text-nm-bg' : 'text-nm-muted hover:text-nm-fg',
              )}
            >
              {on && (
                <motion.span
                  layoutId={`${id}-pill`}
                  className="absolute inset-0 -z-0 rounded-full bg-nm-fg"
                  transition={{ duration: dur.tab, ease: EASE }}
                />
              )}
              <span className="relative">{o.label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
