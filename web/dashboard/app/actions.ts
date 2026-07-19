"use server";

import { redirect } from "next/navigation";
import { clearTokens } from "@/lib/session";

export async function logout() {
  await clearTokens();
  redirect("/login");
}
