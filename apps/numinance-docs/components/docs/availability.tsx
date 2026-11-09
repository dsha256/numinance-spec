import { CAPABILITIES, type Capability } from '@/lib/docs/capabilities';
import { StatusBadge } from './status';
import { specUrl } from '@/lib/docs/spec';

/** Readiness matrix generated from lib/docs/capabilities.ts. */
export function AvailabilityMatrix({ group, ids }: { group?: Capability['group']; ids?: string[] }) {
  const rows = CAPABILITIES.filter((c) => (group ? c.group === group : true) && (ids ? ids.includes(c.id) : true));
  return (
    <div className="not-prose relative my-6 overflow-x-auto rounded-md border border-fd-border" tabIndex={0} role="region" aria-label="Capability readiness matrix">
      <table className="w-full min-w-[46rem] border-collapse text-left text-sm">
        <thead>
          <tr className="bg-fd-muted font-mono text-xs uppercase tracking-wider text-fd-muted-foreground">
            <th scope="col" className="px-3 py-2 font-normal">Capability</th>
            <th scope="col" className="px-3 py-2 font-normal">State</th>
            <th scope="col" className="px-3 py-2 font-normal">Public surface</th>
            <th scope="col" className="px-3 py-2 font-normal">How to try</th>
            <th scope="col" className="px-3 py-2 font-normal">Next milestone</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((c) => (
            <tr key={c.id} className="border-t border-fd-border align-top">
              <th scope="row" className="px-3 py-2.5 text-left font-medium text-fd-foreground">
                {c.source ? (
                  <a className="underline decoration-fd-border underline-offset-4 hover:decoration-nm-accent" href={specUrl(c.source)} target="_blank" rel="noopener noreferrer">{c.name}</a>
                ) : c.name}
                <div className="mt-0.5 font-mono text-[0.7rem] font-normal text-fd-muted-foreground">{c.group}</div>
              </th>
              <td className="px-3 py-2.5"><StatusBadge kind={c.state} /></td>
              <td className="px-3 py-2.5 text-fd-muted-foreground">{c.surface}</td>
              <td className="px-3 py-2.5 font-mono text-[0.8rem] text-fd-muted-foreground break-words">{c.howToTry}</td>
              <td className="px-3 py-2.5 text-fd-muted-foreground">{c.next}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
