#!/usr/bin/env python3
"""Verify that an explicit Obsidian vault name resolves to the expected path."""
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path


def normalized(path: Path) -> str:
    return os.path.normcase(os.path.realpath(os.fspath(path)))


def same_path(left: Path, right: Path) -> bool:
    try:
        return left.samefile(right)
    except OSError:
        return normalized(left) == normalized(right)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault-root", required=True, type=Path)
    parser.add_argument("--vault-name", required=True)
    return parser.parse_args()


def configure_output() -> None:
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure:
            reconfigure(errors="backslashreplace")


def main() -> int:
    configure_output()
    args = parse_args()
    if not args.vault_root.is_absolute():
        print("OBSIDIAN_BINDING: --vault-root must be absolute", file=sys.stderr)
        return 2
    expected = args.vault_root.expanduser().resolve()
    if not expected.is_dir():
        print(f"OBSIDIAN_BINDING: expected vault directory does not exist: {expected}", file=sys.stderr)
        return 2
    vault_name = args.vault_name.strip()
    if not vault_name:
        print("OBSIDIAN_BINDING: --vault-name must not be empty", file=sys.stderr)
        return 2

    executable = shutil.which("obsidian")
    if not executable:
        print("OBSIDIAN_BINDING: Obsidian CLI is unavailable", file=sys.stderr)
        return 2
    result = subprocess.run(
        [executable, f"vault={vault_name}", "vault", "info=path"],
        text=True,
        encoding="utf-8",
        errors="replace",
        capture_output=True,
        check=False,
    )
    lines = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    if result.returncode or not lines:
        detail = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
        print(f"OBSIDIAN_BINDING: unable to resolve {vault_name!r}: {detail}", file=sys.stderr)
        return 1

    actual_text = lines[-1]
    actual = Path(actual_text).expanduser()
    if not actual.is_absolute() or not same_path(actual, expected):
        print(
            f"OBSIDIAN_BINDING: mismatch for {vault_name!r}; "
            f"expected {expected}, observed {actual_text}",
            file=sys.stderr,
        )
        return 1

    print(f"OBSIDIAN_BINDING: OK ({expected})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
