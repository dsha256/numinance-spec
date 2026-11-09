import type { Metadata } from 'next';
import { Reveal } from '@/components/motion/reveal';
import { ButtonLink } from '@/components/ui/button-link';
import { IntegrationsExplorer } from '@/components/marketing/integrations-explorer';

export const metadata: Metadata = {
  title: 'Integrations',
  description:
    'How Numinance extends through typed ports, providers and adapters, each with a declared capability boundary.',
  alternates: { canonical: '/integrations' },
};

const PRINCIPLES = [
  {
    t: 'Ports define the contract',
    d: 'A port is a typed interface for one kind of engineering capability: source control, CI, verification, knowledge. Every port uses the same request, result and diagnostic conventions.',
  },
  {
    t: 'Adapters implement it',
    d: 'A provider or adapter connects an existing system to a port. Swapping one adapter for another does not change how the engine reasons or how policy is evaluated.',
  },
  {
    t: 'Boundaries are declared',
    d: 'An adapter states what it may and may not do. The host enforces that declaration. Extension happens through typed, governed interfaces instead of unrestricted access.',
  },
];

export default function IntegrationsPage() {
  return (
    <>
      <header className="nm-container pb-16 pt-32 md:pb-24 md:pt-44">
        <Reveal className="max-w-4xl">
          <p className="nm-eyebrow mb-5">Integrations</p>
          <h1 className="text-[clamp(2.4rem,6vw,4.8rem)] font-light leading-[1.02] tracking-[-0.035em]">
            Open interfaces, <br className="hidden sm:block" />
            <span className="text-nm-muted">not custom plumbing.</span>
          </h1>
          <p className="nm-lede mt-8">
            Numinance reaches the systems engineers already use through provider and adapter contracts. The contract is
            public. The intelligence behind it is not.
          </p>
        </Reveal>
      </header>

      <section className="border-t border-nm-line py-20 md:py-28" aria-labelledby="contracts">
        <div className="nm-container grid gap-12 md:grid-cols-[1fr_2fr]">
          <h2 id="contracts" className="nm-h2">
            Provider and adapter <em>contracts</em>
          </h2>
          <ol className="divide-y divide-nm-line border-y border-nm-line">
            {PRINCIPLES.map((p, i) => (
              <li key={p.t} className="grid gap-2 py-6 sm:grid-cols-[3rem_1fr]">
                <span className="font-mono text-xs text-nm-faint">0{i + 1}</span>
                <div>
                  <h3 className="text-lg font-medium">{p.t}</h3>
                  <p className="mt-2 max-w-[60ch] text-nm-muted">{p.d}</p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section className="border-t border-nm-line py-20 md:py-28" aria-labelledby="explore">
        <div className="nm-container">
          <h2 id="explore" className="nm-h2 mb-4">
            Select a port. <em>Select an adapter.</em>
          </h2>
          <p className="nm-lede mb-12">
            See the shared contract and the capability boundary an adapter declares.
          </p>
          <IntegrationsExplorer />
        </div>
      </section>

      <section className="border-t border-nm-line py-20 md:py-28" aria-labelledby="develop">
        <div className="nm-container grid gap-10 md:grid-cols-[1fr_2fr]">
          <h2 id="develop" className="nm-h2">
            Adapter development <em>and conformance</em>
          </h2>
          <div>
            <p className="nm-lede">
              Building an adapter means implementing a port and declaring its boundary. Conformance suites are intended
              to let anyone check an implementation against the contract, independently of Numinance.
            </p>
            <p className="mt-4 max-w-[62ch] text-nm-muted">
              Providers listed here exist in the public tree under numinance-spec/plugins. Numinance is pre-alpha: presence in the tree does not imply production support.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <ButtonLink href="/docs/integrations">Integration docs</ButtonLink>
              <ButtonLink href="/docs/integrations/conformance" variant="secondary">
                Conformance
              </ButtonLink>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
