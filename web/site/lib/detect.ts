import type { Arch, OSId } from "./downloads";

// Client-only platform detection for the smart download experience. OS comes from
// the UA/platform string; CPU architecture prefers the high-entropy
// `navigator.userAgentData` (Chromium) and falls back to sensible per-OS defaults
// where it isn't exposed (Safari/Firefox) — Apple Silicon for modern Macs, x64
// elsewhere. Never blocks render: callers seed a default, then refine on mount.

interface UADataLike {
  platform?: string;
  getHighEntropyValues?: (hints: string[]) => Promise<{ architecture?: string; bitness?: string; platform?: string }>;
}

function uaData(): UADataLike | undefined {
  if (typeof navigator === "undefined") return undefined;
  return (navigator as Navigator & { userAgentData?: UADataLike }).userAgentData;
}

export function detectOS(): OSId {
  if (typeof navigator === "undefined") return "macos";
  const plat = uaData()?.platform ?? "";
  const s = `${plat} ${navigator.userAgent} ${navigator.platform ?? ""}`.toLowerCase();
  if (s.includes("win")) return "windows";
  if (s.includes("mac") || s.includes("iphone") || s.includes("ipad")) return "macos";
  if (s.includes("linux") || s.includes("android") || s.includes("cros")) return "linux";
  return "macos";
}

/** Best-effort CPU arch. Resolves async where high-entropy hints are available. */
export async function detectArch(os: OSId): Promise<Arch> {
  const data = uaData();
  try {
    if (data?.getHighEntropyValues) {
      const hi = await data.getHighEntropyValues(["architecture", "bitness"]);
      const a = (hi.architecture ?? "").toLowerCase();
      if (a.includes("arm")) return "arm64";
      if (a === "x86" || a.includes("amd") || a.includes("x86")) return "amd64";
    }
  } catch {
    /* fall through to heuristics */
  }
  // Heuristic fallback: most Macs sold since 2020 are Apple Silicon.
  return os === "macos" ? "arm64" : "amd64";
}
