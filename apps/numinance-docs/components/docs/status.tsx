import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { STATUS, type StatusKind } from '@/lib/docs/status';
import { SPEC_REPO } from '@/lib/docs/spec';

const toneClass: Record<(typeof STATUS)[StatusKind]['tone'], string> = {
  ok: 'border-nm-ok/50 text-nm-ok bg-nm-ok/10',
  info: 'border-nm-accent/50 text-nm-accent bg-nm-accent-soft',
  warn: 'border-nm-warn/50 text-nm-warn bg-nm-warn/10',
  muted: 'border-fd-border text-fd-muted-foreground bg-fd-muted',
  accent: 'border-lp-orange/60 text-lp-orange bg-lp-orange/10',
};

/** Inline status chip. Always text + glyph, never colour alone. */
export function StatusBadge({ kind, label, className }: { kind: StatusKind; label?: string; className?: string }) {
  const s = STATUS[kind];
  return (
    <span
      className={cn(
        'not-prose inline-flex items-center gap-1.5 whitespace-nowrap rounded-sm border px-1.5 py-0.5 align-middle font-mono text-xs leading-none',
        toneClass[s.tone],
        className,
      )}
      title={s.meaning}
    >
      <span aria-hidden="true">{s.glyph}</span>
      {label ?? s.label}
    </span>
  );
}

/** Block status banner placed at the top of a capability page. */
export function FeatureStatus({ kind, children, label }: { kind: StatusKind; children?: ReactNode; label?: string }) {
  const s = STATUS[kind];
  return (
    <div
      role="note"
      aria-label={`Status: ${label ?? s.label}`}
      className="not-prose my-6 flex flex-wrap items-start gap-x-4 gap-y-2 rounded-md border border-fd-border bg-fd-card px-4 py-3"
    >
      <StatusBadge kind={kind} label={label} />
      <div className="min-w-0 flex-1 basis-64 text-sm leading-relaxed text-fd-muted-foreground [&_p]:m-0 [&_a]:underline [&_code]:font-mono [&_code]:text-[0.85em]">
        {children ?? s.meaning}
      </div>
    </div>
  );
}

/** Roadmap item. Gives no command, package name, version or date. */
export function ComingSoon({ title, children }: { title?: string; children?: ReactNode }) {
  return (
    <div className="not-prose my-6 rounded-md border border-dashed border-lp-orange/60 bg-lp-orange/5 px-4 py-4">
      <div className="mb-2 flex items-center gap-2">
        <StatusBadge kind="coming-soon" />
        {title ? <span className="text-sm font-medium text-fd-foreground">{title}</span> : null}
      </div>
      <div className="text-sm leading-relaxed text-fd-muted-foreground [&_a]:underline [&_p]:my-1">{children}</div>
    </div>
  );
}

/** Architecture that is designed but not shipped. Keeps design prose visibly separate from fact. */
export function DesignNote({ title = 'Design intent', children }: { title?: string; children: ReactNode }) {
  return (
    <aside className="not-prose my-6 rounded-md border-l-2 border-nm-warn bg-fd-card px-4 py-3">
      <div className="mb-1 flex items-center gap-2 font-mono text-xs uppercase tracking-wider text-nm-warn">
        <span aria-hidden="true">◌</span> {title}
        <span className="text-fd-muted-foreground normal-case tracking-normal">· under development, not shipped</span>
      </div>
      <div className="text-sm leading-relaxed text-fd-foreground/90 [&_a]:underline [&_code]:font-mono [&_code]:text-[0.85em] [&_p]:my-1.5">{children}</div>
    </aside>
  );
}

/** One-line product truth under the title. */
export function ShortProductTruth({ children }: { children: ReactNode }) {
  return <div className="mb-6 border-l-2 border-lp-orange pl-4 text-lg leading-relaxed text-fd-foreground [&_p]:m-0">{children}</div>;
}

/** Legacy provenance note (kept for pages not yet migrated to SourceReference). */
export function SpecNote({ sources }: { sources?: ReactNode }) {
  return (
    <aside className="not-prose my-4 rounded-md border border-fd-border bg-fd-card px-4 py-2.5 text-sm text-fd-muted-foreground">
      <span className="font-mono text-xs uppercase tracking-wider text-nm-accent">Source</span>{' '}
      {sources ? <>{sources}. </> : null}
      Public <a className="underline" href={SPEC_REPO} target="_blank" rel="noopener noreferrer">numinance-spec</a> is normative; paths are relative to a checkout. Numinance is pre-alpha.
    </aside>
  );
}

const legacy = { preAlpha: 'pre-alpha', designed: 'designed', underDevelopment: 'designed', tbc: 'illustrative' } as const;
/** Legacy inline chip. Prefer <StatusBadge kind="…" />. */
export function Status({ kind }: { kind: keyof typeof legacy }) {
  return <StatusBadge kind={legacy[kind]} label={kind === 'tbc' ? 'Status: to be confirmed' : undefined} />;
}

/** Example that is not the output of a real run. */
export function IllustrativeNote({ children }: { children?: ReactNode }) {
  return (
    <aside className="not-prose my-4 flex gap-3 rounded-md border border-dashed border-fd-border bg-fd-card px-4 py-2.5 text-sm text-fd-muted-foreground">
      <StatusBadge kind="illustrative" />
      <div className="min-w-0 flex-1 [&_p]:m-0">{children ?? 'This example is illustrative. It is not output from a real run.'}</div>
    </aside>
  );
}
