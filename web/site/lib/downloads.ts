import { site } from "./site";

// CLI archives and desktop installers both ship as GitHub Releases on the main
// uzcreator/trqsh repo (`.goreleaser.yaml` for the CLI, `desktop-build.yml` for the
// app). CLI releases are tagged `v<version>`, desktop releases `desktop-v<version>`.

const v = site.version;
const base = `${site.githubUrl}/releases/download/v${v}`;

// The desktop app (Tauri) versions independently of the CLI; its assets live under
// the `desktop-v<version>` tag. Asset names follow Tauri's bundler convention.
const dv = process.env.TRQSH_DESKTOP_VERSION || "0.3.1";
const guiBase = `${site.githubUrl}/releases/download/desktop-v${dv}`;

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
const scoopSnippet: InstallSnippet = { label: "Scoop", command: "scoop bucket add trqsh https://github.com/uzcreator/scoop-bucket; scoop install trqsh" };
const scriptSnippet: InstallSnippet = { label: "Install script", command: installShCommand, recommended: true };

export const OS_DOWNLOADS: Record<OSId, OSDownload> = {
  macos: {
    id: "macos",
    name: "macOS",
    tagline: "Native desktop app · or CLI",
    desktop: [
      { label: "Apple Silicon", href: `${guiBase}/trqsh_${dv}_aarch64.dmg`, arch: "arm64", kind: "dmg" },
    ],
    cli: [scriptSnippet, npmSnippet, pipSnippet],
    archives: [
      { label: "Apple Silicon", href: `${base}/trqsh_${v}_darwin_arm64.tar.gz`, arch: "arm64", kind: "tar.gz" },
      { label: "Intel", href: `${base}/trqsh_${v}_darwin_amd64.tar.gz`, arch: "amd64", kind: "tar.gz" },
    ],
  },
  windows: {
    id: "windows",
    name: "Windows",
    tagline: "Desktop app · or CLI via npm / Scoop",
    desktop: [
      { label: "Windows installer", href: `${guiBase}/trqsh_${dv}_x64-setup.exe`, arch: "amd64", kind: "exe" },
      { label: "MSI package", href: `${guiBase}/trqsh_${dv}_x64_en-US.msi`, arch: "amd64", kind: "msi" },
    ],
    cli: [{ ...npmSnippet, recommended: true }, scoopSnippet, pipSnippet],
    archives: [
      { label: "Windows x64", href: `${base}/trqsh_${v}_windows_amd64.zip`, arch: "amd64", kind: "zip" },
      { label: "Windows ARM64", href: `${base}/trqsh_${v}_windows_arm64.zip`, arch: "arm64", kind: "zip" },
    ],
  },
  linux: {
    id: "linux",
    name: "Linux",
    tagline: "Desktop app (AppImage / deb / rpm) · or CLI",
    desktop: [
      { label: "AppImage", href: `${guiBase}/trqsh_${dv}_amd64.AppImage`, arch: "amd64", kind: "AppImage" },
      { label: "Debian / Ubuntu", href: `${guiBase}/trqsh_${dv}_amd64.deb`, arch: "amd64", kind: "deb" },
      { label: "Fedora / RHEL", href: `${guiBase}/trqsh-${dv}-1.x86_64.rpm`, arch: "amd64", kind: "rpm" },
    ],
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

/** The headline download for an OS: the desktop app if it ships there, else the CLI archive. */
export function primaryDownload(os: OSId, arch: Arch | null): Asset {
  const desk = OS_DOWNLOADS[os].desktop;
  if (desk.length > 0) return desk[0];
  return archiveFor(os, arch);
}

/** The recommended CLI one-liner for an OS (first `recommended`, else first). */
export function primaryCli(os: OSId): InstallSnippet {
  const list = OS_DOWNLOADS[os].cli;
  return list.find((s) => s.recommended) ?? list[0];
}
