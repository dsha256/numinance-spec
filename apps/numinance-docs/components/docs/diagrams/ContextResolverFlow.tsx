'use client';

import { useEffect, useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramButton, DiagramShell, Segmented, useReducedMotion } from './kit';

type St = 'healthy' | 'stale' | 'conflicting' | 'denied' | 'drifted';
type Id = 'repo' | 'adr' | 'ticket' | 'runbook' | 'kb';

/** `conn` sources are read through a KnowledgeConnection (MCP or REST); worktree sources are not. */
const SRC: { id: Id; name: string; via: string; conn: boolean; required: boolean }[] = [
  { id: 'repo', name: 'Repository docs', via: 'worktree, human-authored', conn: false, required: false },
  { id: 'adr', name: 'ADR', via: 'worktree', conn: false, required: false },
  { id: 'ticket', name: 'Ticket', via: 'KnowledgeConnection', conn: true, required: true },
  { id: 'runbook', name: 'Runbook', via: 'KnowledgeConnection', conn: true, required: false },
  { id: 'kb', name: 'Knowledge connection', via: 'MCP or REST search', conn: true, required: false },
];
const STATES: Record<St, string> = { healthy: 'Healthy', stale: 'Stale', conflicting: 'Conflicting', denied: 'Inaccessible (denied)', drifted: 'Tool inventory drifted (quarantined)' };
const ALL_OK: Record<Id, St> = { repo: 'healthy', adr: 'healthy', ticket: 'healthy', runbook: 'healthy', kb: 'healthy' };
const PRESETS: { label: string; v: Record<Id, St> }[] = [
  { label: 'All healthy', v: ALL_OK },
  { label: 'Stale runbook', v: { ...ALL_OK, runbook: 'stale' } },
  { label: 'ADR conflicts', v: { ...ALL_OK, adr: 'conflicting' } },
  { label: 'Ticket denied', v: { ...ALL_OK, ticket: 'denied' } },
  { label: 'Connection drifted', v: { ...ALL_OK, kb: 'drifted' } },
];

const STAGES = [
  ['Authorized sources', 'Only sources the connection scope and the principal can reach.'],
  ['Search and fetch', 'Every call runs under the principal\'s authorization (authz_context_digest). Denied sources and quarantined connections drop out here.'],
  ['Deduplicate', 'Identical content collapses by digest.'],
  ['Redact', 'Secrets and PII spans are redacted before persistence; BundleSource.redactions keeps the count.'],
  ['Freshness check', 'A newer source version or an age past a threshold marks SOURCE_STALENESS_STALE.'],
  ['Conflict analysis', 'Sources that disagree become a BundleConflict with every claim; never resolved silently.'],
  ['ContextBundle', 'Cited guidance and unresolved questions, stored by digest and never mutated.'],
] as const;

type Q = { text: string; blocking: boolean };
function compute(v: Record<Id, St>) {
  const fetched = SRC.filter((s) => v[s.id] !== 'denied' && v[s.id] !== 'drifted');
  const dropped = SRC.length - fetched.length;
  const stale = fetched.filter((s) => v[s.id] === 'stale');
  const conflicts = fetched.filter((s) => v[s.id] === 'conflicting');
  const qs: Q[] = [];
  for (const s of SRC) {
    const st = v[s.id];
    if (st === 'drifted') qs.push({ blocking: true, text: `${s.name}: tool inventory drifted from the pinned hashes; the connection is quarantined and needs re-approval.` });
    if (st === 'denied') qs.push({ blocking: s.required, text: s.required ? `${s.name}: not readable under this principal, and the task cannot be understood without it.` : `${s.name}: not readable under this principal; its guidance is missing.` });
    if (st === 'stale') qs.push({ blocking: false, text: `${s.name}: a newer version exists or it is past the age threshold; is it still current?` });
    if (st === 'conflicting') {
      const peer = fetched.find((p) => p.id !== s.id && v[p.id] !== 'conflicting');
      qs.push({ blocking: true, text: `${s.name} and ${peer ? peer.name : 'another source'} disagree; which claim binds?` });
    }
  }
  const blocking = qs.some((q) => q.blocking);
  const outcome: 'proceed' | 'open' | 'stop' = blocking ? 'stop' : qs.length ? 'open' : 'proceed';
  // Counts leaving each stage (dedupe and redact pass sources through; they change content, not the count).
  const out = [SRC.length, fetched.length, fetched.length, fetched.length, fetched.length, fetched.length, fetched.length];
  return { fetched, dropped, stale, conflicts, qs, outcome, out };
}

const OUT = {
  proceed: { glyph: '✓', word: 'Proceed', cls: 'border-nm-ok text-nm-ok', why: 'No unresolved questions, no conflict, nothing dropped. The bundle goes to the executor.' },
  open: { glyph: '◐', word: 'Proceed with open questions', cls: 'border-nm-warn text-nm-warn', why: 'Only non-blocking questions remain. The bundle goes to the executor with those questions attached and the stale sources labelled.' },
  stop: { glyph: '■', word: 'Stop and escalate', cls: 'border-nm-deny text-nm-deny', why: 'At least one question is blocking. The system does not guess; the blocking question is posted back for a human and the change does not proceed on this bundle.' },
} as const;

export function ContextResolverFlow() {
  const [v, setV] = useState<Record<Id, St>>(PRESETS[1].v);
  const [reveal, setReveal] = useState<number>(STAGES.length); // SSR and first paint show the finished flow
  const [running, setRunning] = useState(false);
  const reduced = useReducedMotion();
  const r = compute(v);
  const o = OUT[r.outcome];
  const preset = PRESETS.find((p) => SRC.every((s) => p.v[s.id] === v[s.id]))?.label;

  useEffect(() => {
    if (!running) return;
    if (reveal >= STAGES.length) {
      setRunning(false);
      return;
    }
    const t = setTimeout(() => setReveal((n) => n + 1), 650);
    return () => clearTimeout(t);
  }, [running, reveal]);

  const run = () => {
    if (reduced) return;
    setReveal(1);
    setRunning(true);
  };
  const stop = () => {
    setRunning(false);
    setReveal(STAGES.length);
  };
  const tr = reduced ? '' : 'transition-all duration-300';
  const live = `Outcome: ${o.word}. ${r.fetched.length} of ${SRC.length} sources reached the bundle; ${r.qs.filter((q) => q.blocking).length} blocking and ${r.qs.filter((q) => !q.blocking).length} non-blocking unresolved questions; ${r.conflicts.length} conflicts.`;

  return (
    <DiagramShell
      title="Context resolver flow"
      question="How does context get acquired, and when does the system stop instead of guessing?"
      controls={
        <>
          <div role="group" aria-label="Scenario" className="flex flex-wrap items-center gap-1">
            <span className="mr-1 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Scenario</span>
            {PRESETS.map((p) => <DiagramButton key={p.label} pressed={preset === p.label} onClick={() => { setRunning(false); setReveal(STAGES.length); setV(p.v); }}>{p.label}</DiagramButton>)}
          </div>
          <span className="mx-1 hidden h-5 border-l border-fd-border sm:block" aria-hidden />
          {running ? <DiagramButton onClick={stop}>Stop</DiagramButton> : <DiagramButton onClick={run} disabled={reduced === true}>Run once</DiagramButton>}
        </>
      }
      legend={
        <>
          <span><span className="text-nm-ok">✓</span> proceed</span>
          <span><span className="text-nm-warn">◐</span> proceed with open questions</span>
          <span><span className="text-nm-deny">■</span> stop and escalate</span>
          <span><span className="text-nm-accent">◇</span> typed field of ContextBundle</span>
        </>
      }
      explanation={
        <>
          <p>The resolver builds a small <code>ContextBundle</code> for one stage of one work unit. Stages: authorized sources; search and fetch under the principal&apos;s authorization; deduplicate by digest; redact secrets and PII; check freshness; analyse conflicts; emit the bundle.</p>
          <p>The bundle has <code>sources[]</code> (BundleSource, with <code>staleness</code>, <code>access_basis</code>, <code>trust</code>), <code>cited_guidance[]</code> (CitedGuidance: <code>ref</code>, optional <code>section</code>), <code>conflicts[]</code> (BundleConflict), <code>unresolved_questions[]</code> (UnresolvedQuestion: <code>question</code>, <code>blocking</code>) and <code>pinned_toolsets</code>. Every source is <code>TRUST_CLASS_UNTRUSTED_RETRIEVED</code>.</p>
          <p>Outcome rules used by this simulation (illustrative; the real bundle records the questions and the profile decides what blocks):</p>
          <ol>
            <li>A drifted tool inventory quarantines its connection. Nothing is read from it, and a blocking question is raised.</li>
            <li>A denied source is dropped. If it is the ticket, which carries the task, the question is blocking; otherwise it is open.</li>
            <li>A stale source stays in the bundle, labelled stale, with an open question.</li>
            <li>Conflicting sources produce a BundleConflict with both claims and a blocking question, because the resolver may not choose silently.</li>
            <li>Any blocking question gives Stop and escalate. Otherwise any open question gives Proceed with open questions. Otherwise Proceed.</li>
          </ol>
          <p>Retrieved text is untrusted guidance and cannot create policy. There is no copy of the whole knowledge base and no mandatory vector database. The model never gets direct enterprise MCP access; the executor sees the bundle only. Simulated; nothing is connected to a running engine.</p>
        </>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div className="min-w-0 space-y-3">
          <div className="rounded-sm border border-fd-border bg-fd-background p-2 sm:p-3">
            <div className="mb-2 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Source state</div>
            <ul className="m-0 list-none space-y-2 p-0">
              {SRC.map((s) => (
                <li key={s.id} className="border-b border-fd-border/60 pb-2 last:border-b-0 last:pb-0">
                  <Segmented
                    label={`${s.name} · ${s.via}`}
                    value={v[s.id]}
                    onChange={(x) => { setRunning(false); setReveal(STAGES.length); setV({ ...v, [s.id]: x }); }}
                    options={(Object.keys(STATES) as St[]).filter((k) => s.conn || (k !== 'denied' && k !== 'drifted')).map((k) => ({ value: k, label: STATES[k] }))}
                  />
                </li>
              ))}
            </ul>
          </div>

          <ol className="m-0 list-none space-y-0 p-0">
            {STAGES.map(([name, note], i) => {
              const shown = i < reveal;
              const here = running && i === reveal - 1;
              const cut = i > 0 && r.out[i] < r.out[i - 1];
              return (
                <li key={name} className={cn('relative border-l-2 pb-2 pl-4', tr, shown ? (here ? 'border-lp-orange' : 'border-fd-border') : 'border-fd-border/40 opacity-40')}>
                  <span aria-hidden className={cn('absolute -left-[7px] top-1 size-3 rounded-full border-2 bg-fd-card', here ? 'border-lp-orange bg-lp-orange' : 'border-fd-border')} />
                  <div className="flex flex-wrap items-baseline gap-x-2 font-mono text-xs">
                    <span className="font-semibold">{i + 1} · {name}</span>
                    <span className={cn(cut ? 'text-nm-deny' : 'text-fd-muted-foreground')}>{r.out[i]} of {SRC.length} sources{cut ? ` (${r.out[i - 1] - r.out[i]} dropped)` : ''}</span>
                    {i === 4 && r.stale.length ? <span className="text-nm-warn">{r.stale.length} stale</span> : null}
                    {i === 5 && r.conflicts.length ? <span className="text-nm-deny">{r.conflicts.length} in conflict</span> : null}
                  </div>
                  <p className="m-0 text-xs leading-snug text-fd-muted-foreground">{note}</p>
                </li>
              );
            })}
          </ol>
        </div>

        <div className="min-w-0 space-y-3">
          <div role="status" aria-live="polite" className={cn('rounded-sm border-2 px-3 py-2', o.cls)}>
            <div className="flex items-center gap-2 font-mono text-sm font-semibold uppercase tracking-wide"><span aria-hidden>{o.glyph}</span>{o.word}</div>
            <p className="m-0 mt-1 text-sm leading-relaxed text-fd-foreground">{o.why}</p>
            <span className="sr-only">{live}</span>
          </div>

          <div className={cn('rounded-sm border border-fd-border bg-fd-background p-2 sm:p-3', tr, reveal < STAGES.length && 'opacity-40')}>
            <div className="mb-2 font-mono text-xs uppercase tracking-wider text-nm-accent"><span aria-hidden>◇ </span>ContextBundle · spec</div>
            <dl className="m-0 space-y-2 text-sm">
              <div>
                <dt className="font-mono text-xs text-fd-muted-foreground">sources[] · BundleSource</dt>
                <dd className="m-0">
                  {r.fetched.length ? (
                    <ul className="m-0 list-none p-0">
                      {r.fetched.map((s) => (
                        <li key={s.id} className="flex flex-wrap gap-x-2 font-mono text-xs">
                          <span>{s.name}</span>
                          <span className={v[s.id] === 'stale' ? 'text-nm-warn' : 'text-fd-muted-foreground'}>staleness: {v[s.id] === 'stale' ? 'STALE' : 'FRESH'}</span>
                          <span className="text-fd-muted-foreground">trust: UNTRUSTED_RETRIEVED</span>
                        </li>
                      ))}
                    </ul>
                  ) : <span className="font-mono text-xs text-nm-deny">none</span>}
                </dd>
              </div>
              <div>
                <dt className="font-mono text-xs text-fd-muted-foreground">cited_guidance[] · CitedGuidance {'{'}ref, section{'}'}</dt>
                <dd className="m-0 font-mono text-xs">{r.fetched.filter((s) => v[s.id] !== 'conflicting').length} cited · advisory only, untrusted</dd>
              </div>
              <div>
                <dt className="font-mono text-xs text-fd-muted-foreground">conflicts[] · BundleConflict</dt>
                <dd className="m-0 font-mono text-xs">{r.conflicts.length ? `${r.conflicts.length} with every claim kept, none silently resolved` : 'none'}</dd>
              </div>
              <div>
                <dt className="font-mono text-xs text-fd-muted-foreground">unresolved_questions[] · UnresolvedQuestion {'{'}question, blocking{'}'}</dt>
                <dd className="m-0">
                  {r.qs.length ? (
                    <ul className="m-0 list-none space-y-1 p-0">
                      {r.qs.map((q, i) => (
                        <li key={i} className="text-xs leading-snug">
                          <span className={cn('mr-1.5 rounded-sm border px-1 font-mono', q.blocking ? 'border-nm-deny/60 text-nm-deny' : 'border-nm-warn/60 text-nm-warn')}>{q.blocking ? '■ blocking: true' : '◐ blocking: false'}</span>
                          {q.text}
                        </li>
                      ))}
                    </ul>
                  ) : <span className="font-mono text-xs">none</span>}
                </dd>
              </div>
            </dl>
          </div>

          <ul className="m-0 list-none space-y-1.5 p-0 text-sm leading-relaxed">
            <li className="rounded-sm border border-lp-yellow/70 bg-lp-yellow/15 px-3 py-2"><b>Retrieved text is untrusted guidance.</b> It can inform a plan; it cannot create policy, widen a grant or change a tool list.</li>
            <li className="rounded-sm border border-fd-border px-3 py-2"><b>No copy of the knowledge base.</b> Only what one change needs is fetched, by reference and digest. No mandatory vector database.</li>
            <li className="rounded-sm border border-fd-border px-3 py-2"><b>The model never gets direct enterprise MCP access.</b> The executor sees the bundle only; the resolver reads through pinned, read-only tools.</li>
          </ul>
        </div>
      </div>
    </DiagramShell>
  );
}
