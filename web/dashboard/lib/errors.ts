// Maps the frozen §8 error taxonomy (pkg/proto) to friendly copy + an optional
// upgrade CTA, so the dashboard shows actionable messages instead of raw codes.

export interface FriendlyError {
  title: string;
  detail: string;
  upgrade?: boolean; // show an "Upgrade" CTA
}

const MAP: Record<string, FriendlyError> = {
  ERR_AUTH_REQUIRED: { title: "Sign in required", detail: "Your session expired. Please sign in again." },
  ERR_AUTH_INVALID: { title: "Invalid credentials", detail: "That API key or token is not valid." },
  ERR_VERSION_UNSUPPORTED: { title: "Update required", detail: "Your agent is too old for this edge. Update the `trqsh` CLI." },
  ERR_QUOTA_TUNNELS: { title: "Tunnel limit reached", detail: "You've hit the concurrent-tunnel limit for your plan.", upgrade: true },
  ERR_QUOTA_BANDWIDTH: { title: "Bandwidth quota exceeded", detail: "This month's bandwidth allowance is used up.", upgrade: true },
  ERR_QUOTA_REQUESTS: { title: "Request quota exceeded", detail: "This month's request allowance is used up.", upgrade: true },
  ERR_SUBDOMAIN_TAKEN: { title: "Subdomain taken", detail: "That subdomain is already claimed. Try another." },
  ERR_SUBDOMAIN_FORBIDDEN: { title: "Subdomain not allowed", detail: "Reserve this subdomain first, or upgrade for more.", upgrade: true },
  ERR_DOMAIN_UNVERIFIED: { title: "Domain not verified", detail: "Add the DNS records and verify the domain before using it." },
  ERR_PLAN_FORBIDS: { title: "Not on your plan", detail: "This feature requires a paid plan.", upgrade: true },
  ERR_PROTO_UNSUPPORTED: { title: "Unsupported protocol", detail: "That tunnel protocol isn't available here." },
  ERR_PORT_UNAVAILABLE: { title: "Port unavailable", detail: "That remote port is taken. Try another or use an ephemeral one." },
  ERR_RATE_LIMITED: { title: "Slow down", detail: "You're making requests too quickly. Try again shortly." },
  ERR_INTERNAL: { title: "Something went wrong", detail: "An unexpected error occurred. Please try again." },
  ERR_UPSTREAM_UNREACHABLE: { title: "Local service unreachable", detail: "The edge couldn't reach your local service." },
};

export function friendlyError(code?: string, fallback?: string): FriendlyError {
  if (code && MAP[code]) return MAP[code];
  return { title: "Error", detail: fallback || "Request failed. Please try again." };
}
