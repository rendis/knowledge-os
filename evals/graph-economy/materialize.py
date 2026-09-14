#!/usr/bin/env python3
"""Build a Norte Logistics fixture vault in a destination directory."""
from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
DIST = HERE.parents[1]
OVERLAY = HERE / "fixtures" / "norte" / "overlay"
INVEST = HERE / "fixtures" / "norte" / "investigations"
INSTALL = DIST / "install.sh"


def materialize(dest: Path, *, disable_topics: bool = False) -> Path:
    dest = dest.expanduser().resolve()
    if dest.exists():
        shutil.rmtree(dest)
    dest.mkdir(parents=True)
    cmd = [
        "sh",
        str(INSTALL),
        "init",
        "--dest",
        str(dest),
        "--cell-name",
        "Platform Libraries" if disable_topics else "Norte Logistics",
        "--purpose",
        "Shared platform libraries" if disable_topics else "Last-mile routing and fleet visibility.",
        "--system",
        "platform:Platform" if disable_topics else "routing:Routing",
        "--yes",
    ]
    if not disable_topics:
        cmd.extend(["--system", "fleet:Fleet"])
    else:
        cmd.append("--disable-topics")
    result = subprocess.run(cmd, cwd=str(DIST), text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise SystemExit(result.stderr or result.stdout)
    if not disable_topics:
        for src in OVERLAY.rglob("*"):
            if src.is_file():
                target = dest / src.relative_to(OVERLAY)
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src, target)
        investigations = dest / "investigations"
        shutil.copytree(INVEST, investigations, dirs_exist_ok=True)
    return dest


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dest", type=Path, required=True)
    parser.add_argument("--disable-topics", action="store_true")
    args = parser.parse_args()
    path = materialize(args.dest, disable_topics=args.disable_topics)
    print(path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
