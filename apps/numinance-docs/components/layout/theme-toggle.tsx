'use client';

import { useEffect, useState } from 'react';
import { flushSync } from 'react-dom';
import { useTheme } from 'next-themes';
import { Moon, Sun } from 'lucide-react';
import { cn } from '@/lib/cn';

/**
 * Light / dark switch: a compact pill with both a sun and a moon; the active mode sits in a filled
 * knob that slides across (transform only). Where the View Transition API exists the new theme is
 * dithered in with a pixel mask (see `lp-dither-in` in globals.css); elsewhere, and under reduced
 * motion, it switches at once.
 */
export function ThemeToggle({ className }: { className?: string }) {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  const dark = mounted && resolvedTheme === 'dark';

  const toggle = () => {
    const next = dark ? 'light' : 'dark';
    const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown };
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!doc.startViewTransition || reduce) return setTheme(next);
    doc.startViewTransition(() => flushSync(() => setTheme(next)));
  };

  return (
    <button
      type="button"
      role="switch"
      aria-checked={dark}
      onClick={toggle}
      aria-label="Dark theme"
      title={dark ? 'Switch to light theme' : 'Switch to dark theme'}
      className={cn(
        'relative inline-grid h-8 w-[3.75rem] shrink-0 grid-cols-2 items-center rounded-full border border-lp-line-strong p-[3px] text-lp-ink',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-orange',
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'absolute top-[3px] left-[3px] size-6 rounded-full bg-lp-ink transition-transform duration-200 ease-out motion-reduce:transition-none',
          dark && 'translate-x-7',
        )}
      />
      <span aria-hidden className={cn('relative z-10 grid size-6 place-items-center', !dark && 'text-lp-paper')}>
        <Sun className="size-3.5" />
      </span>
      <span aria-hidden className={cn('relative z-10 grid size-6 place-items-center', dark && 'text-lp-paper')}>
        <Moon className="size-3.5" />
      </span>
    </button>
  );
}
