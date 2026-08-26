#!/usr/bin/env python3
"""Resolve the canonical cell knowledge vault using identity and markers."""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from instance import load_instance

MARKERS = (
    "AGENTS.md",
    "00-Home.md",
    "instance.yaml",
    "90-Meta/Convenciones.md",
    "90-Meta/Auditoria - Framework.md",
)


def run(command: list[str], encoding: str | None = None) -> subprocess.CompletedProcess[str]:
    executable = shutil.which(command[0])
    resolved = [executable or command[0], *command[1:]]
    try:
        return subprocess.run(
            resolved,
            text=True,
            encoding=encoding,
            errors="replace" if encoding else None,
            capture_output=True,
            check=False,
        )
    except FileNotFoundError as error:
        return subprocess.CompletedProcess(resolved, 127, "", str(error))


def normalize_remote(remote: str) -> str | None:
    value = remote.strip()
    if not value:
        return None
    scp = re.fullmatch(r"(?:[^@]+@)?([^:]+):(.+)", value)
    if scp and "://" not in value:
        host, path = scp.groups()
    else:
        try:
            parsed = urlparse(value if "://" in value else f"https://{value}")
            host, path = parsed.hostname or "", parsed.path
        except ValueError:
            return None
    path = path.strip("/")
    if path.casefold().endswith(".git"):
        path = path[:-4]
    return f"{host.lower()}/{path.lower()}" if host and path else None


def git_remote(path: Path) -> str | None:
    result = run(["git", "-C", str(path), "remote", "get-url", "origin"])
    return result.stdout.strip() if result.returncode == 0 else None


def marker_check(path: Path) -> tuple[bool, list[str]]:
    missing = [marker for marker in MARKERS if not (path / marker).is_file()]
    return not missing, missing


def validate_vault(path: Path) -> dict[str, Any]:
    root = path.resolve()
    markers_valid, missing = marker_check(root)
    declared = ""
    instance_error: str | None = None
    try:
        instance = load_instance(root / "instance.yaml")
        declared = str(instance["vault"]["remote"])
    except Exception as error:
        instance_error = f"{type(error).__name__}: {error}"
    remote = git_remote(root)
    identity = normalize_remote(remote or "")
    declared_identity = normalize_remote(declared)
    remote_ok = not declared or (
        declared_identity is not None and identity == declared_identity
    )
    return {
        "path": str(root),
        "valid": markers_valid and instance_error is None and remote_ok,
        "remote": remote,
        "declared_remote": declared,
        "remote_identity": identity,
        "missing_markers": missing,
        "instance_error": instance_error,
    }


def ancestors(path: Path) -> list[Path]:
    resolved = path.resolve()
    start = resolved if resolved.is_dir() else resolved.parent
    return [start, *start.parents]


def path_identity(path: Path) -> tuple[Any, ...]:
    resolved = path.resolve()
    try:
        metadata = resolved.stat()
    except OSError:
        return ("path", os.path.normcase(os.path.realpath(os.fspath(resolved))))
    if metadata.st_ino:
        return ("filesystem", metadata.st_dev, metadata.st_ino)
    return ("path", os.path.normcase(os.path.realpath(os.fspath(resolved))))


def obsidian_vaults() -> tuple[bool, list[dict[str, str]]]:
    result = run(["obsidian", "vaults", "verbose"], encoding="utf-8")
    if result.returncode:
        return False, []
    records: list[dict[str, str]] = []
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        parts = line.split("\t", 1)
        if len(parts) != 2:
            parts = line.split(maxsplit=1)
        if len(parts) == 2:
            records.append({"name": parts[0].strip(), "path": parts[1].strip()})
    return True, records


def workspace_source_context(vault_root: Path) -> dict[str, Any]:
    helper = vault_root / "90-Meta" / "workspace-config.py"
    unavailable_context = {
        "status": "unavailable",
        "roots": [],
        "clone_root": None,
        "clone_origin": None,
        "clone_authorized": False,
        "warnings": ["workspace configuration API is unavailable; run configure-workspace"],
    }
    if not helper.is_file():
        return unavailable_context
    result = run(
        [sys.executable, "-B", str(helper), "--vault-root", str(vault_root), "status", "--format", "json"]
    )
    try:
        response = json.loads(result.stdout)
        context = response["source_context"]
    except (json.JSONDecodeError, KeyError, TypeError):
        return unavailable_context
    return context if isinstance(context, dict) else unavailable_context


def orientation(vault_root: Path) -> dict[str, Any]:
    helper = vault_root / "90-Meta" / "instance.py"
    if not helper.is_file():
        return {"ready": False, "reason": "complete-bootstrap"}
    result = run(
        [
            sys.executable,
            "-B",
            "-c",
            "import json,sys; sys.path.insert(0, sys.argv[1]); from instance import orientation_status; "
            "from pathlib import Path; print(json.dumps(orientation_status(Path(sys.argv[2]))))",
            str(helper.parent),
            str(vault_root),
        ]
    )
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        return {"ready": False, "reason": "complete-bootstrap", "issues": [result.stderr.strip()]}


def payload(status: str, **values: Any) -> dict[str, Any]:
    return {"status": status, **values}


def resolved_payload(
    root: Path,
    registered_by_path: dict[tuple[Any, ...], str],
    obsidian_available: bool,
    source: str,
    declared_remote: str,
) -> dict[str, Any]:
    resolved_root = root.resolve()
    vault_name = registered_by_path.get(path_identity(resolved_root))
    return payload(
        "resolved",
        vault_root=str(resolved_root),
        obsidian_vault=vault_name,
        obsidian_available=obsidian_available,
        interaction_mode="obsidian-cli" if vault_name else "filesystem",
        source_context=workspace_source_context(resolved_root),
        orientation=orientation(resolved_root),
        resolution_source=source,
        declared_remote=declared_remote,
    )


def resolve(explicit: Path | None, cwd: Path) -> tuple[int, dict[str, Any]]:
    obsidian_available, registered = obsidian_vaults()
    registered_by_path = {
        path_identity(Path(item["path"]).expanduser()): item["name"] for item in registered
    }
    if explicit:
        for candidate in ancestors(explicit.expanduser()):
            checked = validate_vault(candidate)
            if checked["valid"]:
                return 0, resolved_payload(
                    Path(checked["path"]),
                    registered_by_path,
                    obsidian_available,
                    "explicit",
                    str(checked["declared_remote"]),
                )
        return 2, payload("invalid", reason="The explicit path is not inside a cell vault.", checked_path=str(explicit))
    for candidate in ancestors(cwd):
        checked = validate_vault(candidate)
        if checked["valid"]:
            return 0, resolved_payload(
                Path(checked["path"]),
                registered_by_path,
                obsidian_available,
                "cwd",
                str(checked["declared_remote"]),
            )
    matches = []
    for item in registered:
        checked = validate_vault(Path(item["path"]).expanduser())
        if checked["valid"]:
            matches.append(
                {
                    "name": item["name"],
                    "path": checked["path"],
                    "declared_remote": checked["declared_remote"],
                }
            )
    if len(matches) == 1:
        return 0, resolved_payload(
            Path(matches[0]["path"]),
            registered_by_path,
            obsidian_available,
            "obsidian",
            str(matches[0]["declared_remote"]),
        )
    if len(matches) > 1:
        return 2, payload(
            "ambiguous",
            reason="Multiple registered vaults match cell markers.",
            candidates=[
                {"name": item["name"], "path": item["path"]}
                for item in matches
            ],
        )
    return 2, payload("not_found", reason="No local cell vault was found.", obsidian_available=obsidian_available)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--path", type=Path)
    parser.add_argument("--cwd", type=Path, default=Path.cwd())
    args = parser.parse_args()
    code, data = resolve(args.path, args.cwd.expanduser().resolve())
    print(json.dumps(data, indent=2))
    return code


if __name__ == "__main__":
    sys.exit(main())
