import Link from 'next/link';
import { Logo } from '@/components/brand/logo';
import { siteConfig } from '@/lib/site-config';
import { Wordmark } from '@/components/landing/wordmark';

type FooterLink = { label: string; href: string; external?: boolean };

const specRepo = siteConfig.repo.url;
const columns: { title: string; links: FooterLink[] }[] = [
  {
    title: 'Product',
    links: [
      { label: 'System', href: '/#system' },
      { label: 'Architecture', href: '/architecture' },
      { label: 'Trust & Evidence', href: '/evidence' },
      { label: 'Integrations', href: '/integrations' },
    ],
  },
  {
    title: 'Developers',
    links: [
      { label: 'Documentation', href: '/docs' },
      { label: 'Protocol', href: '/protocol' },
      { label: 'SDKs', href: '/docs/sdk' },
      { label: 'CLI', href: '/docs/cli' },
      { label: 'Conformance', href: '/docs/integrations/conformance' },
    ],
  },
  {
    title: 'Project',
    links: [
      { label: 'Status & changelog', href: '/changelog' },
      { label: 'Governance', href: '/docs/project/governance' },
      { label: 'Versioning', href: '/docs/project/versioning' },
      { label: 'RFCs', href: '/docs/project/rfcs' },
      { label: 'License', href: '/docs/project/license' },
    ],
  },
  { title: 'Follow', links: specRepo ? [{ label: 'GitHub', href: specRepo, external: true }] : [] },
];

const link = 'text-sm text-lp-ink-2 transition-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper';

export function SiteFooter() {
  return (
    <footer className="border-t border-lp-line-strong pb-(--lp-status-h)">
      <div className="mx-auto max-w-(--lp-max) px-(--lp-gutter) pb-16 sheet:border-x sheet:border-lp-line">
        <Wordmark />
        <div className="h-12" />
        <div className="grid gap-12 md:grid-cols-[1.3fr_repeat(4,1fr)]">
          <div className="max-w-xs">
            <Logo className="h-6" />
            <p className="mt-5 text-sm leading-relaxed text-lp-ink-2">{siteConfig.tagline}</p>
            <p className="mt-3 font-pixel text-[13px] text-lp-ink-2">IN DEVELOPMENT</p>
          </div>
          {columns.map((c) => (
            <nav key={c.title} aria-label={c.title}>
              <h3 className="mb-4 font-pixel text-[13px] uppercase text-lp-ink-2">{c.title}</h3>
              <ul className="space-y-2.5">
                {c.links.map((l) => (
                  <li key={l.label}>
                    {l.external ? (
                      <a href={l.href} target="_blank" rel="noreferrer noopener" className={link}>{l.label}</a>
                    ) : (
                      <Link href={l.href} className={link}>{l.label}</Link>
                    )}
                  </li>
                ))}
                {c.links.length === 0 && <li className="text-sm text-lp-ink-2">Public repository: pre-publication.</li>}
              </ul>
            </nav>
          ))}
        </div>
        <div aria-hidden className="mt-16 h-2.5 bg-[repeating-linear-gradient(to_right,var(--lp-line-strong)_0_1px,transparent_1px_24px)]" />
        <div className="mt-4 flex flex-col gap-3 text-xs text-lp-ink-2 md:flex-row md:justify-between">
          <p>© {new Date().getFullYear()} Numinance. All rights reserved.</p>
          <p className="max-w-xl md:text-right">
            The Apache-2.0 license applies to the <span className="font-mono">numinance-spec</span> repository only — not to the proprietary Numinance engine.
          </p>
        </div>
      </div>
    </footer>
  );
}
