import { cn } from "@/lib/utils";

// A styled terminal mock for the hero — shows the whole value prop in one glance:
// one command, a live HTTPS URL, the local inspector, and a request log. Static
// markup with a CSS-only blinking caret (no client JS), so it stays cheap.
export function Terminal({ className }: { className?: string }) {
  return (
    <div className={cn("glass overflow-hidden rounded-xl", className)}>
      <div className="flex items-center gap-2 border-b border-white/5 px-4 py-2.5">
        <span className="h-3 w-3 rounded-full bg-critical/70" />
        <span className="h-3 w-3 rounded-full bg-warning/70" />
        <span className="h-3 w-3 rounded-full bg-good/70" />
        <span className="ml-2 text-xs text-muted">trqsh — bash</span>
        <span className="ml-auto inline-flex items-center gap-1.5 text-[0.7rem] text-good">
          <span className="h-1.5 w-1.5 animate-pulse-ring rounded-full bg-good" /> online
        </span>
      </div>
      <div className="overflow-x-auto px-4 py-4 font-mono text-[0.82rem] leading-relaxed">
        <pre className="text-secondary">
          <span className="text-muted">$ </span>
          <span className="text-foreground">trqsh http 3000</span>
        </pre>
        <pre className="mt-2 text-secondary">
          <span className="text-good">●</span> session online{"  "}
          <span className="text-muted">transport</span> <span className="text-brand">quic</span>{"  "}
          <span className="text-muted">region</span> us-east
        </pre>
        <pre className="mt-1 text-secondary">
          <span className="text-muted">Forwarding </span>
          <span className="font-semibold text-brand text-glow">https://tidy-otter-4f2a.trqsh.uz</span>
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
          <span className="text-good">200</span> GET&nbsp;&nbsp;/            12ms
          <br />
          <span className="text-good">200</span> GET&nbsp;&nbsp;/api/health  3ms
          <br />
          <span className="text-brand">201</span> POST /api/orders 28ms
        </pre>
        <pre className="mt-2 text-secondary">
          <span className="text-muted">$ </span>
          <span className="caret" />
        </pre>
      </div>
    </div>
  );
}
