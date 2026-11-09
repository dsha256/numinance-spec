/**
 * Central status vocabulary. Every badge in the docs is rendered from this table, so the
 * wording and meaning stay identical everywhere. Do not hand-write status prose in MDX.
 */
export type StatusKind =
  | 'public-spec'
  | 'pre-alpha'
  | 'designed'
  | 'planned'
  | 'coming-soon'
  | 'illustrative';

export interface StatusMeta {
  label: string;
  /** One sentence: what the label promises and what it does not. */
  meaning: string;
  /** Semantic tone, rendered with text + glyph as well as colour. */
  tone: 'ok' | 'info' | 'warn' | 'muted' | 'accent';
  glyph: string;
}

export const STATUS: Record<StatusKind, StatusMeta> = {
  'public-spec': {
    label: 'Available in public spec',
    meaning: 'Source, schema, CLI function or verifier exists in the public numinance-spec tree. It can still be pre-alpha.',
    tone: 'ok',
    glyph: '●',
  },
  'pre-alpha': {
    label: 'Pre-alpha',
    meaning: 'Code exists, but APIs, packaging and stability may change before a tagged release.',
    tone: 'info',
    glyph: '◐',
  },
  designed: {
    label: 'Under development',
    meaning: 'Part of the designed architecture. Not verified as distributed or runnable; interfaces may change.',
    tone: 'warn',
    glyph: '◌',
  },
  planned: {
    label: 'Planned distribution',
    meaning: 'An install channel is intended. It has not been published or verified, so no install command is given.',
    tone: 'muted',
    glyph: '◇',
  },
  'coming-soon': {
    label: 'Coming soon',
    meaning: 'On the roadmap. No package name, version, date or command is promised.',
    tone: 'accent',
    glyph: '◆',
  },
  illustrative: {
    label: 'Illustrative',
    meaning: 'A simulation or example. It is not connected to a running engine and its output is not from a real run.',
    tone: 'muted',
    glyph: '○',
  },
};

export const STATUS_ORDER: StatusKind[] = ['public-spec', 'pre-alpha', 'designed', 'planned', 'coming-soon', 'illustrative'];
