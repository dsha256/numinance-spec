'use client';

import { useId, useState } from 'react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { ChevronDown } from 'lucide-react';
import { cn } from '@/lib/cn';
import { EASE, dur, stagger } from '@/lib/motion';

type NodeId = 'intent' | 'engine' | 'policy' | 'protocol' | 'execution' | 'evidence' | 'verifier';
type InteractiveId = Exclude<NodeId, 'intent' | 'verifier'>;
type RegionKind = 'private' | 'open' | 'customer';

type Detail = {
  responsibility: string;
  private: string;
  open: string;
  line: string;
  listHeading?: string;
  list?: string[];
};

type NodeDef = {
  id: NodeId;
  index: string;
  title: string;
  sub: string;
  region?: RegionKind;
  tag?: string;
  chips?: string[];
  interactive: boolean;
  /** Plain-language description for non-interactive nodes. */
  note?: string;
};

const REGION_LABEL: Record<RegionKind, string> = {
  private: 'Proprietary',
  open: 'Apache-2.0 · numinance-spec',
  customer: 'Customer-controlled environment',
};

const NODES: NodeDef[] = [
  {
    id: 'intent',
    index: '01',
    title: 'Engineering intent / ticket',
    sub: 'The objective that starts the work.',
    interactive: false,
    note: 'Work begins as an objective: a ticket, a request or a described change. Nothing here grants authority.',
  },
  {
    id: 'engine',
    index: '02',
    title: 'Numinance Engine',
    sub: 'Reasoning and orchestration. Proposes; never authorizes.',
    region: 'private',
    chips: ['understand', 'plan', 'act', 'verify', 'evidence'],
    interactive: true,
  },
  {
    id: 'policy',
    index: '03',
    title: 'Policy decision + scoped grant',
    sub: 'Evaluated outside the model.',
    tag: 'Outside the model',
    interactive: true,
  },
  {
    id: 'protocol',
    index: '04',
    title: 'Open protocol · plugins · host boundary',
    sub: 'Typed, governed interfaces.',
    region: 'open',
    interactive: true,
  },
  {
    id: 'execution',
    index: '05',
    title: 'Execution environment',
    sub: 'Sandboxed, in the customer account.',
    region: 'customer',
    interactive: true,
  },
  {
    id: 'evidence',
    index: '06',
    title: 'Evidence · artifacts · receipts',
    sub: 'Records preserved for outside verification.',
    region: 'customer',
    interactive: true,
  },
  {
    id: 'verifier',
    index: '07',
    title: 'Independent offline verification',
    sub: 'The public evidence/v1 verifier.',
    region: 'open',
    interactive: false,
    note: 'The public evidence/v1 verifier can run offline, apart from the engine. It checks exported evidence against trust material the caller pins.',
  },
];

const NODE = Object.fromEntries(NODES.map((n) => [n.id, n])) as Record<NodeId, NodeDef>;

const EDGES = [
  { id: 'e1', from: 'intent', to: 'engine', label: 'objective' },
  { id: 'e2', from: 'engine', to: 'policy', label: 'proposed privileged capability' },
  { id: 'e3', from: 'policy', to: 'protocol', label: 'authorized invocation' },
  { id: 'e4', from: 'protocol', to: 'execution', label: 'typed call' },
  { id: 'e5', from: 'execution', to: 'evidence', label: 'produces' },
  { id: 'e6', from: 'evidence', to: 'verifier', label: 'exported' },
] as const satisfies readonly { id: string; from: NodeId; to: NodeId; label: string }[];

const DETAIL: Record<InteractiveId, Detail> = {
  engine: {
    responsibility:
      'Turns an objective into bounded work. It follows a fixed loop of understand, plan, act, verify and evidence, and proposes the capabilities it needs along the way.',
    private:
      'The reasoning and orchestration runtime. It is proprietary and is not part of numinance-spec.',
    open: 'Only its boundary. The engine meets the rest of the system through typed requests and recorded results.',
    line: 'The intelligence is private. The contract is open.',
  },
  policy: {
    responsibility:
      'Evaluates a proposed privileged capability outside the language model. A permitted request receives a scoped grant, bounded by explicit authority, risk, environment and expiration.',
    private: 'The engine’s own reasoning for wanting the action. Policy does not take the model’s word for it.',
    open: 'The governance vocabulary: capabilities, risk classes, execution modes and scoped authority are defined in the public spec.',
    line: 'A proposed action is not an authorization.',
  },
  protocol: {
    responsibility:
      'The typed boundary between the engine, plugins and the host. Plugins and executor adapters implement it. The host mediates it.',
    private: 'Nothing from the engine’s internals crosses this boundary.',
    open: 'The contract itself, published as numinance-spec under Apache-2.0.',
    line: 'Extensions get typed, governed interfaces instead of unrestricted access.',
    listHeading: 'Public boundary',
    list: [
      'numinance.plugin.v1',
      'PluginService',
      'HostService',
      'ActionContext',
      'ActionResult',
      'Envelope',
      'Diagnostic',
      'Digest',
      'Reference',
      'numinance.kinds.v1',
    ],
  },
  execution: {
    responsibility:
      'Where authorized work actually runs: a sandbox inside the customer’s controlled environment. A credential broker and controlled egress sit between the work and any secret.',
    private:
      'Customer code, credentials, inference and infrastructure actions. The design keeps them in the customer’s cloud account.',
    open: 'The host boundary through which sandboxed work is invoked.',
    line: 'Credentials are referenced by short-lived opaque handles. A coding agent is never handed a reusable secret.',
    listHeading: 'Concepts',
    list: ['customer environment', 'sandbox', 'credential broker', 'short-lived opaque handles', 'controlled egress'],
  },
  evidence: {
    responsibility:
      'Execution produces evidence, artifacts and receipts that can be exported and checked without the engine. The public verifier can run offline.',
    private: 'Nothing is needed from the engine to check an export.',
    open: 'The evidence/v1 specification and the public verifier, run with numi evidence verify and caller-supplied keys.',
    line: 'A verified result is checked against keys you pinned, not keys the export supplies.',
    listHeading: 'Pinned by the caller',
    list: ['runner key(s)', 'log key', 'origin'],
  },
};

const SUMMARY =
  'Diagram summary. Work flows top to bottom through seven layers. An engineering ticket reaches the private Numinance Engine, which proposes privileged capabilities. An external policy decision, outside the model, turns a proposal into a scoped grant or a denial. An authorized invocation crosses the open protocol and host boundary, defined in numinance-spec under Apache-2.0. Work runs in a customer-controlled execution environment and produces evidence, artifacts and receipts. An independent offline verifier, the public evidence/v1 verifier, checks that evidence. This is a conceptual explainer, not a claim that every layer is a literal synchronous runtime hop.';

const INTERACTIVE = NODES.filter((n) => n.interactive).map((n) => n.id) as InteractiveId[];

const HATCH = {
  backgroundImage: 'repeating-linear-gradient(135deg, var(--nm-line) 0 1px, transparent 1px 9px)',
} as const;

function connected(sel: NodeId): Set<NodeId> {
  const s = new Set<NodeId>([sel]);
  for (const e of EDGES) {
    if (e.from === sel) s.add(e.to);
    if (e.to === sel) s.add(e.from);
  }
  return s;
}

function RegionTag({ kind, className }: { kind: RegionKind; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2 font-mono text-[0.7rem] uppercase tracking-[0.12em] text-nm-faint', className)}>
      <span
        aria-hidden
        className={cn(
          'inline-block size-3 shrink-0 rounded-[3px] border',
          kind === 'private' && 'border-dashed border-nm-fg/50',
          kind === 'open' && 'border-nm-fg/50 bg-nm-fg/15',
          kind === 'customer' && 'border-nm-line-strong',
        )}
        style={kind === 'private' ? HATCH : undefined}
      />
      {REGION_LABEL[kind]}
    </span>
  );
}

/* ───────────── Detail panel ───────────── */

function NodeDetail({ id }: { id: InteractiveId }) {
  const d = DETAIL[id];
  const n = NODE[id];
  return (
    <div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span className="font-mono text-[0.72rem] text-nm-accent">{n.index}</span>
        {n.region && <RegionTag kind={n.region} />}
        {n.tag && <span className="font-mono text-[0.7rem] uppercase tracking-[0.12em] text-nm-faint">{n.tag}</span>}
      </div>
      <h3 className="mt-3 text-xl font-semibold tracking-tight md:text-2xl">{n.title}</h3>
      <p className="mt-3 text-[0.95rem] leading-relaxed text-nm-muted">{d.responsibility}</p>

      <p className="mt-6 border-l-2 border-nm-accent pl-4 text-base font-medium leading-snug text-nm-fg md:text-lg">{d.line}</p>

      <dl className="mt-6 grid gap-px overflow-hidden rounded-lg border border-nm-line bg-nm-line sm:grid-cols-2">
        <div className="bg-nm-raised p-4">
          <dt className="nm-eyebrow mb-2">What’s private</dt>
          <dd className="text-sm leading-relaxed text-nm-muted">{d.private}</dd>
        </div>
        <div className="bg-nm-raised p-4">
          <dt className="nm-eyebrow mb-2">What’s open</dt>
          <dd className="text-sm leading-relaxed text-nm-muted">{d.open}</dd>
        </div>
      </dl>

      {d.list && (
        <div className="mt-6">
          <p className="nm-eyebrow mb-3">{d.listHeading}</p>
          <ul className="flex flex-wrap gap-2">
            {d.list.map((item) => (
              <li key={item} className="rounded-md border border-nm-line bg-nm-raised px-2.5 py-1 font-mono text-[0.75rem] text-nm-fg">
                {item}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

/* ───────────── Desktop graph ───────────── */

function Edge({
  label,
  active,
  inside,
  reduce,
}: {
  label: string;
  active: boolean;
  inside?: boolean;
  reduce: boolean;
}) {
  return (
    <div className="relative h-12" aria-hidden>
      <svg
        width="16"
        height="48"
        viewBox="0 0 16 48"
        fill="none"
        className={cn(
          'absolute top-0 transition-colors duration-200',
          inside ? 'left-5' : 'left-8',
          active ? 'text-nm-accent' : 'text-nm-line-strong',
        )}
      >
        <motion.line
          x1="8"
          y1="0"
          x2="8"
          y2="41"
          stroke="currentColor"
          strokeWidth="1.5"
          variants={{
            hidden: { pathLength: reduce ? 1 : 0 },
            show: { pathLength: 1, transition: { duration: 0.5, ease: EASE } },
          }}
        />
        <path d="M3 37 L8 44 L13 37" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      <span
        className={cn(
          'absolute inset-y-0 flex items-center font-mono text-[0.72rem] transition-colors duration-200',
          inside ? 'left-12' : 'left-14',
          active ? 'text-nm-accent' : 'text-nm-faint',
        )}
      >
        {label}
      </span>
    </div>
  );
}

function GraphNode({
  def,
  selected,
  faded,
  related,
  onSelect,
}: {
  def: NodeDef;
  selected: boolean;
  faded: boolean;
  related: boolean;
  onSelect: (id: InteractiveId) => void;
}) {
  const body = (
    <>
      <span className="font-mono text-[0.72rem] text-nm-faint">{def.index}</span>
      <span className="min-w-0 flex-1">
        <span className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="text-[0.98rem] font-semibold leading-tight tracking-tight">{def.title}</span>
          {def.tag && <span className="font-mono text-[0.68rem] uppercase tracking-[0.12em] text-nm-faint">{def.tag}</span>}
        </span>
        <span className="mt-1 block text-[0.84rem] leading-snug text-nm-muted">{def.sub}</span>
        {def.chips && (
          <span className="mt-3 flex flex-wrap gap-1.5">
            {def.chips.map((c, i) => (
              <span
                key={c}
                className="rounded border border-nm-line bg-nm-bg px-1.5 py-0.5 font-mono text-[0.68rem] text-nm-muted"
              >
                {i + 1} {c}
              </span>
            ))}
          </span>
        )}
      </span>
    </>
  );
  const base = cn(
    'relative flex w-full items-start gap-4 rounded-lg border px-4 py-3.5 text-left transition-[opacity,border-color,background-color] duration-200',
    faded && 'opacity-40',
  );
  if (!def.interactive) {
    return (
      <div className={cn(base, 'bg-nm-bg/60', related ? 'border-nm-accent/60' : 'border-nm-line')}>{body}</div>
    );
  }
  return (
    <button
      type="button"
      data-node={def.id}
      aria-pressed={selected}
      onClick={() => onSelect(def.id as InteractiveId)}
      onFocus={() => onSelect(def.id as InteractiveId)}
      onPointerEnter={(e) => {
        if (e.pointerType === 'mouse') onSelect(def.id as InteractiveId);
      }}
      className={cn(
        base,
        'min-h-11 cursor-pointer bg-nm-raised hover:border-nm-line-strong',
        selected ? 'border-nm-accent bg-nm-accent-soft' : 'border-nm-line-strong',
      )}
    >
      {selected && <span aria-hidden className="absolute inset-y-2 left-0 w-0.5 rounded-full bg-nm-accent" />}
      {body}
    </button>
  );
}

function Region({ kind, children }: { kind: RegionKind; children: React.ReactNode }) {
  return (
    <div
      className={cn(
        'rounded-xl border px-3 pb-3',
        kind === 'private' && 'border-dashed border-nm-line-strong',
        kind === 'open' && 'border-nm-line-strong bg-nm-raised/40',
        kind === 'customer' && 'border-nm-line',
      )}
      style={kind === 'private' ? HATCH : undefined}
    >
      <div className="py-2.5">
        <RegionTag kind={kind} />
      </div>
      {children}
    </div>
  );
}

function DesktopGraph({
  selected,
  onSelect,
  reduce,
}: {
  selected: InteractiveId;
  onSelect: (id: InteractiveId) => void;
  reduce: boolean;
}) {
  const near = connected(selected);
  const hiddenItem = reduce ? { opacity: 1, y: 0 } : { opacity: 0, y: 14 };
  const item = {
    hidden: hiddenItem,
    show: { opacity: 1, y: 0, transition: { duration: dur.enter, ease: EASE } },
  };
  const edge = (id: (typeof EDGES)[number]['id'], inside?: boolean) => {
    const e = EDGES.find((x) => x.id === id)!;
    return (
      <motion.div variants={item}>
        <Edge label={e.label} active={e.from === selected || e.to === selected} inside={inside} reduce={reduce} />
      </motion.div>
    );
  };
  const node = (id: NodeId) => (
    <motion.div variants={item}>
      <GraphNode
        def={NODE[id]}
        selected={id === selected}
        faded={!near.has(id)}
        related={id !== selected && near.has(id)}
        onSelect={onSelect}
      />
    </motion.div>
  );

  const move = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const dir = e.key === 'ArrowDown' || e.key === 'ArrowRight' ? 1 : e.key === 'ArrowUp' || e.key === 'ArrowLeft' ? -1 : 0;
    if (!dir) return;
    const i = INTERACTIVE.indexOf(selected);
    const next = INTERACTIVE[(i + dir + INTERACTIVE.length) % INTERACTIVE.length];
    e.preventDefault();
    e.currentTarget.querySelector<HTMLButtonElement>(`[data-node="${next}"]`)?.focus();
  };

  return (
    <motion.div
      role="group"
      aria-label="Numinance architecture layers, top to bottom. Arrow keys move between selectable layers."
      onKeyDown={move}
      initial="hidden"
      whileInView="show"
      viewport={{ once: true, amount: 0.12 }}
      transition={{ staggerChildren: stagger.node }}
    >
      {node('intent')}
      {edge('e1')}
      <motion.div variants={item}>
        <Region kind="private">{node('engine')}</Region>
      </motion.div>
      {edge('e2')}
      {node('policy')}
      {edge('e3')}
      <motion.div variants={item}>
        <Region kind="open">{node('protocol')}</Region>
      </motion.div>
      {edge('e4')}
      <motion.div variants={item}>
        <Region kind="customer">
          {node('execution')}
          {edge('e5', true)}
          {node('evidence')}
        </Region>
      </motion.div>
      {edge('e6')}
      <motion.div variants={item}>
        <Region kind="open">{node('verifier')}</Region>
      </motion.div>
    </motion.div>
  );
}

/* ───────────── Mobile stepper ───────────── */

function MobileStepper({ selected, onSelect }: { selected: InteractiveId; onSelect: (id: InteractiveId) => void }) {
  const uid = useId();
  return (
    <ol className="md:hidden" aria-label="Numinance architecture layers, top to bottom">
      {NODES.map((n, i) => {
        const edgeIn = EDGES.find((e) => e.to === n.id);
        const isSel = n.id === selected;
        const edgeActive = !!edgeIn && (edgeIn.from === selected || edgeIn.to === selected);
        const last = i === NODES.length - 1;
        return (
          <li key={n.id} className="relative pl-11">
            {!last && <span aria-hidden className="absolute bottom-0 left-[0.9rem] top-8 w-px bg-nm-line-strong" />}
            <span
              aria-hidden
              className={cn(
                'absolute left-0 top-0 grid size-7 place-items-center rounded-full border bg-nm-bg font-mono text-[0.68rem]',
                isSel ? 'border-nm-accent text-nm-accent' : 'border-nm-line-strong text-nm-muted',
              )}
            >
              {n.index}
            </span>
            {edgeIn && (
              <p className={cn('mb-2 font-mono text-[0.72rem]', edgeActive ? 'text-nm-accent' : 'text-nm-faint')}>
                {edgeIn.label}
              </p>
            )}
            <div
              className={cn(
                'mb-6 rounded-lg border',
                isSel ? 'border-nm-accent bg-nm-accent-soft' : 'border-nm-line-strong bg-nm-raised',
                !n.interactive && 'border-nm-line bg-transparent',
              )}
            >
              {n.interactive ? (
                <button
                  type="button"
                  aria-expanded={isSel}
                  aria-controls={`${uid}-${n.id}`}
                  onClick={() => onSelect(n.id as InteractiveId)}
                  className="flex min-h-12 w-full items-start gap-3 px-4 py-3.5 text-left"
                >
                  <span className="min-w-0 flex-1">
                    {n.region && <RegionTag kind={n.region} className="mb-2" />}
                    <span className="block text-base font-semibold leading-tight tracking-tight">{n.title}</span>
                    <span className="mt-1 block text-sm leading-snug text-nm-muted">{n.sub}</span>
                  </span>
                  <ChevronDown
                    aria-hidden
                    className={cn('mt-1 size-4 shrink-0 text-nm-muted transition-transform duration-200', isSel && 'rotate-180')}
                  />
                </button>
              ) : (
                <div className="px-4 py-3.5">
                  {n.region && <RegionTag kind={n.region} className="mb-2" />}
                  <p className="text-base font-semibold leading-tight tracking-tight">{n.title}</p>
                  <p className="mt-1 text-sm leading-snug text-nm-muted">{n.note}</p>
                </div>
              )}
              {n.interactive && (
                <AnimatePresence initial={false}>
                  {isSel && (
                    <motion.div
                      id={`${uid}-${n.id}`}
                      key="detail"
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      transition={{ duration: dur.panel, ease: EASE }}
                      className="border-t border-nm-line px-4 py-5"
                    >
                      <NodeDetail id={n.id as InteractiveId} />
                    </motion.div>
                  )}
                </AnimatePresence>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

/* ───────────── Explorer ───────────── */

export function ArchitectureExplorer() {
  const [selected, setSelected] = useState<InteractiveId>('policy');
  const reduceMotion = useReducedMotion() ?? false;
  const summaryId = useId();

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
        <p className="font-mono text-[0.72rem] uppercase tracking-[0.12em] text-nm-faint">Interactive architecture preview</p>
        <ul className="flex flex-wrap items-center gap-x-5 gap-y-2" aria-label="Legend">
          <li><RegionTag kind="private" /></li>
          <li><RegionTag kind="open" /></li>
          <li><RegionTag kind="customer" /></li>
          <li className="hidden items-center gap-2 font-mono text-[0.7rem] uppercase tracking-[0.12em] text-nm-faint md:flex">
            <span aria-hidden className="inline-block h-0.5 w-4 bg-nm-accent" />
            Connected path
          </li>
        </ul>
      </div>

      <p id={summaryId} className="sr-only">
        {SUMMARY}
      </p>

      <div className="hidden gap-8 md:grid lg:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)] lg:gap-12" aria-describedby={summaryId}>
        <DesktopGraph selected={selected} onSelect={setSelected} reduce={reduceMotion} />
        <div className="lg:sticky lg:top-24 lg:self-start">
          <div className="rounded-xl border border-nm-line-strong bg-nm-raised p-6 md:p-8">
            <AnimatePresence mode="wait" initial={false}>
              <motion.div
                key={selected}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                transition={{ duration: dur.panel, ease: EASE }}
              >
                <NodeDetail id={selected} />
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>

      <MobileStepper selected={selected} onSelect={setSelected} />

      <p className="mt-8 max-w-3xl border-t border-nm-line pt-4 text-sm text-nm-muted">
        Conceptual explainer — not a claim that every box is a literal synchronous runtime hop.
      </p>
    </div>
  );
}
