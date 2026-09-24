"""Thin launcher for the Go testscan binary. No parsing or rules here."""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path


def resolve_bin() -> Path:
    if env := os.environ.get("TESTSCAN_BIN"):
        p = Path(env)
        if not p.is_file():
            raise FileNotFoundError(f"TESTSCAN_BIN not a file: {env}")
        return p

    name = "testscan.exe" if os.name == "nt" else "testscan"
    bundled = Path(__file__).resolve().parent / "bin" / name
    if bundled.is_file():
        return bundled

    which = shutil.which("testscan")
    if which:
        p = Path(which).resolve()
        # не выбирать сам uvx/pip entry-point (рекурсия)
        argv0 = Path(sys.argv[0]).resolve()
        if p != argv0 and p.is_file():
            return p

    raise FileNotFoundError(
        "testscan binary not found. Install from PyPI (uvx testscan@latest), "
        "set TESTSCAN_BIN, run python/scripts/embed_bin.ps1 (or .sh), "
        "or put the Go binary on PATH."
    )


def main() -> None:
    try:
        bin_path = resolve_bin()
    except FileNotFoundError as exc:
        print(f"testscan: {exc}", file=sys.stderr)
        raise SystemExit(2) from exc

    # Windows + uvx: subprocess надёжнее os.execv
    raise SystemExit(subprocess.call([str(bin_path), *sys.argv[1:]]))


if __name__ == "__main__":
    main()
