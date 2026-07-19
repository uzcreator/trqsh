import type { Metadata } from "next";
import { Lock, Unlock } from "lucide-react";
import { loadOpenApi, type ApiOperation } from "@/lib/openapi";
import { CodeBlock } from "@/components/code-block";
import { cn } from "@/lib/utils";

export const metadata: Metadata = {
  title: "API reference",
  description: "The Rift Control API, generated from the canonical OpenAPI spec.",
};

const METHOD_STYLES: Record<string, string> = {
  GET: "bg-series-2/15 text-series-2",
  POST: "bg-series-1/15 text-series-1",
  PUT: "bg-series-3/20 text-serious",
  PATCH: "bg-series-3/20 text-serious",
  DELETE: "bg-critical/15 text-critical",
};

export default function ApiReferencePage() {
  const spec = loadOpenApi();

  if (!spec) {
    return (
      <div className="py-10">
        <h1 className="text-3xl font-semibold tracking-tight text-foreground">API reference</h1>
        <p className="mt-4 rounded-lg border border-warning/40 bg-warning/10 p-4 text-sm text-serious">
          The OpenAPI document (docs/openapi.yaml) could not be loaded at build time. The reference
          renders from that file — check that it exists and is valid YAML.
        </p>
      </div>
    );
  }

  return (
    <div className="grid gap-10 py-10 xl:grid-cols-[minmax(0,1fr)_13rem]">
      <div className="min-w-0">
        <h1 className="text-3xl font-semibold tracking-tight text-foreground">{spec.title}</h1>
        <p className="mt-2 text-sm text-muted tabular">Version {spec.version}</p>
        {spec.description && <p className="mt-3 max-w-2xl text-secondary">{spec.description}</p>}

        <div className="mt-5 rounded-lg border border-border bg-surface p-4">
          <div className="text-xs font-semibold uppercase tracking-wide text-muted">Base URL</div>
          {spec.servers.map((s) => (
            <div key={s} className="mt-1 font-mono text-sm text-foreground">
              {s}
            </div>
          ))}
          <p className="mt-3 text-sm text-secondary">
            Authenticate with a <span className="font-medium text-foreground">Bearer JWT</span>{" "}
            (dashboard) or an <span className="font-medium text-foreground">API key</span>{" "}
            (programmatic). Endpoints marked <Lock className="inline h-3.5 w-3.5" /> require auth.
          </p>
        </div>

        <div className="mt-10 flex flex-col gap-12">
          {spec.groups.map((group) => (
            <section key={group.anchor} id={group.anchor} className="scroll-mt-24">
              <h2 className="mb-4 border-b border-border pb-2 text-xl font-semibold tracking-tight text-foreground">
                {group.name}
              </h2>
              <div className="flex flex-col gap-4">
                {group.operations.map((op) => (
                  <Operation key={op.id} op={op} />
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>

      <aside className="hidden xl:block">
        <div className="sticky top-24">
          <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">Endpoints</div>
          <ul className="flex flex-col gap-1 border-l border-border">
            {spec.groups.map((g) => (
              <li key={g.anchor}>
                <a
                  href={`#${g.anchor}`}
                  className="-ml-px block border-l-2 border-transparent py-0.5 pl-3 text-sm text-secondary transition-colors hover:border-primary hover:text-foreground"
                >
                  {g.name}
                </a>
              </li>
            ))}
          </ul>
        </div>
      </aside>
    </div>
  );
}

function Operation({ op }: { op: ApiOperation }) {
  return (
    <div id={op.id} className="scroll-mt-24 overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
      <div className="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
        <span
          className={cn(
            "rounded px-2 py-0.5 font-mono text-xs font-semibold",
            METHOD_STYLES[op.method] ?? "bg-accent text-primary"
          )}
        >
          {op.method}
        </span>
        <span className="font-mono text-sm text-foreground">{op.path}</span>
        <span className="ml-auto text-muted" title={op.auth ? "Requires authentication" : "Public"}>
          {op.auth ? <Lock className="h-4 w-4" /> : <Unlock className="h-4 w-4" />}
        </span>
      </div>
      <div className="px-4 py-4">
        {op.summary && <p className="text-sm font-medium text-foreground">{op.summary}</p>}
        {op.description && <p className="mt-1 text-sm text-secondary">{op.description}</p>}

        {op.params.length > 0 && (
          <div className="mt-4">
            <div className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-muted">Parameters</div>
            <div className="overflow-x-auto rounded-md border border-border">
              <table className="w-full text-sm">
                <tbody>
                  {op.params.map((p) => (
                    <tr key={`${p.in}-${p.name}`} className="border-b border-border last:border-0">
                      <td className="px-3 py-2 font-mono text-xs text-foreground">{p.name}</td>
                      <td className="px-3 py-2 text-xs text-muted">{p.in}</td>
                      <td className="px-3 py-2 text-xs text-secondary">{p.type ?? "—"}</td>
                      <td className="px-3 py-2 text-xs">
                        {p.required ? <span className="text-critical">required</span> : <span className="text-muted">optional</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {op.requestBody && (
          <p className="mt-4 text-sm text-secondary">
            <span className="text-xs font-semibold uppercase tracking-wide text-muted">Request body</span>
            <br />
            <code className="font-mono text-xs text-primary">{op.requestBody}</code> (application/json)
          </p>
        )}

        {op.responses.length > 0 && (
          <div className="mt-4">
            <div className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-muted">Responses</div>
            <ul className="flex flex-col gap-1">
              {op.responses.map((r) => (
                <li key={r.status} className="flex items-baseline gap-2 text-sm">
                  <span
                    className={cn(
                      "rounded px-1.5 py-0.5 font-mono text-xs font-medium",
                      r.status.startsWith("2") ? "bg-good/15 text-good" : "bg-warning/20 text-serious"
                    )}
                  >
                    {r.status}
                  </span>
                  <span className="text-secondary">{r.description}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
  );
}
