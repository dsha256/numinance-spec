'use client';

import { Fragment, type ReactNode } from 'react';
import { motion } from 'motion/react';
import { cn } from '@/lib/cn';
import { Chip } from '@/components/landing/ui/chip';
import { Shadow, Window } from '@/components/landing/ui/window';
import type { ResultRow, Tone } from './data';
import type { Derived } from './timeline';

/* ---------- hand-rolled highlighting (no deps) ---------- */

const C = {
  key: 'text-lp-orange',
  comment: 'text-lp-ink-2 italic',
  ph: 'text-lp-indigo',
  dim: 'text-lp-ink-2',
  str: 'text-lp-ink',
  pending: 'text-lp-ink-2 underline decoration-dotted underline-offset-4',
  ok: 'text-lp-ink font-medium',
  weak: 'text-lp-indigo',
} as const;

const TOKEN = /(<[^>]+>|…|caller must pin keys|evidence\/v1)/g;

function value(text: string, k: string): ReactNode[] {
  return text
    .split(TOKEN)
    .filter(Boolean)
    .map((t, i) => {
      const cls = t.startsWith('<') ? C.ph : t === '…' ? C.dim : t.startsWith('needs') ? C.weak : t === 'evidence/v1' ? C.str : '';
      return (
        <span key={`${k}${i}`} className={cls}>
          {t}
        </span>
      );
    });
}

/** `key: value`, `key   value` or `# comment`; `forceKey` makes the first word a key even without a colon */
function highlight(text: string, k: string, view: boolean): ReactNode {
  if (text.trimStart().startsWith('#')) return <span className={C.comment}>{text}</span>;
  if (text.trimStart().startsWith('<') || text.trimStart().startsWith('…')) return <>{value(text, k)}</>;
  const m = /^(\s*)([\w./-]+)(:?)(\s*)(.*)$/.exec(text);
  if (!m || (!m[3] && !view)) return <>{value(text, k)}</>;
  return (
    <>
      {m[1]}
      <span className={C.key}>{m[2]}</span>
      <span className={C.dim}>{m[3]}</span>
      {m[4]}
      {value(m[5], k)}
    </>
  );
}

const markerClass = (t: string) => (t === 'consistent' ? C.ok : t === 'unverified' ? C.pending : C.str);

/* ---------- editor ---------- */

const TONE_NOTE: Record<Tone, string> = {
  bad: 'text-lp-orange',
  ok: 'text-lp-ink-2',
  weak: 'text-lp-indigo',
  pending: 'text-lp-ink-2',
};

const MIN_LINES = 10;

export function Editor({
  d,
  rw,
  view,
  onView,
  caret,
}: {
  d: Derived;
  rw: number;
  view: number;
  onView: (i: number) => void;
  caret: boolean;
}) {
  const { w, landed } = d;
  const file = w.files[Math.min(view, w.files.length - 1)];
  const isEdit = view === w.edit.file;
  const e = w.edit;
  const crumbs = file.path;
  const rows = Math.max(MIN_LINES, file.lines.length);

  const editedLine = (lineText: string, key: string) => {
    const mid =
      d.edit === 'initial' || d.edit === 'flag' || d.edit === 'select' || d.edit === 'strike'
        ? e.from
        : d.edit === 'rewrite'
          ? e.to.slice(0, rw)
          : e.to;
    const midCls =
      d.edit === 'select'
        ? 'bg-lp-yellow text-lp-on-yellow'
        : d.edit === 'strike'
          ? 'bg-lp-yellow/40 text-lp-ink-2 line-through decoration-lp-orange decoration-2'
          : d.edit === 'flag'
            ? e.beforeTone === 'bad'
              ? 'text-lp-orange underline decoration-wavy decoration-lp-orange underline-offset-4'
              : 'outline-1 outline-dashed outline-lp-indigo outline-offset-2 ' + markerClass(e.from)
            : e.midKind === 'key'
              ? C.key
              : markerClass(mid);
    void lineText;
    return (
      <>
        {highlight(e.pre, key + 'p', true)}
        <span className={midCls}>{mid}</span>
        {d.edit === 'rewrite' && caret && <span aria-hidden className="ms-px inline-block h-[1.1em] w-[0.55em] translate-y-[0.2em] animate-lp-caret bg-lp-orange" />}
        <span className={C.dim}>{e.post}</span>
      </>
    );
  };

  return (
    <div className="relative isolate">
      <Shadow />
      <Window className="relative" title={<><span className="text-lp-ink-2">{w.root}/</span>{crumbs[crumbs.length - 1]}</>}>
        {/* tabs */}
        <div role="tablist" aria-label="Open files" className="flex h-9 shrink-0 items-end overflow-x-auto border-b border-lp-line-strong bg-lp-paper-2">
          {w.files.map((f, i) => {
            const here = landed > i;
            const modified = i === e.file && (d.edit === 'strike' || d.edit === 'rewrite' || d.edit === 'final');
            const name = f.path[f.path.length - 1];
            return (
              <motion.button
                key={w.id + name}
                type="button"
                role="tab"
                aria-selected={view === i}
                tabIndex={view === i ? 0 : -1}
                disabled={!here}
                onClick={() => onView(i)}
                initial={false}
                animate={{ opacity: here ? 1 : 0.0 }}
                transition={{ duration: 0.2 }}
                className={cn(
                  'relative flex h-full shrink-0 items-center gap-2 border-e border-lp-line-strong px-3 font-mono text-xs outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-lp-ink',
                  view === i ? 'bg-lp-paper text-lp-ink' : 'text-lp-ink-2 hover:text-lp-ink',
                  !here && 'pointer-events-none',
                )}
              >
                {view === i && <span aria-hidden className="absolute inset-x-0 top-0 h-0.5 bg-lp-orange" />}
                {name}
                {modified && <span aria-label="modified" className="size-1.5 rounded-full bg-lp-orange" />}
              </motion.button>
            );
          })}
        </div>
        {/* path */}
        <div className="flex h-7 shrink-0 items-center gap-1 overflow-hidden border-b border-lp-line px-3 font-mono text-[11px] whitespace-nowrap text-lp-ink-2">
          {crumbs.map((c, i) => (
            <Fragment key={c + i}>
              {i > 0 && <span aria-hidden>/</span>}
              <span className={i === crumbs.length - 1 ? 'text-lp-ink' : ''}>{c}</span>
            </Fragment>
          ))}
        </div>
        {/* code */}
        <div className="relative overflow-x-auto bg-lp-paper py-2" tabIndex={0} role="region" aria-label={`${crumbs[crumbs.length - 1]} (illustrative)`}>
          <div className="min-w-max font-mono text-[13px] leading-[22px]">
            {Array.from({ length: rows }, (_, i) => {
              const text = file.lines[i];
              const hasLine = text !== undefined && landed > view;
              const edited = isEdit && i === e.line && hasLine;
              const modified = edited && (d.edit === 'final' || d.edit === 'rewrite');
              const flagged = edited && d.edit !== 'initial' && d.edit !== 'final' && e.beforeTone === 'bad';
              const note = !hasLine
                ? null
                : edited
                  ? d.edit === 'flag' || d.edit === 'select' || d.edit === 'strike'
                    ? { t: e.before, tone: e.beforeTone }
                    : d.edit === 'final'
                      ? { t: e.after, tone: 'ok' as Tone }
                      : null
                  : (() => {
                      const n = w.notes.find((x) => x[0] === view && x[1] === i);
                      return n && d.stage === 2 ? { t: n[2], tone: 'weak' as Tone } : null;
                    })();
              return (
                <div
                  key={i}
                  className={cn(
                    'flex h-[22px] border-s-2',
                    modified ? 'border-lp-sun bg-lp-yellow/20' : flagged ? 'border-lp-orange bg-lp-orange/10' : 'border-transparent',
                  )}
                >
                  <span aria-hidden className="sticky left-0 z-1 w-10 shrink-0 bg-inherit pe-3 text-end text-lp-ink-2 select-none tabular-nums">
                    {text !== undefined ? i + 1 : ''}
                  </span>
                  <span className="pe-6 whitespace-pre">
                    {hasLine ? (edited ? editedLine(text, `e${i}`) : highlight(text, `l${i}`, w.id === 'verify')) : null}
                    {note && <span className={cn('ms-4 text-xs', TONE_NOTE[note.tone])}>{note.t}</span>}
                  </span>
                </div>
              );
            })}
          </div>
          {landed === 0 && (
            <p className="absolute inset-x-0 top-1/2 -translate-y-1/2 text-center font-pixel text-[13px] text-lp-ink-2">no files yet — waiting for the factory</p>
          )}
        </div>
        {/* result panel */}
        <Result d={d} />
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-lp-line-strong px-3 py-1.5">
          <span className="font-pixel text-[13px] leading-5 text-lp-orange">illustrative — not captured from a run</span>
          <Chip className="ms-auto h-5 px-1.5 text-[12px] text-lp-ink-2">{d.label}</Chip>
        </div>
      </Window>
    </div>
  );
}

function ToneMark({ tone }: { tone: Tone }) {
  return (
    <svg aria-hidden viewBox="0 0 10 10" shapeRendering="crispEdges" className="mt-[5px] size-2.5 shrink-0">
      {tone === 'ok' && <path d="M0 0h10v10H0z" className="fill-lp-sun" />}
      {tone === 'bad' && <path d="M0 0h10v10H0z M2 2h6v6H2z" fillRule="evenodd" className="fill-lp-orange" />}
      {tone === 'weak' && <path d="M0 0h10v10H0z M1 1h8v8H1z" fillRule="evenodd" strokeDasharray="2 2" className="fill-lp-indigo" />}
      {tone === 'pending' && <path d="M0 0h10v10H0z M1.5 1.5h7v7h-7z" fillRule="evenodd" className="fill-lp-ink-2" />}
    </svg>
  );
}

function Result({ d }: { d: Derived }) {
  const st = d.stage;
  const rows: readonly ResultRow[] = d.w.result;
  return (
    <div className="border-t border-lp-line-strong bg-lp-paper-2" aria-live="polite">
      <div className="flex h-7 items-center gap-2 border-b border-lp-line px-3">
        <span className="font-pixel text-[13px] leading-none">RESULT</span>
        <span className="font-mono text-[11px] text-lp-ink-2">{st === 0 ? 'waiting' : st === 1 ? 'first pass' : 'after the edit'}</span>
      </div>
      <div className="min-h-[9.25rem] px-3 py-2">
        {st === 0 ? (
          <p className="font-mono text-xs leading-6 text-lp-ink-2">{d.w.readout.run} …</p>
        ) : (
          <dl className="font-mono text-xs leading-[22px]">
            {rows.map((r) => {
              const i = st - 1;
              return (
                <div key={r.label} className="flex gap-2">
                  <ToneMark tone={r.tone[i]} />
                  <dt className="w-[8.5rem] shrink-0 text-lp-ink-2 sm:w-40">{r.label}</dt>
                  <motion.dd
                    key={r.value[i]}
                    initial={{ opacity: 0.2 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.3 }}
                    className={cn('min-w-0 break-words', r.tone[i] === 'bad' && 'text-lp-orange', r.tone[i] === 'weak' && 'text-lp-indigo', r.tone[i] === 'ok' && 'text-lp-ink')}
                  >
                    {r.value[i]}
                  </motion.dd>
                </div>
              );
            })}
          </dl>
        )}
        {st > 0 && <p className="mt-1.5 border-t border-dashed border-lp-line-strong pt-1.5 text-xs leading-5 text-lp-ink-2">{d.w.resultFoot}</p>}
      </div>
    </div>
  );
}
