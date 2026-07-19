import { useEffect, useState } from "react";
import { Check, DownloadCloud, LogOut } from "lucide-react";
import { agent, type AppSettings, type UpdateInfo } from "@/lib/agent";
import { applyTheme, type Theme } from "@/lib/theme";
import { friendlyError } from "@/lib/errors";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Spinner } from "@/components/ui/spinner";

const APP_VERSION = "0.1.0";

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
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [checking, setChecking] = useState(false);

  useEffect(() => {
    agent.settings().then(setSettings).catch((e) => setError(friendlyError(e)));
  }, []);

  const patch = (p: Partial<AppSettings>) =>
    setSettings((s) => (s ? { ...s, ...p } : s));

  const save = async () => {
    if (!settings) return;
    setSaving(true);
    setError(null);
    try {
      await agent.saveSettings(settings);
      applyTheme(settings.theme);
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setSaving(false);
    }
  };

  const checkUpdate = async () => {
    setChecking(true);
    try {
      setUpdate(await agent.checkUpdate());
    } catch (e) {
      setError(friendlyError(e));
    } finally {
      setChecking(false);
    }
  };

  if (!settings) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Spinner className="size-5 text-muted" />
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
          <Row title="Start at login" hint="Launch Rift when you sign in.">
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
        <CardHeader>
          <CardTitle>Updates</CardTitle>
        </CardHeader>
        <CardContent>
          <Row title="Version" hint={`Rift ${APP_VERSION}`}>
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

      {error && (
        <p className="rounded-md border border-critical/30 bg-critical/10 px-3 py-2 text-xs text-critical">
          {error}
        </p>
      )}

      <div className="flex items-center justify-between">
        <Button variant="ghost" className="text-critical hover:bg-critical/10" onClick={onDisconnect}>
          <LogOut />
          Disconnect
        </Button>
        <Button onClick={save} disabled={saving}>
          {saving ? <Spinner /> : saved ? <Check /> : null}
          {saved ? "Saved" : "Save changes"}
        </Button>
      </div>
    </div>
  );
}
