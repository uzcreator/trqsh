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
import stat
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path

from . import __version__

REPO = os.environ.get("TRQSH_REPO", "trqsh/trqsh")
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


def _extract(archive_path: Path, ext: str, dest: Path) -> None:
    if ext == "zip":
        with zipfile.ZipFile(archive_path) as zf:
            zf.extractall(dest)
    else:
        with tarfile.open(archive_path) as tf:
            try:
                tf.extractall(dest, filter="data")  # py3.12+ safe extraction
            except TypeError:
                tf.extractall(dest)


def ensure_binary() -> Path:
    """Return the path to the trqsh binary, downloading it once if needed."""
    target = bin_path()
    if target.exists():
        return target

    goos, goarch, ext = _target()
    archive = f"trqsh_{VERSION}_{goos}_{goarch}.{ext}"
    base = f"https://github.com/{REPO}/releases/download/v{VERSION}"

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
