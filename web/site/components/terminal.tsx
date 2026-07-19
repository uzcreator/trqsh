import { cn } from "@/lib/utils";

// A static, styled terminal mock for the hero — shows the whole value prop in one
// glance: one command, a live HTTPS URL, the local inspector, and a request log.
export function Terminal({ className }: { className?: string }) {
  return (
    <div className={cn("overflow-hidden rounded-xl border border-border bg-surface shadow-lg", className)}>
      <div className="flex items-center gap-2 border-b border-border px-4 py-2.5">
        <span className="h-3 w-3 rounded-full bg-critical/70" />
        <span className="h-3 w-3 rounded-full bg-warning/70" />
        <span className="h-3 w-3 rounded-full bg-good/70" />
        <span className="ml-2 text-xs text-muted">rift — bash</span>
      </div>
      <div className="overflow-x-auto px-4 py-4 font-mono text-[0.82rem] leading-relaxed">
        <pre className="text-secondary">
          <span className="text-muted">$ </span>
          <span className="text-foreground">rift http 3000</span>
        </pre>
        <pre className="mt-2 text-secondary">
          <span className="text-good">●</span> session online{"  "}
          <span className="text-muted">transport</span> quic{"  "}
          <span className="text-muted">region</span> us-east
        </pre>
        <pre className="mt-1 text-secondary">
          <span className="text-muted">Forwarding </span>
          <span className="font-semibold text-primary">https://tidy-otter-4f2a.rift.sh</span>
          <span className="text-muted"> → </span>
          http://localhost:3000
        </pre>
        <pre className="text-secondary">
          <span className="text-muted">Inspect{"    "}</span>
          http://localhost:4040
        </pre>
        <pre className="mt-2 text-muted">
          HTTP requests
          <br />
          <span className="text-good">200</span> GET  /            12ms
          <br />
          <span className="text-good">200</span> GET  /api/health  3ms
          <br />
          <span className="text-primary">201</span> POST /api/orders 28ms
        </pre>
      </div>
    </div>
  );
}
