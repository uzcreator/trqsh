import { cookies } from "next/headers";

const ACCESS = "trqsh_access";
const REFRESH = "trqsh_refresh";

// Session cookies are scoped to the whole base domain so the API (api.<base>)
// and the dashboard (app.<base>) share one session. Logout MUST clear the same
// domain — otherwise the `.trqsh.uz` cookie survives and the old account stays
// signed in.
const BASE = process.env.NEXT_PUBLIC_TRQSH_BASE_DOMAIN || "";
const DOMAIN = BASE && BASE !== "localhost" ? "." + BASE : undefined;
const SECURE = process.env.NODE_ENV === "production";

export interface Tokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

/** Read the session tokens from httpOnly cookies (server-side only). */
export async function getTokens(): Promise<{ access?: string; refresh?: string }> {
  const c = await cookies();
  return { access: c.get(ACCESS)?.value, refresh: c.get(REFRESH)?.value };
}

/**
 * Persist tokens as httpOnly cookies scoped to the base domain. Only valid inside
 * a Server Action or Route Handler; calling it during a plain render throws.
 */
export async function setTokens(t: Tokens): Promise<void> {
  const c = await cookies();
  const opts = {
    httpOnly: true,
    secure: SECURE,
    sameSite: "lax" as const,
    path: "/",
    domain: DOMAIN,
    maxAge: 60 * 60 * 24 * 30,
  };
  c.set(ACCESS, t.access_token, opts);
  c.set(REFRESH, t.refresh_token, opts);
}

/** Clear the session (logout) — expires both the domain-scoped and any host-only
 * cookie so a different account can sign in cleanly. */
export async function clearTokens(): Promise<void> {
  const c = await cookies();
  for (const name of [ACCESS, REFRESH]) {
    c.set(name, "", { httpOnly: true, secure: SECURE, sameSite: "lax", path: "/", domain: DOMAIN, maxAge: 0 });
    c.set(name, "", { httpOnly: true, secure: SECURE, sameSite: "lax", path: "/", maxAge: 0 });
  }
}
