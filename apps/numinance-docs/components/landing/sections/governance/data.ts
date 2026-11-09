export type ScenarioId = 'A' | 'B';
export type Flag = 'flag' | 'ok' | null;

export const ROW_LABELS = [
  'requested operation',
  'scope',
  'risk',
  'grant',
  'policy',
  'required approval',
] as const;

export interface Scenario {
  id: ScenarioId;
  action: string;
  /** selector label */
  label: string;
  short: string;
  risk: 'HIGH' | 'LOW';
  result: 'APPROVAL REQUIRED' | 'PERMITTED' | 'LEVEL TOO LOW';
  rows: readonly { v: string; f: Flag }[];
  why: readonly string[];
  log: string;
}

/** Illustrative scenarios. Headline stamps (HIGH, APPROVAL REQUIRED, PERMITTED) are display text; the risk row and notes name the real enum members RISK_CLASS_HIGH/LOW and DECISION_OUTCOME_REQUIRE_APPROVAL/ALLOW. */
export const SCENARIOS: Record<ScenarioId, Scenario> = {
  A: {
    id: 'A',
    action: 'Deploy payments-api to production',
    label: 'Production deployment — approval required',
    short: 'deploy to production',
    risk: 'HIGH',
    result: 'APPROVAL REQUIRED',
    rows: [
      { v: 'deploy · payments-api', f: null },
      { v: 'production · exceeds grant', f: 'flag' },
      { v: 'RISK_CLASS_HIGH', f: 'flag' },
      { v: 'no production deploy grant', f: 'flag' },
      { v: 'production change needs approver', f: null },
      { v: 'human approver · pending', f: 'flag' },
    ],
    why: [
      'scope exceeds the grant',
      'RISK_CLASS_HIGH: a human approver is required',
      'decided by policy, outside the model',
      'the model cannot override this gate',
    ],
    log: 'deploy payments-api · held · APPROVAL REQUIRED',
  },
  B: {
    id: 'B',
    action: 'Run repository type check',
    label: 'Repository check — permitted',
    short: 'type check',
    risk: 'LOW',
    result: 'PERMITTED',
    rows: [
      { v: 'run · repository type check', f: null },
      { v: 'repository workspace', f: 'ok' },
      { v: 'RISK_CLASS_LOW', f: 'ok' },
      { v: 'check runs · matched', f: 'ok' },
      { v: 'low-risk check · allow', f: 'ok' },
      { v: 'none required', f: 'ok' },
    ],
    why: [
      'scope is within the grant',
      'RISK_CLASS_LOW: no approval required',
      'the tool ran inside the execution bay',
      'the result was recorded to the log',
    ],
    log: 'run type check · exit ok · PERMITTED',
  },
};

/** Variant of scenario B when the requested level resolves below what the check needs (selected on the ladder). */
export const B_LOW: Scenario = {
  ...SCENARIOS.B,
  result: 'LEVEL TOO LOW',
  rows: [
    { v: 'run · repository type check', f: null },
    { v: 'repository workspace', f: 'ok' },
    { v: 'RISK_CLASS_LOW', f: 'ok' },
    { v: 'check runs · matched', f: 'ok' },
    { v: 'effective level below IMPLEMENT', f: 'flag' },
    { v: 'raise the level (a human decision)', f: 'flag' },
  ],
  why: [
    'the effective level is below what the check needs',
    'a requested level never lifts a floor',
    'decided by policy, outside the model',
    'nothing ran in the execution bay',
  ],
  log: 'run type check · held · LEVEL TOO LOW',
};

/** Phase clock. 0 idle · 1 approach · 2..7 readouts · 8 verdict · 9 explain / pass · 10 settled. */
export const LAST = 10;
export const DELAYS: readonly number[] = [250, 1050, 560, 560, 560, 560, 560, 560, 900, 1300];

export const START = 0.22;
export const HOLD = 0.41;
export const PASS = 0.93;
export const START_M = 0.2;
export const PASS_M = 0.86;

export function statusFor(s: ScenarioId, phase: number): string | null {
  if (phase <= 0) return null;
  if (phase === 1) return 'gate: request received · evaluating';
  if (phase <= 7) return `gate: reading ${ROW_LABELS[phase - 2]}`;
  if (s === 'A') return phase === 8 ? 'gate: grant missing · held' : phase === 9 ? 'gate: approval required · ticket held' : 'gate: held · awaiting human approver';
  return phase === 8 ? 'gate: policy allows · lifting' : phase === 9 ? 'gate: ticket passed · tool bay active' : 'gate: recorded · permitted';
}

export const DUTIES = [
  { id: 'author', label: 'author', note: 'proposes the change' },
  { id: 'reviewer', label: 'independent reviewer', note: 'not the author' },
  { id: 'release', label: 'release identity', note: 'the only identity that deploys' },
  { id: 'approver', label: 'human approver', note: 'approves; accepts autonomy promotion' },
] as const;

/** The eight real autonomy levels, ordered (AUTONOMY_LEVEL_OBSERVE … _OPERATE). UNSPECIFIED is never a level and is never shown. */
export const LEVELS = [
  { id: 'OBSERVE', note: 'reads only' },
  { id: 'SUGGEST', note: 'proposes, writes nothing' },
  { id: 'IMPLEMENT', note: 'writes in a sandbox' },
  { id: 'PR', note: 'opens pull requests' },
  { id: 'MERGE', note: 'merges when policy allows' },
  { id: 'STAGE', note: 'deploys to pre-production' },
  { id: 'RELEASE', note: 'deploys to production' },
  { id: 'OPERATE', note: 'operates against production' },
] as const;
export type LevelId = (typeof LEVELS)[number]['id'];

/**
 * Illustrative resolution per scenario, following the AutonomyPolicy rule: effective level = the lowest matching rule
 * level (else the default), then no higher than the lowest max_level of the matching floors.
 * The design default floor (when none is set) is SUGGEST for HIGH and PRIVILEGED risk and PR for the rest; it is shown as a
 * design default, not as active product behaviour. Configuration never lifts a floor or the host sandbox ceiling.
 * A scoped grant is a separate check: a level never stands in for it.
 */
export interface Ladder {
  configured: LevelId;
  configuredNote: string;
  cap: LevelId;
  capNote: string;
  effective: LevelId;
  needs: LevelId;
  grant: string;
  grantOk: boolean;
}
export const LADDERS: Record<ScenarioId, Ladder> = {
  A: {
    configured: 'RELEASE',
    configuredNote: 'illustrative operator rule',
    cap: 'SUGGEST',
    capNote: 'floor · RISK_CLASS_HIGH · design default',
    effective: 'SUGGEST',
    needs: 'RELEASE',
    grant: 'none for production deploy',
    grantOk: false,
  },
  B: {
    configured: 'PR',
    configuredNote: 'illustrative org default',
    cap: 'PR',
    capNote: 'floor · low risk · design default',
    effective: 'PR',
    needs: 'IMPLEMENT',
    grant: 'repository check · matched',
    grantOk: true,
  },
};
export const levelIndex = (id: LevelId) => LEVELS.findIndex((l) => l.id === id);

/** effective level = the requested level, never above the floor (cap) */
export function resolveLevel(sid: ScenarioId, requested: LevelId): { effective: LevelId; permitted: boolean } {
  const L = LADDERS[sid];
  const effective = LEVELS[Math.min(levelIndex(requested), levelIndex(L.cap))].id;
  return { effective, permitted: sid === 'B' && levelIndex(effective) >= levelIndex(L.needs) };
}

/** phase at which each marker appears (phase 3 reads scope, 4 risk, 5 grant, 8 verdict) */
export const LADDER_AT = { configured: 3, cap: 4, grant: 5, effective: 8 } as const;
