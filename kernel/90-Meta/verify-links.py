#!/usr/bin/env python3
"""Detect broken wikilinks, alias targets, and unexpected orphans."""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

from vault_frontmatter import read_frontmatter

WIKILINK = re.compile(r"\[\[([^\]|#]+)(?:#[^\]|]*)?(?:\|[^\]]*)?\]\]")


def note_index(root: Path) -> tuple[dict[str, Path], dict[str, str], set[str]]:
    skip = {".git", ".agents", ".obsidian", ".investigations", ".operations", "plan"}
    by_stem: dict[str, Path] = {}
    alias_to_stem: dict[str, str] = {}
    for path in root.rglob("*.md"):
        if any(part in skip or part.startswith(".") for part in path.relative_to(root).parts[:-1]):
            continue
        stem = path.stem
        by_stem[stem] = path
        aliases = read_frontmatter(path).get("aliases") or []
        if isinstance(aliases, str):
            aliases = [aliases]
        for alias in aliases:
            alias_to_stem[str(alias)] = stem
    return by_stem, alias_to_stem, set(by_stem)


def verify(root: Path) -> dict[str, list[str]]:
    by_stem, alias_to_stem, stems = note_index(root)
    broken: list[str] = []
    alias_targets: list[str] = []
    hidden: list[str] = []
    mentioned: set[str] = set()
    skip = {".git", ".agents", ".obsidian", ".investigations", ".operations", "plan"}
    for path in root.rglob("*.md"):
        rel_parts = path.relative_to(root).parts
        if any(part in skip or part.startswith(".") for part in rel_parts[:-1]):
            continue
        text = path.read_text(encoding="utf-8")
        for match in WIKILINK.finditer(text):
            target = match.group(1).strip()
            if ".agents/" in target:
                hidden.append(f"{path.relative_to(root)} -> {target}")
                continue
            mentioned.add(target)
            if target in stems:
                continue
            if target in alias_to_stem:
                alias_targets.append(f"{path.relative_to(root)} -> alias {target} (use {alias_to_stem[target]})")
                continue
            broken.append(f"{path.relative_to(root)} -> {target}")
    protected = {"00-Home", "AGENTS", "CLAUDE", "README"}
    orphans = sorted(
        stem
        for stem in stems
        if stem not in mentioned
        and stem not in protected
        and by_stem[stem].parent.name not in {"90-Meta"}
        and stem not in {"Convenciones", "Auditoria - Framework", "Flujos", "Operacion", "Aprendizajes"}
    )
    return {
        "broken": broken,
        "alias_targets": alias_targets,
        "hidden": hidden,
        "orphans": orphans,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    args = parser.parse_args()
    result = verify(args.root.resolve())
    failed = bool(result["broken"] or result["alias_targets"] or result["hidden"])
    for key, rows in result.items():
        print(f"{key}: {len(rows)}")
        for row in rows:
            print(f"  {row}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
