import Link from 'next/link';
import { Chip } from '@/components/landing/ui/chip';
import { Terms } from '@/components/landing/ui/figure';
import { cn } from '@/lib/cn';
import { SPEC_REPO_URL } from '@/lib/site-config';

const PX = 'font-pixel text-[13px] leading-4 antialiased';

const AREAS = ['proto/', 'schemas/', 'sdk/go', 'sdk/python', 'sdk/typescript', 'cli/', 'conformance/', 'plugins/', 'rfcs/', 'docs/evidence/'] as const;

const SPEC_LINKS = [
  { href: '/docs/protocol', label: 'Protocol' },
  { href: '/docs/sdk', label: 'SDKs' },
  { href: '/docs/cli', label: 'CLI' },
  { href: '/docs/project/governance', label: 'Governance' },
] as const;

/** Open-spec block: the private engine, the open contract, and the real public areas. */
export function Boundary() {
  return (
    <div>
      <div className="flex flex-col gap-5">
        <h3 className="max-w-[24ch] text-[clamp(1.75rem,1rem+2.4vw,3rem)] leading-[1.04] font-light tracking-[-0.03em] text-balance">
          <Terms runs={['The brain is private.\nThe contract is ', { term: 'open' }, '.']} />
        </h3>
        <p className="max-w-[56ch] text-base leading-relaxed text-lp-ink-2 md:text-lg">
          Numinance&apos;s intelligence engine remains proprietary. The public specification defines the protocols, configuration schemas, SDKs, integration contracts, conformance tooling, and independent evidence verification boundary.
        </p>
        <p className="font-pixel text-[15px] leading-5 text-lp-ink">Build against the contract. Not our internals.</p>
      </div>
      <div className="mt-8 grid grid-cols-1 border border-lp-line-strong sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]" role="group" aria-label="Private engine and open specification">
        <div className="relative flex min-h-16 flex-col justify-center gap-1 overflow-hidden px-3 py-3 md:px-5">
          <div aria-hidden className="lp-hazard absolute inset-x-0 bottom-0 h-1.5 opacity-60" />
          <p className={cn(PX, 'text-lp-ink-2')}>PRIVATE ENGINE</p>
          <p className="text-xs leading-4 text-lp-ink-2">Proprietary. Not in the repository.</p>
        </div>
        <div className="relative flex min-h-16 flex-col justify-center gap-2 border-t-2 border-dashed border-lp-ink bg-lp-paper-2 px-3 py-3 sm:border-s-2 sm:border-t-0 md:px-5">
          <span aria-hidden className="absolute -start-px top-1/2 size-3 -translate-x-1/2 -translate-y-1/2 border border-lp-ink bg-lp-yellow max-sm:hidden" />
          <p className={cn(PX, 'flex flex-wrap items-center gap-2')}>
            <span>NUMINANCE-SPEC</span> <span className="text-lp-ink-2">/</span> <span>APACHE-2.0</span> <span className="text-lp-ink-2">/</span> <span>PUBLIC</span>
          </p>
          <ul className="flex flex-wrap gap-1">
            {AREAS.map((a) => (
              <li key={a}>
                <Chip className="h-5 px-1.5 font-mono text-[12px]">{a}</Chip>
              </li>
            ))}
          </ul>
        </div>
      </div>
      <p className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
        <a
          href={SPEC_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="min-h-6 font-medium underline decoration-lp-orange decoration-2 underline-offset-4 hover:text-lp-orange focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-lp-orange"
        >
          numinance-spec on GitHub <span aria-hidden>↗</span>
        </a>
        {SPEC_LINKS.map((l) => (
          <Link
            key={l.href}
            href={l.href}
            className="min-h-6 font-medium underline decoration-lp-orange decoration-2 underline-offset-4 hover:text-lp-orange focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-lp-orange"
          >
            {l.label} <span aria-hidden>→</span>
          </Link>
        ))}
      </p>
    </div>
  );
}
