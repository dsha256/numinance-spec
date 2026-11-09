/**
 * WORKFLOW stages and what each region shows at each. The introduction is planned by the rig's engine (rig/engine.ts):
 * its scans bring the stages up in order, and each region's state is a pure function of the stage on show.
 */
export interface StageDef {
  id: string;
  /** chip label */
  chip: string;
  /** exact status string (status plate + status bar) */
  status: string;
}

export const STAGES: readonly StageDef[] = [
  { id: 'intake', chip: 'Intake', status: 'INTENT RECEIVED' },
  { id: 'inspection', chip: 'Inspect', status: 'CONSTRAINTS EVALUATED' },
  { id: 'execution', chip: 'Execute', status: 'ACTION DISPATCHED' },
  { id: 'evidence', chip: 'Evidence', status: 'EVIDENCE RECORDED' },
  { id: 'verification', chip: 'Verify', status: 'VERIFICATION DEMO' },
  { id: 'reset', chip: 'Reset', status: 'RESET' },
];

export type PaneState = 'idle' | 'active' | 'done';
/** region index 0 INTENT, 1 AUTHORITY, 2 EXECUTION, 3 EVIDENCE */
export function paneState(region: number, stage: number): PaneState {
  if (stage >= 5 || stage === 0) return 'idle';
  switch (region) {
    case 0:
    case 1:
      return stage === 1 ? 'active' : 'done';
    case 2:
      return stage < 2 ? 'idle' : stage === 2 ? 'active' : 'done';
    default:
      return stage < 3 ? 'idle' : 'active';
  }
}
