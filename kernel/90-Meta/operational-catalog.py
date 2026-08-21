#!/usr/bin/env python3
"""Resolve the operational catalog derived from vault Markdown."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from vault_frontmatter import read_frontmatter

AREAS = ("Base de datos", "DevOps", "GCP", "Jira", "Reporteria")


class CatalogError(Exception):
    """Stable catalog error with a machine-readable code."""

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


def notes(root: Path) -> list[dict[str, Any]]:
    operation_root = root / "60-Operacion"
    result: list[dict[str, Any]] = []
    for path in sorted(operation_root.glob("*/*.md")):
        if path.stem == path.parent.name:
            continue
        fields = read_frontmatter(path)
        if fields.get("tipo") != "operacional":
            raise CatalogError("contract-invalid", f"{path}: tipo must be operacional")
        result.append(
            {
                "basename": path.stem,
                "path": path.relative_to(root).as_posix(),
                "area": fields.get("area"),
                "clase": fields.get("clase"),
                "estado": fields.get("estado"),
                "report-id": fields.get("report-id"),
            }
        )
    return result


def list_areas(root: Path) -> list[dict[str, str]]:
    result = []
    for area in AREAS:
        path = root / "60-Operacion" / area / f"{area}.md"
        if not path.is_file():
            raise CatalogError("contract-invalid", f"missing area MOC: {area}")
        result.append({"area": area, "path": path.relative_to(root).as_posix()})
    return result


def list_reports(root: Path) -> list[dict[str, Any]]:
    reports = [item for item in notes(root) if item["clase"] == "reporte"]
    ids = [item["report-id"] for item in reports]
    if any(not value for value in ids) or len(ids) != len(set(ids)):
        raise CatalogError("contract-invalid", "report-id values must be present and unique")
    return sorted(reports, key=lambda item: item["report-id"])


def resolve(root: Path, key: str, value: str) -> dict[str, Any]:
    matches = [item for item in notes(root) if item.get(key) == value]
    if not matches:
        raise CatalogError("not-found", f"no operational note for {key}={value}")
    if len(matches) > 1:
        raise CatalogError("ambiguous", f"multiple operational notes for {key}={value}")
    return matches[0]


def emit(value: Any, output_format: str) -> None:
    if output_format == "json":
        print(json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2))
        return
    rows = value if isinstance(value, list) else [value]
    for row in rows:
        print("\t".join(f"{key}={row[key]}" for key in sorted(row) if row[key] is not None))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--format", choices=("json", "text"), default="json")
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("list-areas")
    subparsers.add_parser("list-reports")
    resolver = subparsers.add_parser("resolve")
    group = resolver.add_mutually_exclusive_group(required=True)
    group.add_argument("--basename")
    group.add_argument("--report-id")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = args.root.resolve()
    try:
        if args.command == "list-areas":
            result = list_areas(root)
        elif args.command == "list-reports":
            result = list_reports(root)
        elif args.basename:
            result = resolve(root, "basename", args.basename)
        else:
            result = resolve(root, "report-id", args.report_id)
        emit(result, args.format)
        return 0
    except CatalogError as error:
        print(json.dumps({"error": error.code, "message": str(error)}, ensure_ascii=False))
        return 2


if __name__ == "__main__":
    sys.exit(main())
