import { onThemeChange } from '@/lib/landing/palette';

/** a CSS colour as 0-1 channels (#rrggbb, rgb(), or anything the canvas resolves) */
let probe: CanvasRenderingContext2D | null = null;
export function cssRgb(color: string): [number, number, number] {
  probe ??= document.createElement('canvas').getContext('2d', { willReadFrequently: true })!;
  probe.clearRect(0, 0, 1, 1);
  probe.fillStyle = '#000';
  probe.fillStyle = color.trim();
  probe.fillRect(0, 0, 1, 1);
  const d = probe.getImageData(0, 0, 1, 1).data;
  return [d[0] / 255, d[1] / 255, d[2] / 255];
}

export const isDark = () => document.documentElement.classList.contains('dark');
export const onTheme = onThemeChange;
