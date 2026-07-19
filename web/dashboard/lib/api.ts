import { getTokens, setTokens, type Tokens } from "./session";

const BASE = process.env.RIFT_API_URL || "http://localhost:8080";

// ---- Domain types (mirror Part 05 responses; see docs/openapi.yaml) ----

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  oauth_provider?: string;
  created_at: string;
}
export interface Org {
  id: string;
  name: string;
  plan: string;
  stripe_customer_id?: string;
  created_at: string;
}
export interface Account {
  user: User;
  orgs: Org[];
  active_org: string;
}
export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  last_used_at?: string | null;
  revoked_at?: string | null;
  created_at: string;
}
export interface CreatedApiKey extends ApiKey {
  api_key: string;
}
export interface ReservedSubdomain {
  id: string;
  org_id: string;
  subdomain: string;
  created_at: string;
}
export interface CustomDomain {
  id: string;
  org_id: string;
  domain: string;
  verify_token: string;
  verified_at?: string | null;
  cert_status: string;
  created_at: string;
}
export interface DnsInstructions {
  txt_name: string;
  txt_value: string;
  cname_name: string;
  cname_value: string;
}
export interface AddedDomain {
  domain: CustomDomain;
  dns_instructions: DnsInstructions;
}
export interface UsageRecord {
  bytes_in: number;
  bytes_out: number;
  requests: number;
  window_start: string;
  window_end: string;
}
export interface UsageResponse {
  usage: UsageRecord;
  limits: { bandwidth_bytes_mo: number; requests_mo: number };
  plan: string;
}
export interface Tunnel {
  id: string;
  proto: string;
  public_url: string;
  local_addr: string;
  region?: string;
  bytes_in?: number;
  bytes_out?: number;
  requests?: number;
}
export interface Plan {
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
  inspector_history: number;
  metered_seats: boolean;
  price_monthly_cents: number;
  price_annual_cents: number;
}
export interface Subscription {
  id: string;
  plan: string;
  status: string;
  current_period_end?: string;
  cancel_at_period_end?: boolean;
}
export interface SubscriptionResponse {
  plan: string;
  billing_enabled: boolean;
  subscription?: Subscription;
}

export class ApiError extends Error {
  status: number;
  code?: string;
  constructor(status: number, message: string, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function refreshTokens(refresh: string): Promise<Tokens> {
  const res = await fetch(`${BASE}/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh }),
    cache: "no-store",
  });
  if (!res.ok) throw new ApiError(res.status, "session expired");
  return (await res.json()) as Tokens;
}

async function parse<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const body = text ? JSON.parse(text) : {};
  if (!res.ok) {
    throw new ApiError(res.status, body.error || res.statusText, body.error_code || body.code);
  }
  return body as T;
}

/**
 * Authenticated server-side fetch against the control API. On a 401 with a
 * refresh token present it transparently refreshes and retries once; the new
 * token is persisted when running in a Server Action / Route Handler, and used
 * in-memory for the current render otherwise (setTokens is a no-op there).
 */
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const { access, refresh } = await getTokens();
  const doFetch = (token?: string) =>
    fetch(`${BASE}/v1${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(init?.headers || {}),
      },
      cache: "no-store",
    });

  let res = await doFetch(access);
  if (res.status === 401 && refresh) {
    try {
      const tokens = await refreshTokens(refresh);
      try {
        await setTokens(tokens);
      } catch {
        /* not in an action/route handler; use the new token for this request only */
      }
      res = await doFetch(tokens.access_token);
    } catch {
      /* fall through with the original 401 */
    }
  }
  return parse<T>(res);
}

// ---- Unauthenticated auth calls ----

export async function login(email: string, password?: string): Promise<{ user: User; org: Org; tokens: Tokens }> {
  const res = await fetch(`${BASE}/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
    cache: "no-store",
  });
  return parse(res);
}

export async function signup(email: string, name: string): Promise<{ user: User; org: Org; tokens: Tokens }> {
  const res = await fetch(`${BASE}/v1/auth/signup`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, name }),
    cache: "no-store",
  });
  return parse(res);
}

// ---- Typed resource methods ----

/** Resolve a request to null instead of throwing, for pages that degrade gracefully. */
export async function safe<T>(p: Promise<T>): Promise<T | null> {
  try {
    return await p;
  } catch {
    return null;
  }
}

export const api = {
  account: () => apiFetch<Account>("/account"),
  orgs: () => apiFetch<Org[]>("/orgs"),
  apiKeys: () => apiFetch<ApiKey[]>("/api-keys"),
  createApiKey: (name: string) =>
    apiFetch<CreatedApiKey>("/api-keys", { method: "POST", body: JSON.stringify({ name }) }),
  revokeApiKey: (id: string) => apiFetch<void>(`/api-keys/${id}`, { method: "DELETE" }),

  subdomains: () => apiFetch<ReservedSubdomain[]>("/subdomains"),
  reserveSubdomain: (subdomain: string) =>
    apiFetch<ReservedSubdomain>("/subdomains", { method: "POST", body: JSON.stringify({ subdomain }) }),
  releaseSubdomain: (id: string) => apiFetch<void>(`/subdomains/${id}`, { method: "DELETE" }),

  domains: () => apiFetch<CustomDomain[]>("/domains"),
  addDomain: (domain: string) =>
    apiFetch<AddedDomain>("/domains", { method: "POST", body: JSON.stringify({ domain }) }),
  verifyDomain: (id: string) => apiFetch<CustomDomain>(`/domains/${id}/verify`, { method: "POST" }),

  usage: () => apiFetch<UsageResponse>("/usage"),
  tunnels: () => apiFetch<Tunnel[]>("/tunnels"),
  plans: () => apiFetch<Plan[]>("/plans"),

  subscription: () => apiFetch<SubscriptionResponse>("/billing/subscription"),
  checkout: (plan: string, cadence: string) =>
    apiFetch<{ url: string; session_id: string }>("/billing/checkout", {
      method: "POST",
      body: JSON.stringify({ plan, cadence }),
    }),
  portal: () => apiFetch<{ url: string }>("/billing/portal", { method: "POST" }),
};
