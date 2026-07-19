// Maps the frozen §8 error taxonomy (pkg/proto/errors.go) to friendly copy.
// Errors cross the Wails boundary as plain strings that embed the ERR_* code
// (e.g. "ERR_QUOTA_BANDWIDTH: monthly limit reached"), so we match on substring.

const MESSAGES: Record<string, string> = {
  ERR_AUTH_REQUIRED: "You need to connect first. Paste an API key to sign in.",
  ERR_AUTH_INVALID: "That API key wasn't accepted. Check it and try again.",
  ERR_VERSION_UNSUPPORTED: "This app is out of date. Please update to continue.",
  ERR_QUOTA_TUNNELS: "You've hit your plan's concurrent-tunnel limit.",
  ERR_QUOTA_BANDWIDTH: "You've used all of this month's bandwidth. Upgrade to keep going.",
  ERR_QUOTA_REQUESTS: "You've hit this month's request limit. Upgrade to keep going.",
  ERR_SUBDOMAIN_TAKEN: "That subdomain is already taken. Try another.",
  ERR_SUBDOMAIN_FORBIDDEN: "That subdomain isn't available on your plan.",
  ERR_DOMAIN_UNVERIFIED: "That custom domain isn't verified yet.",
  ERR_PLAN_FORBIDS: "Your plan doesn't include this feature.",
  ERR_PROTO_UNSUPPORTED: "That protocol isn't supported on your plan.",
  ERR_PORT_UNAVAILABLE: "That remote port is unavailable. Try another.",
  ERR_RATE_LIMITED: "Slow down a moment — you're being rate limited.",
  ERR_UPSTREAM_UNREACHABLE: "Couldn't reach your local service. Is it running?",
  ERR_INTERNAL: "Something went wrong on our side. Please try again.",
};

/** Extract a human message from any thrown value. */
export function friendlyError(err: unknown): string {
  const raw =
    err instanceof Error ? err.message : typeof err === "string" ? err : String(err);
  for (const code of Object.keys(MESSAGES)) {
    if (raw.includes(code)) return MESSAGES[code];
  }
  // Strip a leading "ERR_UNKNOWN: " style prefix if present, else show as-is.
  const cleaned = raw.replace(/^ERR_[A-Z_]+:\s*/, "").trim();
  return cleaned || "Something went wrong. Please try again.";
}

/** The bare ERR_* code, if present — handy for tone/severity decisions. */
export function errorCode(err: unknown): string | null {
  const raw = err instanceof Error ? err.message : String(err);
  const m = raw.match(/ERR_[A-Z_]+/);
  return m ? m[0] : null;
}
