import type { ReactNode } from 'react';
import { SPEC_REPO, SPEC_REVISION_SHORT, SPEC_VERIFIED_ON, specUrl } from '@/lib/docs/spec';

/** Source provenance block: public numinance-spec paths pinned to the verified revision. */
export function SourceReference({ paths, note }: { paths: string[]; note?: ReactNode }) {
  return (
    <div className="not-prose my-5 rounded-md border border-fd-border bg-fd-card px-4 py-3 text-sm">
      <div className="mb-2 flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">
        <span className="text-nm-accent">Source</span>
        <a className="normal-case tracking-normal underline" href={SPEC_REPO} target="_blank" rel="noopener noreferrer">
          dsha256/numinance-spec
        </a>
        <span className="normal-case tracking-normal">@ {SPEC_REVISION_SHORT}</span>
      </div>
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {paths.map((p) => (
          <li key={p} className="min-w-0">
            <a
              className="break-all font-mono text-[0.8125rem] text-fd-foreground underline decoration-fd-border underline-offset-4 hover:decoration-nm-accent"
              href={specUrl(p)}
              target="_blank"
              rel="noopener noreferrer"
            >
              {p}
            </a>
          </li>
        ))}
      </ul>
      {note ? <div className="mt-2 text-fd-muted-foreground [&_p]:m-0">{note}</div> : null}
    </div>
  );
}

/**
 * Wraps a code block whose commands/output were really executed. Only use it for snippets that
 * were run; the label states the spec revision and date. Never wrap invented output.
 */
export function VerifiedSnippet({ children, what = 'Command and output captured from a real run', exit }: { children: ReactNode; what?: string; exit?: string }) {
  return (
    <figure className="my-5">
      {children}
      <figcaption className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-xs text-fd-muted-foreground">
        <span className="text-nm-ok">✓ verified</span>
        <span>{what}</span>
        <span>· numinance-spec @ {SPEC_REVISION_SHORT} · {SPEC_VERIFIED_ON}</span>
        {exit ? <span>· {exit}</span> : null}
      </figcaption>
    </figure>
  );
}
