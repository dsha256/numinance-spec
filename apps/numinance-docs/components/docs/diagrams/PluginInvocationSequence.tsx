'use client';

import { useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramButton, DiagramShell, Segmented, Decision, useReducedMotion } from './kit';

type Scenario = 'allowed' | 'denied' | 'undeclared';
type Lane = 0 | 1 | 2 | 3;

const LANES = [
  { id: 'host', short: 'Host', long: 'Plugin host + policy decision point' },
  { id: 'provider', short: 'Provider', long: 'Plugin subprocess' },
  { id: 'proxy', short: 'Egress', long: 'Broker / egress proxy' },
  { id: 'evidence', short: 'Evidence', long: 'Evidence writer (runner identity)' },
] as const;

type Step = {
  id: string;
  title: string;
  /** exact RPC or mechanism name */
  rpc: string;
  from: Lane;
  to: Lane;
  fields: string[];
  holds: string;
  cannot: string;
  only?: Scenario[];
  except?: Scenario[];
  tone?: 'deny' | 'kernel';
};

const STEPS: Step[] = [
  {
    id: 'launch', title: 'Host launches the subprocess', rpc: 'exec + NUMINANCE_PLUGIN_COOKIE', from: 0, to: 1,
    fields: ['NUMINANCE_PLUGIN_COOKIE = MagicCookie (plugin.CookieEnv)', 'NUMINANCE_PLUGIN_PROTOCOLS (offer)', 'host address env (plugin.HostAddrEnv)'],
    holds: 'Host: it starts the process and offers the protocol versions. The host identifies the plugin by the connection it established at launch, never by request content.',
    cannot: 'The cookie is not a secret and not authority: it only stops a user from running the provider binary by hand. A provider with a wrong cookie refuses to serve.',
  },
  {
    id: 'handshake', title: 'Negotiate versions and capabilities', rpc: 'PluginService.Handshake', from: 0, to: 1,
    fields: ['HandshakeRequest: spec_version, protocol_versions, ports, capabilities', 'HandshakeResponse: protocol_version, spec_version, capabilities'],
    holds: 'Host: offers; the provider can only choose from the offer. A negotiated capability must be in both the host offer and the manifest, otherwise the handshake aborts.',
    cannot: 'Select a lower protocol version later, or negotiate a capability that is not in its manifest.',
  },
  {
    id: 'manifest', title: 'Read the declared manifest', rpc: 'PluginService.GetManifest', from: 0, to: 1,
    fields: ['GetManifestResponse: manifest', 'Manifest: plugin_id, digest, signer_identity, ports, capabilities, permissions_delta', 'CapabilityDescriptor: credential_intents, network_intents, filesystem_intents, evidence_declared'],
    holds: 'Host: the manifest must equal (proto.Equal) the installed manifest.yaml whose digest the lockfile pins, or the host refuses to start the plugin.',
    cannot: 'Widen its own declared intents at run time. The manifest is never a source of intents the install did not review.',
  },
  {
    id: 'descriptors', title: 'Read the port descriptors', rpc: 'PluginService.GetDescriptors', from: 0, to: 1,
    fields: ['GetDescriptorsResponse: descriptor_set (FileDescriptorSet)'],
    holds: 'Host: its own compiled copy of each port file is authoritative; a differing option fails install.',
    cannot: 'Redefine a method annotation (risk, exec mode, dry run) to change how the host treats it.',
  },
  {
    id: 'decide', title: 'Policy decision and grant', rpc: 'policy evaluation (host, outside the model)', from: 0, to: 0, tone: 'kernel',
    fields: ['ActionContext: actor_kind, actor_principal, grant_id, policy_decision_id, profile_digest, plan_digest'],
    holds: 'Host: the decision and the grant are made here, before Invoke, by deterministic policy. Your policy decides. Not the model.',
    cannot: 'Take part. The provider has not been sent the action yet and cannot influence, delay or pre-approve the decision.',
    except: ['denied'],
  },
  {
    id: 'decide-deny', title: 'Policy decision: DENY', rpc: 'policy evaluation (host, outside the model)', from: 0, to: 0, tone: 'deny', only: ['denied'],
    fields: ['ActionContext is never built', 'Invoke is never sent'],
    holds: 'Host: the decision is DENY. The action stops here. Nothing reaches the provider process.',
    cannot: 'Learn that the action was attempted, or retry it. A denial is not an error the provider can handle.',
  },
  {
    id: 'deny-record', title: 'Denial recorded as evidence', rpc: 'host appends a policy-decision record', from: 0, to: 3, only: ['denied'],
    fields: ['policy decision and denial reason appended under the runner identity'],
    holds: 'Evidence writer (runner identity): denials are recorded, not just approvals. The provider never appends.',
    cannot: 'Suppress or rewrite the record.',
  },
  {
    id: 'invoke', title: 'Invoke: carry one approved action', rpc: 'PluginService.Invoke (server-streaming)', from: 0, to: 1,
    fields: ['InvokeRequest: envelope', 'Envelope: method, context, request', 'ActionContext: invocation_id, change_id, run_id, grant_id, policy_decision_id, idempotency_key, deadline, budget, residency, dry_run'],
    holds: 'Host: it builds the Envelope and sets ActionContext. The target method annotations in the host descriptor copy govern the call, never Invoke itself.',
    cannot: 'Edit ActionContext, extend the deadline or budget, or act on any invocation_id the host did not assign.',
    except: ['denied'],
  },
  {
    id: 'cred', title: 'Provider asks for a credential handle', rpc: 'HostService.RequestCredential', from: 1, to: 0, except: ['undeclared'],
    fields: ['RequestCredentialRequest: invocation_id, intent', 'RequestCredentialResponse: handle (sensitive), expires_at, egress_proxy'],
    holds: 'Host + broker: the intent must be in the capability credential_intents and allowed by the grant. The response is an opaque handle bound to the invocation and the intent.',
    cannot: 'Receive credential material. The handle is useless outside the egress proxy and expires no later than the invocation deadline.',
    only: ['allowed'],
  },
  {
    id: 'cred-reject', title: 'Provider asks for an undeclared intent', rpc: 'HostService.RequestCredential', from: 1, to: 0, only: ['undeclared'], tone: 'deny',
    fields: ['RequestCredentialRequest: invocation_id, intent = not in credential_intents', 'no RequestCredentialResponse handle'],
    holds: 'Host: any intent not in the capability credential_intents that the grant allows is rejected. No handle is issued.',
    cannot: 'Obtain a handle, or a credential, for something its manifest and grant did not declare.',
  },
  {
    id: 'reject-record', title: 'Host records the rejected call', rpc: 'host appends the call and outcome', from: 0, to: 3, only: ['undeclared'],
    fields: ['the host records each HostService call and its outcome in the invocation evidence'],
    holds: 'Evidence writer (runner identity): the rejection is evidence. The provider may end the stream with its own ActionResult; the host record stands either way.',
    cannot: 'Erase the attempt.',
  },
  {
    id: 'egress', title: 'Request goes through the egress proxy', rpc: 'absolute-form HTTP to egress_proxy', from: 1, to: 2, only: ['allowed'],
    fields: ['request carries the handle', 'destination must be one declared in network_intents'],
    holds: 'Egress proxy: terminates the request, injects the real token, and re-originates it over TLS to the declared destination.',
    cannot: 'See the token, or reach a host it did not declare. Without a proxy the handle is not usable.',
  },
  {
    id: 'emit', title: 'Provider submits evidence', rpc: 'HostService.EmitEvidence', from: 1, to: 3, only: ['allowed'],
    fields: ['EmitEvidenceRequest: invocation_id, emit_key, record_type, payload, artifacts', 'EmitEvidenceResponse: evidence_id'],
    holds: 'Host: validates, then appends under the runner identity. record_type must be in evidence_declared and in the host allow-list; ids, actor, grant and timestamp come from host state.',
    cannot: 'Append to the chain, set record fields, or emit host-authored kinds (lifecycle, policy decision, gate, approval, credential issuance).',
  },
  {
    id: 'progress', title: 'Progress while the stream runs', rpc: 'HostService.ReportProgress', from: 1, to: 0, only: ['allowed'],
    fields: ['ReportProgressRequest: invocation_id, completed, total, reason'],
    holds: 'Host: accepts STARTED, RUNNING, WAITING, COMPLETED only; completed never decreases.',
    cannot: 'Report completed above a non-zero total, or send free text (there is no message field).',
  },
  {
    id: 'result', title: 'Stream ends with the result', rpc: 'InvokeResponse stream, final message', from: 1, to: 0, except: ['denied'],
    fields: ['InvokeResponse: result', 'ActionResult: status, evidence_ids, side_effects, duration, diagnostics'],
    holds: 'Host: records the outcome in evidence. Every message but the last carries INVOKE_STATUS_RUNNING.',
    cannot: 'Claim evidence the host did not append: evidence_ids only names ids the host returned.',
  },
  {
    id: 'outcome', title: 'Host records the outcome', rpc: 'host appends the result record', from: 0, to: 3, except: ['denied'],
    fields: ['outcome appended under the runner identity'],
    holds: 'Evidence writer (runner identity): the append is the host, never the plugin.',
    cannot: 'Choose its own record id, actor or timestamp.',
  },
  {
    id: 'health', title: 'Health check', rpc: 'PluginService.Health', from: 0, to: 1, only: ['allowed'],
    fields: ['HealthResponse: status (HEALTH_STATUS_SERVING | HEALTH_STATUS_NOT_SERVING)'],
    holds: 'Host: HEALTH_STATUS_UNSPECIFIED is rejected.',
    cannot: 'Health says nothing about correctness; it only says the plugin is serving.',
  },
  {
    id: 'shutdown', title: 'Shutdown', rpc: 'PluginService.Shutdown', from: 0, to: 1,
    fields: ['ShutdownRequest / ShutdownResponse (no fields)'],
    holds: 'Host: first cancels in-flight operations and revokes credential handles and egress grants, then kills the process after a bounded grace period.',
    cannot: 'Start new work after Shutdown, or keep a handle alive.',
  },
];

const SCENARIOS: { value: Scenario; label: string; summary: string }[] = [
  { value: 'allowed', label: 'Allowed', summary: 'Policy allows the action; the provider runs it and every privileged step goes through the host.' },
  { value: 'denied', label: 'Denied by policy', summary: 'Policy denies the action on the host. Invoke never reaches the provider; the denial is recorded as evidence.' },
  { value: 'undeclared', label: 'Undeclared capability', summary: 'The provider asks for a credential intent it never declared. The host rejects it and records the call.' },
];

const visible = (s: Scenario) => STEPS.filter((x) => (!x.only || x.only.includes(s)) && !(x.except && x.except.includes(s)));

function Arrow({ step, active, reduce }: { step: Step; active: boolean; reduce: boolean }) {
  const lo = Math.min(step.from, step.to);
  const hi = Math.max(step.from, step.to);
  const right = step.to >= step.from;
  const color = active ? 'text-lp-orange' : 'text-fd-muted-foreground';
  return (
    <div className="relative grid h-7 grid-cols-4" aria-hidden>
      {LANES.map((l) => (
        <div key={l.id} className="relative"><span className="absolute inset-y-0 left-1/2 w-px bg-fd-border" /></div>
      ))}
      {lo === hi ? (
        <span className={cn('absolute top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-sm border px-1 font-mono text-[11px] leading-4', active ? 'border-lp-orange' : 'border-fd-border', step.tone === 'kernel' ? 'bg-lp-yellow text-lp-on-yellow' : 'bg-fd-card', color)} style={{ left: `${lo * 25 + 12.5}%` }}>
          {step.tone === 'deny' ? '✕' : '◆'}
        </span>
      ) : (
        <span className={cn('absolute top-1/2 flex h-4 -translate-y-1/2 items-center', color)} style={{ left: `${lo * 25 + 12.5}%`, width: `${(hi - lo) * 25}%` }}>
          <span className={cn('h-0 flex-1 border-t-2', active ? 'border-lp-orange' : 'border-current', reduce ? '' : 'transition-colors')} />
          <span className="-ml-0.5 font-mono text-[11px] leading-none" style={{ order: right ? 1 : -1 }}>{right ? '▶' : '◀'}</span>
        </span>
      )}
    </div>
  );
}

export function PluginInvocationSequence() {
  const reduce = !!useReducedMotion();
  const [scenario, setScenario] = useState<Scenario>('allowed');
  const [i, setI] = useState(0);
  const steps = visible(scenario);
  const idx = Math.min(i, steps.length - 1);
  const cur = steps[idx];
  const sc = SCENARIOS.find((s) => s.value === scenario)!;
  const reached = (n: number) => n <= idx;

  const live = `${sc.label}. Step ${idx + 1} of ${steps.length}: ${cur.title}. ${cur.rpc}. ${cur.holds}`;

  return (
    <DiagramShell
      title="Plugin invocation sequence"
      question="In what order do host and provider talk, and who decides what?"
      controls={
        <>
          <Segmented<Scenario> label="Scenario" value={scenario} onChange={(v) => { setScenario(v); setI(0); }} options={SCENARIOS.map(({ value, label }) => ({ value, label }))} />
          <span className="flex gap-1">
            <DiagramButton onClick={() => setI(Math.max(0, idx - 1))} disabled={idx === 0}>Back</DiagramButton>
            <DiagramButton onClick={() => setI(Math.min(steps.length - 1, idx + 1))} disabled={idx === steps.length - 1}>Next step</DiagramButton>
            <DiagramButton onClick={() => setI(0)} disabled={idx === 0}>Reset</DiagramButton>
          </span>
        </>
      }
      legend={
        <>
          <span><span className="text-lp-orange">▶</span> selected message</span>
          <span><span className="rounded-sm bg-lp-yellow px-1 text-lp-on-yellow">◆</span> host policy decision</span>
          <span><span className="text-nm-deny">✕</span> denied or rejected</span>
          <span><span className="text-nm-accent">Name</span> typed RPC from the spec</span>
        </>
      }
      explanation={
        <>
          <p>Illustrative and simulated: nothing here talks to a running host. RPC and field names are the ones in <code>proto/numinance/plugin/v1</code> and <code>sdk/go/plugin</code>.</p>
          <ol>
            <li>The host starts the provider subprocess with <code>NUMINANCE_PLUGIN_COOKIE</code> set to the magic cookie, then calls <code>Handshake</code>, <code>GetManifest</code> and <code>GetDescriptors</code> on <code>PluginService</code>.</li>
            <li>The policy decision and the grant happen on the host, outside the model and before the provider sees the action.</li>
            <li>If allowed, the host calls <code>Invoke</code> (server-streaming) with an <code>Envelope</code> whose <code>ActionContext</code> the host sets. The provider may call back on <code>HostService</code>: <code>RequestCredential</code> returns an opaque handle that only works through the egress proxy, which injects the real token; <code>EmitEvidence</code> submits evidence that the host appends under the runner identity; <code>ReportProgress</code> reports progress.</li>
            <li>The stream ends with an <code>ActionResult</code>; the host records the outcome, may call <code>Health</code>, and finally <code>Shutdown</code>, which first revokes handles and egress grants.</li>
            <li>If policy denies, <code>Invoke</code> is never sent and the denial is recorded as evidence. If the provider requests a credential intent that is not in its declared <code>credential_intents</code>, the host rejects it and records the call.</li>
          </ol>
          <p>The provider never holds authority: it cannot approve, widen a grant, see a credential, or append evidence.</p>
        </>
      }
    >
      <p className="m-0 mb-3 text-sm text-fd-muted-foreground">{sc.summary}</p>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
        <div>
          <div className="grid grid-cols-4 pb-1 text-center font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">
            {LANES.map((l, n) => (
              <span key={l.id} title={l.long} className={cn('px-0.5', n === 0 && 'text-fd-foreground')}>{l.short}</span>
            ))}
          </div>
          <ol className="m-0 list-none space-y-1 p-0">
            {steps.map((s, n) => {
              const active = n === idx;
              return (
                <li key={s.id}>
                  <button
                    type="button"
                    onClick={() => setI(n)}
                    aria-current={active ? 'step' : undefined}
                    className={cn(
                      'block w-full rounded-sm border px-2 py-1.5 text-left',
                      reduce ? '' : 'transition-colors',
                      active ? 'border-lp-orange bg-lp-orange/10' : 'border-transparent hover:border-fd-border',
                      !reached(n) && 'opacity-60',
                    )}
                  >
                    <span className="flex items-baseline gap-2">
                      <span className="font-mono text-[11px] text-fd-muted-foreground">{n + 1}</span>
                      <span className={cn('min-w-0 break-words font-mono text-xs', active ? 'text-lp-orange' : 'text-nm-accent')}>{s.rpc}</span>
                    </span>
                    <Arrow step={s} active={active} reduce={reduce} />
                  </button>
                </li>
              );
            })}
          </ol>
          <p className="m-0 mt-2 font-mono text-[11px] text-fd-muted-foreground">
            {LANES.map((l) => `${l.short}: ${l.long}`).join(' · ')}
          </p>
        </div>

        <div className="rounded-sm border border-fd-border bg-fd-background p-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-xs text-fd-muted-foreground">Step {idx + 1} of {steps.length}</span>
            {cur.tone === 'kernel' ? <Decision kind="allow" /> : null}
            {cur.tone === 'deny' ? <Decision kind="deny" /> : null}
          </div>
          <h4 className="m-0 mt-1 text-sm font-medium text-fd-foreground">{cur.title}</h4>
          <p className="m-0 mt-1 break-words font-mono text-xs text-nm-accent">{cur.rpc}</p>
          <dl className="m-0 mt-3 space-y-3 text-sm">
            <div>
              <dt className="font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Message and fields</dt>
              <dd className="m-0 mt-1">
                <ul className="m-0 list-none space-y-1 p-0 font-mono text-xs">
                  {cur.fields.map((f) => <li key={f} className="break-words">{f}</li>)}
                </ul>
              </dd>
            </div>
            <div className={cn('rounded-sm border p-2', cur.tone === 'kernel' ? 'border-lp-yellow bg-lp-yellow/20' : 'border-fd-border')}>
              <dt className="font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Who holds authority</dt>
              <dd className="m-0 mt-1 text-fd-foreground">{cur.holds}</dd>
            </div>
            <div className="rounded-sm border border-nm-deny/40 p-2">
              <dt className="font-mono text-[11px] uppercase tracking-wider text-nm-deny">✕ The provider can not</dt>
              <dd className="m-0 mt-1 text-fd-foreground">{cur.cannot}</dd>
            </div>
          </dl>
        </div>
      </div>
      <p className="sr-only" aria-live="polite" role="status">{live}</p>
    </DiagramShell>
  );
}
