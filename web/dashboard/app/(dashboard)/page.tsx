import Link from "next/link";
import { ArrowUpRight, Globe, KeyRound, Network, Terminal } from "lucide-react";
import { api, safe } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { StatTile, MeterTile } from "@/components/stat-tile";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PlanBadge } from "@/components/plan-badge";
import { CopyButton } from "@/components/copy-button";
import { formatBytes, formatNumber, pctOf } from "@/lib/format";

export default async function OverviewPage() {
  const [usage, subs, subdomains, domains, keys, tunnels] = await Promise.all([
    safe(api.usage()),
    safe(api.subscription()),
    safe(api.subdomains()),
    safe(api.domains()),
    safe(api.apiKeys()),
    safe(api.tunnels()),
  ]);

  const plan = subs?.plan ?? usage?.plan ?? "free";
  const bwUsed = (usage?.usage.bytes_in ?? 0) + (usage?.usage.bytes_out ?? 0);
  const bwLimit = usage?.limits.bandwidth_bytes_mo ?? 0;
  const bwPct = pctOf(bwUsed, bwLimit);
  const activeKeys = (keys ?? []).filter((k) => !k.revoked_at).length;

  return (
    <div>
      <PageHeader
        title="Overview"
        description="Your account at a glance."
        action={
          plan === "free" ? (
            <Link href="/billing">
              <Button>Upgrade plan</Button>
            </Link>
          ) : undefined
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MeterTile
          label="Bandwidth"
          used={formatBytes(bwUsed)}
          limitLabel={bwLimit ? formatBytes(bwLimit) : "∞"}
          pct={bwPct}
          note={bwLimit ? `${bwPct}% of monthly allowance` : "Metered"}
        />
        <StatTile label="Requests" value={formatNumber(usage?.usage.requests ?? 0)} sub="This period" />
        <StatTile label="Active tunnels" value={(tunnels ?? []).length} sub="Live now" icon={Network} />
        <StatTile
          label="Plan"
          value={<PlanBadge plan={plan} />}
          sub={activeKeys + " API key" + (activeKeys === 1 ? "" : "s")}
        />
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Terminal className="h-4 w-4 text-muted" /> Quick start
            </CardTitle>
            <CardDescription>Expose a local service in seconds.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Step n={1} title="Install the agent">
              <CodeLine text="curl -fsSL https://trqsh.uz/install | sh" />
            </Step>
            <Step n={2} title="Authenticate">
              <CodeLine text="trqsh login" />
            </Step>
            <Step n={3} title="Start a tunnel">
              <CodeLine text="trqsh http 3000" />
            </Step>
          </CardContent>
        </Card>

        <div className="flex flex-col gap-4">
          <QuickLink href="/domains" icon={Globe} title="Domains" desc={`${(subdomains ?? []).length} subdomains · ${(domains ?? []).length} custom`} />
          <QuickLink href="/keys" icon={KeyRound} title="API keys" desc={`${activeKeys} active`} />
          <QuickLink href="/usage" icon={ArrowUpRight} title="Usage & limits" desc="Track your monthly allowance" />
        </div>
      </div>
    </div>
  );
}

function Step({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-3">
      <div className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-semibold text-primary">
        {n}
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">{title}</p>
        <div className="mt-1.5">{children}</div>
      </div>
    </div>
  );
}

function CodeLine({ text }: { text: string }) {
  return (
    <div className="flex items-center justify-between gap-2 rounded-md border border-border bg-page px-3 py-2">
      <code className="truncate font-mono text-xs text-secondary">{text}</code>
      <CopyButton value={text} label="" />
    </div>
  );
}

function QuickLink({
  href,
  icon: Icon,
  title,
  desc,
}: {
  href: string;
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  desc: string;
}) {
  return (
    <Link href={href}>
      <Card className="p-4 transition-colors hover:border-border-strong">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-accent">
            <Icon className="h-4 w-4 text-primary" />
          </div>
          <div className="min-w-0">
            <p className="text-sm font-medium">{title}</p>
            <p className="truncate text-xs text-secondary">{desc}</p>
          </div>
        </div>
      </Card>
    </Link>
  );
}
