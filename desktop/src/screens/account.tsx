import { useCallback, useEffect, useState } from "react";
import { ArrowUpRight, Check, RefreshCw, Sparkles } from "lucide-react";
import { agent, cloud, host, type HostInfo } from "@/lib/agent";
import { UPGRADE_URL } from "@/lib/links";
import { bytes, count } from "@/lib/format";
import type { Me, Status, Tunnel } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Meter } from "@/components/ui/meter";
import { Stat } from "@/components/stat";

const GB = 1024 ** 3;

/** Fallback monthly bandwidth allowance per plan (bytes), used only if the cloud
 *  limits haven't loaded yet. Real numbers come from GET /cloud/me. */
const PLAN_LIMITS: Record<string, number> = {
  free: 10 * GB,
  pro: 200 * GB,
  team: 1024 * GB,
};

const PRO_PERKS = [
  "10 concurrent tunnels & reserved subdomains",
  "Custom domains with automatic TLS",
  "200 GB/mo bandwidth, higher rate limits",
  "TCP, UDP & TLS tunnels",
];

const planTone = (plan: string) =>
  plan === "free" ? "neutral" : plan === "pro" ? "primary" : "good";

function fmtDate(d: Date): string {
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function Account({ status, tunnels }: { status: Status; tunnels: Tunnel[] }) {
  const [env, setEnv] = useState<HostInfo | null>(null);
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    host.env().then(setEnv).catch(() => {});
  }, []);

  const load = useCallback(() => {
    setLoading(true);
    cloud
      .me()
      .then(setMe)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);
  useEffect(() => load(), [load]);

  const dashboard = env?.dashboard_url ?? "https://app.trqsh.uz";

  // Live session totals across active tunnels (secondary, informational).
  const session = tunnels.reduce(
    (acc, t) => ({
      requests: acc.requests + t.metrics.requests,
      bytes: acc.bytes + t.metrics.bytes_in + t.metrics.bytes_out,
    }),
    { requests: 0, bytes: 0 },
  );

  const plan = (me?.plan || status.plan || "free").toLowerCase();
  const isFree = plan === "free";
  const limitBytes = me?.limits.bandwidth_bytes_mo || PLAN_LIMITS[plan] || PLAN_LIMITS.free;
  const usedBytes = me ? me.usage.bytes_in + me.usage.bytes_out : session.bytes;
  const monthRequests = me ? me.usage.requests : session.requests;

  const expiresAt = me?.plan_expires_at ? new Date(me.plan_expires_at) : null;
  const expired = !!expiresAt && expiresAt.getTime() < Date.now();
  const planNote = !expiresAt
    ? isFree
      ? "Free plan"
      : "Active"
    : expired
      ? `Expired ${fmtDate(expiresAt)}`
      : `Active until ${fmtDate(expiresAt)}`;

  const user = me?.user;
  const displayName = user?.name || user?.email || status.account_id || "Your account";
  const initial = (user?.name || user?.email || "?").trim().charAt(0).toUpperCase();

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <div className="flex items-center justify-between">
        <h1 className="text-base font-semibold">Account</h1>
        <Button variant="ghost" size="sm" onClick={load} disabled={loading} title="Refresh">
          <RefreshCw className={loading ? "animate-spin" : ""} />
        </Button>
      </div>

      {/* Profile */}
      <Card>
        <CardContent className="flex items-center gap-4">
          {user?.avatar_url ? (
            <img
              src={user.avatar_url}
              alt=""
              referrerPolicy="no-referrer"
              className="size-12 shrink-0 rounded-full ring-1 ring-border"
            />
          ) : (
            <div className="flex size-12 shrink-0 items-center justify-center rounded-full bg-primary/10 text-lg font-semibold text-primary ring-1 ring-primary/20">
              {initial}
            </div>
          )}
          <div className="min-w-0 flex-1">
            <p className="truncate font-medium">{displayName}</p>
            {user?.email && <p className="truncate text-sm text-muted">{user.email}</p>}
          </div>
          {user?.oauth_provider && (
            <Badge tone="neutral" className="capitalize">
              {user.oauth_provider}
            </Badge>
          )}
        </CardContent>
      </Card>

      {/* Plan */}
      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle>Plan</CardTitle>
          <Badge tone={planTone(plan)} className="uppercase">
            {plan}
          </Badge>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid grid-cols-3 gap-2">
            <Stat label="Status" value={<span className={expired ? "text-critical" : ""}>{planNote}</span>} />
            <Stat label="Account" value={<span className="font-mono text-xs">{me?.org.id || status.account_id || "—"}</span>} />
            <Stat label="Transport" value={<span className="uppercase text-xs">{status.kind}</span>} />
          </div>
        </CardContent>
      </Card>

      {/* Usage */}
      <Card>
        <CardHeader>
          <CardTitle>Usage this month</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid grid-cols-3 gap-2">
            <Stat label="Requests" value={<span className="text-wire">{count(monthRequests)}</span>} />
            <Stat label="Used" value={<span className="text-wire">{bytes(usedBytes)}</span>} />
            <Stat label="Session" value={<span className="text-wire">{bytes(session.bytes)}</span>} />
          </div>
          <Meter
            value={usedBytes}
            max={limitBytes}
            label="Bandwidth"
            sublabel={`${bytes(usedBytes)} of ${bytes(limitBytes)} this month`}
          />
          {!me && !loading && (
            <p className="text-xs text-muted">
              Couldn&apos;t reach the control plane — showing this session&apos;s totals. Check your
              connection and refresh.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Upgrade / manage */}
      {isFree ? (
        <Card className="border-primary/30 bg-primary/5">
          <CardHeader className="flex-row items-center gap-2">
            <Sparkles className="size-4 text-primary" />
            <CardTitle>Upgrade to Pro</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <ul className="flex flex-col gap-1.5">
              {PRO_PERKS.map((p) => (
                <li key={p} className="flex items-center gap-2 text-sm text-secondary">
                  <Check className="size-3.5 shrink-0 text-good" />
                  {p}
                </li>
              ))}
            </ul>
            <Button onClick={() => agent.openURL(UPGRADE_URL)} className="self-start">
              <ArrowUpRight />
              Upgrade
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => agent.openURL(UPGRADE_URL)}>
            <ArrowUpRight />
            Change plan
          </Button>
          <Button variant="ghost" onClick={() => agent.openURL(dashboard)}>
            Open dashboard
          </Button>
        </div>
      )}
    </div>
  );
}
