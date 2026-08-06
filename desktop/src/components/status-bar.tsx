import type { Status } from "@/lib/types";
import { StatusDot } from "@/components/ui/status-dot";

/** Bottom-docked vitals strip, spanning the full window width (under the rail
 *  too) — the console's persistent "this is what's happening right now,"
 *  visible regardless of screen or panel state. Sole owner of the connection
 *  indicator (the titlebar's old pill duplicated this). */
export function StatusBar({
  status,
  tunnelCount,
  version,
}: {
  status: Status;
  tunnelCount: number;
  version: string;
}) {
  const dot = status.connected ? "online" : "offline";
  const text = status.connected ? status.edge || "Connected" : "Disconnected";

  return (
    <footer className="flex h-6 shrink-0 items-center gap-4 border-t border-border bg-surface-2 px-3 text-[11px] text-muted">
      <div className="flex min-w-0 items-center gap-1.5">
        <StatusDot status={dot} />
        <span className="truncate">{text}</span>
      </div>
      <span className="font-mono uppercase tracking-wide">{status.kind}</span>
      <span className="tabular font-mono">
        {tunnelCount} {tunnelCount === 1 ? "tunnel" : "tunnels"}
      </span>
      <div className="flex-1" />
      <span className="tabular font-mono">v{version}</span>
    </footer>
  );
}
