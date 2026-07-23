import { useEffect, useRef, useState } from "react";
import { ExternalLink, Globe, Plus, Square } from "lucide-react";
import { agent } from "@/lib/agent";
import { bytes, count } from "@/lib/format";
import { friendlyError } from "@/lib/errors";
import type { Tunnel } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useNow, useWindowWidth } from "@/lib/hooks";
import { tunnelCurl } from "@/lib/curl";
import { Sparkline } from "@/components/sparkline";
import { useToast } from "@/components/ui/toast";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { StatusDot } from "@/components/ui/status-dot";
import { Spinner } from "@/components/ui/spinner";
import { CopyButton } from "@/components/copy-button";
import { Stat } from "@/components/stat";
import { Empty } from "@/components/empty";

const dotFor = (s: string): "online" | "connecting" | "error" | "offline" =>
  s === "online" ? "online" : s === "connecting" ? "connecting" : s === "error" ? "error" : "offline";

const isWebProto = (p: string) => p === "http" || p === "https" || p === "tls";

function fmtUptime(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${pad(m)}:${pad(sec)}`;
}

function TunnelCard({ t, onChanged }: { t: Tunnel; onChanged: () => void }) {
  const toast = useToast();
  const [stopping, setStopping] = useState(false);
  const [firstSeen] = useState(() => Date.now());
  const [spark, setSpark] = useState<number[]>([]);
  const [rate, setRate] = useState(0);
  const now = useNow(1000);
  const lastReq = useRef(t.metrics.requests);
  const lastSec = useRef(Math.floor(now / 1000));
  const web = isWebProto(t.proto);

  // Sample request rate once per wall-clock second into a 30-point buffer.
  useEffect(() => {
    const sec = Math.floor(now / 1000);
    if (sec === lastSec.current) return;
    lastSec.current = sec;
    const delta = Math.max(0, t.metrics.requests - lastReq.current);
    lastReq.current = t.metrics.requests;
    setRate(delta);
    setSpark((prev) => [...prev, delta].slice(-30));
  }, [now, t.metrics.requests]);

  const stop = async () => {
    setStopping(true);
    try {
      await agent.stopTunnel(t.id);
      onChanged();
    } catch (e) {
      toast.error("Couldn't stop tunnel", { description: friendlyError(e) });
    } finally {
      setStopping(false);
    }
  };

  return (
    <div className="card-hover flex flex-col rounded-lg border border-border bg-surface p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex items-center gap-2">
            <StatusDot status={dotFor(t.status)} />
            <span className="truncate text-sm font-semibold">{t.name}</span>
            <Badge tone="neutral" className="uppercase">
              {t.proto}
            </Badge>
            <span className="tabular text-xs text-muted">
              {t.status === "online" ? fmtUptime(now - firstSeen) : t.status}
            </span>
          </div>
          <div className="flex items-center gap-1.5">
            <a
              href={web ? t.public_url : undefined}
              onClick={(e) => {
                e.preventDefault();
                if (web) agent.openURL(t.public_url);
              }}
              className={cn(
                "selectable truncate font-mono text-sm text-primary",
                web && "hover:underline",
              )}
              title={t.public_url}
            >
              {t.public_url}
            </a>
            <CopyButton
              value={t.public_url}
              className="size-7"
              onCopied={() => toast.success("Public URL copied")}
            />
            {web && (
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
        <div className="flex shrink-0 flex-col items-end gap-2">
          <Button variant="secondary" size="sm" onClick={stop} disabled={stopping}>
            {stopping ? <Spinner /> : <Square className="size-3.5" />}
            Stop
          </Button>
          <CopyButton
            value={tunnelCurl(t)}
            label="cURL"
            variant="ghost"
            onCopied={() => toast.success("cURL command copied")}
          />
        </div>
      </div>

      {web && (
        <div className="mt-3 flex items-end gap-3">
          <div className="min-w-0 flex-1">
            <Sparkline data={spark} height={30} />
          </div>
          <span className="tabular shrink-0 text-[10px] text-muted">{rate}/s</span>
        </div>
      )}

      <div className="mt-3 grid grid-cols-4 gap-2 border-t border-border pt-3">
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
  onNew,
  onChanged,
}: {
  tunnels: Tunnel[];
  onNew: () => void;
  onChanged: () => void;
}) {
  const twoCol = useWindowWidth() >= 1040;

  const totals = tunnels.reduce(
    (acc, t) => ({
      requests: acc.requests + t.metrics.requests,
      bytesIn: acc.bytesIn + t.metrics.bytes_in,
      bytesOut: acc.bytesOut + t.metrics.bytes_out,
    }),
    { requests: 0, bytesIn: 0, bytesOut: 0 },
  );

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-base font-semibold">Tunnels</h1>
          <p className="text-xs text-muted">
            {tunnels.length === 0
              ? "No active tunnels"
              : `${tunnels.length} active ${tunnels.length === 1 ? "tunnel" : "tunnels"}`}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {tunnels.length > 0 && (
            <div className="hidden items-center gap-3 text-xs text-muted sm:flex">
              <span className="tabular">{count(totals.requests)} req</span>
              <span className="tabular">↓ {bytes(totals.bytesIn)}</span>
              <span className="tabular">↑ {bytes(totals.bytesOut)}</span>
            </div>
          )}
          <Button onClick={onNew}>
            <Plus />
            New tunnel
          </Button>
        </div>
      </div>

      {tunnels.length === 0 ? (
        <Empty
          icon={Globe}
          title="No tunnels running"
          hint="Start a tunnel to expose a local port to the public internet with a secure HTTPS URL."
          action={
            <Button variant="secondary" onClick={onNew}>
              <Plus />
              Start your first tunnel
            </Button>
          }
        />
      ) : (
        <div className={cn("grid gap-3", twoCol ? "grid-cols-2" : "grid-cols-1")}>
          {tunnels.map((t) => (
            <TunnelCard key={t.id} t={t} onChanged={onChanged} />
          ))}
        </div>
      )}
    </div>
  );
}
