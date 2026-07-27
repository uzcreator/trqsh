"use client";

import * as React from "react";
import { Check, Minus } from "lucide-react";
import { cardPlans, paygPlan, buildMatrix, plans, type CatalogPlan } from "@/lib/plans";
import { formatBytes, formatPrice, formatRetention } from "@/lib/format";
import { signupUrl } from "@/lib/site";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type Cadence = "monthly" | "annual";

const POPULAR = "pro";

export function PricingTable() {
  const [cadence, setCadence] = React.useState<Cadence>("monthly");

  return (
    <div>
      <div className="mb-8 flex justify-center">
        <div className="inline-flex rounded-md border border-border-strong bg-surface p-0.5 text-sm">
          <Toggle active={cadence === "monthly"} onClick={() => setCadence("monthly")} label="Monthly" />
          <Toggle active={cadence === "annual"} onClick={() => setCadence("annual")} label="Annual" note="Save more" />
        </div>
      </div>

      <div className="grid gap-5 md:grid-cols-3">
        {cardPlans.map((p) => (
          <PlanCard key={p.code} plan={p} cadence={cadence} />
        ))}
      </div>

      {paygPlan && (
        <div className="mt-5 flex flex-col items-center justify-between gap-3 rounded-lg border border-border bg-surface p-5 text-center shadow-sm sm:flex-row sm:text-left">
          <div>
            <h3 className="text-base font-semibold text-foreground">{paygPlan.name}</h3>
            <p className="text-sm text-secondary">
              Metered bandwidth, tunnels, and domains — pay only for what you use. Ideal for spiky or
              programmatic workloads.
            </p>
          </div>
          <a href={`${signupUrl}?plan=payg`} className={cn(buttonVariants({ variant: "outline" }), "shrink-0")}>
            Talk to sales
          </a>
        </div>
      )}

      <FeatureMatrix cadence={cadence} />
    </div>
  );
}

function Toggle({ active, onClick, label, note }: { active: boolean; onClick: () => void; label: string; note?: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex items-center gap-2 rounded px-4 py-1.5 font-medium transition-colors",
        active ? "bg-accent text-primary" : "text-secondary hover:text-foreground"
      )}
    >
      {label}
      {note && <span className="rounded-full bg-good/15 px-1.5 py-0.5 text-[0.65rem] font-semibold text-good">{note}</span>}
    </button>
  );
}

function PlanCard({ plan, cadence }: { plan: CatalogPlan; cadence: Cadence }) {
  const popular = plan.code === POPULAR;
  const isAnnual = cadence === "annual";
  // Annual is shown as its effective per-month price (yearly total ÷ 12) with the
  // billed-yearly figure underneath — matches how the plans are priced ("$2/mo
  // billed yearly") rather than a large yearly lump sum.
  const cents = isAnnual ? Math.round(plan.price_annual_cents / 12) : plan.price_monthly_cents;
  const suffix = plan.price_monthly_cents === 0 ? "forever" : "/mo";

  const features = planHighlights(plan);
  const cta = plan.code === "free" ? "Start free" : `Get ${plan.name}`;
  // Free signs up in the dashboard; paid plans go to the manual (Telegram) upgrade
  // flow since there's no self-serve checkout.
  const href =
    plan.code === "free"
      ? `${signupUrl}?plan=${plan.code}`
      : `/upgrade?plan=${plan.code}&cadence=${cadence}`;

  return (
    <Card className={cn("card-hover relative flex flex-col p-6", popular && "border-primary ring-1 ring-primary")}>
      {popular && (
        <Badge variant="default" className="absolute -top-2.5 right-5">
          Most popular
        </Badge>
      )}
      <h3 className="text-lg font-semibold text-foreground">{plan.name}</h3>
      <div className="mt-2 flex items-baseline gap-1.5">
        <span className="text-3xl font-semibold tracking-tight text-foreground tabular">{formatPrice(cents)}</span>
        <span className="text-sm text-muted">{suffix}</span>
      </div>
      {isAnnual && plan.price_annual_cents > 0 && (
        <p className="mt-0.5 text-xs text-muted tabular">billed {formatPrice(plan.price_annual_cents)}/yr</p>
      )}
      <p className="mt-1 text-sm text-secondary">{planTagline(plan)}</p>

      <a
        href={href}
        className={cn(buttonVariants({ variant: popular ? "default" : "outline" }), "mt-5 w-full")}
      >
        {cta}
      </a>

      <ul className="mt-6 flex flex-1 flex-col gap-2.5 text-sm">
        {features.map((f) => (
          <li key={f} className="flex items-start gap-2">
            <Check className="mt-0.5 h-4 w-4 shrink-0 text-good" />
            <span className="text-secondary">{f}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function planTagline(p: CatalogPlan): string {
  switch (p.code) {
    case "free":
      return "For side projects and trying things out.";
    case "pro":
      return "For developers who ship every day.";
    case "team":
      return "For teams sharing tunnels and domains.";
    default:
      return "";
  }
}

// Highlights are derived from the generated catalog, never hardcoded numbers.
function planHighlights(p: CatalogPlan): string[] {
  const out: string[] = [
    `${p.max_concurrent_tunnels || "Metered"} concurrent tunnels`,
    `${p.max_bandwidth_bytes_mo ? formatBytes(p.max_bandwidth_bytes_mo) : "Metered"} bandwidth / month`,
    `${p.max_reserved_subdomains} reserved subdomain${p.max_reserved_subdomains === 1 ? "" : "s"}`,
    p.allow_custom_domains ? `${p.max_custom_domains} custom domains` : "HTTP/S + TCP tunnels",
  ];
  if (p.allow_udp) out.push("UDP + TLS tunnels");
  out.push(`${formatRetention(p.inspector_history)} inspector history`);
  if (p.code === "team") out.push("Team seats, SSO/SAML, priority support");
  return out;
}

function FeatureMatrix({ cadence }: { cadence: Cadence }) {
  const rows = buildMatrix(plans);
  return (
    <div className="mt-14">
      <h2 className="mb-5 text-center text-xl font-semibold tracking-tight text-foreground">Compare every plan</h2>
      <div className="overflow-x-auto rounded-lg border border-border bg-surface shadow-sm">
        <table className="w-full min-w-[640px] text-sm">
          <thead>
            <tr className="border-b border-border">
              <th className="p-4 text-left font-medium text-secondary">Feature</th>
              {plans.map((p) => (
                <th key={p.code} className="p-4 text-left font-semibold text-foreground">
                  {p.name}
                  <div className="text-xs font-normal text-muted tabular">
                    {p.price_monthly_cents === 0 && p.code === "free"
                      ? "Free"
                      : p.code === "payg"
                        ? "Metered"
                        : formatPrice(cadence === "annual" ? p.price_annual_cents : p.price_monthly_cents) +
                          (cadence === "annual" ? "/yr" : "/mo")}
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.label} className="border-b border-border last:border-0">
                <td className="p-4 text-secondary">
                  {row.label}
                  {row.hint && <span className="ml-1.5 text-xs text-muted">({row.hint})</span>}
                </td>
                {row.cells.map((cell, i) => (
                  <td key={i} className="p-4 tabular">
                    <Cell value={cell} />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Cell({ value }: { value: string | boolean }) {
  if (value === true) return <Check className="h-4 w-4 text-good" aria-label="Included" />;
  if (value === false) return <Minus className="h-4 w-4 text-muted" aria-label="Not included" />;
  return <span className="text-foreground">{value}</span>;
}
