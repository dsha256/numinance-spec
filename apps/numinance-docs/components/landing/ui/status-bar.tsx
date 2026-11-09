'use client';

import { cn } from '@/lib/cn';
import { onStatus, useActive, useFigures } from '@/lib/landing/status';

/** Fixed bottom bar: a lit lamp, the active figure's printed status line, and the chapter index. */
export function StatusBar() {
  return (
    <div className="fixed inset-x-0 bottom-0 z-50 h-(--lp-status-h) border-t border-lp-line-strong bg-lp-paper text-xs font-medium">
      <div className="mx-auto flex h-full max-w-(--lp-max) items-stretch">
        <p aria-hidden className="flex min-w-0 flex-1 items-center gap-2 ps-(--lp-gutter) pe-3 font-pixel text-[13px] leading-4 font-normal tabular-nums antialiased">
          <span className="size-2 shrink-0 rounded-full border border-lp-ink bg-lp-sun" />
          <Line />
        </p>
        <Chapters />
      </div>
    </div>
  );
}

function print(el: HTMLSpanElement) {
  return onStatus((text) => {
    if (el.textContent === text) return;
    el.textContent = text;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    el.animate({ translate: ['0', '100%'] }, { duration: 300, easing: 'steps(12)', pseudoElement: '::after' });
  });
}

function Line() {
  return (
    <span
      ref={print}
      className="relative truncate after:absolute after:inset-0 after:translate-x-full after:bg-lp-paper"
    >
      ready
    </span>
  );
}

function Chapters() {
  const figures = useFigures();
  const active = useActive();
  return (
    <nav aria-label="Chapters" className="flex min-w-0 items-stretch overflow-x-auto pe-(--lp-gutter) [scrollbar-width:none] max-md:hidden [&::-webkit-scrollbar]:hidden">
      {figures.map((f) => (
        <a
          key={f.id}
          href={`#${f.id}`}
          aria-current={active === f.id ? 'location' : undefined}
          className={cn(
            'flex shrink-0 items-center border-s border-lp-line-strong px-1.5 uppercase tracking-wider last:border-e hover:bg-lp-ink hover:text-lp-paper xl:px-2.5',
            active === f.id && 'bg-lp-yellow text-lp-on-yellow',
          )}
        >
          {f.label}
        </a>
      ))}
    </nav>
  );
}
