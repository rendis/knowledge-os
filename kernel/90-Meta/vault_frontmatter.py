"""Minimal, dependency-free reader for this vault's frontmatter subset."""
from __future__ import annotations

import csv
import re
from pathlib import Path
from typing import Any


def _strip_comment(value: str) -> str:
    quote = ""
    for index, char in enumerate(value):
        if char in "\"'":
            if not quote:
                quote = char
            elif quote == char:
                quote = ""
        if char == "#" and not quote and (index == 0 or value[index - 1].isspace()):
            return value[:index].rstrip()
    return value.strip()


def parse_scalar(value: str) -> Any:
    value = _strip_comment(value).strip()
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        if not inner:
            return []
        items = next(csv.reader([inner], skipinitialspace=True))
        return [item.strip().strip("\"'") for item in items]
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        return value[1:-1]
    if value in {"null", "~"}:
        return None
    return value


def split_frontmatter(text: str) -> tuple[dict[str, Any], str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}, text
    try:
        end = lines.index("---", 1)
    except ValueError:
        return {}, text

    fields: dict[str, Any] = {}
    current: str | None = None
    for line in lines[1:end]:
        match = re.match(r"^([A-Za-z0-9_-]+):(?:\s*(.*))?$", line)
        if match:
            current = match.group(1)
            raw = match.group(2) or ""
            fields[current] = parse_scalar(raw) if raw else []
            continue
        item = re.match(r"^\s+-\s+(.*)$", line)
        if item and current:
            if not isinstance(fields[current], list):
                fields[current] = []
            fields[current].append(parse_scalar(item.group(1)))
    return fields, "\n".join(lines[end + 1 :]) + "\n"


def read_frontmatter(path: Path) -> dict[str, Any]:
    return split_frontmatter(path.read_text(encoding="utf-8"))[0]
