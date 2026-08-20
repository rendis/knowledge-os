#!/usr/bin/env python3
"""Local workspace configuration. Sole writer is configure-workspace."""
from __future__ import annotations

import argparse
import json
import os
import stat
import sys
from pathlib import Path
from typing import Any

SCHEMA = {
    "version": 1,
    "workspace": {"repository_roots": [], "managed_clone": {"enabled": False, "root": ""}},
    "skills": {"manage-development-handoff": {"worktree_root": ""}},
}


def _atomic_write(path: Path, text: str) -> None:
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(text, encoding="utf-8")
    os.replace(tmp, path)
    path.chmod(stat.S_IRUSR | stat.S_IWUSR)


def config_path(vault_root: Path) -> Path:
    return vault_root / ".knowledge-os-config.yaml"


def dump_config(data: dict[str, Any]) -> str:
    roots = data.get("workspace", {}).get("repository_roots") or []
    managed = data.get("workspace", {}).get("managed_clone") or {}
    worktree = (
        (data.get("skills") or {}).get("manage-development-handoff") or {}
    ).get("worktree_root") or ""
    ports = ((data.get("skills") or {}).get("inspect-database") or {}).get("environments") or {}
    lines = [
        "version: 1",
        "workspace:",
        "  repository_roots:",
    ]
    if roots:
        for root in roots:
            lines.append(f'    - "{root}"')
    else:
        lines.append("    []")
    lines.extend(
        [
            "  managed_clone:",
            f'    enabled: {"true" if managed.get("enabled") else "false"}',
            f'    root: "{managed.get("root") or ""}"',
            "skills:",
            "  manage-development-handoff:",
            f'    worktree_root: "{worktree}"',
            "  inspect-database:",
            "    environments:",
        ]
    )
    if ports:
        for env, port in ports.items():
            lines.append(f"      {env}:")
            lines.append(f"        proxy_port: {port}")
    else:
        lines.append("      {}")
    lines.append("")
    return "\n".join(lines)


def load_config(vault_root: Path) -> dict[str, Any]:
    path = config_path(vault_root)
    if not path.is_file():
        return {
            "status": "uninitialized",
            "source_context": {
                "status": "unavailable",
                "roots": [],
                "clone_root": None,
                "clone_origin": None,
                "clone_authorized": False,
                "warnings": ["workspace configuration is missing; run configure-workspace"],
            },
        }
    text = path.read_text(encoding="utf-8")
    roots: list[str] = []
    clone_root = ""
    clone_enabled = False
    worktree = ""
    ports: dict[str, str] = {}
    section = ""
    current_env = ""
    for raw in text.splitlines():
        line = raw.strip()
        if line.startswith("repository_roots:"):
            section = "roots"
            continue
        if line.startswith("managed_clone:"):
            section = "clone"
            continue
        if line.startswith("worktree_root:"):
            worktree = line.split(":", 1)[1].strip().strip('"')
            continue
        if line.startswith("environments:"):
            section = "ports"
            continue
        if section == "roots" and line.startswith("- "):
            roots.append(line[2:].strip().strip('"'))
        if section == "clone" and line.startswith("enabled:"):
            clone_enabled = "true" in line.lower()
        if section == "clone" and line.startswith("root:"):
            clone_root = line.split(":", 1)[1].strip().strip('"')
        if section == "ports" and line.endswith(":") and not line.startswith("proxy_port"):
            current_env = line[:-1].strip()
        if section == "ports" and line.startswith("proxy_port:") and current_env:
            ports[current_env] = line.split(":", 1)[1].strip()
    source_roots = [
        {"path": root, "origin": "config", "managed": clone_enabled and root == clone_root}
        for root in roots
        if root
    ]
    return {
        "status": "initialized" if roots else "initialized",
        "source_context": {
            "status": "ok" if roots else "unavailable",
            "roots": source_roots,
            "clone_root": clone_root or None,
            "clone_origin": "config" if clone_enabled else None,
            "clone_authorized": bool(clone_enabled and clone_root),
            "warnings": [] if roots else ["no repository roots configured"],
        },
        "worktree_root": worktree or None,
        "proxy_ports": ports,
    }


def locate_repository(vault_root: Path, remote: str) -> dict[str, Any]:
    from urllib.parse import urlparse
    import re
    import subprocess

    def normalize(value: str) -> str | None:
        value = value.strip()
        if not value:
            return None
        scp = re.fullmatch(r"(?:[^@]+@)?([^:]+):(.+)", value)
        if scp and "://" not in value:
            host, path = scp.groups()
        else:
            parsed = urlparse(value if "://" in value else f"https://{value}")
            host, path = parsed.hostname or "", parsed.path
        path = path.strip("/")
        if path.casefold().endswith(".git"):
            path = path[:-4]
        return f"{host.lower()}/{path.lower()}" if host and path else None

    wanted = normalize(remote)
    if not wanted:
        return {"status": "invalid", "error": "remote is empty"}
    status = load_config(vault_root)
    matches: list[str] = []
    for root in status["source_context"]["roots"]:
        path = Path(root["path"])
        candidates = [path] if (path / ".git").exists() or (path / ".git").is_file() else list(path.iterdir()) if path.is_dir() else []
        for candidate in candidates:
            if not candidate.is_dir():
                continue
            result = subprocess.run(
                ["git", "-C", str(candidate), "remote", "get-url", "origin"],
                capture_output=True,
                text=True,
                check=False,
            )
            if result.returncode == 0 and normalize(result.stdout) == wanted:
                matches.append(str(candidate.resolve()))
    if len(matches) == 1:
        return {"status": "ok", "path": matches[0], "remote": remote}
    if not matches:
        return {"status": "not_found", "remote": remote}
    return {"status": "ambiguous", "matches": matches, "remote": remote}


def schema_repository(vault_root: Path) -> dict[str, Any]:
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from instance import load_instance, InstanceError

    try:
        instance = load_instance(vault_root / "instance.yaml")
    except InstanceError as error:
        return {"status": "uninitialized", "error": str(error)}
    remote = str((instance["sources"].get("schema_repository") or {}).get("remote") or "").strip()
    if not remote:
        return {"status": "not_configured", "error": "sources.schema_repository.remote is empty"}
    located = locate_repository(vault_root, remote)
    located["note"] = (instance["sources"].get("schema_repository") or {}).get("note")
    return located


def database_proxy_port(vault_root: Path, environment: str) -> dict[str, Any]:
    ports = load_config(vault_root).get("proxy_ports") or {}
    if environment not in ports:
        return {"status": "proxy_port_not_configured", "environment": environment}
    return {"status": "ok", "environment": environment, "port": ports[environment]}


def initialize(vault_root: Path, roots: list[str], managed_root: str, worktree_root: str) -> dict[str, Any]:
    data = {
        "version": 1,
        "workspace": {
            "repository_roots": roots,
            "managed_clone": {"enabled": bool(managed_root), "root": managed_root},
        },
        "skills": {
            "manage-development-handoff": {"worktree_root": worktree_root},
            "inspect-database": {"environments": {}},
        },
    }
    _atomic_write(config_path(vault_root), dump_config(data))
    return load_config(vault_root)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault-root", required=True, type=Path)
    parser.add_argument(
        "command",
        choices=[
            "status",
            "initialize",
            "update",
            "locate-repository",
            "development-worktree-root",
            "repository",
            "database-proxy-port",
        ],
    )
    parser.add_argument("--format", default="json")
    parser.add_argument("--repository-root", action="append", default=[])
    parser.add_argument("--managed-clone-root", default="")
    parser.add_argument("--development-worktree-root", default="")
    parser.add_argument("--proxy-port", default="")
    parser.add_argument("remote", nargs="?", default="")
    args = parser.parse_args()
    root = args.vault_root.expanduser().resolve()
    if args.command == "status":
        print(json.dumps(load_config(root), indent=2))
        return 0
    if args.command in {"initialize", "update"}:
        print(
            json.dumps(
                initialize(
                    root,
                    args.repository_root,
                    args.managed_clone_root,
                    args.development_worktree_root,
                ),
                indent=2,
            )
        )
        return 0
    if args.command == "locate-repository":
        print(json.dumps(locate_repository(root, args.remote), indent=2))
        return 0 if locate_repository(root, args.remote).get("status") == "ok" else 2
    if args.command == "development-worktree-root":
        value = load_config(root).get("worktree_root")
        if args.format == "value":
            print(value or "")
        else:
            print(json.dumps({"worktree_root": value}, indent=2))
        return 0 if value else 3
    if args.command == "repository":
        print(json.dumps(schema_repository(root), indent=2))
        return 0 if schema_repository(root).get("status") == "ok" else 2
    if args.command == "database-proxy-port":
        env = args.remote or ""
        payload = database_proxy_port(root, env)
        if args.format == "value" and payload.get("status") == "ok":
            print(payload["port"])
            return 0
        print(json.dumps(payload, indent=2))
        return 0 if payload.get("status") == "ok" else 3
    return 2


if __name__ == "__main__":
    sys.exit(main())
