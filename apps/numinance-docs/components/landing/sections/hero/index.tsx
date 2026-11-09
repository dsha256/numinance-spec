import { ArrowDown } from 'lucide-react';
import { SPEC_REPO_URL } from '@/lib/site-config';
import { Figure, StatusPlate } from '@/components/landing/ui/figure';
import { Ruler } from '@/components/landing/ui/sheet';
import { Glitch } from './glitch';
import { Reveal } from './reveal';
import { MarsStage } from './mars/stage';

const invert =
  'transition-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper focus-visible:outline-none';
const cta = 'inline-flex h-11 items-center justify-center gap-2 border border-lp-line-strong px-5 font-medium max-sm:w-full';

/** One screen below the menu: the claim on the left, the engineering core on the right. */
export function Hero() {
  return (
    <Figure
      id="numinance"
      chapter="NUMINANCE"
      status="ready"
      aria-label="Numinance"
      className="grid min-h-[calc(100svh-var(--lp-menu-h))] grid-rows-[auto_auto_auto] gap-x-12 px-(--lp-gutter) @container lg:grid-cols-2 lg:grid-rows-[1fr_auto]"
    >
      <div className="py-10 lg:self-center lg:py-8">
        <Reveal delay={0.3}>
          <div className="max-[419px]:[&>span]:text-[11px]! max-[419px]:[&>span]:leading-6!">
            <StatusPlate>NUMINANCE / AUTONOMOUS ENGINEERING</StatusPlate>
          </div>
        </Reveal>
        <Reveal delay={0.4} className="mt-6">
          <h1 className="text-[min(4.75rem,100cqw/6.4)] leading-[1.02] font-light text-nowrap tracking-[-0.04em] lg:text-[min(7.2vw,(100cqw-3rem)/12.7,6.5rem)]">
            Give it context.
            <br />
            Let it{' '}
            <span className="font-medium text-lp-orange">
              <Glitch>engineer</Glitch>.
            </span>
          </h1>
        </Reveal>
        <Reveal delay={0.5}>
          <p className="mt-6 max-w-[34rem] text-base leading-relaxed text-lp-ink-2 md:text-lg">
            Numinance is building an autonomous AI software engineer that understands your codebase before it acts, then
            takes engineering work as far as you authorize&mdash;from proposing changes to production operations. Policy
            sets the limits. Evidence lets you check the work.
          </p>
        </Reveal>
        <Reveal delay={0.6} className="mt-8 flex flex-wrap items-center gap-3">
          <a
            href={SPEC_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={`${cta} bg-lp-yellow text-[#121210] ${invert}`}
          >
            Explore the open spec
          </a>
          <a href="#workflow" className={`${cta} text-lp-ink ${invert}`}>
            See how it works
            <ArrowDown aria-hidden className="size-4" />
          </a>
        </Reveal>
        <Reveal delay={0.7} className="mt-8 space-y-2">
          <p className="text-sm text-lp-ink-2">Not another coding assistant. Engineering you can delegate.</p>
          <p className="font-pixel text-[13px] leading-4 tracking-wide text-lp-ink">
            PRE-ALPHA / <span className="text-lp-orange">ENGINE IN DEVELOPMENT</span>
          </p>
        </Reveal>
      </div>
      <MarsStage />
      <Ruler />
    </Figure>
  );
}
