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
