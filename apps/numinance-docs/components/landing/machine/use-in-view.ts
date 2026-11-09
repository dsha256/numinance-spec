"use client";

import { useEffect, useRef, type RefObject } from "react";

/**
 * Calls `onEnter` every time the element crosses `threshold` into view (and `onLeave` when it leaves).
 * The latest callbacks are always used, so they may close over state that changes after mount
 * (e.g. the reduced-motion flag, which only becomes known after hydration).
 */
export function useInView(
  ref: RefObject<Element | null>,
  onEnter: () => void,
  onLeave?: () => void,
  threshold = 0.35,
) {
  const enter = useRef(onEnter);
  const leave = useRef(onLeave);
  enter.current = onEnter;
  leave.current = onLeave;

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const io = new IntersectionObserver(
      ([e]) => (e.isIntersecting ? enter.current() : leave.current?.()),
      { threshold },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [ref, threshold]);
}
