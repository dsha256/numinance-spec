'use client';

import { useId, useState } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { Lock, Unlock } from 'lucide-react';
import { Tabs, tabPanelProps } from '@/components/ui/tabs';
import { cn } from '@/lib/cn';
import { EASE, dur } from '@/lib/motion';

const CATEGORIES = [
  { name: 'Protobuf', note: 'Typed ports and governance annotations' },
  { name: 'SDKs', note: 'Go, Python and TypeScript' },
  { name: 'Schemas', note: 'JSON Schemas for configuration kinds' },
  { name: 'Conformance', note: 'Suites for independent implementers' },
  { name: 'Adapters', note: 'Integration adapters' },
  { name: 'Verifier', note: 'Offline evidence verification' },
  { name: 'CLI', note: 'The numi command line' },
  { name: 'RFCs', note: 'Public design record' },
] as const;

const PACKAGES = [
  {
    pkg: 'numinance.annotations.v1',
    role: 'Governance annotations',
    members: ['RiskClass', 'ExecMode'],
  },
  {
    pkg: 'numinance.types.v1',
    role: 'Shared types',
    members: ['PrincipalKind', 'ResidencyClass', 'TrustClass', 'AutonomyLevel', 'ChangeState', 'Digest', 'Reference'],
  },
  {
    pkg: 'numinance.plugin.v1',
    role: 'Plugin boundary',
    members: ['PluginService', 'HostService', 'Envelope', 'ActionContext', 'ActionResult', 'Diagnostic', 'Manifest and capability descriptions'],
  },
  {
    pkg: 'numinance.kinds.v1',
    role: 'Configuration envelope',
    members: ['apiVersion', 'kind', 'metadata', 'spec', 'extensions'],
  },
] as const;

const SDKS = [
  {
    id: 'go',
    label: 'Go',
    path: 'numinance-spec/sdk/go',
    purpose: 'Generated contract bindings and plugin entry points for Go. The Go tree also hosts the offline evidence verifier.',
  },
  {
    id: 'python',
    label: 'Python',
    path: 'numinance-spec/sdk/python',
    purpose: 'Generated contract bindings and plugin entry points for Python.',
  },
  {
    id: 'typescript',
    label: 'TypeScript',
    path: 'numinance-spec/sdk/typescript',
    purpose: 'Generated contract bindings and plugin entry points for TypeScript.',
  },
] as const;

/** Private engine | Open protocol composition with the eight public categories beneath. */
export function ProtocolSplit() {
  return (
    <div>
      <div className="grid overflow-hidden rounded-2xl border border-nm-line-strong md:grid-cols-[1fr_auto_1fr]">
        <div className="bg-nm-surface p-6 md:p-10">
          <div className="mb-6 flex items-center justify-between">
            <h3 className="text-xl font-semibold tracking-tight">Private Engine</h3>
            <span className="inline-flex items-center gap-1.5 rounded-full border border-nm-line-strong px-3 py-1 font-mono text-[0.68rem] uppercase tracking-wider text-nm-muted">
              <Lock className="size-3" aria-hidden /> Proprietary
            </span>
          </div>
          <ul className="space-y-3 text-sm leading-6 text-nm-muted">
            <li>Autonomous engineering intelligence</li>
            <li>Orchestration runtime</li>
            <li>Understand, plan, act, verify, evidence loop</li>
          </ul>
          <p className="mt-6 text-xs leading-5 text-nm-faint">Not part of numinance-spec. Internals are not published.</p>
        </div>

        <div className="relative flex items-center justify-center border-y border-nm-line-strong bg-nm-bg px-6 py-4 md:w-28 md:border-x md:border-y-0 md:py-0">
          <span className="font-mono text-[0.68rem] uppercase tracking-[0.2em] text-nm-accent md:[writing-mode:vertical-rl]">
            Typed contract boundary
          </span>
        </div>

        <div className="bg-nm-raised p-6 md:p-10">
          <div className="mb-6 flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-xl font-semibold tracking-tight">Open Protocol</h3>
            <span className="inline-flex items-center gap-1.5 rounded-full border border-nm-accent px-3 py-1 font-mono text-[0.68rem] uppercase tracking-wider text-nm-accent">
              <Unlock className="size-3" aria-hidden /> Apache-2.0 · numinance-spec
            </span>
          </div>
          <ul className="space-y-3 text-sm leading-6 text-nm-muted">
            <li>Inspect the contracts</li>
            <li>Generate clients and build compatible adapters</li>
            <li>Validate configuration and verify evidence</li>
          </ul>
          <p className="mt-6 text-xs leading-5 text-nm-faint">No dependency on the proprietary engine&rsquo;s internals.</p>
        </div>
      </div>

      <ul className="grid grid-cols-2 gap-px border-x border-b border-nm-line-strong bg-nm-line sm:grid-cols-4">
        {CATEGORIES.map((c, i) => (
          <motion.li
            key={c.name}
            initial={{ opacity: 0, y: 16 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: '0px 0px -10% 0px' }}
            transition={{ duration: dur.enter, ease: EASE, delay: (i % 4) * 0.07 }}
            className="bg-nm-bg p-4 md:p-5"
          >
            <p className="text-sm font-semibold">{c.name}</p>
            <p className="mt-1 text-xs leading-5 text-nm-muted">{c.note}</p>
          </motion.li>
        ))}
      </ul>
    </div>
  );
}

/** Inventory of public proto packages. Identifiers only; no schema text is reproduced. */
export function ProtoPackages() {
  return (
    <div className="rounded-2xl border border-nm-line p-5 md:p-8">
      <div className="mb-6 flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-lg font-semibold tracking-tight">Protocol packages</h3>
        <p className="font-mono text-xs text-nm-faint">contract source: numinance-spec/proto/</p>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        {PACKAGES.map((p) => (
          <div key={p.pkg} className="rounded-xl border border-nm-line bg-nm-raised p-5 transition-colors duration-200 hover:border-nm-line-strong">
            <p className="font-mono text-sm font-medium text-nm-accent">{p.pkg}</p>
            <p className="mt-1 text-sm text-nm-muted">{p.role}</p>
            <ul className="mt-4 flex flex-wrap gap-1.5" aria-label={`${p.pkg} contents`}>
              {p.members.map((m) => (
                <li key={m} className="rounded-md border border-nm-line bg-nm-bg px-2 py-1 font-mono text-[0.72rem] text-nm-fg">
                  {m}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <p className="mt-5 text-xs leading-5 text-nm-faint">
        Members are listed by name. Enum values, fields and message shapes are defined in the proto sources.
      </p>
    </div>
  );
}

export function SdkShowcase() {
  const [tab, setTab] = useState<string>('go');
  const idBase = useId();
  const sdk = SDKS.find((s) => s.id === tab) ?? SDKS[0];
  return (
    <div className="rounded-2xl border border-nm-line p-5 md:p-8">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <h3 className="text-lg font-semibold tracking-tight">SDKs</h3>
        <Tabs items={SDKS.map((s) => ({ id: s.id, label: s.label }))} value={tab} onChange={setTab} label="SDK language" idBase={idBase} size="sm" />
      </div>
      <div {...tabPanelProps(idBase, tab)} className={cn('min-h-[9rem] rounded-xl outline-offset-4')}>
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={tab}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: dur.tab, ease: EASE }}
          >
            <p className="nm-eyebrow mb-2">Intended location</p>
            <p className="break-all font-mono text-base text-nm-fg">{sdk.path}/</p>
            <p className="mt-4 max-w-[56ch] text-sm leading-6 text-nm-muted">{sdk.purpose}</p>
          </motion.div>
        </AnimatePresence>
      </div>
      <p className="mt-5 border-t border-nm-line pt-4 text-xs leading-5 text-nm-faint">SDK details are published with the specification.</p>
    </div>
  );
}
