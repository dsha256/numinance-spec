import { WORKFLOWS, type Workflow } from './data';

/**
 * CLI figure timeline. ONE clock `t` (ms since the run started) drives everything; every visual is a pure
 * function of it (`sample`). There are no per-phase timers and no idle beats: each step starts the instant
 * the work it depends on is done, and independent work overlaps.
 *
 *   t = 0            command starts typing AND the machine starts spinning up
 *   enter            last key: files start leaving the crate, one every CADENCE ms (they overlap in transit)
 *   land[i]          file i arrives: tree row + editor tab appear immediately
 *   flag             the instant the last file lands, the line is flagged
 *   select → strike → write → result      each follows the previous with only the time the eye needs
 *
 * Pacing follows the Fumadocs CLI scene (MIT): a key every ~20ms, files dispatched on a fixed cadence.
 */
export type Phase = 'typing' | 'dispense' | 'flag' | 'select' | 'strike' | 'rewrite' | 'result' | 'done';
export const PHASES: readonly Phase[] = ['typing', 'dispense', 'flag', 'select', 'strike', 'rewrite', 'result', 'done'];
const at = (p: Phase) => PHASES.indexOf(p);

/** typing: ms per key (deterministic jitter on top) */
const KEY = 16;
/** a beat after the last key before Enter registers */
const ENTER = 40;
/** ms between two files leaving the crate */
export const CADENCE = 240;
/** ms a file takes from the outlet to the tree */
export const TRAVEL = 950;
/** the ram's stroke, ms */
export const RAM = 360;
/** the diagnostic is read, the line selected, struck */
const FLAG = 340;
const SELECT = 200;
const STRIKE = 180;
/** ms per rewritten character */
const LETTER = 45;
/** result stays marked as the latest event this long before the run reads as idle */
const RESULT = 400;
/** machine spin up / down, ms */
export const SPIN_UP = 380;
export const SPIN_DOWN = 520;

const charAt = (i: number) => KEY + ((i * 7) % 9);

export interface Plan {
  wf: number;
  w: Workflow;
  n: number;
  /** ms at which key i has been pressed */
  keys: number[];
  enter: number;
  launch: number[];
  land: number[];
  flag: number;
  select: number;
  strike: number;
  write: number;
  result: number;
  done: number;
  /** after this the clock has nothing left to move */
  stop: number;
}

const plans = new Map<number, Plan>();

export function plan(wf: number): Plan {
  const hit = plans.get(wf);
  if (hit) return hit;
  const w = WORKFLOWS[wf];
  const n = w.files.length;
  const keys: number[] = [];
  let k = 0;
  for (let i = 0; i < w.command.length; i++) keys.push((k += charAt(i)));
  const enter = k + ENTER;
  const launch = Array.from({ length: n }, (_, i) => enter + i * CADENCE);
  const land = launch.map((l) => l + TRAVEL);
  const flag = land[n - 1];
  const select = flag + FLAG;
  const strike = select + SELECT;
  const write = strike + STRIKE;
  const result = write + w.edit.to.length * LETTER;
  const done = result + RESULT;
  const p: Plan = { wf, w, n, keys, enter, launch, land, flag, select, strike, write, result, done, stop: done + SPIN_DOWN };
  plans.set(wf, p);
  return p;
}

const count = (arr: number[], t: number) => {
  let c = 0;
  for (const x of arr) if (x <= t) c++;
  return c;
};
const smooth = (x: number) => {
  const u = x < 0 ? 0 : x > 1 ? 1 : x;
  return u * u * (3 - 2 * u);
};

/** Everything that changes during a run, for one instant. */
export interface Frame {
  p: Plan;
  wf: number;
  t: number;
  phase: Phase;
  /** characters of the command typed */
  typed: number;
  launched: number;
  landed: number;
  /** characters of the replacement typed */
  rw: number;
  /** spin 0..1, continuous */
  spin: number;
  /** 0 off · 1 winding up/down · 2 running */
  power: 0 | 1 | 2;
}

export function sample(p: Plan, t: number): Frame {
  const phase: Phase =
    t >= p.done ? 'done'
    : t >= p.result ? 'result'
    : t >= p.write ? 'rewrite'
    : t >= p.strike ? 'strike'
    : t >= p.select ? 'select'
    : t >= p.flag ? 'flag'
    : t >= p.enter ? 'dispense'
    : 'typing';
  const spin = t >= p.done ? 1 - smooth((t - p.done) / SPIN_DOWN) : smooth(t / SPIN_UP);
  return {
    p,
    wf: p.wf,
    t,
    phase,
    typed: count(p.keys, t),
    launched: count(p.launch, t),
    landed: count(p.land, t),
    rw: phase === 'rewrite' ? Math.min(p.w.edit.to.length, Math.floor((t - p.write) / LETTER)) : t >= p.result ? p.w.edit.to.length : 0,
    spin,
    power: spin < 0.01 ? 0 : t < SPIN_UP || t >= p.done ? 1 : 2,
  };
}

/** the settled frame of a run (reduced motion, first paint) */
export const finalFrame = (wf: number): Frame => sample(plan(wf), plan(wf).stop + 1);

/** React only needs to re-render when one of these changes; positions, spin and typed text are written to the DOM. */
export const frameKey = (f: Frame) => `${f.wf}|${f.phase}|${f.launched}|${f.landed}|${f.rw}|${f.power}`;

export interface Derived {
  w: Workflow;
  launched: number;
  landed: number;
  power: 0 | 1 | 2;
  /** 0 nothing yet · 1 flagged result · 2 corrected result */
  stage: 0 | 1 | 2;
  /** the edit line, in order of the sequence */
  edit: 'initial' | 'flag' | 'select' | 'strike' | 'rewrite' | 'final';
  running: boolean;
  label: string;
  phase: Phase;
}

export function derive(f: Frame): Derived {
  const s = f.phase;
  const p = at(s);
  const stage = p < at('flag') ? 0 : p < at('result') ? 1 : 2;
  const edit = p < at('flag') ? 'initial' : s === 'flag' ? 'flag' : s === 'select' ? 'select' : s === 'strike' ? 'strike' : s === 'rewrite' ? 'rewrite' : 'final';
  const label =
    s === 'typing' ? 'typing'
    : s === 'dispense' ? 'dispensing'
    : s === 'flag' ? 'checking'
    : s === 'select' || s === 'strike' || s === 'rewrite' ? 'rewriting'
    : s === 'result' ? 'result'
    : 'idle';
  return { w: f.p.w, launched: f.launched, landed: f.landed, power: f.power, stage, edit, running: s !== 'done', label, phase: s };
}

export function statusLine(f: Frame, d: Derived): string {
  const n = d.w.files.length;
  switch (f.phase) {
    case 'typing':
      return 'cli: typing command · powering up';
    case 'dispense':
      return `cli: dispensing ${Math.min(d.landed, n)}/${n} files`;
    case 'flag':
      return d.w.id === 'validate' ? 'validating files · 1 diagnostic' : 'verifying export · links unverified';
    case 'select':
    case 'strike':
    case 'rewrite':
      return `cli: rewriting line ${d.w.edit.line + 1}`;
    case 'result':
    case 'done':
      return f.phase === 'done'
        ? 'cli: idle · illustrative'
        : d.w.id === 'validate'
          ? `${n} files checked · schema pass only`
          : 'keys: caller must pin; no built-in root';
  }
}
