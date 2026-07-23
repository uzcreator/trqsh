import { site } from "./site";

// Real release artifacts produced by Part 08 (.goreleaser.yaml + release.yml).
// Archive names follow goreleaser's `trqsh_<version>_<os>_<arch>` template (zip on
// Windows); GUI bundle names come from release.yml's signing/upload steps
// (Trqsh.app.zip on macOS, trqsh-gui.exe on Windows). Version is deploy-time env.

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
  /** Signed desktop app bundles (arch-specific where it matters). */
  desktop: Asset[];
  /** Package-manager one-liners for the CLI. */
  cli: InstallSnippet[];
  /** Direct CLI archive downloads. */
  archives: Asset[];
}

export const OS_DOWNLOADS: Record<OSId, OSDownload> = {
  macos: {
    id: "macos",
    name: "macOS",
    tagline: "Signed & notarized · Apple Silicon and Intel",
    desktop: [
      { label: "trqsh for macOS", href: `${base}/Trqsh.app.zip`, arch: "universal", kind: ".app" },
    ],
    cli: [
      { label: "Homebrew", command: "brew install trqsh/tap/trqsh", recommended: true },
      { label: "Shell script", command: installShCommand },
      { label: "MacPorts", command: "sudo port install trqsh" },
    ],
    archives: [
      { label: "Apple Silicon", href: `${base}/trqsh_${v}_darwin_arm64.tar.gz`, arch: "arm64", kind: "tar.gz" },
      { label: "Intel", href: `${base}/trqsh_${v}_darwin_amd64.tar.gz`, arch: "amd64", kind: "tar.gz" },
    ],
  },
  windows: {
    id: "windows",
    name: "Windows",
    tagline: "Authenticode-signed installer · x64 and ARM64",
    desktop: [
      { label: "trqsh for Windows", href: `${base}/trqsh-gui.exe`, arch: "amd64", kind: ".exe" },
    ],
    cli: [
      { label: "winget", command: "winget install trqsh.trqsh", recommended: true },
      { label: "Scoop", command: "scoop install trqsh" },
      { label: "Chocolatey", command: "choco install trqsh" },
    ],
    archives: [
      { label: "Windows x64", href: `${base}/trqsh_${v}_windows_amd64.zip`, arch: "amd64", kind: "zip" },
      { label: "Windows ARM64", href: `${base}/trqsh_${v}_windows_arm64.zip`, arch: "arm64", kind: "zip" },
    ],
  },
  linux: {
    id: "linux",
    name: "Linux",
    tagline: "AppImage, .deb / .rpm, or a static binary",
    desktop: [
      { label: "trqsh AppImage (x86_64)", href: `${base}/trqsh_${v}_linux_amd64.AppImage`, arch: "amd64", kind: "AppImage" },
    ],
    cli: [
      { label: "Shell script", command: installShCommand, recommended: true },
      { label: "Debian / Ubuntu", command: `sudo apt install ./trqsh_${v}_linux_amd64.deb` },
      { label: "Fedora / RHEL", command: `sudo dnf install ./trqsh_${v}_linux_amd64.rpm` },
      { label: "Arch (AUR)", command: "yay -S trqsh-bin" },
    ],
    archives: [
      { label: "Linux x64", href: `${base}/trqsh_${v}_linux_amd64.tar.gz`, arch: "amd64", kind: "tar.gz" },
      { label: "Linux ARM64", href: `${base}/trqsh_${v}_linux_arm64.tar.gz`, arch: "arm64", kind: "tar.gz" },
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

/** Best desktop asset for a detected arch, falling back to the first listed. */
export function desktopFor(os: OSId, arch: Arch | null): Asset {
  const list = OS_DOWNLOADS[os].desktop;
  if (arch) {
    const match = list.find((a) => a.arch === arch || a.arch === "universal");
    if (match) return match;
  }
  return list[0];
}

/** The recommended CLI one-liner for an OS (first `recommended`, else first). */
export function primaryCli(os: OSId): InstallSnippet {
  const list = OS_DOWNLOADS[os].cli;
  return list.find((s) => s.recommended) ?? list[0];
}
