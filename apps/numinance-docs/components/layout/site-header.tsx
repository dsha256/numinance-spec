'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { Menu, X } from 'lucide-react';
import { Logo } from '@/components/brand/logo';
import { GithubMark } from '@/components/brand/github-mark';
import { ThemeToggle } from './theme-toggle';
import { nav, siteConfig } from '@/lib/site-config';
import { cn } from '@/lib/cn';
import { EASE } from '@/lib/motion';

const invert = 'transition-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper focus-visible:outline-none';

export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const esc = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    window.addEventListener('keydown', esc);
    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener('keydown', esc);
    };
  }, [open]);

  const isActive = (href: string) => !href.includes('#') && (pathname === href || pathname.startsWith(href + '/'));
  const repoUrl = siteConfig.repo.url;

  return (
    <>
      <header className="sticky top-0 z-50 h-(--lp-menu-h) border-b border-lp-line-strong bg-lp-paper/95 backdrop-blur-sm">
        <div className="mx-auto flex h-full max-w-(--lp-max) items-center justify-between px-(--lp-gutter)">
          <Link href="/" aria-label="Numinance home" className="flex h-6 items-center">
            <Logo className="h-full" priority />
          </Link>

          <nav aria-label="Primary" className="hidden items-center lg:flex">
            {nav.map((item) => (
              <Link
                key={item.label}
                href={item.href}
                aria-current={isActive(item.href) ? 'page' : undefined}
                className={cn(
                  'relative px-3.5 py-2 text-sm',
                  invert,
                  isActive(item.href) && 'after:absolute after:inset-x-3.5 after:bottom-0 after:h-0.5 after:bg-lp-orange',
                )}
              >
                {item.label}
              </Link>
            ))}
          </nav>

          <div className="flex items-center gap-3">
            <div className="hidden lg:block">
              <ThemeToggle />
            </div>
            {repoUrl && (
              <a
                href={repoUrl}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Numinance specification on GitHub"
                title="Numinance specification on GitHub"
                className="hidden size-9 items-center justify-center text-lp-ink hover:text-lp-orange focus-visible:text-lp-orange focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-orange lg:inline-flex"
              >
                <GithubMark size={24} />
              </a>
            )}
            <Link
              href="/protocol"
              className={cn('hidden h-9 items-center whitespace-nowrap border border-lp-line-strong bg-lp-yellow px-4 text-sm font-medium text-lp-on-yellow lg:inline-flex', invert)}
            >
              Explore the open spec
            </Link>
            <button
              type="button"
              className="inline-flex size-11 items-center justify-center lg:hidden"
              aria-label={open ? 'Close menu' : 'Open menu'}
              aria-expanded={open}
              aria-controls="mobile-menu"
              onClick={() => setOpen((v) => !v)}
            >
              {open ? <X className="size-5" aria-hidden /> : <Menu className="size-5" aria-hidden />}
            </button>
          </div>
        </div>
      </header>

      <AnimatePresence>
        {open && (
          <motion.div
            id="mobile-menu"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.18, ease: EASE }}
            className="fixed inset-0 z-40 overflow-y-auto bg-lp-paper pt-(--lp-menu-h) lg:hidden"
          >
            <nav aria-label="Mobile" className="px-(--lp-gutter) py-4">
              {nav.map((item, i) => (
                <Link
                  key={item.label}
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className="flex min-h-16 items-center justify-between border-b border-lp-line text-3xl font-light tracking-tight"
                >
                  <span><span className="me-3 font-pixel text-sm text-lp-ink-2">0{i + 1}</span>{item.label}</span>
                  <span aria-hidden className="text-lp-ink-2">→</span>
                </Link>
              ))}
              {repoUrl && (
                <a
                  href={repoUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex min-h-16 items-center justify-between border-b border-lp-line text-3xl font-light tracking-tight"
                >
                  <span className="flex items-center gap-3">
                    <GithubMark size={26} className="text-lp-ink" />
                    GitHub
                  </span>
                  <span aria-hidden className="text-lp-ink-2">↗</span>
                </a>
              )}
              <div className="mt-8 flex items-center justify-between">
                <span className="font-pixel text-sm text-lp-ink-2">THEME</span>
                <ThemeToggle />
              </div>
              <p className="mt-10 font-pixel text-sm text-lp-ink-2">NUMINANCE / IN DEVELOPMENT</p>
            </nav>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
