import { Network } from "lucide-react";
import { api, safe } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, THead, TBody, TR, TH, TD } from "@/components/ui/table";
import { CopyButton } from "@/components/copy-button";
import { formatBytes, formatNumber } from "@/lib/format";

export default async function TunnelsPage() {
  const tunnels = (await safe(api.tunnels())) ?? [];

  return (
    <div>
      <PageHeader title="Tunnels" description="Live tunnels served by the edge for your account." />

      {tunnels.length === 0 ? (
        <EmptyState
          icon={Network}
          title="No active tunnels"
          description="Run `trqsh http 3000` (or tcp/udp) to open a tunnel. Active tunnels show up here in real time."
        />
      ) : (
        <Card>
          <Table>
            <THead>
              <TR>
                <TH>Proto</TH>
                <TH>Public URL</TH>
                <TH>Local</TH>
                <TH>Region</TH>
                <TH className="text-right">In</TH>
                <TH className="text-right">Out</TH>
                <TH className="text-right">Reqs</TH>
              </TR>
            </THead>
            <TBody>
              {tunnels.map((t) => (
                <TR key={t.id}>
                  <TD>
                    <Badge variant="outline" className="uppercase">
                      {t.proto}
                    </Badge>
                  </TD>
                  <TD>
                    <div className="flex items-center gap-2">
                      <a
                        href={t.public_url}
                        target="_blank"
                        rel="noreferrer"
                        className="font-mono text-xs text-primary hover:underline"
                      >
                        {t.public_url}
                      </a>
                      <CopyButton value={t.public_url} label="" />
                    </div>
                  </TD>
                  <TD className="font-mono text-xs text-secondary">{t.local_addr}</TD>
                  <TD className="text-secondary">{t.region || "—"}</TD>
                  <TD className="text-right tabular text-secondary">{formatBytes(t.bytes_in ?? 0)}</TD>
                  <TD className="text-right tabular text-secondary">{formatBytes(t.bytes_out ?? 0)}</TD>
                  <TD className="text-right tabular text-secondary">{formatNumber(t.requests ?? 0)}</TD>
                </TR>
              ))}
            </TBody>
          </Table>
        </Card>
      )}

      <p className="mt-4 text-xs text-muted">
        The live tunnel list is served by the edge registry via the control API. It populates once an
        agent connects and binds a tunnel.
      </p>
    </div>
  );
}
