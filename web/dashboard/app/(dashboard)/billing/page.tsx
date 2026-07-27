import Link from "next/link";
import { Check, Send } from "lucide-react";
import { api, safe, type Plan } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PlanBadge } from "@/components/plan-badge";
import { formatBytes, formatPrice } from "@/lib/format";
import { cn } from "@/lib/utils";

const RANK: Record<string, number> = { free: 0, pro: 1, team: 2, payg: 1 };
const SITE = process.env.TRQSH_SITE_URL || "https://trqsh.uz";

function fmtDate(s?: string | null): string {
  if (!s) return "";
  const d = new Date(s);
  return isNaN(d.getTime())
    ? ""
    : d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export default async function BillingPage({
  searchParams,
}: {
  searchParams: Promise<{ cadence?: string }>;
}) {
  const sp = await searchParams;
  const cadence = sp.cadence === "annual" ? "annual" : "monthly";
  const [me, plansRaw] = await Promise.all([safe(api.me()), safe(api.plans())]);

  const currentPlan = me?.plan ?? "free";
  const expiresAt = me?.plan_expires_at ?? null;
  const expDate = fmtDate(expiresAt);
  const expired = !!expiresAt && new Date(expiresAt).getTime() < Date.now();
  const plans = (plansRaw ?? [])
    .filter((p) => p.code !== "payg")
    .sort((a, b) => (RANK[a.code] ?? 9) - (RANK[b.code] ?? 9));

  return (
    <div>
      <PageHeader title="Billing" description="Your plan and how to change it." />

      <Card className="mb-6">
        <CardContent className="flex flex-wrap items-center justify-between gap-4 p-5">
          <div className="flex items-center gap-3">
            <span className="text-sm text-secondary">Current plan</span>
            <PlanBadge plan={currentPlan} />
            {expDate &&
              (expired ? (
                <Badge variant="warning">Expired {expDate}</Badge>
              ) : (
                <Badge variant="good">Active until {expDate}</Badge>
              ))}
          </div>
          <a
            href={`${SITE}/upgrade?plan=${currentPlan === "team" ? "team" : "pro"}&cadence=${cadence}`}
            className={cn(buttonVariants({ variant: "outline" }))}
          >
            <Send className="h-4 w-4" />
            Contact us to change
          </a>
        </CardContent>
      </Card>

      <div className="mb-4 inline-flex rounded-md border border-border-strong bg-surface p-0.5 text-sm">
        <CadenceTab active={cadence === "monthly"} href="/billing?cadence=monthly" label="Monthly" />
        <CadenceTab active={cadence === "annual"} href="/billing?cadence=annual" label="Annual" />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {plans.map((p) => (
          <PlanCard key={p.code} plan={p} cadence={cadence} current={p.code === currentPlan} />
        ))}
      </div>

      <p className="mt-4 text-xs text-muted">
        We don&apos;t take card payments yet. To upgrade, message us on Telegram with your account email
        and the plan you want — we activate it manually, usually within a few hours.
      </p>
    </div>
  );
}

function CadenceTab({ active, href, label }: { active: boolean; href: string; label: string }) {
  return (
    <Link
      href={href}
      className={cn(
        "rounded px-3 py-1 font-medium transition-colors",
        active ? "bg-accent text-primary" : "text-secondary hover:text-foreground"
      )}
    >
      {label}
    </Link>
  );
}

function PlanCard({ plan, cadence, current }: { plan: Plan; cadence: string; current: boolean }) {
  // Annual shows the effective per-month price (yearly total ÷ 12), matching the
  // marketing site.
  const monthlyCents =
    cadence === "annual" ? Math.round(plan.price_annual_cents / 12) : plan.price_monthly_cents;
  const features = [
    `${plan.max_bandwidth_bytes_mo ? formatBytes(plan.max_bandwidth_bytes_mo) : "Metered"} bandwidth`,
    `${plan.max_concurrent_tunnels || "Metered"} concurrent tunnels`,
    `${plan.max_reserved_subdomains} reserved subdomains`,
    plan.allow_custom_domains ? `${plan.max_custom_domains} custom domains` : "HTTP/S + TCP",
    plan.allow_udp ? "UDP + TLS tunnels" : null,
  ].filter(Boolean) as string[];
  const paid = plan.code !== "free";

  return (
    <Card className={cn("flex flex-col", current && "border-primary ring-1 ring-primary")}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{plan.name}</CardTitle>
          {current && <Badge variant="good">Current</Badge>}
        </div>
        <CardDescription>
          <span className="text-2xl font-semibold text-foreground">{formatPrice(monthlyCents)}</span>
          {monthlyCents > 0 && <span className="text-sm text-muted"> /mo</span>}
          {cadence === "annual" && plan.price_annual_cents > 0 && (
            <span className="block text-xs text-muted">billed {formatPrice(plan.price_annual_cents)}/yr</span>
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col">
        <ul className="flex flex-1 flex-col gap-2 text-sm">
          {features.map((f) => (
            <li key={f} className="flex items-center gap-2">
              <Check className="h-4 w-4 shrink-0 text-good" />
              <span className="text-secondary">{f}</span>
            </li>
          ))}
        </ul>
        <div className="mt-5">
          {current ? (
            <Button variant="outline" className="w-full" disabled>
              Current plan
            </Button>
          ) : paid ? (
            <a
              href={`${SITE}/upgrade?plan=${plan.code}&cadence=${cadence}`}
              className={cn(buttonVariants({ variant: "default" }), "w-full")}
            >
              Get {plan.name}
            </a>
          ) : (
            <Button variant="outline" className="w-full" disabled>
              Free plan
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
