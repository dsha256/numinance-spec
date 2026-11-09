import type { Metadata } from 'next';
import Link from 'next/link';
import { EditorialBlock } from '@/components/marketing/evidence-section';
import { EvidenceCallout, EvidenceStage } from '@/components/marketing/evidence-stage';

export const metadata: Metadata = {
  title: 'Evidence',
  description:
    'How Numinance records autonomous engineering work as a chain of evidence, and why an offline verifier needs caller-pinned trust roots before it can report a verified result.',
  alternates: { canonical: '/evidence' },
};

export default function EvidencePage() {
  return (
    <>
      <header className="nm-container pb-16 pt-32 md:pb-20 md:pt-44">
        <p className="nm-eyebrow mb-5">VERIFY THE WORK, NOT THE CLAIM</p>
        <h1 className="max-w-4xl text-[clamp(2.4rem,6vw,4.6rem)] font-light leading-[1.02] tracking-[-0.035em]">
          An autonomous engineer should be able to prove <span className="text-nm-muted">what happened.</span>
        </h1>
        <p className="nm-lede mt-6">
          The output can look convincing. The record should be independently checkable. This page explains the idea. The technical reference lives in the{' '}
          <Link href="/docs/evidence" className="text-nm-fg underline underline-offset-4">
            evidence docs
          </Link>
          .
        </p>
        <p className="mt-4 font-mono text-xs uppercase tracking-wider text-nm-faint">Pre-alpha · Design intent, not a shipped product</p>
      </header>

      <section className="nm-container pb-20" aria-label="Interactive evidence preview">
        <EvidenceStage />
        <EvidenceCallout />
      </section>

      <EditorialBlock eyebrow="01 · Definition" title="What evidence is">
        <p>
          Evidence is a record of what an autonomous engineer did, kept in a form that someone else can check without asking the engineer. It is separate from
          the output. A pull request can look correct and still say nothing about how it was produced, who reviewed it, or what authority was in force.
        </p>
        <p>
          Numinance defines this record as part of the open specification, as the <code>evidence/v1</code> format. The format is independent of the private
          engine.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="02 · Structure" title="A chain of recorded actions">
        <p>
          Records are appended in order and never edited. Each record carries a reference to the digest of the record before it. Change an earlier record and
          every later reference stops matching.
        </p>
        <p>
          That makes tampering detectable and tells you where it happened. It also means the chain alone proves little: someone who controls the whole chain
          can rebuild it end to end.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="03 · Anchoring" title="Receipts and a transparency log">
        <p>
          To close that gap, the design pairs the chain with signed material from the runner and with an append-only log kept outside the runner&rsquo;s
          control. Conceptually, the runner signs what it did, and the log commits to the fact that the record existed.
        </p>
        <p>
          How receipts and log entries are encoded, and how they are checked, is defined in the specification. See{' '}
          <Link href="/docs/evidence">the reference</Link>.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="04 · Portability" title="Export bundles">
        <p>
          Evidence leaves the system as an export directory. It is a folder with a defined layout, not a single JSON file. The verifier expects that layout and
          is not meant to be pointed at arbitrary files.
        </p>
        <p>You can hand an export to an auditor, a customer or a different team. They do not need an account, a network path to the engine, or its source.</p>
      </EditorialBlock>

      <EditorialBlock eyebrow="05 · Trust roots" title="Why Verified needs caller-pinned keys">
        <p>
          An export can carry keys. Those keys cannot vouch for the export, because whoever produced the bundle could have supplied them. Trust has to come
          from outside it.
        </p>
        <p>
          So a fully verified result requires the caller to supply the runner key(s), the log key and the log origin, subject to the specification. Without
          them, the verifier can still tell you the chain is internally coherent. That is a weaker statement, and the toggle above shows the difference. The
          levels are verified, failed, unauthenticated, self-consistent, incomplete, unsigned and unanchored. There is no built-in trust root, and a key that comes from the export never reaches verified.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="06 · Tooling" title="The offline verifier">
        <p>
          The verifier is a public tool in <code>numinance-spec</code>. It runs offline, on an export directory, with the keys you provide. It does not call
          the engine.
        </p>
        <p>
          The command is <code>numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR</code>. A key mode is required: without one it exits 2.
          Pinned keys can reach <code>verified</code>; <code>--trust-bundle-keys</code> tops out at <code>self-consistent</code> and <code>--skip-signatures</code> at
          <code>unauthenticated</code>. See the <Link href="/docs/cli/evidence-verify">CLI reference</Link>.
        </p>
      </EditorialBlock>

      <EditorialBlock eyebrow="07 · Limits" title="What the verifier does not prove">
        <ul>
          <li>That the engineer&rsquo;s code is correct or free of defects.</li>
          <li>That a policy was well written. It can show a decision was recorded, not that the decision was wise.</li>
          <li>That anything unrecorded did not happen.</li>
          <li>Anything about a key you pinned without checking where it came from.</li>
          <li>Anything beyond the guarantees the specification states for the level reported.</li>
        </ul>
        <p>
          Evidence narrows what you have to take on faith. It does not remove the need for review.{' '}
          <Link href="/docs/evidence">Read the technical reference</Link>, or see the <Link href="/protocol">open protocol</Link> around it.
        </p>
      </EditorialBlock>
    </>
  );
}
