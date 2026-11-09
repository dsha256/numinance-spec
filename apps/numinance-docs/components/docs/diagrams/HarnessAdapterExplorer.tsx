'use client';

import { useState } from 'react';
import { cn } from '@/lib/cn';
import { StatusBadge } from '../status';
import { Decision, DiagramButton, DiagramShell, Segmented, useReducedMotion, useStepper } from './kit';

type Exec = 'fake' | 'claude-cli' | 'codex-cli' | 'reviewer';
type Scenario = 'allow' | 'deny';

/** Sources: authorexecutor port (proto), ExecutorManifest example (claude-cli-bedrock.yaml), executor layer design. Adapters are designed, not shipped. */
const EXECS: Record<Exec, { label: string; harness: string; how: string; caps: string[]; capNote: string; write: boolean }> = {
  fake: {
    label: 'fake',
    harness: 'In-process deterministic executor',
    how: 'Applies scripted diffs and emits the canonical event stream. Used to drive the lifecycle in tests.',
    caps: [],
    capNote: 'No example manifest is published in the spec; the capabilities come from the manifest the router pins.',
    write: true,
  },
  'claude-cli': {
    label: 'claude-cli',
    harness: 'Adapter supervising a pinned Claude Code CLI',
    how: 'A Go adapter drives the harness over its own JSON-lines protocol inside the work-unit sandbox; the harness keeps its own planning and tools.',
    caps: ['FILESYSTEM_READ', 'FILESYSTEM_WRITE', 'SHELL', 'SUBAGENTS', 'MCP', 'STRUCTURED_PLAN', 'APPROVAL_CALLBACKS'],
    capNote: 'From the published example ExecutorManifest claude-cli-bedrock.',
    write: true,
  },
  'codex-cli': {
    label: 'codex-cli',
    harness: 'Adapter supervising a pinned Codex CLI',
    how: 'A Go adapter drives the harness with a per-run configuration inside the sandbox; the application key never enters it.',
    caps: [],
    capNote: 'No example manifest is published in the spec; the capabilities come from the manifest the router pins.',
    write: true,
  },
  reviewer: {
    label: 'separate-vendor reviewer (read-only)',
    harness: 'Read-only reviewer loop on a different model vendor',
    how: 'Runs as the reviewer-agent identity. Its request carries no allowed_paths and no credential_capabilities; the host builds no candidate from it.',
    caps: ['FILESYSTEM_READ'],
    capNote: 'allowed_capabilities may not include FILESYSTEM_WRITE, SHELL or NETWORK for a reviewer-agent run.',
    write: false,
  },
};

const FRAME = [
  ['policy', 'Policy decision', 'evaluated outside the model before every privileged action'],
  ['grant', 'Grant', 'capabilities, allowed and denied paths, budget'],
  ['broker', 'Credential broker', 'hands out handles, never key material'],
  ['proxy', 'Egress proxy', 'only the allowed destinations'],
  ['protected', 'Protected paths', 'policy, CI definitions, IAM, workflow files'],
  ['identity', 'Review and release identity', 'no self-approval; only the release pipeline deploys'],
] as const;

type Step = { ev: string; by: 'harness' | 'host'; active: string[]; text: (e: Exec, s: Scenario, tool: string) => string };
const STEPS: Step[] = [
  { ev: 'TOOL_INTENT', by: 'harness', active: ['middle'], text: (_e, _s, t) => `The harness reports it is about to ${t}. This is a claim by the provider; it authorizes nothing.` },
  {
    ev: 'POLICY_DECISION', by: 'host', active: ['policy', 'grant'],
    text: (e, s) =>
      s === 'allow'
        ? `The host evaluates the intent against the resolved grant. ${e === 'reviewer' ? 'FILESYSTEM_READ is in allowed_capabilities.' : 'The path lies inside allowed_paths.'}`
        : e === 'reviewer'
          ? 'The host evaluates the intent against the resolved grant. A reviewer-agent run has no write capability and no allowed_paths.'
          : 'The host evaluates the intent against the resolved grant. The path is a protected resource, outside every grant whatever allowed_paths says.',
  },
  { ev: 'TOOL_ALLOWED | TOOL_DENIED', by: 'host', active: ['broker', 'protected', 'proxy'], text: (_e, s) => (s === 'allow' ? 'TOOL_ALLOWED echoes the broker allow. The echo authorizes nothing; the sandbox and broker already enforce the grant.' : 'TOOL_DENIED echoes the denial. The sandbox, broker and protected-path check refuse it regardless of what the harness does next.') },
  { ev: 'TOOL_RESULT', by: 'harness', active: ['middle'], text: (_e, s) => (s === 'allow' ? 'The harness reports the result and continues. The final candidate is still the host\'s own diff of the worktree, checked again against protected paths.' : 'The harness reports a refused call. No file changed; the denial is recorded by the host as evidence.') },
];

export function HarnessAdapterExplorer() {
  const [exec, setExec] = useState<Exec>('claude-cli');
  const [scn, setScn] = useState<Scenario>('deny');
  const st = useStepper(STEPS.length);
  const reduced = useReducedMotion();
  const x = EXECS[exec];
  const tool = scn === 'allow' ? (x.write ? 'write src/parser.go (inside allowed_paths)' : 'read src/parser.go') : x.write ? 'write .github/workflows/release.yml (a protected path)' : 'write src/parser.go';
  const step = STEPS[st.i];
  const verdict = scn === 'allow' ? 'allow' : 'deny';
  const evName = st.i === 2 ? (scn === 'allow' ? 'TOOL_ALLOWED' : 'TOOL_DENIED') : step.ev;
  const tr = reduced ? '' : 'transition-colors duration-300';

  return (
    <DiagramShell
      title="Harness adapter explorer"
      question="When the executor changes, what stays fixed?"
      controls={
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
          <Segmented label="Executor" value={exec} onChange={setExec} options={(Object.keys(EXECS) as Exec[]).map((k) => ({ value: k, label: EXECS[k].label }))} />
          <Segmented label="Tool call" value={scn} onChange={(v) => { setScn(v); st.reset(); }} options={[{ value: 'allow', label: 'allowed example' }, { value: 'deny', label: 'denied example' }]} />
        </div>
      }
      legend={
        <>
          <span><span className="rounded-sm bg-lp-yellow px-1 text-lp-on-yellow">■</span> fixed Numinance frame</span>
          <span><span className="text-nm-accent">◇</span> swappable harness layer</span>
          <span><span className="text-lp-orange">●</span> acting at this step</span>
          <span>Telemetry is conceptual, bound to the canonical event stream</span>
        </>
      }
      explanation={
        <>
          <p>The authorexecutor port drives one coding harness for one run of one work unit inside a sandbox. Four executors are shown: <code>fake</code>, <code>claude-cli</code>, <code>codex-cli</code> and a read-only reviewer on a different model vendor. All adapters are designed and under development; the spec publishes the port, the event stream, the conformance case list and one example ExecutorManifest (claude-cli-bedrock), not the adapters.</p>
          <p>Fixed whichever executor runs: the policy decision, the grant, the credential broker, the egress proxy, the protected paths, and the review and release identities. What swaps is the harness layer and the capability manifest the router pins. Nothing a request says is advisory, and nothing a provider reports is authority.</p>
          <p>The sequence is conceptual. The harness reports <code>TOOL_INTENT</code>; the host takes a policy decision (a host record, not a provider event); <code>TOOL_ALLOWED</code> or <code>TOOL_DENIED</code> echoes the broker; <code>TOOL_RESULT</code> reports the outcome. In the denied example a write to a protected workflow file is refused for every write-capable executor. For the reviewer, any write is refused because its run has no write capability and no allowed paths.</p>
          <p>The harness owns intelligence. Numinance owns authority. A compromised model can only attempt what its grant allows, and every attempt is decided and recorded outside it. Illustrative and simulated; nothing is connected to a running engine.</p>
        </>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
        <div className="min-w-0">
          {/* Fixed frame wrapping the swappable layer */}
          <div className="rounded-sm border-2 border-lp-yellow bg-lp-yellow/10 p-2 sm:p-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2 font-mono text-xs">
              <span className="rounded-sm bg-lp-yellow px-1.5 py-0.5 text-lp-on-yellow">Numinance frame · identical for every executor</span>
              <span className="text-fd-muted-foreground">authority</span>
            </div>
            <ul className="m-0 grid list-none grid-cols-1 gap-1.5 p-0 sm:grid-cols-2">
              {FRAME.map(([id, name, sub]) => {
                const on = step.active.includes(id);
                const denyHere = id === 'protected' && st.i === 2 && scn === 'deny';
                return (
                  <li key={id} className={cn('rounded-sm border bg-fd-card px-2 py-1.5', tr, on ? (denyHere ? 'border-nm-deny' : 'border-lp-orange') : 'border-fd-border')}>
                    <div className="flex items-center gap-1.5 font-mono text-xs">
                      <span aria-hidden>{on ? '●' : '○'}</span>
                      <span className="font-semibold">{name}</span>
                      {on ? <span className="sr-only">(active in this step)</span> : null}
                    </div>
                    <div className="text-xs leading-snug text-fd-muted-foreground">{sub}</div>
                  </li>
                );
              })}
            </ul>

            <div className={cn('mt-2 rounded-sm border border-dashed bg-fd-card p-2 sm:p-3', tr, step.active.includes('middle') ? 'border-lp-orange' : 'border-nm-accent/60')}>
              <div className="mb-1 flex flex-wrap items-center gap-2 font-mono text-xs">
                <span className="text-nm-accent" aria-hidden>◇</span>
                <span className="uppercase tracking-wider text-nm-accent">Harness layer · swaps</span>
                <StatusBadge kind="designed" label="Adapter: designed" />
              </div>
              <div className="text-sm font-semibold" key={exec}>{x.harness}</div>
              <p className="m-0 mt-1 text-sm leading-relaxed text-fd-muted-foreground">{x.how}</p>
              <div className="mt-2 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Capability manifest</div>
              <div className="mt-1 flex flex-wrap gap-1">
                {x.caps.length ? x.caps.map((c) => (
                  <span key={c} className="rounded-sm border border-nm-accent/50 px-1.5 py-0.5 font-mono text-xs text-nm-accent">EXECUTOR_CAPABILITY_{c}</span>
                )) : <span className="font-mono text-xs text-fd-muted-foreground">declared by the pinned ExecutorManifest</span>}
              </div>
              <p className="m-0 mt-1 flex flex-wrap items-center gap-2 text-xs text-fd-muted-foreground">
                {exec === 'claude-cli' ? <StatusBadge kind="public-spec" label="Example manifest: public spec" /> : null}
                {x.capNote}
              </p>
              <div className="mt-2 rounded-sm border border-fd-border bg-fd-background px-2 py-1.5 font-mono text-xs text-fd-muted-foreground">
                <span aria-hidden>▸ </span>model: untrusted proposer. Gains nothing beyond the grant.
              </div>
            </div>
          </div>
          <p className="m-0 mt-2 font-mono text-xs text-fd-muted-foreground">Frame unchanged: 6 of 6. Swapped: harness layer, capability manifest, model route.</p>
        </div>

        <div className="min-w-0 space-y-3">
          <div className="rounded-sm border border-fd-border bg-fd-background p-2 sm:p-3">
            <div className="mb-2 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Conceptual telemetry · bound to the canonical event stream</div>
            <div className="mb-2 rounded-sm border border-fd-border px-2 py-1 font-mono text-xs">tool call: {tool}</div>
            <ol className="m-0 list-none space-y-1.5 p-0">
              {STEPS.map((s, i) => {
                const name = i === 2 ? (scn === 'allow' ? 'TOOL_ALLOWED' : 'TOOL_DENIED') : s.ev;
                const done = i <= st.i;
                return (
                  <li key={s.ev} aria-current={i === st.i ? 'step' : undefined} className={cn('flex min-h-9 flex-wrap items-center gap-x-2 gap-y-1 rounded-sm border px-2 py-1.5 font-mono text-xs', tr, i === st.i ? 'border-lp-orange bg-lp-orange/10' : 'border-fd-border', !done && 'text-fd-muted-foreground opacity-60')}>
                    <span aria-hidden>{i + 1}</span>
                    <span className="font-semibold">{name}</span>
                    <span className="text-fd-muted-foreground">{s.by === 'host' ? 'host-recorded' : 'harness claim'}</span>
                    {i === 2 && done ? <span className="ml-auto"><Decision kind={verdict} /></span> : null}
                  </li>
                );
              })}
            </ol>
            <div className="mt-2 flex flex-wrap gap-2">
              <DiagramButton onClick={st.prev} disabled={st.first}>Back</DiagramButton>
              <DiagramButton onClick={st.next} disabled={st.last}>Next step</DiagramButton>
              <DiagramButton onClick={st.reset} disabled={st.first}>Reset</DiagramButton>
            </div>
          </div>

          <div role="status" aria-live="polite" className="rounded-sm border border-lp-orange/60 bg-lp-orange/5 px-3 py-2 text-sm leading-relaxed">
            <span className="font-mono text-xs uppercase tracking-wider text-lp-orange">Step {st.i + 1} of {STEPS.length} · {evName}</span>
            <p className="m-0 mt-1">{step.text(exec, scn, tool)}</p>
          </div>

          <div className="rounded-sm border border-lp-yellow/70 bg-lp-yellow/15 px-3 py-2 text-sm leading-relaxed">
            <b>The harness owns intelligence. Numinance owns authority.</b> Even a compromised model gains nothing beyond its grant: each attempt is decided outside it and recorded.
          </div>
        </div>
      </div>
    </DiagramShell>
  );
}
