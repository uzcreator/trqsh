"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

// Pointer-driven 3D tilt. Sets --rx/--ry (and optional glare vars) on a `.tilt`
// element from the cursor position, so children with `.tilt-layer` (translateZ)
// parallax in real 3D. Disabled on coarse pointers (touch) and when the visitor
// prefers reduced motion — it then renders as a plain, static container.
export function Tilt({
  children,
  className,
  max = 9,
  glare = false,
}: {
  children: React.ReactNode;
  className?: string;
  /** Peak rotation in degrees at the edges. */
  max?: number;
  glare?: boolean;
}) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [enabled, setEnabled] = React.useState(false);

  React.useEffect(() => {
    const fine = window.matchMedia("(pointer: fine)").matches;
    const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    setEnabled(fine && !still);
  }, []);

  const onMove = React.useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      const el = ref.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const px = (e.clientX - r.left) / r.width;
      const py = (e.clientY - r.top) / r.height;
      el.style.setProperty("--ry", `${(px - 0.5) * max * 2}deg`);
      el.style.setProperty("--rx", `${-(py - 0.5) * max * 2}deg`);
      if (glare) {
        el.style.setProperty("--gx", `${px * 100}%`);
        el.style.setProperty("--gy", `${py * 100}%`);
        el.style.setProperty("--glare", "0.9");
      }
    },
    [max, glare]
  );

  const reset = React.useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.style.setProperty("--rx", "0deg");
    el.style.setProperty("--ry", "0deg");
    if (glare) el.style.setProperty("--glare", "0");
  }, [glare]);

  if (!enabled) {
    return <div className={className}>{children}</div>;
  }

  return (
    <div
      ref={ref}
      onPointerMove={onMove}
      onPointerLeave={reset}
      className={cn("tilt", className)}
    >
      {children}
      {glare && <span className="tilt-glare" aria-hidden />}
    </div>
  );
}
