// Typed client over the Go agent's loopback control API (internal/agent/localapi.go).
// Every method is a fetch against http://127.0.0.1:4041 (JSON) with a bearer
// token; live updates arrive over the /events SSE stream, re-broadcast here as a
// typed subscription. This replaces the old Wails Call.ByName bridge — there is
// no in-process Go backend and no mock in production; the UI drives the real
// engine out-of-process, exactly like `trqsh status`/`stop` do.
//
// Two runtimes:
//   • Tauri (packaged app): the endpoint (base URL + token) comes from the Rust
//     side via invoke("get_agent_endpoint"); the Go agent runs as a sidecar.
//   • Browser dev (`pnpm dev` with no Tauri): falls back to 127.0.0.1:4041 with
//     an optional VITE_CONTROL_TOKEN, so the real UI can be developed against a
//     locally-run `trqsh daemon` (TRQSH_CONTROL_NO_AUTH=1) — the MSVC-free proof.
//
// All payloads are snake_case: the Go side marshals with encoding/json, so the
// keys are the Go json tags (see lib/types.ts).

import type {
  AgentEvent,
  CapturedRequest,
  Status,
  Tunnel,
  TunnelSpec,
} from "./types";

/** App-level settings persisted locally (theme/tray/etc). Connection fields
 *  mirror the agent config; changing them takes effect on the next daemon
 *  start. */
export interface AppSettings {
  server: string;
  transport: "auto" | "quic" | "tcp";
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

/** Host environment + product deep links. Lets the UI adapt window chrome to
 *  the OS and build links without hardcoding an origin. */
export interface HostInfo {
  os: string; // "darwin" | "windows" | "linux"
  arch: string;
  version: string;
  dashboard_url: string;
  keys_url: string;
  billing_url: string;
  docs_url: string;
}

/** Sign-in + transport snapshot from GET /session. `authed` means an API key is
 *  stored (the login gate); `status.connected` is the live edge transport, which
 *  only comes up once a tunnel is running (connection is lazy). */
export interface Session {
  authed: boolean;
  status: Status;
}

// ---------------------------------------------------------------------------
// Runtime + endpoint resolution
// ---------------------------------------------------------------------------

/** True when running inside the Tauri WebView (packaged app). */
export function hasTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

interface Endpoint {
  base: string;
  token: string;
}

let endpointCache: Endpoint | null = null;

async function resolveEndpoint(): Promise<Endpoint> {
  if (hasTauri()) {
    try {
      const { invoke } = await import("@tauri-apps/api/core");
      const ep = await invoke<Endpoint>("get_agent_endpoint");
      return { base: stripSlash(ep.base), token: ep.token || "" };
    } catch {
      /* fall through to the loopback default */
    }
  }
  const env = (import.meta as unknown as { env: Record<string, string> }).env;
  const base = env?.VITE_CONTROL_ADDR || "http://127.0.0.1:4041";
  const token = env?.VITE_CONTROL_TOKEN || "";
  return { base: stripSlash(base), token };
}

async function endpoint(): Promise<Endpoint> {
  if (endpointCache) return endpointCache;
  const ep = await resolveEndpoint();
  // In Tauri an empty token means the daemon hasn't written control.token yet;
  // don't cache it, so the next call re-reads once the agent is up. In browser
  // dev (or with auth disabled) an empty token is legitimate, so cache it.
  if (ep.token || !hasTauri()) endpointCache = ep;
  return ep;
}

function stripSlash(s: string): string {
  return s.replace(/\/+$/, "");
}

async function api<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const { base, token } = await endpoint();
  const headers: Record<string, string> = {};
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  // Bound every call so a dead or slow control endpoint (e.g. the agent still
  // starting) fails fast and the UI retries, instead of hanging. 15s comfortably
  // covers the slowest call (opening a tunnel connects to the edge, ~10s).
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 15000);
  try {
    const res = await fetch(base + path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: controller.signal,
    });
    if (!res.ok) throw new Error(await errorMessage(res));
    if (res.status === 204) return undefined as T;
    const ct = res.headers.get("content-type") || "";
    if (!ct.includes("application/json")) return undefined as T;
    return (await res.json()) as T;
  } finally {
    clearTimeout(timer);
  }
}

/** Extract a useful message from a non-2xx control-API response. The Go side
 *  returns `{"error": "..."}` for upstream failures and plain text otherwise. */
async function errorMessage(res: Response): Promise<string> {
  try {
    const ct = res.headers.get("content-type") || "";
    if (ct.includes("application/json")) {
      const j = (await res.json()) as { error?: string };
      if (j?.error) return j.error;
    } else {
      const t = (await res.text()).trim();
      if (t) return t;
    }
  } catch {
    /* ignore */
  }
  return `HTTP ${res.status}`;
}

// ---------------------------------------------------------------------------
// Agent API
// ---------------------------------------------------------------------------

export interface AgentAPI {
  session(): Promise<Session>;
  login(token: string): Promise<Session>;
  logout(): Promise<void>;
  status(): Promise<Status>;
  startTunnel(spec: TunnelSpec): Promise<Tunnel>;
  stopTunnel(id: string): Promise<void>;
  list(): Promise<Tunnel[]>;
  recent(): Promise<CapturedRequest[]>;
  /** Clear the agent's captured-request history (not just the on-screen list). */
  clearRequests(): Promise<void>;
  replay(id: string): Promise<void>;
  settings(): Promise<AppSettings>;
  saveSettings(s: AppSettings): Promise<void>;
  checkUpdate(): Promise<UpdateInfo>;
  openURL(url: string): Promise<void>;
  /** Subscribe to live agent events. Returns an unsubscribe function. */
  onEvent(cb: (e: AgentEvent) => void): () => void;
}

const SETTINGS_KEY = "trqsh.settings";

const defaultSettings: AppSettings = {
  server: "",
  transport: "auto",
  insecure: false,
  start_at_login: false,
  minimize_to_tray: true,
  theme: "system",
};

function loadSettings(): AppSettings {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY);
    if (raw) return { ...defaultSettings, ...JSON.parse(raw) };
  } catch {
    /* ignore */
  }
  return { ...defaultSettings };
}

/** True if dotted version `a` is strictly newer than `b` (e.g. "0.2.4" > "0.2.3").
 *  Non-numeric parts (e.g. a "dev" build) count as 0, so any real release is
 *  newer than a dev build. */
export function isVersionNewer(a: string, b: string): boolean {
  const pa = a.split(".").map((n) => parseInt(n, 10) || 0);
  const pb = b.split(".").map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const x = pa[i] || 0;
    const y = pb[i] || 0;
    if (x !== y) return x > y;
  }
  return false;
}

export const agent: AgentAPI = {
  session: () => api<Session>("GET", "/session"),
  login: (token) => api<Session>("POST", "/login", { token }),
  logout: () => api<void>("POST", "/logout"),
  status: () => api<Status>("GET", "/status"),
  startTunnel: (spec) => api<Tunnel>("POST", "/tunnels", spec),
  stopTunnel: (id) => api<void>("DELETE", `/tunnels/${encodeURIComponent(id)}`),
  list: () => api<Tunnel[]>("GET", "/tunnels"),
  recent: () => api<CapturedRequest[]>("GET", "/requests"),
  clearRequests: () => api<void>("DELETE", "/requests"),

  // Replaying a captured request isn't wired on the control API yet; surface a
  // clear message instead of silently doing nothing.
  async replay() {
    throw new Error("Replay isn't available yet");
  },

  async settings() {
    const s = loadSettings();
    if (!s.server) {
      try {
        const st = await api<Status>("GET", "/status");
        s.server = st.edge;
      } catch {
        /* leave blank */
      }
    }
    return s;
  },
  async saveSettings(s) {
    try {
      localStorage.setItem(SETTINGS_KEY, JSON.stringify(s));
    } catch {
      /* ignore */
    }
  },

  // Ask the agent for the latest published desktop release (it queries the
  // public GitHub releases from Go, so there's no WebView CORS/CSP issue), then
  // compare it against this build's version. Any failure is treated as "no
  // update" so a flaky network never blocks or nags the user.
  async checkUpdate() {
    try {
      const latest = await api<{ version: string; notes: string; url: string }>(
        "GET",
        "/update",
      );
      const current = (await host.env()).version;
      return {
        available: !!latest.version && isVersionNewer(latest.version, current),
        version: latest.version,
        notes: latest.notes || "",
        url: latest.url || "",
      };
    } catch {
      return { available: false, version: "", notes: "", url: "" };
    }
  },

  async openURL(url) {
    if (hasTauri()) {
      try {
        const { invoke } = await import("@tauri-apps/api/core");
        await invoke("open_url", { url });
        return;
      } catch {
        /* fall through */
      }
    }
    if (typeof window !== "undefined") window.open(url, "_blank", "noopener");
  },

  onEvent(cb) {
    let es: EventSource | null = null;
    let stopped = false;
    void (async () => {
      const { base, token } = await endpoint();
      if (stopped) return;
      const url = `${base}/events${token ? `?token=${encodeURIComponent(token)}` : ""}`;
      es = new EventSource(url);
      es.onmessage = (m) => {
        try {
          cb(JSON.parse(m.data) as AgentEvent);
        } catch {
          /* ignore malformed frame */
        }
      };
      // On error the browser auto-reconnects; nothing to do.
    })();
    return () => {
      stopped = true;
      es?.close();
    };
  },
};

/** Resolves the session, retrying while the agent sidecar is still starting up
 *  (connection refused, or 401 before the control token is readable). Gives up
 *  after ~4s and rethrows so the caller can fall back to the sign-in screen. */
export async function waitForSession(): Promise<Session> {
  let lastErr: unknown;
  for (let i = 0; i < 14; i++) {
    try {
      return await agent.session();
    } catch (e) {
      lastErr = e;
      await new Promise((r) => setTimeout(r, 300));
    }
  }
  throw lastErr;
}

// ---------------------------------------------------------------------------
// Host: environment + native window controls
// ---------------------------------------------------------------------------

export interface HostAPI {
  env(): Promise<HostInfo>;
  minimize(): Promise<void>;
  toggleMaximize(): Promise<void>;
  isMaximized(): Promise<boolean>;
  hide(): Promise<void>;
  quit(): Promise<void>;
  /** Subscribe to a named UI event (e.g. tray → "ui:new-tunnel"). */
  onUI(name: string, cb: () => void): () => void;
}

/** Best-effort OS guess from the user agent, so browser preview renders the
 *  correct window chrome (custom buttons on win/linux, native inset on mac). */
function guessOS(): string {
  if (typeof navigator === "undefined") return "linux";
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes("mac")) return "darwin";
  if (ua.includes("win")) return "windows";
  return "linux";
}

function realHost(): HostAPI {
  const winApi = () =>
    import("@tauri-apps/api/window").then((m) => m.getCurrentWindow());
  const core = () => import("@tauri-apps/api/core");
  return {
    async env() {
      const { invoke } = await core();
      return invoke<HostInfo>("get_host_info");
    },
    async minimize() {
      (await winApi()).minimize();
    },
    async toggleMaximize() {
      (await winApi()).toggleMaximize();
    },
    async isMaximized() {
      return (await winApi()).isMaximized();
    },
    async hide() {
      (await winApi()).hide();
    },
    async quit() {
      const { invoke } = await core();
      await invoke("quit");
    },
    onUI(name, cb) {
      let un: (() => void) | undefined;
      let cancelled = false;
      void import("@tauri-apps/api/event").then(({ listen }) =>
        listen(name, () => cb()).then((u) => {
          if (cancelled) u();
          else un = u;
        }),
      );
      return () => {
        cancelled = true;
        un?.();
      };
    },
  };
}

function browserHost(): HostAPI {
  let maximized = false;
  return {
    async env() {
      return {
        os: guessOS(),
        arch: "amd64",
        version: "dev",
        dashboard_url: "https://app.trqsh.uz",
        keys_url: "https://app.trqsh.uz/keys",
        billing_url: "https://app.trqsh.uz/billing",
        docs_url: "https://trqsh.uz/docs",
      };
    },
    async minimize() {},
    async toggleMaximize() {
      maximized = !maximized;
    },
    async isMaximized() {
      return maximized;
    },
    async hide() {},
    async quit() {},
    onUI() {
      return () => {};
    },
  };
}

export const host: HostAPI = hasTauri() ? realHost() : browserHost();
