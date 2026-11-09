/**
 * Central site configuration. The public `numinance-spec` repository is live; the
 * Apache-2.0 license applies to it only, not to the proprietary Numinance engine.
 * Nothing here is a claim about shipped product functionality (the project is pre-alpha).
 */
/** The one official public Numinance specification repository. */
export const SPEC_REPO_URL = 'https://github.com/dsha256/numinance-spec';

export const siteConfig = {
  name: 'Numinance',
  url: process.env.NEXT_PUBLIC_SITE_URL ?? 'http://localhost:3000',
  title: 'Numinance — Autonomous AI Engineering, Governed and Verifiable',
  description:
    'Numinance is building an AI software engineer that understands your systems, works within configurable autonomy boundaries, and produces independently verifiable evidence.',
  ogTitle: 'Give it context. Let it engineer.',
  ogLine: 'Configurable autonomy. Governed execution. Independently verifiable evidence.',
  tagline: 'Autonomous engineering. Verifiable by design.',
  /**
   * The public specification repository (external GitHub). Fixed on purpose — no environment
   * variable may redirect it. It is distinct from the internal `/protocol` and `/docs` pages,
   * which explain the specification.
   */
  repo: {
    url: SPEC_REPO_URL,
    public: true,
  },
} as const;

export const nav = [
  { label: 'Product', href: '/#system' },
  { label: 'Architecture', href: '/architecture' },
  { label: 'Protocol', href: '/protocol' },
  { label: 'Evidence', href: '/evidence' },
  { label: 'Docs', href: '/docs' },
] as const;
