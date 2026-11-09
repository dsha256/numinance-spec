'use client';

import { useState } from 'react';
import { cn } from '@/lib/cn';
import { DiagramShell, Segmented } from './kit';

type Keys = 'pinned' | 'bundle' | 'skip';
type Anchor = 'pinned' | 'none';
type Tamper = 'none' | 'edit' | 'remove' | 'wrong';
type St = 'ok' | 'weak' | 'bad' | 'skip';

const ST: Record<St, { g: string; w: string; c: string }> = {
  ok: { g: '✓', w: 'verified', c: 'border-nm-ok/50 text-nm-ok' },
  weak: { g: '◐', w: 'weak', c: 'border-nm-warn/60 text-nm-warn' },
  bad: { g: '✕', w: 'failed', c: 'border-nm-deny/60 text-nm-deny' },
  skip: { g: '–', w: 'not checked', c: 'border-fd-border text-fd-muted-foreground' },
};

const MEANING: Record<string, string> = {
  chain_broken: 'a sequence gap, a bad link, a reordered or edited record',
  signer_untrusted: 'a record names a key that is not trusted',
  part_contradiction: 'a part is declared not applicable and the chain does not bear it out',
};

const CMD_HEAD = 'numi evidence verify';

function Chip({ st, label }: { st: St; label?: string }) {
  const s = ST[st];
  return (
    <span className={cn('inline-flex items-center gap-1 rounded-sm border px-1 font-mono text-[11px] leading-5', s.c)}>
      <span aria-hidden>{s.g}</span>{label ?? s.w}
    </span>
  );
}

function compute(keys: Keys, anchor: Anchor, tamper: Tamper) {
  const chainBad = tamper === 'edit' || tamper === 'remove';
  const signerBad = tamper === 'wrong' && keys === 'pinned';
  const findings: string[] = [];
  if (chainBad) findings.push('chain_broken');
  if (signerBad) findings.push('signer_untrusted', 'part_contradiction', 'part_contradiction');
  const failed = findings.length > 0;
  const weaknesses: string[] = [];
  if (!failed) {
    if (keys === 'skip') weaknesses.push('no signature was checked');
    if (keys === 'bundle') weaknesses.push('signatures were verified only under keys the export claims');
    if (anchor === 'none') weaknesses.push('the head was not proved under a pinned log');
  }
  const level = failed ? 'failed' : keys === 'skip' ? 'unauthenticated' : keys === 'bundle' ? 'self-consistent' : anchor === 'none' ? 'unanchored' : 'verified';
  const exit = failed ? 3 : level === 'verified' ? 0 : 5;
  const records = tamper === 'remove' ? 11 : 12;
  const downstream = failed; // anchor proof, attestation, manifest are "not checked" when the chain did not verify

  // per-record link/signature state
  const sig: St = signerBad ? 'bad' : keys === 'skip' ? 'skip' : keys === 'bundle' ? 'weak' : 'ok';
  const att: St = downstream ? 'skip' : keys === 'skip' || keys === 'bundle' ? 'weak' : 'ok';
  const cp: St = downstream ? 'skip' : anchor === 'pinned' ? 'ok' : 'skip';

  const lines: string[] = [
    'change: chg_01010101010101010101010101010101',
    `records: ${records}${tamper === 'none' || tamper === 'wrong' ? ', head sha256:8dec4e3d…' : ', head sha256:(differs)'}`,
  ];
  if (!failed) {
    lines.push('state: ended Closed', `level: ${level}`, 'chain: records link, hash and end as stated');
    lines.push(keys === 'pinned' ? 'signatures: verified under the pinned runner keys' : keys === 'bundle' ? 'signatures: verified under keys the export states only' : 'signatures: not checked');
    lines.push(anchor === 'pinned' ? 'anchor: head proved under the pinned log' : 'anchor: not checked (no log pinned)');
    lines.push('attestation: ' + (keys === 'pinned' ? 'signed, re-derived from the chain' : 'signature not authenticated, re-derived from the chain'));
    weaknesses.forEach((w) => lines.push(`WEAK ${w}`));
  } else {
    lines.push('level: failed', 'anchor, attestation, manifest: not checked (the chain did not verify)');
    findings.forEach((f) => lines.push(`FAIL ${f}: ${MEANING[f]}`));
  }
  lines.push(`result: ${level}`);
  return { level, exit, findings, weaknesses, records, sig, att, cp, lines, chainBad, signerBad, failed };
}

export function EvidenceChainInspector() {
  const [keys, setKeys] = useState<Keys>('pinned');
  const [anchor, setAnchor] = useState<Anchor>('pinned');
  const [tamper, setTamper] = useState<Tamper>('none');
  const r = compute(keys, anchor, tamper);

  const flags = [
    keys === 'pinned' ? '--runner-key keys/runner.pub' : keys === 'bundle' ? '--trust-bundle-keys' : '--skip-signatures',
    ...(anchor === 'pinned' ? ['--log-key keys/log.pub', '--log-origin numinance.test/log'] : []),
    '--complete',
  ];

  // drawn records: 1..4 and 5 then head (12 or 11)
  const cells: { label: string; st: St; note: string; gap?: boolean }[] = [1, 2, 3, 4, 5].map((n) => {
    let st: St = 'ok';
    let note = 'links to previous';
    if (tamper === 'edit' && n === 3) { st = 'bad'; note = 'content changed after hashing'; }
    if (tamper === 'remove' && n === 3) { st = 'bad'; note = 'MISSING: sequence gap'; }
    if (tamper === 'remove' && n === 4) { st = 'bad'; note = 'previous_record_hash points at a record that is gone'; }
    return { label: tamper === 'remove' && n === 3 ? 'seq 3 (removed)' : `seq ${n}`, st, note, gap: tamper === 'remove' && n === 3 };
  });
  cells.push({ label: `seq ${r.records} (head)`, st: 'ok', note: 'terminal state: Closed' });

  const sigFor = (c: { st: St }) => (c.st === 'bad' && !r.signerBad ? ('skip' as St) : r.sig);

  return (
    <DiagramShell
      title="Evidence chain inspector"
      question="What does verification actually establish, and which inputs does it depend on?"
      controls={
        <>
          <Segmented<Keys> label="Key source" value={keys} onChange={setKeys} options={[{ value: 'pinned', label: 'Caller-pinned keys' }, { value: 'bundle', label: 'Keys from the export' }, { value: 'skip', label: 'Skip signatures' }]} />
          <Segmented<Anchor> label="Anchor" value={anchor} onChange={setAnchor} options={[{ value: 'pinned', label: 'Pinned log' }, { value: 'none', label: 'No log' }]} />
          <Segmented<Tamper> label="Tamper" value={tamper} onChange={setTamper} options={[{ value: 'none', label: 'None' }, { value: 'edit', label: 'Edit record' }, { value: 'remove', label: 'Remove record' }, { value: 'wrong', label: 'Wrong signer' }]} />
        </>
      }
      legend={
        <>
          <span className="text-nm-ok">✓ verified</span>
          <span className="text-nm-warn">◐ weak (consistent, not authenticated)</span>
          <span className="text-nm-deny">✕ failed</span>
          <span>– not checked</span>
        </>
      }
      explanation={
        <>
          <p>Illustrative and simulated: no export is read. The rules are those of <code>docs/evidence/verification.md</code>; the baseline report is the real output of the open verifier on the conformance export <code>exports/closed-author</code>.</p>
          <ul>
            <li>You must choose exactly one key mode. Pinned runner keys (<code>--runner-key</code>) are the only way to reach <code>verified</code>. <code>--trust-bundle-keys</code> takes the keys the export claims and tops out at <code>self-consistent</code>. <code>--skip-signatures</code> tops out at <code>unauthenticated</code>.</li>
            <li>A pinned log (<code>--log-key</code> with <code>--log-origin</code>) is needed to prove the head. Without it the result is <code>unanchored</code> (exit 5) unless <code>--allow-unanchored</code> is given.</li>
            <li>Any finding makes the level <code>failed</code> (exit 3). Edit or remove a record: <code>chain_broken</code>. A chain signed by a key you did not pin: <code>signer_untrusted</code>. With keys from the export or signatures skipped, the same forged-but-consistent chain is not caught, which is why those levels are weaker.</li>
            <li>Exit codes: 0 verified, 3 failed, 5 consistent but weaker than verified.</li>
          </ul>
          <p>Verified does not prove the code is correct, the tests are thorough, the model reasoned well, or that a compromised host logged every event.</p>
        </>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <div className="space-y-3">
          <div>
            <p className="m-0 mb-1 font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Hash-linked records (drawing 6 of {r.records})</p>
            <ol className="m-0 grid list-none grid-cols-2 gap-1.5 p-0 sm:grid-cols-3">
              {cells.map((c, n) => (
                <li key={c.label} className={cn('rounded-sm border bg-fd-background p-2', ST[c.st].c.split(' ')[0], c.gap && 'border-dashed')}>
                  <div className="flex items-center justify-between gap-1">
                    <span className="font-mono text-xs text-fd-foreground">{c.label}</span>
                    <Chip st={c.st} label={c.st === 'ok' ? 'link' : c.gap ? 'gap' : 'broken'} />
                  </div>
                  <p className="m-0 mt-1 text-[11px] leading-4 text-fd-muted-foreground">{n === 0 ? 'previous_record_hash empty (sequence 1)' : c.note}</p>
                  <div className="mt-1.5"><Chip st={c.gap ? 'skip' : sigFor(c)} label={`signature ${ST[c.gap ? 'skip' : sigFor(c)].w}`} /></div>
                </li>
              ))}
            </ol>
          </div>

          <div className="grid gap-1.5 sm:grid-cols-2">
            <div className="rounded-sm border border-fd-border bg-fd-background p-2">
              <p className="m-0 font-mono text-xs text-fd-foreground">attestation.json</p>
              <p className="m-0 my-1 text-[11px] leading-4 text-fd-muted-foreground">signed summary, re-derived from the chain</p>
              <Chip st={r.att} />
            </div>
            <div className="rounded-sm border border-fd-border bg-fd-background p-2">
              <p className="m-0 font-mono text-xs text-fd-foreground">head-checkpoint + inclusion proof</p>
              <p className="m-0 my-1 text-[11px] leading-4 text-fd-muted-foreground">{anchor === 'pinned' ? 'checked under the pinned log key and origin' : 'present in the export, but no log is pinned: never trusted'}</p>
              <Chip st={r.cp} label={r.cp === 'skip' && anchor === 'none' && !r.failed ? 'not anchored' : undefined} />
            </div>
          </div>

          <div className="rounded-sm border border-fd-border p-2">
            <p className="m-0 mb-1 font-mono text-[11px] uppercase tracking-wider text-fd-muted-foreground">Verifier inputs (what you bring, not what the export says)</p>
            <ul className="m-0 grid list-none gap-1 p-0 font-mono text-xs sm:grid-cols-2">
              <li className={cn(keys === 'pinned' ? 'text-nm-ok' : 'text-fd-muted-foreground line-through')}>{keys === 'pinned' ? '✓' : '–'} pinned runner key(s)</li>
              <li className={cn(anchor === 'pinned' ? 'text-nm-ok' : 'text-fd-muted-foreground line-through')}>{anchor === 'pinned' ? '✓' : '–'} pinned log key + origin</li>
              <li className="col-span-full text-fd-muted-foreground">provenance/signers.json and any checkpoint in the export are claims, not trust.</li>
            </ul>
          </div>
        </div>

        <div className="space-y-3">
          <div className="rounded-sm border border-fd-border bg-nm-term p-3 font-mono text-xs leading-5 text-nm-term-fg">
            <p className="m-0 break-words"><span aria-hidden>$ </span>{CMD_HEAD} {flags.join(' ')} exports/closed-author</p>
            <p className="m-0 mt-1 text-[11px] opacity-70">simulated output, shape as in the real CLI</p>
            <pre className="m-0 mt-2 whitespace-pre-wrap break-words font-mono">{r.lines.join('\n')}</pre>
            <p className="m-0 mt-2">exit {r.exit}</p>
          </div>
          <div className={cn('rounded-sm border p-2 text-sm', r.level === 'verified' ? 'border-nm-ok/50' : r.failed ? 'border-nm-deny/60' : 'border-nm-warn/60')}>
            <p className="m-0 flex flex-wrap items-center gap-2">
              <Chip st={r.level === 'verified' ? 'ok' : r.failed ? 'bad' : 'weak'} label={`level: ${r.level}`} />
              <span className="font-mono text-xs text-fd-muted-foreground">exit {r.exit}</span>
            </p>
            <p className="m-0 mt-1 text-fd-foreground">
              {r.level === 'verified' && 'Provenance and chain integrity hold under keys and a log you pinned.'}
              {r.level === 'unanchored' && 'Consistent and signed by your key, but nothing proves this is the one chain the log committed to.'}
              {r.level === 'self-consistent' && 'The directory agrees with itself. Anyone could have built it and signed it with their own key.'}
              {r.level === 'unauthenticated' && 'Links and digests hold. No signature was checked, so no one is established as the author.'}
              {r.level === 'failed' && `${r.findings.length} finding${r.findings.length === 1 ? '' : 's'}: the export is rejected.`}
            </p>
          </div>
          {tamper === 'wrong' && keys !== 'pinned' && !r.failed ? (
            <p className="m-0 rounded-sm border border-nm-warn/60 p-2 text-xs text-fd-foreground">
              <span aria-hidden>◐ </span>A chain built and signed by someone else passes here. Only a key you pinned makes it a finding (signer_untrusted).
            </p>
          ) : null}
        </div>
      </div>

      <div className="mt-4 rounded-sm border border-lp-yellow bg-lp-yellow/15 p-3">
        <p className="m-0 font-mono text-[11px] uppercase tracking-wider text-fd-foreground">What this does NOT prove, at any level</p>
        <ul className="m-0 mt-1 grid list-none gap-x-4 gap-y-1 p-0 text-sm text-fd-foreground sm:grid-cols-2">
          <li><span aria-hidden>✕ </span>The code is correct.</li>
          <li><span aria-hidden>✕ </span>The tests are thorough.</li>
          <li><span aria-hidden>✕ </span>The model reasoned well.</li>
          <li><span aria-hidden>✕ </span>Completeness against a compromised host: events it never logged are not in the chain.</li>
        </ul>
      </div>
      <p className="sr-only" aria-live="polite" role="status">
        Level {r.level}, exit {r.exit}. {r.findings.length ? `Findings: ${r.findings.join(', ')}.` : r.weaknesses.length ? `Weaknesses: ${r.weaknesses.join('; ')}.` : 'No findings or weaknesses.'}
      </p>
    </DiagramShell>
  );
}
