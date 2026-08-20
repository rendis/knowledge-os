#!/usr/bin/env python3
"""Exact Obsidian vault-name to filesystem binding check."""
from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault-root", required=True, type=Path)
    parser.add_argument("--vault-name", required=True)
    args = parser.parse_args()
    root = args.vault_root.expanduser().resolve()
    if shutil.which("obsidian") is None:
        print(json.dumps({"status": "cli_missing", "bound": False}))
        return 2
    result = subprocess.run(
        ["obsidian", "vaults", "verbose"],
        capture_output=True,
        text=True,
        check=False,
        encoding="utf-8",
    )
    if result.returncode:
        print(json.dumps({"status": "cli_error", "bound": False}))
        return 2
    bound = False
    for line in result.stdout.splitlines():
        parts = line.split("\t", 1)
        if len(parts) != 2:
            parts = line.split(maxsplit=1)
        if len(parts) == 2 and parts[0].strip() == args.vault_name:
            bound = Path(parts[1].strip()).expanduser().resolve() == root
            break
    payload = {"status": "ok" if bound else "mismatch", "bound": bound, "vault_name": args.vault_name}
    print(json.dumps(payload))
    return 0 if bound else 1


if __name__ == "__main__":
    sys.exit(main())
