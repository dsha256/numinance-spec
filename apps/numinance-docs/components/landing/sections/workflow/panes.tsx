'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Check, Clipboard } from 'lucide-react';
import { cn } from '@/lib/cn';
import { setStatus } from '@/lib/landing/status';
import type { PaneState } from './model';

/* ---------- hand-rolled highlighter (YAML + proto-ish shapes), no dependencies ---------- */

const TOKEN =
  /(#.*$|\/\/.*$)|("[^"]*")|(\b(?:inv|chg|run|grant|dec|ev)_[A-Za-z0-9…]+)|(\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b)|(\b[A-Z][a-z]+(?:[A-Z][a-z]*)*\b)|([A-Za-z_][\w-]*)(?=\s*:)|([{}[\],:])|(\s+)|([^\s{}[\],:#"]+)/g;

function tokens(line: string): ReactNode[] {
  const out: ReactNode[] = [];
  let k = 0;
  for (const m of line.matchAll(TOKEN)) {
    const [text, com, str, id, enm, type, key, punc] = m;
    const cls = com
      ? 'text-lp-ink-2 italic'
      : str
        ? 'text-lp-orange'
        : id
          ? 'text-lp-orange'
          : enm
            ? 'text-lp-indigo'
            : type
              ? 'font-medium text-lp-ink'
              : key
                ? 'text-lp-ink-2'
                : punc
                  ? 'text-lp-ink-2/70'
                  : 'text-lp-ink';
    out.push(
      <span key={k++} className={cls}>
        {text}
      </span>,
    );
  }
  return out;
}

export function Code({ lines, hot, on }: { lines: readonly string[]; hot?: readonly number[]; on: boolean }) {
  return (
    <pre className="m-0 overflow-x-auto font-mono text-[11.5px] leading-3.5 [scrollbar-width:none]">
      {lines.map((l, i) => {
        const lit = on && hot?.includes(i);
        return (
          <div
            key={i}
            data-lit={lit ? '' : undefined}
            className={cn(
              'relative whitespace-pre border-l-2 pl-2 transition-colors duration-300',
              lit ? 'border-lp-orange bg-lp-yellow/20' : 'border-transparent',
            )}
          >
            {l ? tokens(l) : ' '}
          </div>
        );
      })}
    </pre>
  );
}

/* ---------- chrome ---------- */

const NAMES = ['INTENT', 'AUTHORITY', 'EXECUTION', 'EVIDENCE'] as const;

export function Region({
  i,
  state,
  right,
  children,
  className,
}: {
  i: number;
  state: PaneState;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      data-region={i}
      data-state={state}
      aria-label={`${i + 1} ${NAMES[i]}`}
      className={cn('relative flex min-h-0 min-w-0 flex-col overflow-hidden bg-lp-paper', className)}
    >
      <header className="flex h-8 shrink-0 items-center gap-2 border-b border-lp-line px-3 font-pixel text-[13px] leading-4">
        <span
          className={cn(
            'inline-flex h-5 items-center gap-1.5 border border-lp-ink px-1.5 transition-colors duration-300',
            state === 'active' ? 'bg-lp-yellow text-lp-on-yellow' : 'bg-lp-paper text-lp-ink',
          )}
        >
          {String(i + 1).padStart(2, '0')} / {NAMES[i]}
          {state === 'done' && <Check aria-hidden className="size-3 text-lp-orange" strokeWidth={3} />}
        </span>
        <span className="ml-auto min-w-0 truncate text-lp-ink-2">{right}</span>
      </header>
      <div
        className={cn(
          'min-h-0 flex-1 px-3 pt-2.5 pb-3 transition-opacity duration-300 ease-out',
          state === 'idle' ? 'opacity-55' : 'opacity-100',
        )}
      >
        {children}
      </div>
      <span
        aria-hidden
        className={cn(
          'pointer-events-none absolute inset-1 border border-lp-ink transition-opacity duration-300',
          state === 'active' ? 'opacity-100' : 'opacity-0',
        )}
      />
    </section>
  );
}

const Tag = ({ children, on, className }: { children: ReactNode; on?: boolean; className?: string }) => (
  <span
    className={cn(
      'inline-flex h-5 items-center border px-1.5 font-pixel text-[13px] leading-4 whitespace-nowrap transition-colors duration-300',
      on ? 'border-lp-ink bg-lp-yellow text-lp-on-yellow' : 'border-lp-line-strong text-lp-ink-2',
      className,
    )}
  >
    {children}
  </span>
);

/* ---------- 01 INTENT ---------- */

export const POLICY_PATH = 'schemas/examples/AutonomyPolicy/org-ladder.yaml';
const POLICY = [
  'apiVersion: numinance.dev/v1',
  'kind: AutonomyPolicy',
  'metadata: {name: org-ladder, scope: SCOPE_ORG}',
  'spec:',
  '  defaults: {level: AUTONOMY_LEVEL_PR}',
  '  rules:',
  '    - {match: {playbook: dependency-upgrade,',
  '               risk: RISK_CLASS_LOW},',
  '       level: AUTONOMY_LEVEL_MERGE}',
  '  floors:',
  '    rules:',
  '      - {match: {risk: RISK_CLASS_HIGH},',
  '         max_level: AUTONOMY_LEVEL_SUGGEST}',
  '      - {match: {environment: production},',
  '         max_level: AUTONOMY_LEVEL_MERGE}',
];

export function Define({ state }: { state: PaneState }) {
  return (
    <div className="flex h-full flex-col gap-1.5">
      <div className="flex items-center gap-2 font-mono text-[11.5px] leading-4 text-lp-ink-2 lg:hidden">
        <span aria-hidden className="size-1.5 shrink-0 bg-lp-indigo" />
        <span className="min-w-0 truncate">{POLICY_PATH}</span>
      </div>
      <Code lines={POLICY} hot={[5, 6, 7, 8]} on={state !== 'idle'} />
    </div>
  );
}

/* ---------- 02 AUTHORITY ---------- */

const RISKS = ['READ', 'LOW', 'MEDIUM', 'HIGH', 'PRIVILEGED'] as const;
const OUTCOMES = ['ALLOW', 'DENY', 'REQUIRE_APPROVAL'] as const;
const LADDER = ['OBSERVE', 'SUGGEST', 'IMPLEMENT', 'PR', 'MERGE', 'STAGE', 'RELEASE', 'OPERATE'] as const;

export function Govern({ state }: { state: PaneState }) {
  const on = state !== 'idle';
  return (
    <div className="grid h-full content-start gap-x-6 gap-y-2.5 text-[12px] leading-4 lg:grid-cols-2">
      <div className="flex flex-col gap-2.5">
        <div className="flex flex-col gap-1 font-mono text-[11.5px]">
          <span className="font-pixel text-[13px] text-lp-ink-2">requested action</span>
          <span className="text-lp-ink">playbook: dependency-upgrade</span>
          <span className="text-lp-ink-2">via numinance.ports.scm</span>
        </div>
        <div className="flex flex-col gap-1">
          <span className="font-pixel text-[13px] text-lp-ink-2">RiskClass</span>
          <div className="flex flex-wrap gap-1">
            {RISKS.map((r) => (
              <Tag key={r} on={on && r === 'LOW'}>
                {r}
              </Tag>
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-1">
          <span className="font-pixel text-[13px] text-lp-ink-2">DecisionOutcome</span>
          <div className="flex flex-wrap gap-1">
            {OUTCOMES.map((o) => (
              <Tag key={o} on={on && o === 'ALLOW'} className={on && o === 'ALLOW' ? 'border-lp-orange' : undefined}>
                {o}
              </Tag>
            ))}
          </div>
          <span className="font-pixel text-[13px] text-lp-ink-2">{on ? 'illustrative evaluation' : 'pending'}</span>
        </div>
      </div>

      <div className="flex flex-col gap-2.5">
        <div className="flex flex-col gap-1">
          <span className="font-pixel text-[13px] text-lp-ink-2">AutonomyLevel · default PR, rule → MERGE</span>
          <div className="flex items-end gap-0.75" aria-hidden>
            {LADDER.map((l, i) => {
              const rule = l === 'MERGE';
              return (
                <div key={l} className="flex flex-1 flex-col items-center gap-0.5">
                  <span className="h-3 font-pixel text-[13px] leading-3 text-lp-orange">{on && rule ? '▼' : ''}</span>
                  <span
                    className={cn(
                      'w-full border border-lp-line-strong transition-colors duration-300',
                      on && rule ? 'bg-lp-ink' : i <= 4 && on ? 'bg-lp-ink-2/45' : 'bg-lp-paper-2',
                    )}
                    style={{ height: 6 + i * 3 }}
                  />
                </div>
              );
            })}
          </div>
          <div className="flex justify-between font-mono text-[11px] text-lp-ink-2">
            <span>OBSERVE</span>
            <span>OPERATE</span>
          </div>
        </div>
        <div className="flex flex-col gap-1">
          <span className="font-pixel text-[13px] text-lp-ink-2">PrincipalKind · separation</span>
          <div className="flex flex-wrap items-center gap-1">
            <Tag>AUTHOR_AGENT</Tag>
            <span className="font-pixel text-[13px] text-lp-orange">≠</span>
            <Tag>REVIEWER_AGENT</Tag>
            <span className="font-pixel text-[13px] text-lp-ink-2">/</span>
            <Tag>HUMAN_APPROVER</Tag>
          </div>
        </div>
        <p className="font-pixel text-[13px] leading-4 text-lp-orange">the decision is made outside the model</p>
      </div>
    </div>
  );
}

/* ---------- 03 EXECUTION ---------- */

const ENVELOPE = [
  'Envelope {',
  '  method: numinance.ports.scm.v1 …',
  '  context: ActionContext {',
  '    invocation_id:      inv_…',
  '    change_id:          chg_…',
  '    actor_kind:         PRINCIPAL_KIND_AUTHOR_AGENT',
  '    grant_id:           grant_…',
  '    policy_decision_id: dec_…',
  '    plan_digest:        …',
  '  }',
  '  request: Any',
  '}',
];
const SET_BY_HOST = [2, 3, 4, 5, 6, 7, 8];

export function Execute({ state, stage }: { state: PaneState; stage: number }) {
  const on = state !== 'idle';
  const emit = stage >= 3 && stage < 5;
  return (
    <div className="grid h-full content-start gap-x-6 gap-y-2 xl:grid-cols-[auto_minmax(0,1fr)]">
      <Code lines={ENVELOPE} hot={SET_BY_HOST} on={on} />
      <div className="flex flex-col gap-2 font-mono text-[11.5px] leading-4">
        <span
          className={cn(
            'inline-flex w-fit flex-wrap items-center gap-x-1.5 border px-1.5 py-0.5 transition-colors duration-300',
            on ? 'border-lp-ink bg-lp-yellow/25' : 'border-lp-line-strong',
          )}
        >
          <span className="font-medium whitespace-nowrap">PluginService.Invoke</span>
          <span className="whitespace-nowrap text-lp-ink-2">(server-streaming)</span>
        </span>
        <svg aria-hidden viewBox="0 0 10 14" className={cn('ml-4 h-3.5 transition-colors duration-300', emit ? 'text-lp-indigo' : 'text-lp-line-strong')}>
          <path d="M5 0v12M1 8l4 5 4-5" fill="none" stroke="currentColor" strokeWidth="1.5" />
        </svg>
        <span
          className={cn(
            'inline-flex w-fit items-center border px-1.5 py-0.5 transition-colors duration-300',
            emit ? 'border-lp-indigo text-lp-indigo' : 'border-lp-line-strong text-lp-ink-2',
          )}
        >
          HostService.EmitEvidence
        </span>
        <p className="text-lp-ink-2">
          <span className="text-lp-orange">context</span> is set by the host after the policy decision — the plugin never
          supplies it.
        </p>
      </div>
    </div>
  );
}

/* ---------- 04 EVIDENCE ---------- */

export const VERIFY_CMD = 'numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR';

function CopyCmd() {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-stretch border border-lp-line-strong bg-lp-paper">
      <code className="flex min-w-0 flex-1 items-baseline gap-1.5 self-center px-2 py-1 font-mono text-[11.5px] leading-4">
        <span aria-hidden className="text-lp-ink-2 select-none">
          $
        </span>
        <span className="min-w-0 break-words">{VERIFY_CMD}</span>
      </code>
      <button
        type="button"
        aria-label={copied ? 'Copied' : `Copy "${VERIFY_CMD}"`}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(VERIFY_CMD);
            setCopied(true);
            setStatus('copied: numi evidence verify');
            setTimeout(() => setCopied(false), 1600);
          } catch {}
        }}
        className="flex w-9 shrink-0 items-center justify-center border-s border-lp-line-strong transition-none hover:bg-lp-ink hover:text-lp-paper focus-visible:bg-lp-ink focus-visible:text-lp-paper focus-visible:outline-none"
      >
        {copied ? <Check className="size-3.5" /> : <Clipboard className="size-3.5" />}
      </button>
    </div>
  );
}

const LEVELS = ['unauthenticated', 'self-consistent', 'verified'] as const;

function Rec({ id, fresh, shown }: { id: string; fresh?: boolean; shown: boolean }) {
  const rows: [string, string][] = [
    ['ev_id', `ev_…${id}`],
    ['change_id', 'chg_…'],
    ['action', 'ports.scm'],
    ['policy_decision_id', 'dec_…'],
    ['grant_id', 'grant_…'],
    ['signature', 'sig…'],
  ];
  return (
    <div
      data-fresh={fresh ? '' : undefined}
      className={cn(
        'min-w-0 border bg-lp-paper px-1.5 py-1 font-mono text-[11px] leading-3.5 transition-[opacity,translate] duration-500 ease-out',
        fresh ? 'flex-[2.2] border-lp-ink max-lg:flex-1' : 'flex-1 border-lp-line-strong',
        shown ? 'translate-x-0 opacity-100' : '-translate-x-3 opacity-0',
      )}
    >
      {(fresh ? rows : rows.slice(0, 3)).map(([k, v]) => (
        <div key={k} className="flex justify-between gap-2">
          <span className="truncate text-lp-ink-2">{k}</span>
          <span className={cn('truncate', k === 'ev_id' ? 'text-lp-orange' : k === 'signature' ? 'text-lp-indigo' : 'text-lp-ink')}>{v}</span>
        </div>
      ))}
    </div>
  );
}

const Link2 = ({ lit }: { lit: boolean }) => (
  <svg aria-hidden viewBox="0 0 18 10" className={cn('w-3.5 shrink-0 self-center transition-colors duration-300', lit ? 'text-lp-indigo' : 'text-lp-line-strong')}>
    <path d="M0 5h18M12 1l5 4-5 4" fill="none" stroke="currentColor" strokeWidth="1.5" />
  </svg>
);

export function Verify({ stage }: { stage: number }) {
  const rec = stage >= 3 && stage < 5;
  const verified = stage === 4;
  return (
    <div className="flex h-full flex-col gap-1.5">
      <div className="flex items-stretch gap-1" aria-hidden>
        <div className="contents max-lg:hidden">
          <Rec id="a41" shown />
          <Link2 lit />
          <Rec id="b07" shown />
          <Link2 lit={rec} />
        </div>
        {rec ? <Rec id="c9e" fresh shown /> : <div className="min-h-[94px] flex-[2.2] border border-dashed max-lg:flex-1 border-lp-line-strong px-1.5 py-1 font-pixel text-[13px] text-lp-ink-2">next record</div>}
      </div>
      <p className="font-pixel text-[13px] leading-4 text-lp-ink-2 lg:hidden">hash-linked to 2 earlier records</p>
      <CopyCmd />
      <div className="flex flex-wrap items-center gap-1" aria-label="verification levels">
        {LEVELS.map((l) => (
          <Tag key={l} on={verified && l === 'verified'} className={verified && l === 'verified' ? 'border-lp-orange' : undefined}>
            {l}
          </Tag>
        ))}
        <span className={cn('font-pixel text-[13px] leading-4 transition-colors', verified ? 'text-lp-orange' : 'text-lp-ink-2')}>
          {verified ? 'SIMULATED' : rec ? 'not verified yet' : 'not run'}
        </span>
      </div>
      <p className="mt-auto text-[12px] leading-4 text-lp-ink-2">
        <span className="text-lp-ink">verified</span> needs caller-pinned keys; a key from the export never reaches it. Without them
        the best level is <span className="text-lp-ink">self-consistent</span> or <span className="text-lp-ink">unauthenticated</span>.{' '}
        <Link href="/docs/evidence" className="underline underline-offset-2 hover:text-lp-ink">
          Evidence docs
        </Link>
      </p>
    </div>
  );
}
