/** Landing colours resolved from the live CSS variables, for canvas drawing. */
export interface Palette {
  paper: string;
  paper2: string;
  ink: string;
  ink2: string;
  line: string;
  lineStrong: string;
  orange: string;
  sun: string;
  yellow: string;
  indigo: string;
  dark: boolean;
}

export function readPalette(el: Element = document.documentElement): Palette {
  const s = getComputedStyle(el);
  const v = (n: string) => s.getPropertyValue(n).trim();
  return {
    paper: v('--lp-paper'),
    paper2: v('--lp-paper-2'),
    ink: v('--lp-ink'),
    ink2: v('--lp-ink-2'),
    line: v('--lp-line'),
    lineStrong: v('--lp-line-strong'),
    orange: v('--lp-orange'),
    sun: v('--lp-sun'),
    yellow: v('--lp-yellow'),
    indigo: v('--lp-indigo'),
    dark: document.documentElement.classList.contains('dark'),
  };
}

/** Calls `cb` whenever the theme class on <html> changes; returns its stop. */
export function onThemeChange(cb: () => void): () => void {
  const mo = new MutationObserver(cb);
  mo.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
  return () => mo.disconnect();
}

/** "#rrggbb" | "rgb(...)" → [r,g,b]. Computed custom properties come back as written. */
export function rgb(color: string): [number, number, number] {
  const c = document.createElement('canvas').getContext('2d')!;
  c.fillStyle = color;
  const out = c.fillStyle as string;
  if (out.startsWith('#')) {
    const n = parseInt(out.slice(1), 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
  }
  const m = out.match(/\d+(\.\d+)?/g)!.map(Number);
  return [m[0], m[1], m[2]];
}
