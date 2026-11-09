import type { ReactNode } from 'react';
import { StatusBadge } from './status';

/** Frame for diagrams and screenshots: caption, status label and a source line. */
export function DocsFigure({
  caption,
  children,
  illustrative = true,
  source,
}: {
  caption: ReactNode;
  children: ReactNode;
  illustrative?: boolean;
  source?: ReactNode;
}) {
  return (
    <figure className="not-prose my-8 overflow-hidden rounded-md border border-fd-border bg-fd-card">
      <div className="relative">{children}</div>
      <figcaption className="flex flex-wrap items-start gap-x-3 gap-y-1 border-t border-fd-border px-4 py-2.5 text-sm text-fd-muted-foreground">
        {illustrative ? <StatusBadge kind="illustrative" /> : null}
        <span className="min-w-0 flex-1 basis-64">{caption}</span>
        {source ? <span className="font-mono text-xs">{source}</span> : null}
      </figcaption>
    </figure>
  );
}
