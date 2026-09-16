#!/usr/bin/env python3
"""Run the portable Python lint and security baseline."""
from __future__ import annotations

import argparse
import importlib.util
import json
import subprocess
import sys
from pathlib import Path


def run(command: list[str], root: Path) -> int:
    return subprocess.run(command, cwd=root, check=False).returncode


def run_bandit(command: list[str], root: Path) -> int:
    """A zero exit code is insufficient when Bandit skipped failed scans."""
    result = subprocess.run(
        [*command, "--format", "json"], cwd=root, check=False,
        capture_output=True, text=True,
    )
    if result.stderr:
        print(result.stderr, file=sys.stderr, end="")
    try:
        report = json.loads(result.stdout)
        if not isinstance(report, dict) or not all(
            isinstance(report.get(key), list) for key in ("errors", "results")
        ):
            raise ValueError("missing errors/results lists")
    except (ValueError, TypeError):
        print("Bandit analysis incomplete: invalid or missing JSON report", file=sys.stderr)
        return 2
    if report["errors"] or report["results"]:
        print(json.dumps(report, indent=2))
    if report["errors"]:
        print("Bandit analysis incomplete: resolve scanner errors before acceptance", file=sys.stderr)
        return 2
    return result.returncode or (1 if report["results"] else 0)


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
    return_codes = [run(commands[0], root)]
    return_codes.extend(run_bandit(command, root) for command in commands[1:])
    return max(code if code >= 0 else 128 - code for code in return_codes)


if __name__ == "__main__":
    sys.exit(main())
