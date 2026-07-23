import { cn } from "@/lib/utils";

type Tone = "primary" | "good" | "warning" | "critical";

const barTone: Record<Tone, string> = {
  primary: "bg-primary",
  good: "bg-good",
  warning: "bg-warning",
  critical: "bg-critical",
};

/** A labelled usage meter (bandwidth, quota). Auto-escalates tone as it fills
 *  unless an explicit tone is given. */
export function Meter({
  value,
  max,
  label,
  sublabel,
  tone,
  className,
}: {
  value: number;
  max: number;
  label?: React.ReactNode;
  sublabel?: React.ReactNode;
  tone?: Tone;
  className?: string;
}) {
  const pct = max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0;
  const resolved: Tone = tone ?? (pct >= 90 ? "critical" : pct >= 75 ? "warning" : "primary");
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      {(label || sublabel) && (
        <div className="flex items-baseline justify-between gap-2">
          {label && <span className="text-xs font-medium text-secondary">{label}</span>}
          {sublabel && <span className="tabular text-xs text-muted">{sublabel}</span>}
        </div>
      )}
      <div className="h-2 w-full overflow-hidden rounded-full bg-border/60">
        <div
          className={cn("h-full rounded-full transition-[width] duration-500", barTone[resolved])}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
