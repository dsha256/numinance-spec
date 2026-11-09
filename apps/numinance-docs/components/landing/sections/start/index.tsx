import Link from 'next/link';
import { Figure, FigureHeader, FigureLede } from '@/components/landing/ui/figure';
import { Command } from '@/components/landing/ui/command';
import { Ruler } from '@/components/landing/ui/sheet';
import { Shadow } from '@/components/landing/ui/window';
import { SPEC_REPO_URL } from '@/lib/site-config';
import { Stage } from './stage';

const base =
  'relative flex min-h-12 items-center justify-center border px-5 font-pixel text-[15px] leading-4 outline-none focus-visible:ring-2 focus-visible:ring-lp-ink focus-visible:ring-offset-2 focus-visible:ring-offset-lp-paper max-sm:w-full';

/** Closing figure: a pinned CRT-and-landscape finale (see ./stage.tsx for the scroll-driven camera). */
export function Start() {
  return (
    <Figure id="start" chapter="START" status="start" aria-label="Start">
      <Stage>
        <FigureHeader className="max-w-none lg:text-[clamp(1.75rem,0.9rem+1.7vw,2.9rem)]" title={['Autonomy is the goal.\n', { term: 'Accountability' }, ' is the rule.']} />
        <FigureLede>
          We&apos;re building beyond prompts toward engineering you can delegate. Explore the public protocol, understand the boundaries, and see how independently verifiable autonomy is taking shape.
        </FigureLede>
        <Command command="numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR" className="bg-lp-paper" />
        <div className="flex flex-wrap gap-x-3 gap-y-4">
          <span className="relative isolate max-sm:w-full">
            <Shadow />
            <a
              href={SPEC_REPO_URL}
              target="_blank"
              rel="noopener noreferrer"
              className={`${base} border-lp-ink bg-lp-yellow text-lp-on-yellow`}
            >
              Explore the specification
            </a>
          </span>
          <Link href="/architecture" className={`${base} border-lp-line-strong bg-lp-paper hover:bg-lp-ink hover:text-lp-paper`}>
            Explore the architecture
          </Link>
          <a href={SPEC_REPO_URL} target="_blank" rel="noopener noreferrer" className={`${base} border-lp-line-strong bg-lp-paper hover:bg-lp-ink hover:text-lp-paper`}>
            View on GitHub ↗
          </a>
        </div>
      </Stage>
      <div className="px-(--lp-gutter)">
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-lp-line py-4 font-pixel text-[13px] leading-5 text-lp-ink-2">
          <span>NUMINANCE / BUILDING AUTONOMOUS ENGINEERING</span>
          <span aria-hidden className="hidden h-px flex-1 bg-lp-line sm:block" />
          <span>END OF SHEET SET</span>
        </p>
        <Ruler />
      </div>
    </Figure>
  );
}
