import { cn } from "@/lib/utils";

/** Centered empty-state placeholder with an icon, message, and optional action. */
export function Empty({
  icon: Icon,
  title,
  hint,
  action,
  className,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  hint?: string;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-1 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border-strong bg-surface/50 p-10 text-center",
        className,
      )}
    >
      <div className="flex size-11 items-center justify-center rounded-full bg-accent">
        <Icon className="size-5 text-muted" />
      </div>
      <div className="flex flex-col gap-1">
        <p className="text-sm font-medium text-foreground">{title}</p>
        {hint && <p className="max-w-xs text-xs text-muted">{hint}</p>}
      </div>
      {action}
    </div>
  );
}
