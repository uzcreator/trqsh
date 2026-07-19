import { ArrowUpRight, Check, Sparkles } from "lucide-react";
import { agent } from "@/lib/agent";
import { bytes, count } from "@/lib/format";
import type { Status, Tunnel } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Stat } from "@/components/stat";

const DASHBOARD_BILLING_URL = "https://dashboard.rift.dev/billing";

const PRO_PERKS = [
  "Unlimited tunnels & reserved subdomains",
  "Custom domains with automatic TLS",
  "100 GB/mo bandwidth, higher rate limits",
  "TCP, UDP & TLS tunnels",
];

const planTone = (plan: string) =>
  plan === "free" ? "neutral" : plan === "pro" ? "primary" : "good";

export function Account({ status, tunnels }: { status: Status; tunnels: Tunnel[] }) {
  const totals = tunnels.reduce(
    (acc, t) => ({
      requests: acc.requests + t.metrics.requests,
      bytesIn: acc.bytesIn + t.metrics.bytes_in,
      bytesOut: acc.bytesOut + t.metrics.bytes_out,
    }),
    { requests: 0, bytesIn: 0, bytesOut: 0 },
  );
  const isFree = (status.plan || "free").toLowerCase() === "free";

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <h1 className="text-base font-semibold">Account</h1>

      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle>Plan</CardTitle>
          <Badge tone={planTone((status.plan || "free").toLowerCase())} className="uppercase">
            {status.plan || "Free"}
          </Badge>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid grid-cols-3 gap-2">
            <Stat label="Account" value={<span className="font-mono text-xs">{status.account_id || "—"}</span>} />
            <Stat label="Edge" value={<span className="text-xs">{status.edge || "—"}</span>} />
            <Stat label="Transport" value={<span className="uppercase text-xs">{status.kind}</span>} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Session usage</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-3 gap-2">
            <Stat label="Requests" value={count(totals.requests)} />
            <Stat label="Bytes in" value={bytes(totals.bytesIn)} />
            <Stat label="Bytes out" value={bytes(totals.bytesOut)} />
          </div>
          <p className="mt-3 text-xs text-muted">
            Live totals across active tunnels this session. Full billing history lives in the
            dashboard.
          </p>
        </CardContent>
      </Card>

      {isFree && (
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
            <Button onClick={() => agent.openURL(DASHBOARD_BILLING_URL)} className="self-start">
              <ArrowUpRight />
              Upgrade in dashboard
            </Button>
          </CardContent>
        </Card>
      )}

      {!isFree && (
        <Button
          variant="secondary"
          className="self-start"
          onClick={() => agent.openURL(DASHBOARD_BILLING_URL)}
        >
          <ArrowUpRight />
          Manage billing
        </Button>
      )}
    </div>
  );
}
