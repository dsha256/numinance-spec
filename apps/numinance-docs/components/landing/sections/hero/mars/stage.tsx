'use client';
import { useState, type CSSProperties } from 'react';
import { motion, useScroll, useTransform } from 'motion/react';
import { DITHER_MASKS } from '@/lib/landing/dither';
import { useScene } from '@/lib/landing/use-scene';
import { STAGE_ICONS } from '@/components/landing/ui/chip';
import { cn } from '@/lib/cn';
import { STAGES } from '../engine';
import { Dock } from './dock';
import { createMarsHero } from './engine';
import { createSnapshot, createStore } from './machine';

const shadow: CSSProperties = { maskImage: DITHER_MASKS[6], maskSize: '4px 4px' };
const tail: CSSProperties = { left: 'calc(var(--tail, 20px) - 6px)', clipPath: 'polygon(0 0, 100% 0, 50% 100%)' };
const dashed = 'bg-[repeating-linear-gradient(to_bottom,var(--lp-line-strong)_0_3px,transparent_3px_7px)]';

const chipBase =
  'absolute top-0 left-0 z-[5] inline-flex h-6 items-center gap-1.5 border border-lp-line-strong bg-lp-paper px-2 font-pixel text-[13px] leading-4 whitespace-nowrap antialiased will-change-transform transition-opacity duration-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-orange data-[state=off]:opacity-0 data-[active=true]:data-[hue=indigo]:border-lp-indigo data-[active=true]:data-[hue=indigo]:bg-lp-indigo data-[active=true]:data-[hue=indigo]:text-white data-[active=true]:data-[hue=yellow]:border-lp-ink data-[active=true]:data-[hue=yellow]:bg-lp-yellow data-[active=true]:data-[hue=yellow]:text-lp-on-yellow';

const EXTRA = [
  { id: 'review', label: 'REVIEW', name: 'Independent review identity (read-only)' },
  { id: 'release', label: 'RELEASE', name: 'Release pipeline identity' },
] as const;

/**
 * The hero stage: the halftone core on one canvas and one coordinate frame, the DOM chips and bubble, plus a
 * compact control dock. The first time the stage is visible it plays the whole simulated story by itself (preset
 * policy-permitted scenario, never a faked click), then settles into the quiet interactive autonomous mode; the
 * dock's stage rail browses each stage afterwards. Reduced motion lands on the settled state. Everything shown is
 * illustrative and pre-alpha.
 */
export function MarsStage() {
  const [store] = useState(() => createStore(createSnapshot('DORMANT', 'approval', true)));
  const [failed, setFailed] = useState(false);
  const ref = useScene<HTMLDivElement>((el, ctx) =>
    createMarsHero(el, { ...ctx, store, settled: false, onFail: () => setFailed(true) }),
  );
  const { scrollYProgress } = useScroll({ target: ref, offset: ['start start', 'end start'] });
  const y = useTransform(scrollYProgress, [0, 1], [0, 70]);
  const scale = useTransform(scrollYProgress, [0, 1], [1, 0.95]);
  const opacity = useTransform(scrollYProgress, [0, 0.8], [1, 0.15]);
  const extend = useTransform(scrollYProgress, [0.05, 0.7], [0, 1]);

  return (
    <div
      ref={ref}
      className="relative isolate -mx-(--lp-gutter) aspect-[5/8] max-lg:max-h-[700px] lg:mx-0 lg:-me-(--lp-gutter) lg:aspect-auto lg:min-h-[560px]"
    >
      <motion.div style={{ y, scale, opacity }} className="absolute inset-0 origin-[50%_55%]">
        <canvas aria-hidden className={cn('font-pixel absolute inset-0 size-full', failed && 'hidden')} />
        {failed && (
          <div aria-hidden className="absolute inset-0 grid place-items-center">
            <div className="relative aspect-square w-[62%] max-w-[360px] rounded-full border border-lp-line-strong bg-lp-paper-2">
              <div className="absolute inset-x-[-18%] top-1/2 h-[26%] -translate-y-1/2 rounded-[50%] border border-dashed border-lp-line-strong" />
              <p className="absolute inset-x-0 top-1/2 -translate-y-1/2 text-center font-pixel text-[12px] text-lp-ink-2">canvas unavailable</p>
            </div>
          </div>
        )}
        {STAGES.map((s, i) => (
          <button
            key={s.id}
            type="button"
            data-node={i}
            data-state="off"
            tabIndex={-1}
            aria-label={`${s.label} waypoint receipt`}
            className={chipBase}
            style={{ pointerEvents: 'none' }}
          >
            <svg aria-hidden viewBox="0 0 8 8" shapeRendering="crispEdges" className="size-2.5 shrink-0 fill-current">
              <path d={STAGE_ICONS[s.id]} />
            </svg>
            <span data-l className="data-[x=1]:hidden [[data-compact=true]:not([data-active=true])_&]:hidden">
              {s.label}
            </span>
          </button>
        ))}
        {EXTRA.map((e, k) => (
          <button
            key={e.id}
            type="button"
            data-node={5 + k}
            data-state="off"
            tabIndex={-1}
            aria-label={e.name}
            className={chipBase}
            style={{ pointerEvents: 'none' }}
          >
            <span data-l className="[[data-compact=true]:not([data-active=true])_&]:hidden">
              {e.label}
            </span>
          </button>
        ))}
        <div
          data-tip
          data-state="off"
          aria-hidden
          className="pointer-events-none absolute top-0 left-0 z-[6] w-max max-w-[24rem] border border-lp-line-strong bg-lp-paper px-2 py-1 font-pixel text-[12px] leading-4 text-lp-ink antialiased will-change-transform data-[state=off]:invisible"
        />
        <div
          data-bubble
          data-state="off"
          className="pointer-events-none absolute top-0 left-0 isolate w-max pr-0.75 pb-2.5 font-pixel text-[13px] leading-4 antialiased data-[state=off]:invisible"
        >
          <span aria-hidden style={shadow} className="absolute top-0.75 right-0 bottom-1.75 left-0.75 bg-lp-ink-2" />
          <span className="relative grid justify-items-start bg-lp-ink px-2.5 py-2 whitespace-pre text-lp-paper *:col-start-1 *:row-start-1">
            <span data-sizer className="invisible" />
            <span data-said />
          </span>
          <span aria-hidden style={tail} className="absolute bottom-0 h-2.5 w-3 bg-lp-ink" />
        </div>
        <Dock store={store} />
      </motion.div>
      <p className="sr-only">
        Illustration: a halftone engineering core that plays a simulated Mars-like autonomous run by itself, with a stage rail to browse each step.
        Authorized sources are retrieved, the context bundle is validated, an external policy decides what the agent may do (in the
        autoplay a preset policy already permits this qualified scope, so no per-change approval is shown), and
        only then does a short autonomous cycle (understand, plan, act, verify, evidence) run. Everything shown is illustrative
        and pre-alpha, not a product capture and not real authorization.
      </p>
      {[0.27, 0.74].map((x) => (
        <motion.span
          key={x}
          aria-hidden
          style={{ scaleY: extend, left: `${x * 100}%` }}
          className={`pointer-events-none absolute top-full z-0 h-24 w-px origin-top ${dashed}`}
        />
      ))}
    </div>
  );
}
