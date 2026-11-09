/**
 * Two illustrative workflows for the CLI figure. Every file name, file body and printed result below
 * is ILLUSTRATIVE: nothing was captured from a run. The commands are real (see cli/README.md and the evidence verifier in numinance-spec).
 */

export type Tone = 'ok' | 'bad' | 'weak' | 'pending';

/** One line changed by the scene: `pre` + (`from` → `to`) + `post` */
export interface Edit {
  file: number;
  line: number;
  pre: string;
  from: string;
  to: string;
  post: string;
  midKind: 'key' | 'marker';
  /** shown at the line's end while the line is flagged / after it was rewritten */
  before: string;
  after: string;
  beforeTone: Tone;
}

export interface FileDef {
  /** path segments; the last is the file name */
  path: readonly string[];
  /** lines as they read before the edit (the edit's line holds `pre + from + post`) */
  lines: readonly string[];
}

export interface TreeRow {
  name: string;
  depth: number;
  dir?: boolean;
  /** index of the file this row is (files) or the first file inside it (folders): lands → row shows */
  file: number;
}

export interface ResultRow {
  label: string;
  /** value per stage: [flagged, after the edit] */
  value: readonly [string, string];
  tone: readonly [Tone, Tone];
}

export interface Workflow {
  id: 'validate' | 'verify';
  label: string;
  hint: string;
  command: string;
  /** real commands get a copy button */
  confirmed: boolean;
  /** short tab label and the chip under the command */
  badge: string;
  chip: string;
  source: { name: string; note: string };
  root: string;
  files: readonly FileDef[];
  tree: readonly TreeRow[];
  edit: Edit;
  /** static, always-on notes: [file, line, text] */
  notes: readonly (readonly [number, number, string])[];
  readout: { run: string; check: string; lit: string };
  result: readonly ResultRow[];
  resultFoot: string;
  summary: string;
}

export const VERIFY = 'numi evidence verify --runner-key FILE --log-key FILE --log-origin ORIGIN DIR';

const envelope = (name: string): string[] => [
  '# illustrative — not captured from a run',
  'apiVersion: …',
  'kind: …',
  'metadata:',
  `  name: ${name}`,
  'spec:',
  '  …',
  'extensions: …',
];

const profile = envelope('<name>');
profile[3] = 'metdata:';

const record = (n: number): string[] => [
  `# record-${n} · illustrative`,
  '<digest of previous record>',
  '…',
  '<digest of this record>',
];

export const WORKFLOWS: readonly Workflow[] = [
  {
    id: 'validate',
    label: 'Validate configuration',
    hint: 'schema check of a kinds-envelope file',
    command: 'go run ./cli/numi validate --files schemas/examples/SDLCProfile',
    confirmed: true,
    badge: 'from cli/README.md',
    chip: 'from cli/README.md · schema + loader checks only',
    source: { name: 'numinance-spec', note: 'schemas/examples' },
    root: 'schemas',
    files: [
      { path: ['schemas', 'examples', 'SDLCProfile', 'payments-api.yaml'], lines: profile },
      { path: ['schemas', 'examples', 'SDLCProfile', 'org-baseline.yaml'], lines: envelope('<name>') },
    ],
    tree: [
      { name: 'schemas', depth: 0, dir: true, file: 0 },
      { name: 'examples', depth: 1, dir: true, file: 0 },
      { name: 'SDLCProfile', depth: 2, dir: true, file: 0 },
      { name: 'payments-api.yaml', depth: 3, file: 0 },
      { name: 'org-baseline.yaml', depth: 3, file: 1 },
    ],
    edit: {
      file: 0,
      line: 3,
      pre: '',
      from: 'metdata',
      to: 'metadata',
      post: ':',
      midKind: 'key',
      before: '← diagnostic · <message>',
      after: '← corrected',
      beforeTone: 'bad',
    },
    notes: [],
    readout: { run: 'validating files', check: '2 files checked', lit: 'VALIDATE' },
    result: [
      { label: 'files', value: ['2 files checked', '2 files checked'], tone: ['ok', 'ok'] },
      { label: 'diagnostics', value: ['1 · payments-api.yaml:4 · <message>', '0'], tone: ['bad', 'ok'] },
      { label: 'schema (open CLI)', value: ['<diagnostics>', '<pass>'], tone: ['bad', 'ok'] },
      { label: 'profile resolution', value: ['not covered · private engine', 'not covered · private engine'], tone: ['weak', 'weak'] },
      { label: 'readiness', value: ['not implied', 'not implied'], tone: ['weak', 'weak'] },
    ],
    resultFoot: 'A schema pass says nothing about policy resolution.',
    summary:
      'Illustrative run of the validate command (from cli/README.md). Two files land in the project tree, one line in the first file is flagged, corrected, and the schema result changes. Schema validation is not profile resolution and not readiness.',
  },
  {
    id: 'verify',
    label: 'Verify evidence',
    hint: 'verifier over an evidence/v1 export directory',
    command: VERIFY,
    confirmed: true,
    badge: 'real verifier',
    chip: 'real command · evidence/v1 export directory',
    source: { name: 'DIR', note: 'caller’s export directory' },
    root: 'evidence-export',
    files: [
      {
        path: ['evidence-export', 'bundle-manifest.json'],
        lines: [
          '# annotated view of the export · illustrative',
          'export     evidence/v1',
          'records    <n>',
          'links      unverified',
          'keys       caller must pin keys',
          'level      not reported',
        ],
      },
      { path: ['evidence-export', 'records.ndjson'], lines: record(1) },
      { path: ['evidence-export', 'head-checkpoint.json'], lines: record(2) },
      { path: ['evidence-export', 'attestation.json'], lines: record(3) },
    ],
    tree: [
      { name: 'evidence-export', depth: 0, dir: true, file: 0 },
      { name: 'bundle-manifest.json', depth: 1, file: 0 },
      { name: 'records.ndjson', depth: 1, file: 1 },
      { name: 'head-checkpoint.json', depth: 1, file: 2 },
      { name: 'attestation.json', depth: 1, file: 3 },
    ],
    edit: {
      file: 0,
      line: 3,
      pre: 'links      ',
      from: 'unverified',
      to: 'consistent',
      post: '',
      midKind: 'marker',
      before: '← not yet checked',
      after: '← structure only',
      beforeTone: 'pending',
    },
    notes: [[0, 4, '← no built-in trust root']],
    readout: { run: 'verifying export', check: '4 files read', lit: 'VERIFY' },
    result: [
      { label: 'bundle-manifest', value: ['found', 'found'], tone: ['ok', 'ok'] },
      { label: 'record links', value: ['unverified', 'consistent'], tone: ['pending', 'ok'] },
      { label: 'keys', value: ['pinned by the caller (FILE)', 'pinned by the caller (FILE)'], tone: ['weak', 'weak'] },
      { label: 'level', value: ['pending', 'not reported · SIMULATED'], tone: ['weak', 'weak'] },
    ],
    resultFoot: 'Hash links prove structure only; verified needs pinned keys, never keys from the export.',
    summary:
      'Illustrative run of the real evidence verify command over an export directory. Four files land in the tree, the record links change from unverified to consistent, and the level stays unreported: verified needs runner and log keys pinned by the caller, and there is no built-in trust root.',
  },
];
