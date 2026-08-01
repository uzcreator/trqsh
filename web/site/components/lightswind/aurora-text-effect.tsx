import { cn } from "@/lib/utils";

export interface AuroraTextEffectProps {
  children: React.ReactNode;
  className?: string;
  /** Gradient stops the aurora flows through — CSS color strings. */
  colors?: string[];
  /** One full pan cycle, in seconds. */
  speed?: number;
  /** Soft blurred duplicate behind the text, in the same colors. */
  glow?: boolean;
}

const DEFAULT_COLORS = [
  "rgb(var(--brand))",
  "rgb(var(--brand-3))",
  "rgb(var(--glow))",
  "rgb(var(--brand-2))",
  "rgb(var(--brand-3))",
  "rgb(var(--brand))",
];

// Flowing multi-stop gradient text, panned slowly across a wide background —
// "aurora" in the northern-lights sense, not a single static gradient. A
// blurred duplicate of the same text sits behind it for the glow. Server-
// renderable (no state/effects), so it composes fine inside client components
// that reveal the text themselves, e.g. a typewriter — see
// components/hero-headline.tsx, which wraps its (already-clipped) text in
// this. Respects prefers-reduced-motion via the site-wide rule in globals.css.
export function AuroraTextEffect({
  children,
  className,
  colors = DEFAULT_COLORS,
  speed = 6,
  glow = true,
}: AuroraTextEffectProps) {
  const style: React.CSSProperties = {
    backgroundImage: `linear-gradient(115deg, ${colors.join(", ")})`,
    backgroundSize: "300% 100%",
    animationDuration: `${speed}s`,
  };

  return (
    <span className={cn("aurora-text", className)} style={style}>
      {glow && (
        <span className="aurora-text-glow" style={style} aria-hidden="true">
          {children}
        </span>
      )}
      {children}
    </span>
  );
}
