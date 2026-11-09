'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { motion, useInView, useReducedMotion } from 'motion/react';
import { RotateCcw } from 'lucide-react';
import { cn } from '@/lib/cn';
import { EASE, dur } from '@/lib/motion';
import { Tabs, tabPanelProps, type TabItem } from '@/components/ui/tabs';

type TypeName =
  | 'PrincipalKind'
  | 'RiskClass'
  | 'ExecMode'
  | 'ResidencyClass'
  | 'TrustClass'
  | 'AutonomyLevel'
  | 'ChangeState';

type Duty = 'author' | 'reviewer' | 'release' | 'human';

type Scenario = {
  id: string;
  label: string;
  outcome: 'permitted' | 'denied';
  facts: { k: string; v: string }[];
  /** REQUESTED, RISK CLASSIFIED, POLICY EVALUATED, outcome. */
  stages: [string, string, string, string];
  uses: TypeName[];
  flagged?: Duty;
};

const STAGE_LABELS = ['Requested', 'Risk classified', 'Policy evaluated'] as const;

const SCENARIOS: Scenario[] = [
  {
    id: 'permit',
    label: 'Author requests, policy permits',
    outcome: 'permitted',
    facts: [
      { k: 'Action', v: 'OpenPullRequest' },
      { k: 'Principal', v: 'Author agent' },
      { k: 'Resource', v: 'Fictional repository, feature branch' },
      { k: 'Grant', v: 'Present, scoped, unexpired' },
    ],
    stages: [
      'The author agent asks to open a pull request for its own branch.',
      'Moderate: visible to others, but reversible (illustrative).',
      'An unexpired grant covers this action, resource and environment.',
      'A grant scoped to opening the pull request. It does not cover approving or merging it.',
    ],
    uses: ['PrincipalKind', 'RiskClass', 'ExecMode', 'AutonomyLevel'],
  },
  {
    id: 'self-approve',
    label: 'Author attempts to approve own work',
    outcome: 'denied',
    facts: [
      { k: 'Action', v: 'Approve pull request (illustrative)' },
      { k: 'Principal', v: 'Author agent' },
      { k: 'Resource', v: 'The pull request it authored' },
      { k: 'Grant', v: 'Not the deciding factor' },
    ],
    stages: [
      'The agent that authored the change asks to approve it.',
      'Elevated: approval gates what ships (illustrative).',
      'Separation of duties applies. The requester is the author of this change.',
      'Denied. The author cannot approve its own work. An independent reviewer or a human approver must act.',
    ],
    uses: ['PrincipalKind', 'ChangeState', 'AutonomyLevel'],
    flagged: 'author',
  },
  {
    id: 'no-grant',
    label: 'No valid grant',
    outcome: 'denied',
    facts: [
      { k: 'Action', v: 'OpenPullRequest' },
      { k: 'Principal', v: 'Author agent' },
      { k: 'Resource', v: 'Fictional repository, feature branch' },
      { k: 'Grant', v: 'None valid' },
    ],
    stages: [
      'The author agent asks to open a pull request.',
      'Moderate, the same as the permitted case (illustrative).',
      'No grant matches this action here, or the matching grant has expired.',
      'Denied. A proposal carries no authority without a valid grant.',
    ],
    uses: ['PrincipalKind', 'RiskClass', 'ExecMode'],
  },
];

const TABS: TabItem[] = SCENARIOS.map((s) => ({ id: s.id, label: s.label }));

const VOCABULARY: { name: TypeName; meaning: string; example: string }[] = [
  { name: 'PrincipalKind', meaning: 'What kind of actor is asking.', example: 'An autonomous author, a human approver or a release pipeline.' },
  { name: 'RiskClass', meaning: 'How consequential an action is.', example: 'A read-only lookup versus a change other people will see.' },
  { name: 'ExecMode', meaning: 'How an action is allowed to run.', example: 'Run for real inside a sandbox, or only describe what would happen.' },
  { name: 'ResidencyClass', meaning: 'Where data and execution are allowed to live.', example: 'Stays inside the customer’s own cloud account.' },
  { name: 'TrustClass', meaning: 'How much trust a component or result is given.', example: 'A reviewed first-party component versus an unreviewed third-party one.' },
  { name: 'AutonomyLevel', meaning: 'How much the system may do without a person.', example: 'Only propose, or act within a bounded grant.' },
  { name: 'ChangeState', meaning: 'Where a change sits in its lifecycle.', example: 'Drafted, under review or released.' },
];

const DUTIES: { id: Duty; name: string; role: string }[] = [
  { id: 'author', name: 'Author', role: 'Proposes and implements the change.' },
  { id: 'reviewer', name: 'Independent reviewer', role: 'Reviews the change. Not the author.' },
  { id: 'release', name: 'Release pipeline', role: 'Holds deployment authority under the design.' },
  { id: 'human', name: 'Human approver', role: 'Approves where policy requires a person.' },
];

/* ───────────── Separation of duties ───────────── */

export function SeparationOfDuties({ flagged }: { flagged?: Duty }) {
  return (
    <figure>
      <ol className="grid gap-px overflow-hidden rounded-xl border border-nm-line bg-nm-line sm:grid-cols-2 lg:grid-cols-4">
        {DUTIES.map((d, i) => {
          const hit = flagged === d.id;
          return (
            <li key={d.id} className={cn('relative bg-nm-raised p-5 transition-colors duration-200', hit && 'bg-nm-surface')}>
              <p className="font-mono text-[0.72rem] text-nm-faint">{String(i + 1).padStart(2, '0')}</p>
              <p className="mt-2 text-base font-semibold tracking-tight">{d.name}</p>
              <p className="mt-1 text-sm leading-snug text-nm-muted">{d.role}</p>
              {hit && (
                <p className="mt-3 font-mono text-[0.7rem] uppercase tracking-[0.1em] text-nm-deny">Requester in this scenario</p>
              )}
            </li>
          );
        })}
      </ol>
      <figcaption className="mt-4 text-sm leading-relaxed text-nm-muted">
        <span className="font-medium text-nm-fg">Separation of duties:</span> author · independent reviewer · release
        pipeline · human approver are distinct identities. The author cannot approve its own work. Illustrative.
      </figcaption>
    </figure>
  );
}

/* ───────────── Vocabulary matrix ───────────── */

function VocabularyMatrix({ uses }: { uses: TypeName[] }) {
  const used = new Set(uses);
  const mark = (n: TypeName) =>
    used.has(n) ? (
      <span className="inline-flex items-center gap-2 font-mono text-[0.72rem] text-nm-accent">
        <span aria-hidden className="size-1.5 rounded-full bg-nm-accent" />
        consulted
      </span>
    ) : (
      <span className="font-mono text-[0.72rem] text-nm-faint">not used</span>
    );
  return (
    <div>
      <div className="hidden overflow-hidden rounded-xl border border-nm-line md:block">
        <table className="w-full border-collapse text-left text-sm">
          <caption className="sr-only">
            Governance vocabulary with plain-English meanings and illustrative values. Illustrative values are descriptive text, not enum members.
          </caption>
          <thead>
            <tr className="border-b border-nm-line bg-nm-raised">
              <th scope="col" className="nm-eyebrow px-5 py-3 font-normal">Type</th>
              <th scope="col" className="nm-eyebrow px-5 py-3 font-normal">Plain-English meaning</th>
              <th scope="col" className="nm-eyebrow px-5 py-3 font-normal">Illustrative value</th>
              <th scope="col" className="nm-eyebrow px-5 py-3 font-normal">In this decision</th>
            </tr>
          </thead>
          <tbody>
            {VOCABULARY.map((v) => (
              <tr key={v.name} className="border-b border-nm-line last:border-b-0">
                <th scope="row" className="px-5 py-3.5 align-top font-mono text-[0.8rem] font-medium text-nm-fg">{v.name}</th>
                <td className="px-5 py-3.5 align-top text-nm-muted">{v.meaning}</td>
                <td className="px-5 py-3.5 align-top text-nm-muted">{v.example}</td>
                <td className="px-5 py-3.5 align-top">{mark(v.name)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ul className="grid gap-px overflow-hidden rounded-xl border border-nm-line bg-nm-line md:hidden">
        {VOCABULARY.map((v) => (
          <li key={v.name} className="bg-nm-raised p-4">
            <div className="flex items-center justify-between gap-3">
              <p className="font-mono text-[0.82rem] font-medium">{v.name}</p>
              {mark(v.name)}
            </div>
            <p className="mt-2 text-sm text-nm-muted">{v.meaning}</p>
            <p className="mt-1 text-sm text-nm-muted">
              <span className="text-nm-faint">Illustrative: </span>
              {v.example}
            </p>
          </li>
        ))}
      </ul>

      <p className="mt-4 max-w-3xl text-sm leading-relaxed text-nm-muted">
        The illustrative column is descriptive text, not a list of enum members. The real members of these types are defined in{' '}
        <code className="font-mono text-[0.8rem] text-nm-fg">numinance.annotations.v1</code> /{' '}
        <code className="font-mono text-[0.8rem] text-nm-fg">numinance.types.v1</code>.
      </p>
    </div>
  );
}

/* ───────────── Decision sequence ───────────── */

function Sequence({ scenario, step }: { scenario: Scenario; step: number }) {
  const permitted = scenario.outcome === 'permitted';
  const done = step >= 4;
  return (
    <ol className="grid gap-px overflow-hidden rounded-xl border border-nm-line bg-nm-line md:grid-cols-4">
      {[0, 1, 2, 3].map((i) => {
        const shown = i < step;
        const current = i === step - 1 && !done;
        return (
          <li key={i} className="relative min-h-[8.5rem] bg-nm-raised p-5">
            <div className="flex items-center gap-3">
              <span className="font-mono text-[0.72rem] text-nm-faint">{String(i + 1).padStart(2, '0')}</span>
              {i < 3 ? (
                <p className={cn('font-mono text-[0.78rem] uppercase tracking-[0.08em]', shown ? 'text-nm-fg' : 'text-nm-faint')}>
                  {STAGE_LABELS[i]}
                </p>
              ) : (
                <p className="flex flex-wrap items-center gap-x-2 font-mono text-[0.78rem] uppercase tracking-[0.08em]">
                  <span
                    className={cn(
                      'transition-opacity duration-200',
                      done && permitted ? 'text-nm-ok' : done ? 'text-nm-faint line-through opacity-50' : 'text-nm-faint',
                    )}
                  >
                    Permitted
                  </span>
                  <span className="text-nm-faint" aria-hidden>|</span>
                  <span
                    className={cn(
                      'transition-opacity duration-200',
                      done && !permitted ? 'text-nm-deny' : done ? 'text-nm-faint line-through opacity-50' : 'text-nm-faint',
                    )}
                  >
                    Denied
                  </span>
                </p>
              )}
            </div>
            <motion.p
              key={`${scenario.id}-${i}-${shown}`}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: shown ? 1 : 0.5, y: 0 }}
              transition={{ duration: dur.panel, ease: EASE }}
              className={cn('mt-3 text-sm leading-relaxed', shown ? 'text-nm-muted' : 'text-nm-faint')}
            >
              {shown ? scenario.stages[i] : 'Pending'}
            </motion.p>
            {current && <span aria-hidden className="absolute inset-x-0 top-0 h-0.5 bg-nm-accent" />}
            {i === 3 && done && (
              <span
                aria-hidden
                className={cn('absolute inset-x-0 top-0 h-0.5', permitted ? 'bg-nm-ok' : 'bg-nm-deny')}
              />
            )}
          </li>
        );
      })}
    </ol>
  );
}

/* ───────────── Inspector ───────────── */

export function GovernanceInspector() {
  const [scenarioId, setScenarioId] = useState(SCENARIOS[0].id);
  const [step, setStep] = useState(0);
  const [run, setRun] = useState(0);
  const reduce = useReducedMotion();
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, amount: 0.35 });
  const idBase = useId();
  const scenario = SCENARIOS.find((s) => s.id === scenarioId) ?? SCENARIOS[0];

  useEffect(() => {
    if (reduce) {
      setStep(4);
      return;
    }
    if (!inView) return;
    let n = 1;
    setStep(1);
    const t = setInterval(() => {
      n += 1;
      setStep(n);
      if (n >= 4) clearInterval(t);
    }, 700);
    return () => clearInterval(t);
  }, [scenarioId, run, inView, reduce]);

  return (
    <div ref={ref} className="space-y-16">
      <div>
        <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
          <Tabs items={TABS} value={scenarioId} onChange={setScenarioId} label="Illustrative scenario" idBase={idBase} size="sm" />
          <div className="flex items-center gap-4">
            <span className="font-mono text-[0.72rem] uppercase tracking-[0.12em] text-nm-faint">
              Illustrative workflow · Pre-alpha
            </span>
            <button
              type="button"
              onClick={() => setRun((r) => r + 1)}
              className="inline-flex min-h-11 items-center gap-2 rounded-full border border-nm-line-strong px-4 text-sm font-medium transition-colors hover:bg-nm-surface"
            >
              <RotateCcw className="size-3.5" aria-hidden />
              Replay
            </button>
          </div>
        </div>

        <div {...tabPanelProps(idBase, scenario.id)} className="rounded-xl outline-offset-4">
          <div className="mb-px grid gap-px overflow-hidden rounded-t-xl border border-b-0 border-nm-line bg-nm-line sm:grid-cols-2 lg:grid-cols-4">
            {scenario.facts.map((f) => (
              <div key={f.k} className="bg-nm-raised px-5 py-4">
                <p className="nm-eyebrow mb-1.5">{f.k}</p>
                <p className="font-mono text-[0.82rem] text-nm-fg">{f.v}</p>
              </div>
            ))}
          </div>
          <Sequence scenario={scenario} step={step} />
        </div>
        <p className="sr-only" role="status">
          {step >= 4 ? `${scenario.label}: ${scenario.outcome}. ${scenario.stages[3]}` : ''}
        </p>
        <p className="mt-4 text-sm text-nm-muted">
          Fictional action and principals. The decision is made outside the model. This is a state sequence, not a recording of a real run.
        </p>
      </div>

      <div>
        <h3 className="mb-2 text-xl font-semibold tracking-tight">Governance vocabulary</h3>
        <p className="mb-6 max-w-2xl text-sm text-nm-muted">
          Seven typed concepts describe who is asking, how risky it is, and how it may run. Rows marked consulted are used
          in the scenario above.
        </p>
        <VocabularyMatrix uses={scenario.uses} />
      </div>

      <div>
        <h3 className="mb-6 text-xl font-semibold tracking-tight">Separation of duties</h3>
        <SeparationOfDuties flagged={scenario.flagged} />
      </div>
    </div>
  );
}
