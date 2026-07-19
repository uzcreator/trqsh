"use client";

import { useActionState, useState } from "react";
import { AlertTriangle, KeyRound } from "lucide-react";
import { createKeyAction, revokeKeyAction, type CreateKeyState } from "./actions";
import type { ApiKey } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Table, THead, TBody, TR, TH, TD } from "@/components/ui/table";
import { CopyButton } from "@/components/copy-button";
import { EmptyState } from "@/components/empty-state";
import { formatDate, timeAgo } from "@/lib/format";

export function KeysManager({ keys }: { keys: ApiKey[] }) {
  const [state, action, pending] = useActionState<CreateKeyState, FormData>(createKeyAction, {});
  const [name, setName] = useState("");

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>Create an API key</CardTitle>
          <CardDescription>
            Use it with <code className="font-mono text-xs">rift login --key</code> or the{" "}
            <code className="font-mono text-xs">RIFT_API_KEY</code> env var.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            action={(fd) => {
              action(fd);
              setName("");
            }}
            className="flex flex-wrap items-end gap-3"
          >
            <div className="flex min-w-[200px] flex-1 flex-col gap-1.5">
              <label className="text-sm font-medium" htmlFor="key-name">
                Name
              </label>
              <Input
                id="key-name"
                name="name"
                placeholder="laptop, ci, …"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={pending}>
              {pending ? "Creating…" : "Create key"}
            </Button>
          </form>

          {state.error && <p className="mt-3 text-sm text-critical">{state.error}</p>}

          {state.key && (
            <div className="mt-4 rounded-md border border-good/40 bg-good/10 p-4">
              <div className="mb-2 flex items-center gap-2 text-sm font-medium text-good">
                <AlertTriangle className="h-4 w-4" />
                Copy your key now — it won&apos;t be shown again.
              </div>
              <div className="flex items-center justify-between gap-2 rounded-md border border-border bg-surface px-3 py-2">
                <code className="truncate font-mono text-xs">{state.key}</code>
                <CopyButton value={state.key} />
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {keys.length === 0 ? (
        <EmptyState icon={KeyRound} title="No API keys yet" description="Create one above to connect the CLI or GUI." />
      ) : (
        <Card>
          <Table>
            <THead>
              <TR>
                <TH>Name</TH>
                <TH>Prefix</TH>
                <TH>Last used</TH>
                <TH>Created</TH>
                <TH>Status</TH>
                <TH className="text-right">Actions</TH>
              </TR>
            </THead>
            <TBody>
              {keys.map((k) => (
                <TR key={k.id}>
                  <TD className="font-medium">{k.name || "—"}</TD>
                  <TD className="font-mono text-xs text-secondary">{k.prefix}…</TD>
                  <TD className="text-secondary">{timeAgo(k.last_used_at)}</TD>
                  <TD className="text-secondary">{formatDate(k.created_at)}</TD>
                  <TD>
                    {k.revoked_at ? (
                      <Badge variant="critical">Revoked</Badge>
                    ) : (
                      <Badge variant="good">Active</Badge>
                    )}
                  </TD>
                  <TD className="text-right">
                    {!k.revoked_at && (
                      <form
                        action={revokeKeyAction}
                        onSubmit={(e) => {
                          if (!confirm("Revoke this key? Agents using it will stop connecting.")) e.preventDefault();
                        }}
                      >
                        <input type="hidden" name="id" value={k.id} />
                        <Button type="submit" variant="ghost" size="sm" className="text-critical hover:bg-critical/10">
                          Revoke
                        </Button>
                      </form>
                    )}
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
