#!/usr/bin/env python3
"""Check broken, alias-targeted and orphan Obsidian notes."""
from __future__ import annotations

import argparse
import collections
import re
import sys
from pathlib import Path

from vault_frontmatter import split_frontmatter

EXPECTED_ORPHANS = {"00-Home", "README"}
EXCLUDED_ROOT_FILES = {"AGENTS.md", "CLAUDE.md", "AGENTS.personal.md"}


def visible_files(root: Path, pattern: str) -> list[Path]:
    files: list[Path] = []
    for path in root.rglob(pattern):
        relative = path.relative_to(root)
        if any(part.startswith(".") or part == "investigations" for part in relative.parts):
            continue
        if relative.as_posix() in EXCLUDED_ROOT_FILES:
            continue
        files.append(path)
    return sorted(files)


def strip_code(text: str) -> str:
    text = re.sub(r"```.*?```", "", text, flags=re.DOTALL)
    return re.sub(r"`[^`\n]*`", "", text)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    return parser.parse_args()


def verify(root: Path) -> dict[str, list[str]]:
    root = root.resolve()
    note_paths = visible_files(root, "*.md")
    notes: dict[str, Path] = {}
    aliases: dict[str, str] = {}
    duplicates: dict[str, list[str]] = collections.defaultdict(list)

    for path in note_paths:
        base = path.stem
        if base in notes:
            duplicates[base].extend([str(notes[base].relative_to(root)), str(path.relative_to(root))])
        else:
            notes[base] = path
        values = split_frontmatter(path.read_text(encoding="utf-8"))[0].get("aliases", [])
        if isinstance(values, str):
            values = [values]
        for alias in values:
            if alias:
                aliases[str(alias)] = base

    base_files = {path.name for path in visible_files(root, "*.base")}
    inbound: set[str] = set()
    broken: dict[str, set[str]] = collections.defaultdict(set)
    alias_links: dict[str, set[str]] = collections.defaultdict(set)
    hidden_agent_links: dict[str, set[str]] = collections.defaultdict(set)

    for path in note_paths:
        source = str(path.relative_to(root))
        text = strip_code(path.read_text(encoding="utf-8"))
        for target in re.findall(r"\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)", text):
            normalized_target = target.replace("\\", "/")
            if re.search(r"(^|/)\.agents(?:/|$)", normalized_target):
                hidden_agent_links[target].add(source)
        for target in re.findall(r"\[\[([^\]|#]+)", text):
            target = target.strip()
            if target in notes:
                inbound.add(target)
            elif target in base_files:
                continue
            elif target in aliases:
                alias_links[target].add(source)
            else:
                broken[target].add(source)

    expected = sorted(
        notes[name].relative_to(root).as_posix()
        for name in EXPECTED_ORPHANS
        if name in notes and name not in inbound
    )
    orphan_paths = sorted(
        path.relative_to(root).as_posix()
        for name, path in notes.items()
        if name not in inbound and name not in EXPECTED_ORPHANS
    )
    orphan_stems = sorted(Path(path).stem for path in orphan_paths)
    broken_rows = sorted(
        f"{source} -> {target}"
        for target, sources in broken.items()
        for source in sources
    )
    alias_rows = sorted(
        f"{source} -> alias {target} (use {aliases[target]})"
        for target, sources in alias_links.items()
        for source in sources
    )
    hidden_rows = sorted(
        f"{source} -> {target}"
        for target, sources in hidden_agent_links.items()
        for source in sources
    )
    duplicate_rows = sorted(
        f"{base}: {sorted(set(paths))}"
        for base, paths in duplicates.items()
    )
    return {
        "broken": broken_rows,
        "alias_targets": alias_rows,
        "hidden": hidden_rows,
        "duplicates": duplicate_rows,
        "orphans": orphan_stems,
        "orphan_paths": orphan_paths,
        "expected_orphans": expected,
    }


def main() -> int:
    result = verify(parse_args().root)

    for label, key in (
        ("BROKEN LINKS", "broken"),
        ("ALIAS LINKS", "alias_targets"),
        ("HIDDEN AGENT LINKS", "hidden"),
        ("DUPLICATE BASENAMES", "duplicates"),
        ("ORPHANS", "orphan_paths"),
        ("EXPECTED ORPHANS", "expected_orphans"),
    ):
        rows = result[key]
        print(f"{label}: {len(rows)}")
        for row in rows:
            print(f"  {row}")

    return 1 if any(
        result[key]
        for key in ("broken", "alias_targets", "hidden", "duplicates", "orphans")
    ) else 0


if __name__ == "__main__":
    sys.exit(main())
