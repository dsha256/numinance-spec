'use client';

/**
 * Shared kit for the docs diagrams. Rules every diagram follows (see /docs/architecture):
 *  - deterministic initial state (identical on server and client; no randomness, no Date.now in render)
 *  - keyboard operable controls (native <button>/radio semantics), visible focus
 *  - reduced motion: no transitions when the user asks for it; state changes are instant
 *  - a static text explanation that stands alone without the graphic
 *  - nothing autoplays on mount; playback only starts from a user action and stops at the end
 */
import { useId, useState, useSyncExternalStore, type ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { StatusBadge } from '../status';

const RM_QUERY = '(prefers-reduced-motion: reduce)';
/**
 * Hydration-safe reduced-motion flag. The server snapshot is always `false`, so server and first
 * client render match; React then re-renders with the real preference. Do not use motion's own
 * hook for attributes: its first client render already reads the media query and mismatches.
 */
export function useReducedMotion(): boolean {
  return useSyncExternalStore(
    (cb) => {
      const m = window.matchMedia(RM_QUERY);
      m.addEventListener('change', cb);
      return () => m.removeEventListener('change', cb);
    },
    () => window.matchMedia(RM_QUERY).matches,
    () => false,
  );
}

export function DiagramShell({
  title,
  question,
  controls,
  children,
  explanation,
  legend,
  status = 'illustrative',
  className,
}: {
  title: string;
  /** The one hard engineering question this diagram answers. */
  question: string;
  controls?: ReactNode;
  children: ReactNode;
  /** Static, screen-reader-complete description of the same information. */
  explanation: ReactNode;
  legend?: ReactNode;
  status?: 'illustrative' | 'public-spec';
  className?: string;
}) {
  const id = useId();
  return (
    <section
      aria-labelledby={`${id}-t`}
      className={cn('not-prose my-8 overflow-hidden rounded-md border border-fd-border bg-fd-card text-fd-foreground', className)}
    >
      <header className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 border-b border-fd-border px-4 py-3">
        <div className="min-w-0">
          <h3 id={`${id}-t`} className="m-0 font-mono text-xs uppercase tracking-[0.12em] text-nm-accent">{title}</h3>
          <p className="m-0 mt-1 text-sm text-fd-muted-foreground">{question}</p>
        </div>
        <StatusBadge kind={status} label={status === 'illustrative' ? 'Illustrative · simulated' : undefined} />
      </header>
      {controls ? <div className="flex flex-wrap items-center gap-2 border-b border-fd-border px-4 py-2.5">{controls}</div> : null}
      <div className="px-2 py-3 sm:px-4 sm:py-4">{children}</div>
      {legend ? <div className="flex flex-wrap gap-x-4 gap-y-1 border-t border-fd-border px-4 py-2 font-mono text-xs text-fd-muted-foreground">{legend}</div> : null}
      <details className="group border-t border-fd-border">
        <summary className="cursor-pointer select-none px-4 py-2 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground hover:text-fd-foreground">
          Text description
        </summary>
        <div className="space-y-2 px-4 pb-4 pt-1 text-sm leading-relaxed text-fd-muted-foreground [&_code]:font-mono [&_code]:text-[0.85em] [&_li]:my-0.5 [&_ol]:list-decimal [&_ol]:pl-5 [&_ul]:list-disc [&_ul]:pl-5">
          {explanation}
        </div>
      </details>
    </section>
  );
}

/** Radio-group segmented control (arrow keys move, Tab leaves). */
export function Segmented<T extends string>({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: string }[];
}) {
  const id = useId();
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex flex-wrap items-center gap-1">
      <span id={id} className="mr-1 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">{label}</span>
      {options.map((o, i) => {
        const on = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={on}
            tabIndex={on ? 0 : -1}
            onClick={() => onChange(o.value)}
            onKeyDown={(e) => {
              const n = options.length;
              let j = -1;
              if (e.key === 'ArrowRight' || e.key === 'ArrowDown') j = (i + 1) % n;
              if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') j = (i + n - 1) % n;
              if (j >= 0) {
                e.preventDefault();
                onChange(options[j].value);
                (e.currentTarget.parentElement?.querySelectorAll('[role=radio]')[j] as HTMLElement | undefined)?.focus();
              }
            }}
            className={cn(
              'min-h-9 rounded-sm border px-3 py-1.5 font-mono text-xs transition-colors',
              on
                ? 'border-lp-ink bg-lp-ink text-lp-paper'
                : 'border-fd-border bg-fd-background text-fd-foreground hover:border-fd-foreground/50',
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

export function DiagramButton({ children, onClick, disabled, pressed, label }: { children: ReactNode; onClick: () => void; disabled?: boolean; pressed?: boolean; label?: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-pressed={pressed}
      aria-label={label}
      className="min-h-9 rounded-sm border border-fd-border bg-fd-background px-3 py-1.5 font-mono text-xs text-fd-foreground hover:border-fd-foreground/50 disabled:cursor-not-allowed disabled:opacity-40 aria-pressed:border-lp-ink aria-pressed:bg-lp-ink aria-pressed:text-lp-paper"
    >
      {children}
    </button>
  );
}

/** Stepper state: manual stepping first, optional one-shot play that stops at the end. */
export function useStepper(count: number) {
  const [i, setI] = useState(0);
  return {
    i,
    set: (n: number) => setI(Math.max(0, Math.min(count - 1, n))),
    next: () => setI((v) => Math.min(count - 1, v + 1)),
    prev: () => setI((v) => Math.max(0, v - 1)),
    reset: () => setI(0),
    last: i === count - 1,
    first: i === 0,
  };
}

/** Allow / deny / require-approval, always with a glyph and a word. */
export function Decision({ kind }: { kind: 'allow' | 'deny' | 'approval' | 'pause' }) {
  const m = {
    allow: ['✓', 'ALLOW', 'text-nm-ok border-nm-ok/50'],
    deny: ['✕', 'DENY', 'text-nm-deny border-nm-deny/50'],
    approval: ['◆', 'REQUIRE APPROVAL', 'text-nm-warn border-nm-warn/50'],
    pause: ['‖', 'PAUSE', 'text-nm-warn border-nm-warn/50'],
  }[kind];
  return <span className={cn('inline-flex items-center gap-1 rounded-sm border px-1.5 py-0.5 font-mono text-xs', m[2])}><span aria-hidden>{m[0]}</span>{m[1]}</span>;
}
