/** Numinance motion language — see master prompt §7. */
export const EASE = [0.22, 1, 0.36, 1] as const;

export const dur = {
  enter: 0.6,
  tab: 0.3,
  panel: 0.26,
  hover: 0.18,
} as const;

export const stagger = { card: 0.065, node: 0.085 } as const;
