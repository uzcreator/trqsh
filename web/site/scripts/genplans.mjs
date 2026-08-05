#!/usr/bin/env node
// Regenerates web/site/lib/catalog.generated.ts from the control API's public
// plan catalog (GET /v1/plans/public — internal/api/handlers_plans_public.go,
// backed by internal/billing.Catalog). The site is TypeScript in its own repo
// and can't import Go source directly, so this fetches the same data over
// HTTP instead of the old Go-based generator it replaces.
//
// Regenerate: node scripts/genplans.mjs   (or: make site-plans)
// This does NOT run as part of a normal `pnpm build` — only CI's drift check
// and explicit local regeneration invoke it, so a routine build never depends
// on the API being reachable. See docs/openapi.yaml's /plans/public entry.

import { writeFile } from "node:fs/promises";

const apiUrl = process.env.TRQSH_API_URL || "https://api.trqsh.uz";
const rank = { free: 0, pro: 1, team: 2, payg: 3 };

async function main() {
  const url = `${apiUrl}/v1/plans/public`;
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`fetch ${url}: HTTP ${res.status}`);
  }
  const contentType = res.headers.get("content-type") || "";
  if (!contentType.includes("json")) {
    throw new Error(`fetch ${url}: unexpected content-type ${contentType || "(none)"}`);
  }
  const plans = await res.json();
  if (!Array.isArray(plans) || plans.length === 0) {
    throw new Error(`fetch ${url}: expected a non-empty plan array, got ${JSON.stringify(plans).slice(0, 200)}`);
  }
  plans.sort((a, b) => (rank[a.code] ?? 99) - (rank[b.code] ?? 99));

  const out = `// Code generated from the control API (GET /v1/plans/public) by
// web/site/scripts/genplans.mjs. DO NOT EDIT — run "make site-plans" (or
// "node scripts/genplans.mjs") after changing the plan catalog in
// internal/billing/plans.go.
//
// This is how the site consumes the single source of truth for pricing
// without hardcoding: prices/limits shown here can never drift from what the
// edge enforces.

export interface CatalogPlan {
  code: string;
  name: string;
  max_concurrent_tunnels: number;
  max_bandwidth_bytes_mo: number;
  max_requests_mo: number;
  max_reserved_subdomains: number;
  max_custom_domains: number;
  allow_custom_domains: boolean;
  allow_tcp: boolean;
  allow_tls: boolean;
  allow_udp: boolean;
  rate_limit_rps: number;
  /** Go time.Duration in nanoseconds (see formatRetention). */
  inspector_history: number;
  metered_seats: boolean;
  price_monthly_cents: number;
  price_annual_cents: number;
}

export const CATALOG: CatalogPlan[] = ${JSON.stringify(plans, null, 2)};
`;

  const dest = new URL("../lib/catalog.generated.ts", import.meta.url);
  await writeFile(dest, out);
  console.log(`wrote ${dest.pathname} (${plans.length} plans)`);
}

main().catch((err) => {
  console.error("genplans:", err.message);
  process.exit(1);
});
