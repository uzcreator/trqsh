import fs from "node:fs";
import yaml from "js-yaml";
import { openapiPath } from "./paths";

// Parses the canonical Part 05 OpenAPI (docs/openapi.yaml) at build time and
// shapes it into groups the /docs/api page renders. Reading the same file the
// API is documented from guarantees the reference can't drift from the contract.

export interface ApiParam {
  name: string;
  in: string;
  required: boolean;
  type?: string;
  description?: string;
}
export interface ApiResponse {
  status: string;
  description: string;
}
export interface ApiOperation {
  id: string;
  method: string;
  path: string;
  summary?: string;
  description?: string;
  auth: boolean;
  params: ApiParam[];
  requestBody?: string;
  responses: ApiResponse[];
}
export interface ApiGroup {
  name: string;
  anchor: string;
  operations: ApiOperation[];
}
export interface ApiSpec {
  title: string;
  version: string;
  description?: string;
  servers: string[];
  groups: ApiGroup[];
}

const METHOD_ORDER = ["get", "post", "put", "patch", "delete"];

// Human names for the first path segment, so groups read nicely.
const GROUP_NAMES: Record<string, string> = {
  auth: "Authentication",
  account: "Account",
  "api-keys": "API keys",
  subdomains: "Reserved subdomains",
  domains: "Custom domains",
  usage: "Usage",
  orgs: "Organizations",
  tunnels: "Tunnels",
  plans: "Plans",
  billing: "Billing",
};

function titleCase(s: string): string {
  return s.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function schemaName(ref: unknown): string | undefined {
  if (typeof ref === "string") return ref.split("/").pop();
  if (ref && typeof ref === "object") {
    const r = ref as Record<string, unknown>;
    if (typeof r.$ref === "string") return r.$ref.split("/").pop();
    if (typeof r.type === "string") return r.type;
  }
  return undefined;
}

/** Load + shape the spec. Returns null if the file is missing or unparseable. */
export function loadOpenApi(): ApiSpec | null {
  let doc: Record<string, unknown>;
  try {
    doc = yaml.load(fs.readFileSync(openapiPath, "utf8")) as Record<string, unknown>;
  } catch {
    return null;
  }
  if (!doc || typeof doc !== "object") return null;

  const info = (doc.info ?? {}) as Record<string, string>;
  const servers = Array.isArray(doc.servers)
    ? (doc.servers as { url?: string }[]).map((s) => s.url ?? "").filter(Boolean)
    : [];
  const paths = (doc.paths ?? {}) as Record<string, Record<string, unknown>>;

  const groups = new Map<string, ApiOperation[]>();

  for (const [p, methods] of Object.entries(paths)) {
    const segment = p.split("/").filter(Boolean)[0] ?? "misc";
    for (const method of METHOD_ORDER) {
      const op = methods[method] as Record<string, unknown> | undefined;
      if (!op) continue;

      // security: [] on an operation means public; otherwise the global default (auth).
      const sec = op.security as unknown[] | undefined;
      const auth = !(Array.isArray(sec) && sec.length === 0);

      const params: ApiParam[] = Array.isArray(op.parameters)
        ? (op.parameters as Record<string, unknown>[]).map((pr) => ({
            name: String(pr.name ?? ""),
            in: String(pr.in ?? ""),
            required: Boolean(pr.required),
            type: schemaName(pr.schema),
            description: pr.description ? String(pr.description) : undefined,
          }))
        : [];

      let requestBody: string | undefined;
      const rb = op.requestBody as Record<string, unknown> | undefined;
      if (rb) {
        const content = rb.content as Record<string, { schema?: unknown }> | undefined;
        const json = content?.["application/json"];
        requestBody = schemaName(json?.schema) ?? "object";
      }

      const responses: ApiResponse[] = op.responses
        ? Object.entries(op.responses as Record<string, { description?: string }>).map(([status, r]) => ({
            status,
            description: r?.description ? String(r.description) : "",
          }))
        : [];

      const list = groups.get(segment) ?? [];
      list.push({
        id: `${method}-${p}`.replace(/[^\w]+/g, "-").replace(/^-|-$/g, "").toLowerCase(),
        method: method.toUpperCase(),
        path: p,
        summary: op.summary ? String(op.summary) : undefined,
        description: op.description ? String(op.description) : undefined,
        auth,
        params,
        requestBody,
        responses,
      });
      groups.set(segment, list);
    }
  }

  const orderedKeys = Object.keys(GROUP_NAMES).filter((k) => groups.has(k));
  for (const k of groups.keys()) if (!orderedKeys.includes(k)) orderedKeys.push(k);

  return {
    title: info.title ?? "Rift Control API",
    version: info.version ?? "1.0.0",
    description: info.description,
    servers,
    groups: orderedKeys.map((k) => ({
      name: GROUP_NAMES[k] ?? titleCase(k),
      anchor: k,
      operations: groups.get(k) ?? [],
    })),
  };
}
