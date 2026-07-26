// Small presentation helpers shared across screens. Kept dependency-free so the
// same logic can be lifted into web/dashboard if we ever converge the two.

const KB = 1024;
const MB = KB * 1024;
const GB = MB * 1024;
const TB = GB * 1024;

/** Human-readable byte count, e.g. 1536 -> "1.5 KB". */
export function bytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "0 B";
  if (n < KB) return `${n} B`;
  if (n < MB) return `${(n / KB).toFixed(1)} KB`;
  if (n < GB) return `${(n / MB).toFixed(1)} MB`;
  if (n < TB) return `${(n / GB).toFixed(2)} GB`;
  return `${(n / TB).toFixed(2)} TB`;
}

/** Compact integer with thousands separators, e.g. 12345 -> "12,345". */
export function count(n: number): string {
  if (!Number.isFinite(n)) return "0";
  return Math.round(n).toLocaleString("en-US");
}

/** Duration in ms rendered as ms / s. */
export function duration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}

/** Clock time (HH:MM:SS) from an ISO timestamp or Date. */
export function clock(t: string | Date): string {
  const d = typeof t === "string" ? new Date(t) : t;
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleTimeString("en-US", { hour12: false });
}

/** HTTP status → semantic status token used by the palette. */
export function statusTone(status: number): "good" | "warning" | "serious" | "critical" | "muted" {
  if (status === 0) return "muted";
  if (status < 300) return "good";
  if (status < 400) return "warning";
  if (status < 500) return "serious";
  return "critical";
}

/** Decode a Go []byte JSON field (base64) into a UTF-8 string for display. */
export function decodeBody(b64: string | undefined | null): string {
  if (!b64) return "";
  try {
    const bin = atob(b64);
    const bytesArr = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    return new TextDecoder("utf-8", { fatal: false }).decode(bytesArr);
  } catch {
    return "";
  }
}
