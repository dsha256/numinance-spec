import type { HeroEvent, HeroState, Rail, Snapshot } from './machine';

/**
 * User-facing words. Deliberately separate from the state machine: labels are DERIVED for display
 * and never read back by it. Every line is qualified as a simulation.
 */
export interface Label {
  head: string;
  sub: string;
  /** which of the two milestones, if any, is currently true (shown as a pill) */
  tone: 'idle' | 'working' | 'hold' | 'ok' | 'auto';
}

export function labelFor(s: Snapshot): Label {
  const v = s.vars;
  switch (s.state) {
    case 'DORMANT':
    case 'REST':
      return { head: 'CORE / DORMANT', sub: 'no context requested', tone: 'idle' };
    case 'BOOT':
      return { head: 'INITIALIZING CONTEXT', sub: 'illustrative', tone: 'working' };
    case 'CONTEXT_RETRIEVING':
      return { head: 'UNDERSTAND / RESOLVING AUTHORIZED SOURCES', sub: 'simulated, no live source access', tone: 'working' };
    case 'CONTEXT_VALIDATING':
      return { head: 'VALIDATING CONTEXT BUNDLE', sub: 'retrieved content is guidance, not authority', tone: 'working' };
    case 'BLOCKED_CONTEXT':
      return {
        head: 'UNRESOLVED CONTEXT',
        sub: v.drive === 'auto' ? 'stale source flagged: the runtime re-fetches it automatically' : 'stale source blocks the bundle: re-fetch to continue',
        tone: 'hold',
      };
    case 'CONTEXT_READY':
      return { head: 'CONTEXT READY / scoped bundle', sub: 'validated, not authorized: policy decides next', tone: 'ok' };
    case 'POLICY_EVALUATING':
      return { head: 'AUTHORITY EVALUATION', sub: 'context ready is not authority: policy decides', tone: 'working' };
    case 'POLICY_PERMITTED':
      return { head: 'POLICY PERMITS / QUALIFIED SCOPE', sub: 'policy permits this scope: no per-change approval', tone: 'ok' };
    case 'NEEDS_APPROVAL':
      return { head: 'APPROVAL REQUIRED', sub: 'simulated click, not a real grant', tone: 'hold' };
    case 'BLOCKED_POLICY':
      return v.approvalState === 'declined'
        ? { head: 'HELD / approval required', sub: 'staying supervised: no autonomous work started', tone: 'hold' }
        : { head: 'BLOCKED BY POLICY', sub: 'the policy cap does not allow this scope', tone: 'hold' };
    case 'APPROVED':
      return { head: 'APPROVED / SIMULATED', sub: 'demo only: starting the run', tone: 'ok' };
    case 'AUTONOMOUS_SEQUENCE':
      return { head: 'FULL AUTONOMY / QUALIFIED SCOPE', sub: 'ENGINEERING / IN PROGRESS', tone: 'auto' };
    case 'AUTONOMOUS_INTERACTIVE':
      return { head: 'FULL AUTONOMY / QUALIFIED SCOPE', sub: 'EVIDENCE RECORDED / DEMONSTRATION', tone: 'auto' };
  }
}

/** a short sentence for the aria-live region / status bar */
export function announceFor(s: Snapshot): string {
  const l = labelFor(s);
  return `[illustrative] ${l.head.toLowerCase()}: ${l.sub}`;
}

export interface Action {
  id: string;
  label: string;
  event: HeroEvent;
  kind: 'primary' | 'secondary' | 'quiet';
}

/** the compact controls for a state; `reduced` replaces the clock with an explicit Continue */
export function actionsFor(s: Snapshot, reduced: boolean): Action[] {
  const reset: Action = { id: 'reset', label: 'Reset', event: { type: 'RESET' }, kind: 'quiet' };
  const speed: Action = { id: 'speed', label: s.vars.accelerate ? 'Normal speed' : 'Speed up', event: { type: 'ACCELERATE' }, kind: 'quiet' };
  const replay: Action = { id: 'replay', label: 'Replay', event: { type: 'REPLAY' }, kind: 'secondary' };
  const guided: Action = { id: 'guided', label: 'Step through', event: { type: 'START' }, kind: 'quiet' };
  const v = s.vars;
  const auto = v.drive === 'auto';
  // the human gate and the held states keep their own controls in every mode
  if (s.state !== 'NEEDS_APPROVAL' && s.state !== 'BLOCKED_POLICY' && auto) {
    if (s.state === 'AUTONOMOUS_INTERACTIVE' || v.stop !== null) return [replay, guided, reset];
    return [{ id: 'skip', label: 'Skip to result', event: { type: 'SKIP' }, kind: 'secondary' }, reset];
  }
  switch (s.state) {
    case 'DORMANT':
      return [{ ...replay, label: 'Play' }, guided];
    case 'REST':
      return [replay, guided];
    case 'BOOT':
      return reduced ? [] : [speed, reset];
    case 'CONTEXT_RETRIEVING':
      return reduced ? [{ id: 'cont', label: 'Continue', event: { type: 'RETRIEVED' }, kind: 'primary' }, reset] : [speed, reset];
    case 'CONTEXT_VALIDATING':
    case 'CONTEXT_READY':
    case 'POLICY_EVALUATING':
    case 'POLICY_PERMITTED':
      return reduced ? [reset] : [speed, reset];
    case 'BLOCKED_CONTEXT':
      return [{ id: 'refetch', label: 'Re-fetch source', event: { type: 'REFETCH' }, kind: 'primary' }, reset];
    case 'NEEDS_APPROVAL':
      return [
        { id: 'approve', label: 'Approve simulated run', event: { type: 'APPROVE' }, kind: 'primary' },
        { id: 'deny', label: 'Stay supervised', event: { type: 'DENY' }, kind: 'secondary' },
        reset,
      ];
    case 'BLOCKED_POLICY':
      return s.vars.approvalState === 'declined'
        ? [{ id: 'reconsider', label: 'Reconsider', event: { type: 'RECONSIDER' }, kind: 'secondary' }, reset]
        : [reset];
    case 'APPROVED':
      return [];
    case 'AUTONOMOUS_SEQUENCE':
      return [{ id: 'skip', label: 'Skip to result', event: { type: 'SKIP' }, kind: 'quiet' }, reset];
    case 'AUTONOMOUS_INTERACTIVE':
      return [replay, guided, reset];
  }
}

export const SCENARIO_LABEL = {
  approval: 'Scenario: policy asks for approval',
  preauthorized: 'Scenario: already authorized by policy',
  'policy-deny': 'Scenario: policy blocks this scope',
} as const;

/** the browse rail: full labels and 390px-safe short ones */
export const RAIL_LABEL: Record<Rail, { full: string; short: string; title: string }> = {
  context: { full: 'CONTEXT', short: 'CTX', title: 'Context retrieval: authorized sources and receipts' },
  validate: { full: 'VALIDATE', short: 'VAL', title: 'Validation: stale source flagged, re-fetched, context ready' },
  policy: { full: 'POLICY', short: 'POL', title: 'External policy: permits this qualified scope, no per-change approval' },
  approval: { full: 'APPROVAL', short: 'APR', title: 'Human gate: approve the simulated run or stay supervised' },
  autonomous: { full: 'AUTONOMY', short: 'AUT', title: 'Autonomous cycle: understand, plan, act, verify, evidence' },
  evidence: { full: 'EVIDENCE', short: 'EVD', title: 'Settled: evidence recorded (demonstration), interactive' },
};

/** the two separate achievements, shown side by side: context readiness is not authority */
export function factsFor(s: Snapshot): { context: string; authority: string } | null {
  const v = s.vars;
  if (v.contextReadiness === 'unrequested') return null;
  const context = { unrequested: '', retrieving: 'RETRIEVING', validating: 'VALIDATING', blocked: 'BLOCKED (STALE)', ready: 'READY' }[v.contextReadiness];
  let authority = 'NOT EVALUATED';
  if (v.policyDecision === 'pending') authority = 'EVALUATING';
  else if (v.policyDecision === 'permitted') authority = 'PERMITTED BY POLICY';
  else if (v.policyDecision === 'denied') authority = 'BLOCKED BY POLICY';
  else if (v.policyDecision === 'requires_approval')
    authority = v.approvalState === 'approved_demo' ? 'APPROVED (SIMULATED)' : v.approvalState === 'declined' ? 'HELD / SUPERVISED' : 'APPROVAL REQUIRED';
  return { context, authority };
}

export const isDormantLike = (s: HeroState) => s === 'DORMANT' || s === 'REST';
