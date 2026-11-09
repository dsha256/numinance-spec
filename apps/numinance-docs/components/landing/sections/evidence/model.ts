export type Scenario = 'valid' | 'altered' | 'untrusted';
export type Outcome = 'verified' | 'consistent' | 'broken';
export type RecState = 'pending' | 'inspecting' | 'ok' | 'altered' | 'flagged';
export type LinkState = 'pending' | 'ok' | 'broken' | 'void';
export type VaultState = 'idle' | 'empty' | 'loaded' | 'mismatch' | 'skipped';

export const COMMAND = 'numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR';

/** Placeholder digests only: these are not real hashes. */
export const RECORDS = [
  { id: 'intent', n: '01', name: ['INTENT', 'RECORD'], digest: 'sha256:7f3a…c91e', prev: null, abbr: '01',
    fields: [['objective', 'illustrative change request'], ['requested by', 'placeholder principal'], ['authority', 'none implied by this record']] },
  { id: 'plan', n: '02', name: ['PLAN', 'RECORD'], digest: 'sha256:2b90…e47d', prev: 'sha256:7f3a…c91e', abbr: '02',
    fields: [['scope', 'bounded (illustrative)'], ['steps', 'three, in order'], ['refers to', 'intent record']] },
  { id: 'execution', n: '03', name: ['EXECUTION', 'RECORD'], digest: 'sha256:a1c4…09f2', prev: 'sha256:2b90…e47d', abbr: '03',
    fields: [['tool calls', 'four (illustrative)'], ['ran in', 'customer environment'], ['refers to', 'plan record']] },
  { id: 'artifact', n: '04', name: ['ARTIFACT', 'DIGEST'], digest: 'sha256:d83e…5b16', prev: 'sha256:a1c4…09f2', abbr: '04',
    fields: [['artifact', 'change.patch (illustrative)'], ['content digest', 'sha256:5c0d…18ab'], ['refers to', 'execution record']] },
  { id: 'result', n: '05', name: ['VERIFICATION', 'RESULT'], digest: 'sha256:4e07…ba3c', prev: 'sha256:d83e…5b16', abbr: '05',
    fields: [['checks', 'recorded as passed (illustrative)'], ['claims', 'none about authenticity'], ['refers to', 'artifact digest']] },
] as const;

export const TAMPERED = 2;
export const TAMPERED_DIGEST = 'sha256:e6d2…7710';

/** ms per tick of the verification run */
export const TICK_MS = 55;
export const TYPE_TICKS = Math.ceil(COMMAND.length / 2);
export const GAP = 6;
export const STEP_TICKS = 10;
export const STEPS = 7; // five records, trust, result
export const START = TYPE_TICKS + GAP;
export const FINAL = START + STEPS * STEP_TICKS;

export function view(tick: number) {
  const typed = Math.min(COMMAND.length, tick * 2);
  const step = tick < START ? 0 : Math.min(STEPS, Math.floor((tick - START) / STEP_TICKS) + 1);
  return { typed, step };
}

export function outcome(s: Scenario, pinned: boolean): Outcome {
  if (s === 'altered') return 'broken';
  if (s === 'valid' && pinned) return 'verified';
  return 'consistent';
}

export function recState(s: Scenario, i: number, step: number): RecState {
  if (step < i + 1) return 'pending';
  const active = step === i + 1;
  if (s === 'altered') {
    if (i < TAMPERED) return active ? 'inspecting' : 'ok';
    if (i === TAMPERED) return active ? 'inspecting' : 'altered';
    return active ? 'inspecting' : 'flagged';
  }
  return active ? 'inspecting' : 'ok';
}

/** Link i joins record i and i+1; it is evaluated when record i+1 is inspected. */
export function linkState(s: Scenario, i: number, step: number): LinkState {
  if (step < i + 2) return 'pending';
  if (s !== 'altered') return 'ok';
  if (i < TAMPERED) return 'ok';
  if (i === TAMPERED) return 'broken';
  return 'void';
}

export function vaultState(s: Scenario, pinned: boolean, step: number): VaultState {
  if (step < 6) return s === 'valid' && !pinned ? 'empty' : 'idle';
  if (s === 'altered') return 'skipped';
  if (s === 'untrusted') return 'mismatch';
  return pinned ? 'loaded' : 'empty';
}

export interface TLine {
  text: string;
  tone: 'cmd' | 'dim' | 'ok' | 'bad' | 'warn' | 'plain';
}

export function lines(s: Scenario, pinned: boolean, step: number): TLine[] {
  const out: TLine[] = [];
  const pad = (t: string) => t.padEnd(18, ' ');
  out.push({ text: '# simulated output · illustrative, not a real verifier run', tone: 'dim' });
  if (step >= 0) out.push({ text: 'reading evidence/v1 export directory …', tone: 'dim' });
  RECORDS.forEach((r, i) => {
    if (step < i + 1) return;
    const name = r.name.join(' ');
    const st = recState(s, i, step);
    const idx = `[${i + 1}/5]`;
    if (s === 'altered' && i === TAMPERED) {
      out.push({ text: `${idx} ${pad(name)} recomputed ${TAMPERED_DIGEST}`, tone: 'bad' });
      if (step > i + 1) out.push({ text: `      MISMATCH · recorded ${r.digest}`, tone: 'bad' });
    } else if (s === 'altered' && i > TAMPERED) {
      out.push({ text: `${idx} ${pad(name)} skipped · predecessor failed`, tone: 'warn' });
    } else {
      out.push({ text: `${idx} ${pad(name)} ${r.digest}${st === 'inspecting' ? ' …' : ' link ok'}`, tone: st === 'inspecting' ? 'plain' : 'ok' });
    }
  });
  if (step >= 6) {
    if (s === 'altered') out.push({ text: 'trust: not evaluated, chain is broken', tone: 'warn' });
    else if (s === 'untrusted') {
      out.push({ text: 'trust: caller-pinned keys and origin loaded', tone: 'plain' });
      out.push({ text: 'trust: signing key is not among the pinned keys', tone: 'bad' });
    } else if (pinned) {
      out.push({ text: 'trust: caller-pinned keys and origin loaded', tone: 'plain' });
      out.push({ text: 'trust: signing key matches pinned authority', tone: 'ok' });
    } else {
      out.push({ text: 'trust: no keys pinned (no built-in trust root)', tone: 'warn' });
      out.push({ text: 'trust: keys from the export never reach verified', tone: 'warn' });
    }
  }
  if (step >= 7) {
    const o = outcome(s, pinned);
    if (o === 'verified') {
      out.push({ text: 'result: level verified (simulated)', tone: 'ok' });
      out.push({ text: '# real runs: exit 0, level_enum verified, weakened_by_flags empty', tone: 'dim' });
    } else if (o === 'broken') {
      out.push({ text: 'result: level failed · digest mismatch at EXECUTION RECORD', tone: 'bad' });
      out.push({ text: '# later records cannot be relied on', tone: 'dim' });
    } else {
      out.push({ text: 'result: self-consistent at best · NOT verified', tone: 'warn' });
      out.push({ text: '# consistent is not trusted · not "verified"', tone: 'dim' });
    }
  }
  return out;
}

export const RESULT = {
  verified: {
    label: 'VERIFIED · ILLUSTRATIVE',
    body: 'The chain is structurally consistent and the simulated trust material the caller pinned establishes the expected authority. A real run reports one of seven levels (verified, failed, unauthenticated, self-consistent, incomplete, unsigned, unanchored); only pinned keys can reach verified.',
  },
  consistent: {
    label: 'CONSISTENT ≠ TRUSTED',
    body: 'The records link correctly, but trust has not been established. A key carried inside an export cannot vouch for that export, so it never reaches verified. Only runner and log keys the caller pins can.',
  },
  broken: {
    label: 'MISMATCH DETECTED',
    body: 'EXECUTION RECORD no longer hashes to the digest ARTIFACT DIGEST points at. Records after the break cannot be relied on, whatever keys are pinned.',
  },
} as const;

export function statusFor(s: Scenario, pinned: boolean, step: number): string {
  if (step <= 0) return 'verifier: ready';
  if (step <= 5) {
    const r = RECORDS[step - 1];
    const nm = r.name.join(' ').toLowerCase();
    if (s === 'altered' && step - 1 === TAMPERED) return `verifier: ${nm} digest mismatch`;
    if (s === 'altered' && step - 1 > TAMPERED) return `verifier: ${nm} skipped`;
    return `verifier: ${nm} · link ok`;
  }
  if (step === 6) return s === 'altered' ? 'verifier: trust not evaluated' : s === 'valid' && pinned ? 'verifier: trust pinned, key matches' : s === 'untrusted' ? 'verifier: signing key not pinned' : 'verifier: no trust pinned';
  const o = outcome(s, pinned);
  return o === 'verified' ? 'verifier: verified (illustrative)' : o === 'broken' ? 'verifier: mismatch detected' : 'verifier: consistent ≠ trusted';
}
