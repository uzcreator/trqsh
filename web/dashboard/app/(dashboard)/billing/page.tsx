import Link from "next/link";
import { Check } from "lucide-react";
import { api, safe, type Plan } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PlanBadge } from "@/components/plan-badge";
import { checkoutAction, portalAction } from "./actions";
import { formatBytes, formatPrice } from "@/lib/format";
import { cn } from "@/lib/utils";

const RANK: Record<string, number> = { free: 0, pro: 1, team: 2, payg: 1 };

export default async function BillingPage({
  searchParams,
}: {
  searchParams: Promise<{ cadence?: string; error?: string }>;
}) {
  const sp = await searchParams;
  const cadence = sp.cadence === "annual" ? "annual" : "monthly";
  const [subs, plansRaw] = await Promise.all([safe(api.subscription()), safe(api.plans())]);

  const currentPlan = subs?.plan ?? "free";
  const billingEnabled = subs?.billing_enabled ?? false;
  const plans = (plansRaw ?? [])
    .filter((p) => p.code !== "payg")
    .sort((a, b) => (RANK[a.code] ?? 9) - (RANK[b.code] ?? 9));

  return (
    <div>
      <PageHeader
        title="Billing"
        description="Manage your subscription, plan, and invoices."
        action={
          subs?.subscription || currentPlan !== "free" ? (
            <form action={portalAction}>
              <Button variant="outline" type="submit">
                Manage billing
              </Button>
            </form>
          ) : undefined
        }
      />

      {sp.error && (
        <div className="mb-6 rounded-lg border border-critical/40 bg-critical/10 p-4 text-sm text-critical">
          Could not open Stripe {sp.error === "portal" ? "Customer Portal" : "Checkout"}. Check that billing
          is configured, then try again.
        </div>
      )}

      <Card className="mb-6">
        <CardContent className="flex flex-wrap items-center justify-between gap-4 p-5">
          <div className="flex items-center gap-3">
            <span className="text-sm text-secondary">Current plan</span>
            <PlanBadge plan={currentPlan} />
            {subs?.subscription?.status && (
              <Badge variant={subs.subscription.status === "active" ? "good" : "warning"}>
                {subs.subscription.status}
              </Badge>
            )}
          </div>
          {!billingEnabled && (
            <span className="text-xs text-muted">
              Stripe is not configured on this environment — checkout is disabled.
            </span>
          )}
        </CardContent>
      </Card>

      <div className="mb-4 inline-flex rounded-md border border-border-strong bg-surface p-0.5 text-sm">
        <CadenceTab active={cadence === "monthly"} href="/billing?cadence=monthly" label="Monthly" />
        <CadenceTab active={cadence === "annual"} href="/billing?cadence=annual" label="Annual" />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {plans.map((p) => (
          <PlanCard
            key={p.code}
            plan={p}
            cadence={cadence}
            current={p.code === currentPlan}
            downgrade={(RANK[p.code] ?? 9) < (RANK[currentPlan] ?? 0)}
            billingEnabled={billingEnabled}
          />
        ))}
      </div>

      <p className="mt-4 text-xs text-muted">
        Upgrades open Stripe Checkout; “Manage billing” opens the Stripe Customer Portal to change your
        card, download invoices, or cancel. Plan changes take effect via webhook.
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

function PlanCard({
  plan,
  cadence,
  current,
  downgrade,
  billingEnabled,
}: {
  plan: Plan;
  cadence: string;
  current: boolean;
  downgrade: boolean;
  billingEnabled: boolean;
}) {
  const cents = cadence === "annual" ? plan.price_annual_cents : plan.price_monthly_cents;
  const suffix = cadence === "annual" ? "/yr" : "/mo";
  const features = [
    `${plan.max_bandwidth_bytes_mo ? formatBytes(plan.max_bandwidth_bytes_mo) : "Metered"} bandwidth`,
    `${plan.max_concurrent_tunnels || "Metered"} concurrent tunnels`,
    `${plan.max_reserved_subdomains} reserved subdomains`,
    plan.allow_custom_domains ? `${plan.max_custom_domains} custom domains` : "HTTP/S + TCP",
    plan.allow_udp ? "UDP + TLS tunnels" : null,
  ].filter(Boolean) as string[];

  return (
    <Card className={cn("flex flex-col", current && "border-primary ring-1 ring-primary")}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{plan.name}</CardTitle>
          {current && <Badge variant="good">Current</Badge>}
        </div>
        <CardDescription>
          <span className="text-2xl font-semibold text-foreground">{formatPrice(cents)}</span>
          {cents > 0 && <span className="text-sm text-muted"> {suffix}</span>}
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
          ) : (
            <form action={checkoutAction}>
              <input type="hidden" name="plan" value={plan.code} />
              <input type="hidden" name="cadence" value={cadence} />
              <Button className="w-full" type="submit" disabled={!billingEnabled}>
                {downgrade ? "Switch to " + plan.name : "Upgrade to " + plan.name}
              </Button>
            </form>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
