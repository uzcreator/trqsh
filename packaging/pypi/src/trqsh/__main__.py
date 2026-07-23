"""Console entry point: exec the real trqsh binary with the caller's arguments."""

from __future__ import annotations

import os
import subprocess
import sys

from ._runtime import ensure_binary


def main() -> None:
    binary = str(ensure_binary())
    args = [binary, *sys.argv[1:]]
    if os.name == "nt":
        # os.execv on Windows doesn't preserve the console cleanly; use subprocess.
        sys.exit(subprocess.run(args).returncode)
    os.execv(binary, args)


if __name__ == "__main__":
    main()
