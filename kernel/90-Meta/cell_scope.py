#!/usr/bin/env python3
"""Read repository scope decisions from the cell scope note."""
from __future__ import annotations

import re
from pathlib import Path

REPOSITORY_NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]*$")


def markdown_section(text: str, heading: str) -> str:
    marker = f"## {heading}"
    if marker not in text:
        return ""
    return text.split(marker, 1)[1].split("\n## ", 1)[0]


def approved_cross_scope_repositories(
    root: Path,
    prefixes: tuple[str, ...],
    container: str,
) -> set[str]:
    """Return repository names explicitly listed in the scope table."""
    scope_path = root / "90-Meta/Alcance.md"
    if not scope_path.is_file():
        return set()
    section = markdown_section(
        scope_path.read_text(encoding="utf-8"),
        "Repositorios fuente",
    )
    candidates: set[str] = set()
    for line in section.splitlines():
        match = re.match(r"^\s*\|\s*`([^`]+)`\s*\|", line)
        if match and REPOSITORY_NAME.fullmatch(match.group(1)):
            candidates.add(match.group(1))
    folded_prefixes = tuple(value.casefold() for value in prefixes)
    folded_container = container.casefold()
    return {
        name
        for name in candidates
        if name.casefold() != folded_container
        and not name.casefold().startswith(folded_prefixes)
    }
