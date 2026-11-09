import { cn } from '@/lib/cn';
import { CopyButton } from './copy-button';

/**
 * Terminal / code surface. `copy` is the exact text placed on the clipboard —
 * pass the runnable command, not decorated output.
 */
export function CodeWindow({
  title,
  copy,
  children,
  className,
  badge,
}: {
  title?: string;
  copy?: string;
  children: React.ReactNode;
  className?: string;
  badge?: string;
}) {
  return (
    <div
      className={cn(
        'min-w-0 max-w-full overflow-hidden rounded-xl border border-white/10 bg-nm-term text-nm-term-fg shadow-[0_1px_0_rgba(255,255,255,0.04)_inset]',
        className,
      )}
    >
      <div className="flex h-10 items-center justify-between border-b border-white/10 px-4">
        <div className="flex items-center gap-3">
          <span className="flex gap-1.5" aria-hidden>
            <i className="size-2.5 rounded-full bg-white/15" />
            <i className="size-2.5 rounded-full bg-white/15" />
            <i className="size-2.5 rounded-full bg-white/15" />
          </span>
          {title && <span className="font-mono text-xs text-white/50">{title}</span>}
        </div>
        <div className="flex items-center gap-2">
          {badge && <span className="font-mono text-[0.65rem] uppercase tracking-widest text-white/40">{badge}</span>}
          {copy && <CopyButton text={copy} />}
        </div>
      </div>
      <div className="overflow-x-auto p-4 font-mono text-[0.8rem] leading-relaxed md:p-5">{children}</div>
    </div>
  );
}
