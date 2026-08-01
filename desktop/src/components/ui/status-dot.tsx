import { cn } from "@/lib/utils";

type DotStatus = "online" | "connecting" | "error" | "offline";

const tone: Record<DotStatus, string> = {
  online: "bg-good",
  connecting: "bg-warning",
  error: "bg-critical",
  offline: "bg-muted",
};

/** Small state indicator. "online" radiates two staggered signal rings — the
 *  app's signature motif for "this is a live connection, right now" (see
 *  .signal-ring in index.css). "connecting" gets a plainer single ping;
 *  offline/error are static, nothing to signal. */
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
        <>
          <span className="signal-ring absolute inline-flex size-full rounded-full bg-good" />
          <span className="signal-ring-delayed absolute inline-flex size-full rounded-full bg-good" />
        </>
      )}
      <span
        className={cn(
          "relative inline-flex size-2.5 rounded-full",
          tone[status],
          status === "online" && "shadow-[0_0_6px_rgb(var(--good)/0.8)]",
        )}
      />
    </span>
  );
}
