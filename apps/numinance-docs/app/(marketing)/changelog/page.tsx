import type { Metadata } from 'next';
import { Reveal } from '@/components/motion/reveal';
import { StatusList } from '@/components/marketing/status-section';

export const metadata: Metadata = {
  title: 'Status and changelog',
  description: 'Where Numinance stands today: pre-alpha, what is defined, what is under development, and what has been published.',
  alternates: { canonical: '/changelog' },
};

const AREAS = [
  {
    t: 'Public specification',
    d: 'Defined and under active development. Contracts, kinds and governance documents are published for scrutiny and may change.',
  },
  {
    t: 'Evidence format',
    d: 'Evidence v1. See the evidence documentation for the current state of the format and the verification semantics.',
  },
  {
    t: 'SDKs and adapters',
    d: 'Under development. Providers in the public tree (numinance-spec/plugins) are not a readiness claim; folder presence does not imply production support.',
  },
  {
    t: 'Autonomous engine',
    d: 'Proprietary and pre-alpha. Descriptions of engine behavior on this site are design intent, not shipped functionality.',
  },
];

const ENTRIES = [{ title: 'Website foundation and public documentation published', when: 'Initial release' }];

export default function ChangelogPage() {
  return (
    <>
      <header className="nm-container pb-16 pt-32 md:pb-20 md:pt-44">
        <Reveal className="max-w-4xl">
          <p className="nm-eyebrow mb-5">Status and changelog</p>
          <h1 className="text-[clamp(2.4rem,6vw,4.8rem)] font-light leading-[1.02] tracking-[-0.035em]">
            Numinance is pre-alpha.
          </h1>
          <p className="nm-lede mt-8">
            We publish the contract and verification surface early so the boundaries of autonomous engineering can be
            scrutinized before the platform matures. No release dates, completion percentages or general availability
            are claimed.
          </p>
        </Reveal>
      </header>

      <section className="nm-container pb-20" aria-labelledby="status-now">
        <h2 id="status-now" className="sr-only">
          Current status
        </h2>
        <StatusList />
      </section>

      <section className="border-t border-nm-line py-20 md:py-28" aria-labelledby="by-area">
        <div className="nm-container grid gap-10 md:grid-cols-[1fr_2fr]">
          <h2 id="by-area" className="nm-h2">
            What this <em>means.</em>
          </h2>
          <dl className="divide-y divide-nm-line border-y border-nm-line">
            {AREAS.map((a) => (
              <div key={a.t} className="py-6">
                <dt className="text-lg font-medium">{a.t}</dt>
                <dd className="mt-2 max-w-[60ch] text-nm-muted">{a.d}</dd>
              </div>
            ))}
          </dl>
        </div>
      </section>

      <section className="border-t border-nm-line py-20 md:py-28" aria-labelledby="changelog">
        <div className="nm-container grid gap-10 md:grid-cols-[1fr_2fr]">
          <h2 id="changelog" className="nm-h2">
            Changelog
          </h2>
          <ol className="divide-y divide-nm-line border-y border-nm-line">
            {ENTRIES.map((e) => (
              <li key={e.title} className="grid gap-1 py-6 sm:grid-cols-[10rem_1fr]">
                <span className="font-mono text-xs uppercase tracking-[0.14em] text-nm-faint">{e.when}</span>
                <span className="text-lg font-medium">{e.title}</span>
              </li>
            ))}
          </ol>
        </div>
      </section>
    </>
  );
}
