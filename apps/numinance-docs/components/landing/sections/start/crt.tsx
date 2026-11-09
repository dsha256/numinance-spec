import type { Ref } from 'react';
import { DITHER_MASKS } from '@/lib/landing/dither';
import { OperatorSprite } from '@/components/landing/machine/operator-sprite';
import type { Line } from './script';

const CREAM = '#e7dcc0';
const CREAM_SHADE = '#cdbf9c';
const EDGE = '#1c1a15';

const ink: Record<Line['kind'], string> = {
  dim: 'text-[#8f8a78]',
  boot: 'text-[#9ec8a0]',
  prompt: 'text-[#f4cf57]',
  out: 'text-[#e9e3cf]',
  note: 'text-[#f08a3c]',
  blank: '',
};

function Screw({ className }: { className: string }) {
  return (
    <span aria-hidden className={`absolute size-[1.4%] rounded-full border border-[#1c1a15]/70 bg-[#bfb08a] ${className}`}>
      <span className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 rotate-[35deg] bg-[#1c1a15]/70" />
    </span>
  );
}

/**
 * The CRT computer. The box is a plain HTML/CSS object (thick cream casing, recessed glass, base unit with
 * floppy slot and vents) so the 80x24 terminal on its glass is real, selectable-by-AT text.
 * `screenRef` is the glass (the camera aims at it); `termRef` receives the typed lines.
 */
export function Crt({
  lines,
  typing,
  screenRef,
  camRef,
}: {
  lines: { line: Line; text: string }[];
  typing: boolean;
  screenRef: Ref<HTMLDivElement>;
  camRef: Ref<HTMLDivElement>;
}) {
  return (
    <div ref={camRef} className="absolute inset-0 will-change-transform">
      {/* dithered cast shadow, same silhouette as the machine */}
      <div aria-hidden className="absolute inset-0 translate-x-[2.2%] translate-y-[1.6%]" style={{ maskImage: DITHER_MASKS[7], maskSize: '4px 4px' }}>
        <div className="absolute inset-x-[4%] top-0 h-[74%] rounded-[5%/6%] bg-lp-ink-2" />
        <div className="absolute inset-x-0 top-[79%] h-[19%] rounded-[2%] bg-lp-ink-2" />
      </div>

      {/* neck */}
      <div aria-hidden className="absolute inset-x-[22%] top-[72%] h-[9%] border-2" style={{ background: CREAM_SHADE, borderColor: EDGE }} />

      {/* base unit */}
      <div
        aria-hidden
        className="absolute inset-x-0 top-[79%] h-[19%] rounded-[2%] border-2"
        style={{ background: CREAM, borderColor: EDGE, boxShadow: 'inset 0 4px 0 rgba(255,255,255,.4), inset 0 -6px 0 rgba(0,0,0,.1)' }}
      >
        {/* floppy slot */}
        <span className="absolute top-[24%] left-[7%] block h-[16%] w-[34%] border-2" style={{ background: '#17150f', borderColor: EDGE }} />
        <span className="absolute top-[58%] left-[7%] block h-[6%] w-[34%]" style={{ background: CREAM_SHADE }} />
        <span className="absolute top-[30%] left-[43%] size-[2.2%] min-w-1.5 bg-[#e0442b]" />
        {/* vents */}
        <span
          className="absolute top-[22%] right-[7%] block h-[56%] w-[30%] opacity-80"
          style={{ background: `repeating-linear-gradient(to bottom, ${EDGE} 0 2px, transparent 2px 7px)` }}
        />
        <Screw className="top-[12%] left-[2%]" />
        <Screw className="top-[12%] right-[2%]" />
      </div>

      {/* monitor shell */}
      <div
        className="absolute inset-x-[4%] top-0 h-[74%] rounded-[5%/6%] border-2"
        style={{ background: CREAM, borderColor: EDGE, boxShadow: 'inset 8px 8px 0 rgba(255,255,255,.35), inset -12px -14px 0 rgba(0,0,0,.09)' }}
      >
        <Screw className="top-[3%] left-[2.2%]" />
        <Screw className="top-[3%] right-[2.2%]" />

        {/* recessed bezel */}
        <div
          className="absolute inset-x-[6%] top-[6%] bottom-[16%] rounded-[4%/5%] border-2"
          style={{ background: CREAM_SHADE, borderColor: EDGE, boxShadow: 'inset 6px 7px 0 rgba(0,0,0,.18), inset -3px -3px 0 rgba(255,255,255,.3)' }}
        >
          {/* glass */}
          <div
            ref={screenRef}
            className="absolute inset-[4.5%] overflow-hidden rounded-[5%/6.5%] border-2 bg-[#070a08] [container-type:size]"
            style={{ borderColor: EDGE, boxShadow: 'inset 0 0 40px rgba(0,0,0,.9)' }}
          >
            <div data-raster className="absolute inset-0 origin-center" style={{ transform: 'scaleY(var(--on, 1))', opacity: 'var(--on-o, 1)' }}>
              <div role="img" aria-label={SR} className="absolute inset-0 flex flex-col justify-end overflow-hidden px-[3.2%] py-[3%] font-mono text-[11px] leading-[15px]">
                <div aria-hidden className="min-w-0">
                  {lines.map(({ line, text }, i) => (
                    <div key={i} className={`min-h-[15px] [overflow-wrap:anywhere] whitespace-pre-wrap ${ink[line.kind]}`}>
                      {text}
                      {i === lines.length - 1 && line.kind === 'prompt' && typing ? <Cursor /> : null}
                    </div>
                  ))}
                  {!typing && lines.length > 0 ? (
                    <div className="min-h-[15px] text-[#f4cf57]">
                      $ <Cursor />
                    </div>
                  ) : null}
                </div>
              </div>
              {/* phosphor scanlines + curvature vignette + glass glint */}
              <div
                aria-hidden
                className="pointer-events-none absolute inset-0"
                style={{
                  background:
                    'repeating-linear-gradient(to bottom, rgba(0,0,0,.28) 0 1px, transparent 1px 3px), radial-gradient(ellipse at 50% 50%, transparent 55%, rgba(0,0,0,.55) 100%)',
                }}
              />
              <div aria-hidden className="pointer-events-none absolute top-[4%] left-[5%] h-[14%] w-[30%] -skew-x-12 bg-white/[0.045]" />
            </div>
          </div>
        </div>

        {/* chin: maker's plate, lamp, switch */}
        <div className="absolute inset-x-[6%] bottom-[3.5%] flex h-[9%] items-center justify-between">
          <span className="border px-[2%] py-[0.5%] font-pixel text-[11px] leading-4 tracking-wide" style={{ color: EDGE, borderColor: EDGE }}>
            NUMINANCE · N-1
          </span>
          <span className="flex items-center gap-[10px]">
            <span aria-hidden className="size-2 rounded-full border" style={{ borderColor: EDGE, background: 'var(--lamp, #e0442b)', boxShadow: '0 0 0 2px rgba(0,0,0,.08)' }} />
            <span aria-hidden className="block h-[14px] w-[26px] border-2" style={{ borderColor: EDGE, background: CREAM_SHADE, boxShadow: 'inset 0 -5px 0 rgba(0,0,0,.2)' }} />
          </span>
        </div>
      </div>

      {/* the operator keeps the machine company */}
      <div aria-hidden className="absolute right-[3%] top-[79%] -translate-y-full max-lg:hidden">
        <OperatorSprite pose="idle" cell={4} flip />
      </div>
    </div>
  );
}

function Cursor() {
  return <span aria-hidden className="ms-px inline-block h-[1.05em] w-[0.6em] translate-y-[2px] animate-pulse bg-[#f4cf57] motion-reduce:animate-none" />;
}

const SR =
  'Illustrative terminal session on a retro computer, not captured from a run: numi evidence verify with a runner key, a log key and a log origin and placeholder output, followed by two commands from cli/README.md.';
