import type { ComponentProps } from 'react';
import { cn } from '@/lib/cn';

/** Small pixel-type label with an optional 8x8 pixel icon (SVG path in an 8-cell grid). */
export function Chip({ icon, children, className, ...props }: ComponentProps<'span'> & { icon?: string }) {
  return (
    <span
      {...props}
      className={cn(
        'inline-flex h-6 items-center gap-1.5 border border-lp-line-strong bg-lp-paper px-2 font-pixel text-[13px] leading-4 whitespace-nowrap antialiased',
        className,
      )}
    >
      {icon && (
        <svg aria-hidden viewBox="0 0 8 8" shapeRendering="crispEdges" className="size-2.5 shrink-0 fill-current">
          <path d={icon} />
        </svg>
      )}
      {children}
    </span>
  );
}

/** 8x8 pixel icons for the five lifecycle stages. */
export const STAGE_ICONS = {
  understand: 'M3 0h2v1H3zM1 1h2v1H1zM5 1h2v1H5zM0 3h1v2H0zM7 3h1v2H7zM1 6h2v1H1zM5 6h2v1H5zM3 7h2v1H3zM3 3h2v2H3z',
  plan: 'M0 1h8v1H0zM0 4h8v1H0zM0 7h8v1H0zM0 1h1v6H0zM7 1h1v6H7z',
  act: 'M3 0h2v3H3zM1 3h6v2H1zM2 5h1v3H2zM5 5h1v3H5z',
  verify: 'M0 4h1v1H0zM1 5h1v1H1zM2 6h1v1H2zM3 5h1v1H3zM4 4h1v1H4zM5 3h1v1H5zM6 2h1v1H6zM7 1h1v1H7z',
  evidence: 'M1 0h5v1H1zM1 1h1v7H1zM6 1h1v1H6zM2 7h5v1H2zM3 3h3v1H3zM3 5h3v1H3zM7 2h1v6H7z',
} as const;
