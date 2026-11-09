'use client';

import { useId, useState } from 'react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { Check, X } from 'lucide-react';
import { Tabs, tabPanelProps } from '@/components/ui/tabs';
import { cn } from '@/lib/cn';
import { EASE, dur } from '@/lib/motion';

type Adapter = { id: string; name: string; kind: string; status: string };
type Port = {
  id: string;
  label: string;
  role: string;
  contract: string[];
  may: string[];
  mayNot: string[];
  adapters: Adapter[];
};

const PORTS: Port[] = [
  {
    id: 'source-control',
    label: 'Source control',
    role: 'Reads and proposes changes to repositories.',
    contract: [
      'Typed requests and results for repository operations',
      'Every call carries the action context of the requesting principal',
      'Failures return structured diagnostics, not free text',
    ],
    may: ['Read repository content within the granted scope', 'Propose a change for review'],
    mayNot: ['Hold reusable credentials', 'Merge or release without the required identity', 'Reach outside the declared scope'],
    adapters: [{ id: 'github-scm', name: 'GitHub source-control provider', kind: 'Source control provider · numinance.ports.scm.v1', status: 'official provider · plugins/official/scm-github' }],
  },
  {
    id: 'ci',
    label: 'CI',
    role: 'Runs and reports on automated pipelines.',
    contract: [
      'Typed requests to start and observe pipeline runs',
      'Results are recorded as evidence inputs',
      'Same envelope and diagnostic shape as every other port',
    ],
    may: ['Trigger and observe runs within the granted scope', 'Report outcomes back to the host'],
    mayNot: ['Alter pipeline definitions outside a reviewed change', 'Read secrets directly', 'Approve its own output'],
    adapters: [{ id: 'github-actions', name: 'GitHub Actions CI provider', kind: 'CI provider · numinance.ports.ciexecutor.v1', status: 'official provider · plugins/official/ci-github-actions' }],
  },
  {
    id: 'verification',
    label: 'Verification',
    role: 'Checks that a change meets stated constraints.',
    contract: [
      'Typed requests describing what is to be checked',
      'Findings returned as structured results the host can attest to',
      'Independent of the component that authored the change',
    ],
    may: ['Read the material under verification', 'Report findings and a verdict'],
    mayNot: ['Modify the change under verification', 'Suppress or rewrite findings', 'Make network calls the host did not grant'],
    adapters: [
      { id: 'static', name: 'Repository / static verification provider', kind: 'Verification provider · numinance.ports.verifier.v1', status: 'official provider · plugins/official/verifier-repo' },
      { id: 'python', name: 'Python verification provider', kind: 'Verification provider · numinance.ports.verifier.v1', status: 'official provider · plugins/official/verifier-python' },
    ],
  },
  {
    id: 'knowledge',
    label: 'Knowledge',
    role: 'Supplies context to understanding and planning.',
    contract: [
      'Typed queries and results for context retrieval',
      'Provenance travels with every returned item',
      'Read-oriented by design',
    ],
    may: ['Return context for a scoped query', 'Attach provenance to results'],
    mayNot: ['Write to systems of record', 'Return content outside the granted scope', 'Act as an authorization source'],
    adapters: [{ id: 'sample-knowledge', name: 'Sample knowledge provider', kind: 'Knowledge provider (sample) · numinance.ports.knowledge.v1', status: 'example provider · plugins/examples/sample-knowledge' }],
  },
];

export function IntegrationsExplorer() {
  const uid = useId();
  const reduce = useReducedMotion();
  const [portId, setPortId] = useState(PORTS[0].id);
  const [adapterByPort, setAdapterByPort] = useState<Record<string, string>>({});
  const port = PORTS.find((p) => p.id === portId)!;
  const adapter = port.adapters.find((a) => a.id === adapterByPort[port.id]) ?? port.adapters[0];

  return (
    <div>
      <p className="nm-eyebrow mb-4">Interactive architecture preview</p>
      <Tabs
        items={PORTS.map((p) => ({ id: p.id, label: p.label }))}
        value={portId}
        onChange={setPortId}
        label="Select a port"
        idBase={uid}
        size="sm"
      />

      <div {...tabPanelProps(uid, port.id)} className="mt-6 overflow-hidden rounded-2xl border border-nm-line bg-nm-raised">
        <div className="grid md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1.15fr)]">
          {/* Adapters */}
          <div className="p-5 md:p-8">
            <p className="nm-eyebrow mb-1">Adapters</p>
            <p className="mb-5 text-sm text-nm-muted">{port.role}</p>
            <div role="radiogroup" aria-label={`Adapters for ${port.label}`} className="flex flex-col gap-3">
              {port.adapters.map((a) => {
                const on = a.id === adapter.id;
                return (
                  <button
                    key={a.id}
                    type="button"
                    role="radio"
                    aria-checked={on}
                    onClick={() => setAdapterByPort((s) => ({ ...s, [port.id]: a.id }))}
                    className={cn(
                      'min-h-14 rounded-xl border p-4 text-left transition-colors duration-200',
                      on ? 'border-nm-accent bg-nm-accent-soft' : 'border-nm-line hover:border-nm-line-strong hover:bg-nm-surface',
                    )}
                  >
                    <span className="block text-[0.95rem] font-medium text-nm-fg">{a.name}</span>
                    <span className="mt-1 block text-xs text-nm-muted">{a.kind}</span>
                    <span className="mt-3 inline-flex items-center rounded-full border border-nm-line-strong px-2.5 py-0.5 font-mono text-[0.7rem] text-nm-muted">
                      {a.status}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Socket + spine */}
          <div className="relative flex items-center justify-center border-y border-nm-line py-6 md:w-28 md:border-x md:border-y-0 md:py-0" aria-hidden>
            <svg viewBox="0 0 112 220" className="hidden h-full min-h-[220px] w-full md:block">
              <line x1="56" y1="0" x2="56" y2="220" stroke="var(--nm-line-strong)" strokeWidth="1" />
              <motion.g
                key={adapter.id}
                initial={reduce ? false : { x: -26, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                transition={{ duration: dur.panel, ease: EASE }}
              >
                <line x1="0" y1="110" x2="48" y2="110" stroke="var(--nm-accent)" strokeWidth="1.5" />
                <rect x="40" y="98" width="16" height="24" rx="3" fill="var(--nm-bg-raised)" stroke="var(--nm-accent)" strokeWidth="1.5" />
              </motion.g>
              <rect x="56" y="98" width="14" height="24" rx="3" fill="var(--nm-bg-raised)" stroke="var(--nm-line-strong)" strokeWidth="1.5" />
              <line x1="70" y1="110" x2="112" y2="110" stroke="var(--nm-line-strong)" strokeWidth="1.5" />
            </svg>
            <div className="flex items-center gap-3 md:hidden">
              <span className="h-px w-8 bg-nm-accent" />
              <span className="font-mono text-[0.72rem] uppercase tracking-[0.14em] text-nm-faint">same socket</span>
              <span className="h-px w-8 bg-nm-line-strong" />
            </div>
          </div>

          {/* Contract + boundary */}
          <div className="p-5 md:p-8" aria-live="polite">
            <AnimatePresence mode="wait" initial={false}>
              <motion.div
                key={`${port.id}-${adapter.id}`}
                initial={reduce ? false : { opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={reduce ? undefined : { opacity: 0 }}
                transition={{ duration: dur.panel, ease: EASE }}
              >
                <p className="nm-eyebrow mb-3">Shared contract · {port.label}</p>
                <ul className="space-y-2 border-l border-nm-accent pl-4 text-sm text-nm-fg">
                  {port.contract.map((c) => (
                    <li key={c}>{c}</li>
                  ))}
                </ul>

                <p className="nm-eyebrow mb-3 mt-8">Declared capability boundary · {adapter.name}</p>
                <div className="grid gap-5 sm:grid-cols-2">
                  <BoundaryList title="May" items={port.may} ok />
                  <BoundaryList title="May not" items={port.mayNot} />
                </div>
                <p className="mt-6 text-xs leading-relaxed text-nm-faint">
                  Illustrative summary. Typed, governed interfaces instead of unrestricted access; each provider's manifest declares its capabilities and network destinations. Authoritative contracts are the protos under numinance-spec/proto/numinance/ports.
                </p>
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>

      <p className="mt-5 max-w-3xl text-sm leading-relaxed text-nm-muted">
        Labels name where each provider sits in the public tree (numinance-spec/plugins). Pre-alpha: presence in the tree does not imply production support.
      </p>
    </div>
  );
}

function BoundaryList({ title, items, ok = false }: { title: string; items: string[]; ok?: boolean }) {
  const Icon = ok ? Check : X;
  return (
    <div>
      <p className="mb-2 font-mono text-[0.72rem] uppercase tracking-[0.14em] text-nm-muted">{title}</p>
      <ul className="space-y-2">
        {items.map((i) => (
          <li key={i} className="flex gap-2 text-sm text-nm-muted">
            <Icon className={cn('mt-0.5 size-3.5 shrink-0', ok ? 'text-nm-ok' : 'text-nm-deny')} aria-hidden />
            <span>{i}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
