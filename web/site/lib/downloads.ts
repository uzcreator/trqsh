import { site } from "./site";

// Real release artifacts produced by Part 08 (.goreleaser.yaml + release.yml).
// Archive names follow goreleaser's `rift_<version>_<os>_<arch>` template (zip on
// Windows); GUI bundle names come from release.yml's signing/upload steps
// (Rift.app.zip on macOS, rift-gui.exe on Windows). Version is deploy-time env.

const v = site.version;
const base = `${site.githubUrl}/releases/download/v${v}`;

export const releasesUrl = `${site.githubUrl}/releases/latest`;
export const checksumsUrl = `${base}/checksums.txt`;
export const installShCommand = `curl -fsSL ${site.installShUrl} | sh`;

export type OSId = "macos" | "windows" | "linux";

export interface Asset {
  label: string;
  href: string;
  arch?: string;
}
export interface InstallSnippet {
  label: string;
  command: string;
  prompt?: boolean;
}
export interface OSDownload {
  id: OSId;
  name: string;
  /** Signed desktop app bundles. */
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
    desktop: [{ label: "Rift for macOS (Apple Silicon & Intel)", href: `${base}/Rift.app.zip`, arch: "universal" }],
    cli: [
      { label: "Homebrew", command: "brew install rift/tap/rift" },
      { label: "curl", command: installShCommand },
    ],
    archives: [
      { label: "macOS · Apple Silicon", href: `${base}/rift_${v}_darwin_arm64.tar.gz`, arch: "arm64" },
      { label: "macOS · Intel", href: `${base}/rift_${v}_darwin_amd64.tar.gz`, arch: "amd64" },
    ],
  },
  windows: {
    id: "windows",
    name: "Windows",
    desktop: [{ label: "Rift for Windows (Authenticode-signed)", href: `${base}/rift-gui.exe`, arch: "x64" }],
    cli: [{ label: "Scoop", command: "scoop install rift" }],
    archives: [
      { label: "Windows · x64", href: `${base}/rift_${v}_windows_amd64.zip`, arch: "amd64" },
      { label: "Windows · ARM64", href: `${base}/rift_${v}_windows_arm64.zip`, arch: "arm64" },
    ],
  },
  linux: {
    id: "linux",
    name: "Linux",
    desktop: [{ label: "Rift AppImage & .deb — on the Releases page", href: releasesUrl }],
    cli: [{ label: "curl", command: installShCommand }],
    archives: [
      { label: "Linux · x64", href: `${base}/rift_${v}_linux_amd64.tar.gz`, arch: "amd64" },
      { label: "Linux · ARM64", href: `${base}/rift_${v}_linux_arm64.tar.gz`, arch: "arm64" },
    ],
  },
};

export const OS_ORDER: OSId[] = ["macos", "windows", "linux"];
