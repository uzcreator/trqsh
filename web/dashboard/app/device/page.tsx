import { Github, Waypoints } from "lucide-react";
import { getTokens } from "@/lib/session";
import { Button } from "@/components/ui/button";
import { DeviceApprove } from "./device-approve";

const API = process.env.TRQSH_API_URL || "http://localhost:8080";

// Device-authorization approval page (RFC 8628 style) used by the desktop app's
// "Sign in with browser" flow. It is reachable signed-in or signed-out: a
// signed-in user approves immediately; a signed-out user signs in via OAuth and
// is returned right back here (the ?code= is preserved through login).
export default async function DevicePage({
  searchParams,
}: {
  searchParams: Promise<{ code?: string }>;
}) {
  const { code = "" } = await searchParams;
  const { access } = await getTokens();
  const signedIn = !!access;
  const next = `/device${code ? `?code=${encodeURIComponent(code)}` : ""}`;
  const oauth = (provider: string) => `${API}/v1/auth/oauth/${provider}?next=${encodeURIComponent(next)}`;

  return (
    <main className="flex min-h-screen items-center justify-center bg-page px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Waypoints className="h-6 w-6" />
          </div>
          <h1 className="text-xl font-semibold tracking-tight">Connect a device</h1>
          <p className="mt-1 text-sm text-secondary">Authorize the trqsh desktop app.</p>
        </div>

        <div className="rounded-lg border border-border bg-surface p-6 shadow-sm">
          {signedIn ? (
            <DeviceApprove code={code} />
          ) : (
            <div className="flex flex-col gap-4">
              <p className="text-sm text-secondary">Sign in to approve this device.</p>
              <a href={oauth("github")}>
                <Button variant="outline" className="w-full" type="button">
                  <Github className="h-4 w-4" />
                  Continue with GitHub
                </Button>
              </a>
              <a href={oauth("google")}>
                <Button variant="outline" className="w-full" type="button">
                  Continue with Google
                </Button>
              </a>
            </div>
          )}
        </div>

        <p className="mt-4 text-center text-xs text-muted">
          You&apos;re approving the code shown in the desktop app. Never enter a code someone sent you.
        </p>
      </div>
    </main>
  );
}
