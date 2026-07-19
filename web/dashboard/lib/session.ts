import { cookies } from "next/headers";

const ACCESS = "rift_access";
const REFRESH = "rift_refresh";

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
 * Persist tokens as httpOnly cookies. Only valid inside a Server Action or Route
 * Handler; calling it during a plain render throws (callers guard with try/catch).
 */
export async function setTokens(t: Tokens): Promise<void> {
  const c = await cookies();
  const base = {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path: "/",
  };
  c.set(ACCESS, t.access_token, { ...base, maxAge: 60 * 60 * 24 * 30 });
  c.set(REFRESH, t.refresh_token, { ...base, maxAge: 60 * 60 * 24 * 30 });
}

/** Clear the session (logout). */
export async function clearTokens(): Promise<void> {
  const c = await cookies();
  c.delete(ACCESS);
  c.delete(REFRESH);
}
