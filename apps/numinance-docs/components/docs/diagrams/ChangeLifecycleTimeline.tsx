'use client';

import { useEffect, useId, useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramShell, DiagramButton, Decision, Segmented, useStepper } from './kit';

type StageId =
  | 'understand' | 'plan' | 'implement' | 'verify' | 'repair' | 'rehearse'
  | 'release-gate' | 'merge' | 'release' | 'observe' | 'recover' | 'seal';
type Scenario = 'happy' | 'repair' | 'exhausted' | 'approval' | 'denied';
type Outcome = 'ok' | 'fail' | 'pause' | 'deny' | 'escalate';

interface Stage {
  id: StageId;
  label: string;
  gate?: boolean;
  state: string;
  actor: string;
  temporal: string;
  artifacts: string;
  evidence: string;
}

const STAGES: Stage[] = [
  { id: 'understand', label: 'Understand / Classify', state: 'Opened, Understood, Classified', actor: 'engine (read-only grant)',
    temporal: 'change_id, state, context bundle artifact_ref + sha256', artifacts: 'Repository index, resolved context bundle, risk classification',
    evidence: 'Context resolved: sources used and digests' },
  { id: 'plan', label: 'Plan gate', gate: true, state: 'Planned, PlanGate', actor: 'engine plans; policy decides',
    temporal: 'plan artifact_ref + sha256, policy_decision_id, grant_id', artifacts: 'The frozen plan and its predicted touched resources',
    evidence: 'Policy decision with reasons, plan digest frozen before any write grant' },
  { id: 'implement', label: 'Implement', state: 'Implementing, Integrated', actor: 'author-agent (work-unit grant)',
    temporal: 'work_unit_id, run_id, candidate artifact_ref + sha256, status', artifacts: 'Worktree changes, diff, prompts, logs, completions',
    evidence: 'Author run completed: candidate digest, grant id, budget used' },
  { id: 'verify', label: 'Verify', state: 'Verifying', actor: 'reviewer-agent (read-only) and deterministic verifiers',
    temporal: 'verdict status, findings artifact_ref + sha256', artifacts: 'Findings bodies per layer, test output',
    evidence: 'VerdictIssued: layers, findings reference' },
  { id: 'repair', label: 'Repair (bounded)', state: 'Repairing', actor: 'engine re-enters Implementing; author-agent',
    temporal: 'attempt counter, findings artifact_ref as the only new input', artifacts: 'Findings from the failed verification',
    evidence: 'Repair attempt recorded with its number and findings digest' },
  { id: 'rehearse', label: 'Rehearse', state: 'Rehearsing', actor: 'engine with environment provider; reviewer identity',
    temporal: 'rehearsal artifact_ref + sha256, environment id, status', artifacts: 'Timings, plan digests, disposable environment logs',
    evidence: 'Rehearsal evidence: timings and plan digests' },
  { id: 'release-gate', label: 'Release gate', gate: true, state: 'ReleaseGate', actor: 'policy decides; release-pipeline identity only after allow',
    temporal: 'policy_decision_id, verdict ref, approver_principal if required', artifacts: 'Evidence list the gate consumed',
    evidence: 'Gate decision referencing the verdict, never raw findings' },
  { id: 'merge', label: 'Merge', state: 'Merging', actor: 'release-pipeline',
    temporal: 'operation_id, status, merge reference', artifacts: 'Pull request and merge result in your SCM',
    evidence: 'Merge recorded with operation id; PR-level autonomy ends here as Closed' },
  { id: 'release', label: 'Release', state: 'Releasing', actor: 'release-pipeline (the only deployer)',
    temporal: 'operation_id, deployment ref, status', artifacts: 'Deploy output and manifests in your systems',
    evidence: 'Deployment recorded under the release identity' },
  { id: 'observe', label: 'Observe', state: 'Observing', actor: 'engine evaluates canary window',
    temporal: 'canary window status, SLO result enum', artifacts: 'Telemetry readings in your sink',
    evidence: 'Canary result against declared SLOs' },
  { id: 'recover', label: 'Recover', state: 'Recovering, RolledBack', actor: 'release-pipeline (rollback)',
    temporal: 'rollback operation_id, status', artifacts: 'Rollback output; a corrective Change is opened',
    evidence: 'Rollback recorded; incident Change opened' },
  { id: 'seal', label: 'Evidence seal', state: 'Closed', actor: 'runner identity (evidence writer)',
    temporal: 'final status, evidence range', artifacts: 'The evidence sequence in the append-only store',
    evidence: 'Sequence sealed with an outcome record' },
];
const STAGE_BY_ID = Object.fromEntries(STAGES.map((s) => [s.id, s])) as Record<StageId, Stage>;

interface Step {
  stage: StageId;
  outcome: Outcome;
  note: string;
  /** Overrides of the stage defaults. */
  state?: string;
  actor?: string;
  evidence?: string;
  decision?: 'allow' | 'deny' | 'approval' | 'pause';
}

const st = (stage: StageId, note: string, o: Partial<Step> = {}): Step => ({ stage, outcome: 'ok', note, ...o });

const tail = (): Step[] => [
  st('rehearse', 'A disposable environment rehearses migrations and deployment; it is destroyed at its TTL.'),
  st('release-gate', 'Policy consumes the verdict and the evidence list and allows the release.', { decision: 'allow' }),
  st('merge', 'The release pipeline merges. At PR level the Change closes here; higher levels continue.'),
  st('release', 'Only the release pipeline deploys, and only if the autonomy level allows.'),
  st('observe', 'The canary window is evaluated against the declared SLOs.'),
  st('seal', 'The runner seals the evidence sequence and the Change closes.', { state: 'Closed' }),
];

const SCENARIOS: Record<Scenario, { label: string; blurb: string; steps: Step[] }> = {
  happy: {
    label: 'Happy path',
    blurb: 'Every gate allows and verification passes first time.',
    steps: [
      st('understand', 'Context is resolved from authorised sources and the Change is classified.'),
      st('plan', 'Policy allows the plan under the initial grant.', { decision: 'allow' }),
      st('implement', 'One work unit runs in a sandbox under a work-unit grant.'),
      st('verify', 'Deterministic checks pass; the reviewer adds findings, never a verdict.'),
      ...tail(),
    ],
  },
  repair: {
    label: 'Verification fails, then repair',
    blurb: 'A failing check feeds one bounded repair attempt, then verification passes.',
    steps: [
      st('understand', 'Context is resolved and the Change is classified.'),
      st('plan', 'Policy allows the plan.', { decision: 'allow' }),
      st('implement', 'The author-agent produces a first candidate.'),
      st('verify', 'A deterministic check fails. The findings are stored and referenced by digest.', { outcome: 'fail', state: 'Verifying (fail)' }),
      st('repair', 'Attempt 1 of 2 (the default cap). The findings reference is the only new input.', { state: 'Repairing' }),
      st('implement', 'The author-agent runs again under a fresh grant.', { state: 'Implementing (repair 1)' }),
      st('verify', 'All verification layers pass this time.'),
      ...tail(),
    ],
  },
  exhausted: {
    label: 'Repair exhausted, escalated',
    blurb: 'Two bounded repairs fail. The Change escalates to a human instead of trying forever.',
    steps: [
      st('understand', 'Context is resolved and the Change is classified.'),
      st('plan', 'Policy allows the plan.', { decision: 'allow' }),
      st('implement', 'First candidate.'),
      st('verify', 'Verification fails.', { outcome: 'fail', state: 'Verifying (fail)' }),
      st('repair', 'Attempt 1 of 2.'),
      st('implement', 'Second candidate.', { state: 'Implementing (repair 1)' }),
      st('verify', 'Verification fails again.', { outcome: 'fail', state: 'Verifying (fail)' }),
      st('repair', 'Attempt 2 of 2.', { state: 'Repairing (attempt 2)' }),
      st('implement', 'Third candidate.', { state: 'Implementing (repair 2)' }),
      st('verify', 'Verification fails a third time. The cap is reached.', { outcome: 'escalate', state: 'Escalated', actor: 'engine; waits for a human signal',
        evidence: 'Escalation recorded with the findings digest; no grant is widened', decision: 'pause' }),
    ],
  },
  approval: {
    label: 'Approval required',
    blurb: 'Policy answers require approval. The workflow waits for a human other than the author.',
    steps: [
      st('understand', 'Context is resolved and the Change is classified as higher risk.'),
      st('plan', 'Policy answers require approval. The workflow now waits on a signal with a timer (default 7 days).',
        { outcome: 'pause', decision: 'approval', state: 'PlanGate (waiting)', actor: 'policy; human approver pending',
          evidence: 'Policy decision: require approval, approver group, author excluded' }),
      st('plan', 'A human approver grants. The signal is rejected if the approver equals the author.',
        { decision: 'allow', state: 'PlanGate (approved)', actor: 'human approver',
          evidence: 'Approval record with approver principal and decision id' }),
      st('implement', 'Work proceeds under the approved grant.'),
      st('verify', 'Verification passes.'),
      ...tail(),
    ],
  },
  denied: {
    label: 'Denied by policy',
    blurb: 'The plan gate denies. The denial is recorded and the Change is abandoned with no write grant ever issued.',
    steps: [
      st('understand', 'Context is resolved and the Change is classified.'),
      st('plan', 'Policy denies the plan. Denials are evidence too.',
        { outcome: 'deny', decision: 'deny', state: 'Abandoned', actor: 'policy decision point',
          evidence: 'Denial with rule ids as reasons and the policy decision id' }),
      st('seal', 'The runner seals the sequence. No code was written.', { state: 'Abandoned (sealed)' }),
    ],
  },
};

const SCENARIO_KEYS = Object.keys(SCENARIOS) as Scenario[];

export function ChangeLifecycleTimeline() {
  const [scenario, setScenario] = useState<Scenario>('happy');
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState<'1' | '2'>('1');
  const sc = SCENARIOS[scenario];
  const n = sc.steps.length;
  const sp = useStepper(n);
  const rid = useId();
  const step = sc.steps[sp.i];
  const stage = STAGE_BY_ID[step.stage];
  const pausedHere = step.outcome === 'pause' || step.outcome === 'escalate';

  useEffect(() => {
    if (!playing) return;
    if (sp.i >= n - 1) {
      setPlaying(false);
      return;
    }
    const t = setTimeout(() => sp.set(sp.i + 1), speed === '2' ? 700 : 1400);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playing, speed, sp.i, n]);

  const visited = new Set(sc.steps.map((s) => s.stage));
  const doneUpTo = new Set(sc.steps.slice(0, sp.i).map((s) => s.stage));
  const nextStage = sp.i < n - 1 ? sc.steps[sp.i + 1].stage : null;

  const pickScenario = (v: Scenario) => {
    setPlaying(false);
    setScenario(v);
    sp.reset();
  };

  const controls = (
    <div className="flex w-full flex-col gap-2">
      <Segmented label="Scenario" value={scenario} onChange={pickScenario} options={SCENARIO_KEYS.map((k) => ({ value: k, label: SCENARIOS[k].label }))} />
      <div className="flex flex-wrap items-center gap-2">
        <DiagramButton onClick={() => { setPlaying(false); sp.prev(); }} disabled={sp.first} label="Previous step">← Prev</DiagramButton>
        <button
          type="button"
          onClick={() => { setPlaying(false); sp.next(); }}
          disabled={sp.last}
          aria-label="Next step"
          className="min-h-9 rounded-sm border border-lp-ink bg-lp-yellow px-3 py-1.5 font-mono text-xs text-lp-on-yellow disabled:cursor-not-allowed disabled:opacity-40"
        >
          Next →
        </button>
        {playing ? (
          <DiagramButton onClick={() => setPlaying(false)}>■ Stop</DiagramButton>
        ) : (
          <DiagramButton onClick={() => { if (sp.last) sp.reset(); setPlaying(true); }}>▶ Play once</DiagramButton>
        )}
        <Segmented label="Speed" value={speed} onChange={setSpeed} options={[{ value: '1', label: '1x' }, { value: '2', label: '2x' }]} />
        <label className="flex min-w-40 flex-1 items-center gap-2 font-mono text-xs text-fd-muted-foreground" htmlFor={`${rid}-r`}>
          Scrub
          <input
            id={`${rid}-r`}
            type="range"
            min={0}
            max={n - 1}
            step={1}
            value={sp.i}
            onChange={(e) => { setPlaying(false); sp.set(Number(e.target.value)); }}
            aria-label="Scrub through lifecycle steps"
            aria-valuetext={`Step ${sp.i + 1} of ${n}: ${stage.label}`}
            className="h-9 w-full min-w-24 flex-1 accent-[var(--color-lp-orange)]"
          />
          <span className="whitespace-nowrap">{sp.i + 1}/{n}</span>
        </label>
      </div>
    </div>
  );

  const legend = (
    <>
      <span className="text-lp-orange">▣ current</span>
      <span>✓ visited</span>
      <span>◇ gate</span>
      <span>┄ not on this path</span>
      <span>next action is yellow</span>
    </>
  );

  const explanation = (
    <>
      <p>
        A Change is a durable state machine, not one long agent session. The agent implements one work unit at a time; planning,
        gates, verification, release and recovery are performed by Numinance with fresh authority per stage. Orchestration payloads carry
        references and digests only; source, diffs, prompts, logs and findings bodies live in customer artifact storage.
      </p>
      <p>
        Stages in order: {STAGES.map((s) => s.label).join(', ')}. Verification failure re-enters implementation with the findings
        reference as the only new input, at most max_repair_attempts times (default 2 in the generated Playbook), then the Change is
        escalated. Plan and release gates can allow, deny or require approval; approvals come from a human other than the author.
      </p>
      {SCENARIO_KEYS.map((k) => (
        <div key={k}>
          <p><strong>{SCENARIOS[k].label}.</strong> {SCENARIOS[k].blurb}</p>
          <ol>
            {SCENARIOS[k].steps.map((s, i) => (
              <li key={i}>
                {STAGE_BY_ID[s.stage].label}: {s.note}
                {' '}Acts: {s.actor ?? STAGE_BY_ID[s.stage].actor}. Evidence: {s.evidence ?? STAGE_BY_ID[s.stage].evidence}.
              </li>
            ))}
          </ol>
        </div>
      ))}
    </>
  );

  return (
    <DiagramShell
      title="Change lifecycle"
      question="What does a durable change do from objective to evidence, and where does it pause?"
      controls={controls}
      legend={legend}
      explanation={explanation}
    >
      <ol className="m-0 grid list-none grid-cols-2 gap-1.5 p-0 min-[480px]:grid-cols-3 md:grid-cols-6">
        {STAGES.map((s) => {
          const current = s.id === step.stage;
          const on = visited.has(s.id);
          const done = doneUpTo.has(s.id) && !current;
          const isNext = s.id === nextStage && !current;
          return (
            <li
              key={s.id}
              aria-current={current ? 'step' : undefined}
              className={cn(
                'm-0 flex min-h-14 flex-col justify-between rounded-sm border px-2 py-1.5 text-xs transition-colors motion-reduce:transition-none',
                current && 'border-lp-orange bg-lp-orange/10 ring-1 ring-lp-orange',
                !current && isNext && 'border-lp-ink bg-lp-yellow text-lp-on-yellow',
                !current && !isNext && on && 'border-fd-border bg-fd-background',
                !on && 'border-dashed border-fd-border text-fd-muted-foreground opacity-70',
              )}
            >
              <span className="font-mono">
                {s.gate ? '◇ ' : ''}
                {done ? '✓ ' : ''}
                {!on ? '┄ ' : ''}
                {s.label}
              </span>
              {current && step.outcome !== 'ok' ? (
                <span className={cn('font-mono', step.outcome === 'deny' || step.outcome === 'fail' ? 'text-nm-deny' : 'text-nm-warn')}>
                  {step.outcome === 'fail' ? '✕ failed' : step.outcome === 'deny' ? '✕ denied' : step.outcome === 'pause' ? '‖ paused' : '‖ escalated'}
                </span>
              ) : null}
            </li>
          );
        })}
      </ol>

      <div className="mt-3 rounded-md border border-fd-border bg-fd-background p-3">
        <div className="flex flex-wrap items-center gap-2">
          <h4 className="m-0 text-sm font-semibold">
            Step {sp.i + 1} of {n} · {stage.label}
          </h4>
          {step.decision ? <Decision kind={step.decision} /> : null}
          {step.outcome === 'fail' ? <span className="font-mono text-xs text-nm-deny">✕ VERIFICATION FAILED</span> : null}
        </div>
        <p className="m-0 mt-1 text-sm text-fd-muted-foreground">{step.note}</p>
        {pausedHere ? (
          <p className="m-0 mt-2 rounded-sm border border-nm-warn/50 px-2 py-1 text-sm text-nm-warn">
            ‖ The workflow is waiting for a human. Nothing runs autonomously until the signal arrives.
          </p>
        ) : null}
        <dl className="m-0 mt-3 grid gap-x-4 gap-y-2 text-sm sm:grid-cols-2">
          <Field k="Lifecycle state" v={step.state ?? stage.state} mono />
          <Field k="Who acts" v={step.actor ?? stage.actor} />
          <Field k="Temporal holds (references only)" v={stage.temporal} mono />
          <Field k="Customer artifact storage holds" v={stage.artifacts} />
          <div className="sm:col-span-2">
            <Field k="Evidence record emitted" v={step.evidence ?? stage.evidence} accent />
          </div>
        </dl>
      </div>
      <p className="sr-only" aria-live="polite" role="status">
        {`${sc.label}, step ${sp.i + 1} of ${n}: ${stage.label}. ${step.note}${pausedHere ? ' Paused for a human.' : ''}`}
      </p>
    </DiagramShell>
  );
}

function Field({ k, v, mono, accent }: { k: string; v: string; mono?: boolean; accent?: boolean }) {
  return (
    <div className={cn('min-w-0 rounded-sm', accent && 'border-l-2 border-lp-orange pl-2')}>
      <dt className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">{k}</dt>
      <dd className={cn('m-0 mt-0.5 break-words', mono && 'font-mono text-xs leading-relaxed')}>{v}</dd>
    </div>
  );
}
