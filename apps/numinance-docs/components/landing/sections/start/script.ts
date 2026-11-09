/**
 * The terminal session shown on the CRT. ILLUSTRATIVE: not captured from a run. All three commands are
 * real (evidence verifier; cli/README.md). Output is placeholder prose, with no invented output keys.
 */
export type Kind = 'dim' | 'boot' | 'prompt' | 'out' | 'note' | 'blank';
export interface Line {
  text: string;
  kind: Kind;
  /** typed key by key (prompts) rather than printed at once */
  typed?: boolean;
}

export const SESSION: readonly Line[] = [
  { kind: 'dim', text: 'illustrative session — not captured from a run' },
  { kind: 'boot', text: '[ ok ] display   80x24' },
  { kind: 'boot', text: '[ ok ] terminal  ready' },
  { kind: 'blank', text: '' },
  { kind: 'prompt', typed: true, text: '$ numi evidence verify --runner-key FILE \\' },
  { kind: 'prompt', typed: true, text: '    --log-key FILE --log-origin ORIGIN DIR' },
  { kind: 'out', text: '  reading DIR ...' },
  { kind: 'out', text: '  hash links: <structural check>' },
  { kind: 'out', text: '  keys: <pinned by the caller>' },
  { kind: 'out', text: '  result: <illustrative placeholder>' },
  { kind: 'note', text: '  no built-in trust root.' },
  { kind: 'blank', text: '' },
  { kind: 'dim', text: '# from cli/README.md' },
  { kind: 'prompt', typed: true, text: '$ go run ./cli/numi version' },
  { kind: 'dim', text: '# from cli/README.md' },
  { kind: 'prompt', typed: true, text: '$ go run ./cli/numi validate --files \\' },
  { kind: 'prompt', typed: true, text: '    schemas/examples/SDLCProfile' },
];

/** Cost of a line in the typing budget: typed keys cost 1 each, printed lines a quarter per char. */
const cost = (l: Line) => (l.typed ? l.text.length : Math.max(2, l.text.length * 0.22));
export const TOTAL = SESSION.reduce((n, l) => n + cost(l), 0);

/** The lines (with the last one possibly partial) that are on the glass at `t` in 0..1. */
export function visible(t: number): { lines: { line: Line; text: string }[]; typing: boolean } {
  let budget = t * TOTAL;
  const lines: { line: Line; text: string }[] = [];
  let typing = false;
  for (const line of SESSION) {
    if (budget <= 0) break;
    const c = cost(line);
    if (budget >= c) {
      lines.push({ line, text: line.text });
      budget -= c;
    } else {
      lines.push({ line, text: line.text.slice(0, Math.floor((budget / c) * line.text.length)) });
      typing = !!line.typed;
      budget = 0;
    }
  }
  return { lines, typing };
}
