'use client';

import { Fragment, useId, useState } from 'react';
import { cn } from '@/lib/cn';
import { CAPABILITIES, type Capability } from '@/lib/docs/capabilities';
import { STATUS, STATUS_ORDER, type StatusKind } from '@/lib/docs/status';
import { specUrl } from '@/lib/docs/spec';
import { DiagramShell, Segmented } from './kit';
import { StatusBadge } from '../status';

type Group = Capability['group'];
const GROUPS: Group[] = ['Open CLI', 'Protocol', 'SDKs', 'Evidence', 'Integrations', 'Runtime', 'Distribution'];
const KINDS = STATUS_ORDER.filter((k) => k !== 'illustrative');
const LANGS: { id: string; label: string }[] = [
  { id: 'sdk-go', label: 'Go' },
  { id: 'sdk-python', label: 'Python' },
  { id: 'sdk-ts', label: 'TypeScript' },
  { id: 'sdk-rust', label: 'Rust' },
  { id: 'sdk-java', label: 'Java' },
];
const byId = (id: string) => CAPABILITIES.find((c) => c.id === id);
const count = (k: StatusKind) => CAPABILITIES.filter((c) => c.state === k).length;

export function CapabilityMatrix() {
  const uid = useId();
  const [kinds, setKinds] = useState<StatusKind[]>(KINDS);
  const [group, setGroup] = useState<'all' | Group>('all');
  const [open, setOpen] = useState<string[]>([]);

  const rows = CAPABILITIES.filter((c) => kinds.includes(c.state) && (group === 'all' || c.group === group));
  const groups = GROUPS.filter((g) => rows.some((r) => r.group === g));
  const toggleKind = (k: StatusKind) => setKinds((cur) => (cur.includes(k) ? cur.filter((x) => x !== k) : [...cur, k]));
  const toggleOpen = (id: string) => setOpen((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]));

  const controls = (
    <div className="flex w-full flex-col gap-2">
      <div role="group" aria-label="Filter by status" className="flex flex-wrap items-center gap-1">
        <span className="mr-1 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Status</span>
        {KINDS.map((k) => {
          const on = kinds.includes(k);
          return (
            <button
              key={k}
              type="button"
              aria-pressed={on}
              onClick={() => toggleKind(k)}
              className={cn(
                'min-h-9 rounded-sm border px-2.5 py-1.5 font-mono text-xs transition-colors',
                on ? 'border-lp-ink bg-lp-ink text-lp-paper' : 'border-fd-border bg-fd-background text-fd-foreground hover:border-fd-foreground/50',
              )}
            >
              <span aria-hidden>{on ? '■' : '□'} {STATUS[k].glyph} </span>
              {STATUS[k].label} ({count(k)})
            </button>
          );
        })}
      </div>
      <Segmented
        label="Group"
        value={group}
        onChange={setGroup}
        options={[{ value: 'all' as const, label: 'All' }, ...GROUPS.map((g) => ({ value: g, label: g }))]}
      />
    </div>
  );

  const explanation = (
    <>
      <p>
        Capabilities of the public numinance-spec repository and the proprietary runtime, by group, with a status from the shared status
        vocabulary. Rust and Java SDKs are coming soon: no source distribution, package name, version or date exists. Python and
        TypeScript are generated stubs at pre-alpha, not verified published packages. Go has handwritten packages plus generated stubs.
      </p>
      {GROUPS.map((g) => (
        <div key={g}>
          <p><strong>{g}</strong></p>
          <ul>
            {CAPABILITIES.filter((c) => c.group === g).map((c) => (
              <li key={c.id}>
                {c.name}: {STATUS[c.state].label}. {c.surface}. Try: {c.howToTry}. Next: {c.next}.
              </li>
            ))}
          </ul>
        </div>
      ))}
    </>
  );

  const legend = (
    <>
      {KINDS.map((k) => (
        <span key={k}>{STATUS[k].glyph} {STATUS[k].label}</span>
      ))}
    </>
  );

  return (
    <DiagramShell
      title="Capability matrix"
      question="What exists today, per language and surface?"
      controls={controls}
      legend={legend}
      explanation={explanation}
    >
      <div className="mb-4">
        <p className="m-0 mb-1.5 font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">SDK language coverage</p>
        <ul className="m-0 grid list-none grid-cols-2 gap-1.5 p-0 min-[520px]:grid-cols-3 md:grid-cols-5">
          {LANGS.map((l) => {
            const c = byId(l.id);
            if (!c) return null;
            return (
              <li
                key={l.id}
                className={cn(
                  'm-0 flex min-h-16 flex-col items-start justify-between gap-1 rounded-sm border bg-fd-background px-2 py-1.5',
                  c.state === 'coming-soon' ? 'border-dashed border-lp-orange/60' : 'border-fd-border',
                )}
              >
                <span className="text-sm font-medium">{l.label}</span>
                <StatusBadge kind={c.state} />
              </li>
            );
          })}
        </ul>
      </div>

      {rows.length === 0 ? (
        <div className="rounded-md border border-dashed border-fd-border p-4 text-sm text-fd-muted-foreground">
          No capabilities match the selected statuses.{' '}
          <button type="button" onClick={() => setKinds(KINDS)} className="min-h-9 underline">Show all statuses</button>
        </div>
      ) : (
        <>
          <div className="relative hidden overflow-x-auto sm:block" tabIndex={0} role="region" aria-label="Capability table">
            <table className="w-full min-w-[40rem] border-collapse text-left text-sm">
              <caption className="sr-only">Capabilities grouped by surface, with status. Use the Details buttons to expand a row.</caption>
              <thead>
                <tr className="border-b border-fd-border font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">
                  <th scope="col" className="py-2 pr-3 font-normal">Capability</th>
                  <th scope="col" className="py-2 pr-3 font-normal">Status</th>
                  <th scope="col" className="py-2 pr-3 font-normal">Surface</th>
                  <th scope="col" className="py-2 font-normal"><span className="sr-only">Details</span></th>
                </tr>
              </thead>
              {groups.map((g) => (
                <tbody key={g}>
                  <tr>
                    <th scope="colgroup" colSpan={4} className="bg-fd-muted px-2 py-1.5 font-mono text-xs font-normal uppercase tracking-wider text-fd-foreground">
                      {g}
                    </th>
                  </tr>
                  {rows.filter((r) => r.group === g).map((r) => {
                    const isOpen = open.includes(r.id);
                    const did = `${uid}-t-${r.id}`;
                    return (
                      <Fragment key={r.id}>
                        <tr className="border-b border-fd-border align-top">
                          <th scope="row" className="py-2 pr-3 font-medium">{r.name}</th>
                          <td className="py-2 pr-3"><StatusBadge kind={r.state} /></td>
                          <td className="py-2 pr-3 text-fd-muted-foreground">{r.surface}</td>
                          <td className="py-1.5 text-right">
                            <button
                              type="button"
                              aria-expanded={isOpen}
                              aria-controls={did}
                              onClick={() => toggleOpen(r.id)}
                              className="min-h-9 rounded-sm border border-fd-border px-2.5 font-mono text-xs hover:border-fd-foreground/50"
                            >
                              {isOpen ? '− Hide' : '+ Details'}
                              <span className="sr-only"> for {r.name}</span>
                            </button>
                          </td>
                        </tr>
                        {isOpen ? (
                          <tr id={did} className="border-b border-fd-border bg-fd-background">
                            <td colSpan={4} className="px-2 py-3"><Detail c={r} /></td>
                          </tr>
                        ) : null}
                      </Fragment>
                    );
                  })}
                </tbody>
              ))}
            </table>
          </div>

          <div className="space-y-4 sm:hidden">
            {groups.map((g) => (
              <section key={g} aria-label={g}>
                <h4 className="m-0 mb-1.5 font-mono text-xs uppercase tracking-wider text-fd-foreground">{g}</h4>
                <ul className="m-0 list-none space-y-1.5 p-0">
                  {rows.filter((r) => r.group === g).map((r) => {
                    const isOpen = open.includes(r.id);
                    const did = `${uid}-c-${r.id}`;
                    return (
                      <li key={r.id} className="m-0 rounded-sm border border-fd-border bg-fd-background p-2.5">
                        <p className="m-0 text-sm font-medium">{r.name}</p>
                        <div className="mt-1"><StatusBadge kind={r.state} /></div>
                        <p className="m-0 mt-1.5 text-sm text-fd-muted-foreground">{r.surface}</p>
                        <button
                          type="button"
                          aria-expanded={isOpen}
                          aria-controls={did}
                          onClick={() => toggleOpen(r.id)}
                          className="mt-2 min-h-9 rounded-sm border border-fd-border px-2.5 font-mono text-xs"
                        >
                          {isOpen ? '− Hide details' : '+ Details'}
                        </button>
                        {isOpen ? <div id={did} className="mt-2 border-t border-fd-border pt-2"><Detail c={r} /></div> : null}
                      </li>
                    );
                  })}
                </ul>
              </section>
            ))}
          </div>
        </>
      )}
      <p className="sr-only" aria-live="polite" role="status">
        {`Showing ${rows.length} of ${CAPABILITIES.length} capabilities${group === 'all' ? '' : ` in ${group}`}.`}
      </p>
    </DiagramShell>
  );
}

function Detail({ c }: { c: Capability }) {
  return (
    <dl className="m-0 grid gap-x-4 gap-y-2 text-sm md:grid-cols-3">
      <div className="min-w-0">
        <dt className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">How to try</dt>
        <dd className="m-0 mt-0.5 break-words font-mono text-xs leading-relaxed">{c.howToTry}</dd>
      </div>
      <div className="min-w-0">
        <dt className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Source</dt>
        <dd className="m-0 mt-0.5 break-words">
          {c.source ? (
            <a href={specUrl(c.source)} target="_blank" rel="noopener noreferrer" className="font-mono text-xs text-nm-accent underline underline-offset-2">
              {c.source}
              <span className="sr-only"> (opens in a new tab)</span>
            </a>
          ) : (
            <span className="text-fd-muted-foreground">No public source for this item.</span>
          )}
        </dd>
      </div>
      <div className="min-w-0">
        <dt className="font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">Next milestone</dt>
        <dd className="m-0 mt-0.5">{c.next}</dd>
      </div>
    </dl>
  );
}
