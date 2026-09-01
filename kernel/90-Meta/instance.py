"""Load and validate a cell instance.yaml. No third-party YAML dependency."""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

ALLOWED_ADAPTERS = ("gcp", "postgres", "reports")
ALLOWED_PROFILES = ("production-gate", "documented-source", "mixed")
ALLOWED_LOCALES = ("es", "en")
DEFAULT_TYPES = (
    "sistema",
    "servicio",
    "componente",
    "recurso-runtime",
    "repositorio",
    "topic",
    "flujo",
    "integracion-externa",
    "glosario",
    "operacional",
    "aprendizaje",
    "indice",
)
MARKERS = (
    "AGENTS.md",
    "00-Home.md",
    "instance.yaml",
    "90-Meta/Convenciones.md",
    "90-Meta/Auditoria - Framework.md",
)


class InstanceError(ValueError):
    pass


def _parse_simple_yaml(text: str) -> dict[str, Any]:
    """Parse the closed instance.yaml subset (maps, lists of scalars/maps)."""
    try:
        import yaml  # type: ignore

        data = yaml.safe_load(text)
        if not isinstance(data, dict):
            raise InstanceError("instance.yaml must be a mapping")
        return data
    except ImportError:
        pass
    return _parse_minimal_yaml(text)


def _parse_minimal_yaml(text: str) -> dict[str, Any]:
    """Indentation-based subset sufficient for dump_instance output."""
    raw_lines = [line.rstrip() for line in text.splitlines()]
    lines = [line for line in raw_lines if line.strip() and not line.lstrip().startswith("#")]
    root: dict[str, Any] = {}
    stack: list[tuple[int, Any, str | None]] = [(-1, root, None)]

    def parent(indent: int) -> tuple[int, Any, str | None]:
        while stack and stack[-1][0] >= indent:
            stack.pop()
        return stack[-1]

    for index, line in enumerate(lines):
        indent = len(line) - len(line.lstrip(" "))
        stripped = line.strip()
        _indent, container, _ = parent(indent)
        if stripped.startswith("- "):
            item = stripped[2:]
            if isinstance(container, dict):
                # convert last empty dict assignment to a list
                raise InstanceError(f"list item with dict container: {stripped}")
            if not isinstance(container, list):
                raise InstanceError(f"list item outside a list: {stripped}")
            if ":" in item and not item.startswith("["):
                key, _, raw = item.partition(":")
                mapping: dict[str, Any] = {key.strip(): _scalar(raw.strip())}
                container.append(mapping)
                stack.append((indent, mapping, None))
            else:
                container.append(_scalar(item))
            continue
        key, _, raw = stripped.partition(":")
        key = key.strip()
        raw = raw.strip()
        if not isinstance(container, dict):
            raise InstanceError(f"key {key} inside a list")
        if raw == "" or raw == "|":
            next_kind = _next_kind(lines, index, indent)
            if next_kind == "list":
                value: Any = []
            else:
                value = {}
            container[key] = value
            stack.append((indent, value, key))
        else:
            container[key] = _scalar(raw)
    return root


def _next_kind(lines: list[str], index: int, indent: int) -> str:
    for line in lines[index + 1 :]:
        if not line.strip():
            continue
        nxt = len(line) - len(line.lstrip(" "))
        if nxt <= indent:
            return "map"
        return "list" if line.lstrip().startswith("- ") else "map"
    return "map"


def _scalar(value: str) -> Any:
    if value in {"[]", ""}:
        return [] if value == "[]" else ""
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        if not inner:
            return []
        return [part.strip().strip("\"'") for part in inner.split(",") if part.strip()]
    if (value.startswith('"') and value.endswith('"')) or (
        value.startswith("'") and value.endswith("'")
    ):
        return value[1:-1]
    if value in {"true", "false"}:
        return value == "true"
    if re.fullmatch(r"-?\d+", value):
        return int(value)
    return value


def dump_instance(data: dict[str, Any]) -> str:
    """Serialize instance.yaml with a stable layout."""
    lines = ["# Cell identity. Kernel update never overwrites this file.", "version: 1", ""]
    cell = data["cell"]
    lines.append("cell:")
    lines.append(f"  name: {_quote(cell['name'])}")
    lines.append(f"  purpose: {_quote(cell['purpose'])}")
    lines.append("")
    lines.append("systems:")
    for system in data["systems"]:
        lines.append(f"  - id: {_quote(system['id'])}")
        lines.append(f"    name: {_quote(system['name'])}")
        aliases = system.get("aliases") or []
        lines.append(f"    aliases: [{', '.join(_quote(a) for a in aliases)}]")
    lines.append("")
    trackers = data.get("trackers") or []
    if trackers:
        lines.append("trackers:")
        for tracker in trackers:
            lines.append(f"  - id: {_quote(tracker['id'])}")
            lines.append(f"    provider: {_quote(tracker['provider'])}")
            lines.append(f"    url: {_quote(tracker['url'])}")
    else:
        lines.append("trackers: []")
    lines.append("")
    vault = data.get("vault") or {}
    lines.append("vault:")
    lines.append(f"  remote: {_quote(vault.get('remote') or '')}")
    sources = data.get("sources") or {}
    lines.append("sources:")
    lines.append(f"  github_org: {_quote(sources.get('github_org') or '')}")
    prefixes = sources.get("repo_prefixes") or []
    lines.append(f"  repo_prefixes: [{', '.join(_quote(p) for p in prefixes)}]")
    roots = [item for item in (sources.get("discovery_roots") or []) if str(item).strip()]
    if roots:
        lines.append("  discovery_roots:")
        for root in roots:
            lines.append(f"    - {_quote(root)}")
    else:
        lines.append("  discovery_roots: []")
    schema = sources.get("schema_repository") or {}
    lines.append("  schema_repository:")
    lines.append(f"    remote: {_quote(schema.get('remote') or '')}")
    lines.append(f"    note: {_quote(schema.get('note') or '')}")
    graph = data.get("graph") or {}
    types = graph.get("enabled_types") or list(DEFAULT_TYPES)
    lines.append("graph:")
    lines.append("  enabled_types:")
    for item in types:
        lines.append(f"    - {item}")
    evidence = data.get("evidence") or {}
    lines.append("evidence:")
    lines.append(f"  profile: {evidence.get('profile') or 'production-gate'}")
    adapters = data.get("adapters") or []
    lines.append(f"adapters: [{', '.join(adapters)}]")
    locale = data.get("locale") or {}
    lines.append("locale:")
    lines.append(f"  notes: {locale.get('notes') or 'es'}")
    lines.append("")
    return "\n".join(lines)


def _quote(value: str) -> str:
    text = str(value).replace('"', '\\"')
    return f'"{text}"'


def _canonical_tracker_url(value: object) -> str:
    raw = str(value or "").strip()
    parsed = urlparse(raw)
    try:
        port = parsed.port
    except ValueError as error:
        raise InstanceError(f"invalid tracker URL: {raw}") from error
    if (
        parsed.scheme.casefold() != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        raise InstanceError(f"tracker URL must be a credential-free HTTPS URL: {raw}")
    host = parsed.hostname.casefold()
    if port:
        host = f"{host}:{port}"
    path = parsed.path.rstrip("/")
    return f"https://{host}{path}"


def validate_instance(data: dict[str, Any]) -> dict[str, Any]:
    if not isinstance(data, dict):
        raise InstanceError("instance.yaml must be a mapping")
    cell = data.get("cell") or {}
    if not isinstance(cell, dict) or not str(cell.get("name") or "").strip():
        raise InstanceError("cell.name is required")
    if not str(cell.get("purpose") or "").strip():
        raise InstanceError("cell.purpose is required")
    systems = data.get("systems") or []
    if not isinstance(systems, list) or not systems:
        raise InstanceError("systems must be a non-empty list")
    normalized_systems = []
    seen = set()
    for item in systems:
        if not isinstance(item, dict):
            raise InstanceError("each system must be a mapping with id and name")
        system_id = str(item.get("id") or "").strip()
        name = str(item.get("name") or "").strip()
        if not system_id or not name:
            raise InstanceError("each system needs id and name")
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", system_id):
            raise InstanceError(f"system id must be kebab-case: {system_id}")
        if system_id in seen:
            raise InstanceError(f"duplicate system id: {system_id}")
        seen.add(system_id)
        aliases = item.get("aliases") or []
        if isinstance(aliases, str):
            aliases = [aliases]
        normalized_systems.append(
            {"id": system_id, "name": name, "aliases": [str(a) for a in aliases if str(a).strip()]}
        )
    trackers = data.get("trackers") or []
    if not isinstance(trackers, list):
        raise InstanceError("trackers must be a list")
    normalized_trackers: list[dict[str, str]] = []
    seen_tracker_ids: set[str] = set()
    seen_tracker_targets: set[tuple[str, str]] = set()
    for item in trackers:
        if not isinstance(item, dict):
            raise InstanceError("each tracker must be a mapping with id, provider, and url")
        tracker_id = str(item.get("id") or "").strip()
        provider = str(item.get("provider") or "").strip().casefold()
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", tracker_id):
            raise InstanceError(f"tracker id must be kebab-case: {tracker_id}")
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", provider):
            raise InstanceError(f"tracker provider must be kebab-case: {provider}")
        url = _canonical_tracker_url(item.get("url"))
        if tracker_id in seen_tracker_ids:
            raise InstanceError(f"duplicate tracker id: {tracker_id}")
        target = (provider, url)
        if target in seen_tracker_targets:
            raise InstanceError(f"duplicate tracker target: {provider} {url}")
        seen_tracker_ids.add(tracker_id)
        seen_tracker_targets.add(target)
        normalized_trackers.append(
            {"id": tracker_id, "provider": provider, "url": url}
        )
    profile = ((data.get("evidence") or {}).get("profile")) or "production-gate"
    if profile not in ALLOWED_PROFILES:
        raise InstanceError(f"unknown evidence.profile: {profile}")
    locale = ((data.get("locale") or {}).get("notes")) or "es"
    if locale not in ALLOWED_LOCALES:
        raise InstanceError(f"unknown locale.notes: {locale}")
    adapters = data.get("adapters") or []
    if isinstance(adapters, str):
        adapters = [part.strip() for part in adapters.split(",") if part.strip()]
    unknown = [item for item in adapters if item not in ALLOWED_ADAPTERS]
    if unknown:
        raise InstanceError(f"unknown adapters: {unknown}")
    types = ((data.get("graph") or {}).get("enabled_types")) or list(DEFAULT_TYPES)
    return {
        "version": int(data.get("version") or 1),
        "cell": {"name": str(cell["name"]).strip(), "purpose": str(cell["purpose"]).strip()},
        "systems": normalized_systems,
        "trackers": normalized_trackers,
        "vault": {"remote": str(((data.get("vault") or {}).get("remote") or "")).strip()},
        "sources": {
            "github_org": str(((data.get("sources") or {}).get("github_org") or "")).strip(),
            "repo_prefixes": list(((data.get("sources") or {}).get("repo_prefixes") or [])),
            "discovery_roots": list(((data.get("sources") or {}).get("discovery_roots") or [])),
            "schema_repository": (data.get("sources") or {}).get("schema_repository") or {},
        },
        "graph": {"enabled_types": list(types)},
        "evidence": {"profile": profile},
        "adapters": list(adapters),
        "locale": {"notes": locale},
    }


def load_instance(path: Path) -> dict[str, Any]:
    if not path.is_file():
        raise InstanceError(f"missing {path}")
    return validate_instance(_parse_simple_yaml(path.read_text(encoding="utf-8")))


def pending_inventory_listed(home_text: str) -> bool:
    """True only when Home has a Pending inventory heading with documented remotes."""
    marker = "## Pending inventory"
    if marker not in home_text:
        return False
    section = home_text.split(marker, 1)[1].split("\n## ", 1)[0]
    for line in section.splitlines():
        stripped = line.strip()
        if stripped.startswith("- ") and "No discovery roots" not in stripped:
            return True
    return False


def orientation_status(vault_root: Path) -> dict[str, Any]:
    issues: list[str] = []
    instance_path = vault_root / "instance.yaml"
    home = vault_root / "00-Home.md"
    try:
        instance = load_instance(instance_path)
    except InstanceError as error:
        return {
            "ready": False,
            "reason": "complete-bootstrap",
            "issues": [str(error)],
            "start_here": ["instance.yaml", "00-Home.md"],
        }
    if not home.is_file():
        issues.append("00-Home.md is missing")
    systems_dir = vault_root / "10-Sistemas"
    for system in instance["systems"]:
        note = systems_dir / f"{system['name']}.md"
        if not note.is_file():
            issues.append(f"missing system note: 10-Sistemas/{system['name']}.md")
    missing_markers = [marker for marker in MARKERS if not (vault_root / marker).is_file()]
    issues.extend(f"missing marker: {marker}" for marker in missing_markers)
    ready = not issues
    return {
        "ready": ready,
        "reason": "ready" if ready else "complete-bootstrap",
        "cell": instance["cell"],
        "systems": [item["name"] for item in instance["systems"]],
        "issues": issues,
        "start_here": ["00-Home.md", "instance.yaml", "10-Sistemas/"],
        "evidence_profile": instance["evidence"]["profile"],
        "enabled_types": instance["graph"]["enabled_types"],
        "pending_inventory": pending_inventory_listed(home.read_text(encoding="utf-8"))
        if home.is_file()
        else False,
    }


def to_json(data: dict[str, Any]) -> str:
    return json.dumps(data, indent=2, ensure_ascii=False)
