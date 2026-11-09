/** 4x4 Bayer matrix, values 0..15. */
export const BAYER4 = [
  [0, 8, 2, 10],
  [12, 4, 14, 6],
  [3, 11, 1, 9],
  [15, 7, 13, 5],
] as const;

/** Threshold in 0..1 for pixel (x, y); light > threshold ⇒ ink. */
export const bayer = (x: number, y: number) => (BAYER4[y & 3][x & 3] + 0.5) / 16;

/**
 * CSS mask-image data URIs: DITHER_MASKS[n] shows n/16 of each 4x4 cell (n = 0..16), used for
 * dithered shadows and the theme view-transition. Use with `mask-size: 4px 4px`.
 */
export const DITHER_MASKS: readonly string[] = Array.from({ length: 17 }, (_, n) => {
  const cells: string[] = [];
  for (let y = 0; y < 4; y++)
    for (let x = 0; x < 4; x++) if (BAYER4[y][x] < n) cells.push(`M${x} ${y}h1v1h-1z`);
  const svg = `<svg xmlns='http://www.w3.org/2000/svg' width='4' height='4' shape-rendering='crispEdges'><path d='${cells.join('')}'/></svg>`;
  return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
});
