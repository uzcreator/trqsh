import { Check, Minus } from "lucide-react";
import { planByCode } from "@/lib/plans";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

// Honest three-way comparison. We credit competitors where they're genuinely
// strong (ngrok's inspector; Cloudflare's free unmetered transport + OSS agent),
// and only claim wins we can defend. Rift's own numbers come from the catalog so
// this table can't overstate our free tier.

type Val = boolean | string;
interface Row {
  feature: string;
  rift: Val;
  ngrok: Val;
  cloudflare: Val;
  note?: string;
}

const free = planByCode("free");
const riftFreeBandwidth = free ? `${formatBytes(free.max_bandwidth_bytes_mo)}/mo` : "10 GB/mo";

const ROWS: Row[] = [
  { feature: "QUIC / HTTP-3 transport", rift: true, ngrok: false, cloudflare: true },
  { feature: "UDP tunnels", rift: true, ngrok: false, cloudflare: "Limited", note: "Cloudflare routes UDP only via WARP private networks, not public tunnels." },
  { feature: "Native desktop app (Mac/Win/Linux)", rift: true, ngrok: false, cloudflare: false },
  { feature: "Local request inspector + replay", rift: true, ngrok: true, cloudflare: false },
  { feature: "Open-source agent", rift: true, ngrok: false, cloudflare: true },
  { feature: "Random subdomain, no domain needed", rift: true, ngrok: true, cloudflare: false, note: "Cloudflare Tunnel requires you to bring a domain onto Cloudflare's DNS." },
  { feature: "Free-tier transfer", rift: riftFreeBandwidth, ngrok: "1 GB/mo", cloudflare: "Unmetered", note: "Cloudflare Tunnel is free and unmetered but HTTP-first and tied to Cloudflare's network; ngrok's free tier tightened in 2026." },
  { feature: "Free-tier reserved subdomains", rift: String(free?.max_reserved_subdomains ?? 1), ngrok: "1 static domain", cloudflare: "—" },
];

export function ComparisonTable() {
  return (
    <div>
      <div className="overflow-x-auto rounded-lg border border-border bg-surface shadow-sm">
        <table className="w-full min-w-[680px] text-sm">
          <thead>
            <tr className="border-b border-border">
              <th className="p-4 text-left font-medium text-secondary">Capability</th>
              <th className="p-4 text-left font-semibold text-primary">Rift</th>
              <th className="p-4 text-left font-medium text-secondary">ngrok</th>
              <th className="p-4 text-left font-medium text-secondary">Cloudflare Tunnel</th>
            </tr>
          </thead>
          <tbody>
            {ROWS.map((row) => (
              <tr key={row.feature} className="border-b border-border last:border-0 align-top">
                <td className="p-4 text-secondary">
                  {row.feature}
                  {row.note && <span className="mt-0.5 block text-xs text-muted">{row.note}</span>}
                </td>
                <td className={cn("p-4 tabular", "bg-accent/40")}>
                  <ValueCell value={row.rift} strong />
                </td>
                <td className="p-4 tabular">
                  <ValueCell value={row.ngrok} />
                </td>
                <td className="p-4 tabular">
                  <ValueCell value={row.cloudflare} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-3 text-xs text-muted">
        Reflects public product positioning as of 2026; verify current specifics with each vendor.
        We&apos;d rather be accurate than flattering.
      </p>
    </div>
  );
}

function ValueCell({ value, strong }: { value: Val; strong?: boolean }) {
  if (value === true) return <Check className={cn("h-4 w-4", strong ? "text-primary" : "text-good")} aria-label="Yes" />;
  if (value === false) return <Minus className="h-4 w-4 text-muted" aria-label="No" />;
  return <span className={strong ? "font-medium text-foreground" : "text-secondary"}>{value}</span>;
}
