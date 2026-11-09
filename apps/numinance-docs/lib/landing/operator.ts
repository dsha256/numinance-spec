import type { Palette } from './palette';

/**
 * The Numinance operator: a tiny original pixel-art inspection robot. 11x14 cells.
 * k = outline (ink)  p = panel (paper)  y = visor signal (yellow)  i = chest (indigo)  o = beacon (orange)
 */
export const OPERATOR_W = 11;
export const OPERATOR_H = 14;

const HEAD = [
  '....oo.....',
  '.....k.....',
  '..kkkkkkk..',
  '.kpppppppk.',
  '.kpkkkkkpk.',
  '.kpkykykpk.',
  '.kpkkkkkpk.',
  '.kpppppppk.',
  '..kkkkkkk..',
];
const HEAD_BLINK = [...HEAD.slice(0, 5), '.kpkkkkkpk.', ...HEAD.slice(6)];
const HEAD_BEACON_OFF = ['.....k.....', ...HEAD.slice(1)];
const BODY = ['kkppiiippkk', 'kkppipippkk', '.kpppppppk.'];
const LEGS_STAND = ['..kk...kk..', '.kkk...kkk.'];
const LEGS_STEP_A = ['...kk.kk...', '..kkk.kkk..'];
const LEGS_STEP_B = ['..kk...kk..', '..kk...kk..'];

export const OPERATOR_FRAMES = {
  idle0: [...HEAD, ...BODY, ...LEGS_STAND],
  idle1: [...HEAD_BEACON_OFF.map((r, i) => (i === 0 ? '...........' : r)), ...BODY, ...LEGS_STAND].slice(0, 14),
  blink: [...HEAD_BLINK, ...BODY, ...LEGS_STAND],
  walkA: [...HEAD, ...BODY, ...LEGS_STEP_A],
  walkB: [...HEAD, ...BODY, ...LEGS_STEP_B],
} as const;
export type OperatorFrame = keyof typeof OPERATOR_FRAMES;

for (const [name, rows] of Object.entries(OPERATOR_FRAMES)) {
  if (rows.length !== OPERATOR_H || rows.some((r) => r.length !== OPERATOR_W))
    throw new Error(`operator frame ${name} is ${rows.length} rows / bad width`);
}

/** Draw the operator with its top-left at (x, y); `cell` is the pixel size. Crisp, no smoothing. */
export function drawOperator(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  cell: number,
  frame: OperatorFrame,
  p: Pick<Palette, 'ink' | 'paper' | 'yellow' | 'indigo' | 'orange'>,
  flip = false,
) {
  const col = { k: p.ink, p: p.paper, y: p.yellow, i: p.indigo, o: p.orange } as const;
  const rows = OPERATOR_FRAMES[frame];
  for (let r = 0; r < rows.length; r++)
    for (let c = 0; c < OPERATOR_W; c++) {
      const ch = rows[r][flip ? OPERATOR_W - 1 - c : c] as keyof typeof col | '.';
      if (ch === '.') continue;
      ctx.fillStyle = col[ch];
      ctx.fillRect(Math.round(x + c * cell), Math.round(y + r * cell), Math.ceil(cell), Math.ceil(cell));
    }
}

/** The official symbol as a 6x6 pixel glyph, for tiny pixel-art contexts (matches the real geometry). */
export const SYMBOL_PIXELS = ['##.###', '##.###', '##.###', '##....', '######', '######'] as const;
