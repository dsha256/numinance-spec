"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { useReducedMotion } from '@/lib/landing/use-reduced';
import { Chip } from "@/components/landing/ui/chip";
import { Shadow, Window } from "@/components/landing/ui/window";
import { OperatorRider } from "@/components/landing/machine/operator-sprite";
import {
  PixelButton,
  REPLAY_ICON,
} from "@/components/landing/machine/pixel-button";
import { useInView } from "@/components/landing/machine/use-in-view";
import { SYMBOL_PIXELS } from "@/lib/landing/operator";
import { scopedStatus } from "@/lib/landing/status";
const setStatus = scopedStatus('system');
import { cn } from "@/lib/cn";
import "@/components/landing/machine/machine.css";

type State = "idle" | "active" | "done";

const TICK = 340;
const MAX = 17;

/** Tiny pixel glyph made of rects. */
function Pixels({
  rows,
  x,
  y,
  s = 1,
}: {
  rows: readonly string[];
  x: number;
  y: number;
  s?: number;
}) {
  return rows.flatMap((row, r) =>
    [...row].map((ch, c) =>
      ch === "#" ? (
        <rect
          key={`${r}-${c}`}
          x={x + c * s}
          y={y + r * s}
          width={s}
          height={s}
          fill="currentColor"
          stroke="none"
        />
      ) : null,
    ),
  );
}

const SVG =
  "overflow-visible fill-none stroke-current [shape-rendering:crispEdges]";

function Bay({
  state,
  label,
  sub,
  children,
  className,
  big = false,
  index,
}: {
  state: State;
  label: string;
  sub: string;
  children: ReactNode;
  className?: string;
  big?: boolean;
  index?: string;
}) {
  return (
    <div
      data-state={state}
      className={cn(
        "mx-bay relative z-10 flex items-center gap-3 border bg-lp-paper px-2 transition-colors duration-200",
        state === "idle"
          ? "border-lp-line text-lp-ink-2"
          : "border-lp-ink text-lp-ink",
        className,
      )}
    >
      <svg
        aria-hidden
        viewBox="0 0 64 40"
        className={cn(SVG, "h-10 w-16 shrink-0", big && "lg:h-[60px] lg:w-24")}
      >
        {children}
      </svg>
      <div className={cn("min-w-0", big && "lg:text-center")}>
        <div className="font-pixel text-[13px] leading-4 antialiased">
          {label}
        </div>
        <div className="font-mono text-[11px] leading-4 text-lp-ink-2">
          {sub}
        </div>
      </div>
      {index && (
        <span
          aria-hidden
          className="absolute top-0.5 right-1 font-pixel text-[11px] leading-3 text-lp-ink-2 max-lg:hidden"
        >
          {index}
        </span>
      )}
      <span
        aria-hidden
        className={cn(
          "absolute top-1 left-1 size-1.5 transition-colors max-lg:hidden",
          state === "active"
            ? "bg-lp-sun"
            : state === "done"
              ? "bg-lp-ink"
              : "bg-lp-line-strong",
        )}
      />
    </div>
  );
}

const ORANGE = "stroke-lp-orange";
const FILL_O = "fill-lp-orange stroke-none";

/* ---------- Track B stations (illustrations) ---------- */
function Intent() {
  return (
    <>
      <rect x="10.5" y="4.5" width="44" height="31" />
      <path d="M16 12.5h22M16 18.5h30M16 24.5h14" />
      <rect
        className={cn("caret s-act", FILL_O)}
        x="32"
        y="22"
        width="3"
        height="5"
      />
    </>
  );
}
function Understand() {
  return (
    <>
      <path d="M14 10L32 20L50 8M32 20L22 32M32 20L48 31" />
      {[
        [11, 7],
        [29, 17],
        [47, 5],
        [19, 29],
        [45, 28],
      ].map(([x, y]) => (
        <rect
          key={`${x}${y}`}
          x={x}
          y={y}
          width="6"
          height="6"
          className="fill-lp-paper"
        />
      ))}
      <g className={cn("s-seen", FILL_O)}>
        <rect x="29" y="17" width="6" height="6" />
        <rect x="11" y="7" width="6" height="6" />
        <rect x="47" y="5" width="6" height="6" />
      </g>
    </>
  );
}
function Plan() {
  return (
    <>
      <path d="M4 9.5v21M4 9.5h4M4 19.5h4M4 29.5h4" />
      {[
        [7, 48],
        [17, 34],
        [27, 40],
      ].map(([y, w], i) => (
        <g key={y}>
          <rect x="9.5" y={y - 0.5} width="48" height="6" />
          <rect
            className={cn("grow", FILL_O)}
            style={{ transitionDelay: `${i * 90}ms` }}
            x="10"
            y={y}
            width={w - 1}
            height="5"
          />
        </g>
      ))}
    </>
  );
}
function Act() {
  return (
    <>
      <path d="M6 6.5h52M10 4v5M54 4v5" />
      <g className="piston">
        <path d="M32 6v12" />
        <rect x="28" y="18" width="8" height="6" className="fill-current" />
      </g>
      <rect x="14.5" y="31.5" width="36" height="5" />
      <path className={cn("s-act", ORANGE)} d="M26 29l-2 2M38 29l2 2M32 29v2" />
    </>
  );
}
function Gate() {
  return (
    <>
      <circle cx="12" cy="6" r="2.5" className={cn("fill-lp-paper", ORANGE)} />
      <circle
        cx="12"
        cy="6"
        r="1.5"
        className={cn("blink s-seen fill-lp-yellow stroke-none")}
      />
      <rect x="10" y="10" width="4" height="24" className="fill-current" />
      <g className="gate-arm">
        <rect
          x="14.5"
          y="14.5"
          width="38"
          height="4"
          className="fill-lp-paper"
        />
        <path d="M20 14.5v4M28 14.5v4M36 14.5v4M44 14.5v4" />
      </g>
      <path d="M8 34.5h14" />
    </>
  );
}
function Verify() {
  return (
    <>
      <rect x="10.5" y="6.5" width="40" height="27" />
      <path d="M16 14.5h20M16 20.5h26M16 26.5h14" />
      <rect
        className={cn("scan s-act", FILL_O)}
        x="12"
        y="7"
        width="2"
        height="26"
      />
      <path
        className={cn("s-done", ORANGE)}
        d="M40 25l4 4 10-12"
        strokeWidth="2"
      />
    </>
  );
}
function Record() {
  return (
    <>
      <g className="slip">
        <rect
          x="17.5"
          y="21.5"
          width="29"
          height="18"
          className="fill-lp-paper"
        />
        <path d="M22 26.5h20M22 31.5h14M22 35.5h18" />
      </g>
      <rect x="8.5" y="6.5" width="47" height="15" className="fill-lp-paper" />
      <path d="M16 21.5h32" strokeWidth="2" />
      <rect
        x="13"
        y="11"
        width="10"
        height="2"
        className="fill-current stroke-none"
      />
      <rect
        className={cn("blink s-act", FILL_O)}
        x="46"
        y="10"
        width="4"
        height="4"
      />
    </>
  );
}
function Outcome() {
  return (
    <>
      <rect x="6.5" y="8.5" width="28" height="24" />
      <path d="M11 14.5h14M11 19.5h18M11 24.5h10" />
      <path
        className="s-done stroke-lp-orange"
        d="M24 26l3 3 6-7"
        strokeWidth="2"
      />
      <g className="s-done">
        <rect
          x="39.5"
          y="10.5"
          width="18"
          height="24"
          className="fill-lp-paper"
        />
        <path d="M43 28.5h11M43 32.5h8" />
        <Pixels rows={SYMBOL_PIXELS} x={43} y={14} s={2} />
      </g>
    </>
  );
}

/* ---------- Track A stations ---------- */
function Prompt() {
  return (
    <>
      <rect x="10.5" y="8.5" width="44" height="22" />
      <path d="M16 15.5h24M16 22.5h12" />
      <rect
        className={cn("caret s-act", FILL_O)}
        x="30"
        y="20"
        width="3"
        height="5"
      />
    </>
  );
}
function Code() {
  return (
    <>
      <rect x="10.5" y="6.5" width="44" height="28" />
      <path d="M16 13.5h10M20 18.5h18M20 23.5h12M16 28.5h6" />
    </>
  );
}
function Output() {
  return (
    <>
      <rect x="10.5" y="8.5" width="44" height="22" strokeDasharray="3 2" />
      <path className="s-seen" d="M16 16.5h14M16 22.5h22" />
    </>
  );
}

const B_STEPS: readonly {
  k: string;
  label: string;
  sub: string;
  draw: ReactNode;
}[] = [
  { k: "intent", label: "INTENT", sub: "goal + limits", draw: <Intent /> },
  {
    k: "understand",
    label: "UNDERSTAND",
    sub: "read the system",
    draw: <Understand />,
  },
  { k: "plan", label: "PLAN", sub: "scope + risk", draw: <Plan /> },
  { k: "act", label: "ACT", sub: "scoped tools", draw: <Act /> },
  { k: "gate", label: "POLICY GATE", sub: "hold or release", draw: <Gate /> },
  { k: "verify", label: "VERIFY", sub: "checks + review", draw: <Verify /> },
  { k: "record", label: "RECORD", sub: "evidence slip", draw: <Record /> },
  {
    k: "outcome",
    label: "VERIFIED OUTCOME",
    sub: "change + receipt",
    draw: <Outcome />,
  },
];

export function SystemMachine() {
  const reduced = useReducedMotion() === true;
  const root = useRef<HTMLDivElement>(null);
  const [tick, setTick] = useState(MAX);
  const [mounted, setMounted] = useState(false);
  const timer = useRef<ReturnType<typeof setInterval> | undefined>(undefined);
  const played = useRef(false);

  const tickRef = useRef(MAX);
  const visible = useRef(false);

  /** one interval, alive only while a run is unfinished, the figure is on screen and the tab is visible */
  const pump = useCallback(() => {
    clearInterval(timer.current);
    timer.current = undefined;
    if (!played.current || tickRef.current >= MAX || !visible.current || document.hidden) return;
    timer.current = setInterval(() => {
      tickRef.current = Math.min(MAX, tickRef.current + 1);
      setTick(tickRef.current);
      if (tickRef.current >= MAX) pump();
    }, TICK);
  }, []);

  const play = useCallback(() => {
    played.current = true;
    tickRef.current = 0;
    setTick(0);
    setStatus("code generation: prompt → code");
    pump();
  }, [pump]);

  useEffect(() => {
    setMounted(true);
    const vis = () => pump();
    document.addEventListener("visibilitychange", vis);
    return () => {
      clearInterval(timer.current);
      document.removeEventListener("visibilitychange", vis);
    };
  }, [pump]);
  useInView(
    root,
    () => {
      visible.current = true;
      if (!played.current && !reduced) play();
      else pump();
    },
    () => {
      visible.current = false;
      pump();
    },
    0.2,
  );

  // narrate in the status bar
  useEffect(() => {
    if (!played.current || reduced) return;
    if (tick === 3) setStatus("prompt → code: output unreviewed, no evidence");
    if (tick === 8) setStatus("intent → engineering system: authority checked");
    if (tick === MAX)
      setStatus("evidence recorded: slip printed (concept)");
  }, [tick, reduced]);

  const final = reduced || !mounted || tick >= MAX;
  const t = final ? MAX : tick;
  const ai = Math.min(t, 2);
  const aDone = t >= 3;
  const bi = Math.min(Math.floor(t / 2), 7);
  const bAll = t >= MAX;
  const stateB = (j: number): State =>
    bAll || j < bi ? "done" : j === bi ? "active" : "idle";
  const stateA = (j: number): State =>
    aDone
      ? j === 2
        ? "done"
        : "done"
      : j < ai
        ? "done"
        : j === ai
          ? "active"
          : "idle";

  return (
    <Window
      ref={root as never}
      title="engineering-compare.sch"
      className="relative"
      actions={
        <span
          aria-hidden
          className="relative size-3.5 border border-lp-line-strong bg-lp-paper"
        />
      }
    >
      {/* instrument strip */}
      <div className="relative flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-lp-line px-3 py-2">
        <Chip>concept illustration</Chip>
        <span className="font-mono text-xs text-lp-ink-2">
          same request, two ways to handle it
        </span>
        <div className="ms-auto">
          <PixelButton
            icon={REPLAY_ICON}
            onClick={play}
            aria-label="Replay the comparison"
          >
            Replay
          </PixelButton>
        </div>
      </div>

      <div className="lp-grid relative bg-size-[24px_24px] px-3 pt-4 pb-5 lg:px-5">
        {/* TRACK A */}
        <div className="mb-3 flex items-baseline gap-3">
          <span className="font-pixel text-[13px] leading-4 text-lp-ink-2">
            A
          </span>
          <span className="font-pixel text-[13px] leading-4">
            PROMPT → CODE
          </span>
        </div>
        <div
          className="relative [--n:3] [--ay:30px] lg:[--n:8]"
          style={{ "--i": ai } as CSSProperties}
        >
          <div className="mx-line mx-hz lg:right-[calc(5.5/8*100%)]!" />
          <div
            className="mx-fill mx-hz"
            style={{ "--dur": "0.22s" } as CSSProperties}
          />
          <div
            aria-hidden
            className="mx-at mx-hz size-3.5 border border-lp-ink bg-lp-yellow"
            style={
              { "--dur": "0.22s", opacity: aDone ? 0 : 1 } as CSSProperties
            }
          />
          <div className="grid grid-cols-3 lg:grid-cols-8">
            {[
              {
                k: "prompt",
                label: "PROMPT",
                sub: "one message",
                draw: <Prompt />,
              },
              { k: "code", label: "CODE", sub: "generated", draw: <Code /> },
              {
                k: "output",
                label: "OUTPUT",
                sub: "unreviewed",
                draw: <Output />,
              },
            ].map((s, j) => (
              <div key={s.k} className="relative px-1.5 lg:px-2.5">
                <Bay
                  state={stateA(j)}
                  label={s.label}
                  sub={s.sub}
                  className="flex-col gap-0.5 px-1 pt-2 pb-1.5 text-center max-sm:[&>div]:max-w-full"
                >
                  {s.draw}
                </Bay>
                {j === 2 && (
                  <span
                    aria-hidden
                    className={cn(
                      "absolute top-[38px] left-1/2 z-20 -translate-x-1/2 -rotate-6 transition-opacity duration-200",
                      aDone ? "opacity-100" : "opacity-0",
                    )}
                  >
                    <span className="relative isolate block">
                      <Shadow />
                      <span className="relative block border border-dashed border-lp-ink bg-lp-paper px-1.5 py-0.5 font-pixel text-[11px] leading-3 whitespace-nowrap">
                        NO EVIDENCE
                      </span>
                    </span>
                  </span>
                )}
              </div>
            ))}
            <div className="relative col-span-5 hidden items-center px-4 lg:flex">
              <p className="ps-5 font-pixel text-[13px] leading-5 text-lp-ink-2">
                <span
                  className={cn(
                    "transition-opacity",
                    aDone ? "opacity-100" : "opacity-0",
                  )}
                >
                  done in one step — and nothing else happened
                </span>
              </p>
            </div>
          </div>
          <p className="mt-2 flex flex-wrap gap-x-5 gap-y-1 font-mono text-[11px] leading-4 text-lp-ink-2">
            <span>stages: 1</span>
            <span>policy: none</span>
            <span>reviewed: {aDone ? "no" : "…"}</span>
            <span>evidence: {aDone ? "none" : "…"}</span>
          </p>
        </div>

        <div aria-hidden className="my-5 h-px bg-lp-line-strong lg:my-6" />

        {/* TRACK B */}
        <div className="mb-3 flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="font-pixel text-[13px] leading-4 text-lp-ink-2">
            B
          </span>
          <span className="font-pixel text-[13px] leading-4">
            INTENT → ENGINEERING SYSTEM → VERIFIED OUTCOME
          </span>
        </div>
        <div>
          <div
            className="relative [--ax:14px] [--ay:98px] [--n:8] lg:pt-14"
            style={{ "--i": bi } as CSSProperties}
          >
            <div className="mx-line mx-lg" />
            <div className="mx-fill mx-lg" />
            <div
              aria-hidden
              className="mx-at mx-lg size-3.5 border border-lp-ink bg-lp-yellow shadow-[2px_2px_0_var(--lp-ink)]"
              style={{ opacity: bAll ? 0 : 1 }}
            />
            <OperatorRider index={bi} count={8} bp="lg" ry="56px" hold={bAll} />
            {/* "engineering system" bracket */}
            <div
              aria-hidden
              className="absolute top-0 right-0 bottom-0 w-5 hidden max-lg:block"
              style={
                {
                  top: "calc(1 / 8 * 100%)",
                  bottom: "calc(1 / 8 * 100%)",
                } as CSSProperties
              }
            >
              <div className="absolute inset-y-0 right-0 w-1.5 border-y border-r border-lp-line-strong" />
              <span className="absolute top-1/2 left-0 -translate-y-1/2 [writing-mode:vertical-rl] rotate-180 bg-lp-paper py-1 font-pixel text-[11px] leading-3 text-lp-ink-2">
                ENGINEERING SYSTEM
              </span>
            </div>
            <div
              aria-hidden
              className="absolute top-1 right-[calc(1/8*100%)] left-[calc(1/8*100%)] hidden h-3 border-x border-t border-lp-line-strong lg:block"
            >
              <span className="absolute -top-2 left-1/2 -translate-x-1/2 bg-lp-paper px-2 font-pixel text-[11px] leading-4 text-lp-ink-2">
                ENGINEERING SYSTEM
              </span>
            </div>
            <div className="flex flex-col ps-8 pe-5 lg:grid lg:grid-cols-8 lg:ps-0 lg:pe-0">
              {B_STEPS.map((s, j) => (
                <div key={s.k} className="relative my-1 lg:my-0 lg:px-2.5">
                  <Bay
                    big
                    index={String(j + 1).padStart(2, "0")}
                    state={stateB(j)}
                    label={s.label}
                    sub={s.sub}
                    className="h-12 lg:h-[132px] lg:flex-col lg:gap-1 lg:px-0 lg:pt-3 lg:pb-2.5"
                  >
                    {s.draw}
                  </Bay>
                </div>
              ))}
            </div>
          </div>
          <p className="mt-2 flex flex-wrap gap-x-5 gap-y-1 font-mono text-[11px] leading-4 text-lp-ink-2">
            <span>stages: 6</span>
            <span>
              policy gate:{" "}
              {stateB(4) === "done"
                ? "released"
                : stateB(4) === "active"
                  ? "holding"
                  : "closed"}
            </span>
            <span>reviewed: {bAll ? "yes — independent" : "…"}</span>
            <span>evidence: {bAll ? "1 record" : "…"}</span>
          </p>
        </div>
      </div>
    </Window>
  );
}
