import { Section } from '@/components/ui/section';
import { Reveal } from '@/components/motion/reveal';
import { ButtonLink } from '@/components/ui/button-link';

export const STATUS_ITEMS = [
  { label: 'PUBLIC SPECIFICATION', value: 'Defined / under active development' },
  { label: 'EVIDENCE FORMAT', value: 'Evidence v1 / see docs' },
  { label: 'SDKs & ADAPTERS', value: 'Under development' },
  { label: 'AUTONOMOUS ENGINE', value: 'Proprietary / pre-alpha' },
] as const;

export function StatusList() {
  return (
    <dl className="grid border-y border-nm-line sm:grid-cols-2 lg:grid-cols-4">
      {STATUS_ITEMS.map((s, i) => (
        <Reveal
          key={s.label}
          delay={i * 0.065}
          className="border-b border-nm-line py-6 last:border-b-0 sm:px-6 sm:odd:pl-0 lg:border-b-0 lg:border-l lg:first:border-l-0 lg:first:pl-0"
        >
          <dt className="nm-eyebrow">{s.label}</dt>
          <dd className="mt-3 text-lg font-medium leading-snug tracking-tight">{s.value}</dd>
        </Reveal>
      ))}
    </dl>
  );
}

export function StatusSection() {
  return (
    <Section
      id="status"
      eyebrow="EARLY IN THE JOURNEY"
      title="The foundation is taking shape."
      lede="Numinance is pre-alpha. We're publishing the contract and verification surface early because the boundaries of autonomous engineering deserve scrutiny before the platform reaches maturity."
    >
      <StatusList />
      <div className="mt-10">
        <ButtonLink href="/changelog" variant="ghost" className="-ml-3">
          Status and changelog
        </ButtonLink>
      </div>
    </Section>
  );
}
