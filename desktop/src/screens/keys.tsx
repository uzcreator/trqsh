import { useCallback, useEffect, useState } from "react";
import { KeyRound, Plus, RefreshCw, ShieldAlert, Trash2 } from "lucide-react";
import { cloud } from "@/lib/agent";
import type { ApiKey, CreatedApiKey } from "@/lib/types";
import { friendlyError } from "@/lib/errors";
import { useToast } from "@/components/ui/toast";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { CopyButton } from "@/components/copy-button";

function fmtDate(s?: string | null): string {
  if (!s) return "—";
  const d = new Date(s);
  return isNaN(d.getTime())
    ? "—"
    : d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

/** API keys manager: list, create (shown once), and revoke keys for the org.
 *  The agent tunnels with one of these keys; the desktop reads/writes them
 *  through the cloud proxy. */
export function Keys() {
  const toast = useToast();
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    cloud.keys
      .list()
      .then(setKeys)
      .catch((e) => toast.error("Couldn't load keys", { description: friendlyError(e) }))
      .finally(() => setLoading(false));
  }, [toast]);
  useEffect(() => load(), [load]);

  const revoke = async (id: string) => {
    try {
      await cloud.keys.revoke(id);
      toast.success("Key revoked");
      load();
    } catch (e) {
      toast.error("Couldn't revoke key", { description: friendlyError(e) });
    }
  };

  const active = keys.filter((k) => !k.revoked_at);

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-base font-semibold">API keys</h1>
          <p className="text-sm text-muted">Credentials your agents use to open tunnels.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={load} disabled={loading} title="Refresh">
            <RefreshCw className={loading ? "animate-spin" : ""} />
          </Button>
          <Button size="sm" onClick={() => setDialogOpen(true)}>
            <Plus />
            New key
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Active keys</CardTitle>
        </CardHeader>
        <CardContent>
          {loading && keys.length === 0 ? (
            <div className="flex items-center justify-center py-6 text-muted">
              <Spinner />
            </div>
          ) : active.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-6 text-center text-muted">
              <KeyRound className="size-8" />
              <p className="text-sm">No API keys yet. Create one to connect an agent.</p>
              <Button size="sm" onClick={() => setDialogOpen(true)}>
                <Plus />
                New key
              </Button>
            </div>
          ) : (
            <div className="flex flex-col gap-1.5">
              {active.map((k) => (
                <div
                  key={k.id}
                  className="flex items-center gap-3 rounded-md border border-border bg-page px-3 py-2"
                >
                  <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-accent text-secondary">
                    <KeyRound className="size-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{k.name || "key"}</p>
                    <p className="truncate font-mono text-xs text-muted">{k.prefix}…</p>
                  </div>
                  <div className="hidden text-right text-xs text-muted sm:block">
                    <p>Created {fmtDate(k.created_at)}</p>
                    <p>Last used {fmtDate(k.last_used_at)}</p>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted hover:text-critical"
                    title="Revoke key"
                    onClick={() => revoke(k.id)}
                  >
                    <Trash2 />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <NewKeyDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        onCreated={load}
      />
    </div>
  );
}

function NewKeyDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const toast = useToast();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<CreatedApiKey | null>(null);

  const close = () => {
    if (created) onCreated();
    setName("");
    setCreated(null);
    setBusy(false);
    onClose();
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      const k = await cloud.keys.create(name.trim() || "desktop");
      setCreated(k);
    } catch (err) {
      toast.error("Couldn't create key", { description: friendlyError(err) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={close}
      title={created ? "Copy your key now" : "New API key"}
      description={
        created
          ? "This is the only time the full key is shown."
          : "Name the key so you remember where it's used."
      }
    >
      {created ? (
        <div className="flex flex-col gap-3">
          <div className="flex items-center gap-2 rounded-md border border-warning/30 bg-warning/10 px-3 py-2 text-xs text-warning">
            <ShieldAlert className="size-4 shrink-0" />
            Store it somewhere safe — you won&apos;t be able to see it again.
          </div>
          <div className="flex items-center gap-2 rounded-md border border-border bg-page p-2">
            <code className="flex-1 truncate font-mono text-xs">{created.api_key}</code>
            <CopyButton value={created.api_key} label="Copy" variant="secondary" />
          </div>
          <Button onClick={close} className="self-end">
            Done
          </Button>
        </div>
      ) : (
        <form onSubmit={submit} className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="key-name">Name</Label>
            <Input
              id="key-name"
              autoFocus
              placeholder="desktop"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={busy} className="self-end">
            {busy ? <Spinner /> : <Plus />}
            {busy ? "Creating…" : "Create key"}
          </Button>
        </form>
      )}
    </Dialog>
  );
}
