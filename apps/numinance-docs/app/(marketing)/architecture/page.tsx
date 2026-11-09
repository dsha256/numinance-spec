import type { Metadata } from 'next';
import Link from 'next/link';
import { Reveal } from '@/components/motion/reveal';
import { Section } from '@/components/ui/section';
import { ButtonLink } from '@/components/ui/button-link';
import { CodeWindow } from '@/components/ui/code-window';
import { ArchitectureExplorer } from '@/components/marketing/architecture-explorer';
import { GovernanceInspector } from '@/components/marketing/governance-inspector';
import { siteConfig } from '@/lib/site-config';

export const metadata: Metadata = {
  title: 'Architecture',
  description:
    'How Numinance separates private intelligence from an open contract: a fixed engineering loop, external policy, bounded grants, distinct identities, customer-controlled execution and independently verifiable evidence.',
  alternates: { canonical: '/architecture' },
};

const LOOP = [
  { n: '01', t: 'Understand', d: 'Discover context, contracts and constraints.' },
  { n: '02', t: 'Plan', d: 'Choose a bounded approach and the checks expected to pass.' },
  { n: '03', t: 'Act', d: 'Invoke explicitly authorized capabilities.' },
  { n: '04', t: 'Verify', d: 'Evaluate results against independent checks.' },
  { n: '05', t: 'Evidence', d: 'Preserve records for outside verification.' },
];

const SPLIT = [
  {
    title: 'Numinance Engine',
    badge: 'Proprietary',
    items: [
      'The autonomous engineering intelligence and orchestration runtime.',
      'Reasoning about codebases and constraints, and routing work to appropriate executors.',
      'Not published in numinance-spec. Internal algorithms and infrastructure are not described on this site.',
    ],
  },
  {
    title: 'numinance-spec',
    badge: 'Apache-2.0',
    items: [
      'Protobuf definitions, typed ports and governance annotations.',
      'Configuration kinds and JSON Schemas.',
      'The evidence/v1 chain and export verification specification, plus the numi CLI for public capabilities.',
      'Conformance suites, sample plugins, integration adapters and SDKs, as present in the public tree.',
    ],
  },
];

const GRANT_BOUNDS = [
  { t: 'Authority', d: 'Which capability, on which resource, for which principal.' },
  { t: 'Risk', d: 'How consequential the action is, as classified by the governance vocabulary.' },
  { t: 'Environment', d: 'Where the action may run.' },
  { t: 'Expiration', d: 'Grants are time-limited. An expired grant authorizes nothing.' },
];

const CLOUD = [
  {
    t: 'Designed to stay in the customer account',
    d: 'Source code, credentials, inference and infrastructure actions. The data plane runs in the customer’s cloud account.',
  },
  {
    t: 'What the vendor control plane sees',
    d: 'Limited coordination metadata. Metadata is not code and it is not credentials. The design bounds what crosses the line. It does not claim that no data of any kind ever crosses it.',
  },
  {
    t: 'Credential custody',
    d: 'A credential broker and controlled egress stand between sandboxed work and secrets. Work receives short-lived opaque handles, not reusable secrets.',
  },
];

const IDENTIFIERS = [
  { id: 'numinance.plugin.v1', d: 'The plugin protocol package.' },
  { id: 'PluginService', d: 'The service a plugin implements.' },
  { id: 'HostService', d: 'The service the host exposes to plugins.' },
  { id: 'ActionContext', d: 'The context an invocation carries.' },
  { id: 'ActionResult', d: 'The typed outcome of an invocation.' },
  { id: 'Diagnostic', d: 'A structured report of a problem.' },
  { id: 'Envelope', d: 'The common wrapper for configuration kinds, in numinance.kinds.v1.' },
  { id: 'Digest · Reference', d: 'Content addressing and typed links between records.' },
];

const ONWARD = [
  { href: '/docs/architecture', t: 'Architecture docs', d: 'The model in reference form.' },
  { href: '/docs/concepts', t: 'Concepts', d: 'Vocabulary and building blocks.' },
  { href: '/protocol', t: 'Protocol', d: 'The open contract and its boundary.' },
  { href: '/evidence', t: 'Evidence', d: 'What verification checks, and what it trusts.' },
];

function Prose({ title, children, id }: { title: React.ReactNode; children: React.ReactNode; id: string }) {
  return (
    <section id={id} aria-labelledby={`${id}-h`} className="border-t border-nm-line py-20 md:py-28">
      <div className="nm-container grid gap-10 md:grid-cols-[1fr_2fr] md:gap-16">
        <Reveal>
          <h2 id={`${id}-h`} className="nm-h2">
            {title}
          </h2>
        </Reveal>
        <Reveal delay={0.08} className="min-w-0">
          {children}
        </Reveal>
      </div>
    </section>
  );
}

export default function ArchitecturePage() {
  return (
    <>
      <header className="nm-container pb-16 pt-32 md:pb-24 md:pt-44">
        <Reveal className="max-w-4xl">
          <p className="nm-eyebrow mb-5">Architecture</p>
          <h1 className="text-[clamp(2.4rem,6vw,4.8rem)] font-light leading-[1.02] tracking-[-0.035em]">
            Intelligence and trust <br className="hidden sm:block" />
            <span className="text-nm-muted">are separate layers.</span>
          </h1>
          <p className="nm-lede mt-8">
            The engine reasons. Policy grants authority. The evidence can be verified independently. This page walks
            through each layer and the boundary that separates it from the next.
          </p>
          <p className="mt-8 max-w-2xl border-l-2 border-nm-accent pl-4 text-sm leading-relaxed text-nm-muted">
            <span className="font-medium text-nm-fg">Status.</span> This describes the designed model. Numinance is
            pre-alpha. Nothing here claims a shipped product, a customer deployment or a certification.
          </p>
        </Reveal>
      </header>

      <Section id="layers" eyebrow="The model, end to end" title={<>Seven layers, <em>one direction of authority.</em></>}>
        <ArchitectureExplorer />
      </Section>

      <Prose id="split" title={<>Private intelligence. <em>Open contract.</em></>}>
        <p className="nm-lede">
          Numinance is two things with a deliberate line between them. The engine is proprietary. The contract around it
          is public and meant to be inspected and implemented independently.
        </p>
        <div className="mt-10 grid gap-px overflow-hidden rounded-xl border border-nm-line bg-nm-line sm:grid-cols-2">
          {SPLIT.map((s) => (
            <div key={s.title} className="bg-nm-raised p-6">
              <div className="flex items-baseline justify-between gap-4">
                <h3 className="text-lg font-semibold tracking-tight">{s.title}</h3>
                <span className="font-mono text-[0.72rem] uppercase tracking-[0.12em] text-nm-faint">{s.badge}</span>
              </div>
              <ul className="mt-4 space-y-3 text-sm leading-relaxed text-nm-muted">
                {s.items.map((i) => (
                  <li key={i} className="border-t border-nm-line pt-3 first:border-t-0 first:pt-0">
                    {i}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <p className="mt-6 text-sm text-nm-muted">
          {siteConfig.repo.public ? (
            <a href={siteConfig.repo.url ?? '#'} className="underline underline-offset-4" target="_blank" rel="noreferrer noopener">
              numinance-spec repository
            </a>
          ) : (
            <span className="font-mono text-[0.72rem] uppercase tracking-[0.12em]">numinance-spec · Pre-publication</span>
          )}
        </p>
      </Prose>

      <Prose id="loop" title={<>A fixed <em>engineering loop.</em></>}>
        <p className="nm-lede">
          The engine does not improvise its process. Every objective moves through the same five states, which turns
          open-ended work into a sequence of inspectable decisions.
        </p>
        <ol className="mt-10 divide-y divide-nm-line border-y border-nm-line">
          {LOOP.map((l) => (
            <li key={l.n} className="grid grid-cols-[3rem_8rem_1fr] items-baseline gap-x-4 py-4 sm:grid-cols-[3rem_9rem_1fr]">
              <span className="font-mono text-[0.72rem] text-nm-faint">{l.n}</span>
              <span className="font-mono text-[0.8rem] uppercase tracking-[0.08em]">{l.t}</span>
              <span className="text-sm text-nm-muted">{l.d}</span>
            </li>
          ))}
        </ol>
        <p className="mt-6 text-sm text-nm-muted">The loop is a design. Individual stages are under development.</p>
      </Prose>

      <Prose id="grants" title={<>Bounded <em>grants.</em></>}>
        <p className="nm-lede">
          The model proposes. A proposed action is not an authorization. Before a privileged action, policy is evaluated
          outside the language model, and a permitted request receives a grant with explicit limits.
        </p>
        <dl className="mt-10 grid gap-px overflow-hidden rounded-xl border border-nm-line bg-nm-line sm:grid-cols-2">
          {GRANT_BOUNDS.map((g) => (
            <div key={g.t} className="bg-nm-raised p-5">
              <dt className="nm-eyebrow mb-2">{g.t}</dt>
              <dd className="text-sm leading-relaxed text-nm-muted">{g.d}</dd>
            </div>
          ))}
        </dl>
        <div className="mt-12">
          <h3 className="mb-6 text-lg font-semibold tracking-tight">Decision inspector</h3>
          <GovernanceInspector />
        </div>
      </Prose>

      <Prose id="identity" title={<>Identity <em>separation.</em></>}>
        <p className="nm-lede">
          Authorship, review, release and approval belong to different identities. The author of a change cannot approve
          it. Under the design, deployment authority belongs to the release identity, not to the agent that wrote the code.
        </p>
        <p className="mt-6 text-sm text-nm-muted">
          The strip in the decision inspector above shows the four roles. Roles are enforced by policy, not by asking the
          model to behave.
        </p>
      </Prose>

      <Prose id="customer-cloud" title={<>The customer-cloud <em>model.</em></>}>
        <p className="nm-lede">
          The technical design separates a customer-controlled data plane from vendor control-plane services. This is the
          designed deployment model. It does not imply a generally available on-premises installer or that every topology
          has shipped.
        </p>
        <ul className="mt-10 divide-y divide-nm-line border-y border-nm-line">
          {CLOUD.map((c) => (
            <li key={c.t} className="grid gap-2 py-5 sm:grid-cols-[14rem_1fr] sm:gap-8">
              <h3 className="text-base font-semibold tracking-tight">{c.t}</h3>
              <p className="text-sm leading-relaxed text-nm-muted">{c.d}</p>
            </li>
          ))}
        </ul>
      </Prose>

      <Prose id="plugins" title={<>Plugins and the <em>host boundary.</em></>}>
        <p className="nm-lede">
          Extensions run behind a typed boundary. A plugin implements one side of a published contract. The host
          mediates the other. Sandboxed work reaches nothing the host has not been asked, and allowed, to provide.
        </p>
        <dl className="mt-10 divide-y divide-nm-line border-y border-nm-line">
          {IDENTIFIERS.map((i) => (
            <div key={i.id} className="grid gap-1 py-3.5 sm:grid-cols-[14rem_1fr] sm:gap-8">
              <dt className="font-mono text-[0.8rem]">{i.id}</dt>
              <dd className="text-sm text-nm-muted">{i.d}</dd>
            </div>
          ))}
        </dl>
        <p className="mt-6 text-sm text-nm-muted">
          Descriptions are conceptual. Exact fields and members are defined in numinance-spec.
        </p>
      </Prose>

      <Prose id="evidence" title={<>Evidence, <em>checked outside.</em></>}>
        <p className="nm-lede">
          Execution produces evidence, artifacts and receipts. Exported evidence is meant to be inspected independently of
          the engine. The public verifier can run offline.
        </p>
        <CodeWindow title="terminal" copy="numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR" badge="Illustrative" className="mt-10">
          <p className="whitespace-pre-wrap break-all">
            <span className="text-white/40">$ </span>numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR
          </p>
        </CodeWindow>
        <p className="mt-6 text-sm leading-relaxed text-nm-muted">
          The verifier does not silently trust keys supplied by the export. A fully verified result requires the caller to
          pin the runner key(s), the log key and the origin, as the evidence specification defines. There is no built-in trust root.
        </p>
      </Prose>

      <section className="border-t border-nm-line py-20 md:py-28" aria-labelledby="onward-h">
        <div className="nm-container">
          <h2 id="onward-h" className="nm-h2 mb-10">
            Keep <em>reading.</em>
          </h2>
          <ul className="grid gap-px overflow-hidden rounded-xl border border-nm-line bg-nm-line sm:grid-cols-2 lg:grid-cols-4">
            {ONWARD.map((o) => (
              <li key={o.href} className="bg-nm-raised">
                <Link href={o.href} className="block h-full p-6 transition-colors hover:bg-nm-surface">
                  <span className="text-base font-semibold tracking-tight">{o.t}</span>
                  <span className="mt-2 block text-sm text-nm-muted">{o.d}</span>
                  <span className="mt-4 block font-mono text-[0.72rem] text-nm-faint">{o.href}</span>
                </Link>
              </li>
            ))}
          </ul>
          <div className="mt-12 flex flex-wrap gap-3">
            <ButtonLink href="/docs/architecture">Read the architecture docs</ButtonLink>
            <ButtonLink href="/protocol" variant="secondary">
              Explore the protocol
            </ButtonLink>
          </div>
        </div>
      </section>
    </>
  );
}
