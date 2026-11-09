import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import { Section } from '@/components/ui/section';
import { Reveal } from '@/components/motion/reveal';
import { siteConfig } from '@/lib/site-config';
import { ProtoPackages, ProtocolSplit, SdkShowcase } from './protocol-split';

const LINKS = [
  { href: '/docs/protocol', label: 'Protocol docs' },
  { href: '/docs/sdk', label: 'SDK guide' },
  { href: '/docs/project/governance', label: 'Governance' },
  { href: '/docs/project/versioning', label: 'Versioning' },
] as const;

export function ProtocolLinks() {
  const cls =
    'group inline-flex min-h-11 items-center gap-1.5 rounded-full border border-nm-line-strong px-4 text-sm font-medium transition-colors hover:bg-nm-surface';
  return (
    <ul className="flex flex-wrap gap-3">
      {LINKS.map((l) => (
        <li key={l.href}>
          <Link href={l.href} className={cls}>
            {l.label}
          </Link>
        </li>
      ))}
      <li>
        {siteConfig.repo.public ? (
          <a href={siteConfig.repo.url ?? undefined} target="_blank" rel="noreferrer noopener" className={cls}>
            numinance-spec repository
            <ArrowUpRight className="size-4 opacity-60" aria-hidden />
          </a>
        ) : (
          <span className="inline-flex min-h-11 items-center rounded-full border border-dashed border-nm-line-strong px-4 text-sm text-nm-muted">
            numinance-spec repository · Pre-publication
          </span>
        )}
      </li>
    </ul>
  );
}

export function ProtocolSection() {
  return (
    <Section
      id="protocol"
      eyebrow="THE OPEN BOUNDARY"
      title={
        <>
          The intelligence is private. <br className="hidden md:block" />
          <em>The contract is open.</em>
        </>
      }
      lede={
        <>
          <code className="font-mono text-[0.9em] text-nm-fg">numinance-spec</code> is the Apache-2.0 integration and verification layer around Numinance. Build
          compatible adapters, inspect the contracts, generate clients, validate configuration and verify evidence without depending on the proprietary
          engine&rsquo;s internals.
        </>
      }
    >
      <div className="space-y-6">
        <Reveal>
          <ProtocolSplit />
        </Reveal>
        <Reveal>
          <ProtoPackages />
        </Reveal>
        <Reveal>
          <SdkShowcase />
        </Reveal>
        <Reveal>
          <ProtocolLinks />
        </Reveal>
      </div>
    </Section>
  );
}
