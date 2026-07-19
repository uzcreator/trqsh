import { CATALOG, type CatalogPlan } from "./catalog.generated";
import { formatBytes, formatRetention } from "./format";

export type { CatalogPlan };

/** All plans, cheapest → most flexible (order fixed by the generator). */
export const plans = CATALOG;

/** Flat-rate subscription tiers rendered as pricing cards. */
export const cardPlans = CATALOG.filter((p) => p.code !== "payg");

/** The metered plan, shown as a separate "scale beyond" strip. */
export const paygPlan = CATALOG.find((p) => p.code === "payg");

export function planByCode(code: string): CatalogPlan | undefined {
  return CATALOG.find((p) => p.code === code);
}

export type Cell = string | boolean;
export interface MatrixRow {
  label: string;
  /** One cell per plan, aligned to `plans` order. */
  cells: Cell[];
  hint?: string;
}

// Support tier and SSO are qualitative §11 facts not encoded as numbers in the Go
// catalog, so they're derived from the plan code here (they can't drift a price).
const SUPPORT: Record<string, string> = {
  free: "Community",
  pro: "Email",
  team: "Priority",
  payg: "Priority",
};

/** Builds the comparison matrix straight from the generated catalog. */
export function buildMatrix(list: CatalogPlan[] = CATALOG): MatrixRow[] {
  const tunnels = (p: CatalogPlan) => (p.max_concurrent_tunnels ? String(p.max_concurrent_tunnels) : "Metered");
  const bandwidth = (p: CatalogPlan) => (p.max_bandwidth_bytes_mo ? formatBytes(p.max_bandwidth_bytes_mo) : "Metered");
  const rate = (p: CatalogPlan) => (p.rate_limit_rps ? `${p.rate_limit_rps} req/s` : "Unbounded");
  const seats = (p: CatalogPlan) => p.code === "team" || p.code === "payg";

  return [
    { label: "Concurrent tunnels", cells: list.map(tunnels) },
    { label: "Bandwidth / month", cells: list.map(bandwidth) },
    { label: "Reserved subdomains", cells: list.map((p) => String(p.max_reserved_subdomains)) },
    {
      label: "Custom domains",
      cells: list.map((p) => (p.allow_custom_domains ? String(p.max_custom_domains) : false)),
    },
    { label: "HTTP / HTTPS", cells: list.map(() => true) },
    { label: "TCP tunnels", cells: list.map((p) => p.allow_tcp) },
    { label: "TLS tunnels", cells: list.map((p) => p.allow_tls) },
    { label: "UDP tunnels", cells: list.map((p) => p.allow_udp), hint: "ngrok has none" },
    { label: "Request rate limit", cells: list.map(rate) },
    { label: "Inspector history", cells: list.map((p) => formatRetention(p.inspector_history)) },
    { label: "Team seats & SSO/SAML", cells: list.map(seats) },
    { label: "Support", cells: list.map((p) => SUPPORT[p.code] ?? "Community") },
  ];
}
