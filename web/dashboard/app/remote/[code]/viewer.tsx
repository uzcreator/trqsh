"use client";

import { useEffect, useRef, useState } from "react";
import { Radio, Send, Smartphone, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

// Mirrors internal/api/remote.go's remoteEvent/remoteState/remoteTunnel wire
// shapes — this is the one place in the dashboard that talks SSE instead of
// the usual REST + Server Component pattern (see app/api/remote's proxy).
interface RemoteTunnel {
  id: string;
  public_url: string;
  local_addr: string;
  status: string;
  requests: number;
}
interface RemoteState {
  online: boolean;
  edge: string;
  tunnels: RemoteTunnel[];
}
interface RemoteEvent {
  type: "lines" | "state" | "command" | "presence" | "ended";
  lines?: string[];
  state?: RemoteState;
  connected?: boolean;
}

type Phase = "connecting" | "live" | "ended" | "error";

const QUICK_ACTIONS = ["/status", "/ls", "/requests", "/whoami"];
const MAX_LINES = 500;

export function RemoteViewer({ code }: { code: string }) {
  const [phase, setPhase] = useState<Phase>("connecting");
  const [agentConnected, setAgentConnected] = useState(false);
  const [lines, setLines] = useState<string[]>([]);
  const [state, setState] = useState<RemoteState | null>(null);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let closedByServer = false;
    let everConnected = false;
    let errorCount = 0;
    const es = new EventSource(`/api/remote/${encodeURIComponent(code)}/viewer`);

    es.onopen = () => {
      everConnected = true;
      errorCount = 0;
      setPhase((p) => (p === "ended" ? p : "live"));
    };
    // EventSource retries transient drops on its own; only give up once we've
    // never successfully connected at all (a bad/expired code), after a
    // couple of attempts rather than the very first blip.
    es.onerror = () => {
      errorCount++;
      if (!everConnected && errorCount >= 3) {
        setPhase("error");
        es.close();
      }
    };
    es.onmessage = (e) => {
      let ev: RemoteEvent;
      try {
        ev = JSON.parse(e.data);
      } catch {
        return; // a stray comment/keepalive line, or a malformed frame
      }
      switch (ev.type) {
        case "lines":
          setLines((prev) => [...prev, ...(ev.lines ?? [])].slice(-MAX_LINES));
          break;
        case "state":
          if (ev.state) setState(ev.state);
          break;
        case "presence":
          setAgentConnected(!!ev.connected);
          break;
        case "ended":
          closedByServer = true;
          setPhase("ended");
          es.close();
          break;
      }
    };

    return () => {
      if (!closedByServer) es.close();
    };
  }, [code]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines]);

  async function send(text: string) {
    const trimmed = text.trim();
    if (!trimmed || sending) return;
    setSending(true);
    try {
      await fetch(`/api/remote/${encodeURIComponent(code)}/command`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text: trimmed }),
      });
      setInput("");
    } finally {
      setSending(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-lg flex-col gap-4 bg-page px-4 py-6">
      <header className="flex items-center justify-between">
        <div className="flex items-baseline gap-2">
          <span className="text-lg font-semibold tracking-tight">trqsh</span>
          <span className="text-sm text-muted">remote</span>
        </div>
        <PhaseBadge phase={phase} agentConnected={agentConnected} />
      </header>

      {phase === "ended" && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-10 text-center">
            <WifiOff className="h-8 w-8 text-muted" />
            <p className="font-medium">This pairing has ended</p>
            <p className="text-sm text-secondary">Run /qr again in the console to reconnect.</p>
          </CardContent>
        </Card>
      )}
      {phase === "error" && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-10 text-center">
            <WifiOff className="h-8 w-8 text-critical" />
            <p className="font-medium">Couldn&apos;t connect</p>
            <p className="text-sm text-secondary">Check the code, or run /qr again for a fresh one.</p>
          </CardContent>
        </Card>
      )}

      {(phase === "connecting" || phase === "live") && (
        <>
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="flex items-center justify-between text-sm font-medium text-secondary">
                <span>Console</span>
                {phase === "live" && !agentConnected && <span className="text-xs text-warning">disconnected</span>}
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              {state ? (
                <>
                  <div className="mb-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                    <span className={`h-2 w-2 rounded-full ${state.online ? "bg-good" : "bg-muted"}`} />
                    <span>{state.online ? "online" : "idle"}</span>
                    {state.edge && <span className="text-muted">· edge {state.edge}</span>}
                    <span className="text-muted">
                      · {state.tunnels.length} tunnel{state.tunnels.length === 1 ? "" : "s"}
                    </span>
                  </div>
                  {state.tunnels.length > 0 && (
                    <div className="flex flex-col gap-2">
                      {state.tunnels.map((t) => (
                        <div
                          key={t.id}
                          className="flex items-center justify-between gap-2 rounded-md border border-border bg-page px-3 py-2 text-sm"
                        >
                          <div className="min-w-0">
                            <a
                              href={t.public_url}
                              target="_blank"
                              rel="noreferrer"
                              className="block truncate font-mono text-primary"
                            >
                              {t.public_url}
                            </a>
                            <div className="truncate text-xs text-muted">→ {t.local_addr}</div>
                          </div>
                          <Badge variant={t.status === "online" ? "good" : "outline"} className="shrink-0">
                            {t.requests} req
                          </Badge>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              ) : (
                <p className="text-sm text-muted">Waiting for the console…</p>
              )}
            </CardContent>
          </Card>

          <div
            ref={scrollRef}
            className="min-h-[30vh] max-h-[45vh] flex-1 overflow-y-auto rounded-lg border border-border bg-surface p-3 font-mono text-xs leading-relaxed text-secondary"
          >
            {lines.length === 0 ? (
              <p className="text-muted">No output yet.</p>
            ) : (
              lines.map((l, i) => (
                <div key={i} className="whitespace-pre-wrap break-words">
                  {l || " "}
                </div>
              ))
            )}
          </div>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void send(input);
            }}
            className="flex gap-2"
          >
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="/status"
              autoCapitalize="off"
              autoComplete="off"
              className="font-mono"
              disabled={!agentConnected}
            />
            <Button type="submit" size="icon" disabled={!agentConnected || sending || !input.trim()}>
              <Send className="h-4 w-4" />
            </Button>
          </form>
          <div className="flex flex-wrap gap-2">
            {QUICK_ACTIONS.map((a) => (
              <Button
                key={a}
                type="button"
                variant="outline"
                size="sm"
                disabled={!agentConnected}
                onClick={() => void send(a)}
              >
                {a}
              </Button>
            ))}
          </div>
        </>
      )}

      <p className="mt-auto flex items-center justify-center gap-1.5 pt-2 text-center text-xs text-muted">
        <Smartphone className="h-3.5 w-3.5" /> paired session — closing the console ends this too
      </p>
    </main>
  );
}

function PhaseBadge({ phase, agentConnected }: { phase: Phase; agentConnected: boolean }) {
  if (phase === "connecting") return <Badge variant="warning">connecting…</Badge>;
  if (phase === "ended") return <Badge variant="muted">ended</Badge>;
  if (phase === "error") return <Badge variant="critical">unreachable</Badge>;
  return (
    <Badge variant={agentConnected ? "good" : "warning"}>
      <Radio className="h-3 w-3" />
      {agentConnected ? "live" : "console offline"}
    </Badge>
  );
}
