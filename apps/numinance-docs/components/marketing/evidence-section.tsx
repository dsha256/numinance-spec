import { Section } from '@/components/ui/section';
import { Reveal } from '@/components/motion/reveal';
import { EvidenceCallout, EvidenceStage } from './evidence-stage';

export function EvidenceSection() {
  return (
    <Section
      id="evidence"
      eyebrow="VERIFY THE WORK, NOT THE CLAIM"
      title={
        <>
          An autonomous engineer <br className="hidden md:block" /><em>should be able to prove what happened.</em>
        </>
      }
      lede="The output can look convincing. The record should be independently checkable. Numinance defines a portable evidence format, a chain of recorded actions, trust semantics, and an offline verifier that does not require access to the private intelligence engine."
    >
      <Reveal>
        <EvidenceStage />
        <EvidenceCallout />
      </Reveal>
    </Section>
  );
}

/** Prose block shared by the standalone /evidence and /protocol pages. */
export function EditorialBlock({ eyebrow, title, children }: { eyebrow: string; title: string; children: React.ReactNode }) {
  return (
    <section className="border-t border-nm-line py-16 md:py-20">
      <div className="nm-container grid gap-8 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] md:gap-16">
        <div>
          <p className="nm-eyebrow mb-3">{eyebrow}</p>
          <h2 className="text-2xl font-semibold leading-tight tracking-[-0.02em] md:text-3xl">{title}</h2>
        </div>
        <div className="max-w-[62ch] space-y-4 text-base leading-7 text-nm-muted [&_a]:text-nm-fg [&_a]:underline [&_a]:underline-offset-4 [&_code]:font-mono [&_code]:text-[0.85em] [&_code]:text-nm-fg [&_li]:ml-5 [&_li]:list-disc [&_li]:pl-1">
          {children}
        </div>
      </div>
    </section>
  );
}
