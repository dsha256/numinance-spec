import type { Metadata } from 'next';
import Link from 'next/link';
import { EditorialBlock } from '@/components/marketing/evidence-section';
import { ProtocolLinks } from '@/components/marketing/protocol-section';
import { ProtoPackages, ProtocolSplit, SdkShowcase } from '@/components/marketing/protocol-split';

export const metadata: Metadata = {
  title: 'Open protocol',
  description:
    'The Numinance engine is private. numinance-spec is the Apache-2.0 contract around it: protobuf definitions, schemas, SDKs, conformance, a CLI and an evidence verifier.',
  alternates: { canonical: '/protocol' },
};

export default function ProtocolPage() {
  return (
    <>
      <header className="nm-container pb-16 pt-32 md:pb-20 md:pt-44">
        <p className="nm-eyebrow mb-5">THE OPEN BOUNDARY</p>
        <h1 className="max-w-4xl text-[clamp(2.4rem,6vw,4.6rem)] font-light leading-[1.02] tracking-[-0.035em]">
          The intelligence is private. <span className="text-nm-muted">The contract is open.</span>
        </h1>
        <p className="nm-lede mt-6">
          <code className="font-mono text-[0.9em] text-nm-fg">numinance-spec</code> is the Apache-2.0 integration and verification layer around Numinance.
          This page is the overview. Details live in the{' '}
          <Link href="/docs/protocol" className="text-nm-fg underline underline-offset-4">
            protocol docs
          </Link>
          .
        </p>
        <p className="mt-4 font-mono text-xs uppercase tracking-wider text-nm-faint">Pre-alpha · Specification published separately</p>
      </header>

      <section className="nm-container space-y-6 pb-20" aria-label="Protocol overview">
        <ProtocolSplit />
        <ProtoPackages />
        <SdkShowcase />
        <ProtocolLinks />
      </section>

      <EditorialBlock eyebrow="01 · Why a boundary" title="Two things, kept apart">
        <p>
          The engine is the part that reasons about code and decides what to do. It is proprietary and stays private. Everything it needs in order to be
          governed, extended and checked is expressed as a contract that is open.
        </p>
        <p>
          The line is deliberate. You should not need the engine&rsquo;s source to know what it is allowed to do, what it hands to a plugin, or what it
          recorded.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="02 · What you can do" title="Work against the contract">
        <ul>
          <li>Read the protobuf definitions and generate clients.</li>
          <li>Build adapters and plugins against typed, governed interfaces.</li>
          <li>Validate configuration against published schemas.</li>
          <li>Run conformance suites against your own implementation.</li>
          <li>
            Verify <Link href="/evidence">evidence</Link> offline.
          </li>
        </ul>
      </EditorialBlock>

      <EditorialBlock eyebrow="03 · Stability" title="Governance, versioning and RFCs">
        <p>
          Contracts are versioned in their package names. Changes to them go through a public RFC process, and the project documents how it is governed.
        </p>
        <p>
          See <Link href="/docs/project/governance">governance</Link>, <Link href="/docs/project/versioning">versioning</Link> and the{' '}
          <Link href="/docs/project/rfcs">RFCs</Link>.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="04 · Honest status" title="What this page does not claim">
        <p>
          Numinance is pre-alpha. The listings above name contract packages and categories of material. They are not a promise that any adapter or SDK is
          ready. Providers and SDKs are described from the public tree (numinance-spec); the repository is public on GitHub.
        </p>
        <p>
          Start with the <Link href="/docs/sdk">SDK guide</Link> or the <Link href="/docs/protocol">protocol reference</Link>.
        </p>
      </EditorialBlock>
    </>
  );
}
