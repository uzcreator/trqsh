import { cn } from "@/lib/utils";

export interface BorderBeamProps {
  className?: string;
  /** Diameter of the moving beam segment, in px. */
  size?: number;
  /** One full trip around the border, in seconds. */
  duration?: number;
  /** Stagger the start, in seconds — offsets a second beam from a first. */
  delay?: number;
  colorFrom?: string;
  colorTo?: string;
  /** Inset of the beam's own track from the parent's edge, in px — should
   *  roughly match the parent's border width so the beam rides on top of it. */
  borderWidth?: number;
}

// A bright segment that continuously travels around its parent's border via
// CSS motion path (offset-path: border-box) — the parent's *real* border-box
// edge, rounded corners included, no shape math needed. Drop it in as the
// last child of a `position: relative` element with its own border-radius;
// this reads `rounded-[inherit]`. Pure CSS, no JS, degrades to a static glow
// blob if offset-path isn't supported.
export function BorderBeam({
  className,
  size = 200,
  duration = 8,
  delay = 0,
  colorFrom = "rgb(var(--brand))",
  colorTo = "rgb(var(--glow))",
  borderWidth = 2,
}: BorderBeamProps) {
  return (
    <div
      className="pointer-events-none absolute inset-0 overflow-hidden rounded-[inherit]"
      style={{ padding: borderWidth }}
      aria-hidden
    >
      <div
        className={cn("border-beam-segment", className)}
        style={{
          width: size,
          height: size,
          background: `linear-gradient(to left, ${colorFrom}, ${colorTo}, transparent)`,
          animationDuration: `${duration}s`,
          animationDelay: `${delay}s`,
        }}
      />
    </div>
  );
}
