const API = process.env.TRQSH_API_URL || "http://localhost:8080";

// Proxies the /qr pairing viewer's SSE stream so the browser only ever talks
// to this same origin, never api.<base> directly — the same "never call the
// cloud directly" boundary the desktop app's local API already keeps for its
// WebView (see internal/agent/cloudapi.go), applied here for the public web
// viewer instead of a native WebView. Long-lived by design: this deployment
// runs as a persistent `next start` Node process (see deploy/docker-compose),
// not a request-scoped serverless function, so streaming through it is fine.
export async function GET(_req: Request, { params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  const upstream = await fetch(`${API}/v1/remote/sessions/${encodeURIComponent(code)}/viewer`, {
    headers: { Accept: "text/event-stream" },
    cache: "no-store",
  });

  if (!upstream.ok || !upstream.body) {
    return new Response(upstream.body, {
      status: upstream.status,
      headers: { "Content-Type": upstream.headers.get("Content-Type") ?? "application/json" },
    });
  }
  return new Response(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
    },
  });
}
