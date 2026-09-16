"""Load and validate a cell instance.yaml. No third-party YAML dependency."""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

ALLOWED_ADAPTERS = ("reports",)
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
KEBAB_CASE_RE = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*")


class InstanceError(ValueError):
    pass


def _parse_simple_yaml(text: str) -> dict[str, Any]:
    """Use the same closed parser regardless of optional installed packages."""
    try:
        return _parse_minimal_yaml(text)
    except InstanceError as error:
        # Public callers persist this stable error code in their reports.
        raise InstanceError("invalid instance.yaml syntax") from error


def _syntax(detail: str) -> InstanceError:
    return InstanceError(f"invalid instance.yaml syntax: {detail}")


def _outside_quotes(value: str, *, flow: bool = False, comments: bool = False) -> list[int]:
    """Scan syntax positions; quotes only delimit a scalar at its beginning.

    Quotes and brackets inside a block plain scalar are literal text. Flow-list
    commas delimit new scalars, while a block plain scalar may contain commas.
    """
    positions = []
    quote = None
    scalar_start = True
    index = 0
    while index < len(value):
        char = value[index]
        if quote == '"' and char == "\\":
            index += 2
            continue
        if quote == "'" and char == "'" and value[index:index + 2] == "''":
            index += 2
            continue
        if quote:
            if char == quote:
                quote = None
                scalar_start = False
        elif comments and char == "#" and (index == 0 or value[index - 1].isspace()):
            positions.append(index)
            return positions
        elif scalar_start and char in "\"'":
            quote = char
        else:
            positions.append(index)
            if char.isspace():
                pass
            elif char == ":" and (index + 1 == len(value) or value[index + 1].isspace()):
                scalar_start = True
            elif scalar_start and char == "[":
                flow = True
            elif flow and char == ",":
                scalar_start = True
            elif scalar_start and char == "-" and index + 1 < len(value) and value[index + 1].isspace():
                pass
            else:
                scalar_start = False
        index += 1
    if quote:
        raise _syntax("unterminated quoted scalar")
    return positions


def _mapping_entry(value: str) -> tuple[str, str] | None:
    for index in _outside_quotes(value):
        if value[index] == ":" and (index + 1 == len(value) or value[index + 1].isspace()):
            key = _scalar(value[:index].strip())
            if not isinstance(key, str) or not key:
                raise _syntax("mapping keys must be non-empty strings")
            return key, value[index + 1:].strip()
    return None


def _parse_minimal_yaml(text: str) -> dict[str, Any]:
    """Parse block maps/sequences and single-line scalar lists, without YAML deps.

    Strings may be plain, JSON double-quoted, or YAML single-quoted. Comments,
    booleans, decimal integers and empty {} are supported. Other YAML features
    (tags, anchors, aliases, multiline strings and flow maps) fail explicitly.
    """
    lines: list[tuple[int, str]] = []
    for line in text.splitlines():
        if "\t" in line[:len(line) - len(line.lstrip())]:
            raise _syntax("tabs in indentation")
        # Comment text is not syntax; the shared scan respects scalar context.
        positions = _outside_quotes(line, comments=True)
        for index in positions:
            if line[index] == "#" and (index == 0 or line[index - 1].isspace()):
                line = line[:index]
                break
        if not line.strip():
            continue
        stripped = line.strip()
        lines.append((len(line) - len(line.lstrip(" ")), stripped))

    def block(index: int, indent: int) -> tuple[Any, int]:
        sequence = lines[index][1].startswith("- ") or lines[index][1] == "-"
        result: Any = [] if sequence else {}
        while index < len(lines) and lines[index][0] == indent:
            value = lines[index][1]
            is_item = value.startswith("- ") or value == "-"
            if is_item != sequence:
                raise _syntax("mixed mapping and sequence")
            if sequence:
                value = value[1:].strip()
                entry = _mapping_entry(value)
                if entry is not None:
                    # A compact mapping item has its first key two columns
                    # beyond the sequence indentation, like emitted YAML.
                    lines[index] = (indent + 2, value)
                    item, index = block(index, indent + 2)
                elif not value and index + 1 < len(lines) and lines[index + 1][0] > indent:
                    item, index = block(index + 1, lines[index + 1][0])
                else:
                    item = _scalar(value)
                    index += 1
                result.append(item)
            else:
                entry = _mapping_entry(value)
                if entry is None:
                    raise _syntax("expected mapping entry")
                key, raw = entry
                if key in result:
                    raise _syntax(f"duplicate mapping key: {key}")
                index += 1
                if not raw and index < len(lines) and lines[index][0] > indent:
                    item, index = block(index, lines[index][0])
                else:
                    item = _scalar(raw) if raw else {}
                result[key] = item
            if index < len(lines) and lines[index][0] > indent:
                raise _syntax("unexpected indentation")
        return result, index

    if not lines or lines[0][0] != 0:
        raise _syntax("expected root mapping")
    root, end = block(0, 0)
    if end != len(lines) or not isinstance(root, dict):
        raise _syntax("expected root mapping")
    return root


def _scalar(value: str, *, flow: bool = False) -> Any:
    if value == "[]":
        return []
    if value == "{}":
        return {}
    if value.startswith("["):
        if not value.endswith("]"):
            raise _syntax("unterminated flow list")
        inner = value[1:-1].strip()
        if not inner:
            return []
        commas = [index for index in _outside_quotes(inner, flow=True) if inner[index] == ","]
        boundaries = [-1, *commas, len(inner)]
        parts = [inner[left + 1:right].strip() for left, right in zip(boundaries, boundaries[1:])]
        if not parts[-1]:
            parts.pop()  # YAML permits a trailing comma.
        if any(not part for part in parts):
            raise _syntax("empty flow-list item")
        items = [_scalar(part, flow=True) for part in parts]
        if any(isinstance(item, (dict, list)) for item in items):
            raise _syntax("flow lists support scalars only")
        return items
    if value.startswith('"'):
        try:
            result = json.loads(value)
        except json.JSONDecodeError as error:
            raise _syntax("invalid double-quoted scalar") from error
        if not isinstance(result, str):
            raise _syntax("expected quoted string")
        return result
    if value.startswith("'"):
        if re.fullmatch(r"'(?:[^']|'')*'", value) is None:
            raise _syntax("invalid single-quoted scalar")
        return value[1:-1].replace("''", "'")
    if not value or value.startswith(("!", "&", "*", "|", ">", "{", "]", ",", "%", "@", "`")) or value in {"---", "...", "-"}:
        raise _syntax("unsupported scalar syntax")
    if any((flow and value[index] in "[]{}") or (value[index] == ":" and (index + 1 == len(value) or value[index + 1].isspace())) for index in _outside_quotes(value)):
        raise _syntax("unsupported plain scalar syntax")
    if value in {"true", "false"}:
        return value == "true"
    if value in {"null", "Null", "NULL", "~"}:
        return None
    if re.fullmatch(r"-?(?:0|[1-9]\d*)", value):
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
    branches = sources.get("reference_branches") or {}
    if branches:
        lines.append("  reference_branches:")
        for repository, branch in sorted(branches.items()):
            lines.append(f"    {_quote(repository)}: {_quote(branch)}")
    schema = sources.get("schema_repository") or {}
    lines.append("  schema_repository:")
    lines.append(f"    remote: {_quote(schema.get('remote') or '')}")
    lines.append(f"    note: {_quote(schema.get('note') or '')}")
    targets = data.get("database_targets", [])
    if targets:
        lines.append("database_targets:")
        for target in targets:
            lines.append(f"  - id: {_quote(target['id'])}")
            for field in ("system", "environment", "instance", "database", "procedure", "port_key"):
                if field in target:
                    lines.append(f"    {field}: {_quote(target[field])}")
            for field in ("schemas", "repositories"):
                lines.append(f"    {field}: [{', '.join(_quote(v) for v in target[field])}]")
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
    capabilities = data.get("capabilities") or {}
    if capabilities:
        lines.append("capabilities:")
        for name, notes in sorted(capabilities.items()):
            lines.append(f"  {name}: [{', '.join(_quote(note) for note in notes)}]")
    locale = data.get("locale") or {}
    lines.append("locale:")
    lines.append(f"  notes: {locale.get('notes') or 'es'}")
    lines.append("")
    return "\n".join(lines)


def _quote(value: str) -> str:
    return json.dumps(str(value), ensure_ascii=False)


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
        if KEBAB_CASE_RE.fullmatch(system_id) is None:
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
        if KEBAB_CASE_RE.fullmatch(tracker_id) is None:
            raise InstanceError(f"tracker id must be kebab-case: {tracker_id}")
        if KEBAB_CASE_RE.fullmatch(provider) is None:
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
    capabilities = data.get("capabilities") or {}
    if not isinstance(capabilities, dict):
        raise InstanceError("capabilities must map capability names to procedure basenames")
    normalized_capabilities = {}
    for name, notes in capabilities.items():
        if not isinstance(name, str) or KEBAB_CASE_RE.fullmatch(name) is None:
            raise InstanceError("capability names must be kebab-case")
        if not isinstance(notes, list) or not notes or any(
            not isinstance(note, str) or not note.strip() or note != note.strip()
            or any(char in note for char in ("/", "\\", "\n", "\r", "[", "]", "#"))
            or note in {".", ".."} or note.endswith(".md") for note in notes
        ):
            raise InstanceError(f"capabilities.{name} must contain canonical note basenames")
        if len(set(notes)) != len(notes):
            raise InstanceError(f"duplicate procedure in capabilities.{name}")
        normalized_capabilities[name] = list(notes)
    types = ((data.get("graph") or {}).get("enabled_types")) or list(DEFAULT_TYPES)
    targets = validate_database_targets(data.get("database_targets", []), seen)
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
            "reference_branches": validate_reference_branches((data.get("sources") or {}).get("reference_branches", {})),
        },
        "graph": {"enabled_types": list(types)},
        "evidence": {"profile": profile},
        "adapters": list(adapters),
        "locale": {"notes": locale},
        "capabilities": normalized_capabilities,
        "database_targets": targets,
    }


def valid_branch_name(value: Any) -> bool:
    """Portable Git branch-name syntax; never accept a ref expression or option."""
    return bool(
        isinstance(value, str) and value and value not in {"@", "HEAD"}
        and not value.startswith(("-", "refs/"))
        and not value.endswith(".") and ".." not in value and "@{" not in value
        and not re.search(r"[\x00-\x20\x7f~^:?*\[\\]", value)
        and all(part and not part.startswith(".") and not part.endswith(".lock") for part in value.split("/"))
    )


def validate_reference_branches(value: Any) -> dict[str, str]:
    if not isinstance(value, dict):
        raise InstanceError("sources.reference_branches must map repository basenames to branch names")
    for repository, branch in value.items():
        if (not isinstance(repository, str) or re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]*", repository) is None
                or not valid_branch_name(branch)):
            raise InstanceError("sources.reference_branches requires repository basenames and valid Git branch names")
    return dict(value)


def reference_branches(instance: dict[str, Any], repository: str) -> tuple[str, ...]:
    """Explicit repository policy; absent entries retain the legacy ordered fallback."""
    branch = instance.get("sources", {}).get("reference_branches", {}).get(repository)
    return (branch,) if branch else ("main", "master")


def validate_database_targets(targets: Any, systems: set[str]) -> list[dict[str, Any]]:
    """Targets describe access; repositories are optional evidence, never credentials."""
    if not isinstance(targets, list):
        raise InstanceError("database_targets must be a list")
    result, ids = [], set()
    fields = {"id", "system", "environment", "instance", "database", "procedure", "port_key", "schemas", "repositories"}
    for target in targets:
        if not isinstance(target, dict) or set(target) - fields:
            raise InstanceError("invalid database target fields")
        normalized = {}
        for field in fields - {"schemas", "repositories"}:
            if field == "port_key" and field not in target:
                continue
            value = target.get(field)
            if not isinstance(value, str) or not value.strip() or value != value.strip() or any(c in value for c in "\n\r\0"):
                raise InstanceError(f"database target requires {field}")
            normalized[field] = value
        if KEBAB_CASE_RE.fullmatch(normalized["id"]) is None or normalized["id"] in ids:
            raise InstanceError("database target id must be unique kebab-case")
        if normalized["system"] not in systems:
            raise InstanceError("database target system is not declared")
        procedure = normalized["procedure"]
        if any(c in procedure for c in "/\\[]#") or procedure.endswith(".md") or procedure in {".", ".."}:
            raise InstanceError("database target procedure must be a canonical basename")
        for field in ("schemas", "repositories"):
            values = target.get(field, [])
            if not isinstance(values, list) or any(not isinstance(v, str) or not v.strip() or v != v.strip() or any(c in v for c in "\n\r\0") for v in values):
                raise InstanceError(f"database target {field} must be a list of names")
            if len(values) != len(set(values)):
                raise InstanceError(f"duplicate database target {field}")
            normalized[field] = list(values)
        for remote in normalized["repositories"]:
            _canonical_tracker_url(remote)
        ids.add(normalized["id"])
        result.append(normalized)
    return result


def load_instance(path: Path) -> dict[str, Any]:
    if not path.is_file():
        raise InstanceError(f"missing {path}")
    return validate_instance(_parse_simple_yaml(path.read_text(encoding="utf-8")))


def pending_inventory_listed(home_text: str) -> bool:
    """True only when Home has a Pending inventory heading with documented remotes."""
    for marker in ("## Pending inventory", "## Inventario pendiente"):
        if marker not in home_text:
            continue
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
