'use client';

import { useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramShell, Segmented } from './kit';
import { StatusBadge } from '../status';

type Mode = 'managed' | 'sovereign' | 'airgapped';

interface ModeDef {
  label: string;
  heading: string;
  /** Whether a link to a vendor control plane exists at all. */
  link: 'outbound' | 'none' | 'unavailable';
  linkLabel: string;
  summary: string;
  rows: { k: string; v: string }[];
}

const MODES: Record<Mode, ModeDef> = {
  managed: {
    label: 'Managed',
    heading: 'Managed: the vendor runs the thin control plane',
    link: 'outbound',
    linkLabel: 'outbound-only link from the runner',
    summary:
      'The runner opens an outbound connection to the vendor-operated control plane. Nothing in the vendor plane can reach into yours, and only the metadata listed below may travel.',
    rows: [
      { k: 'Who operates the control plane', v: 'The vendor, as a thin metadata service and dashboard.' },
      { k: 'Link direction', v: 'Outbound from the runner. No inbound path into your network.' },
      { k: 'Workflow payloads', v: 'References and digests only, encrypted with a key you hold; payloads outside the allow-list or over 16 KB are rejected.' },
      { k: 'Human approvals', v: 'Made in the dashboard by a human principal and validated against the author identity before acceptance.' },
      { k: 'If the link is cut', v: 'Installed runtime keeps applying policy, completing authorised work and exporting evidence.' },
    ],
  },
  sovereign: {
    label: 'Sovereign',
    heading: 'Sovereign: no dependency on a vendor control plane',
    link: 'none',
    linkLabel: 'no vendor link required',
    summary:
      'The product works without the vendor control plane. You run the control components yourself, so nothing needs to leave your network for the system to function.',
    rows: [
      { k: 'Who operates the control plane', v: 'You. The runtime has no dependency on the vendor-operated one.' },
      { k: 'Link direction', v: 'None required. Updates are pulled by the runner inside your own update window.' },
      { k: 'Workflow payloads', v: 'Same reference-and-digest rule; the store they point to is yours.' },
      { k: 'Human approvals', v: 'Made through your own identity provider, under the same no-self-approval rule.' },
      { k: 'Licence', v: 'A signed, offline licence file. Expiry stops upgrades and new packs, never authorised work.' },
    ],
  },
  airgapped: {
    label: 'Air-gapped · Coming soon',
    heading: 'Air-gapped: a later design milestone',
    link: 'unavailable',
    linkLabel: 'not available yet',
    summary:
      'Air-gapped operation is on the roadmap as a later milestone after managed and sovereign. No package, date or procedure is promised, so the diagram shows no data flow for it.',
    rows: [
      { k: 'Status', v: 'Coming soon. Not designed in enough detail to document exact crossings.' },
      { k: 'Today', v: 'Choose sovereign to keep the control plane out of your network.' },
    ],
  },
};

const PLANE = [
  ['Runner', 'host agent'],
  ['Engine', 'workflows'],
  ['Sandbox + harness', 'per work unit'],
  ['Broker + egress proxy', 'handles, default-deny'],
  ['Artifact store', 'content by reference'],
  ['Evidence store', 'append-only'],
  ['Source and credentials', 'repos, CI, secret managers'],
] as const;

const MAY_CROSS = [
  ['References', 'change_id, work_unit_id, run_id, artifact_ref'],
  ['Digests', 'sha256 of artifacts, policy and plan digests'],
  ['Statuses', 'state and gate outcomes as enum codes, timestamps'],
  ['Non-content telemetry', 'runner heartbeat, versions, breaker position, redacted structured events'],
] as const;

const NEVER_CROSS = [
  ['Source code', 'worktrees and repositories stay in your plane'],
  ['Diffs and findings bodies', 'stored as artifacts, travel as references'],
  ['Prompts and completions', 'content, so claim-checked into your artifact store'],
  ['Secrets and credentials', 'broker mints handles locally; sandbox holds none'],
  ['Evidence content', 'records stay in your append-only store; only digests and status can be summarised'],
] as const;

export function DeploymentBoundary() {
  const [mode, setMode] = useState<Mode>('managed');
  const m = MODES[mode];
  const off = mode !== 'managed' ? true : false;
  const disabled = mode === 'airgapped';

  const controls = (
    <Segmented
      label="Deployment mode"
      value={mode}
      onChange={setMode}
      options={(Object.keys(MODES) as Mode[]).map((k) => ({ value: k, label: MODES[k].label }))}
    />
  );

  const legend = (
    <>
      <span className="text-nm-ok">✓ may cross</span>
      <span className="text-nm-deny">✕ never leaves</span>
      <span>solid line: link exists · dashed: no link</span>
      <span>customer plane is identical in every mode</span>
    </>
  );

  const explanation = (
    <>
      <p>
        The customer plane is fixed in every deployment mode: {PLANE.map((p) => p[0]).join(', ')}. Only the link to a vendor control plane
        changes. Everything is design intent under development; nothing here is connected to a running engine.
      </p>
      <ul>
        {(Object.keys(MODES) as Mode[]).map((k) => (
          <li key={k}>
            <strong>{MODES[k].label}.</strong> {MODES[k].summary}
            <ul>
              {MODES[k].rows.map((r) => (
                <li key={r.k}>{r.k}: {r.v}</li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
      <p>May cross to the vendor in managed mode: {MAY_CROSS.map((x) => `${x[0]} (${x[1]})`).join('; ')}.</p>
      <p>Never leaves the customer plane in any mode: {NEVER_CROSS.map((x) => `${x[0]} (${x[1]})`).join('; ')}.</p>
    </>
  );

  return (
    <DiagramShell
      title="Deployment boundary"
      question="In each deployment mode, exactly what may cross to the vendor?"
      controls={controls}
      legend={legend}
      explanation={explanation}
    >
      <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(7rem,9rem)_minmax(0,0.8fr)] md:items-stretch">
        <div className="rounded-md border border-fd-foreground/40 p-3">
          <p className="m-0 font-mono text-xs uppercase tracking-wider text-fd-foreground">Customer data plane · fixed</p>
          <ul className="m-0 mt-2 grid list-none grid-cols-1 gap-1.5 p-0 min-[460px]:grid-cols-2">
            {PLANE.map(([n, s]) => (
              <li key={n} className="m-0 rounded-sm border border-fd-border bg-fd-background px-2 py-1.5">
                <span className="block text-sm font-medium">{n}</span>
                <span className="block font-mono text-xs text-fd-muted-foreground">{s}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="flex flex-row items-center justify-center gap-2 md:flex-col" aria-hidden="true">
          <div
            className={cn(
              'h-0.5 w-full flex-1 border-t-2 transition-[border-color,opacity] motion-reduce:transition-none md:h-full md:w-0.5 md:flex-none md:grow md:border-l-2 md:border-t-0',
              m.link === 'outbound' ? 'border-solid border-lp-orange' : 'border-dashed border-fd-border opacity-70',
            )}
          />
          <span className={cn('shrink-0 text-center font-mono text-xs', m.link === 'outbound' ? 'text-lp-orange' : 'text-fd-muted-foreground')}>
            {m.link === 'outbound' ? '→ metadata only' : '✕ no link'}
          </span>
          <div
            className={cn(
              'h-0.5 w-full flex-1 border-t-2 transition-[border-color,opacity] motion-reduce:transition-none md:h-full md:w-0.5 md:flex-none md:grow md:border-l-2 md:border-t-0',
              m.link === 'outbound' ? 'border-solid border-lp-orange' : 'border-dashed border-fd-border opacity-70',
            )}
          />
        </div>

        <div
          className={cn(
            'rounded-md border p-3 transition-colors motion-reduce:transition-none',
            m.link === 'outbound' ? 'border-fd-foreground/40' : 'border-dashed border-fd-border',
          )}
        >
          <p className="m-0 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">
            {mode === 'managed' ? 'Vendor control plane' : mode === 'sovereign' ? 'Vendor control plane · not used' : 'Not available'}
          </p>
          <p className="m-0 mt-2 text-sm leading-snug text-fd-muted-foreground">
            {mode === 'managed'
              ? 'Metadata service and dashboard. Holds change status, fleet state and licence state.'
              : mode === 'sovereign'
                ? 'Optional and absent from the data path. You operate the control components.'
                : 'Later milestone. Nothing to connect.'}
          </p>
        </div>
      </div>

      <div className="mt-3 rounded-md border border-fd-border bg-fd-background p-3">
        <div className="flex flex-wrap items-center gap-2">
          <h4 className="m-0 text-sm font-semibold">{m.heading}</h4>
          {mode === 'airgapped' ? <StatusBadge kind="coming-soon" /> : <StatusBadge kind="designed" />}
        </div>
        <p className="m-0 mt-1 text-sm text-fd-muted-foreground">{m.summary}</p>
        <dl className="m-0 mt-2 grid gap-x-4 gap-y-1.5 text-sm sm:grid-cols-[minmax(0,13rem)_minmax(0,1fr)]">
          {m.rows.map((r) => (
            <div key={r.k} className="contents">
              <dt className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground sm:pt-0.5">{r.k}</dt>
              <dd className="m-0">{r.v}</dd>
            </div>
          ))}
        </dl>
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <div className={cn('rounded-md border p-3 transition-opacity motion-reduce:transition-none', 'border-nm-ok/50', mode !== 'managed' && 'opacity-60')}>
          <p className="m-0 font-mono text-xs uppercase tracking-wider text-nm-ok">
            ✓ May cross to the vendor{mode === 'managed' ? '' : disabled ? ' (nothing to describe yet)' : ' (nothing required in this mode)'}
          </p>
          <ul className="m-0 mt-2 list-none space-y-1.5 p-0">
            {MAY_CROSS.map(([k, v]) => (
              <li key={k} className={cn('m-0 text-sm', off && 'text-fd-muted-foreground')}>
                <span className="font-medium">{k}</span>
                <span className="text-fd-muted-foreground">: {v}</span>
              </li>
            ))}
          </ul>
        </div>
        <div className="rounded-md border border-nm-deny/50 p-3">
          <p className="m-0 font-mono text-xs uppercase tracking-wider text-nm-deny">✕ Never leaves the customer plane</p>
          <ul className="m-0 mt-2 list-none space-y-1.5 p-0">
            {NEVER_CROSS.map(([k, v]) => (
              <li key={k} className="m-0 text-sm">
                <span className="font-medium">{k}</span>
                <span className="text-fd-muted-foreground">: {v}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <p className="sr-only" aria-live="polite" role="status">
        {mode === 'managed'
          ? 'Managed mode. An outbound link carries references, digests, statuses and non-content telemetry only. Source, diffs, prompts, secrets and evidence content never leave.'
          : mode === 'sovereign'
            ? 'Sovereign mode. No vendor link is required. Source, diffs, prompts, secrets and evidence content never leave.'
            : 'Air-gapped mode is coming soon, a later milestone. No data flow is described.'}
      </p>
    </DiagramShell>
  );
}
