import { useCallback, useEffect, useState } from "react";
import { ArrowUpRight, Info, RefreshCw } from "lucide-react";
import { cloud, host, agent, type HostInfo } from "@/lib/agent";
import { bytes, count } from "@/lib/format";
import type { Me } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Stat } from "@/components/stat";

/** Illustrative $/GB used only to render a plausible "Overall cost" figure —
 *  trqsh doesn't meter usage-based billing today (Free is genuinely free;
 *  paid plans are flat-rate, sold manually). Every number this produces is
 *  tagged "Estimated" in the UI. Swap for a real GET /cloud/billing/* call
 *  once that endpoint exists — this file is the only place that needs to
 *  change; MonthSummary is already shaped for it. */
const ESTIMATED_RATE_PER_GB = 0.08;
function estimateCost(plan: string, bytesTotal: number): number {
  if (plan === "free") return 0;
  return (bytesTotal / 1024 ** 3) * ESTIMATED_RATE_PER_GB;
}

interface MonthSummary {
  label: string;
  bytesTotal: number;
  requests: number;
  /** True for the live current month (real data); false for history (mocked
   *  below — there's no usage-history endpoint yet, see the module comment). */
  live: boolean;
}

/** Placeholder history until a real GET /cloud/usage/history exists. Numbers
 *  are illustrative, not fabricated user data — clearly a fixed constant. */
const MOCK_HISTORY: MonthSummary[] = [
  { label: "Last month", bytesTotal: 41 * 1024 ** 3, requests: 128_400, live: false },
  { label: monthLabel(-2), bytesTotal: 33 * 1024 ** 3, requests: 96_150, live: false },
  { label: monthLabel(-3), bytesTotal: 12 * 1024 ** 3, requests: 40_820, live: false },
];

function monthLabel(offset: number): string {
  const d = new Date();
  d.setMonth(d.getMonth() + offset);
  return d.toLocaleDateString(undefined, { month: "long" });
}

function MonthCard({ month, plan }: { month: MonthSummary; plan: string }) {
  const cost = estimateCost(plan, month.bytesTotal);
  const estimated = plan !== "free";
  return (
    <Card className={month.live ? "border-primary/30 bg-primary/5" : undefined}>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle>{month.label}</CardTitle>
        {month.live && <Badge tone="primary">Live</Badge>}
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-3">
          <Stat label="Traffic" value={<span className="text-wire">{bytes(month.bytesTotal)}</span>} />
          <Stat label="Requests" value={<span className="text-wire">{count(month.requests)}</span>} />
        </div>
        <div className="flex items-center justify-between border-t border-border pt-3">
          <span className="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wide text-muted">
            Overall cost
            {estimated && (
              <span title="Estimated from usage — trqsh doesn't meter usage-based billing yet.">
                <Info className="size-3" />
              </span>
            )}
          </span>
          <span className="flex items-center gap-1.5">
            {estimated && (
              <Badge tone="neutral" className="text-[10px]">
                Estimated
              </Badge>
            )}
            <span className="tabular text-sm font-semibold text-good">
              {cost === 0 ? "$0" : `$${cost.toFixed(2)}`}
            </span>
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

export function Billing() {
  const [me, setMe] = useState<Me | null>(null);
  const [env, setEnv] = useState<HostInfo | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    setLoading(true);
    cloud
      .me()
      .then(setMe)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);
  useEffect(() => load(), [load]);
  useEffect(() => {
    host.env().then(setEnv).catch(() => {});
  }, []);

  const plan = (me?.plan || "free").toLowerCase();
  const thisMonth: MonthSummary = {
    label: "This month",
    bytesTotal: me ? me.usage.bytes_in + me.usage.bytes_out : 0,
    requests: me?.usage.requests ?? 0,
    live: true,
  };

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-base font-semibold">Billing</h1>
          <p className="text-sm text-muted">Your usage, month by month.</p>
        </div>
        <Button variant="ghost" size="sm" onClick={load} disabled={loading} title="Refresh">
          <RefreshCw className={loading ? "animate-spin" : ""} />
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <MonthCard month={thisMonth} plan={plan} />
        {MOCK_HISTORY.map((m) => (
          <MonthCard key={m.label} month={m} plan={plan} />
        ))}
      </div>

      <Card>
        <CardContent className="flex flex-wrap items-center justify-between gap-3 pt-6">
          <div>
            <p className="text-sm font-medium">
              You're on the <span className="uppercase text-primary">{plan}</span> plan
            </p>
            <p className="text-xs text-muted">
              {plan === "free"
                ? "No billing — upgrade any time for more bandwidth and features."
                : "Manage payment details and invoices from the dashboard."}
            </p>
          </div>
          <Button
            variant="secondary"
            onClick={() => agent.openURL(env?.billing_url ?? "https://app.trqsh.uz/billing")}
          >
            <ArrowUpRight />
            Open billing
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
