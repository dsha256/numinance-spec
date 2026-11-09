import { cn } from '@/lib/cn';

/**
 * Official Numinance horizontal lockup. The ink artwork (#121417) is used on
 * light surfaces and the bone artwork (#F4F1EA) on dark ones; both files are
 * served from /public/brand and swapped by theme with CSS only (no flash).
 */
export function Logo({ className, priority = false }: { className?: string; priority?: boolean }) {
  return (
    <span className={cn('inline-flex shrink-0 items-center', className)}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src="/brand/numinance-lockup-horizontal-ink.svg"
        alt="Numinance"
        width={589}
        height={96}
        className="h-full w-auto dark:hidden"
        fetchPriority={priority ? 'high' : undefined}
      />
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src="/brand/numinance-lockup-horizontal-bone.svg"
        alt="Numinance"
        width={589}
        height={96}
        className="hidden h-full w-auto dark:block"
        fetchPriority={priority ? 'high' : undefined}
      />
    </span>
  );
}

/** The official symbol outline (96-unit grid) — identical geometry to symbol-*.svg. */
export const SYMBOL_PATH =
  'M24 0H34A6 6 0 0 1 40 6V48A12 12 0 0 0 52 60H90A6 6 0 0 1 96 66V72A24 24 0 0 1 72 96H20L0 76V24A24 24 0 0 1 24 0Z M58 0H72A24 24 0 0 1 96 24V42A6 6 0 0 1 90 48H58A6 6 0 0 1 52 42V6A6 6 0 0 1 58 0Z';

/** Inline symbol that inherits `currentColor`; for large decorative uses. */
export function Symbol({ className, title }: { className?: string; title?: string }) {
  return (
    <svg
      viewBox="0 0 96 96"
      className={className}
      fill="currentColor"
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      <path d={SYMBOL_PATH} />
    </svg>
  );
}
