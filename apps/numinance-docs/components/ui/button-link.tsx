import Link from 'next/link';
import { cn } from '@/lib/cn';
import { ArrowUpRight } from 'lucide-react';

const base =
  'group inline-flex min-h-11 items-center justify-center gap-2 rounded-full px-5 text-sm font-medium transition-[background,border-color,color,transform] duration-200 active:scale-[0.98]';

const variants = {
  primary: 'bg-nm-fg text-nm-bg hover:opacity-90',
  secondary: 'border border-nm-line-strong text-nm-fg hover:bg-nm-surface',
  ghost: 'text-nm-muted hover:text-nm-fg px-3',
} as const;

export function ButtonLink({
  href,
  variant = 'primary',
  external,
  className,
  children,
}: {
  href: string;
  variant?: keyof typeof variants;
  external?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  const cls = cn(base, variants[variant], className);
  if (external) {
    return (
      <a href={href} target="_blank" rel="noreferrer noopener" className={cls}>
        {children}
        <ArrowUpRight className="size-4 opacity-60 transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5" aria-hidden />
      </a>
    );
  }
  return (
    <Link href={href} className={cls}>
      {children}
    </Link>
  );
}
