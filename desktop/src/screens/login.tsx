import { useEffect, useRef, useState } from "react";
import { ExternalLink, Eye, EyeOff, Globe, KeyRound, Loader2 } from "lucide-react";
import { agent, host, type HostInfo } from "@/lib/agent";
import type { DeviceStart } from "@/lib/types";
import { friendlyError } from "@/lib/errors";
import { useToast } from "@/components/ui/toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { InlineAlert } from "@/components/ui/inline-alert";

/** Turn a device-flow error code into a human sentence. */
function humanDeviceError(code: string): string {
  switch (code) {
    case "expired_token":
      return "The sign-in request timed out. Please try again.";
    case "invalid_device_code":
      return "This sign-in link is no longer valid. Please try again.";
    default:
      return code || "Sign-in failed.";
  }
}

/** Auth screen. Primary path is browser sign-in (OAuth via the device flow);
 *  pasting an API key stays available as an advanced fallback. */
export function Login({ onConnected }: { onConnected: () => void }) {
  const toast = useToast();
  const [env, setEnv] = useState<HostInfo | null>(null);

  // Browser (device-flow) sign-in.
  const [device, setDevice] = useState<DeviceStart | null>(null);
  const [starting, setStarting] = useState(false);
  const [browserErr, setBrowserErr] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);

  // API-key fallback.
  const [showKey, setShowKey] = useState(false);
  const [token, setToken] = useState("");
  const [reveal, setReveal] = useState(false);
  const [busy, setBusy] = useState(false);
  const [keyErr, setKeyErr] = useState<string | null>(null);

  useEffect(() => {
    host.env().then(setEnv).catch(() => {});
  }, []);
  const keysURL = env?.keys_url ?? "https://app.trqsh.uz/keys";

  // Stop polling if the component unmounts (e.g. sign-in succeeds elsewhere).
  useEffect(
    () => () => {
      if (pollRef.current) window.clearTimeout(pollRef.current);
    },
    [],
  );

  const poll = (d: DeviceStart) => {
    const interval = Math.max(2, d.interval || 2) * 1000;
    const tick = async () => {
      try {
        const r = await agent.oauthPoll(d.device_code);
        if (r.authed) {
          onConnected();
          return;
        }
        if (r.error) {
          setBrowserErr(humanDeviceError(r.error));
          setDevice(null);
          return;
        }
      } catch {
        /* transient — keep waiting */
      }
      pollRef.current = window.setTimeout(tick, interval);
    };
    pollRef.current = window.setTimeout(tick, interval);
  };

  const startBrowser = async () => {
    if (starting) return;
    setStarting(true);
    setBrowserErr(null);
    try {
      const d = await agent.oauthStart();
      setDevice(d);
      agent.openURL(d.verification_uri_complete);
      poll(d);
    } catch (err) {
      const msg = friendlyError(err);
      setBrowserErr(msg);
      toast.error("Couldn't start sign-in", { description: msg });
    } finally {
      setStarting(false);
    }
  };

  const cancelBrowser = () => {
    if (pollRef.current) window.clearTimeout(pollRef.current);
    pollRef.current = null;
    setDevice(null);
    setBrowserErr(null);
  };

  const submitKey = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token.trim() || busy) return;
    setBusy(true);
    setKeyErr(null);
    try {
      await agent.login(token.trim());
      onConnected();
    } catch (err) {
      const msg = friendlyError(err);
      setKeyErr(msg);
      toast.error("Couldn't connect", { description: msg });
    } finally {
      setBusy(false);
    }
  };

  // Waiting-for-browser state.
  if (device) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <div className="w-full max-w-sm text-center">
          <div className="mx-auto mb-5 flex size-12 items-center justify-center rounded-xl bg-primary/10 ring-1 ring-primary/20">
            <Loader2 className="size-6 animate-spin text-primary" />
          </div>
          <h1 className="text-lg font-semibold">Finish signing in</h1>
          <p className="mt-1 text-sm text-muted">
            We opened your browser to approve this device. Confirm the code below, then come back.
          </p>

          <div className="my-5 rounded-lg border border-border bg-surface px-4 py-4">
            <p className="text-[11px] uppercase tracking-wide text-muted">Your code</p>
            <p className="mt-1 font-mono text-2xl font-semibold tracking-[0.3em] text-foreground">
              {device.user_code}
            </p>
          </div>

          <div className="flex items-center justify-center gap-2 text-sm text-secondary">
            <Spinner />
            Waiting for approval…
          </div>

          <div className="mt-5 flex flex-col gap-2">
            <Button variant="secondary" onClick={() => agent.openURL(device.verification_uri_complete)}>
              <ExternalLink />
              Reopen browser
            </Button>
            <button
              type="button"
              onClick={cancelBrowser}
              className="rounded-sm text-xs text-muted transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-page"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center gap-3 text-center">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10 ring-1 ring-primary/20">
            <Globe className="size-6 text-primary" />
          </div>
          <div className="flex flex-col gap-1">
            <h1 className="text-lg font-semibold">Sign in to trqsh</h1>
            <p className="text-sm text-muted">
              Sign in with your browser to manage tunnels, domains, and your plan.
            </p>
          </div>
        </div>

        <Button className="w-full" onClick={startBrowser} disabled={starting}>
          {starting ? <Spinner /> : <Globe />}
          {starting ? "Opening browser…" : "Sign in with browser"}
        </Button>

        {browserErr && <InlineAlert className="mt-3">{browserErr}</InlineAlert>}

        {!showKey ? (
          <div className="mt-4 text-center">
            <button
              type="button"
              onClick={() => setShowKey(true)}
              className="rounded-sm text-xs text-muted transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-page"
            >
              Use an API key instead
            </button>
          </div>
        ) : (
          <form onSubmit={submitKey} className="mt-5 flex flex-col gap-3 border-t border-border pt-5">
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
                  className="absolute inset-y-0 right-0 flex w-9 items-center justify-center rounded-sm text-muted transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-page"
                >
                  {reveal ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                </button>
              </div>
            </div>

            {keyErr && <InlineAlert>{keyErr}</InlineAlert>}

            <Button type="submit" variant="secondary" disabled={busy || !token.trim()} className="w-full">
              {busy ? <Spinner /> : <KeyRound />}
              {busy ? "Connecting…" : "Connect with key"}
            </Button>
            <button
              type="button"
              onClick={() => agent.openURL(keysURL)}
              className="rounded-sm text-center text-xs text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-page"
            >
              Get a key from the dashboard →
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
