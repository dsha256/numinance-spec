import { useSyncExternalStore } from 'react';

/**
 * The page's "drawing set" registry. Each <Figure> registers itself; the status bar lists them,
 * highlights the one crossing the middle of the viewport and prints that figure's status line.
 */
export interface FigureEntry {
  el: HTMLElement;
  id: string;
  label: string;
}

let status = 'ready';
let figures: readonly FigureEntry[] = [];
let active: HTMLElement | null = null;
const crossing = new Set<HTMLElement>();
let middle: IntersectionObserver | undefined;
const listeners = new Set<() => void>();
const none: readonly FigureEntry[] = [];

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}
const emit = () => listeners.forEach((l) => l());

/** Latest line a figure tried to print while it was not the active one (shown when it becomes active). */
const pending = new Map<string, string>();

function update() {
  const [first = null] = crossing;
  if (first !== active) {
    active = first;
    status = (first && pending.get(first.id)) || first?.dataset.status || 'ready';
  }
  emit();
}

function onMiddle(records: IntersectionObserverEntry[]) {
  for (const r of records) {
    const el = r.target as HTMLElement;
    if (r.isIntersecting) crossing.add(el);
    else crossing.delete(el);
  }
  update();
}

/** <Figure>'s ref callback. */
export function registerFigure(el: HTMLElement): () => void {
  middle ??= new IntersectionObserver(onMiddle, { rootMargin: '-50% 0px -50% 0px' });
  const entry = { el, id: el.id, label: el.dataset.chapter ?? el.getAttribute('aria-label') ?? el.id };
  figures = [...figures, entry].sort((a, b) =>
    a.el.compareDocumentPosition(b.el) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1,
  );
  middle.observe(el);
  emit();
  return () => {
    middle!.unobserve(el);
    crossing.delete(el);
    figures = figures.filter((f) => f.el !== el);
    update();
  };
}

/** A transient status line (e.g. a demo's current step) until the next one. */
export function setStatus(next: string) {
  // between figures (hero gap, footer) nothing is active: a transient line would be stale
  if (next === status || !active) return;
  status = next;
  emit();
}

/** A figure's own resting line; shown now if it is the active figure. */
export function report(from: Element, text: string) {
  const fig = from.closest<HTMLElement>('[data-figure]');
  if (!fig || fig.dataset.status === text) return;
  fig.dataset.status = text;
  if (fig === active) setStatus(text);
}

export function onStatus(listener: (text: string) => void): () => void {
  let last = status;
  listener(last);
  return subscribe(() => {
    if (status === last) return;
    last = status;
    listener(status);
  });
}

export function useFigures(): readonly FigureEntry[] {
  return useSyncExternalStore(subscribe, () => figures, () => none);
}
export function useActive(): string | null {
  return useSyncExternalStore(subscribe, () => active?.id ?? null, () => null);
}

/**
 * A status setter bound to one figure: lines only print while that figure is the active one, so a
 * timer still running in a figure the reader has scrolled past can never overwrite the current line.
 * The most recent suppressed line is kept and shown when the figure becomes active again.
 */
export function scopedStatus(figureId: string) {
  return (text: string) => {
    if (active?.id === figureId) {
      pending.delete(figureId);
      setStatus(text);
    } else pending.set(figureId, text);
  };
}
