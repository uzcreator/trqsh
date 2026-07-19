import { Activity, ExternalLink } from "lucide-react";
import { api, safe, type Plan } from "@/lib/api";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatRetention } from "@/lib/format";

export default async function InspectPage() {
  const [subs, plans] = await Promise.all([safe(api.subscription()), safe(api.plans())]);
  const planCode = subs?.plan ?? "free";
  const plan: Plan | undefined = (plans ?? []).find((p) => p.code === planCode);
  const retention = plan ? formatRetention(plan.inspector_history) : "1 hour";

  return (
    <div>
      <PageHeader
        title="Request Inspector"
        description="Inspect and replay requests that flowed through your tunnels."
        action={<Badge variant="outline">Retention: {retention}</Badge>}
      />

      <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
        {/* Request list pane */}
        <Card className="min-h-[320px]">
          <div className="border-b border-border px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted">
            Requests
          </div>
          <div className="flex h-[260px] items-center justify-center px-4 text-center text-sm text-secondary">
            Waiting for traffic…
          </div>
        </Card>

        {/* Detail pane */}
        <Card className="flex min-h-[320px] items-center justify-center">
          <CardContent className="flex max-w-md flex-col items-center py-10 text-center">
            <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-accent">
              <Activity className="h-5 w-5 text-primary" />
            </div>
            <p className="font-medium">The cloud inspector activates with live captures</p>
            <p className="mt-1 text-sm text-secondary">
              Captured requests stream here once the edge relays them through the control API. Detail
              (headers, timing, body) and one-click replay to your local service will appear per request,
              retained for {retention} on your plan.
            </p>
            <a
              href="http://127.0.0.1:4040"
              target="_blank"
              rel="noreferrer"
              className="mt-4 inline-flex items-center gap-1.5 text-sm text-primary hover:underline"
            >
              Open the local inspector (:4040) <ExternalLink className="h-3.5 w-3.5" />
            </a>
          </CardContent>
        </Card>
      </div>

      <p className="mt-4 text-xs text-muted">
        The cloud inspector consumes a capture stream surfaced by the agent/edge via the control API
        (websocket relay or stored recent captures per plan retention). That relay is a follow-up on the
        edge; the local inspector at <span className="font-mono">127.0.0.1:4040</span> works today.
      </p>
    </div>
  );
}
