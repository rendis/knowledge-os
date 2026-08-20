#!/usr/bin/env python3
"""List operational areas and resolve notes by basename or report-id."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from vault_frontmatter import read_frontmatter


def notes(root: Path) -> list[dict]:
    folder = root / "60-Operacion"
    rows = []
    if not folder.is_dir():
        return rows
    for path in folder.rglob("*.md"):
        fields = read_frontmatter(path)
        rows.append(
            {
                "basename": path.stem,
                "path": str(path.relative_to(root)),
                "tipo": fields.get("tipo"),
                "clase": fields.get("clase"),
                "report_id": fields.get("report-id"),
            }
        )
    return sorted(rows, key=lambda item: item["basename"])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    parser.add_argument("command", choices=["list-areas", "list-reports", "resolve"])
    parser.add_argument("--report-id", default="")
    parser.add_argument("--basename", default="")
    args = parser.parse_args()
    root = args.root.resolve()
    rows = notes(root)
    if args.command == "list-areas":
        areas = sorted({Path(item["path"]).parts[1] for item in rows if len(Path(item["path"]).parts) > 2})
        print(json.dumps({"areas": areas}, indent=2))
        return 0
    if args.command == "list-reports":
        reports = [item for item in rows if item.get("report_id") or item.get("clase") == "reporte"]
        print(json.dumps({"reports": reports}, indent=2))
        return 0
    target = args.report_id or args.basename
    matches = [
        item
        for item in rows
        if item["basename"] == target or item.get("report_id") == target
    ]
    if len(matches) != 1:
        print(json.dumps({"status": "unresolved", "matches": matches}))
        return 2
    print(json.dumps({"status": "ok", "note": matches[0]}, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
