#!/usr/bin/env python3
"""Validate the deterministic structure of Obsidian Base files."""
from __future__ import annotations

import argparse
import sys
from collections.abc import Mapping
from pathlib import Path
from typing import Any

try:
    from ruamel.yaml import YAML
    from ruamel.yaml.error import YAMLError
except ModuleNotFoundError:
    print(
        "ruamel.yaml is required; install 90-Meta/requirements-ci.txt",
        file=sys.stderr,
    )
    raise SystemExit(2)

from vault_frontmatter import split_frontmatter

TOP_LEVEL_KEYS = {"filters", "formulas", "properties", "summaries", "views"}
FILTER_OPERATORS = {"and", "or", "not"}
CONTRACT_NOTE_PROPERTIES = {
    "aliases",
    "ambiente",
    "area",
    "clase",
    "cobertura-datos",
    "cobertura-entradas",
    "cobertura-flujos",
    "cobertura-infra",
    "cobertura-salidas",
    "commit-analizado",
    "compuesto-por",
    "consume-de",
    "ejecuta",
    "escribe-en",
    "estado",
    "fecha-analisis",
    "gatilla-a",
    "gatillado-por",
    "implementado-por",
    "lee-de",
    "lenguaje",
    "nombre-raw",
    "owner",
    "participa-en",
    "plataforma",
    "proyecto",
    "publica-en",
    "report-id",
    "rama-analizada",
    "region",
    "sistema",
    "tags",
    "tipo",
    "ultima-auditoria",
    "ultima-verificacion",
    "usa-infra",
}
FILE_PROPERTIES = {
    "file.backlinks",
    "file.basename",
    "file.ctime",
    "file.embeds",
    "file.ext",
    "file.file",
    "file.folder",
    "file.links",
    "file.mtime",
    "file.name",
    "file.path",
    "file.properties",
    "file.size",
    "file.tags",
}
MAX_BASE_BYTES = 1_000_000


def nonempty_string(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def relative(path: Path, root: Path) -> str:
    try:
        return str(path.relative_to(root))
    except ValueError:
        return str(path)


def collect_note_properties(root: Path) -> set[str]:
    properties = set(CONTRACT_NOTE_PROPERTIES)
    for path in root.rglob("*.md"):
        relative_path = path.relative_to(root)
        if (
            any(part.startswith(".") or part == "investigations" for part in relative_path.parts)
            or (relative_path.parts and relative_path.parts[0] == "plan")
        ):
            continue
        try:
            fields, _ = split_frontmatter(path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError, ValueError):
            continue
        properties.update(str(key) for key in fields)
    return properties


def validate_filter(value: Any, location: str) -> list[str]:
    if nonempty_string(value):
        return []
    if not isinstance(value, Mapping):
        return [f"{location} must be a non-empty string or filter mapping"]
    keys = list(value)
    if len(keys) != 1 or keys[0] not in FILTER_OPERATORS:
        return [f"{location} must contain exactly one of and, or, or not"]
    operator = keys[0]
    children = value[operator]
    if not isinstance(children, list) or not children:
        return [f"{location}.{operator} must be a non-empty list"]
    issues: list[str] = []
    for index, child in enumerate(children):
        issues.extend(validate_filter(child, f"{location}.{operator}[{index}]"))
    return issues


def validate_string_mapping(value: Any, location: str) -> list[str]:
    if not isinstance(value, Mapping):
        return [f"{location} must be a mapping"]
    issues: list[str] = []
    for key, expression in value.items():
        if not nonempty_string(key):
            issues.append(f"{location} keys must be non-empty strings")
        if not nonempty_string(expression):
            issues.append(f"{location}.{key} must be a non-empty string")
    return issues


def validate_property_reference(
    value: Any,
    location: str,
    note_properties: set[str],
    formulas: set[str],
) -> list[str]:
    if not nonempty_string(value):
        return [f"{location} must be a non-empty property reference"]
    assert isinstance(value, str)
    if value.startswith("formula."):
        formula = value.removeprefix("formula.")
        if formula not in formulas:
            return [f"{location} references undefined formula {value}"]
    elif value.startswith("file."):
        if value not in FILE_PROPERTIES:
            return [f"{location} references unknown file property {value}"]
    elif value.startswith("note."):
        prop = value.removeprefix("note.")
        if prop not in note_properties:
            return [f"{location} references unknown note property {value}"]
    elif value not in note_properties:
        return [f"{location} references unknown note property {value}"]
    return []


def validate_properties(
    value: Any,
    note_properties: set[str],
    formulas: set[str],
) -> list[str]:
    if not isinstance(value, Mapping):
        return ["properties must be a mapping"]
    issues: list[str] = []
    for prop, config in value.items():
        location = f"properties.{prop}"
        issues.extend(
            validate_property_reference(prop, location, note_properties, formulas)
        )
        if not isinstance(config, Mapping):
            issues.append(f"{location} must be a mapping")
            continue
        if "displayName" in config and not isinstance(config["displayName"], str):
            issues.append(f"{location}.displayName must be a string")
    return issues


def validate_view(
    view: Any,
    index: int,
    note_properties: set[str],
    formulas: set[str],
) -> list[str]:
    location = f"views[{index}]"
    if not isinstance(view, Mapping):
        return [f"{location} must be a mapping"]
    issues: list[str] = []
    for required in ("type", "name"):
        if not nonempty_string(view.get(required)):
            issues.append(f"{location}.{required} must be a non-empty string")
    if "filters" in view:
        issues.extend(validate_filter(view["filters"], f"{location}.filters"))
    if "limit" in view:
        limit = view["limit"]
        if isinstance(limit, bool) or not isinstance(limit, int) or limit <= 0:
            issues.append(f"{location}.limit must be a positive integer")
    if "order" in view:
        order = view["order"]
        if not isinstance(order, list):
            issues.append(f"{location}.order must be a list")
        else:
            seen: set[str] = set()
            for order_index, prop in enumerate(order):
                prop_location = f"{location}.order[{order_index}]"
                issues.extend(
                    validate_property_reference(
                        prop, prop_location, note_properties, formulas
                    )
                )
                if isinstance(prop, str) and prop in seen:
                    issues.append(f"{prop_location} duplicates property {prop}")
                elif isinstance(prop, str):
                    seen.add(prop)
    if "groupBy" in view:
        group_by = view["groupBy"]
        if not isinstance(group_by, Mapping):
            issues.append(f"{location}.groupBy must be a mapping")
        else:
            issues.extend(
                validate_property_reference(
                    group_by.get("property"),
                    f"{location}.groupBy.property",
                    note_properties,
                    formulas,
                )
            )
            if group_by.get("direction") not in {"ASC", "DESC"}:
                issues.append(f"{location}.groupBy.direction must be ASC or DESC")
    if "summaries" in view:
        summaries = view["summaries"]
        if not isinstance(summaries, Mapping):
            issues.append(f"{location}.summaries must be a mapping")
        else:
            for prop, summary in summaries.items():
                issues.extend(
                    validate_property_reference(
                        prop,
                        f"{location}.summaries.{prop}",
                        note_properties,
                        formulas,
                    )
                )
                if not nonempty_string(summary):
                    issues.append(
                        f"{location}.summaries.{prop} must name a summary"
                    )
    return issues


def validate_base(path: Path, root: Path, note_properties: set[str]) -> list[str]:
    prefix = relative(path, root)
    try:
        raw = path.read_bytes()
    except OSError as error:
        return [f"{prefix}: cannot read file: {error}"]
    if len(raw) > MAX_BASE_BYTES:
        return [f"{prefix}: file exceeds {MAX_BASE_BYTES} bytes"]
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as error:
        return [f"{prefix}: must be UTF-8: {error}"]

    yaml = YAML(typ="safe", pure=True)
    yaml.allow_duplicate_keys = False
    yaml.max_depth = 64
    try:
        data = yaml.load(text)
    except YAMLError as error:
        return [f"{prefix}: invalid YAML: {error}"]

    if not isinstance(data, Mapping):
        return [f"{prefix}: root must be a mapping"]
    issues: list[str] = []
    unknown = sorted(set(data) - TOP_LEVEL_KEYS, key=str)
    if unknown:
        issues.append(f"unknown top-level keys: {', '.join(map(str, unknown))}")

    if "filters" in data:
        issues.extend(validate_filter(data["filters"], "filters"))

    formulas_value = data.get("formulas", {})
    if "formulas" in data:
        issues.extend(validate_string_mapping(formulas_value, "formulas"))
    formulas = (
        {str(key) for key in formulas_value}
        if isinstance(formulas_value, Mapping)
        else set()
    )

    if "summaries" in data:
        issues.extend(validate_string_mapping(data["summaries"], "summaries"))
    if "properties" in data:
        issues.extend(
            validate_properties(data["properties"], note_properties, formulas)
        )

    views = data.get("views")
    if not isinstance(views, list) or not views:
        issues.append("views must be a non-empty list")
    else:
        names: set[str] = set()
        for index, view in enumerate(views):
            issues.extend(validate_view(view, index, note_properties, formulas))
            if isinstance(view, Mapping) and nonempty_string(view.get("name")):
                name = str(view["name"])
                if name in names:
                    issues.append(f"views[{index}].name duplicates view {name}")
                names.add(name)

    return [f"{prefix}: {issue}" for issue in issues]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--root",
        type=Path,
        default=Path(__file__).resolve().parents[1],
        help="vault root (defaults to the repository root)",
    )
    args = parser.parse_args()
    root = args.root.resolve()
    paths = sorted(root.glob("*.base"))
    if not paths:
        print("BASES: 0")
        print("ISSUES: 1")
        print("No .base files found at the vault root")
        return 1

    note_properties = collect_note_properties(root)
    issues: list[str] = []
    for path in paths:
        issues.extend(validate_base(path, root, note_properties))

    print(f"BASES: {len(paths)}")
    print(f"ISSUES: {len(issues)}")
    for issue in issues:
        print(issue)
    return 1 if issues else 0


if __name__ == "__main__":
    raise SystemExit(main())
