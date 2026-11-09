'use client';

import { useRef } from 'react';
import { useScene, fitCanvas } from '@/lib/landing/use-scene';
import { onThemeChange, readPalette } from '@/lib/landing/palette';
import { drawOperator, OPERATOR_H, OPERATOR_W, type OperatorFrame } from '@/lib/landing/operator';

export type ClerkMode = 'idle' | 'read' | 'hold' | 'ok';

const CELL = 4;

/** The policy clerk: a tiny pixel operator who stands at the gate and reacts to its verdict. */
export function Clerk({ mode, className }: { mode: ClerkMode; className?: string }) {
  const modeRef = useRef(mode);
  modeRef.current = mode;
  const ref = useScene<HTMLDivElement>((el) => {
    const canvas = el.querySelector('canvas')!;
    const { ctx, w, h } = fitCanvas(canvas);
    let pal = readPalette();
    const stop = onThemeChange(() => (pal = readPalette()));
    const frame = (_dt: number, t: number) => {
      ctx.clearRect(0, 0, w, h);
      const m = modeRef.current;
      const ox = Math.round((w - OPERATOR_W * CELL) / 2);
      const oy = h - OPERATOR_H * CELL - 3;
      let f: OperatorFrame = Math.floor(t / 1400) % 2 === 0 ? (Math.floor(t / 90) % 14 === 0 ? 'blink' : 'idle0') : 'idle1';
      if (m === 'read') f = Math.floor(t / 220) % 2 ? 'walkA' : 'walkB';
      const bob = m === 'read' && Math.floor(t / 220) % 2 ? 0 : 0;
      drawOperator(ctx, ox, oy + bob, CELL, f, pal, true);
      // floor plate
      ctx.fillStyle = pal.lineStrong;
      ctx.fillRect(ox - 6, h - 3, OPERATOR_W * CELL + 12, 2);
      // thought pixel above the head
      const cx = ox + OPERATOR_W * CELL + 2;
      if (m === 'hold') {
        ctx.fillStyle = pal.orange;
        ctx.fillRect(cx, 6, 3, 9);
        ctx.fillRect(cx, 18, 3, 3);
      } else if (m === 'ok') {
        ctx.fillStyle = pal.indigo;
        for (const [x, y] of [[0, 9], [3, 12], [6, 9], [9, 6], [12, 3]]) ctx.fillRect(cx + x - 3, y, 3, 3);
      }
    };
    return { frame, cleanup: stop };
  });
  return (
    <div ref={ref} aria-hidden className={className}>
      <canvas className="block size-full" />
    </div>
  );
}
