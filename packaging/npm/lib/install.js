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

// PowerShell source for ensureOnPath below. Static text only — the directory
// value flows in via the TRQSH_PATH_DIR env var (see the existing
// Expand-Archive call above for why: a Windows path can contain a literal
// single quote, e.g. a username like O'Brien, which would break a naively
// interpolated -Command string).
const PATH_FIX_SCRIPT = [
  "$dir = $env:TRQSH_PATH_DIR",
  "$cur = [Environment]::GetEnvironmentVariable('Path','User')",
  "if (-not $cur) { $cur = '' }",
  "$already = $false",
  "foreach ($p in ($cur -split ';')) { if ($p -and ($p.TrimEnd('\\') -ieq $dir.TrimEnd('\\'))) { $already = $true } }",
  "if (-not $already) {",
  "  $new = if ($cur.Trim().Length -gt 0) { $cur.TrimEnd(';') + ';' + $dir } else { $dir }",
  "  [Environment]::SetEnvironmentVariable('Path', $new, 'User')",
  "  Add-Type -Namespace TrqshWin32 -Name NativeMethods -MemberDefinition '[DllImport(\"user32.dll\", SetLastError = true, CharSet = CharSet.Auto)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);'",
  "  $result = [UIntPtr]::Zero",
  "  [TrqshWin32.NativeMethods]::SendMessageTimeout([IntPtr]0xffff, 0x1a, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null",
  "  Write-Output 'ADDED'",
  "} else {",
  "  Write-Output 'ALREADY'",
  "}",
].join("\n");

// ensureOnPath fixes the #1 reported install failure: `npm install -g` links
// the `trqsh` shim into npm's global bin dir, but on plenty of Windows setups
// (custom prefix, portable Node zips, some nvm-windows configs...) that dir
// was never added to PATH, so the freshly installed command isn't found.
// Only runs for a real `-g` install (npm_config_global) on win32 — a local
// `npm i` puts the shim in ./node_modules/.bin, which is never meant to be
// on PATH (that's what npx / package scripts are for), so there's nothing
// to fix there. Broadcasts WM_SETTINGCHANGE after the registry write so a
// terminal opened right after (which inherits Explorer's cached environment
// block) picks up the change without a full logoff.
function ensureOnPath() {
  if (process.platform !== "win32" || process.env.npm_config_global !== "true") return;

  let dir;
  try {
    dir = (process.env.npm_config_prefix || execFileSync("npm", ["config", "get", "prefix"], { encoding: "utf8" })).trim();
  } catch {
    return; // best-effort — never fail the install over a PATH nicety
  }
  if (!dir) return;

  const norm = (p) => p.replace(/\\+$/, "").toLowerCase();
  const already = (process.env.PATH || "").split(path.delimiter).some((p) => p && norm(p) === norm(dir));
  if (already) return;

  try {
    const out = execFileSync(
      "powershell",
      ["-NoProfile", "-NonInteractive", "-Command", PATH_FIX_SCRIPT],
      { encoding: "utf8", env: { ...process.env, TRQSH_PATH_DIR: dir } }
    );
    if (out.includes("ADDED")) {
      process.stderr.write(`trqsh: added ${dir} to your PATH — open a new terminal to use the trqsh command\n`);
    }
  } catch (e) {
    process.stderr.write(
      `trqsh: warning — couldn't update PATH automatically (${e.message}); if 'trqsh' isn't found, add ${dir} to your PATH manually\n`
    );
  }
}

async function install() {
  if (fs.existsSync(C.binPath)) {
    ensureOnPath();
    return C.binPath;
  }

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
      //
      // The archive/dest paths are passed through environment variables and
      // read back as $env:... inside a fixed command string, rather than
      // interpolated into the -Command text directly. A Windows path can
      // legitimately contain a single quote (e.g. a username like O'Brien,
      // or a custom npm prefix), and PowerShell's quoting rules aren't
      // limited to ASCII quote characters either (curly/smart Unicode quotes
      // can also close a quoted string), so naively doubling `'` characters
      // isn't a reliable escape. Env vars carry the values as plain data,
      // with no command text built from them at all.
      execFileSync(
        "powershell",
        ["-NoProfile", "-NonInteractive", "-Command", "Expand-Archive -LiteralPath $env:TRQSH_INSTALL_SRC -DestinationPath $env:TRQSH_INSTALL_DEST -Force"],
        { stdio: "ignore", env: { ...process.env, TRQSH_INSTALL_SRC: tmp, TRQSH_INSTALL_DEST: C.vendorDir } }
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
  ensureOnPath();
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
