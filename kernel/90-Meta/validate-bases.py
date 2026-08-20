#!/usr/bin/env python3
"""Validate root Obsidian Bases YAML structure."""
from __future__ import annotations

import argparse
import sys
from pathlib import Path


def validate(root: Path) -> list[str]:
    issues: list[str] = []
    bases = list(root.glob("*.base"))
    if not bases:
        issues.append("no root .base files")
    for path in bases:
        text = path.read_text(encoding="utf-8")
        if "views:" not in text:
            issues.append(f"{path.name}: missing views")
        if "filters:" not in text:
            issues.append(f"{path.name}: missing filters")
    return issues


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    args = parser.parse_args()
    issues = validate(args.root.resolve())
    if issues:
        print("FAIL")
        for item in issues:
            print(f"- {item}")
        return 1
    print(f"PASS {len(list(args.root.glob('*.base')))} bases")
    return 0


if __name__ == "__main__":
    sys.exit(main())
