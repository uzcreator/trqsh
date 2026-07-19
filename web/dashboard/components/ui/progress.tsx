import { cn } from "@/lib/utils";

/**
 * A magnitude-toward-cap meter (dataviz). Track is recessive; the fill uses the
 * sequential blue by default and reserves status colors for near/over limit.
 */
export function Progress({
  value,
  tone = "default",
  className,
}: {
  value: number;
  tone?: "default" | "warning" | "critical";
  className?: string;
}) {
  const v = Math.max(0, Math.min(100, Math.round(value)));
  const fill =
    tone === "critical" ? "bg-critical" : tone === "warning" ? "bg-warning" : "bg-series-1";
  return (
    <div
      role="progressbar"
      aria-valuenow={v}
      aria-valuemin={0}
      aria-valuemax={100}
      className={cn("h-2 w-full overflow-hidden rounded-full bg-black/5 dark:bg-white/10", className)}
    >
      <div className={cn("h-full rounded-full transition-all", fill)} style={{ width: `${v}%` }} />
    </div>
  );
}
