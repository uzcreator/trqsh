import * as React from "react";
import { cn } from "@/lib/utils";

// Scroll-stacking layout — successive panels are sticky and pile up over the one
// before, so each section literally rises over the previous as you scroll. Where
// the browser supports scroll-driven animations (`animation-timeline: view()`)
// the covered card also scales down and dims (see .stack-card in globals.css);
// elsewhere it degrades to a clean sticky stack. No JS, no dependencies.

export function ScrollStack({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={cn("relative", className)}>{children}</div>;
}

export function StackPanel({
  index,
  children,
  className,
}: {
  index: number;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className="stack-panel pb-8"
      style={{ ["--stack-top" as string]: `calc(7rem + ${index * 1.4}rem)` }}
    >
      <div
        className={cn(
          "stack-card border-gradient relative overflow-hidden rounded-2xl border border-border bg-surface/80 shadow-[0_30px_80px_-40px_rgb(0_0_0/0.9)] backdrop-blur-sm",
          className
        )}
      >
        {children}
      </div>
    </div>
  );
}
