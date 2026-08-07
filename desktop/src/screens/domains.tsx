import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, Clock, Globe, Plus, RefreshCw, Trash2 } from "lucide-react";
import { cloud } from "@/lib/agent";
import type { CustomDomain, ReservedSubdomain } from "@/lib/types";
import { friendlyError } from "@/lib/errors";
import { useToast } from "@/components/ui/toast";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Skeleton } from "@/components/ui/skeleton";
import { CopyButton } from "@/components/copy-button";

const BASE = "trqsh.uz";

/** Domains manager: reserve subdomains (<name>.trqsh.uz) and add/verify custom
 *  domains with the DNS records the control plane requires. */
export function Domains() {
  const toast = useToast();
  const [subs, setSubs] = useState<ReservedSubdomain[]>([]);
  const [domains, setDomains] = useState<CustomDomain[]>([]);
  const [loading, setLoading] = useState(true);

  const [subInput, setSubInput] = useState("");
  const [domInput, setDomInput] = useState("");
  const [subBusy, setSubBusy] = useState(false);
  const [domBusy, setDomBusy] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    Promise.all([cloud.subdomains.list(), cloud.domains.list()])
      .then(([s, d]) => {
        setSubs(s || []);
        setDomains(d || []);
      })
      .catch((e) => toast.error("Couldn't load domains", { description: friendlyError(e) }))
      .finally(() => setLoading(false));
  }, [toast]);
  useEffect(() => load(), [load]);

  const reserve = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = subInput.trim().toLowerCase();
    if (!name || subBusy) return;
    setSubBusy(true);
    try {
      await cloud.subdomains.reserve(name);
      setSubInput("");
      toast.success("Subdomain reserved");
      load();
    } catch (err) {
      toast.error("Couldn't reserve", { description: friendlyError(err) });
    } finally {
      setSubBusy(false);
    }
  };

  const release = async (id: string) => {
    try {
      await cloud.subdomains.release(id);
      load();
    } catch (err) {
      toast.error("Couldn't release", { description: friendlyError(err) });
    }
  };

  const addDomain = async (e: React.FormEvent) => {
    e.preventDefault();
    const d = domInput.trim().toLowerCase();
    if (!d || domBusy) return;
    setDomBusy(true);
    try {
      await cloud.domains.add(d);
      setDomInput("");
      toast.success("Domain added — now add the DNS records below");
      load();
    } catch (err) {
      toast.error("Couldn't add domain", { description: friendlyError(err) });
    } finally {
      setDomBusy(false);
    }
  };

  const verify = async (id: string) => {
    try {
      await cloud.domains.verify(id);
      toast.success("Domain verified");
      load();
    } catch (err) {
      toast.error("Not verified yet", { description: friendlyError(err) });
    }
  };

  return (
    <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
      <div className="flex items-center justify-between">
        <h1 className="text-base font-semibold">Domains</h1>
        <Button variant="ghost" size="sm" onClick={load} disabled={loading} title="Refresh">
          <RefreshCw className={loading ? "animate-spin" : ""} />
        </Button>
      </div>

      {/* Reserved subdomains */}
      <Card>
        <CardHeader>
          <CardTitle>Reserved subdomains</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <form onSubmit={reserve} className="flex gap-2">
            <div className="flex flex-1 items-center rounded-md border border-border bg-page pr-2 focus-within:ring-2 focus-within:ring-ring">
              <Input
                value={subInput}
                onChange={(e) => setSubInput(e.target.value)}
                placeholder="myapp"
                className="border-0 bg-transparent focus-visible:ring-0"
              />
              <span className="shrink-0 text-sm text-muted">.{BASE}</span>
            </div>
            <Button type="submit" disabled={subBusy || !subInput.trim()}>
              {subBusy ? <Spinner /> : <Plus />}
              Reserve
            </Button>
          </form>

          {loading && subs.length === 0 ? (
            <div className="flex flex-col gap-1.5">
              <Skeleton className="h-9" />
              <Skeleton className="h-9" />
            </div>
          ) : subs.length === 0 ? (
            <p className="text-sm text-muted">No reserved subdomains yet.</p>
          ) : (
            <div className="flex flex-col gap-1.5">
              {subs.map((s) => (
                <div
                  key={s.id}
                  className="flex items-center gap-2 rounded-md border border-border bg-page px-3 py-2"
                >
                  <Globe className="size-4 shrink-0 text-muted" />
                  <span className="flex-1 truncate font-mono text-sm text-wire">
                    {s.subdomain}.{BASE}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7 text-muted hover:text-critical"
                    title="Release"
                    onClick={() => release(s.id)}
                  >
                    <Trash2 />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Custom domains */}
      <Card>
        <CardHeader>
          <CardTitle>Custom domains</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <form onSubmit={addDomain} className="flex gap-2">
            <Input
              value={domInput}
              onChange={(e) => setDomInput(e.target.value)}
              placeholder="tunnel.example.com"
              className="flex-1"
            />
            <Button type="submit" disabled={domBusy || !domInput.trim()}>
              {domBusy ? <Spinner /> : <Plus />}
              Add
            </Button>
          </form>

          {loading && domains.length === 0 ? (
            <div className="flex flex-col gap-1.5">
              <Skeleton className="h-9" />
              <Skeleton className="h-9" />
            </div>
          ) : domains.length === 0 ? (
            <p className="text-sm text-muted">
              No custom domains. Add one to serve tunnels on your own hostname (paid plans).
            </p>
          ) : (
            <div className="flex flex-col gap-2">
              {domains.map((d) => (
                <DomainRow key={d.id} domain={d} onVerify={() => verify(d.id)} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function DomainRow({ domain, onVerify }: { domain: CustomDomain; onVerify: () => void }) {
  const verified = !!domain.verified_at;
  const [open, setOpen] = useState(!verified);
  return (
    <div className="rounded-md border border-border bg-page">
      <div className="flex items-center gap-2 px-3 py-2">
        <span className="flex-1 truncate font-mono text-sm text-wire">{domain.domain}</span>
        {verified ? (
          <Badge tone="good">
            <CheckCircle2 className="size-3" />
            Verified
          </Badge>
        ) : (
          <Badge tone="warning">
            <Clock className="size-3" />
            Pending DNS
          </Badge>
        )}
        {!verified && (
          <Button variant="secondary" size="sm" onClick={onVerify}>
            Verify
          </Button>
        )}
        <Button variant="ghost" size="sm" onClick={() => setOpen((v) => !v)}>
          {open ? "Hide" : "DNS"}
        </Button>
      </div>
      {open && !verified && (
        <div className="flex flex-col gap-2 border-t border-border p-3 text-xs">
          <p className="text-muted">Add these DNS records at your registrar, then click Verify:</p>
          <DnsRow label="TXT" name={`_trqsh-challenge.${domain.domain}`} value={domain.verify_token} />
          <DnsRow label="CNAME" name={domain.domain} value={BASE} />
        </div>
      )}
    </div>
  );
}

function DnsRow({ label, name, value }: { label: string; name: string; value: string }) {
  return (
    <div className="flex items-center gap-2 rounded border border-border bg-surface px-2 py-1.5">
      <span className="w-12 shrink-0 font-semibold text-secondary">{label}</span>
      <div className="min-w-0 flex-1">
        <p className="truncate font-mono text-muted">{name}</p>
        <p className="truncate font-mono">{value}</p>
      </div>
      <CopyButton value={value} />
    </div>
  );
}
