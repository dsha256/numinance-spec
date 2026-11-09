import type { ReactNode } from 'react';
import { StatusBar } from './status-bar';

/** The framed editorial sheet for the landing page: 1400px column, hairline frame lines, status bar. */
export function Sheet({ children }: { children: ReactNode }) {
  return (
    <div className="landing overflow-x-clip">
      <div className="mx-auto w-full max-w-(--lp-max) border-lp-line pb-(--lp-status-h) sheet:border-x">{children}</div>
      <StatusBar />
    </div>
  );
}

/** Ruler strip that closes a figure: 24px minor ticks, 240px major ticks. */
export function Ruler() {
  return (
    <div aria-hidden className="relative col-span-full -mx-(--lp-gutter) h-10 border-t border-lp-line-strong">
      <div className="h-1.5 bg-[repeating-linear-gradient(to_right,var(--lp-line-strong)_0_1px,transparent_1px_24px)] bg-position-(--lp-gutter)" />
      <div className="absolute inset-x-0 top-0 h-2.5 bg-[repeating-linear-gradient(to_right,var(--lp-line-strong)_0_1px,transparent_1px_240px)] bg-position-(--lp-gutter)" />
    </div>
  );
}
