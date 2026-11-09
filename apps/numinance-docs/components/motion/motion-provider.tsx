'use client';

import { MotionConfig } from 'motion/react';

/** Respect the OS reduced-motion setting everywhere motion is used. */
export function MotionProvider({ children }: { children: React.ReactNode }) {
  return <MotionConfig reducedMotion="user">{children}</MotionConfig>;
}
