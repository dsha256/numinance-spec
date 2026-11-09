'use client';

import { Lock } from 'lucide-react';
import { type CSSProperties, type KeyboardEvent, useCallback, useEffect, useRef, useState } from 'react';
import { cn } from '@/lib/cn';
import { scopedStatus } from '@/lib/landing/status';
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { Shadow, Window } from '@/components/landing/ui/window';
import { createArchitecture, type Engine } from './engine';
import { ICON } from './icons';
import { VIEWS } from './geometry';
import {
  DISCLOSURE,
  defaultSelection,
  MODULES,
  type ModuleId,
  ORDER,
  type Pick,
  readoutOf,
  type Selection,
} from './model';

const setStatus = scopedStatus('architecture');

/** the figure's height as a share of its width, wide and phone */
function height(view: { left: number; right: number; top: number; bottom: number }): string {
  return `${(((view.top - view.bottom) / (view.right - view.left)) * 100).toFixed(4)}%`;
}
const heights = { '--wide': height(VIEWS.wide), '--phone': height(VIEWS.phone) } as CSSProperties;

const FIXED = new Set<ModuleId>(ORDER.filter((id) => !MODULES[id].swap));

function Plates() {
  return ORDER.map((id) => {
    const fixed = FIXED.has(id);
    const m = MODULES[id];
    return (
      <div
        key={id}
        data-plate={id}
        role="button"
        tabIndex={0}
        aria-label={`${m.name}: ${m.plate!(defaultSelection()[id], 0)}`}
        style={{ transform: 'scale(0)' }}
        className={cn(
          'pointer-events-auto absolute top-0 left-0 flex h-(--arch-plate-h) w-(--arch-plate-w) origin-top-left cursor-pointer border font-pixel text-[13px] leading-4 antialiased select-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-orange',
          fixed
            ? 'border-lp-paper bg-lp-ink text-lp-paper dark:border-lp-ink dark:bg-lp-paper dark:text-lp-ink'
            : 'border-lp-ink bg-lp-paper text-lp-ink hover:bg-lp-yellow hover:text-lp-on-yellow dark:border-lp-paper dark:bg-lp-ink dark:text-lp-paper',
        )}
      >
        <span data-badge aria-hidden className={cn('aspect-square h-full shrink-0', fixed ? 'bg-lp-sun dark:bg-lp-orange' : 'bg-lp-ink dark:bg-lp-paper')}>
          <svg
            viewBox="0 0 8 8"
            shapeRendering="crispEdges"
            className={cn('m-(--arch-icon-at) size-(--arch-icon)', fixed ? 'fill-lp-ink dark:fill-lp-paper' : 'fill-lp-sun dark:fill-lp-orange')}
          >
            <path d={ICON[id]} fillRule="evenodd" />
          </svg>
        </span>
        <span className="flex min-w-0 flex-col ps-(--arch-pad) pt-(--arch-text-at)">
          <span data-role className="whitespace-nowrap opacity-70">
            {m.name}
          </span>
          <span data-name className="mt-(--arch-name-at) truncate pe-1 whitespace-nowrap" />
        </span>
      </div>
    );
  });
}

const DISPLAY =
  'pointer-events-auto absolute top-0 left-0 origin-top-left overflow-hidden font-pixel text-[13px] leading-4 whitespace-pre antialiased select-text';

function Displays() {
  return (
    <>
      <div data-display="engine" style={{ transform: 'scale(0)' }} className={cn(DISPLAY, 'px-1.5 pt-px text-lp-ink dark:text-lp-paper')}>
        <span data-title className="block w-fit bg-lp-sun px-1 empty:hidden" />
        {[0, 1, 2, 3, 4, 5].map((n) => (
          <span key={n} data-line className="block h-4" />
        ))}
      </div>
      <div data-display="kernel" style={{ transform: 'scale(0)' }} className={cn(DISPLAY, 'px-1 pt-px text-lp-paper dark:text-lp-ink')}>
        {MODULES.kernel.groups[0].options.map((o, n) => (
          <span key={o.title} data-line className="block px-0.5 data-hit:bg-lp-sun data-hit:text-lp-ink dark:data-hit:bg-lp-orange">
            {n + 1} {o.short.toUpperCase()}
          </span>
        ))}
      </div>
    </>
  );
}

/** a readout as plain text: the monitor where it is too small, and the figure's alternative */
function ReadoutText({ pick, className }: { pick: Pick; className?: string }) {
  const r = readoutOf(pick);
  return (
    <div className={cn('font-pixel text-[13px] leading-4 antialiased', className)}>
      <p className="w-fit bg-lp-sun px-1 text-lp-ink dark:text-lp-paper">{r.title}</p>
      {r.lines.map((line) => (
        <p key={line} className="whitespace-pre-wrap">
          {line}
        </p>
      ))}
    </div>
  );
}

function Chip({
  checked,
  fixed,
  onPick,
  children,
}: {
  checked: boolean;
  fixed: boolean;
  onPick: () => void;
  children: string;
}) {
  const onKey = (event: KeyboardEvent<HTMLButtonElement>) => {
    const step = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0;
    if (step === 0) return;
    event.preventDefault();
    const chips = [...(event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="radio"]') ?? [])];
    const at = chips.indexOf(event.currentTarget);
    const next = chips[(at + step + chips.length) % chips.length];
    next?.focus();
    next?.click();
  };
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      tabIndex={checked ? 0 : -1}
      onClick={onPick}
      onKeyDown={onKey}
      className="inline-flex cursor-pointer items-center gap-1 border border-lp-line-strong px-1.5 py-1 font-pixel text-[13px] leading-4 text-lp-ink antialiased focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lp-orange aria-checked:bg-lp-ink aria-checked:text-lp-paper not-aria-checked:hover:bg-lp-yellow not-aria-checked:hover:text-lp-on-yellow"
    >
      {fixed && <Lock aria-hidden className="size-2.5 shrink-0" strokeWidth={2.5} />}
      {children}
    </button>
  );
}

function Panel({
  sel,
  visible,
  onPick,
  className,
}: {
  sel: Selection;
  visible: boolean;
  onPick: (pick: Pick) => void;
  className?: string;
}) {
  return (
    <div data-window data-state={visible ? 'on' : 'off'} className={cn('relative isolate', className)}>
      <Shadow />
      <Window className="relative" title={<span className="font-pixel text-[13px] antialiased">Your engineering runtime</span>}>
        <div data-rows className="relative">
          {ORDER.map((id) => {
            const m = MODULES[id];
            const fixed = FIXED.has(id);
            return (
              <div
                key={id}
                data-row={id}
                data-state={visible ? 'on' : 'off'}
                className="relative border-lp-line px-4 py-3 not-first:border-t transition-opacity duration-300 data-[state=off]:opacity-40 data-aim:bg-lp-paper-2"
              >
                <p data-rowname className="flex items-baseline gap-2">
                  <span className="font-pixel text-[13px] antialiased">{m.name}</span>
                  <span className="text-sm/5 text-pretty text-lp-ink-2">{m.role}</span>
                  <span className="ms-auto inline-flex shrink-0 items-center gap-1 font-pixel text-[13px] text-lp-ink-2 antialiased">
                    {fixed && <Lock aria-hidden className="size-3" />}
                    {m.tag}
                  </span>
                </p>
                <div className="mt-2 flex flex-col gap-1.5">
                  {m.groups.map((g, gi) => (
                    <div
                      key={g.id}
                      role="radiogroup"
                      aria-label={fixed ? `Inspect ${m.name} (fixed, cannot be swapped)` : g.label ? `${m.name}: ${g.label}` : m.name}
                      className="flex flex-wrap items-center gap-0.75"
                    >
                      {g.label && <span className="me-1 w-21 shrink-0 font-pixel text-[13px] text-lp-ink-2 antialiased">{g.label}</span>}
                      {g.options.map((o, oi) => (
                        <Chip
                          key={o.title}
                          checked={sel[id][gi] === oi}
                          fixed={fixed}
                          onPick={() => onPick({ mod: id, group: gi, option: oi })}
                        >
                          {o.title}
                        </Chip>
                      ))}
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      </Window>
    </div>
  );
}

/** the figure without WebGL: the same six modules and the same readout, laid out as a list */
function StaticFigure({ sel, focus }: { sel: Selection; focus: Pick }) {
  return (
    <div className="absolute inset-0 flex flex-col gap-2 overflow-hidden border border-lp-line-strong bg-lp-paper-2 p-3 font-pixel text-[13px] antialiased">
      <ReadoutText pick={focus} className="border border-lp-ink bg-lp-paper p-2 text-lp-ink" />
      <ul className="flex flex-col gap-1">
        {ORDER.map((id) => {
          const m = MODULES[id];
          const fixed = FIXED.has(id);
          return (
            <li key={id} className={cn('flex items-center gap-2 border px-2 py-1', fixed ? 'border-lp-ink bg-lp-ink text-lp-paper' : 'border-lp-line-strong bg-lp-paper')}>
              {fixed && <Lock aria-hidden className="size-3" />}
              <span>{m.name}</span>
              <span className="ms-auto opacity-70">{m.plate!(sel[id], focus.mod === id ? focus.group : 0)}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function Stage() {
  const reduced = useReducedMotion();
  const [sel, setSel] = useState<Selection>(defaultSelection);
  const [focus, setFocus] = useState<Pick>({ mod: 'adapters', group: 0, option: 0 });
  const [mode, setMode] = useState<'gl' | 'static'>('gl');
  const [full, setFull] = useState(true);
  const [epoch, setEpoch] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const engine = useRef<Engine | null>(null);
  const latest = useRef({ sel, focus });
  latest.current = { sel, focus };

  const apply = useCallback((pick: Pick) => {
    setSel((s) => ({ ...s, [pick.mod]: s[pick.mod].map((v, i) => (i === pick.group ? pick.option : v)) }));
    setFocus(pick);
  }, []);

  const onPick = useCallback(
    (pick: Pick) => {
      apply(pick);
      engine.current?.request(pick);
    },
    [apply],
  );

  useEffect(() => {
    const root = rootRef.current;
    if (!root) return;
    let retry = 0;
    const made = createArchitecture(
      root,
      {
        sync: apply,
        status: setStatus,
        layout: setFull,
        fail: () => {
          engine.current?.destroy();
          engine.current = null;
          setMode('static');
          // a lost context is made again, finished, after a pause
          retry = window.setTimeout(() => setEpoch((e) => e + 1), 1500);
        },
      },
      { reduced: reduced || epoch > 0, selection: latest.current.sel, focus: latest.current.focus },
    );
    engine.current = made;
    setMode(made ? 'gl' : 'static');
    return () => {
      window.clearTimeout(retry);
      made?.destroy();
      if (engine.current === made) engine.current = null;
    };
  }, [reduced, epoch, apply]);

  const live = mode === 'gl';
  const text = readoutOf(focus);

  return (
    <div ref={rootRef} className="relative mt-8 md:mt-10">
      <div className="grid md:-mt-2 lg:grid-cols-[minmax(0,56.5rem)_23rem] lg:justify-between lg:gap-x-12 xl:grid-cols-[minmax(0,56.5rem)_25rem]">
        <div className="relative -ms-(--lp-gutter) self-start md:ms-0 md:max-w-226" style={heights}>
          <div data-holder className="relative">
            <div className="pb-(--phone) md:pb-(--wide)" />
            {/* clipped, the faces can't take the rest of the page into layers */}
            <div className={cn('pointer-events-none absolute inset-0 overflow-clip', !live && 'hidden')}>
              <Plates />
              <Displays />
            </div>
            <svg data-cables aria-hidden className={cn('pointer-events-none absolute inset-0 z-30 hidden size-full overflow-visible', live && 'lg:block')} />
            {!live && <StaticFigure sel={sel} focus={focus} />}
          </div>
          <p className="mt-2 ms-(--lp-gutter) font-pixel text-[13px] text-lp-ink-2 antialiased md:ms-0">{DISCLOSURE}</p>
          {live && !full && (
            <div className="mt-3 ms-(--lp-gutter) me-(--lp-gutter) border border-lp-line-strong bg-lp-paper-2 p-3 md:ms-0 md:me-0">
              <ReadoutText pick={focus} className="text-lp-ink" />
            </div>
          )}
        </div>
        <div className="mt-6 md:max-w-120 lg:mt-0 lg:self-start">
          <Panel sel={sel} visible={!live} onPick={onPick} />
        </div>
      </div>
      <p className="sr-only" aria-live="polite">
        {`${text.title}. ${text.lines.join('. ')}.`}
      </p>
    </div>
  );
}
