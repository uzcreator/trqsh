import { cn } from "@/lib/utils";

// trqsh brand mark — a "tunnel portal": concentric rounded rings receding to a
// glowing green node, wrapped in a gradient-edged tile. Reads as looking down a
// tunnel toward the localhost you're exposing. Pure inline SVG (theme-aware via
// CSS vars); the mark animates when an ancestor `.group` is hovered.
export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        "relative inline-flex h-8 w-8 items-center justify-center transition-transform duration-500 group-hover:rotate-[8deg]",
        className
      )}
    >
      <svg viewBox="0 0 32 32" fill="none" className="h-full w-full" aria-hidden>
        <defs>
          <linearGradient id="trqsh-mark" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="rgb(var(--brand))" />
            <stop offset="0.5" stopColor="rgb(var(--brand-3))" />
            <stop offset="1" stopColor="rgb(var(--brand-2))" />
          </linearGradient>
        </defs>
        <rect
          x="1.1"
          y="1.1"
          width="29.8"
          height="29.8"
          rx="9"
          fill="rgb(var(--surface-2))"
          stroke="url(#trqsh-mark)"
          strokeWidth="1.5"
        />
        <rect x="6" y="6" width="20" height="20" rx="6.5" fill="none" stroke="rgb(var(--brand) / 0.35)" strokeWidth="1.5" />
        <rect x="10" y="10" width="12" height="12" rx="4" fill="none" stroke="rgb(var(--brand) / 0.7)" strokeWidth="1.5" />
        <circle
          cx="16"
          cy="16"
          r="2.7"
          fill="rgb(var(--brand))"
          className="origin-center transition-transform duration-500 group-hover:scale-125"
        />
      </svg>
      <span className="pointer-events-none absolute inset-0 rounded-[9px] opacity-0 shadow-[0_0_18px_2px_rgb(var(--brand)/0.55)] transition-opacity duration-500 group-hover:opacity-100" />
    </span>
  );
}

export function Logo({
  className,
  withWordmark = true,
}: {
  className?: string;
  withWordmark?: boolean;
}) {
  return (
    <span className={cn("group inline-flex items-center gap-2.5", className)}>
      <LogoMark />
      {withWordmark && (
        <span className="font-display text-[1.4rem] font-normal lowercase leading-none tracking-[-0.01em] text-foreground">
          trqsh
        </span>
      )}
    </span>
  );
}
