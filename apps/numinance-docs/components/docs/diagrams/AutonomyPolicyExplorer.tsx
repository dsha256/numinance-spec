'use client';

import { useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramButton, DiagramShell, Segmented, useReducedMotion } from './kit';

/** Rungs of AutonomyLevel (proto/numinance/types/v1/types.proto), lowest first. UNSPECIFIED is rejected and not a rung. */
const LEVELS = [
  ['OBSERVE', 'reads only'],
  ['SUGGEST', 'proposes, writes nothing'],
  ['IMPLEMENT', 'writes in a sandbox'],
  ['PR', 'opens pull requests'],
  ['MERGE', 'merges when policy allows'],
  ['STAGE', 'deploys to pre-production'],
  ['RELEASE', 'deploys to production'],
  ['OPERATE', 'operations against production'],
] as const;
const I = { SUGGEST: 1, PR: 3, MERGE: 4, RELEASE: 6, OPERATE: 7 } as const;
const RISKS = ['READ', 'LOW', 'MEDIUM', 'HIGH', 'PRIVILEGED'] as const;
type Risk = (typeof RISKS)[number];
type Playbook = 'dependency-upgrade' | 'other' | 'none';
type Env = 'production' | 'other' | 'none';
type RuleLevel = 'MERGE' | 'RELEASE' | 'OPERATE';
type Ctx = { playbook: Playbook; risk: Risk; env: Env; ruleLevel: RuleLevel };

const RULE_IDX: Record<RuleLevel, number> = { MERGE: I.MERGE, RELEASE: I.RELEASE, OPERATE: I.OPERATE };
const PRESETS: { id: string; label: string; ctx: Ctx }[] = [
  { id: 'default', label: 'No rule matches', ctx: { playbook: 'other', risk: 'MEDIUM', env: 'other', ruleLevel: 'MERGE' } },
  { id: 'rule', label: 'Low-risk dependency upgrade', ctx: { playbook: 'dependency-upgrade', risk: 'LOW', env: 'other', ruleLevel: 'MERGE' } },
  { id: 'high', label: 'High-risk floor', ctx: { playbook: 'dependency-upgrade', risk: 'HIGH', env: 'other', ruleLevel: 'MERGE' } },
  { id: 'prod', label: 'Production floor (what-if)', ctx: { playbook: 'dependency-upgrade', risk: 'LOW', env: 'production', ruleLevel: 'RELEASE' } },
];

/** Implements the order stated in autonomy_policy.proto: lowest matching rule else default, then the lowest matching floor max_level. */
function resolve(c: Ctx) {
  const ruleMatch = c.playbook === 'dependency-upgrade' && c.risk === 'LOW'; // rule: risk exact; absent value = no match
  const requested = ruleMatch ? RULE_IDX[c.ruleLevel] : I.PR;
  const f1 = RISKS.indexOf(c.risk) >= RISKS.indexOf('HIGH'); // floor: risk matches that class and every higher one
  const f2 = c.env === 'production' || c.env === 'none'; // floor: a key the change has no value for counts as a match
  const caps = [f1 ? I.SUGGEST : 99, f2 ? I.MERGE : 99];
  const cap = Math.min(...caps);
  const effective = Math.min(requested, cap);
  const clamped = effective < requested;
  const binding = clamped ? [f1 && I.SUGGEST === cap ? 1 : 0, f2 && I.MERGE === cap ? 1 : 0] : [0, 0];
  let why: string;
  if (clamped) {
    const names = [binding[0] ? 'the HIGH-risk floor' : '', binding[1] ? 'the production floor' : ''].filter(Boolean).join(' and ');
    why = `${ruleMatch ? 'The rule' : 'The default'} requests ${LEVELS[requested][0]}, but ${names} caps matching changes at ${LEVELS[cap][0]}. A configured level never bypasses a matching floor, so ${LEVELS[effective][0]} is the highest this change may reach.`;
  } else if (ruleMatch) {
    why = `The dependency-upgrade rule matches and requests ${LEVELS[requested][0]}. ${f1 || f2 ? `A matching floor caps at ${LEVELS[cap][0]}, which does not lower it.` : 'No floor matches, so nothing lowers it.'}`;
  } else {
    why = `No rule matches, so the default ${LEVELS[requested][0]} applies. ${f1 || f2 ? `A matching floor caps at ${LEVELS[cap][0]}, which does not lower it.` : 'No floor matches.'}`;
  }
  return { ruleMatch, requested, f1, f2, cap, capped: f1 || f2, effective, clamped, binding, why };
}

const YAML: { t: string; k?: 'def' | 'rule' | 'f1' | 'f2' }[] = [
  { t: 'apiVersion: numinance.dev/v1' },
  { t: 'kind: AutonomyPolicy' },
  { t: 'metadata: {name: org-ladder, scope: SCOPE_ORG}' },
  { t: 'spec:' },
  { t: '  defaults: {level: AUTONOMY_LEVEL_PR}', k: 'def' },
  { t: '  rules:' },
  { t: '    - {match: {playbook: dependency-upgrade, risk: RISK_CLASS_LOW}, level: AUTONOMY_LEVEL_MERGE}', k: 'rule' },
  { t: '  floors:' },
  { t: '    rules:' },
  { t: '      - {match: {risk: RISK_CLASS_HIGH}, max_level: AUTONOMY_LEVEL_SUGGEST}', k: 'f1' },
  { t: '      - {match: {environment: production}, max_level: AUTONOMY_LEVEL_MERGE}', k: 'f2' },
];

const tag = 'rounded-sm border px-1 py-px text-[11px] leading-none';

function Mark({ ok, yes = 'matched', no = 'not matched' }: { ok: boolean; yes?: string; no?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1 whitespace-nowrap font-mono text-xs', ok ? 'text-lp-orange' : 'text-fd-muted-foreground')}>
      <span aria-hidden>{ok ? '✓' : '–'}</span>
      {ok ? yes : no}
    </span>
  );
}

export function AutonomyPolicyExplorer() {
  const [c, setC] = useState<Ctx>(PRESETS[1].ctx);
  const reduced = useReducedMotion();
  const r = resolve(c);
  const active = PRESETS.find((p) => JSON.stringify(p.ctx) === JSON.stringify(c))?.id ?? 'custom';
  const set = (p: Partial<Ctx>) => setC({ ...c, ...p });
  const envLabel = c.env === 'none' ? 'no environment value' : c.env === 'production' ? 'environment production' : 'another environment';
  const summary = `Effective level ${LEVELS[r.effective][0]} for ${c.playbook === 'none' ? 'a change with no playbook' : `playbook ${c.playbook}`}, risk ${c.risk}, ${envLabel}. ${r.why}`;
  const whatIf = c.ruleLevel !== 'MERGE';

  const lineHit = (k?: string) =>
    k === 'def' ? !r.ruleMatch : k === 'rule' ? r.ruleMatch : k === 'f1' ? r.f1 : k === 'f2' ? r.f2 : false;

  return (
    <DiagramShell
      title="Autonomy policy explorer"
      question="Given defaults, rules and floors, what is the highest level this change may reach?"
      controls={
        <div className="flex w-full flex-col gap-2">
          <div role="group" aria-label="Scenario" className="flex flex-wrap items-center gap-1">
            <span className="mr-1 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Scenario</span>
            {PRESETS.map((p) => (
              <DiagramButton key={p.id} pressed={active === p.id} onClick={() => setC(p.ctx)}>{p.label}</DiagramButton>
            ))}
            {active === 'custom' ? <span className="font-mono text-xs text-fd-muted-foreground">custom</span> : null}
          </div>
          <div className="flex flex-wrap gap-x-5 gap-y-2">
            <Segmented label="Playbook" value={c.playbook} onChange={(v) => set({ playbook: v })} options={[{ value: 'dependency-upgrade', label: 'dependency-upgrade' }, { value: 'other', label: 'other' }, { value: 'none', label: 'none' }]} />
            <Segmented label="Risk" value={c.risk} onChange={(v) => set({ risk: v })} options={RISKS.map((x) => ({ value: x, label: x }))} />
            <Segmented label="Environment" value={c.env} onChange={(v) => set({ env: v })} options={[{ value: 'other', label: 'other' }, { value: 'production', label: 'production' }, { value: 'none', label: 'none' }]} />
            <Segmented label="Rule level (what-if)" value={c.ruleLevel} onChange={(v) => set({ ruleLevel: v })} options={[{ value: 'MERGE', label: 'MERGE (as written)' }, { value: 'RELEASE', label: 'RELEASE' }, { value: 'OPERATE', label: 'OPERATE' }]} />
          </div>
        </div>
      }
      legend={
        <>
          <span><span className="text-lp-orange">✓</span> matched</span>
          <span><span className="rounded-sm bg-lp-yellow px-1 text-lp-on-yellow">■</span> reachable rung</span>
          <span><span className="text-nm-deny">✕</span> blocked by a floor</span>
          <span><span className="text-nm-accent">▤</span> real org-ladder example</span>
        </>
      }
      explanation={
        <>
          <p>The ladder is the AutonomyLevel enum, lowest to highest: OBSERVE (reads only), SUGGEST (proposes, writes nothing), IMPLEMENT (writes in a sandbox), PR (opens pull requests), MERGE (merges when policy allows), STAGE (deploys to pre-production), RELEASE (deploys to production), OPERATE (operations against production).</p>
          <p>Resolution follows <code>AutonomyPolicySpec</code>: the level is the lowest level among matching rules, else <code>defaults.level</code>; then no level above the lowest <code>max_level</code> among the matching floors. The example policy is <code>org-ladder</code>: default PR; one rule (playbook dependency-upgrade and risk LOW sets MERGE); two floors (risk HIGH or higher caps at SUGGEST; environment production caps at MERGE).</p>
          <ul>
            <li>In a rule, risk matches exactly and a key the change has no value for does not match.</li>
            <li>In a floor, risk matches that class and every higher one, and a key the change has no value for counts as a match, which is why the toggle <code>none</code> for environment still matches the production floor.</li>
            <li>The playbook, risk and environment values are the resolver context of the change. They are not extra fields on a rule. The what-if rule level only changes the simulation; the YAML shown is the real example text.</li>
            <li>Kernel floors are always on. Promotion to a higher level is proposed by evaluation and accepted by a human, and is never self-granted.</li>
            <li>An org with no floors gets the shipped floor (SUGGEST for HIGH and PRIVILEGED, PR for the rest); control-pack floors can only add caps.</li>
          </ul>
          <p>Everything here is simulated in the browser from the example policy. Nothing is connected to a running engine.</p>
        </>
      }
    >
      <div className="grid gap-4 md:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]">
        <div className="min-w-0 space-y-3">
          <div className="relative overflow-hidden rounded-sm border border-fd-border bg-fd-background">
            <ol className="m-0 list-none p-0" aria-label="Autonomy ladder, highest rung first">
              {LEVELS.map(([name, desc], i) => ({ name, desc, i })).reverse().map(({ name, desc, i }) => {
                const reachable = i <= r.effective;
                const blocked = i > r.effective && i <= r.requested;
                return (
                  <li key={name} className="flex h-9 items-center gap-2 border-t border-fd-border/70 px-2 font-mono text-xs first:border-t-0">
                    <span className={cn('flex size-5 shrink-0 items-center justify-center rounded-sm text-[11px]', reachable ? 'bg-lp-yellow text-lp-on-yellow' : 'border border-fd-border text-fd-muted-foreground')} aria-hidden>{blocked ? '✕' : i + 1}</span>
                    <span className={cn('w-[5.5rem] shrink-0', blocked && 'text-nm-deny line-through', !reachable && !blocked && 'text-fd-muted-foreground')}>{name}</span>
                    <span className="hidden min-w-0 flex-1 truncate text-fd-muted-foreground sm:inline">{desc}</span>
                    <span className="ml-auto flex shrink-0 gap-1 sm:ml-0">
                      {i === r.requested ? <span className={cn(tag, 'border-fd-border text-fd-muted-foreground')}>requested</span> : null}
                      {r.capped && i === r.cap ? <span className={cn(tag, 'border-nm-deny/60 text-nm-deny')}>cap</span> : null}
                      {i === r.effective ? <span className={cn(tag, 'border-lp-orange bg-lp-orange/10 text-lp-orange')}>effective</span> : null}
                    </span>
                  </li>
                );
              })}
            </ol>
            {/* Marker slides along the ladder; the list is rendered highest rung first. */}
            <div
              aria-hidden
              className="pointer-events-none absolute inset-x-0 top-0 h-9 border-y-2 border-lp-orange"
              style={{
                transform: `translateY(${(LEVELS.length - 1 - r.effective) * 2.25}rem)`,
                transition: reduced ? 'none' : 'transform 450ms cubic-bezier(0.2, 0.8, 0.2, 1)',
              }}
            />
          </div>

          <div role="status" aria-live="polite" className="rounded-sm border border-lp-orange/60 bg-lp-orange/5 px-3 py-2 text-sm">
            <span className="font-mono text-xs uppercase tracking-wider text-lp-orange">Effective level </span>
            <span className="font-mono font-semibold">AUTONOMY_LEVEL_{LEVELS[r.effective][0]}</span>
            <p className="m-0 mt-1 leading-relaxed text-fd-foreground/90">{r.why}</p>
          </div>

          <ol className="m-0 list-none space-y-2 p-0 text-sm">
            <li className="rounded-sm border border-fd-border px-3 py-2">
              <div className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">1 · Default</div>
              <div>defaults.level = PR {r.ruleMatch ? <span className="text-fd-muted-foreground">(overridden by the matching rule)</span> : <span className="text-lp-orange">(used)</span>}</div>
            </li>
            <li className="rounded-sm border border-fd-border px-3 py-2">
              <div className="flex flex-wrap items-center justify-between gap-2"><span className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">2 · Rule: dependency-upgrade, LOW → {c.ruleLevel}</span><Mark ok={r.ruleMatch} /></div>
              <div className="text-fd-muted-foreground">
                playbook (resolver context): {c.playbook} {c.playbook === 'dependency-upgrade' ? '✓' : '✕'} · risk (exact): {c.risk} {c.risk === 'LOW' ? '✓' : '✕'}
              </div>
            </li>
            <li className="rounded-sm border border-fd-border px-3 py-2">
              <div className="flex flex-wrap items-center justify-between gap-2"><span className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">3a · Floor: risk HIGH or higher → max SUGGEST</span><Mark ok={r.f1} yes="matched, caps" no="not matched" /></div>
              <div className="text-fd-muted-foreground">risk (resolver context): {c.risk}; a floor matches that class and every higher one.</div>
            </li>
            <li className="rounded-sm border border-fd-border px-3 py-2">
              <div className="flex flex-wrap items-center justify-between gap-2"><span className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">3b · Floor: environment production → max MERGE</span><Mark ok={r.f2} yes="matched, caps" no="not matched" /></div>
              <div className="text-fd-muted-foreground">environment (resolver context): {c.env === 'none' ? 'no value, which counts as a match in a floor' : c.env === 'production' ? 'production' : 'a different environment'}.</div>
            </li>
            <li className="rounded-sm border border-fd-border px-3 py-2">
              <div className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">4 · Result</div>
              <div>min(requested {LEVELS[r.requested][0]}, lowest matching floor {r.capped ? LEVELS[r.cap][0] : 'none'}) = <b>{LEVELS[r.effective][0]}</b></div>
            </li>
          </ol>
        </div>

        <div className="min-w-0 space-y-3">
          <div className="overflow-hidden rounded-sm border border-fd-border bg-fd-background">
            <div className="flex items-center justify-between border-b border-fd-border px-3 py-1.5 font-mono text-xs text-fd-muted-foreground">
              <span>schemas/examples/AutonomyPolicy/org-ladder.yaml</span>
              <span className="text-nm-accent">real example</span>
            </div>
            <pre tabIndex={0} aria-label="org-ladder.yaml, scrollable" className="m-0 overflow-x-auto py-2 font-mono text-xs leading-6">
              {YAML.map((l, i) => {
                const hit = lineHit(l.k);
                const bind = (l.k === 'f1' && r.binding[0]) || (l.k === 'f2' && r.binding[1]);
                return (
                  <div key={i} className={cn('flex min-w-max border-l-2 pr-3', hit ? 'border-lp-orange bg-lp-orange/10' : 'border-transparent', bind && 'border-nm-deny bg-nm-deny/10')}>
                    <span className="w-9 shrink-0 select-none px-2 text-right text-fd-muted-foreground" aria-hidden>{i + 1}</span>
                    <span className="w-5 shrink-0 select-none" aria-hidden>{bind ? <span className="text-nm-deny">◆</span> : hit ? <span className="text-lp-orange">✓</span> : ''}</span>
                    <code className="whitespace-pre">{l.t}</code>
                    {hit ? <span className="sr-only">{bind ? ' (matched, clamps the level)' : ' (matched)'}</span> : null}
                  </div>
                );
              })}
            </pre>
            {whatIf ? <p className="m-0 border-t border-fd-border px-3 py-1.5 text-xs text-nm-warn">What-if: the rule level is raised to {c.ruleLevel} in the simulation only; line 7 above is the real text.</p> : null}
          </div>

          <ul className="m-0 list-none space-y-1.5 p-0 text-sm leading-relaxed">
            <li className="rounded-sm border border-lp-yellow/70 bg-lp-yellow/15 px-3 py-2"><b>Kernel floors are always on.</b> They sit under every policy and no rule or default removes them.</li>
            <li className="rounded-sm border border-fd-border px-3 py-2"><b>The highest configured level never bypasses a matching floor.</b> Floors cap; they are not suggestions.</li>
            <li className="rounded-sm border border-fd-border px-3 py-2"><b>Promotion is proposed, then accepted.</b> Evaluation may propose a higher level; a human accepts it. An agent never grants itself a rung.</li>
          </ul>
        </div>
      </div>
    </DiagramShell>
  );
}
