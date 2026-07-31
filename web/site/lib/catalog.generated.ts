// Code generated from the control API (GET /v1/plans/public) by
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

export const CATALOG: CatalogPlan[] = [
  {
    "code": "free",
    "name": "Free",
    "max_concurrent_tunnels": 3,
    "max_bandwidth_bytes_mo": 10737418240,
    "max_requests_mo": 0,
    "max_reserved_subdomains": 1,
    "max_custom_domains": 0,
    "allow_custom_domains": false,
    "allow_tcp": true,
    "allow_tls": false,
    "allow_udp": false,
    "rate_limit_rps": 60,
    "inspector_history": 3600000000000,
    "metered_seats": false,
    "price_monthly_cents": 0,
    "price_annual_cents": 0
  },
  {
    "code": "pro",
    "name": "Pro",
    "max_concurrent_tunnels": 10,
    "max_bandwidth_bytes_mo": 214748364800,
    "max_requests_mo": 0,
    "max_reserved_subdomains": 10,
    "max_custom_domains": 5,
    "allow_custom_domains": true,
    "allow_tcp": true,
    "allow_tls": true,
    "allow_udp": true,
    "rate_limit_rps": 600,
    "inspector_history": 2592000000000000,
    "metered_seats": false,
    "price_monthly_cents": 300,
    "price_annual_cents": 2400
  },
  {
    "code": "team",
    "name": "Team",
    "max_concurrent_tunnels": 25,
    "max_bandwidth_bytes_mo": 1099511627776,
    "max_requests_mo": 0,
    "max_reserved_subdomains": 50,
    "max_custom_domains": 50,
    "allow_custom_domains": true,
    "allow_tcp": true,
    "allow_tls": true,
    "allow_udp": true,
    "rate_limit_rps": 2000,
    "inspector_history": 2592000000000000,
    "metered_seats": false,
    "price_monthly_cents": 500,
    "price_annual_cents": 4440
  },
  {
    "code": "payg",
    "name": "Pay-as-you-go",
    "max_concurrent_tunnels": 0,
    "max_bandwidth_bytes_mo": 0,
    "max_requests_mo": 0,
    "max_reserved_subdomains": 1000,
    "max_custom_domains": 1000,
    "allow_custom_domains": true,
    "allow_tcp": true,
    "allow_tls": true,
    "allow_udp": true,
    "rate_limit_rps": 0,
    "inspector_history": 2592000000000000,
    "metered_seats": true,
    "price_monthly_cents": 0,
    "price_annual_cents": 0
  }
];
