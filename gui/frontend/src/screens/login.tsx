import { useState } from "react";
import { ArrowRight, KeyRound } from "lucide-react";
import { agent } from "@/lib/agent";
import { friendlyError } from "@/lib/errors";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";

const DASHBOARD_KEYS_URL = "https://dashboard.rift.dev/keys";

/** Auth screen: paste an API key to connect the agent to the edge. */
export function Login({ onConnected }: { onConnected: () => void }) {
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      await agent.login(token.trim());
      onConnected();
    } catch (err) {
      setError(friendlyError(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center gap-3 text-center">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10">
            <KeyRound className="size-6 text-primary" />
          </div>
          <div className="flex flex-col gap-1">
            <h1 className="text-lg font-semibold">Connect your agent</h1>
            <p className="text-sm text-muted">
              Paste an API key to open a secure tunnel to the nearest edge.
            </p>
          </div>
        </div>

        <form onSubmit={submit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="token">API key</Label>
            <Input
              id="token"
              type="password"
              autoFocus
              placeholder="rk_live_…"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              className="font-mono"
            />
          </div>

          {error && (
            <p className="rounded-md border border-critical/30 bg-critical/10 px-3 py-2 text-xs text-critical">
              {error}
            </p>
          )}

          <Button type="submit" disabled={busy || !token.trim()} className="w-full">
            {busy ? <Spinner /> : <ArrowRight />}
            {busy ? "Connecting…" : "Connect"}
          </Button>
        </form>

        <div className="mt-4 text-center">
          <button
            type="button"
            onClick={() => agent.openURL(DASHBOARD_KEYS_URL)}
            className="text-xs text-primary hover:underline"
          >
            Don't have a key? Get one from the dashboard →
          </button>
        </div>
      </div>
    </div>
  );
}
