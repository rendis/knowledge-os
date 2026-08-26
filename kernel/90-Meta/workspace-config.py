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

CONFIG_NAME = ".knowledge-os-config.yaml"


def config_path(vault_root: Path) -> Path:
    return vault_root / CONFIG_NAME


def _atomic_write(path: Path, text: str) -> None:
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(text, encoding="utf-8")
    os.replace(tmp, path)
    path.chmod(stat.S_IRUSR | stat.S_IWUSR)


def _empty_data() -> dict[str, Any]:
    return {
        "version": 1,
        "workspace": {"repository_roots": [], "managed_clone": {"enabled": False, "root": ""}},
        "skills": {
            "manage-development-handoff": {"worktree_root": ""},
            "inspect-database": {"environments": {}},
        },
    }


def parse_config_text(text: str) -> dict[str, Any]:
    data = _empty_data()
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
    data["workspace"]["repository_roots"] = [item for item in roots if item]
    data["workspace"]["managed_clone"] = {"enabled": clone_enabled, "root": clone_root}
    data["skills"]["manage-development-handoff"]["worktree_root"] = worktree
    data["skills"]["inspect-database"]["environments"] = ports
    return data


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
            value = port.get("proxy_port") if isinstance(port, dict) else port
            lines.append(f"      {env}:")
            lines.append(f"        proxy_port: {value}")
    else:
        lines.append("      {}")
    lines.append("")
    return "\n".join(lines)


def load_data(vault_root: Path) -> dict[str, Any] | None:
    path = config_path(vault_root)
    if not path.is_file():
        return None
    return parse_config_text(path.read_text(encoding="utf-8"))


def load_config(vault_root: Path) -> dict[str, Any]:
    data = load_data(vault_root)
    if data is None:
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
    roots = data["workspace"]["repository_roots"]
    managed = data["workspace"]["managed_clone"]
    clone_root = managed.get("root") or ""
    clone_enabled = bool(managed.get("enabled"))
    worktree = data["skills"]["manage-development-handoff"].get("worktree_root") or ""
    ports = {
        env: (port.get("proxy_port") if isinstance(port, dict) else str(port))
        for env, port in (data["skills"]["inspect-database"].get("environments") or {}).items()
    }
    source_roots = [
        {"path": root, "origin": "config", "managed": clone_enabled and root == clone_root}
        for root in roots
        if root
    ]
    return {
        "status": "initialized" if roots else "uninitialized",
        "source_context": {
            "status": "resolved" if roots else "unavailable",
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
                ["git", "-C", str(candidate), "config", "--get", "remote.origin.url"],
                capture_output=True,
                text=True,
                check=False,
            )
            if result.returncode == 0 and normalize(result.stdout) == wanted:
                matches.append(str(candidate.resolve()))
    if len(matches) == 1:
        return {"status": "ok", "path": matches[0], "remote": wanted}
    if not matches:
        return {"status": "not_found", "remote": wanted}
    return {"status": "ambiguous", "matches": matches, "remote": wanted}


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


def apply_config(
    vault_root: Path,
    *,
    roots: list[str] | None = None,
    managed_root: str | None = None,
    disable_clone: bool = False,
    worktree_root: str | None = None,
    disable_worktree: bool = False,
    proxy_ports: dict[str, str] | None = None,
    replace: bool = False,
) -> dict[str, Any]:
    data = _empty_data() if replace else (load_data(vault_root) or _empty_data())
    if roots is not None:
        data["workspace"]["repository_roots"] = [item for item in roots if item]
    if disable_clone:
        data["workspace"]["managed_clone"] = {"enabled": False, "root": ""}
    elif managed_root is not None:
        data["workspace"]["managed_clone"] = {"enabled": bool(managed_root), "root": managed_root}
    if disable_worktree:
        data["skills"]["manage-development-handoff"]["worktree_root"] = ""
    elif worktree_root is not None:
        data["skills"]["manage-development-handoff"]["worktree_root"] = worktree_root
    if proxy_ports:
        environments = data["skills"]["inspect-database"].setdefault("environments", {})
        environments.update(proxy_ports)
    _atomic_write(config_path(vault_root), dump_config(data))
    return load_config(vault_root)


def initialize(vault_root: Path, roots: list[str], managed_root: str, worktree_root: str) -> dict[str, Any]:
    return apply_config(
        vault_root,
        roots=roots,
        managed_root=managed_root,
        worktree_root=worktree_root,
        replace=True,
    )


def parse_proxy_port(raw: str) -> tuple[str, str]:
    env, _, port = raw.partition("=")
    env, port = env.strip(), port.strip()
    if not env or not port:
        raise ValueError("proxy port must be ENVIRONMENT=PORT")
    return env, port


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
    parser.add_argument("--disable-managed-clone", action="store_true")
    parser.add_argument("--development-worktree-root", default="")
    parser.add_argument("--disable-development-worktree-root", action="store_true")
    parser.add_argument("--proxy-port", action="append", default=[])
    parser.add_argument("remote", nargs="?", default="")
    args = parser.parse_args()
    root = args.vault_root.expanduser().resolve()
    if args.command == "status":
        print(json.dumps(load_config(root), indent=2))
        return 0
    if args.command in {"initialize", "update"}:
        ports = {}
        try:
            for item in args.proxy_port:
                env, port = parse_proxy_port(item)
                ports[env] = port
        except ValueError as error:
            print(json.dumps({"error": str(error)}), file=sys.stderr)
            return 2
        if args.command == "initialize":
            roots, managed, worktree = args.repository_root, args.managed_clone_root, args.development_worktree_root
        else:
            roots = args.repository_root or None
            managed = args.managed_clone_root or None
            worktree = args.development_worktree_root or None
        print(
            json.dumps(
                apply_config(
                    root,
                    roots=roots,
                    managed_root=managed,
                    disable_clone=args.disable_managed_clone,
                    worktree_root=worktree,
                    disable_worktree=args.disable_development_worktree_root,
                    proxy_ports=ports or None,
                    replace=args.command == "initialize",
                ),
                indent=2,
            )
        )
        return 0
    if args.command == "locate-repository":
        payload = locate_repository(root, args.remote)
        print(json.dumps(payload, indent=2))
        return 0 if payload.get("status") == "ok" else 2
    if args.command == "development-worktree-root":
        value = load_config(root).get("worktree_root")
        if args.format == "value":
            print(value or "")
        else:
            print(json.dumps({"worktree_root": value}, indent=2))
        return 0 if value else 3
    if args.command == "repository":
        payload = schema_repository(root)
        print(json.dumps(payload, indent=2))
        return 0 if payload.get("status") == "ok" else 2
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
