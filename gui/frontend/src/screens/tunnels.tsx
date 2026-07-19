import { useState } from "react";
import { ExternalLink, Globe, Plus, Square } from "lucide-react";
import { agent } from "@/lib/agent";
import { bytes, count } from "@/lib/format";
import type { Tunnel } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { StatusDot } from "@/components/ui/status-dot";
import { Spinner } from "@/components/ui/spinner";
import { CopyButton } from "@/components/copy-button";
import { Stat } from "@/components/stat";
import { Empty } from "@/components/empty";
import { StartTunnelDialog } from "@/components/start-tunnel-dialog";

const dotFor = (s: string): "online" | "connecting" | "error" | "offline" =>
  s === "online" ? "online" : s === "connecting" ? "connecting" : s === "error" ? "error" : "offline";

function TunnelCard({ t, onChanged }: { t: Tunnel; onChanged: () => void }) {
  const [stopping, setStopping] = useState(false);
  const isWeb = t.proto === "http" || t.proto === "tls";

  const stop = async () => {
    setStopping(true);
    try {
      await agent.stopTunnel(t.id);
      onChanged();
    } finally {
      setStopping(false);
    }
  };

  return (
    <div className="card-hover rounded-lg border border-border bg-surface p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex items-center gap-2">
            <StatusDot status={dotFor(t.status)} />
            <span className="truncate text-sm font-semibold">{t.name}</span>
            <Badge tone="neutral" className="uppercase">
              {t.proto}
            </Badge>
          </div>
          <div className="flex items-center gap-1.5">
            <a
              href={isWeb ? t.public_url : undefined}
              onClick={(e) => {
                e.preventDefault();
                if (isWeb) agent.openURL(t.public_url);
              }}
              className="selectable truncate font-mono text-sm text-primary hover:underline"
              title={t.public_url}
            >
              {t.public_url}
            </a>
            <CopyButton value={t.public_url} className="size-7" />
            {isWeb && (
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={() => agent.openURL(t.public_url)}
                title="Open in browser"
              >
                <ExternalLink />
              </Button>
            )}
          </div>
          <p className="text-xs text-muted">
            → <span className="selectable font-mono">{t.local_addr}</span>
          </p>
        </div>
        <Button variant="secondary" size="sm" onClick={stop} disabled={stopping}>
          {stopping ? <Spinner /> : <Square className="size-3.5" />}
          Stop
        </Button>
      </div>

      <div className="mt-4 grid grid-cols-4 gap-2 border-t border-border pt-3">
        <Stat label="Requests" value={count(t.metrics.requests)} />
        <Stat label="Conns" value={count(t.metrics.connections)} />
        <Stat label="In" value={bytes(t.metrics.bytes_in)} />
        <Stat label="Out" value={bytes(t.metrics.bytes_out)} />
      </div>
    </div>
  );
}

export function Tunnels({
  tunnels,
  onChanged,
}: {
  tunnels: Tunnel[];
  onChanged: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-base font-semibold">Tunnels</h1>
          <p className="text-xs text-muted">
            {tunnels.length === 0
              ? "No active tunnels"
              : `${tunnels.length} active ${tunnels.length === 1 ? "tunnel" : "tunnels"}`}
          </p>
        </div>
        <Button onClick={() => setDialogOpen(true)}>
          <Plus />
          New tunnel
        </Button>
      </div>

      {tunnels.length === 0 ? (
        <Empty
          icon={Globe}
          title="No tunnels running"
          hint="Start a tunnel to expose a local port to the public internet with a secure HTTPS URL."
          action={
            <Button variant="secondary" onClick={() => setDialogOpen(true)}>
              <Plus />
              Start your first tunnel
            </Button>
          }
        />
      ) : (
        <div className="flex flex-col gap-3">
          {tunnels.map((t) => (
            <TunnelCard key={t.id} t={t} onChanged={onChanged} />
          ))}
        </div>
      )}

      <StartTunnelDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        onStarted={() => {
          setDialogOpen(false);
          onChanged();
        }}
      />
    </div>
  );
}
