'use client';

import { motion, type HTMLMotionProps } from 'motion/react';
import { EASE, dur } from '@/lib/motion';

type RevealProps = HTMLMotionProps<'div'> & { delay?: number; y?: number; once?: boolean };

/** Understated viewport reveal: opacity + 20px rise. Honors reduced motion via MotionConfig. */
export function Reveal({ delay = 0, y = 20, once = true, children, ...rest }: RevealProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once, margin: '0px 0px -12% 0px' }}
      transition={{ duration: dur.enter, ease: EASE, delay }}
      {...rest}
    >
      {children}
    </motion.div>
  );
}
