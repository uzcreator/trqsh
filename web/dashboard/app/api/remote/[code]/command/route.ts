const API = process.env.TRQSH_API_URL || "http://localhost:8080";

// Proxies a command typed/tapped in the /qr viewer to the control plane —
// same-origin boundary as the viewer SSE route (see its sibling route.ts).
export async function POST(req: Request, { params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  const body = await req.text();
  const upstream = await fetch(`${API}/v1/remote/sessions/${encodeURIComponent(code)}/command`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
    cache: "no-store",
  });
  const text = await upstream.text();
  return new Response(text || null, {
    status: upstream.status,
    headers: { "Content-Type": upstream.headers.get("Content-Type") ?? "application/json" },
  });
}
