/** Human-readable byte size (binary units). */
export function formatBytes(n: number): string {
  if (!n || n < 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
  const v = n / Math.pow(1024, i);
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}

/** Compact number, e.g. 12_500 -> "12.5K". */
export function formatNumber(n: number): string {
  if (n < 1000) return String(n);
  const units = ["", "K", "M", "B"];
  const i = Math.min(Math.floor(Math.log10(n) / 3), units.length - 1);
  const v = n / Math.pow(1000, i);
  return `${v >= 100 ? Math.round(v) : v.toFixed(1)}${units[i]}`;
}

/** ISO timestamp -> short local date. */
export function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "—";
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

/** Relative time, e.g. "3 min ago". */
export function timeAgo(iso?: string | null): string {
  if (!iso) return "never";
  const d = new Date(iso).getTime();
  if (isNaN(d)) return "never";
  const s = Math.round((Date.now() - d) / 1000);
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} hr ago`;
  const days = Math.round(h / 24);
  return `${days} day${days === 1 ? "" : "s"} ago`;
}

/** Go time.Duration (nanoseconds) -> human retention string. */
export function formatRetention(ns: number): string {
  const hours = ns / 3.6e12;
  if (hours >= 24) return `${Math.round(hours / 24)} days`;
  if (hours >= 1) return `${Math.round(hours)} hour${hours === 1 ? "" : "s"}`;
  return "1 hour";
}

/** Cents -> "$8" / "$8.50". */
export function formatPrice(cents: number): string {
  if (!cents) return "$0";
  const dollars = cents / 100;
  return Number.isInteger(dollars) ? `$${dollars}` : `$${dollars.toFixed(2)}`;
}

/** Percentage of a used/limit pair, clamped 0..100; limit 0 = unlimited -> 0. */
export function pctOf(used: number, limit: number): number {
  if (!limit || limit <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round((used / limit) * 100)));
}
