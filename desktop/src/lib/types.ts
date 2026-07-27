// TypeScript mirrors of the frozen agent-core Go types (internal/agent/core.go
// and internal/agent/inspect). Field names match the Go json tags exactly:
// Wails marshals across the boundary with encoding/json, so these are the
// snake_case keys that actually arrive in the frontend.

/** Agent connection status. kind: "quic" | "tcp". */
export interface Status {
  connected: boolean;
  account_id: string;
  plan: string;
  edge: string;
  kind: string;
}

/** Request to open a tunnel. Mirrors agent.TunnelSpec. */
export interface TunnelSpec {
  name: string;
  proto: string; // http | tls | tcp | udp
  addr: string; // local target, e.g. "localhost:3000" or "3000"
  subdomain?: string;
  custom_domain?: string;
  basic_auth?: string;
  host_header?: string;
  remote_port?: number;
}

export interface TunnelMetrics {
  connections: number;
  requests: number;
  bytes_in: number;
  bytes_out: number;
}

/** A live tunnel. status: "connecting" | "online" | "error". */
export interface Tunnel {
  id: string;
  name: string;
  proto: string;
  local_addr: string;
  public_url: string;
  status: string;
  metrics: TunnelMetrics;
  /** RFC3339 time the tunnel came online. Server-side truth so uptime survives
   *  a window reload (it used to reset to 0 because it was timed client-side). */
  created_at: string;
}

/** One captured HTTP exchange from the local inspector. Body fields are base64
 *  (Go []byte). Use format.decodeBody to render. */
export interface CapturedRequest {
  id: string;
  tunnel_id: string;
  proto: string;
  method: string;
  host: string;
  path: string;
  status: number;
  started_at: string;
  duration_ms: number;
  req_headers?: Record<string, string>;
  resp_headers?: Record<string, string>;
  req_body?: string;
  resp_body?: string;
  bytes_in: number;
  bytes_out: number;
  local_addr: string;
}

/** Streamed agent event. type: "status" | "tunnel" | "request" | "error". */
export interface AgentEvent {
  type: string;
  status?: Status;
  tunnel?: Tunnel;
  request?: CapturedRequest;
  err?: string;
}

export type TunnelProto = "http" | "tcp" | "udp" | "tls";

// ---------------------------------------------------------------------------
// Cloud control-plane types. These arrive via the Go agent's /cloud/* proxy,
// which forwards to api.trqsh.uz with the stored API key (see cloudapi.go). Keys
// mirror the control API's json tags.
// ---------------------------------------------------------------------------

export interface CloudUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  oauth_provider?: string;
  created_at?: string;
}

export interface CloudOrg {
  id: string;
  name: string;
  plan: string;
  created_at?: string;
  plan_expires_at?: string | null;
}

export interface UsageRecord {
  bytes_in: number;
  bytes_out: number;
  requests: number;
  window_start?: string;
  window_end?: string;
}

/** GET /cloud/me — the single call the Account panel reads. */
export interface Me {
  user: CloudUser;
  org: CloudOrg;
  plan: string;
  plan_expires_at?: string | null;
  via_api_key?: boolean;
  usage: UsageRecord;
  limits: {
    bandwidth_bytes_mo: number;
    requests_mo: number;
    max_concurrent_tunnels: number;
    max_reserved_subdomains: number;
    max_custom_domains: number;
  };
}

export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  last_used_at?: string | null;
  revoked_at?: string | null;
  created_at: string;
}

/** Returned once, on creation, with the plaintext key. */
export interface CreatedApiKey {
  id: string;
  name: string;
  prefix: string;
  api_key: string;
  created_at: string;
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

/** POST /login/oauth/start — begins the browser (device) sign-in flow. */
export interface DeviceStart {
  device_code: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete: string;
  interval: number;
  expires_in: number;
}

/** POST /login/oauth/poll result. */
export interface OAuthPollResult {
  authed?: boolean;
  pending?: boolean;
  error?: string;
}
