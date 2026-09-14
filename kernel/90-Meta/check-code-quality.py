#!/usr/bin/env python3
"""Run the portable Python lint and security baseline."""
from __future__ import annotations

import argparse
import importlib.util
import subprocess
import sys
from pathlib import Path


def run(command: list[str], root: Path) -> int:
    return subprocess.run(command, cwd=root, check=False).returncode


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="distribution or installed vault root")
    args = parser.parse_args()
    root = Path(args.root).expanduser().resolve()
    if not root.is_dir():
        parser.error(f"root is not a directory: {root}")

    meta = Path(__file__).resolve().parent
    missing = [name for name in ("ruff", "bandit") if importlib.util.find_spec(name) is None]
    if missing:
        print(
            f"missing quality tools: {', '.join(missing)}; install with "
            f"{sys.executable} -m pip install -r {meta / 'requirements-ci.txt'}",
            file=sys.stderr,
        )
        return 2

    python = sys.executable
    commands = (
        [python, "-m", "ruff", "check", "--config", str(meta / "ruff.toml"), str(root)],
        [
            python,
            "-m",
            "bandit",
            "-q",
            "--ini",
            str(meta / ".bandit"),
            "--severity-level",
            "medium",
            str(root),
        ],
        [
            python,
            "-m",
            "bandit",
            "-q",
            "--ini",
            str(meta / ".bandit"),
            "--tests",
            "B101",
            str(root),
        ],
    )
    return_codes = [run(command, root) for command in commands]
    return max(code if code >= 0 else 128 - code for code in return_codes)


if __name__ == "__main__":
    sys.exit(main())
