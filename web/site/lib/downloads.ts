import { site } from "./site";

// Release artifacts published to the public trqsh-uz/downloads repo by
// .goreleaser.yaml. Archive names follow `trqsh_<version>_<os>_<arch>` (zip on
// Windows). The desktop GUI isn't part of this release, so the page is CLI-first:
// an install script, the npm/pip wrappers, distro packages, and static binaries.

const v = site.version;
const base = `${site.githubUrl}/releases/download/v${v}`;

export const releasesUrl = `${site.githubUrl}/releases/latest`;
export const checksumsUrl = `${base}/checksums.txt`;
export const installShCommand = `curl -fsSL ${site.installShUrl} | sh`;

export type OSId = "macos" | "windows" | "linux";
export type Arch = "arm64" | "amd64";

export interface Asset {
  label: string;
  href: string;
  arch?: Arch | "universal";
  /** File extension / kind shown as a chip. */
  kind?: string;
}
export interface InstallSnippet {
  label: string;
  command: string;
  /** The most idiomatic path for this OS — highlighted first. */
  recommended?: boolean;
}
export interface OSDownload {
  id: OSId;
  name: string;
  /** One-liner shown under the OS name on the download page. */
  tagline: string;
  /** Signed desktop app bundles — empty until the GUI ships. */
  desktop: Asset[];
  /** Package-manager / script one-liners for the CLI. */
  cli: InstallSnippet[];
  /** Direct binary + distro-package downloads. */
  archives: Asset[];
}

const npmSnippet: InstallSnippet = { label: "npm", command: "npm install -g @trqsh-uz/trqsh" };
const pipSnippet: InstallSnippet = { label: "pipx", command: "pipx install trqsh" };
const scriptSnippet: InstallSnippet = { label: "Install script", command: installShCommand, recommended: true };

export const OS_DOWNLOADS: Record<OSId, OSDownload> = {
  macos: {
    id: "macos",
    name: "macOS",
    tagline: "Apple Silicon and Intel · static binary",
    desktop: [],
    cli: [scriptSnippet, npmSnippet, pipSnippet],
    archives: [
      { label: "Apple Silicon", href: `${base}/trqsh_${v}_darwin_arm64.tar.gz`, arch: "arm64", kind: "tar.gz" },
      { label: "Intel", href: `${base}/trqsh_${v}_darwin_amd64.tar.gz`, arch: "amd64", kind: "tar.gz" },
    ],
  },
  windows: {
    id: "windows",
    name: "Windows",
    tagline: "x64 and ARM64 · static binary",
    desktop: [],
    cli: [{ ...npmSnippet, recommended: true }, pipSnippet],
    archives: [
      { label: "Windows x64", href: `${base}/trqsh_${v}_windows_amd64.zip`, arch: "amd64", kind: "zip" },
      { label: "Windows ARM64", href: `${base}/trqsh_${v}_windows_arm64.zip`, arch: "arm64", kind: "zip" },
    ],
  },
  linux: {
    id: "linux",
    name: "Linux",
    tagline: "Static binary, .deb / .rpm, or install script",
    desktop: [],
    cli: [scriptSnippet, npmSnippet, pipSnippet],
    archives: [
      { label: "Linux x64", href: `${base}/trqsh_${v}_linux_amd64.tar.gz`, arch: "amd64", kind: "tar.gz" },
      { label: "Linux ARM64", href: `${base}/trqsh_${v}_linux_arm64.tar.gz`, arch: "arm64", kind: "tar.gz" },
      { label: "Debian x64", href: `${base}/trqsh_${v}_linux_amd64.deb`, arch: "amd64", kind: "deb" },
      { label: "RPM x64", href: `${base}/trqsh_${v}_linux_amd64.rpm`, arch: "amd64", kind: "rpm" },
    ],
  },
};

export const OS_ORDER: OSId[] = ["macos", "windows", "linux"];

export const OS_LABEL: Record<OSId, string> = {
  macos: "macOS",
  windows: "Windows",
  linux: "Linux",
};

export const ARCH_LABEL: Record<Arch, string> = {
  arm64: "Apple Silicon / ARM64",
  amd64: "Intel / x64",
};

/** The direct binary archive matching a detected arch (falls back to the first). */
export function archiveFor(os: OSId, arch: Arch | null): Asset {
  const list = OS_DOWNLOADS[os].archives;
  if (arch) {
    const match = list.find((a) => a.arch === arch);
    if (match) return match;
  }
  return list[0];
}

/** The recommended CLI one-liner for an OS (first `recommended`, else first). */
export function primaryCli(os: OSId): InstallSnippet {
  const list = OS_DOWNLOADS[os].cli;
  return list.find((s) => s.recommended) ?? list[0];
}
