import { useEffect, useMemo, useState } from "react";
import { Activity, ArrowLeft, Repeat, Trash2 } from "lucide-react";
import { agent } from "@/lib/agent";
import { friendlyError } from "@/lib/errors";
import { bytes, clock, decodeBody, duration, statusTone } from "@/lib/format";
import type { CapturedRequest } from "@/lib/types";
import { cn } from "@/lib/utils";
import { toCurl } from "@/lib/curl";
import { useWindowWidth } from "@/lib/hooks";
import { useToast } from "@/components/ui/toast";
import { Tabs } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
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

// Distinct from statusTone's good/warning/serious/critical (that's for the
// *outcome*, e.g. a 500) — this is the *intent* of the request. GET reads
// data (wire blue), POST creates (signal green), PUT/PATCH mutate (amber),
// DELETE destroys (critical) — deliberately not reusing "primary" here, since
// primary and good now share the same signal green and GET isn't a "live/
// primary action" the way starting a tunnel is.
function methodTone(method: string): string {
  switch (method) {
    case "GET":
      return "text-wire";
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

const METHODS = ["all", "GET", "POST", "PUT", "PATCH", "DELETE"];
const STATUS_CLASSES = [
  { id: "all", label: "All status" },
  { id: "2", label: "2xx" },
  { id: "3", label: "3xx" },
  { id: "4", label: "4xx" },
  { id: "5", label: "5xx" },
];

/** Pretty-print a captured (base64) body: JSON gets indented, else raw text. */
function prettyBody(b64?: string): string {
  const text = decodeBody(b64);
  if (!text) return "";
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
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
              <td className="w-1/3 bg-page/50 px-2 py-1 align-top font-medium text-secondary">{k}</td>
              <td className="selectable break-all px-2 py-1 font-mono text-foreground">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BodyBlock({ body }: { body?: string }) {
  const text = prettyBody(body);
  if (!text) return <p className="text-xs text-muted">Empty body</p>;
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-medium uppercase tracking-wide text-secondary">Body</span>
        <CopyButton value={text} label="Copy" />
      </div>
      <pre className="selectable max-h-72 overflow-auto rounded-md border border-border bg-page/50 p-2 font-mono text-xs text-foreground">
        {text}
      </pre>
    </div>
  );
}

function Detail({ req, onBack }: { req: CapturedRequest; onBack?: () => void }) {
  const [replaying, setReplaying] = useState(false);
  const [tab, setTab] = useState<"request" | "response">("request");
  const toast = useToast();
  const tone = statusTone(req.status);

  const replay = async () => {
    setReplaying(true);
    try {
      await agent.replay(req.id);
      toast.success("Request replayed", { description: `${req.method} ${req.path}` });
    } catch (e) {
      toast.error("Replay failed", { description: friendlyError(e) });
    } finally {
      setReplaying(false);
    }
  };

  return (
    <div className="flex flex-1 flex-col overflow-y-auto">
      <div className="flex flex-col gap-3 border-b border-border p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex items-center gap-2 text-sm">
              {onBack && (
                <button onClick={onBack} className="text-muted hover:text-foreground" aria-label="Back">
                  <ArrowLeft className="size-4" />
                </button>
              )}
              <span className={cn("font-semibold", methodTone(req.method))}>{req.method}</span>
              <span className="selectable break-all font-mono text-foreground">{req.path}</span>
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
              <span className={cn("font-semibold", toneClass[tone])}>{req.status || "—"}</span>
              <span>{duration(req.duration_ms)}</span>
              <span>{clock(req.started_at)}</span>
              <span className="tabular">↓ {bytes(req.bytes_in)}</span>
              <span className="tabular">↑ {bytes(req.bytes_out)}</span>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <CopyButton value={toCurl(req)} label="cURL" variant="secondary" />
            <Button variant="secondary" size="sm" onClick={replay} disabled={replaying}>
              {replaying ? <Spinner /> : <Repeat className="size-3.5" />}
              Replay
            </Button>
          </div>
        </div>
        <Tabs
          tabs={[
            { id: "request", label: "Request" },
            { id: "response", label: "Response" },
          ]}
          active={tab}
          onChange={(id) => setTab(id as "request" | "response")}
        />
      </div>

      <div className="flex flex-col gap-3 p-4">
        {tab === "request" ? (
          <>
            <span className="text-[10px] font-medium uppercase tracking-wide text-secondary">
              Request headers
            </span>
            <HeaderTable headers={req.req_headers} />
            <BodyBlock body={req.req_body} />
          </>
        ) : (
          <>
            <span className="text-[10px] font-medium uppercase tracking-wide text-secondary">
              Response headers
            </span>
            <HeaderTable headers={req.resp_headers} />
            <BodyBlock body={req.resp_body} />
          </>
        )}
      </div>
    </div>
  );
}

function Row({
  c,
  active,
  onClick,
}: {
  c: CapturedRequest;
  active: boolean;
  onClick: () => void;
}) {
  const tone = statusTone(c.status);
  return (
    <button
      onClick={onClick}
      className={cn(
        "relative flex w-full items-center gap-2 border-b border-border px-3 py-2 text-left text-xs transition-colors hover:bg-accent/60",
        active && "bg-accent",
      )}
    >
      {/* Mirrors the activity rail's own selection accent (commit 2) so "this
          is the selected item" reads the same way throughout the app. */}
      {active && (
        <span className="absolute inset-y-0 left-0 w-0.5 bg-primary" aria-hidden />
      )}
      <span className={cn("w-12 shrink-0 font-semibold", methodTone(c.method))}>{c.method}</span>
      <span className="flex-1 truncate font-mono text-foreground">{c.path}</span>
      <span className={cn("tabular w-8 shrink-0 text-right font-semibold", toneClass[tone])}>
        {c.status || "—"}
      </span>
      <span className="tabular w-14 shrink-0 text-right text-muted">{duration(c.duration_ms)}</span>
    </button>
  );
}

export function Inspector({
  captures,
  onClear,
}: {
  captures: CapturedRequest[];
  onClear: () => void;
}) {
  const wide = useWindowWidth() >= 880;
  const [query, setQuery] = useState("");
  const [method, setMethod] = useState("all");
  const [statusClass, setStatusClass] = useState("all");
  const [follow, setFollow] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return captures.filter((c) => {
      if (method !== "all" && c.method !== method) return false;
      if (statusClass !== "all" && String(c.status).charAt(0) !== statusClass) return false;
      if (q) {
        const hay = `${c.method} ${c.path} ${c.host} ${c.status}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [captures, query, method, statusClass]);

  // Auto-follow the newest matching capture (captures arrive newest-first) in the
  // wide two-pane layout; on narrow screens we don't yank the user into detail.
  useEffect(() => {
    if (wide && follow && filtered.length > 0) setSelectedId(filtered[0].id);
  }, [wide, follow, filtered]);

  const selected =
    filtered.find((c) => c.id === selectedId) ?? (wide ? filtered[0] ?? null : null);

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

  const toolbar = (
    <div className="flex flex-wrap items-center gap-2 border-b border-border p-2.5">
      <Input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Filter by path, method, status…"
        className="h-8 min-w-40 flex-1 text-xs"
      />
      <div className="w-24">
        <Select value={method} onChange={(e) => setMethod(e.target.value)} className="h-8 text-xs">
          {METHODS.map((m) => (
            <option key={m} value={m}>
              {m === "all" ? "All methods" : m}
            </option>
          ))}
        </Select>
      </div>
      <div className="w-24">
        <Select
          value={statusClass}
          onChange={(e) => setStatusClass(e.target.value)}
          className="h-8 text-xs"
        >
          {STATUS_CLASSES.map((s) => (
            <option key={s.id} value={s.id}>
              {s.label}
            </option>
          ))}
        </Select>
      </div>
      <Button
        variant={follow ? "default" : "secondary"}
        size="sm"
        onClick={() => setFollow((v) => !v)}
        title="Auto-select the newest request"
      >
        <Activity className="size-3.5" />
        Follow
      </Button>
      <Button variant="ghost" size="sm" onClick={onClear} title="Clear captured requests">
        <Trash2 className="size-3.5" />
        Clear
      </Button>
    </div>
  );

  const list = (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center justify-between px-3 py-1.5 text-[11px] text-muted">
        <span>
          {filtered.length}
          {filtered.length !== captures.length ? ` / ${captures.length}` : ""} shown
        </span>
      </div>
      {filtered.length === 0 ? (
        <p className="px-3 py-8 text-center text-xs text-muted">No requests match your filters.</p>
      ) : (
        <ul className="min-h-0 flex-1 overflow-y-auto">
          {filtered.map((c) => (
            <li key={c.id}>
              <Row c={c} active={selected?.id === c.id} onClick={() => setSelectedId(c.id)} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );

  // Narrow: master OR detail. Wide: master AND detail side-by-side.
  if (!wide) {
    return (
      <div className="flex flex-1 flex-col overflow-hidden">
        {selected ? (
          <Detail key={selected.id} req={selected} onBack={() => setSelectedId(null)} />
        ) : (
          <>
            {toolbar}
            {list}
          </>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      {toolbar}
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <div className="flex w-2/5 min-w-[16rem] max-w-md flex-col border-r border-border">{list}</div>
        {selected ? (
          <Detail key={selected.id} req={selected} />
        ) : (
          <div className="flex flex-1 items-center justify-center text-sm text-muted">
            Select a request
          </div>
        )}
      </div>
    </div>
  );
}
