import { useEffect, useState } from "react";
import { Check, DownloadCloud, LogOut } from "lucide-react";
import { agent, host, type AppSettings, type HostInfo, type UpdateInfo } from "@/lib/agent";
import { applyTheme, type Theme } from "@/lib/theme";
import { friendlyError } from "@/lib/errors";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Spinner } from "@/components/ui/spinner";
import { Dialog } from "@/components/ui/dialog";
import { useToast } from "@/components/ui/toast";
import { InlineAlert } from "@/components/ui/inline-alert";

function Row({
  title,
  hint,
  children,
}: {
  title: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <div className="flex flex-col gap-0.5">
        <span className="text-sm font-medium">{title}</span>
        {hint && <span className="text-xs text-muted">{hint}</span>}
      </div>
      {children}
    </div>
  );
}

export function Settings({ onDisconnect }: { onDisconnect: () => void }) {
  const toast = useToast();
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [checking, setChecking] = useState(false);
  const [confirmOff, setConfirmOff] = useState(false);
  const [env, setEnv] = useState<HostInfo | null>(null);

  useEffect(() => {
    agent.settings().then(setSettings).catch((e) => setLoadError(friendlyError(e)));
    host.env().then(setEnv).catch(() => {});
  }, []);

  const version = env?.version ?? "0.1.0";
  const docs = env?.docs_url;

  const patch = (p: Partial<AppSettings>) =>
    setSettings((s) => (s ? { ...s, ...p } : s));

  const save = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      await agent.saveSettings(settings);
      applyTheme(settings.theme);
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
      toast.success("Settings saved");
    } catch (e) {
      toast.error("Couldn't save settings", { description: friendlyError(e) });
    } finally {
      setSaving(false);
    }
  };

  const checkUpdate = async () => {
    setChecking(true);
    try {
      const info = await agent.checkUpdate();
      setUpdate(info);
      if (info.available) {
        toast.info("Update available", { description: `Version ${info.version} is ready to download.` });
      } else {
        toast.success("You're on the latest version");
      }
    } catch (e) {
      toast.error("Update check failed", { description: friendlyError(e) });
    } finally {
      setChecking(false);
    }
  };

  if (!settings) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        {loadError ? (
          <InlineAlert className="max-w-sm text-center">{loadError}</InlineAlert>
        ) : (
          <Spinner className="size-5 text-muted" />
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <h1 className="text-base font-semibold">Settings</h1>

      <Card>
        <CardHeader>
          <CardTitle>Connection</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="server">Edge server</Label>
            <Input
              id="server"
              value={settings.server}
              onChange={(e) => patch({ server: e.target.value })}
              className="font-mono"
            />
          </div>
          <Row title="Transport" hint="QUIC is fastest; TCP maximizes compatibility.">
            <div className="w-32">
              <Select
                id="transport"
                value={settings.transport}
                onChange={(e) => patch({ transport: e.target.value as AppSettings["transport"] })}
              >
                <option value="auto">Auto</option>
                <option value="quic">QUIC</option>
                <option value="tcp">TCP</option>
              </Select>
            </div>
          </Row>
          <Row
            title="Allow insecure TLS"
            hint="Skip certificate verification. Only for local/self-hosted edges."
          >
            <Switch
              checked={settings.insecure}
              onChange={(v) => patch({ insecure: v })}
            />
          </Row>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Application</CardTitle>
        </CardHeader>
        <CardContent className="divide-y divide-border">
          <Row title="Theme">
            <div className="w-32">
              <Select
                value={settings.theme}
                onChange={(e) => {
                  const t = e.target.value as Theme;
                  patch({ theme: t });
                  applyTheme(t);
                }}
              >
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </Select>
            </div>
          </Row>
          <Row title="Start at login" hint="Launch trqsh when you sign in.">
            <Switch
              checked={settings.start_at_login}
              onChange={(v) => patch({ start_at_login: v })}
            />
          </Row>
          <Row title="Minimize to tray" hint="Keep tunnels running in the background.">
            <Switch
              checked={settings.minimize_to_tray}
              onChange={(v) => patch({ minimize_to_tray: v })}
            />
          </Row>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle>Updates</CardTitle>
          {docs && (
            <Button
              variant="link"
              size="sm"
              className="h-auto px-0"
              onClick={() => agent.openURL(docs)}
            >
              Open docs
            </Button>
          )}
        </CardHeader>
        <CardContent>
          <Row
            title="Version"
            hint={`trqsh ${version}${env?.os ? ` · ${env.os}/${env.arch}` : ""}`}
          >
            <Button variant="secondary" size="sm" onClick={checkUpdate} disabled={checking}>
              {checking ? <Spinner /> : <DownloadCloud className="size-3.5" />}
              Check for updates
            </Button>
          </Row>
          {update && (
            <p className="pt-1 text-xs text-muted">
              {update.available ? (
                <button
                  onClick={() => agent.openURL(update.url)}
                  className="text-primary hover:underline"
                >
                  Version {update.version} is available — download →
                </button>
              ) : (
                "You're on the latest version."
              )}
            </p>
          )}
        </CardContent>
      </Card>

      <div className="flex items-center justify-between">
        <Button
          variant="ghost"
          className="text-critical hover:bg-critical/10"
          onClick={() => setConfirmOff(true)}
        >
          <LogOut />
          Disconnect
        </Button>
        <Button onClick={save} disabled={saving}>
          {saving ? <Spinner /> : saved ? <Check /> : null}
          {saved ? "Saved" : "Save changes"}
        </Button>
      </div>

      <Dialog
        open={confirmOff}
        onClose={() => setConfirmOff(false)}
        title="Disconnect?"
        description="Active tunnels will stop and you'll return to sign-in."
      >
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setConfirmOff(false)}>
            Cancel
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              setConfirmOff(false);
              onDisconnect();
            }}
          >
            <LogOut />
            Disconnect
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
