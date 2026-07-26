import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  BookOpen,
  DownloadCloud,
  ExternalLink,
  Globe,
  LayoutDashboard,
  LogOut,
  Moon,
  Plus,
  Power,
  Settings as SettingsIcon,
  Sun,
  User,
} from "lucide-react";
import { agent, host, waitForSession, type HostInfo } from "@/lib/agent";
import type { AgentEvent, CapturedRequest, Status, Tunnel } from "@/lib/types";
import { applyTheme, storedTheme, type Theme } from "@/lib/theme";
import { useHotkeys, useWindowWidth, type Hotkey } from "@/lib/hooks";
import { ToastProvider, useToast } from "@/components/ui/toast";
import { Titlebar } from "@/components/titlebar";
import { Sidebar, type Screen } from "@/components/sidebar";
import { CommandPalette, type Command } from "@/components/command-palette";
import { StartTunnelDialog } from "@/components/start-tunnel-dialog";
import { Spinner } from "@/components/ui/spinner";
import { Login } from "@/screens/login";
import { Tunnels } from "@/screens/tunnels";
import { Inspector } from "@/screens/inspector";
import { Settings } from "@/screens/settings";
import { Account } from "@/screens/account";

const MAX_CAPTURES = 200;

const emptyStatus: Status = {
  connected: false,
  account_id: "",
  plan: "free",
  edge: "",
  kind: "quic",
};

const isWebProto = (p: string) => p === "http" || p === "https" || p === "tls";

export default function App() {
  return (
    <ToastProvider>
      <Shell />
    </ToastProvider>
  );
}

function Shell() {
  const toast = useToast();
  const width = useWindowWidth();
  const [booted, setBooted] = useState(false);
  const [authed, setAuthed] = useState(false);
  const [status, setStatus] = useState<Status>(emptyStatus);
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [captures, setCaptures] = useState<CapturedRequest[]>([]);
  const [screen, setScreen] = useState<Screen>("tunnels");
  const [theme, setThemeState] = useState<Theme>(() => storedTheme());
  const [os, setOS] = useState("");
  const [env, setEnv] = useState<HostInfo | null>(null);
  const [minTray, setMinTray] = useState(true);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [newOpen, setNewOpen] = useState(false);
  const [manualCollapse, setManualCollapse] = useState<boolean | null>(null);

  const collapsed = manualCollapse ?? width < 940;

  const refreshTunnels = useCallback(() => {
    agent.list().then(setTunnels).catch(() => {});
    // Keep the transport pill fresh: it flips to "connected" once a tunnel
    // brings the edge session up (connection is lazy).
    agent.status().then(setStatus).catch(() => {});
  }, []);

  const hydrate = useCallback(() => {
    refreshTunnels();
    agent.recent().then(setCaptures).catch(() => {});
  }, [refreshTunnels]);

  // Boot: environment (OS + deep links), the tray preference, and the session
  // (are we signed in? + current transport status).
  useEffect(() => {
    let alive = true;
    host
      .env()
      .then((e) => {
        if (!alive) return;
        setEnv(e);
        setOS(e.os);
      })
      .catch(() => {});
    agent
      .settings()
      .then((s) => alive && setMinTray(s.minimize_to_tray))
      .catch(() => {});
    waitForSession()
      .then((s) => {
        if (!alive) return;
        setStatus(s.status);
        setAuthed(s.authed);
        if (s.authed) hydrate();
      })
      .catch(() => {})
      .finally(() => alive && setBooted(true));
    return () => {
      alive = false;
    };
  }, [hydrate]);

  // Single event stream from the Go agent core.
  useEffect(() => {
    const off = agent.onEvent((e: AgentEvent) => {
      switch (e.type) {
        case "status":
          // Transport up/down only moves the connection pill; it does not sign
          // the user out (auth is separate) or drop the tunnel/inspector state.
          if (e.status) setStatus(e.status);
          break;
        case "tunnel":
          if (e.tunnel) {
            const t = e.tunnel;
            setTunnels((prev) => {
              const i = prev.findIndex((x) => x.id === t.id);
              if (i === -1) return [...prev, t];
              const next = prev.slice();
              next[i] = t;
              return next;
            });
          }
          break;
        case "request":
          if (e.request) {
            const r = e.request;
            setCaptures((prev) => [r, ...prev].slice(0, MAX_CAPTURES));
          }
          break;
      }
    });
    return off;
  }, [hydrate]);

  // Tray "New tunnel…" opens the start dialog.
  useEffect(() => host.onUI("ui:new-tunnel", () => setNewOpen(true)), []);

  // Live refresh while signed in. The agent emits per-request events (which feed
  // the inspector) but not tunnel metric snapshots, so we poll the list to keep
  // traffic counters current, and prune inspector captures for any tunnel that
  // has stopped — so stopping a tunnel also stops its traffic in the inspector.
  useEffect(() => {
    if (!authed) return;
    const known = new Set<string>();
    let alive = true;
    const tick = () => {
      agent
        .list()
        .then((next) => {
          if (!alive) return;
          const nextIds = new Set(next.map((t) => t.id));
          setCaptures((caps) =>
            caps.filter((c) => !(known.has(c.tunnel_id) && !nextIds.has(c.tunnel_id))),
          );
          known.clear();
          nextIds.forEach((id) => known.add(id));
          setTunnels(next);
        })
        .catch(() => {});
      agent.status().then((s) => alive && setStatus(s)).catch(() => {});
    };
    tick();
    const iv = setInterval(tick, 2000);
    return () => {
      alive = false;
      clearInterval(iv);
    };
  }, [authed]);

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t);
    applyTheme(t);
  }, []);
  const toggleTheme = useCallback(
    () => setTheme(theme === "dark" ? "light" : "dark"),
    [theme, setTheme],
  );

  const disconnect = useCallback(async () => {
    await agent.logout().catch(() => {});
    setAuthed(false);
    setStatus(emptyStatus);
    setTunnels([]);
    setCaptures([]);
    setScreen("tunnels");
  }, []);

  const closeWindow = useCallback(() => {
    if (minTray) host.hide();
    else host.quit();
  }, [minTray]);

  const checkUpdate = useCallback(async () => {
    try {
      const u = await agent.checkUpdate();
      if (u.available) {
        toast.info("Update available", { description: `Version ${u.version} is ready to download.` });
      } else {
        toast.success("You're on the latest version");
      }
    } catch {
      toast.error("Update check failed");
    }
  }, [toast]);

  // Global keyboard shortcuts.
  const hotkeys = useMemo<Hotkey[]>(() => {
    const list: Hotkey[] = [
      { combo: "mod+k", allowInInput: true, handler: () => setPaletteOpen((v) => !v) },
    ];
    if (authed) {
      list.push(
        { combo: "mod+n", handler: () => setNewOpen(true) },
        { combo: "mod+1", handler: () => setScreen("tunnels") },
        { combo: "mod+2", handler: () => setScreen("inspector") },
        { combo: "mod+3", handler: () => setScreen("account") },
        { combo: "mod+4", handler: () => setScreen("settings") },
      );
    }
    return list;
  }, [authed]);
  useHotkeys(hotkeys);

  // Command palette actions.
  const commands = useMemo<Command[]>(() => {
    const cmds: Command[] = [];
    if (authed) {
      cmds.push(
        { id: "new", label: "New tunnel", group: "Tunnel", icon: Plus, keywords: "start expose port", run: () => setNewOpen(true) },
        { id: "go-tunnels", label: "Go to Tunnels", group: "Navigate", icon: Globe, run: () => setScreen("tunnels") },
        { id: "go-inspector", label: "Go to Inspector", group: "Navigate", icon: Activity, run: () => setScreen("inspector") },
        { id: "go-account", label: "Go to Account", group: "Navigate", icon: User, run: () => setScreen("account") },
        { id: "go-settings", label: "Go to Settings", group: "Navigate", icon: SettingsIcon, run: () => setScreen("settings") },
      );
      const web = tunnels.find((t) => isWebProto(t.proto));
      if (web) {
        cmds.push({
          id: "open-tunnel",
          label: "Open latest tunnel in browser",
          group: "Tunnel",
          icon: ExternalLink,
          run: () => agent.openURL(web.public_url),
        });
      }
      cmds.push({ id: "disconnect", label: "Disconnect", group: "Account", icon: LogOut, run: disconnect });
    }
    cmds.push(
      { id: "theme", label: "Toggle light / dark theme", group: "App", icon: theme === "dark" ? Sun : Moon, run: toggleTheme },
      { id: "update", label: "Check for updates", group: "App", icon: DownloadCloud, run: checkUpdate },
    );
    if (env) {
      cmds.push(
        { id: "dashboard", label: "Open dashboard", group: "App", icon: LayoutDashboard, run: () => agent.openURL(env.dashboard_url) },
        { id: "docs", label: "Open documentation", group: "App", icon: BookOpen, run: () => agent.openURL(env.docs_url) },
      );
    }
    cmds.push({ id: "quit", label: "Quit trqsh", group: "App", icon: Power, run: () => host.quit() });
    return cmds;
  }, [authed, tunnels, env, theme, disconnect, toggleTheme, checkUpdate]);

  if (!booted) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner className="size-5 text-muted" />
      </div>
    );
  }

  const bar = (
    <Titlebar
      status={status}
      os={os}
      theme={theme}
      onToggleTheme={toggleTheme}
      onOpenPalette={() => setPaletteOpen(true)}
      onClose={closeWindow}
    />
  );
  const palette = (
    <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} commands={commands} />
  );

  if (!authed) {
    return (
      <div className="flex h-full flex-col">
        {bar}
        <Login
          onConnected={() => {
            setAuthed(true);
            hydrate();
            agent.status().then(setStatus).catch(() => {});
          }}
        />
        {palette}
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {bar}
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          active={screen}
          onSelect={setScreen}
          requestCount={captures.length}
          collapsed={collapsed}
          onToggleCollapsed={() => setManualCollapse(!collapsed)}
        />
        <main className="flex flex-1 flex-col overflow-hidden">
          <div key={screen} className="animate-screen flex flex-1 flex-col overflow-hidden">
            {screen === "tunnels" && (
              <Tunnels tunnels={tunnels} onNew={() => setNewOpen(true)} onChanged={refreshTunnels} />
            )}
            {screen === "inspector" && <Inspector captures={captures} onClear={() => setCaptures([])} />}
            {screen === "account" && <Account status={status} tunnels={tunnels} />}
            {screen === "settings" && <Settings onDisconnect={disconnect} />}
          </div>
        </main>
      </div>
      <StartTunnelDialog
        open={newOpen}
        onClose={() => setNewOpen(false)}
        onStarted={() => {
          setNewOpen(false);
          refreshTunnels();
        }}
      />
      {palette}
    </div>
  );
}
