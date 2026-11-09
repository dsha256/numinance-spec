'use client';
import type { ReactNode } from 'react';
import { motion } from 'motion/react';
import { useReducedMotion } from '@/lib/landing/use-reduced';

/** opacity + small rise, whole blocks (never per letter), on the storyboard's 300-800ms beat */
export function Reveal({ children, delay = 0.3, className }: { children: ReactNode; delay?: number; className?: string }) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, y: reduced ? 0 : 14 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: reduced ? 0 : 0.5, delay: reduced ? 0 : delay, ease: [0.2, 0.7, 0.2, 1] }}
    >
      {children}
    </motion.div>
  );
}
