// Typed facade over the Wails-bound Go AgentService (gui/app.go). Every method
// is a thin wrapper around Call.ByName("AgentService.<Method>", ...args); live
// updates arrive on the single "agent:event" Wails event, re-broadcast here as a
// typed subscription.
//
// All payloads are snake_case: Wails marshals with encoding/json, so the keys
// are the Go json tags (see lib/types.ts).
//
// When the bundle runs in a plain browser (vite dev / `vite preview`, or the
// build output opened without the desktop shell) there is no Go backend, so we
// fall back to an in-memory mock. This is what lets the whole GUI be developed,
// built, and screenshotted without compiling the Wails/CGO binary.

import { Call, Events } from "@wailsio/runtime";
import type {
  AgentEvent,
  CapturedRequest,
  Status,
  Tunnel,
  TunnelSpec,
} from "./types";

/** GUI-local settings persisted by the Go side (config.yml + keychain). */
export interface AppSettings {
  server: string;
  insecure: boolean;
  start_at_login: boolean;
  minimize_to_tray: boolean;
  theme: "system" | "light" | "dark";
}

/** Result of an auto-update check. */
export interface UpdateInfo {
  available: boolean;
  version: string;
  notes: string;
  url: string;
}

const EVENT = "agent:event";

/** Matches Wails' own runtime detection (Windows WebView2 / macOS WKWebView /
 *  Android). False in a normal browser tab → mock mode. */
export function isDesktop(): boolean {
  if (typeof window === "undefined") return false;
  const w = window as any;
  return !!(
    w.chrome?.webview?.postMessage ||
    w.webkit?.messageHandlers?.external?.postMessage ||
    w.wails?.invoke
  );
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export interface AgentAPI {
  login(token: string): Promise<Status>;
  logout(): Promise<void>;
  status(): Promise<Status>;
  startTunnel(spec: TunnelSpec): Promise<Tunnel>;
  stopTunnel(id: string): Promise<void>;
  list(): Promise<Tunnel[]>;
  recent(): Promise<CapturedRequest[]>;
  replay(id: string): Promise<void>;
  settings(): Promise<AppSettings>;
  saveSettings(s: AppSettings): Promise<void>;
  checkUpdate(): Promise<UpdateInfo>;
  openURL(url: string): Promise<void>;
  /** Subscribe to live agent events. Returns an unsubscribe function. */
  onEvent(cb: (e: AgentEvent) => void): () => void;
}

function realAPI(): AgentAPI {
  const call = <T>(method: string, ...args: unknown[]) =>
    Call.ByName(`AgentService.${method}`, ...args) as Promise<T>;
  return {
    login: (token) => call<Status>("Login", token),
    logout: () => call<void>("Logout"),
    status: () => call<Status>("Status"),
    startTunnel: (spec) => call<Tunnel>("StartTunnel", spec),
    stopTunnel: (id) => call<void>("StopTunnel", id),
    list: () => call<Tunnel[]>("List"),
    recent: () => call<CapturedRequest[]>("Recent"),
    replay: (id) => call<void>("Replay", id),
    settings: () => call<AppSettings>("Settings"),
    saveSettings: (s) => call<void>("SaveSettings", s),
    checkUpdate: () => call<UpdateInfo>("CheckUpdate"),
    openURL: (url) => call<void>("OpenURL", url),
    onEvent: (cb) =>
      Events.On(EVENT, (ev: { data: AgentEvent }) => cb(ev.data)),
  };
}

export const agent: AgentAPI = isDesktop() ? realAPI() : mockAPI();

// ---------------------------------------------------------------------------
// Browser-preview mock
// ---------------------------------------------------------------------------

function mockAPI(): AgentAPI {
  const listeners = new Set<(e: AgentEvent) => void>();
  const emit = (e: AgentEvent) => listeners.forEach((l) => l(e));

  let status: Status = {
    connected: false,
    account_id: "",
    plan: "free",
    edge: "",
    kind: "quic",
  };
  const tunnels = new Map<string, Tunnel>();
  const captures: CapturedRequest[] = [];
  let seq = 0;

  let settings: AppSettings = {
    server: "api.rift.dev:443",
    insecure: false,
    start_at_login: false,
    minimize_to_tray: true,
    theme: "system",
  };

  const sampleReq = (t: Tunnel): CapturedRequest => {
    seq++;
    const paths = ["/", "/api/health", "/users/42", "/assets/app.js", "/webhook"];
    const methods = ["GET", "GET", "GET", "POST", "PUT"];
    const codes = [200, 200, 200, 201, 204, 304, 404, 500];
    const i = seq % paths.length;
    const code = codes[seq % codes.length];
    const dur = 4 + Math.round(Math.random() * 180);
    return {
      id: `r${seq}`,
      tunnel_id: t.id,
      proto: "http",
      method: methods[i],
      host: t.public_url.replace(/^https?:\/\//, ""),
      path: paths[i],
      status: code,
      started_at: new Date().toISOString(),
      duration_ms: dur,
      req_headers: { "User-Agent": "curl/8.4.0", Accept: "*/*" },
      resp_headers: { "Content-Type": "application/json", Server: "rift" },
      req_body: "",
      resp_body: btoa(JSON.stringify({ ok: code < 400 })),
      bytes_in: 120 + (seq % 7) * 30,
      bytes_out: 300 + (seq % 5) * 220,
      local_addr: t.local_addr,
    };
  };

  let traffic: ReturnType<typeof setInterval> | undefined;
  const startTraffic = () => {
    if (traffic) return;
    traffic = setInterval(() => {
      const online = [...tunnels.values()].filter(
        (t) => t.status === "online" && t.proto === "http",
      );
      if (online.length === 0) return;
      const t = online[Math.floor(Math.random() * online.length)];
      const c = sampleReq(t);
      captures.unshift(c);
      if (captures.length > 200) captures.pop();
      t.metrics = {
        connections: t.metrics.connections + (Math.random() < 0.3 ? 1 : 0),
        requests: t.metrics.requests + 1,
        bytes_in: t.metrics.bytes_in + c.bytes_in,
        bytes_out: t.metrics.bytes_out + c.bytes_out,
      };
      emit({ type: "tunnel", tunnel: { ...t } });
      emit({ type: "request", request: c });
    }, 1600);
  };

  return {
    async login(token) {
      await delay(400);
      if (!token || token.length < 4) throw new Error("ERR_AUTH_INVALID: invalid API key");
      status = {
        connected: true,
        account_id: "acct_preview",
        plan: "free",
        edge: "fra1.rift.sh",
        kind: "quic",
      };
      emit({ type: "status", status: { ...status } });
      startTraffic();
      return { ...status };
    },
    async logout() {
      await delay(150);
      status = { ...status, connected: false, account_id: "", edge: "" };
      tunnels.clear();
      emit({ type: "status", status: { ...status } });
    },
    async status() {
      return { ...status };
    },
    async startTunnel(spec) {
      await delay(300);
      if (!status.connected) throw new Error("ERR_AUTH_REQUIRED: not logged in");
      const id = `tun_${++seq}`;
      const sub = spec.subdomain || `swift-${Math.random().toString(36).slice(2, 7)}`;
      const local = spec.addr.includes(":") ? spec.addr : `localhost:${spec.addr}`;
      const url =
        spec.proto === "http" || spec.proto === "tls"
          ? `https://${sub}.rift.sh`
          : `tcp://1.tcp.rift.sh:${10000 + (seq % 5000)}`;
      const t: Tunnel = {
        id,
        name: spec.name || sub,
        proto: spec.proto,
        local_addr: local,
        public_url: url,
        status: "connecting",
        metrics: { connections: 0, requests: 0, bytes_in: 0, bytes_out: 0 },
      };
      tunnels.set(id, t);
      emit({ type: "tunnel", tunnel: { ...t } });
      setTimeout(() => {
        const cur = tunnels.get(id);
        if (!cur) return;
        cur.status = "online";
        emit({ type: "tunnel", tunnel: { ...cur } });
      }, 700);
      return { ...t };
    },
    async stopTunnel(id) {
      await delay(150);
      // User-initiated stop is authoritative; the App re-lists after the call.
      // (Agent-side failures instead arrive as a tunnel event with status "error".)
      tunnels.delete(id);
    },
    async list() {
      return [...tunnels.values()].map((t) => ({ ...t }));
    },
    async recent() {
      return captures.map((c) => ({ ...c }));
    },
    async replay(id) {
      await delay(200);
      const src = captures.find((c) => c.id === id);
      const t = src && tunnels.get(src.tunnel_id);
      if (src && t) {
        const c = sampleReq(t);
        c.path = src.path;
        c.method = src.method;
        captures.unshift(c);
        emit({ type: "request", request: c });
      }
    },
    async settings() {
      return { ...settings };
    },
    async saveSettings(s) {
      await delay(120);
      settings = { ...s };
    },
    async checkUpdate() {
      await delay(500);
      return { available: false, version: "0.1.0", notes: "", url: "" };
    },
    async openURL(url) {
      if (typeof window !== "undefined") window.open(url, "_blank", "noopener");
    },
    onEvent(cb) {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
  };
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}
