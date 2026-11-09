/** Tiny pixel glyph made of rects inside an SVG (rows of '#' and '.'). */
export function Pixels({
  rows,
  x = 0,
  y = 0,
  s = 1,
}: {
  rows: readonly string[];
  x?: number;
  y?: number;
  s?: number;
}) {
  return rows.flatMap((row, r) =>
    [...row].map((ch, c) =>
      ch === "#" ? (
        <rect
          key={`${r}-${c}`}
          x={x + c * s}
          y={y + r * s}
          width={s}
          height={s}
          fill="currentColor"
          stroke="none"
        />
      ) : null,
    ),
  );
}
