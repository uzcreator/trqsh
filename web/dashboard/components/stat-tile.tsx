import * as React from "react";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";

/** A bare KPI tile: label + hero value (proportional figures, per dataviz). */
export function StatTile({
  label,
  value,
  sub,
  icon: Icon,
}: {
  label: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
}) {
  return (
    <Card className="card-hover p-5">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-muted">{label}</span>
        {Icon && <Icon className="h-4 w-4 text-muted" />}
      </div>
      <div className="mt-2 text-2xl font-semibold">{value}</div>
      {sub && <div className="mt-1 text-xs text-secondary">{sub}</div>}
    </Card>
  );
}

/** A KPI tile with a magnitude-toward-limit meter and an at-a-glance status. */
export function MeterTile({
  label,
  used,
  limitLabel,
  pct,
  note,
}: {
  label: string;
  used: React.ReactNode;
  limitLabel: string;
  pct: number;
  note?: React.ReactNode;
}) {
  const tone = pct >= 100 ? "critical" : pct >= 80 ? "warning" : "default";
  return (
    <Card className="card-hover p-5">
      <span className="text-xs font-medium uppercase tracking-wide text-muted">{label}</span>
      <div className="mt-2 flex items-baseline justify-between">
        <span className="text-2xl font-semibold">{used}</span>
        <span className="text-xs text-secondary tabular">of {limitLabel}</span>
      </div>
      <Progress value={pct} tone={tone} className="mt-3" />
      <div
        className={cn(
          "mt-2 text-xs",
          tone === "critical" ? "text-critical" : tone === "warning" ? "text-serious" : "text-secondary"
        )}
      >
        {note ?? `${pct}% used`}
      </div>
    </Card>
  );
}
