/**
 * Scene owner for the Mars / context-ignition hero experiment (illustrative, pre-alpha).
 *
 * A pure, dependency-free finite-state machine. It knows nothing about pixels, timers or copy: the
 * renderer drives it with events, the UI reads labels from `copy.ts`. The five facts below are
 * SEPARATE variables and are never derived from each other:
 *   requestedAutonomy  what the workflow asks for
 *   effectiveAutonomy  what the (simulated) external policy currently lets it do
 *   contextReadiness   state of the context bundle (readiness is not authorization)
 *   policyDecision     the simulated external policy result
 *   approvalState      the visitor's illustrative click (not a real grant)
 */
export const LEVELS = ['OBSERVE', 'SUGGEST', 'IMPLEMENT', 'PR', 'MERGE', 'STAGE', 'RELEASE', 'OPERATE'] as const;
export type Level = (typeof LEVELS)[number];
export const levelIndex = (l: Level) => LEVELS.indexOf(l);
export const minLevel = (a: Level, b: Level): Level => (levelIndex(a) <= levelIndex(b) ? a : b);

export type HeroState =
  | 'DORMANT'
  | 'BOOT'
  | 'CONTEXT_RETRIEVING'
  | 'CONTEXT_VALIDATING'
  | 'BLOCKED_CONTEXT'
  | 'CONTEXT_READY'
  | 'POLICY_EVALUATING'
  | 'POLICY_PERMITTED'
  | 'NEEDS_APPROVAL'
  | 'BLOCKED_POLICY'
  | 'APPROVED'
  | 'AUTONOMOUS_SEQUENCE'
  | 'AUTONOMOUS_INTERACTIVE'
  | 'REST';

export type Scenario = 'approval' | 'preauthorized' | 'policy-deny';
export type ContextReadiness = 'unrequested' | 'retrieving' | 'validating' | 'blocked' | 'ready';
export type PolicyDecision = 'none' | 'pending' | 'requires_approval' | 'permitted' | 'denied';
export type ApprovalState = 'none' | 'requested' | 'approved_demo' | 'declined';
export type StaleSource = 'none' | 'flagged' | 'refetched';

/**
 * The browse rail. Each stage owns a set of machine states; a jump plays that stage and holds at its end.
 * Presentation only: labels live in copy.ts.
 */
export const RAIL = ['context', 'validate', 'policy', 'approval', 'autonomous', 'evidence'] as const;
export type Rail = (typeof RAIL)[number];
export function railOf(s: HeroState): Rail | null {
  switch (s) {
    case 'BOOT':
    case 'CONTEXT_RETRIEVING':
      return 'context';
    case 'CONTEXT_VALIDATING':
    case 'BLOCKED_CONTEXT':
    case 'CONTEXT_READY':
      return 'validate';
    case 'POLICY_EVALUATING':
    case 'POLICY_PERMITTED':
      return 'policy';
    case 'NEEDS_APPROVAL':
    case 'BLOCKED_POLICY':
    case 'APPROVED':
      return 'approval';
    case 'AUTONOMOUS_SEQUENCE':
      return 'autonomous';
    case 'AUTONOMOUS_INTERACTIVE':
      return 'evidence';
    default:
      return null;
  }
}

/** who moves the story: the clock-driven autoplay / browse driver, or the visitor's clicks (guided) */
export type Drive = 'auto' | 'manual';

export interface Vars {
  /** auto: the driver dispatches the same events a visitor would (never the human-gate ones). manual: guided. */
  drive: Drive;
  /** browse: events that would leave this rail stage are not taken, so the scene holds at the stage's end */
  stop: Rail | null;
  /** increments on every restart / jump, so the renderer can restart a state's clock even for the same state */
  epoch: number;
  scenario: Scenario;
  requestedAutonomy: Level;
  effectiveAutonomy: Level;
  /** the policy's cap for this qualified scope (display only; the model never sets it) */
  policyCap: Level;
  contextReadiness: ContextReadiness;
  policyDecision: PolicyDecision;
  approvalState: ApprovalState;
  staleSource: StaleSource;
  /** the demo's stale-source blocker is part of the story; `false` is only for test scripts */
  staleEnabled: boolean;
  accelerate: boolean;
}

export interface Snapshot {
  readonly state: HeroState;
  readonly vars: Readonly<Vars>;
  /** increments on every accepted event: cheap change detection for the renderer and React */
  readonly rev: number;
}

export type HeroEvent =
  | { type: 'START' }
  | { type: 'AUTOPLAY' }
  | { type: 'JUMP'; to: Rail }
  | { type: 'BOOTED' }
  | { type: 'RETRIEVED' }
  | { type: 'STALE_FOUND' }
  | { type: 'REFETCH' }
  | { type: 'VALIDATED' }
  | { type: 'READY_DONE' }
  | { type: 'POLICY_RESULT' }
  | { type: 'APPROVE' }
  | { type: 'DENY' }
  | { type: 'RECONSIDER' }
  | { type: 'BEGIN' }
  | { type: 'SEQUENCE_DONE' }
  | { type: 'SKIP' }
  | { type: 'ACCELERATE' }
  | { type: 'RESET' }
  | { type: 'REPLAY' }
  | { type: 'SET_SCENARIO'; scenario: Scenario };

export const REQUESTED_DEFAULT: Level = 'RELEASE';

/**
 * The simulated external policy. Pure function of the scenario and the request. It is evaluated
 * only at POLICY_EVALUATING and reads neither context readiness nor approval state.
 */
export function evaluatePolicy(scenario: Scenario, requested: Level) {
  switch (scenario) {
    case 'preauthorized':
      // a qualified playbook whose policy already permits this scope: no per-change human approval
      return { decision: 'permitted' as const, cap: minLevel(requested, 'RELEASE'), effective: minLevel(requested, 'RELEASE') };
    case 'policy-deny':
      return { decision: 'denied' as const, cap: 'PR' as Level, effective: 'PR' as Level };
    default:
      // the cap allows the scope, but policy requires a human approver: until then only a PR
      return { decision: 'requires_approval' as const, cap: minLevel(requested, 'RELEASE'), effective: 'PR' as Level };
  }
}

/**
 * The only gate into autonomous work. Context readiness is necessary but never sufficient: the
 * policy must also have permitted the path (preset) or required approval that was given.
 */
export function mayRunAutonomously(v: Readonly<Vars>): boolean {
  if (v.contextReadiness !== 'ready') return false;
  if (v.policyDecision === 'permitted') return true;
  return v.policyDecision === 'requires_approval' && v.approvalState === 'approved_demo';
}

export function initialVars(scenario: Scenario = 'approval', staleEnabled = true): Vars {
  return {
    drive: 'manual',
    stop: null,
    epoch: 0,
    scenario,
    requestedAutonomy: REQUESTED_DEFAULT,
    effectiveAutonomy: 'OBSERVE',
    policyCap: 'OBSERVE',
    contextReadiness: 'unrequested',
    policyDecision: 'none',
    approvalState: 'none',
    staleSource: 'none',
    staleEnabled,
    accelerate: false,
  };
}

export function createSnapshot(state: HeroState = 'DORMANT', scenario: Scenario = 'approval', staleEnabled = true): Snapshot {
  return { state, vars: initialVars(scenario, staleEnabled), rev: 0 };
}

/** the completed post-gate pose, for art-direction review (?hero=autonomous) */
function completedVars(scenario: Scenario, staleEnabled: boolean): Vars {
  const sc: Scenario = scenario === 'policy-deny' ? 'approval' : scenario;
  const p = evaluatePolicy(sc, REQUESTED_DEFAULT);
  return {
    ...initialVars(sc, staleEnabled),
    contextReadiness: 'ready',
    policyDecision: p.decision,
    policyCap: p.cap,
    approvalState: p.decision === 'requires_approval' ? 'approved_demo' : 'none',
    effectiveAutonomy: minLevel(REQUESTED_DEFAULT, p.cap),
    staleSource: staleEnabled ? 'refetched' : 'none',
  };
}
export function completedSnapshot(scenario: Scenario = 'approval', staleEnabled = true): Snapshot {
  return { state: 'AUTONOMOUS_INTERACTIVE', vars: completedVars(scenario, staleEnabled), rev: 0 };
}

/** the autoplay preset: the simulated policy ALREADY permits this qualified playbook (no per-change human approval) */
const AUTO_SCENARIO: Scenario = 'preauthorized';

/**
 * Deterministic pose for a rail stage. The earlier stages' facts are set truthfully (e.g. context is ready
 * before policy), the stage itself starts at its beginning and plays, then holds at its end (`stop`).
 * No human click is ever implied: the gate stage waits for the visitor, every other stage is policy-driven.
 */
function jumpVars(v: Readonly<Vars>, to: Rail): { state: HeroState; vars: Vars } {
  const staleDone: StaleSource = v.staleEnabled ? 'refetched' : 'none';
  const base = (sc: Scenario): Vars => ({ ...initialVars(sc, v.staleEnabled), drive: 'auto', accelerate: v.accelerate, epoch: v.epoch + 1 });
  switch (to) {
    case 'context':
      return { state: 'BOOT', vars: { ...base(AUTO_SCENARIO), contextReadiness: 'retrieving', stop: 'context' } };
    case 'validate':
      return { state: 'CONTEXT_VALIDATING', vars: { ...base(AUTO_SCENARIO), contextReadiness: 'validating', stop: 'validate' } };
    case 'policy':
      return {
        state: 'POLICY_EVALUATING',
        vars: { ...base(AUTO_SCENARIO), contextReadiness: 'ready', policyDecision: 'pending', staleSource: staleDone, stop: 'policy' },
      };
    case 'approval': {
      // the scenario whose policy requires a human approver: the visitor's (simulated) click decides
      const p = evaluatePolicy('approval', REQUESTED_DEFAULT);
      return {
        state: 'NEEDS_APPROVAL',
        vars: {
          ...base('approval'),
          contextReadiness: 'ready',
          policyDecision: p.decision,
          policyCap: p.cap,
          effectiveAutonomy: p.effective,
          approvalState: 'requested',
          staleSource: staleDone,
          stop: 'approval',
        },
      };
    }
    case 'autonomous': {
      const p = evaluatePolicy(AUTO_SCENARIO, REQUESTED_DEFAULT);
      return {
        state: 'AUTONOMOUS_SEQUENCE',
        vars: {
          ...base(AUTO_SCENARIO),
          contextReadiness: 'ready',
          policyDecision: p.decision,
          policyCap: p.cap,
          effectiveAutonomy: p.effective,
          staleSource: staleDone,
        },
      };
    }
    case 'evidence':
      return { state: 'AUTONOMOUS_INTERACTIVE', vars: { ...completedVars(AUTO_SCENARIO, v.staleEnabled), drive: 'auto', accelerate: v.accelerate, epoch: v.epoch + 1 } };
  }
}

const next = (s: Snapshot, state: HeroState, patch: Partial<Vars> = {}, epoch?: number): Snapshot => ({
  state,
  vars: epoch === undefined ? { ...s.vars, ...patch } : { ...s.vars, ...patch, epoch },
  rev: s.rev + 1,
});

function step(s: Snapshot, e: HeroEvent): Snapshot {
  const { state, vars: v } = s;
  switch (e.type) {
    case 'SET_SCENARIO':
      if (state !== 'DORMANT' && state !== 'REST') return s;
      return next(s, state, { scenario: e.scenario });
    case 'ACCELERATE':
      return next(s, state, { accelerate: !v.accelerate });
    case 'START':
      // guided: the visitor's clicks move the story (re-fetch, approve or stay supervised)
      return next(s, 'BOOT', { ...initialVars(state === 'DORMANT' || state === 'REST' ? v.scenario : 'approval', v.staleEnabled), epoch: v.epoch + 1, contextReadiness: 'retrieving' });
    case 'AUTOPLAY':
    case 'REPLAY':
      // autoplay: the preset policy already permits the playbook, so the run needs no per-change approval
      return next(s, 'BOOT', {
        ...initialVars(AUTO_SCENARIO, v.staleEnabled),
        drive: 'auto',
        epoch: v.epoch + 1,
        accelerate: v.accelerate,
        contextReadiness: 'retrieving',
      });
    case 'JUMP': {
      const j = jumpVars(v, e.to);
      return next(s, j.state, j.vars);
    }
    case 'RESET':
      if (state === 'DORMANT' || state === 'REST') return s;
      // the guided scenario is chosen again at rest: autoplay's preset must not leak into it
      return next(s, 'REST', { ...initialVars('approval', v.staleEnabled), epoch: v.epoch + 1 });
    case 'BOOTED':
      return state === 'BOOT' ? next(s, 'CONTEXT_RETRIEVING', { contextReadiness: 'retrieving' }) : s;
    case 'RETRIEVED':
      return state === 'CONTEXT_RETRIEVING' ? next(s, 'CONTEXT_VALIDATING', { contextReadiness: 'validating' }) : s;
    case 'STALE_FOUND':
      if (state !== 'CONTEXT_VALIDATING' || !v.staleEnabled || v.staleSource !== 'none') return s;
      return next(s, 'BLOCKED_CONTEXT', { staleSource: 'flagged', contextReadiness: 'blocked' });
    case 'REFETCH':
      if (state !== 'BLOCKED_CONTEXT') return s;
      return next(s, 'CONTEXT_VALIDATING', { staleSource: 'refetched', contextReadiness: 'validating' });
    case 'VALIDATED':
      // a flagged (unresolved) blocker can never reach "ready"
      if (state !== 'CONTEXT_VALIDATING' || v.staleSource === 'flagged') return s;
      return next(s, 'CONTEXT_READY', { contextReadiness: 'ready' });
    case 'READY_DONE':
      // context readiness never grants anything: the external policy is evaluated next, as a separate step
      return state === 'CONTEXT_READY' && v.contextReadiness === 'ready' ? next(s, 'POLICY_EVALUATING', { policyDecision: 'pending' }) : s;
    case 'POLICY_RESULT': {
      if (state !== 'POLICY_EVALUATING') return s;
      const p = evaluatePolicy(v.scenario, v.requestedAutonomy);
      const patch: Partial<Vars> = { policyDecision: p.decision, policyCap: p.cap, effectiveAutonomy: p.effective };
      if (p.decision === 'denied') return next(s, 'BLOCKED_POLICY', { ...patch, approvalState: 'none' });
      if (p.decision === 'requires_approval') return next(s, 'NEEDS_APPROVAL', { ...patch, approvalState: 'requested' });
      const after = next(s, 'POLICY_PERMITTED', { ...patch, approvalState: 'none' });
      return mayRunAutonomously(after.vars) ? after : s;
    }
    case 'APPROVE':
      if (state !== 'NEEDS_APPROVAL') return s;
      return next(s, 'APPROVED', { approvalState: 'approved_demo', stop: null, effectiveAutonomy: minLevel(v.requestedAutonomy, v.policyCap) });
    case 'DENY':
      if (state !== 'NEEDS_APPROVAL') return s;
      return next(s, 'BLOCKED_POLICY', { approvalState: 'declined' });
    case 'RECONSIDER':
      if (state !== 'BLOCKED_POLICY' || v.policyDecision !== 'requires_approval') return s;
      return next(s, 'NEEDS_APPROVAL', { approvalState: 'requested' });
    case 'BEGIN':
      return (state === 'APPROVED' || state === 'POLICY_PERMITTED') && mayRunAutonomously(v) ? next(s, 'AUTONOMOUS_SEQUENCE') : s;
    case 'SKIP': {
      if (state === 'AUTONOMOUS_SEQUENCE') return next(s, 'AUTONOMOUS_INTERACTIVE');
      // skipping a running autoplay lands on the completed pose of the preset (policy-permitted) scenario only
      const running = railOf(state) !== null && state !== 'AUTONOMOUS_INTERACTIVE' && state !== 'NEEDS_APPROVAL' && state !== 'BLOCKED_POLICY';
      if (!running || v.drive !== 'auto' || v.stop !== null || v.scenario !== AUTO_SCENARIO) return s;
      return next(s, 'AUTONOMOUS_INTERACTIVE', completedVars(AUTO_SCENARIO, v.staleEnabled), v.epoch + 1);
    }
    case 'SEQUENCE_DONE':
      return state === 'AUTONOMOUS_SEQUENCE' ? next(s, 'AUTONOMOUS_INTERACTIVE') : s;
  }
}

/** events that restart or re-aim the whole story: never held back by a browse stop */
const CONTROL: ReadonlySet<HeroEvent['type']> = new Set(['RESET', 'REPLAY', 'AUTOPLAY', 'JUMP', 'SKIP', 'START', 'SET_SCENARIO', 'ACCELERATE']);

/**
 * The public transition. In browse mode (`vars.stop`) the stage plays through but an event that would
 * carry the scene out of that stage is not taken, so the pose holds at the stage's end.
 */
export function transition(s: Snapshot, e: HeroEvent): Snapshot {
  const n = step(s, e);
  const stop = s.vars.stop;
  if (n === s || !stop || CONTROL.has(e.type)) return n;
  return railOf(s.state) === stop && railOf(n.state) !== stop ? s : n;
}

/** states in which the visitor's click is what moves the story, not the clock */
export const WAITING: ReadonlySet<HeroState> = new Set(['DORMANT', 'REST', 'BLOCKED_CONTEXT', 'NEEDS_APPROVAL', 'BLOCKED_POLICY', 'AUTONOMOUS_INTERACTIVE']);
/** states that run a timeline and then emit their own completion event */
export const TIMED: ReadonlySet<HeroState> = new Set(['BOOT', 'CONTEXT_RETRIEVING', 'CONTEXT_VALIDATING', 'CONTEXT_READY', 'POLICY_EVALUATING', 'POLICY_PERMITTED', 'APPROVED', 'AUTONOMOUS_SEQUENCE']);
export const isAutonomous = (s: HeroState) => s === 'AUTONOMOUS_SEQUENCE' || s === 'AUTONOMOUS_INTERACTIVE';

export interface Store {
  get: () => Snapshot;
  dispatch: (e: HeroEvent) => Snapshot;
  subscribe: (l: () => void) => () => void;
}

export function createStore(initial: Snapshot): Store {
  let snap = initial;
  const ls = new Set<() => void>();
  return {
    get: () => snap,
    dispatch: (e) => {
      const n = transition(snap, e);
      if (n !== snap) {
        snap = n;
        ls.forEach((l) => l());
      }
      return snap;
    },
    subscribe: (l) => {
      ls.add(l);
      return () => {
        ls.delete(l);
      };
    },
  };
}
