'use client';

import { useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramButton, DiagramShell, Segmented, Decision, useReducedMotion } from './kit';

type Scenario = 'pass' | 'once' | 'never' | 'reject' | 'canary';
type Max = '0' | '1' | '2' | '3';

const STAGES = [
  { id: 'author', name: 'Author', sub: 'author-agent' },
  { id: 'checks', name: 'Deterministic checks', sub: 'tests, lint, typecheck' },
  { id: 'review', name: 'Independent review', sub: 'reviewer-agent' },
  { id: 'rehearsal', name: 'Rehearsal', sub: 'disposable environment' },
  { id: 'canary', name: 'Canary', sub: 'release-pipeline' },
  { id: 'release', name: 'Release', sub: 'release-pipeline only' },
] as const;

type Outcome = 'pass' | 'fail' | 'repair' | 'escalated' | 'rolledback' | 'closed';
type Ev = { stage: number; outcome: Outcome; title: string; actor: string; evidence: string; attempt: number; branch?: 'repair' | 'escalate' | 'rollback' | 'review' };

const AUTHOR = 'author-agent';
const REVIEWER = 'reviewer-agent (a different identity from the author)';
const RELEASE = 'release-pipeline (the only identity that deploys)';

function build(s: Scenario, max: number): Ev[] {
  const out: Ev[] = [];
  let n = 0;
  const author = (title: string) => out.push({ stage: 0, outcome: 'pass', title, actor: AUTHOR, evidence: 'candidate and diff recorded by the runner', attempt: n });
  const checkFail = () => out.push({ stage: 1, outcome: 'fail', title: 'Checks fail', actor: 'verifier providers, run for the reviewer-agent identity', evidence: 'verifier findings (a reference, not content)', attempt: n });
  const checkPass = () => out.push({ stage: 1, outcome: 'pass', title: 'Checks pass', actor: 'verifier providers, run for the reviewer-agent identity', evidence: 'verifier findings, all clear (VerdictIssued.v1 aggregates the layers)', attempt: n });
  const escalate = (why: string, branch: Ev['branch'] = 'escalate', stage = 1) => out.push({ stage, outcome: 'escalated', title: why, actor: 'host and runner; a human takes over', evidence: 'ChangeEscalated.v1 with the reason', attempt: n, branch });

  author('Author implements the change');
  if (s === 'pass' || s === 'reject' || s === 'canary') checkPass();
  if (s === 'once' || s === 'never') {
    const fixes = s === 'once' ? 1 : max;
    checkFail();
    for (let k = 0; k < fixes; k++) {
      if (n >= max) break;
      n += 1;
      out.push({ stage: 0, outcome: 'repair', title: `Bounded repair, attempt ${n} of ${max}`, actor: AUTHOR, evidence: 'repair attempt recorded; the findings reference is the only new input', attempt: n, branch: 'repair' });
      if (s === 'once') checkPass();
      else checkFail();
    }
    if (s === 'never' || (s === 'once' && max === 0)) {
      escalate(max === 0 ? 'Repair budget is 0: escalate at the first failure' : `Repair budget used (${max} of ${max}): escalate, do not widen`);
      return out;
    }
  }
  if (s === 'reject') {
    out.push({ stage: 2, outcome: 'fail', title: 'Reviewer disagrees with the candidate', actor: REVIEWER, evidence: 'reviewer findings with references; a reviewer gives findings, never the final verdict', attempt: n, branch: 'review' });
    escalate('Reviewer disagreement escalates to a human', 'escalate', 2);
    return out;
  }
  out.push({ stage: 2, outcome: 'pass', title: 'Reviewer agrees', actor: REVIEWER, evidence: 'reviewer findings with references', attempt: n });
  out.push({ stage: 3, outcome: 'pass', title: 'Rehearsal passes', actor: 'verification context in a disposable environment (destroyed at its TTL)', evidence: 'rehearsal evidence: timings, plan digests', attempt: n });
  if (s === 'canary') {
    out.push({ stage: 4, outcome: 'fail', title: 'Canary regresses an SLO', actor: RELEASE, evidence: 'canary evaluation recorded', attempt: n });
    out.push({ stage: 4, outcome: 'rolledback', title: 'Rollback, never roll forward', actor: RELEASE, evidence: 'ChangeRolledBack.v1; a corrective Change is opened', attempt: n, branch: 'rollback' });
    return out;
  }
  out.push({ stage: 4, outcome: 'pass', title: 'Canary window clean', actor: RELEASE, evidence: 'canary evaluation recorded', attempt: n });
  out.push({ stage: 5, outcome: 'closed', title: 'Release complete, change closed', actor: RELEASE, evidence: 'ChangeClosed.v1, then the chain is sealed', attempt: n });
  return out;
}

const SCENARIOS: { value: Scenario; label: string }[] = [
  { value: 'pass', label: 'Pass' },
  { value: 'once', label: 'Fails once, then fixed' },
  { value: 'never', label: 'Never fixed' },
  { value: 'reject', label: 'Reviewer rejects' },
  { value: 'canary', label: 'Canary regresses' },
];

const BRANCHES = [
  { id: 'repair', name: 'Check fails', then: 'bounded repair, then re-check', glyph: '↻' },
  { id: 'budget', name: 'Attempts or budget used up', then: 'pause and escalate; never widen the grant', glyph: '‖' },
  { id: 'review', name: 'Reviewer disagrees', then: 'escalate to a human', glyph: '◆' },
  { id: 'rollback', name: 'Canary regresses', then: 'rollback under the release identity', glyph: '↶' },
] as const;

export function VerificationAndRepair() {
  const reduce = !!useReducedMotion();
  const [scenario, setScenario] = useState<Scenario>('once');
  const [max, setMax] = useState<Max>('2');
  const [i, setI] = useState(0);
  const m = Number(max);
  const evs = build(scenario, m);
  const idx = Math.min(i, evs.length - 1);
  const cur = evs[idx];
  const done = idx === evs.length - 1;
  const terminal = done && (cur.outcome === 'escalated' || cur.outcome === 'rolledback' || cur.outcome === 'closed');

  // stage state is the latest outcome at or before the current event
  const stageState = STAGES.map((_, s) => {
    let st: 'idle' | 'pass' | 'fail' | 'active' = 'idle';
    for (let k = 0; k <= idx; k++) if (evs[k].stage === s) st = evs[k].outcome === 'fail' || evs[k].outcome === 'escalated' ? 'fail' : 'pass';
    if (cur.stage === s && st !== 'fail') st = 'active';
    return st;
  });

  const activeBranch =
    cur.branch === 'repair' ? 'repair' : cur.branch === 'rollback' ? 'rollback' : cur.branch === 'review' ? 'review' : cur.branch === 'escalate' && /budget|Repair/.test(cur.title) ? 'budget' : cur.branch === 'escalate' ? 'review' : null;

  const live = `Step ${idx + 1} of ${evs.length}. ${cur.title}. Acting identity: ${cur.actor}. Repair attempts ${cur.attempt} of ${m}.${terminal ? ` Final state: ${cur.outcome === 'closed' ? 'closed' : cur.outcome === 'rolledback' ? 'rolled back' : 'escalated'}.` : ''}`;

  const finalWord = cur.outcome === 'closed' ? 'Closed' : cur.outcome === 'rolledback' ? 'Rolled back' : cur.outcome === 'escalated' ? 'Escalated' : 'In progress';

  return (
    <DiagramShell
      title="Verification and repair"
      question="When a check fails, what exactly can happen next, and what can never happen?"
      controls={
        <>
          <Segmented<Scenario> label="Scenario" value={scenario} onChange={(v) => { setScenario(v); setI(0); }} options={SCENARIOS} />
          <Segmented<Max> label="max_repair_attempts" value={max} onChange={(v) => { setMax(v); setI(0); }} options={[{ value: '0', label: '0' }, { value: '1', label: '1' }, { value: '2', label: '2 (default)' }, { value: '3', label: '3' }]} />
          <span className="flex gap-1">
            <DiagramButton onClick={() => setI(Math.max(0, idx - 1))} disabled={idx === 0}>Back</DiagramButton>
            <DiagramButton onClick={() => setI(Math.min(evs.length - 1, idx + 1))} disabled={done}>Step</DiagramButton>
            <DiagramButton onClick={() => setI(0)} disabled={idx === 0}>Reset</DiagramButton>
          </span>
        </>
      }
      legend={
        <>
          <span className="text-lp-orange">● current stage</span>
          <span className="text-nm-ok">✓ passed</span>
          <span className="text-nm-deny">✕ failed or escalated</span>
          <span className="rounded-sm bg-lp-yellow px-1 text-lp-on-yellow">◆ fixed rule</span>
        </>
      }
      explanation={
        <>
          <p>Illustrative and simulated: no change is running. The generated Playbook sets <code>max_repair_attempts: 2</code>; the field accepts 0 to 10 and the control here shows 0 to 3.</p>
          <ol>
            <li>The author-agent implements. Deterministic checks (tests, lint, typecheck) run next.</li>
            <li>A reviewer-agent, a separate identity from the author, reviews independently and returns findings.</li>
            <li>The change is rehearsed in a disposable environment, then canaried and released by the release-pipeline, the only identity that deploys.</li>
            <li>If a check fails, the change goes back to the author for a bounded repair, with the findings reference as the only new input. After <code>max_repair_attempts</code> it is escalated to a human.</li>
            <li>If a budget is used up, the change pauses and escalates; it never gets a wider grant. If the reviewer disagrees, it escalates. If the canary regresses, it rolls back.</li>
          </ol>
          <p>Invariant in every case: repair cannot widen the grant, change protected paths, or approve itself.</p>
        </>
      }
    >
      <ol className="m-0 grid list-none grid-cols-2 gap-1.5 p-0 sm:grid-cols-3 lg:grid-cols-6">
        {STAGES.map((s, n) => {
          const st = stageState[n];
          const cls = st === 'active' ? 'border-lp-orange bg-lp-orange/10' : st === 'pass' ? 'border-nm-ok/50' : st === 'fail' ? 'border-nm-deny/60' : 'border-fd-border opacity-70';
          return (
            <li key={s.id} className={cn('rounded-sm border bg-fd-background p-2', reduce ? '' : 'transition-colors', cls)}>
              <div className="flex items-center justify-between gap-1 font-mono text-[11px] text-fd-muted-foreground">
                <span>{n + 1}</span>
                <span className={st === 'pass' ? 'text-nm-ok' : st === 'fail' ? 'text-nm-deny' : st === 'active' ? 'text-lp-orange' : ''}>
                  {st === 'pass' ? '✓ passed' : st === 'fail' ? '✕ failed' : st === 'active' ? '● current' : 'pending'}
                </span>
              </div>
              <p className="m-0 mt-1 text-sm font-medium leading-tight text-fd-foreground">{s.name}</p>
              <p className="m-0 mt-0.5 font-mono text-[11px] leading-4 text-fd-muted-foreground">{s.sub}</p>
            </li>
          );
        })}
      </ol>

      <div className="mt-3 grid gap-3 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className="rounded-sm border border-fd-border bg-fd-background p-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-xs text-fd-muted-foreground">Step {idx + 1} of {evs.length}</span>
            {cur.outcome === 'escalated' ? <Decision kind="pause" /> : null}
            {cur.outcome === 'rolledback' ? <Decision kind="deny" /> : null}
            {cur.outcome === 'closed' ? <Decision kind="allow" /> : null}
            <span className="ml-auto rounded-sm border border-fd-border px-1.5 font-mono text-xs leading-5 text-fd-foreground">state: {finalWord}</span>
          </div>
          <h4 className="m-0 mt-1 text-sm font-medium text-fd-foreground">{cur.title}</h4>
          <dl className="m-0 mt-2 space-y-2 text-sm">
            <div>
              <dt className="font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Who acts</dt>
              <dd className="m-0 text-fd-foreground">{cur.actor}</dd>
            </div>
            <div>
              <dt className="font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Evidence recorded</dt>
              <dd className="m-0 text-fd-foreground">{cur.evidence}</dd>
            </div>
            <div>
              <dt className="font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Repair attempts</dt>
              <dd className="m-0 flex flex-wrap items-center gap-2 font-mono text-xs text-fd-foreground">
                <span aria-label={`${cur.attempt} of ${m}`}>attempt {cur.attempt}/{m}</span>
                <span className="flex gap-0.5" aria-hidden>
                  {Array.from({ length: m }, (_, k) => (
                    <span key={k} className={cn('h-2 w-5 rounded-sm border', k < cur.attempt ? 'border-lp-orange bg-lp-orange' : 'border-fd-border')} />
                  ))}
                  {m === 0 ? <span className="text-fd-muted-foreground">no repair allowed</span> : null}
                </span>
              </dd>
            </div>
          </dl>
        </div>

        <div className="space-y-3">
          <div>
            <p className="m-0 mb-1 font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">What can happen next</p>
            <ul className="m-0 list-none space-y-1 p-0">
              {BRANCHES.map((b) => (
                <li key={b.id} className={cn('flex gap-2 rounded-sm border px-2 py-1 text-xs', activeBranch === b.id ? 'border-lp-orange bg-lp-orange/10' : 'border-fd-border')}>
                  <span aria-hidden className="font-mono text-fd-foreground">{b.glyph}</span>
                  <span className="text-fd-foreground"><strong className="font-medium">{b.name}:</strong> {b.then}</span>
                </li>
              ))}
            </ul>
          </div>
          <div className="rounded-sm border border-lp-yellow bg-lp-yellow/15 p-2">
            <p className="m-0 font-mono text-[11px] uppercase tracking-wider text-fd-foreground">◆ Never, at any step</p>
            <p className="m-0 mt-1 text-sm text-fd-foreground">Repair cannot widen the grant, change protected paths, or approve itself. The author never reviews or deploys its own change; a human or policy accepts, not the model.</p>
          </div>
        </div>
      </div>
      <p className="sr-only" aria-live="polite" role="status">{live}</p>
    </DiagramShell>
  );
}
