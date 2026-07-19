import { useCallback, useEffect, useRef, useState } from "react";
import { agent } from "@/lib/agent";
import type { AgentEvent, CapturedRequest, Status, Tunnel } from "@/lib/types";
import { Titlebar } from "@/components/titlebar";
import { Sidebar, type Screen } from "@/components/sidebar";
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

export default function App() {
  const [booted, setBooted] = useState(false);
  const [status, setStatus] = useState<Status>(emptyStatus);
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [captures, setCaptures] = useState<CapturedRequest[]>([]);
  const [screen, setScreen] = useState<Screen>("tunnels");
  const statusRef = useRef(status);
  statusRef.current = status;

  const refreshTunnels = useCallback(() => {
    agent.list().then(setTunnels).catch(() => {});
  }, []);

  const hydrate = useCallback(() => {
    refreshTunnels();
    agent.recent().then(setCaptures).catch(() => {});
  }, [refreshTunnels]);

  // Boot: read current status (the agent may already be connected on relaunch).
  useEffect(() => {
    let alive = true;
    agent
      .status()
      .then((s) => {
        if (!alive) return;
        setStatus(s);
        if (s.connected) hydrate();
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
          if (e.status) {
            const wasConnected = statusRef.current.connected;
            setStatus(e.status);
            if (e.status.connected && !wasConnected) hydrate();
            if (!e.status.connected) {
              setTunnels([]);
              setCaptures([]);
            }
          }
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

  const disconnect = useCallback(async () => {
    await agent.logout().catch(() => {});
    setStatus(emptyStatus);
    setTunnels([]);
    setCaptures([]);
    setScreen("tunnels");
  }, []);

  if (!booted) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner className="size-5 text-muted" />
      </div>
    );
  }

  if (!status.connected) {
    return (
      <div className="flex h-full flex-col">
        <Titlebar status={status} />
        <Login onConnected={() => agent.status().then(setStatus)} />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <Titlebar status={status} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar active={screen} onSelect={setScreen} requestCount={captures.length} />
        <main className="flex flex-1 flex-col overflow-hidden">
          <div key={screen} className="animate-screen flex flex-1 flex-col overflow-hidden">
            {screen === "tunnels" && <Tunnels tunnels={tunnels} onChanged={refreshTunnels} />}
            {screen === "inspector" && <Inspector captures={captures} />}
            {screen === "account" && <Account status={status} tunnels={tunnels} />}
            {screen === "settings" && <Settings onDisconnect={disconnect} />}
          </div>
        </main>
      </div>
    </div>
  );
}
