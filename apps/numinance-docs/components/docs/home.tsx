import Link from 'next/link';
import type { ReactNode } from 'react';
import { StatusBadge } from './status';
import type { StatusKind } from '@/lib/docs/status';
import { cn } from '@/lib/cn';

/** Slim, static system schematic: authority sits outside the model. No animation; decorative. */
function Schematic() {
  return (
    <svg viewBox="0 0 360 150" role="img" aria-label="Schematic: an objective enters context, the harness proposes work, policy and grants decide, verification checks, and evidence is sealed." className="h-auto w-full max-w-md text-fd-foreground">
      <defs>
        <pattern id="dh-dots" width="8" height="8" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r="0.7" fill="currentColor" opacity="0.18" /></pattern>
      </defs>
      <rect x="0.5" y="0.5" width="359" height="149" fill="url(#dh-dots)" stroke="currentColor" strokeOpacity="0.25" />
      {[
        ['Objective', 14, 'ink'],
        ['Context', 84, 'ink'],
        ['Harness', 154, 'ink'],
        ['Verify', 224, 'ink'],
        ['Evidence', 294, 'ink'],
      ].map(([t, x]) => (
        <g key={t as string}>
          <rect x={x as number} y="26" width="56" height="34" fill="var(--color-fd-background)" stroke="currentColor" strokeOpacity="0.7" />
          <text x={(x as number) + 28} y="47" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="8.5" fill="currentColor">{t as string}</text>
        </g>
      ))}
      {[70, 140, 210, 280].map((x) => (
        <path key={x} d={`M${x} 43h14`} stroke="currentColor" strokeOpacity="0.6" markerEnd="url(#dh-arrow)" />
      ))}
      <defs><marker id="dh-arrow" viewBox="0 0 6 6" refX="5" refY="3" markerWidth="5" markerHeight="5" orient="auto"><path d="M0 0L6 3L0 6z" fill="currentColor" opacity="0.7" /></marker></defs>
      <rect x="14" y="86" width="336" height="40" fill="var(--color-lp-yellow, #f8e875)" fillOpacity="0.9" stroke="#121210" strokeOpacity="0.9" />
      <text x="182" y="103" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="9" fill="#121210">POLICY · GRANTS · BROKER · EGRESS</text>
      <text x="182" y="116" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="7.5" fill="#121210" opacity="0.8">fixed trust kernel, outside the model</text>
      {[42, 112, 182, 252, 322].map((x) => (
        <path key={x} d={`M${x} 60V86`} stroke="currentColor" strokeOpacity="0.4" strokeDasharray="2 3" />
      ))}
    </svg>
  );
}

export function DocsHomeHero({ children }: { children?: ReactNode }) {
  return (
    <div className="not-prose relative mb-10 grid gap-8 overflow-hidden rounded-md border border-fd-border bg-fd-card p-6 md:grid-cols-[1.15fr_1fr] md:p-8">
      <div className="min-w-0">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <StatusBadge kind="pre-alpha" label="Pre-alpha" />
          <StatusBadge kind="public-spec" label="Open CLI + spec: public" />
        </div>
        <h2 className="m-0 text-3xl font-light leading-tight tracking-tight text-fd-foreground md:text-4xl">
          Give it context.<br /><span className="text-lp-orange">Let it engineer.</span>
        </h2>
        <p className="mt-4 max-w-prose text-base leading-relaxed text-fd-muted-foreground">
          Numinance is a governed engineering runtime. A customer-defined policy, not the model, decides how far each piece of work may go. The intelligence is private. The contract is open, and this site documents it.
        </p>
        {children}
      </div>
      <div className="flex items-center justify-center text-fd-foreground"><Schematic /></div>
    </div>
  );
}

export interface PathItem { title: string; href: string; description: string; status?: StatusKind; tag?: string }

/** "What do you want to accomplish?" cards: journeys before module names. */
export function PathCards({ items, cols = 3 }: { items: PathItem[]; cols?: 2 | 3 }) {
  return (
    <div className={cn('not-prose my-6 grid gap-3', cols === 3 ? 'sm:grid-cols-2 lg:grid-cols-3' : 'sm:grid-cols-2')}>
      {items.map((it) => (
        <Link
          key={it.href}
          href={it.href}
          className="group flex min-h-32 flex-col gap-2 rounded-md border border-fd-border bg-fd-card p-4 transition-colors hover:border-fd-foreground/60 focus-visible:border-fd-foreground"
        >
          <div className="flex items-start justify-between gap-2">
            <span className="text-base font-medium text-fd-foreground">{it.title}</span>
            <span aria-hidden className="font-mono text-fd-muted-foreground transition-transform group-hover:translate-x-0.5">→</span>
          </div>
          <p className="m-0 flex-1 text-sm leading-relaxed text-fd-muted-foreground">{it.description}</p>
          {it.status ? <div><StatusBadge kind={it.status} /></div> : null}
        </Link>
      ))}
    </div>
  );
}
