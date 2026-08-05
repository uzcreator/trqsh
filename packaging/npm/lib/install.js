"use strict";

// Downloads the correct prebuilt trqsh binary from the GitHub release, verifies
// its SHA-256 against checksums.txt, and extracts it into ./vendor. Runs as an
// npm `postinstall`; also callable lazily from the launcher if scripts were
// skipped (npm --ignore-scripts). No third-party dependencies.

const fs = require("fs");
const os = require("os");
const path = require("path");
const http = require("http");
const https = require("https");
const crypto = require("crypto");
const { execFileSync } = require("child_process");
const C = require("./common");

function fetch(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 10) return reject(new Error("too many redirects"));
    // https by default; http only for an explicit TRQSH_DOWNLOAD_BASE mirror.
    const transport = url.startsWith("http://") ? http : https;
    transport
      .get(url, { headers: { "User-Agent": "trqsh-npm-installer" } }, (res) => {
        const { statusCode, headers } = res;
        if ([301, 302, 303, 307, 308].includes(statusCode) && headers.location) {
          res.resume();
          return resolve(fetch(new URL(headers.location, url).toString(), redirects + 1));
        }
        if (statusCode !== 200) {
          res.resume();
          return reject(new Error(`HTTP ${statusCode} for ${url}`));
        }
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => resolve(Buffer.concat(chunks)));
      })
      .on("error", reject);
  });
}

function findBinary(dir, name) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      const found = findBinary(p, name);
      if (found) return found;
    } else if (entry.name === name) {
      return p;
    }
  }
  return null;
}

async function verifyChecksum(buf, archive) {
  let sums;
  try {
    sums = (await fetch(C.checksumsUrl)).toString("utf8");
  } catch (e) {
    process.stderr.write(`trqsh: warning — could not fetch checksums.txt (${e.message}); skipping verify\n`);
    return;
  }
  const line = sums
    .split(/\r?\n/)
    .find((l) => l.trim().split(/\s+/).pop() === archive || l.trim().endsWith(`*${archive}`));
  if (!line) {
    process.stderr.write(`trqsh: warning — ${archive} absent from checksums.txt; skipping verify\n`);
    return;
  }
  const want = line.trim().split(/\s+/)[0].toLowerCase();
  const got = crypto.createHash("sha256").update(buf).digest("hex");
  if (want !== got) {
    throw new Error(`checksum mismatch for ${archive}\n  expected ${want}\n  got      ${got}`);
  }
}

async function install() {
  if (fs.existsSync(C.binPath)) return C.binPath;

  const { archive } = C.target();
  const url = C.archiveUrl(archive);
  process.stderr.write(`trqsh: downloading ${archive} (v${C.VERSION})...\n`);

  const buf = await fetch(url);
  if (process.env.TRQSH_SKIP_CHECKSUM !== "1") {
    await verifyChecksum(buf, archive);
  }

  fs.mkdirSync(C.vendorDir, { recursive: true });
  // mkdtempSync (not a predictable pid-based name) avoids a symlink/TOCTOU race
  // in the shared, world-writable system temp dir.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "trqsh-"));
  const tmp = path.join(tmpDir, archive);
  fs.writeFileSync(tmp, buf);
  try {
    if (process.platform === "win32") {
      // PowerShell ships with Windows; more reliable than tar for .zip.
      execFileSync(
        "powershell",
        ["-NoProfile", "-NonInteractive", "-Command", `Expand-Archive -LiteralPath '${tmp}' -DestinationPath '${C.vendorDir}' -Force`],
        { stdio: "ignore" }
      );
    } else {
      execFileSync("tar", ["-xzf", tmp, "-C", C.vendorDir], { stdio: "ignore" });
    }
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }

  if (!fs.existsSync(C.binPath)) {
    const found = findBinary(C.vendorDir, C.binName);
    if (found && found !== C.binPath) fs.renameSync(found, C.binPath);
  }
  if (!fs.existsSync(C.binPath)) {
    throw new Error("trqsh binary not found after extraction");
  }
  if (process.platform !== "win32") fs.chmodSync(C.binPath, 0o755);

  process.stderr.write("trqsh: installed ✓\n");
  return C.binPath;
}

module.exports = { install };

if (require.main === module) {
  install().catch((err) => {
    process.stderr.write(`\ntrqsh: install failed: ${err.message}\n`);
    process.stderr.write(
      "You can download the binary manually from https://github.com/" + C.REPO + "/releases\n"
    );
    process.exit(1);
  });
}
