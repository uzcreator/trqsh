"use server";

import { revalidatePath } from "next/cache";
import { api, type DnsInstructions } from "@/lib/api";

export interface ReserveState {
  error?: string;
  ok?: boolean;
}

export async function reserveSubdomainAction(_prev: ReserveState, fd: FormData): Promise<ReserveState> {
  const sub = String(fd.get("subdomain") || "").trim().toLowerCase();
  if (!sub) return { error: "Enter a subdomain." };
  try {
    await api.reserveSubdomain(sub);
    revalidatePath("/domains");
    return { ok: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Could not reserve subdomain." };
  }
}

export async function releaseSubdomainAction(fd: FormData): Promise<void> {
  const id = String(fd.get("id") || "");
  if (!id) return;
  try {
    await api.releaseSubdomain(id);
  } catch {
    /* surfaced on next render */
  }
  revalidatePath("/domains");
}

export interface AddDomainState {
  error?: string;
  dns?: DnsInstructions;
  domain?: string;
}

export async function addDomainAction(_prev: AddDomainState, fd: FormData): Promise<AddDomainState> {
  const domain = String(fd.get("domain") || "").trim().toLowerCase();
  if (!domain.includes(".")) return { error: "Enter a valid domain (e.g. app.example.com)." };
  try {
    const r = await api.addDomain(domain);
    revalidatePath("/domains");
    return { dns: r.dns_instructions, domain: r.domain.domain };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Could not add domain." };
  }
}

export async function verifyDomainAction(fd: FormData): Promise<void> {
  const id = String(fd.get("id") || "");
  if (!id) return;
  try {
    await api.verifyDomain(id);
  } catch {
    /* surfaced on next render */
  }
  revalidatePath("/domains");
}
