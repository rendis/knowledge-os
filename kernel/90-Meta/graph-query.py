#!/usr/bin/env python3
"""Portable vault graph query. JSON only; no note bodies. Does not require Obsidian."""
from __future__ import annotations

import argparse
import importlib.util
import json
import re
import sys
from pathlib import Path
from typing import Any

_META = Path(__file__).resolve().parent
if str(_META) not in sys.path:
    sys.path.insert(0, str(_META))

from vault_frontmatter import read_frontmatter

WIKILINK = re.compile(r"\[\[([^\]|#]+)(?:#[^\]|]*)?(?:\|[^\]]*)?\]\]")
LINK_FIELDS = (
    "publica-en",
    "gatillado-por",
    "consume-de",
    "lee-de",
    "escribe-en",
    "usa-infra",
    "participa-en",
    "compuesto-por",
    "implementado-por",
    "aplica-a",
    "sistema",
)
SKIP_PARTS = {".git", ".agents", ".obsidian", ".operations", "investigations", "plan"}


def _load_verify():
    path = Path(__file__).resolve().parent / "verify-links.py"
    spec = importlib.util.spec_from_file_location("verify_links", path)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load verify-links.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _stems_from_value(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, list):
        stems: list[str] = []
        for item in value:
            stems.extend(_stems_from_value(item))
        return stems
    text = str(value)
    found = [match.group(1).strip() for match in WIKILINK.finditer(text)]
    if found:
        return found
    stripped = text.strip().strip("\"'")
    return [stripped] if stripped else []


def _iter_notes(root: Path):
    for path in root.rglob("*.md"):
        parts = path.relative_to(root).parts
        if parts == ("AGENTS.personal.md",):
            continue
        if any(part in SKIP_PARTS or part.startswith(".") for part in parts[:-1]):
            continue
        yield path


def build_graph(root: Path) -> dict[str, Any]:
    outgoing: dict[str, list[dict[str, str]]] = {}
    incoming: dict[str, list[dict[str, str]]] = {}
    paths: dict[str, str] = {}
    for path in _iter_notes(root):
        stem = path.stem
        rel = path.relative_to(root).as_posix()
        paths[stem] = rel
        fields = read_frontmatter(path)
        body = path.read_text(encoding="utf-8")
        edges: list[dict[str, str]] = []
        for field in LINK_FIELDS:
            for target in _stems_from_value(fields.get(field)):
                edges.append({"field": field, "target": target})
        for match in WIKILINK.finditer(body):
            target = match.group(1).strip()
            if not any(edge["target"] == target for edge in edges):
                edges.append({"field": "body", "target": target})
        outgoing[stem] = edges
        for edge in edges:
            incoming.setdefault(edge["target"], []).append({"field": edge["field"], "source": stem})
    return {"paths": paths, "outgoing": outgoing, "incoming": incoming}


def neighbors(root: Path, node: str) -> dict[str, Any]:
    graph = build_graph(root)
    return {
        "node": node,
        "path": graph["paths"].get(node),
        "outgoing": graph["outgoing"].get(node, []),
        "incoming": graph["incoming"].get(node, []),
    }


def hygiene(root: Path) -> dict[str, Any]:
    verify = _load_verify().verify(root)
    unresolved_targets = sorted(
        {row.split(" -> ", 1)[1] for row in verify["broken"] if " -> " in row}
    )
    return {
        "unresolved": verify["broken"],
        "unresolved_targets": unresolved_targets,
        "orphans": verify["orphans"],
        "alias_targets": verify["alias_targets"],
    }


def investigations(root: Path, node: str) -> dict[str, Any]:
    matches: list[dict[str, str]] = []
    store = root / "investigations"
    if not store.is_dir():
        return {"node": node, "investigations": []}
    needle = node.casefold()
    for path in sorted(store.glob("*/investigation.md")):
        text = path.read_text(encoding="utf-8")
        stems = [match.group(1).strip() for match in WIKILINK.finditer(text)]
        fields = read_frontmatter(path)
        ident = str(fields.get("id") or path.parent.name)
        if node in stems or needle in text.casefold():
            matches.append(
                {
                    "id": ident,
                    "path": path.relative_to(root).as_posix(),
                    "title": str(fields.get("title") or ident),
                }
            )
    return {"node": node, "investigations": matches}


def orientation(root: Path) -> dict[str, Any]:
    from instance import load_instance, orientation_status

    status = orientation_status(root)
    instance = load_instance(root / "instance.yaml") if (root / "instance.yaml").is_file() else {}
    return {
        "orientation": status,
        "systems": [item.get("name") for item in (instance.get("systems") or [])],
        "enabled_types": ((instance.get("graph") or {}).get("enabled_types")) or [],
    }


def run_query(root: Path, args: list[str]) -> dict[str, Any]:
    if not args:
        raise ValueError("missing command")
    command = args[0]
    node = ""
    if "--node" in args:
        node = args[args.index("--node") + 1]
    root = root.resolve()
    if command == "neighbors":
        return neighbors(root, node)
    if command == "hygiene":
        return hygiene(root)
    if command == "investigations":
        return investigations(root, node)
    if command in {"orientation", "status"}:
        return orientation(root)
    raise ValueError(f"unknown command: {command}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("command", choices=["neighbors", "hygiene", "investigations", "orientation"])
    parser.add_argument("--node", default="")
    args = parser.parse_args()
    argv = [args.command]
    if args.node:
        argv.extend(["--node", args.node])
    try:
        payload = run_query(args.root, argv)
    except Exception as error:  # noqa: BLE001
        print(json.dumps({"error": str(error)}), file=sys.stderr)
        return 2
    print(json.dumps(payload, indent=2, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
