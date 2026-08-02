"use strict";

// Shared configuration for the trqsh npm wrapper: where the prebuilt binary lives
// and how to build the GitHub release URLs. The archive names mirror the
// goreleaser template `trqsh_<version>_<os>_<arch>.<ext>` (.zip on Windows).

const path = require("path");
const pkg = require("../package.json");

const REPO = process.env.TRQSH_REPO || "uzcreator/trqsh";
const VERSION = process.env.TRQSH_VERSION || pkg.version;

/** Map Node's platform/arch onto goreleaser's os/arch + archive extension. */
function target() {
  const goos = { darwin: "darwin", linux: "linux", win32: "windows" }[process.platform];
  const goarch = { x64: "amd64", arm64: "arm64" }[process.arch];
  if (!goos || !goarch) {
    throw new Error(
      `trqsh: unsupported platform ${process.platform}/${process.arch}. ` +
        "Download manually from https://github.com/" + REPO + "/releases"
    );
  }
  const ext = goos === "windows" ? "zip" : "tar.gz";
  return { goos, goarch, ext, archive: `trqsh_${VERSION}_${goos}_${goarch}.${ext}` };
}

const binName = process.platform === "win32" ? "trqsh.exe" : "trqsh";
const vendorDir = path.join(__dirname, "..", "vendor");
const binPath = path.join(vendorDir, binName);
// Where release assets live. Overridable (TRQSH_DOWNLOAD_BASE) for mirrors,
// air-gapped installs, or testing against a local server.
const baseUrl =
  process.env.TRQSH_DOWNLOAD_BASE || `https://github.com/${REPO}/releases/download/v${VERSION}`;

module.exports = {
  REPO,
  VERSION,
  target,
  binName,
  vendorDir,
  binPath,
  baseUrl,
  archiveUrl: (archive) => `${baseUrl}/${archive}`,
  checksumsUrl: `${baseUrl}/checksums.txt`,
};
