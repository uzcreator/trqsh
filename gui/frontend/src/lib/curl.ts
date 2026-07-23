// Build a copy-pasteable cURL command from a captured request or a tunnel.
// A "copy as cURL" affordance is table stakes for a developer tunnel inspector.

import type { CapturedRequest, Tunnel } from "./types";
import { decodeBody } from "./format";

/** Single-quote a value for a POSIX shell, escaping embedded quotes. */
function shq(s: string): string {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}

function requestURL(req: CapturedRequest): string {
  const host = req.host || req.local_addr;
  const scheme = req.proto === "http" ? "https" : "https";
  const path = req.path || "/";
  if (/^https?:\/\//i.test(host)) return host.replace(/\/$/, "") + path;
  return `${scheme}://${host}${path}`;
}

/** A cURL command reproducing a captured request (method, headers, body). */
export function toCurl(req: CapturedRequest): string {
  const parts = [`curl -X ${req.method || "GET"} ${shq(requestURL(req))}`];
  const headers = req.req_headers ?? {};
  for (const [k, v] of Object.entries(headers)) {
    // Skip pseudo/host headers cURL sets itself.
    if (k.toLowerCase() === "content-length") continue;
    parts.push(`-H ${shq(`${k}: ${v}`)}`);
  }
  const body = decodeBody(req.req_body);
  if (body) parts.push(`--data-raw ${shq(body)}`);
  return parts.join(" \\\n  ");
}

/** A cURL command that simply hits a tunnel's public URL. */
export function tunnelCurl(t: Tunnel): string {
  return `curl -i ${shq(t.public_url)}`;
}
