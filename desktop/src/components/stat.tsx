import { cn } from "@/lib/utils";

/** Compact labelled metric used in tunnel rows and the account summary. */
export function Stat({
  label,
  value,
  className,
}: {
  label: string;
  value: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-0.5", className)}>
      <span className="text-[10px] font-medium uppercase tracking-wide text-muted">
        {label}
      </span>
      <span className="tabular text-sm font-semibold text-foreground">{value}</span>
    </div>
  );
}
