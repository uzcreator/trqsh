import { useEffect, useState } from "react";
import { ArrowRight, Eye, EyeOff, KeyRound } from "lucide-react";
import { agent, host, type HostInfo } from "@/lib/agent";
import { friendlyError } from "@/lib/errors";
import { useToast } from "@/components/ui/toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";

/** Auth screen: paste an API key to connect the agent to the edge. */
export function Login({ onConnected }: { onConnected: () => void }) {
  const toast = useToast();
  const [token, setToken] = useState("");
  const [reveal, setReveal] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [env, setEnv] = useState<HostInfo | null>(null);

  useEffect(() => {
    host.env().then(setEnv).catch(() => {});
  }, []);
  const keysURL = env?.keys_url ?? "https://app.trqsh.uz/keys";

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      await agent.login(token.trim());
      onConnected();
    } catch (err) {
      const msg = friendlyError(err);
      setError(msg);
      toast.error("Couldn't connect", { description: msg });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center gap-3 text-center">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10 ring-1 ring-primary/20">
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
            <div className="relative">
              <Input
                id="token"
                type={reveal ? "text" : "password"}
                autoFocus
                placeholder="tq_live_…"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="pr-10 font-mono"
                autoComplete="off"
                spellCheck={false}
              />
              <button
                type="button"
                onClick={() => setReveal((v) => !v)}
                title={reveal ? "Hide key" : "Show key"}
                aria-label={reveal ? "Hide key" : "Show key"}
                className="absolute inset-y-0 right-0 flex w-9 items-center justify-center text-muted transition-colors hover:text-foreground"
              >
                {reveal ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            </div>
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
            onClick={() => agent.openURL(keysURL)}
            className="text-xs text-primary hover:underline"
          >
            Don't have a key? Get one from the dashboard →
          </button>
        </div>
      </div>
    </div>
  );
}
