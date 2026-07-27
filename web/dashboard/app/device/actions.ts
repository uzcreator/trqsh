"use server";

import { api } from "@/lib/api";

export interface ApproveState {
  ok?: boolean;
  error?: string;
}

/** Approve a desktop/CLI device-authorization request for the signed-in account.
 *  The API mints an API key for the org and hands it to the polling device. */
export async function approveDeviceAction(_prev: ApproveState, formData: FormData): Promise<ApproveState> {
  const code = String(formData.get("user_code") || "").trim();
  if (!code) return { error: "Enter the code shown in the app." };
  try {
    await api.approveDevice(code);
    return { ok: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Could not approve this device." };
  }
}
