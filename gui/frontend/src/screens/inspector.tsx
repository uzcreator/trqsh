import { useState } from "react";
import { Activity, Repeat } from "lucide-react";
import { agent } from "@/lib/agent";
import { clock, decodeBody, duration, statusTone } from "@/lib/format";
import type { CapturedRequest } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Empty } from "@/components/empty";
import { CopyButton } from "@/components/copy-button";

const toneClass: Record<ReturnType<typeof statusTone>, string> = {
  good: "text-good",
  warning: "text-warning",
  serious: "text-serious",
  critical: "text-critical",
  muted: "text-muted",
};

function methodTone(method: string): string {
  switch (method) {
    case "GET":
      return "text-primary";
    case "POST":
      return "text-good";
    case "PUT":
    case "PATCH":
      return "text-warning";
    case "DELETE":
      return "text-critical";
    default:
      return "text-secondary";
  }
}

function HeaderTable({ headers }: { headers?: Record<string, string> }) {
  const entries = Object.entries(headers ?? {});
  if (entries.length === 0) return <p className="text-xs text-muted">No headers</p>;
  return (
    <div className="overflow-hidden rounded-md border border-border">
      <table className="w-full text-xs">
        <tbody>
          {entries.map(([k, v]) => (
            <tr key={k} className="border-b border-border last:border-0">
              <td className="w-1/3 bg-page/50 px-2 py-1 align-top font-medium text-secondary">
                {k}
              </td>
              <td className="selectable break-all px-2 py-1 font-mono text-foreground">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BodyBlock({ title, body }: { title: string; body?: string }) {
  const text = decodeBody(body);
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-medium uppercase tracking-wide text-muted">{title}</span>
        {text && <CopyButton value={text} label="Copy" />}
      </div>
      {text ? (
        <pre className="selectable max-h-48 overflow-auto rounded-md border border-border bg-page/50 p-2 font-mono text-xs text-foreground">
          {text}
        </pre>
      ) : (
        <p className="text-xs text-muted">Empty</p>
      )}
    </div>
  );
}

function Detail({ req }: { req: CapturedRequest }) {
  const [replaying, setReplaying] = useState(false);
  const tone = statusTone(req.status);

  const replay = async () => {
    setReplaying(true);
    try {
      await agent.replay(req.id);
    } finally {
      setReplaying(false);
    }
  };

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2 text-sm">
            <span className={cn("font-semibold", methodTone(req.method))}>{req.method}</span>
            <span className="selectable break-all font-mono text-foreground">{req.path}</span>
          </div>
          <div className="flex items-center gap-3 text-xs text-muted">
            <span className={cn("font-semibold", toneClass[tone])}>
              {req.status || "—"}
            </span>
            <span>{duration(req.duration_ms)}</span>
            <span>{clock(req.started_at)}</span>
          </div>
        </div>
        <Button variant="secondary" size="sm" onClick={replay} disabled={replaying}>
          {replaying ? <Spinner /> : <Repeat className="size-3.5" />}
          Replay
        </Button>
      </div>

      <div className="flex flex-col gap-2">
        <span className="text-[10px] font-medium uppercase tracking-wide text-muted">
          Request headers
        </span>
        <HeaderTable headers={req.req_headers} />
      </div>
      <BodyBlock title="Request body" body={req.req_body} />

      <div className="flex flex-col gap-2">
        <span className="text-[10px] font-medium uppercase tracking-wide text-muted">
          Response headers
        </span>
        <HeaderTable headers={req.resp_headers} />
      </div>
      <BodyBlock title="Response body" body={req.resp_body} />
    </div>
  );
}

export function Inspector({ captures }: { captures: CapturedRequest[] }) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected = captures.find((c) => c.id === selectedId) ?? captures[0] ?? null;

  if (captures.length === 0) {
    return (
      <div className="flex flex-1 flex-col p-5">
        <div className="mb-4">
          <h1 className="text-base font-semibold">Inspector</h1>
          <p className="text-xs text-muted">Live HTTP traffic through your tunnels</p>
        </div>
        <Empty
          icon={Activity}
          title="Waiting for requests"
          hint="Requests to your HTTP tunnels appear here in real time. Hit your public URL to see traffic."
        />
      </div>
    );
  }

  return (
    <div className="flex flex-1 overflow-hidden">
      <div className="flex w-1/2 min-w-[18rem] flex-col border-r border-border">
        <div className="border-b border-border p-3">
          <h1 className="text-sm font-semibold">Inspector</h1>
          <p className="text-[11px] text-muted">{captures.length} captured</p>
        </div>
        <ul className="flex-1 overflow-y-auto">
          {captures.map((c) => {
            const active = selected?.id === c.id;
            const tone = statusTone(c.status);
            return (
              <li key={c.id}>
                <button
                  onClick={() => setSelectedId(c.id)}
                  className={cn(
                    "flex w-full items-center gap-2 border-b border-border px-3 py-2 text-left text-xs transition-colors hover:bg-accent/60",
                    active && "bg-accent",
                  )}
                >
                  <span className={cn("w-10 shrink-0 font-semibold", methodTone(c.method))}>
                    {c.method}
                  </span>
                  <span className="flex-1 truncate font-mono text-foreground">{c.path}</span>
                  <span className={cn("tabular w-8 shrink-0 text-right font-semibold", toneClass[tone])}>
                    {c.status || "—"}
                  </span>
                  <span className="tabular w-14 shrink-0 text-right text-muted">
                    {duration(c.duration_ms)}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </div>
      {selected && <Detail key={selected.id} req={selected} />}
    </div>
  );
}
