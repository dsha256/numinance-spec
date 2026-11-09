import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { Shadow } from "@/components/landing/ui/window";

export const REPLAY_ICON =
  "M1 0h5v1H1zM0 1h1v5H0zM6 1h1v1H6zM5 2h3v1H5zM6 3h1v1H6zM1 6h5v1H1z";

/** Square pixel-type button with a dithered shadow; yellow on hover/focus. */
export function PixelButton({
  icon,
  children,
  className,
  ...props
}: ComponentProps<"button"> & { icon?: string }) {
  return (
    <span className="relative isolate inline-flex">
      <Shadow />
      <button
        type="button"
        {...props}
        className={cn(
          "relative inline-flex min-h-11 items-center gap-2 border border-lp-ink bg-lp-paper px-3 font-pixel text-[13px] leading-4 text-lp-ink antialiased transition-colors hover:bg-lp-yellow hover:text-lp-on-yellow focus-visible:bg-lp-yellow focus-visible:text-lp-on-yellow active:translate-x-px active:translate-y-px md:min-h-8",
          className,
        )}
      >
        {icon && (
          <svg
            aria-hidden
            viewBox="0 0 8 8"
            shapeRendering="crispEdges"
            className="size-2.5 fill-current"
          >
            <path d={icon} />
          </svg>
        )}
        {children}
      </button>
    </span>
  );
}
