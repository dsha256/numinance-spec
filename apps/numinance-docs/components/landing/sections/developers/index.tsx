import { SPEC_REPO_URL } from '@/lib/site-config';
import { Figure, FigureHeader, FigureLede, StatusPlate } from '@/components/landing/ui/figure';
import { Ruler } from '@/components/landing/ui/sheet';
import { Boundary } from './boundary';
import { Cli } from './cli';

export function Developers() {
  return (
    <Figure id="developers" chapter="OPEN SPEC" status="open spec · cli" aria-label="Open specification and CLI" className="px-(--lp-gutter) pt-(--lp-section)">
      <Boundary />
      <div className="mt-16 border-t border-lp-line pt-12 md:mt-24 md:pt-16">
        <FigureHeader lead={<StatusPlate>open CLI</StatusPlate>} title={['Build the ', { term: 'integration' }, '.\nNot another workaround.']} />
        <FigureLede className="mt-6">
          Typed protocols. Explicit schemas. SDKs. Conformance tooling. An open CLI for configuration validation and offline evidence verification. The boundaries are defined so developers can build against them.
        </FigureLede>
        <p className="mt-4 font-pixel text-[15px] leading-5 text-lp-ink">The engine is private. The integration surface is yours to explore.</p>
        <a
          href={SPEC_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-6 inline-flex min-h-12 items-center justify-center border border-lp-ink bg-lp-yellow px-5 font-pixel text-[15px] leading-4 text-lp-on-yellow outline-none focus-visible:ring-2 focus-visible:ring-lp-ink focus-visible:ring-offset-2 focus-visible:ring-offset-lp-paper max-sm:w-full"
        >
          Explore numinance-spec <span aria-hidden className="ms-2">↗</span>
        </a>
      </div>
      <div className="mt-10 md:mt-14">
        <Cli />
      </div>
      <div className="mt-(--lp-section)">
        <Ruler />
      </div>
    </Figure>
  );
}
