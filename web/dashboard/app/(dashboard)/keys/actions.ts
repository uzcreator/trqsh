"use server";

import { revalidatePath } from "next/cache";
import { api } from "@/lib/api";

export interface CreateKeyState {
  key?: string;
  prefix?: string;
  error?: string;
}

export async function createKeyAction(_prev: CreateKeyState, formData: FormData): Promise<CreateKeyState> {
  const name = String(formData.get("name") || "").trim() || "default";
  try {
    const k = await api.createApiKey(name);
    revalidatePath("/keys");
    return { key: k.api_key, prefix: k.prefix };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Could not create key." };
  }
}

export async function revokeKeyAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") || "");
  if (!id) return;
  try {
    await api.revokeApiKey(id);
  } catch {
    /* surfaced on next render */
  }
  revalidatePath("/keys");
}
