import Link from "next/link";
import { TriangleAlert } from "lucide-react";
import { api, safe, type Plan } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { MeterTile, StatTile } from "@/components/stat-tile";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { HBar, Legend } from "@/components/hbar";
import { formatBytes, formatNumber, formatDate, pctOf } from "@/lib/format";

export default async function UsagePage() {
  const [usage, plans] = await Promise.all([safe(api.usage()), safe(api.plans())]);

  const bytesIn = usage?.usage.bytes_in ?? 0;
  const bytesOut = usage?.usage.bytes_out ?? 0;
  const bwUsed = bytesIn + bytesOut;
  const bwLimit = usage?.limits.bandwidth_bytes_mo ?? 0;
  const bwPct = pctOf(bwUsed, bwLimit);
  const requests = usage?.usage.requests ?? 0;
  const planCode = usage?.plan ?? "free";
  const plan: Plan | undefined = (plans ?? []).find((p) => p.code === planCode);

  return (
    <div>
      <PageHeader
        title="Usage"
        description={
          usage
            ? `Current period · since ${formatDate(usage.usage.window_start)}`
            : "Your monthly usage and limits."
        }
        action={
          planCode === "free" ? (
            <Link href="/billing">
              <Button>Upgrade</Button>
            </Link>
          ) : undefined
        }
      />

      {bwPct >= 80 && bwLimit > 0 && (
        <div className="mb-6 flex items-center gap-3 rounded-lg border border-warning/40 bg-warning/10 p-4">
          <TriangleAlert className="h-5 w-5 shrink-0 text-serious" />
          <div className="flex-1 text-sm">
            <span className="font-medium">You&apos;ve used {bwPct}% of your bandwidth.</span>{" "}
            <span className="text-secondary">Upgrade to avoid hitting the limit mid-month.</span>
          </div>
          <Link href="/billing">
            <Button size="sm">Upgrade</Button>
          </Link>
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <MeterTile
          label="Bandwidth"
          used={formatBytes(bwUsed)}
          limitLabel={bwLimit ? formatBytes(bwLimit) : "∞"}
          pct={bwPct}
          note={bwLimit ? `${bwPct}% of monthly allowance` : "Metered — billed by usage"}
        />
        <StatTile
          label="Requests"
          value={formatNumber(requests)}
          sub={usage?.limits.requests_mo ? `of ${formatNumber(usage.limits.requests_mo)}` : "Unlimited"}
        />
        <StatTile label="Data in / out" value={`${formatBytes(bytesIn)} / ${formatBytes(bytesOut)}`} sub="This period" />
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Bandwidth breakdown</CardTitle>
            <CardDescription>Inbound vs outbound traffic this period.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <HBar
              data={[
                { label: "In", value: bytesIn, display: formatBytes(bytesIn), colorVar: "rgb(var(--series-1))" },
                { label: "Out", value: bytesOut, display: formatBytes(bytesOut), colorVar: "rgb(var(--series-2))" },
              ]}
            />
            <Legend
              items={[
                { label: "Inbound", colorVar: "rgb(var(--series-1))" },
                { label: "Outbound", colorVar: "rgb(var(--series-2))" },
              ]}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Plan limits</CardTitle>
            <CardDescription>What the {plan?.name ?? planCode} plan includes.</CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-y-3 text-sm">
              <LimitRow label="Bandwidth / mo" value={bwLimit ? formatBytes(bwLimit) : "Metered"} />
              <LimitRow label="Concurrent tunnels" value={plan ? plan.max_concurrent_tunnels || "Metered" : "—"} />
              <LimitRow label="Reserved subdomains" value={plan ? plan.max_reserved_subdomains : "—"} />
              <LimitRow label="Custom domains" value={plan ? (plan.allow_custom_domains ? plan.max_custom_domains : "—") : "—"} />
              <LimitRow label="UDP / TLS" value={plan ? (plan.allow_udp ? "Included" : "—") : "—"} />
              <LimitRow label="Rate limit" value={plan ? (plan.rate_limit_rps ? `${plan.rate_limit_rps} rps` : "Unlimited") : "—"} />
            </dl>
          </CardContent>
        </Card>
      </div>

      <p className="mt-4 text-xs text-muted">
        Usage is aggregated from edge metering for the current billing period. Per-day time series will
        appear once the control API exposes a windowed usage endpoint.
      </p>
    </div>
  );
}

function LimitRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="mt-0.5 font-medium">{value}</dd>
    </div>
  );
}
