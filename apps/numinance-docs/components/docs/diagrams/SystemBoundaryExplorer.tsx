'use client';

import { useMemo, useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramShell } from './kit';

type Zone = 'vendor' | 'customer' | 'kernel' | 'harness';
type Layer = 'authority' | 'data' | 'credentials';

interface NodeDef {
  id: string;
  zone: Zone;
  name: string;
  tag: string;
  holds: string[];
  crosses: string[];
  denied: string[];
}
interface EdgeDef {
  from: string;
  to: string;
  layer: Layer;
  label: string;
}

const ZONES: { id: Zone; title: string; sub: string }[] = [
  { id: 'vendor', title: 'Vendor control plane', sub: 'thin · metadata only · outbound-only from the runner' },
  { id: 'customer', title: 'Customer data plane', sub: 'your infrastructure · source, artifacts, evidence and credentials stay here' },
  { id: 'kernel', title: 'Fixed trust kernel', sub: 'not editable by any grant · outside the model' },
  { id: 'harness', title: 'AI harness', sub: 'replaceable · runs inside the sandbox' },
];

const LAYERS: { id: Layer; label: string; letter: string; hint: string }[] = [
  { id: 'authority', label: 'Authority', letter: 'A', hint: 'who may decide or order' },
  { id: 'data', label: 'Data', letter: 'D', hint: 'what bytes move' },
  { id: 'credentials', label: 'Credentials', letter: 'C', hint: 'where secrets travel' },
];

const NODES: NodeDef[] = [
  {
    id: 'controlplane', zone: 'vendor', name: 'Control plane', tag: 'dashboard · Connect API',
    holds: ['Change status, gate decisions and approvals as metadata', 'Runner fleet state: versions, profiles, breaker position', 'Licence state'],
    crosses: ['Inbound: human approve / reject / escalate with a human principal', 'Inbound: runner heartbeats, references, digests, statuses'],
    denied: ['Source, diffs, prompts, secrets, evidence content', 'A remote kill switch: an installed runtime keeps applying policy and exporting evidence'],
  },
  {
    id: 'runner', zone: 'customer', name: 'Runner', tag: 'host agent',
    holds: ['Host attestation and supervised sandboxes', 'Signed bundles it pulled for this host', 'Evidence signing role (the runner identity writes evidence)'],
    crosses: ['Outbound-only link to the control plane: references, digests, statuses', 'Starts one sandbox per work unit; supervises broker and egress proxy'],
    denied: ['Accepting inbound connections from the vendor', 'Sending content to the control plane'],
  },
  {
    id: 'engine', zone: 'customer', name: 'Engine', tag: 'durable workflows',
    holds: ['Change state machine and per-stage orchestration', 'References and digests only in orchestration payloads'],
    crosses: ['Asks the policy decision point before every privileged action', 'Reads and writes content only through artifact references'],
    denied: ['Raw credentials: the broker is a separate process', 'Content inside workflow payloads'],
  },
  {
    id: 'plugins', zone: 'customer', name: 'Plugins', tag: 'signed · digest-pinned',
    holds: ['Provider capabilities: SCM, CI, knowledge, verifiers', 'Declared permissions in a manifest'],
    crosses: ['Launched by the plugin host; every call carries an ActionContext', 'Reach external systems only with broker-issued handles'],
    denied: ['Running unsigned or unpinned', 'Widening their own permissions without re-approval'],
  },
  {
    id: 'artifacts', zone: 'customer', name: 'Artifact store', tag: 'claim-check content',
    holds: ['Source snapshots, diffs, prompts, findings bodies, logs', 'Everything an orchestration payload points at by reference'],
    crosses: ['Read and written by engine and sandbox through references and sha256 digests'],
    denied: ['Leaving the customer plane toward the control plane'],
  },
  {
    id: 'evidencestore', zone: 'customer', name: 'Evidence store', tag: 'append-only · WORM',
    holds: ['Signed, hash-chained evidence records', 'Denials as well as allowed actions'],
    crosses: ['Appended only by the evidence writer', 'Exported for offline verification'],
    denied: ['Edits or deletions by any agent identity', 'Writes from the harness'],
  },
  {
    id: 'source', zone: 'customer', name: 'Source and systems', tag: 'repos · CI · cloud',
    holds: ['Repositories, tickets, pipelines, deployed environments', 'Branch protection and protected paths'],
    crosses: ['Worktree mounted into the sandbox', 'Reached by egress proxy and plugins under a grant'],
    denied: ['Direct deploys by an author identity: only the release pipeline deploys'],
  },
  {
    id: 'credstore', zone: 'customer', name: 'Credential sources', tag: 'secret managers · IAM',
    holds: ['Long-lived secrets and role definitions, in your systems'],
    crosses: ['Read by the broker only, to mint a scoped, short-lived handle'],
    denied: ['Being copied into the sandbox, the engine or the control plane'],
  },
  {
    id: 'egress', zone: 'customer', name: 'Egress proxy', tag: 'default-deny',
    holds: ['Per-step allow-list pushed by the broker and withdrawn at step end', 'Header injection rules for scoped credentials'],
    crosses: ['Terminates and re-originates sandbox requests, then logs them', 'Injects bearer material so the sandbox never holds it'],
    denied: ['Any host not on the step allow-list', 'Link-local and metadata-service addresses'],
  },
  {
    id: 'pdp', zone: 'kernel', name: 'Policy decision point', tag: 'allow · deny · require approval',
    holds: ['Signed policy bundles and customer floors', 'Every decision with reasons and a decision id'],
    crosses: ['Receives action, actor identity, change state and autonomy level', 'Returns allow, deny or require approval, with approver rules'],
    denied: ['Being overridden by a lower scope or by the model', 'Approving when approver equals author'],
  },
  {
    id: 'broker', zone: 'kernel', name: 'Capability broker', tag: 'handles, not secrets',
    holds: ['Grant-to-credential translation for one step', 'Handle TTL bounded by the activity timeout'],
    crosses: ['Mints scoped handles or egress injection rules from credential sources', 'Revokes handles on step end or breaker trip'],
    denied: ['Returning reusable credentials to the model', 'Issuing outside the current grant'],
  },
  {
    id: 'pluginhost', zone: 'kernel', name: 'Plugin host', tag: 'launches providers',
    holds: ['Manifest and permission checks', 'The only path from a run to a provider'],
    crosses: ['Launches providers; mediates each capability call against the grant'],
    denied: ['Loading an extension that fails signature, digest or permission checks'],
  },
  {
    id: 'sandbox', zone: 'kernel', name: 'Sandbox', tag: 'one per work unit',
    holds: ['Writable worktree and tmpfs; read-only runtime', 'Limits from the budget'],
    crosses: ['Network namespace routed only to the egress proxy'],
    denied: ['Ambient credentials, SSH keys and tokens', 'Direct network access', 'Writing protected paths (policy, CI definitions, IAM)'],
  },
  {
    id: 'evwriter', zone: 'kernel', name: 'Evidence writer', tag: 'runner identity',
    holds: ['Append path for decisions, denials, gate outcomes and artifacts digests'],
    crosses: ['Writes records to the evidence store under the runner identity'],
    denied: ['Accepting records from the author agent', 'A write failure is not ignored: it trips the breaker'],
  },
  {
    id: 'breaker', zone: 'kernel', name: 'Circuit breaker', tag: 'customer-local',
    holds: ['Position: FULL_AUTO, PR_ONLY, READ_ONLY or STOPPED'],
    crosses: ['Revokes handles and pauses runs at the next checkpoint when tripped', 'Reset needs a human and a reason'],
    denied: ['Depending on the control plane', 'Being reset by an agent'],
  },
  {
    id: 'harness', zone: 'harness', name: 'Claude or Codex harness', tag: 'brings the intelligence',
    holds: ['The model loop for one work unit', 'A worktree and the tools its grant allows'],
    crosses: ['Requests go out only through the egress proxy and the plugin host', 'Returns a candidate; verification decides what it earns'],
    denied: ['Credentials, policy files, and approval rights', 'Granting itself more authority', 'Deploying'],
  },
];

const EDGES: EdgeDef[] = [
  { from: 'engine', to: 'pdp', layer: 'authority', label: 'asks before every privileged action' },
  { from: 'pdp', to: 'broker', layer: 'authority', label: 'an allow decision derives the grant the broker honours' },
  { from: 'pdp', to: 'evwriter', layer: 'authority', label: 'every decision and denial is recorded' },
  { from: 'pdp', to: 'sandbox', layer: 'authority', label: 'grant bounds what the run may do' },
  { from: 'pdp', to: 'pluginhost', layer: 'authority', label: 'grant bounds which capabilities a provider may serve' },
  { from: 'sandbox', to: 'harness', layer: 'authority', label: 'contains the harness; the harness cannot edit the grant' },
  { from: 'harness', to: 'pluginhost', layer: 'authority', label: 'requests capabilities, never executes them itself' },
  { from: 'pluginhost', to: 'plugins', layer: 'authority', label: 'launches and mediates providers' },
  { from: 'controlplane', to: 'engine', layer: 'authority', label: 'human approval or rejection, never the model' },
  { from: 'controlplane', to: 'breaker', layer: 'authority', label: 'human trip or reset with reason and principal' },
  { from: 'breaker', to: 'engine', layer: 'authority', label: 'pauses runs at the next checkpoint' },
  { from: 'runner', to: 'sandbox', layer: 'authority', label: 'creates and supervises one per work unit' },
  { from: 'runner', to: 'controlplane', layer: 'data', label: 'references, digests, statuses only (outbound)' },
  { from: 'engine', to: 'artifacts', layer: 'data', label: 'content moves by reference and sha256' },
  { from: 'sandbox', to: 'artifacts', layer: 'data', label: 'candidate output stored by reference' },
  { from: 'source', to: 'sandbox', layer: 'data', label: 'worktree mount for the work unit' },
  { from: 'harness', to: 'egress', layer: 'data', label: 'all network traffic, no direct route' },
  { from: 'egress', to: 'source', layer: 'data', label: 'allow-listed requests, re-originated and logged' },
  { from: 'evwriter', to: 'evidencestore', layer: 'data', label: 'append-only records' },
  { from: 'plugins', to: 'source', layer: 'data', label: 'provider calls to SCM, CI and cloud' },
  { from: 'credstore', to: 'broker', layer: 'credentials', label: 'source of material, read by the broker only' },
  { from: 'broker', to: 'egress', layer: 'credentials', label: 'injection rule per step; sandbox never sees the secret' },
  { from: 'broker', to: 'plugins', layer: 'credentials', label: 'scoped handle, TTL bound to the step' },
  { from: 'breaker', to: 'broker', layer: 'credentials', label: 'trip revokes outstanding handles' },
  { from: 'egress', to: 'source', layer: 'credentials', label: 'adds bearer headers on re-origination' },
];

const byId = Object.fromEntries(NODES.map((n) => [n.id, n])) as Record<string, NodeDef>;
const LAYER_LETTER = Object.fromEntries(LAYERS.map((l) => [l.id, l.letter])) as Record<Layer, string>;

const zoneStyle: Record<Zone, string> = {
  vendor: 'border-fd-border border-dashed',
  customer: 'border-fd-foreground/40',
  kernel: 'border-lp-yellow bg-lp-yellow/10',
  harness: 'border-fd-border border-dashed',
};

export function SystemBoundaryExplorer() {
  const [selected, setSelected] = useState('harness');
  const [hover, setHover] = useState<string | null>(null);
  const [layers, setLayers] = useState<Layer[]>(['authority', 'data', 'credentials']);

  const focus = hover ?? selected;
  const node = byId[focus];

  const links = useMemo(
    () =>
      EDGES.filter((e) => layers.includes(e.layer) && (e.from === focus || e.to === focus)).map((e) => ({
        ...e,
        dir: e.from === focus ? ('out' as const) : ('in' as const),
        other: byId[e.from === focus ? e.to : e.from],
      })),
    [focus, layers],
  );
  const relation = useMemo(() => {
    const m = new Map<string, Set<Layer>>();
    for (const l of links) {
      const s = m.get(l.other.id) ?? new Set<Layer>();
      s.add(l.layer);
      m.set(l.other.id, s);
    }
    return m;
  }, [links]);

  const toggle = (l: Layer) =>
    setLayers((cur) => (cur.includes(l) ? (cur.length > 1 ? cur.filter((x) => x !== l) : cur) : [...cur, l]));

  const controls = (
    <div role="group" aria-label="Layers shown" className="flex flex-wrap items-center gap-1">
      <span className="mr-1 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Layers</span>
      {LAYERS.map((l) => {
        const on = layers.includes(l.id);
        return (
          <button
            key={l.id}
            type="button"
            aria-pressed={on}
            title={l.hint}
            onClick={() => toggle(l.id)}
            className={cn(
              'min-h-9 rounded-sm border px-3 py-1.5 font-mono text-xs transition-colors',
              on ? 'border-lp-ink bg-lp-ink text-lp-paper' : 'border-fd-border bg-fd-background text-fd-foreground hover:border-fd-foreground/50',
            )}
          >
            <span aria-hidden>{on ? '■' : '□'} </span>
            {l.label}
          </button>
        );
      })}
    </div>
  );

  const legend = (
    <>
      <span><span className="text-lp-orange">▣</span> selected</span>
      <span><span className="text-fd-foreground">▢</span> connected on shown layers</span>
      <span><span className="inline-block h-2.5 w-2.5 border border-lp-yellow bg-lp-yellow/40 align-middle" /> fixed trust kernel</span>
      <span>A authority · D data · C credentials</span>
    </>
  );

  const explanation = (
    <>
      <p>
        The customer data plane holds source, artifacts, evidence and credential sources. The vendor control plane is thin and receives
        references, digests and statuses only. Humans approve through it; the model never does. The fixed trust kernel (policy decision
        point, capability broker, plugin host, sandbox, evidence writer, circuit breaker) sits outside every grant. The AI harness is
        replaceable and runs inside the sandbox.
      </p>
      <ul>
        {ZONES.map((z) => (
          <li key={z.id}>
            <strong>{z.title}</strong> ({z.sub}):{' '}
            {NODES.filter((n) => n.zone === z.id).map((n) => n.name).join(', ')}.
          </li>
        ))}
      </ul>
      <p>Connections by layer:</p>
      <ul>
        {EDGES.map((e, i) => (
          <li key={i}>
            [{LAYER_LETTER[e.layer]}] {byId[e.from].name} to {byId[e.to].name}: {e.label}.
          </li>
        ))}
      </ul>
      <p>Per component, what it holds, what crosses its boundary and what is denied:</p>
      <ul>
        {NODES.map((n) => (
          <li key={n.id}>
            <strong>{n.name}</strong>. Holds: {n.holds.join('; ')}. Crosses: {n.crosses.join('; ')}. Denied: {n.denied.join('; ')}.
          </li>
        ))}
      </ul>
    </>
  );

  return (
    <DiagramShell
      title="System boundary explorer"
      question="What can cross which boundary, and who holds authority?"
      controls={controls}
      legend={legend}
      explanation={explanation}
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <div className="space-y-3" onMouseLeave={() => setHover(null)}>
          {ZONES.map((z) => (
            <fieldset key={z.id} className={cn('m-0 min-w-0 rounded-md border px-2.5 pb-2.5 pt-1', zoneStyle[z.id])}>
              <legend className="px-1.5 font-mono text-xs uppercase tracking-wider">
                <span className={cn(z.id === 'kernel' ? 'bg-lp-yellow px-1 text-lp-on-yellow' : 'text-fd-foreground')}>{z.title}</span>
              </legend>
              <p className="m-0 mb-2 px-1 text-xs text-fd-muted-foreground">{z.sub}</p>
              <ul className="m-0 grid list-none grid-cols-1 gap-1.5 p-0 min-[460px]:grid-cols-2 xl:grid-cols-3">
                {NODES.filter((n) => n.zone === z.id).map((n) => {
                  const isSel = n.id === selected;
                  const rel = relation.get(n.id);
                  const dim = !isSel && n.id !== focus && !rel;
                  return (
                    <li key={n.id} className="m-0 p-0">
                      <button
                        type="button"
                        aria-pressed={isSel}
                        onClick={() => setSelected(n.id)}
                        onMouseEnter={() => setHover(n.id)}
                        onFocus={() => {
                          setHover(null);
                          setSelected(n.id);
                        }}
                        className={cn(
                          'flex min-h-11 w-full flex-col items-start rounded-sm border bg-fd-background px-2.5 py-1.5 text-left transition-[opacity,border-color] motion-reduce:transition-none',
                          isSel ? 'border-lp-orange ring-1 ring-lp-orange' : rel ? 'border-fd-foreground' : 'border-fd-border',
                          dim && 'opacity-55',
                        )}
                      >
                        <span className={cn('text-sm font-medium', isSel ? 'text-lp-orange' : 'text-fd-foreground')}>{n.name}</span>
                        <span className="font-mono text-xs text-fd-muted-foreground">{n.tag}</span>
                        {rel ? (
                          <span className="mt-0.5 font-mono text-xs text-fd-foreground">
                            ↔ {[...rel].map((l) => LAYER_LETTER[l]).join(' ')}
                          </span>
                        ) : null}
                      </button>
                    </li>
                  );
                })}
              </ul>
            </fieldset>
          ))}
        </div>

        <aside aria-label={`Details for ${node.name}`} className="min-w-0 rounded-md border border-fd-border bg-fd-background p-3 lg:sticky lg:top-20 lg:self-start">
          <p className="m-0 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">
            {ZONES.find((z) => z.id === node.zone)?.title}
          </p>
          <h4 className="m-0 mt-0.5 text-base font-semibold text-lp-orange">{node.name}</h4>
          <PanelList title="Holds" tone="text-fd-foreground" glyph="●" items={node.holds} />
          <PanelList title="Crosses" tone="text-nm-accent" glyph="→" items={node.crosses} />
          <PanelList title="Denied" tone="text-nm-deny" glyph="✕" items={node.denied} />
          <div className="mt-3">
            <p className="m-0 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Connections on shown layers</p>
            {links.length === 0 ? (
              <p className="m-0 mt-1 text-sm text-fd-muted-foreground">None on the layers currently shown.</p>
            ) : (
              <ul className="m-0 mt-1 list-none space-y-1 p-0">
                {links.map((l, i) => (
                  <li key={i} className="m-0 text-sm leading-snug">
                    <span className="font-mono text-xs text-fd-muted-foreground">
                      [{LAYER_LETTER[l.layer]}] {l.dir === 'out' ? '→' : '←'}{' '}
                    </span>
                    <span className="font-medium">{l.other.name}</span>
                    <span className="text-fd-muted-foreground">: {l.label}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </aside>
      </div>
      <p className="sr-only" aria-live="polite" role="status">
        {`${node.name}. ${links.length} connection${links.length === 1 ? '' : 's'} on the ${layers.join(', ')} layer${layers.length > 1 ? 's' : ''}. ${node.denied[0]}.`}
      </p>
    </DiagramShell>
  );
}

function PanelList({ title, tone, glyph, items }: { title: string; tone: string; glyph: string; items: string[] }) {
  return (
    <div className="mt-3">
      <p className={cn('m-0 font-mono text-xs uppercase tracking-wider', tone)}>
        <span aria-hidden>{glyph} </span>
        {title}
      </p>
      <ul className="m-0 mt-1 list-none space-y-1 p-0">
        {items.map((t) => (
          <li key={t} className="m-0 text-sm leading-snug text-fd-foreground/90">{t}</li>
        ))}
      </ul>
    </div>
  );
}
