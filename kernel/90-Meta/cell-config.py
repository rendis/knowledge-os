#!/usr/bin/env python3
"""Bind optional cell capabilities to existing operational procedures."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import tempfile

from instance import InstanceError, load_instance, validate_instance
from vault_frontmatter import read_frontmatter


def resolve(root: Path, instance: dict, capability: str) -> dict:
    names = instance.get("capabilities", {}).get(capability, [])
    if not names:
        return {"status": "unconfigured", "capability": capability, "procedures": []}
    paths = []
    for name in names:
        matches = [p for p in (root / "60-Operacion").rglob("*.md") if p.stem == name]
        if len(matches) != 1 or matches[0].is_symlink() or not matches[0].resolve().is_relative_to(root.resolve()):
            raise InstanceError(f"capability {capability}: missing, ambiguous or unsafe procedure {name}")
        if read_frontmatter(matches[0]).get("tipo") != "operacional":
            raise InstanceError(f"capability {capability}: {name} is not an operational note")
        paths.append(matches[0].relative_to(root).as_posix())
    return {"status": "configured", "capability": capability, "procedures": paths}


def bind(root: Path, capability: str, procedures: list[str]) -> dict:
    path = root / "instance.yaml"
    if path.is_symlink():
        raise InstanceError("instance.yaml must be a regular file")
    original = path.read_text(encoding="utf-8")
    instance = load_instance(path)
    existing_capabilities = instance["capabilities"].copy()
    instance["capabilities"][capability] = procedures
    instance = validate_instance(instance)
    result = resolve(root, instance, capability)
    # Change only this top-level section: preserve identity, comments and unknown cell-owned fields.
    lines = original.splitlines(keepends=True)
    starts = [i for i, line in enumerate(lines) if re.match(r"^capabilities\s*:", line)]
    if any(re.match(r'^([\"\'])capabilities\1\s*:', line) for line in lines):
        raise InstanceError("capabilities must use an unquoted top-level key for binding")
    if existing_capabilities and not starts:
        raise InstanceError("unsupported capabilities section syntax")
    if len(starts) > 1:
        raise InstanceError("duplicate capabilities sections")
    section = "capabilities:\n" + "".join(
        f"  {name}: {json.dumps(notes, ensure_ascii=False)}\n"
        for name, notes in sorted(instance["capabilities"].items())
    )
    if starts:
        start = starts[0]
        end = next((i for i in range(start + 1, len(lines)) if re.match(r"^[^\s#][^:]*:", lines[i])), len(lines))
        if not re.fullmatch(r"capabilities\s*:\s*(?:#.*)?", lines[start].rstrip()):
            raise InstanceError("binding requires a block capabilities mapping")
        # Trailing comments belong to the following cell-owned field or EOF.
        while end > start + 1 and (not lines[end - 1].strip() or lines[end - 1].lstrip().startswith("#")):
            end -= 1
        comments = []
        for line in lines[start:end]:
            if "#" in line:
                # Valid capability ids and basenames cannot contain '#'.
                comments.append("  " + line[line.index("#"):].rstrip() + "\n")
        section = section.split("\n", 1)[0] + "\n" + "".join(comments) + section.split("\n", 1)[1]
        updated = "".join(lines[:start]) + section + "".join(lines[end:])
    else:
        updated = original + ("\n" if not original.endswith("\n") else "") + "\n" + section
    mode = path.stat().st_mode & 0o777
    with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=root, prefix=".cell-config-", delete=False) as handle:
        temporary = Path(handle.name)
        handle.write(updated)
    try:
        load_instance(temporary)
        if path.read_text(encoding="utf-8") != original:
            raise InstanceError("instance changed during capability binding")
        temporary.chmod(mode)
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)
    return result


def database_target(root: Path, instance: dict, target_id: str) -> dict:
    targets = instance.get("database_targets", [])
    matches = [target for target in targets if target["id"] == target_id]
    if not matches:
        return {"status": "not_configured", "target": target_id}
    target = matches[0]
    binding = resolve(
        root,
        {"capabilities": {"database-inspection": [target["procedure"]]}},
        "database-inspection",
    )
    return {"status": "configured", "target": target, "procedures": binding["procedures"]}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault-root", type=Path, required=True)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("status")
    commands.add_parser("database-targets")
    target_command = commands.add_parser("database-target")
    target_command.add_argument("--target", required=True)
    for name in ("resolve", "bind"):
        command = commands.add_parser(name)
        command.add_argument("--capability", required=True)
        if name == "bind":
            command.add_argument("--procedure", action="append", required=True)
    args = parser.parse_args()
    root = args.vault_root.resolve()
    try:
        instance = load_instance(root / "instance.yaml")
        if args.command == "database-targets":
            result = {"targets": instance.get("database_targets", [])}
        elif args.command == "database-target":
            result = database_target(root, instance, args.target)
        elif args.command == "bind":
            result = bind(root, args.capability, args.procedure)
        elif args.command == "resolve":
            result = resolve(root, instance, args.capability)
        else:
            result = {"evidence_profile": instance["evidence"]["profile"], "capabilities": [
                resolve(root, instance, name) for name in instance["capabilities"]
            ]}
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0
    except (InstanceError, OSError, ValueError) as error:
        print(json.dumps({"status": "invalid", "error": str(error)}))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
