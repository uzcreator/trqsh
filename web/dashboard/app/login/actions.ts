"use server";

import { redirect } from "next/navigation";
import { login, signup } from "@/lib/api";
import { setTokens } from "@/lib/session";

export interface LoginState {
  error?: string;
}

// Dev/email flow: try to sign in; if the user doesn't exist yet, create the
// account. (Production uses OAuth; see the login page.)
export async function authenticate(_prev: LoginState, formData: FormData): Promise<LoginState> {
  const email = String(formData.get("email") || "").trim();
  const name = String(formData.get("name") || "").trim();
  if (!email.includes("@")) return { error: "Enter a valid email address." };

  let ok = false;
  try {
    let res;
    try {
      res = await login(email);
    } catch {
      res = await signup(email, name || email.split("@")[0]);
    }
    await setTokens(res.tokens);
    ok = true;
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Could not sign in." };
  }
  if (ok) redirect("/");
  return { error: "Could not sign in." };
}
