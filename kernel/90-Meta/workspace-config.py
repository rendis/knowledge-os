#!/usr/bin/env python3
"""Local workspace configuration. Sole writer is configure-workspace."""
from __future__ import annotations

import argparse
import json
import os
import re
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


class ConfigError(ValueError):
    """Invalid or unsupported local configuration; never repair implicitly."""


def parse_config_text(text: str) -> dict[str, Any]:
    # Deliberately bounded YAML subset, independent of optional PyYAML installs.
    # Unsupported YAML is rejected before an update can discard its values.
    lines = []
    for number, raw in enumerate(text.splitlines(), 1):
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        if "\t" in raw:
            raise ConfigError(f"line {number}: tabs are unsupported")
        lines.append((len(raw) - len(raw.lstrip()), raw.strip(), number))

    def scalar(raw: str) -> Any:
        if raw in ("[]", "{}"):
            return json.loads(raw)
        if raw.startswith('"'):
            try:
                value = json.loads(raw)
            except ValueError as error:
                raise ConfigError("invalid quoted scalar") from error
            if not isinstance(value, str):
                raise ConfigError("expected a quoted string")
            return value
        if raw.startswith("'"):
            if not re.fullmatch(r"'(?:[^']|'')*'", raw):
                raise ConfigError("invalid quoted scalar")
            return raw[1:-1].replace("''", "'")
        if raw == "true" or raw == "false":
            return raw == "true"
        if re.fullmatch(r"[0-9]+", raw):
            return int(raw)
        if not raw or any(c in raw for c in "[]{}#&*!|>\"'") or ": " in raw:
            raise ConfigError("unsupported or malformed YAML scalar")
        return raw

    def block(index: int, indent: int) -> tuple[Any, int]:
        value: Any = [] if lines[index][1].startswith("- ") else {}
        if lines[index][1] in ("[]", "{}"):
            return scalar(lines[index][1]), index + 1
        while index < len(lines) and lines[index][0] >= indent:
            level, content, number = lines[index]
            if level != indent:
                raise ConfigError(f"line {number}: invalid indentation")
            if isinstance(value, list):
                if not content.startswith("- "):
                    raise ConfigError(f"line {number}: expected list item")
                value.append(scalar(content[2:].strip()))
                index += 1
                continue
            match = re.fullmatch(r'("(?:[^"\\]|\\.)*"|[^:]+):(?: (.*))?', content)
            if not match:
                raise ConfigError(f"line {number}: expected mapping entry")
            raw_key, raw_value = match.groups()
            key = scalar(raw_key.strip())
            if not isinstance(key, str) or key in value:
                raise ConfigError(f"line {number}: invalid or duplicate key")
            index += 1
            if raw_value is None or not raw_value.strip():
                if index >= len(lines) or lines[index][0] <= indent:
                    raise ConfigError(f"line {number}: missing value")
                item, index = block(index, lines[index][0])
            else:
                item = scalar(raw_value.strip())
            value[key] = item
        return value, index

    if not lines or lines[0][0] != 0:
        raise ConfigError("configuration must be a mapping at indentation zero")
    value, end = block(0, 0)
    if end != len(lines):
        raise ConfigError("unexpected trailing configuration")
    return validate_data(value)


def validate_data(value: Any) -> dict[str, Any]:
    def mapping(item: Any, allowed: set[str], label: str) -> dict[str, Any]:
        if not isinstance(item, dict) or set(item) - allowed:
            raise ConfigError(f"{label}: expected supported mapping keys")
        return item

    value = mapping(value, {"version", "workspace", "skills"}, "config")
    if type(value.get("version")) is not int or value["version"] != 1:
        raise ConfigError("version must be 1")
    workspace = mapping(value.get("workspace", {}), {"repository_roots", "managed_clone"}, "workspace")
    skills = mapping(value.get("skills", {}), {"manage-development-handoff", "inspect-database"}, "skills")
    roots = workspace.get("repository_roots", [])
    if not isinstance(roots, list) or any(not isinstance(r, str) or not r or any(c in r for c in "\n\r\0") for r in roots):
        raise ConfigError("repository_roots must be a list of nonempty paths")
    managed = mapping(workspace.get("managed_clone", {}), {"enabled", "root"}, "managed_clone")
    handoff = mapping(skills.get("manage-development-handoff", {}), {"worktree_root"}, "manage-development-handoff")
    database = mapping(skills.get("inspect-database", {}), {"environments"}, "inspect-database")
    enabled, clone, worktree = managed.get("enabled", False), managed.get("root", ""), handoff.get("worktree_root", "")
    if type(enabled) is not bool or any(not isinstance(p, str) or any(c in p for c in "\n\r\0") for p in (clone, worktree)):
        raise ConfigError("invalid managed clone or worktree settings")
    ports = database.get("environments", {})
    if not isinstance(ports, dict):
        raise ConfigError("environments must be a mapping")
    normalized = {}
    for env, port in ports.items():
        if isinstance(port, dict):
            mapping(port, {"proxy_port"}, "environment")
            port = port.get("proxy_port")
        if not isinstance(env, str) or not env.strip() or any(c in env for c in "\n\r\0"):
            raise ConfigError("environment must be a nonempty single-line name")
        if type(port) not in (str, int) or not re.fullmatch(r"[0-9]+", str(port)) or not 1 <= int(port) <= 65535:
            raise ConfigError(f"invalid proxy port for {env}")
        normalized[env] = str(int(port))
    data = _empty_data()
    data["workspace"] = {"repository_roots": roots, "managed_clone": {"enabled": enabled, "root": clone}}
    data["skills"]["manage-development-handoff"]["worktree_root"] = worktree
    data["skills"]["inspect-database"]["environments"] = normalized
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
            lines.append(f"    - {json.dumps(root, ensure_ascii=False)}")
    else:
        lines.append("    []")
    lines.extend(
        [
            "  managed_clone:",
            f'    enabled: {"true" if managed.get("enabled") else "false"}',
            f"    root: {json.dumps(managed.get('root') or '', ensure_ascii=False)}",
            "skills:",
            "  manage-development-handoff:",
            f"    worktree_root: {json.dumps(worktree, ensure_ascii=False)}",
            "  inspect-database:",
            "    environments:",
        ]
    )
    if ports:
        for env, port in ports.items():
            value = port.get("proxy_port") if isinstance(port, dict) else port
            lines.append(f"      {json.dumps(env, ensure_ascii=False)}:")
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
    existing = load_data(vault_root)  # Reject invalid existing state even for initialize.
    data = _empty_data() if replace else (existing or _empty_data())
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
    data = validate_data(data)
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
    try:
        sys.exit(main())
    except (ConfigError, OSError, UnicodeError) as error:
        print(json.dumps({"status": "invalid", "error": str(error)}), file=sys.stderr)
        sys.exit(2)
