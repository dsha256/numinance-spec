"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";
import { useReducedMotion } from '@/lib/landing/use-reduced';
import {
  OPERATOR_H,
  OPERATOR_W,
  drawOperator,
  type OperatorFrame,
} from "@/lib/landing/operator";
import { onThemeChange, readPalette } from "@/lib/landing/palette";
import "./machine.css";

export type OperatorPose = "idle" | "walk" | "still";

/** The Numinance operator on a tiny canvas; walk/idle frames cycle on a timer (static when motion is reduced). */
export function OperatorSprite({
  pose = "idle",
  cell = 3,
  flip = false,
  className,
}: {
  pose?: OperatorPose;
  cell?: number;
  flip?: boolean;
  className?: string;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  const reduced = useReducedMotion() === true;
  const [tick, setTick] = useState(0);
  const [theme, setTheme] = useState(0);

  useEffect(() => onThemeChange(() => setTheme((t) => t + 1)), []);
  useEffect(() => {
    if (reduced || pose === "still") return;
    const id = setInterval(
      () => setTick((t) => t + 1),
      pose === "walk" ? 170 : 800,
    );
    return () => clearInterval(id);
  }, [pose, reduced]);

  const frame: OperatorFrame =
    reduced || pose === "still"
      ? "idle0"
      : pose === "walk"
        ? tick % 2
          ? "walkB"
          : "walkA"
        : tick % 5 === 4
          ? "blink"
          : tick % 2
            ? "idle1"
            : "idle0";

  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    const ctx = c.getContext("2d")!;
    ctx.clearRect(0, 0, c.width, c.height);
    drawOperator(ctx, 0, 0, cell, frame, readPalette(), flip);
  }, [frame, cell, flip, theme]);

  return (
    <canvas
      ref={ref}
      aria-hidden
      width={OPERATOR_W * cell}
      height={OPERATOR_H * cell}
      className={className}
      style={
        {
          width: OPERATOR_W * cell,
          height: OPERATOR_H * cell,
          imageRendering: "pixelated",
        } as CSSProperties
      }
    />
  );
}

/**
 * The operator riding a rail: positioned by CSS from `--i` (index) and `--n` (count), walking while the index changes.
 * Pair with `.mx-md` / `.mx-lg` to switch from a vertical to a horizontal rail.
 */
export function OperatorRider({
  index,
  count,
  bp,
  ry,
  cell = 3,
  hold,
}: {
  index: number;
  count: number;
  bp: "md" | "lg";
  ry: string;
  cell?: number;
  /** Force the idle pose (e.g. at the end of the line). */
  hold?: boolean;
}) {
  const [walking, setWalking] = useState(false);
  const prev = useRef(index);
  useEffect(() => {
    if (prev.current === index) return;
    prev.current = index;
    setWalking(true);
    const id = setTimeout(() => setWalking(false), 520);
    return () => clearTimeout(id);
  }, [index]);
  return (
    <div
      aria-hidden
      className={`mx-rider ${bp === "md" ? "mx-md" : "mx-lg"}`}
      style={{ "--i": index, "--n": count, "--ry": ry } as CSSProperties}
    >
      <OperatorSprite
        pose={walking && !hold ? "walk" : "idle"}
        cell={cell}
        flip={false}
      />
    </div>
  );
}
