"""Locate (or download) the prebuilt trqsh binary for the current platform.

The archive names mirror the goreleaser template
``trqsh_<version>_<os>_<arch>.<ext>`` (.zip on Windows, .tar.gz elsewhere),
downloaded from the GitHub release and verified against ``checksums.txt``. Only
the Python standard library is used.
"""

from __future__ import annotations

import hashlib
import os
import platform
import site
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path

from . import __version__

# CLI releases publish to their own dedicated repo (source stays in
# uzcreator/trqsh; see .goreleaser.yaml).
REPO = os.environ.get("TRQSH_REPO", "uzcreator/trqshcli")
VERSION = os.environ.get("TRQSH_VERSION", __version__)


def _target() -> tuple[str, str, str]:
    goos = {"darwin": "darwin", "linux": "linux", "windows": "windows"}.get(
        platform.system().lower()
    )
    goarch = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }.get(platform.machine().lower())
    if not goos or not goarch:
        raise SystemExit(
            f"trqsh: unsupported platform {platform.system()}/{platform.machine()}. "
            f"Download manually from https://github.com/{REPO}/releases"
        )
    ext = "zip" if goos == "windows" else "tar.gz"
    return goos, goarch, ext


def _bin_name() -> str:
    return "trqsh.exe" if platform.system() == "Windows" else "trqsh"


def _cache_dir() -> Path:
    base = os.environ.get("XDG_CACHE_HOME") or (Path.home() / ".cache")
    d = Path(base) / "trqsh" / VERSION
    d.mkdir(parents=True, exist_ok=True)
    return d


def bin_path() -> Path:
    return _cache_dir() / _bin_name()


def _download(url: str) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": "trqsh-pypi-installer"})
    with urllib.request.urlopen(req) as resp:  # noqa: S310 (trusted GitHub host)
        return resp.read()


def _verify(data: bytes, archive: str, base: str) -> None:
    if os.environ.get("TRQSH_SKIP_CHECKSUM") == "1":
        return
    try:
        sums = _download(f"{base}/checksums.txt").decode()
    except Exception as exc:  # noqa: BLE001
        print(f"trqsh: warning — could not fetch checksums.txt ({exc}); skipping verify", file=sys.stderr)
        return
    want = None
    for line in sums.splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1].lstrip("*") == archive:
            want = parts[0].lower()
            break
    if not want:
        print(f"trqsh: warning — {archive} absent from checksums.txt; skipping verify", file=sys.stderr)
        return
    got = hashlib.sha256(data).hexdigest()
    if got != want:
        raise SystemExit(f"trqsh: checksum mismatch for {archive}\n  expected {want}\n  got      {got}")


def _check_member_path(name: str, dest: Path) -> None:
    """Reject an archive member whose path would land outside dest (zip-slip /
    path traversal via ``../`` or an absolute path). zipfile.extractall() has
    no such guard at all, and tarfile's own guard (``filter="data"``) only
    exists on Python 3.12+, so this is applied explicitly to both formats
    rather than relying on either library's default behavior.
    """
    target = os.path.realpath(os.path.join(dest, name))
    dest_real = os.path.realpath(dest)
    if os.path.commonpath([dest_real, target]) != dest_real:
        raise SystemExit(f"trqsh: refusing to extract {name!r} outside {dest}")


def _extract(archive_path: Path, ext: str, dest: Path) -> None:
    if ext == "zip":
        with zipfile.ZipFile(archive_path) as zf:
            for member in zf.namelist():
                _check_member_path(member, dest)
            zf.extractall(dest)
    else:
        with tarfile.open(archive_path) as tf:
            try:
                tf.extractall(dest, filter="data")  # py3.12+ safe extraction
            except TypeError:
                for member in tf.getmembers():
                    _check_member_path(member.name, dest)
                tf.extractall(dest)


def _console_script_dir() -> Path | None:
    """Best-effort: the directory actually holding the installed trqsh(.exe)
    launcher pip/setuptools generated for the [project.scripts] entry point.

    Not the same as locating *this* module: ``python -m trqsh`` resolves
    ``sys.argv[0]`` to ``__main__.py`` inside site-packages, not the launcher,
    so the venv/system/--user Scripts dirs are checked directly rather than
    derived from argv0 alone.
    """
    shim = _bin_name()
    candidates = []
    argv0 = Path(sys.argv[0]).resolve()
    if argv0.name == shim:
        candidates.append(argv0.parent)
    candidates.append(Path(sys.executable).resolve().parent)  # a venv's own Scripts dir
    candidates.append(Path(sys.executable).resolve().parent / "Scripts")  # system/base install
    if site.ENABLE_USER_SITE:
        candidates.append(Path(site.USER_BASE) / "Scripts")  # `pip install` without admin lands here
    for c in candidates:
        if (c / shim).exists():
            return c
    return None


# PowerShell source for _ensure_on_path below — mirrors the npm wrapper's
# postinstall fix (packaging/npm/lib/install.js). Static text only; the
# directory value flows in via the TRQSH_PATH_DIR env var rather than being
# interpolated into the command text (a Windows path can contain a literal
# single quote, e.g. a username like O'Brien).
_PATH_FIX_SCRIPT = "\n".join(
    [
        "$dir = $env:TRQSH_PATH_DIR",
        "$cur = [Environment]::GetEnvironmentVariable('Path','User')",
        "if (-not $cur) { $cur = '' }",
        "$already = $false",
        "foreach ($p in ($cur -split ';')) { if ($p -and ($p.TrimEnd('\\') -ieq $dir.TrimEnd('\\'))) { $already = $true } }",
        "if (-not $already) {",
        "  $new = if ($cur.Trim().Length -gt 0) { $cur.TrimEnd(';') + ';' + $dir } else { $dir }",
        "  [Environment]::SetEnvironmentVariable('Path', $new, 'User')",
        '  Add-Type -Namespace TrqshWin32 -Name NativeMethods -MemberDefinition \'[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);\'',
        "  $result = [UIntPtr]::Zero",
        "  [TrqshWin32.NativeMethods]::SendMessageTimeout([IntPtr]0xffff, 0x1a, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null",
        "  Write-Output 'ADDED'",
        "} else {",
        "  Write-Output 'ALREADY'",
        "}",
    ]
)


def _ensure_on_path() -> None:
    """Best-effort Windows PATH self-heal.

    pip has no post-install hook — a wheel installs by unpacking, no code
    runs afterward — so unlike the npm wrapper's postinstall, this can't fix
    the very first ``trqsh`` invocation after ``pip install`` (if PATH is
    broken, the shim can't be reached to run this at all). But ``python -m
    trqsh`` / ``py -m trqsh`` always works — python.exe itself is on PATH
    from any standard installer even when Scripts isn't — and running that
    once triggers this self-heal, fixing every bare ``trqsh`` invocation
    from then on.
    """
    if os.name != "nt":
        return
    exe = str(Path(sys.executable)).lower()
    if os.environ.get("PIPX_HOME") or "pipx" in exe:
        return  # pipx manages its own PATH via `pipx ensurepath`
    scripts_dir = _console_script_dir()
    if scripts_dir is None:
        return

    def norm(p: str) -> str:
        return p.rstrip("\\/").lower()

    target = norm(str(scripts_dir))
    if any(norm(p) == target for p in os.environ.get("PATH", "").split(os.pathsep) if p):
        return
    try:
        result = subprocess.run(  # noqa: S603 (fixed arg list, no shell)
            ["powershell", "-NoProfile", "-NonInteractive", "-Command", _PATH_FIX_SCRIPT],
            env={**os.environ, "TRQSH_PATH_DIR": str(scripts_dir)},
            capture_output=True,
            text=True,
            timeout=15,
            check=False,
        )
        if "ADDED" in result.stdout:
            print(f"trqsh: added {scripts_dir} to your PATH — open a new terminal to use the trqsh command", file=sys.stderr)
    except Exception as exc:  # noqa: BLE001 — never block the actual command over a PATH nicety
        print(f"trqsh: warning — couldn't update PATH automatically ({exc})", file=sys.stderr)


def ensure_binary() -> Path:
    """Return the path to the trqsh binary, downloading it once if needed."""
    _ensure_on_path()
    target = bin_path()
    if target.exists():
        return target

    goos, goarch, ext = _target()
    archive = f"trqsh_{VERSION}_{goos}_{goarch}.{ext}"
    # Overridable (TRQSH_DOWNLOAD_BASE) for mirrors, air-gapped installs, or tests.
    base = os.environ.get("TRQSH_DOWNLOAD_BASE") or f"https://github.com/{REPO}/releases/download/v{VERSION}"

    print(f"trqsh: downloading {archive} (v{VERSION})...", file=sys.stderr)
    data = _download(f"{base}/{archive}")
    _verify(data, archive, base)

    tmp = Path(tempfile.mkdtemp())
    archive_path = tmp / archive
    archive_path.write_bytes(data)
    _extract(archive_path, ext, _cache_dir())

    if not target.exists():
        for found in _cache_dir().rglob(target.name):
            found.replace(target)
            break
    if not target.exists():
        raise SystemExit("trqsh: binary not found after extraction")

    if os.name != "nt":
        target.chmod(target.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
    print("trqsh: installed ✓", file=sys.stderr)
    return target
