'use client';

import { useState } from 'react';
import { Check, Copy, FileText } from 'lucide-react';

/** Copy the page as Markdown + open the raw .md export. Last-updated comes from Git history at build time. */
export function PageActions({ url, markdownUrl, lastModified }: { url: string; markdownUrl: string; lastModified?: string }) {
  const [state, setState] = useState<'idle' | 'copied' | 'error'>('idle');
  const date = lastModified ? new Date(lastModified) : null;
  async function copy() {
    try {
      const res = await fetch(markdownUrl);
      if (!res.ok) throw new Error(String(res.status));
      await navigator.clipboard.writeText(await res.text());
      setState('copied');
    } catch {
      setState('error');
    }
    setTimeout(() => setState('idle'), 2000);
  }
  return (
    <div className="not-prose -mt-4 mb-6 flex flex-wrap items-center gap-2 text-xs" data-page-url={url}>
      <button
        type="button"
        onClick={copy}
        className="inline-flex min-h-8 items-center gap-1.5 rounded-sm border border-fd-border bg-fd-card px-2.5 py-1 font-mono text-fd-muted-foreground transition-colors hover:border-fd-foreground/50 hover:text-fd-foreground"
      >
        {state === 'copied' ? <Check className="size-3.5" aria-hidden /> : <Copy className="size-3.5" aria-hidden />}
        {state === 'copied' ? 'Copied Markdown' : state === 'error' ? 'Copy failed' : 'Copy page as Markdown'}
      </button>
      <a
        href={markdownUrl}
        className="inline-flex min-h-8 items-center gap-1.5 rounded-sm border border-fd-border bg-fd-card px-2.5 py-1 font-mono text-fd-muted-foreground transition-colors hover:border-fd-foreground/50 hover:text-fd-foreground"
      >
        <FileText className="size-3.5" aria-hidden /> View as .md
      </a>
      <span aria-live="polite" className="sr-only">{state === 'copied' ? 'Page Markdown copied to clipboard' : ''}</span>
      {date && !Number.isNaN(date.getTime()) ? (
        <span className="ml-auto font-mono text-fd-muted-foreground">
          Updated <time dateTime={date.toISOString()}>{date.toISOString().slice(0, 10)}</time>
        </span>
      ) : null}
    </div>
  );
}
