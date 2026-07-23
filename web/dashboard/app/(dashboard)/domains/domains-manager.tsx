"use client";

import { useActionState, useState } from "react";
import { Globe, ShieldCheck, Trash2 } from "lucide-react";
import {
  addDomainAction,
  releaseSubdomainAction,
  reserveSubdomainAction,
  verifyDomainAction,
  type AddDomainState,
  type ReserveState,
} from "./actions";
import type { CustomDomain, ReservedSubdomain } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { CopyButton } from "@/components/copy-button";
import { formatDate } from "@/lib/format";

export function DomainsManager({
  subdomains,
  domains,
  baseDomain,
}: {
  subdomains: ReservedSubdomain[];
  domains: CustomDomain[];
  baseDomain: string;
}) {
  const [subState, reserveAction, reserving] = useActionState<ReserveState, FormData>(reserveSubdomainAction, {});
  const [domState, addAction, adding] = useActionState<AddDomainState, FormData>(addDomainAction, {});

  return (
    <div className="flex flex-col gap-6">
      {/* Reserved subdomains */}
      <Card>
        <CardHeader>
          <CardTitle>Reserved subdomains</CardTitle>
          <CardDescription>
            Claim a stable name like <span className="font-mono text-xs">myapp.{baseDomain}</span>.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form action={reserveAction} className="flex flex-wrap items-end gap-3">
            <div className="flex min-w-[220px] flex-1 items-center rounded-md border border-border-strong bg-surface pr-3 focus-within:ring-2 focus-within:ring-ring">
              <Input name="subdomain" placeholder="myapp" className="border-0 focus-visible:ring-0" />
              <span className="whitespace-nowrap text-sm text-muted">.{baseDomain}</span>
            </div>
            <Button type="submit" disabled={reserving}>
              {reserving ? "Reserving…" : "Reserve"}
            </Button>
          </form>
          {subState.error && <p className="mt-3 text-sm text-critical">{subState.error}</p>}

          <div className="mt-4 divide-y divide-border">
            {subdomains.length === 0 ? (
              <p className="py-4 text-sm text-secondary">No reserved subdomains yet.</p>
            ) : (
              subdomains.map((s) => (
                <div key={s.id} className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-2">
                    <Globe className="h-4 w-4 text-muted" />
                    <span className="font-mono text-sm">
                      {s.subdomain}.{baseDomain}
                    </span>
                  </div>
                  <form
                    action={releaseSubdomainAction}
                    onSubmit={(e) => {
                      if (!confirm(`Release ${s.subdomain}.${baseDomain}?`)) e.preventDefault();
                    }}
                  >
                    <input type="hidden" name="id" value={s.id} />
                    <Button type="submit" variant="ghost" size="icon" className="text-critical hover:bg-critical/10">
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </form>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>

      {/* Custom domains */}
      <Card>
        <CardHeader>
          <CardTitle>Custom domains</CardTitle>
          <CardDescription>Bring your own domain. Add the DNS records, then verify.</CardDescription>
        </CardHeader>
        <CardContent>
          <form action={addAction} className="flex flex-wrap items-end gap-3">
            <div className="flex min-w-[220px] flex-1 flex-col gap-1.5">
              <Input name="domain" placeholder="app.example.com" />
            </div>
            <Button type="submit" disabled={adding}>
              {adding ? "Adding…" : "Add domain"}
            </Button>
          </form>
          {domState.error && <p className="mt-3 text-sm text-critical">{domState.error}</p>}
          {domState.dns && (
            <div className="mt-4 rounded-md border border-border bg-page p-4">
              <p className="mb-3 text-sm font-medium">Add these DNS records for {domState.domain}:</p>
              <DnsRow type="TXT" name={domState.dns.txt_name} value={domState.dns.txt_value} />
              <DnsRow type="CNAME" name={domState.dns.cname_name} value={domState.dns.cname_value} />
            </div>
          )}

          <div className="mt-4 divide-y divide-border">
            {domains.length === 0 ? (
              <p className="py-4 text-sm text-secondary">No custom domains yet.</p>
            ) : (
              domains.map((d) => <DomainRow key={d.id} domain={d} baseDomain={baseDomain} />)
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function DomainRow({ domain, baseDomain }: { domain: CustomDomain; baseDomain: string }) {
  const [open, setOpen] = useState(false);
  const verified = !!domain.verified_at;
  return (
    <div className="py-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Globe className="h-4 w-4 text-muted" />
          <span className="font-mono text-sm">{domain.domain}</span>
          {verified ? (
            <Badge variant="good">
              <ShieldCheck className="h-3 w-3" /> Verified
            </Badge>
          ) : (
            <Badge variant="warning">Pending</Badge>
          )}
          <Badge variant="muted">cert: {domain.cert_status}</Badge>
        </div>
        <div className="flex items-center gap-2">
          {!verified && (
            <Button type="button" variant="ghost" size="sm" onClick={() => setOpen((v) => !v)}>
              {open ? "Hide DNS" : "Show DNS"}
            </Button>
          )}
          {!verified && (
            <form action={verifyDomainAction}>
              <input type="hidden" name="id" value={domain.id} />
              <Button type="submit" variant="outline" size="sm">
                Verify
              </Button>
            </form>
          )}
        </div>
      </div>
      {!verified && open && (
        <div className="mt-3 rounded-md border border-border bg-page p-4">
          <DnsRow type="TXT" name={`_trqsh-challenge.${domain.domain}`} value={domain.verify_token} />
          <DnsRow type="CNAME" name={domain.domain} value={baseDomain} />
        </div>
      )}
    </div>
  );
}

function DnsRow({ type, name, value }: { type: string; name: string; value: string }) {
  return (
    <div className="flex flex-wrap items-center gap-2 py-1.5 text-xs">
      <Badge variant="outline" className="w-16 justify-center">
        {type}
      </Badge>
      <span className="font-mono text-secondary">{name}</span>
      <span className="text-muted">→</span>
      <span className="truncate font-mono text-secondary">{value}</span>
      <CopyButton value={value} label="" className="ml-auto" />
    </div>
  );
}
