import { Waypoints } from "lucide-react";
import { cn } from "@/lib/utils";

// Brand mark: the same Waypoints glyph the dashboard and GUI use, so the logo is
// consistent across the product.
export function Logo({ className, withWordmark = true }: { className?: string; withWordmark?: boolean }) {
  return (
    <span className={cn("inline-flex items-center gap-2 font-semibold tracking-tight", className)}>
      <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
        <Waypoints className="h-4 w-4" />
      </span>
      {withWordmark && <span className="text-lg">Rift</span>}
    </span>
  );
}
