import { cn } from "@/lib/utils";

type DotStatus = "online" | "connecting" | "error" | "offline";

const tone: Record<DotStatus, string> = {
  online: "bg-good",
  connecting: "bg-warning",
  error: "bg-critical",
  offline: "bg-muted",
};

/** Small state indicator. Pulses while connecting. */
export function StatusDot({
  status,
  className,
}: {
  status: DotStatus;
  className?: string;
}) {
  return (
    <span className={cn("relative flex size-2.5", className)}>
      {status === "connecting" && (
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-warning opacity-60" />
      )}
      {status === "online" && (
        <span
          className="absolute inline-flex size-full animate-ping rounded-full bg-good opacity-40"
          style={{ animationDuration: "2.5s" }}
        />
      )}
      <span className={cn("relative inline-flex size-2.5 rounded-full", tone[status])} />
    </span>
  );
}
