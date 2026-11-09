import type { ComponentProps, ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { DITHER_MASKS } from '@/lib/landing/dither';

/** Striped title bar between two small boxes — the landing's "desktop window". */
export function TitleBar({ title, actions }: { title?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="relative flex h-7 shrink-0 items-center gap-1.5 border-b border-lp-line-strong px-1.5">
      <div aria-hidden className="lp-pinstripe absolute inset-x-1.5 inset-y-0" />
      <span aria-hidden className="relative size-3.5 border border-lp-line-strong bg-lp-paper" />
      <span className="relative mx-auto truncate bg-lp-paper px-2 font-mono text-xs leading-4">{title}</span>
      {actions ?? <span aria-hidden className="relative size-3.5 border border-lp-line-strong bg-lp-paper" />}
    </div>
  );
}

export function Window({
  title,
  actions,
  children,
  className,
  ...props
}: Omit<ComponentProps<'div'>, 'title'> & { title?: ReactNode; actions?: ReactNode }) {
  return (
    <div {...props} className={cn('flex flex-col border border-lp-line-strong bg-lp-paper', className)}>
      <TitleBar title={title} actions={actions} />
      {children}
    </div>
  );
}

/** The dithered drop-shadow under a window: put it first inside a `relative isolate` parent. */
export function Shadow({ className }: { className?: string }) {
  return (
    <span
      aria-hidden
      style={{ maskImage: DITHER_MASKS[6], maskSize: '4px 4px' }}
      className={cn('absolute inset-0 translate-x-0.75 translate-y-1 bg-lp-ink-2', className)}
    />
  );
}
