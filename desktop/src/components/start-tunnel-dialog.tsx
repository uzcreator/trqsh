import { useState } from "react";
import { Rocket } from "lucide-react";
import { agent } from "@/lib/agent";
import { friendlyError } from "@/lib/errors";
import type { TunnelProto, TunnelSpec } from "@/lib/types";
import { useToast } from "./ui/toast";
import { Dialog } from "./ui/dialog";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { Select } from "./ui/select";
import { Spinner } from "./ui/spinner";

const PROTOS: { value: TunnelProto; label: string }[] = [
  { value: "http", label: "HTTP" },
  { value: "tcp", label: "TCP" },
  { value: "udp", label: "UDP" },
  { value: "tls", label: "TLS" },
];

/** Start-tunnel form. Fields adapt to the selected protocol. */
export function StartTunnelDialog({
  open,
  onClose,
  onStarted,
}: {
  open: boolean;
  onClose: () => void;
  onStarted: () => void;
}) {
  const toast = useToast();
  const [proto, setProto] = useState<TunnelProto>("http");
  const [addr, setAddr] = useState("3000");
  const [name, setName] = useState("");
  const [subdomain, setSubdomain] = useState("");
  const [remotePort, setRemotePort] = useState("");
  const [basicAuth, setBasicAuth] = useState("");
  const [advanced, setAdvanced] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isHTTP = proto === "http" || proto === "tls";

  const reset = () => {
    setProto("http");
    setAddr("3000");
    setName("");
    setSubdomain("");
    setRemotePort("");
    setBasicAuth("");
    setAdvanced(false);
    setError(null);
  };

  const close = () => {
    if (busy) return;
    reset();
    onClose();
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!addr.trim() || busy) return;
    setBusy(true);
    setError(null);
    const spec: TunnelSpec = {
      name: name.trim(),
      proto,
      addr: addr.trim(),
      subdomain: isHTTP ? subdomain.trim() || undefined : undefined,
      basic_auth: isHTTP ? basicAuth.trim() || undefined : undefined,
      remote_port: !isHTTP && remotePort ? Number(remotePort) : undefined,
    };
    try {
      const t = await agent.startTunnel(spec);
      toast.success("Tunnel started", { description: t.public_url });
      reset();
      onStarted();
    } catch (err) {
      setError(friendlyError(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={close}
      title="New tunnel"
      description="Expose a local port to the public internet."
    >
      <form onSubmit={submit} className="flex flex-col gap-3">
        <div className="grid grid-cols-[7rem_1fr] gap-3">
          <div className="flex flex-col gap-1.5">
            <Label>Protocol</Label>
            <Select value={proto} onChange={(e) => setProto(e.target.value as TunnelProto)}>
              {PROTOS.map((p) => (
                <option key={p.value} value={p.value}>
                  {p.label}
                </option>
              ))}
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="addr">Local address</Label>
            <Input
              id="addr"
              autoFocus
              placeholder={isHTTP ? "3000 or localhost:3000" : "22"}
              value={addr}
              onChange={(e) => setAddr(e.target.value)}
              className="font-mono"
            />
          </div>
        </div>

        <button
          type="button"
          onClick={() => setAdvanced((v) => !v)}
          className="self-start text-xs text-primary hover:underline"
        >
          {advanced ? "Hide" : "Show"} advanced options
        </button>

        {advanced && (
          <div className="flex flex-col gap-3 rounded-md border border-border bg-page/50 p-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="name">Name (optional)</Label>
              <Input
                id="name"
                placeholder="my-api"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            {isHTTP ? (
              <>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="sub">Reserved subdomain (optional)</Label>
                  <Input
                    id="sub"
                    placeholder="my-app"
                    value={subdomain}
                    onChange={(e) => setSubdomain(e.target.value)}
                    className="font-mono"
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="auth">Basic auth (user:pass, optional)</Label>
                  <Input
                    id="auth"
                    placeholder="user:secret"
                    value={basicAuth}
                    onChange={(e) => setBasicAuth(e.target.value)}
                    className="font-mono"
                  />
                </div>
              </>
            ) : (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="rport">Remote port (optional)</Label>
                <Input
                  id="rport"
                  type="number"
                  placeholder="auto-assigned"
                  value={remotePort}
                  onChange={(e) => setRemotePort(e.target.value)}
                  className="font-mono"
                />
              </div>
            )}
          </div>
        )}

        {error && (
          <p className="rounded-md border border-critical/30 bg-critical/10 px-3 py-2 text-xs text-critical">
            {error}
          </p>
        )}

        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={close} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" disabled={busy || !addr.trim()}>
            {busy ? <Spinner /> : <Rocket />}
            {busy ? "Starting…" : "Start tunnel"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
