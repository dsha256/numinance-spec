/**
 * Feedback figure content. HONESTY: Numinance is pre-alpha; there are NO customer testimonials.
 * The only person card is the featured attribution below.
 */

export type QuoteStatus = 'draft' | 'approved';

export interface Author {
  id: string;
  name: string;
  /** An individual's job title. Attribution is personal: it implies NO company endorsement. */
  role: string;
  featured: boolean;
  /**
   * PROPOSED COPY, not a verified quotation. It needs David Shainidze's explicit approval before
   * release: set `quoteStatus` to 'approved' ONLY after he has signed it off (or replace the text
   * with the wording he approves). The component renders the quote solely when approved.
   */
  quote: string;
  quoteStatus: QuoteStatus;
  /** A real, authorised portrait URL if one exists; otherwise null (initials are shown). */
  avatar: string | null;
}

export const DAVID: Author = {
  id: 'david-shainidze',
  name: 'David Shainidze',
  role: 'Principal Engineer at Fundbox',
  featured: true,
  quote:
    'Great developer experiences are built from clear abstractions, thoughtful details, and tools that stay out of your way.',
  quoteStatus: 'draft',
  avatar: null,
};

/** The only way the UI may read a quote: undefined unless the author approved it. */
export function publishedQuote(author: Author): string | undefined {
  return author.quoteStatus === 'approved' ? author.quote : undefined;
}

export type Socket = 'numinance-spec' | 'evidence/v1' | 'conformance' | 'feedback bus';

export interface Link {
  href: string;
  label: string;
}

export type Entry =
  | { id: 'rfc' | 'status' | 'conformance'; kind: 'note'; from: Socket; title: string; tag: string; body: string[]; link: Link }
  | { id: 'david'; kind: 'person'; from: Socket; title: string; author: Author };

/** Reading / animation order. All hrefs are existing routes. */
export const ENTRIES: readonly Entry[] = [
  {
    id: 'rfc',
    kind: 'note',
    from: 'numinance-spec',
    title: 'rfc-review.md',
    tag: 'RFC / REVIEW',
    body: [
      'Found a weak assumption? Challenge it before it becomes a contract.',
    ],
    link: { href: '/docs/project/rfcs', label: 'Read the RFC process' },
  },
  {
    id: 'david',
    kind: 'person',
    from: 'feedback bus',
    title: 'voice / featured',
    author: DAVID,
  },
  {
    id: 'status',
    kind: 'note',
    from: 'evidence/v1',
    title: 'status.txt',
    tag: 'STATUS / PRE-ALPHA',
    body: [
      'The architecture is public for scrutiny. The autonomous engine is still in development.',
      'No customers, adopters or endorsements are claimed. Nothing here is a benchmark or a certification.',
    ],
    link: { href: '/changelog', label: 'See what has changed' },
  },
  {
    id: 'conformance',
    kind: 'note',
    from: 'conformance',
    title: 'adapters.md',
    tag: 'CONFORMANCE / CONTRIBUTE',
    body: [
      'Compatibility should be demonstrated, not assumed. Help shape the checks.',
    ],
    link: { href: '/docs/integrations/conformance', label: 'Read about conformance' },
  },
];
