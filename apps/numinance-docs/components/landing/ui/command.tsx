'use client';

import { useState, type ComponentProps } from 'react';
import { Check, Clipboard } from 'lucide-react';
import { cn } from '@/lib/cn';
import { setStatus } from '@/lib/landing/status';

/** A shell command with a copy button — only use for commands that are real and worth copying. */
export function Command({ command, className, ...props }: ComponentProps<'div'> & { command: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div {...props} className={cn('flex min-h-11 w-fit max-w-full items-stretch border border-lp-line-strong bg-lp-paper', className)}>
      <code className="flex min-w-0 flex-1 items-baseline gap-2 self-center px-3 py-2.5 font-mono text-sm leading-5">
        <span aria-hidden className="text-lp-ink-2 select-none">$</span>
        <span className="min-w-0 break-words">{command}</span>
      </code>
      <button
        type="button"
        aria-label={copied ? 'Copied' : `Copy "${command}"`}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(command);
            setCopied(true);
            setStatus(`copied: ${command}`);
            setTimeout(() => setCopied(false), 1600);
          } catch {}
        }}
        className="flex w-11 shrink-0 items-center justify-center border-s border-lp-line-strong transition-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper focus-visible:outline-none"
      >
        {copied ? <Check className="size-4" /> : <Clipboard className="size-4" />}
      </button>
    </div>
  );
}
