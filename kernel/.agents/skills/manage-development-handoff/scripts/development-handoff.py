#!/usr/bin/env python3
"""Plan, prepare, apply, validate, or update work-item development handoffs."""
from __future__ import annotations

import argparse
import codecs
import difflib
import hashlib
import hmac
import importlib.util
import io
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import unicodedata
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from ruamel.yaml import YAML
from ruamel.yaml.error import YAMLError

if os.name == "nt":
    import msvcrt
else:
    import fcntl


BUNDLE_SCHEMA_VERSION = 2
HANDOFF_SCHEMA_VERSION = 2
HISTORY_SCHEMA_VERSION = 1
LEGACY_ACTIVE_SCHEMA_VERSION = 1
ACTIVE_SCHEMA_VERSION = 2
CONTROL_SCHEMA_VERSION = 1
DOCUMENTS = ("context.md", "work-item.md", "scope.md")
MANAGED_BEGIN = '<!-- knowledge-os:managed:start id="development handoff" -->'
MANAGED_END = '<!-- knowledge-os:managed:end id="development handoff" -->'
OBSOLETE_MANAGED_BEGIN = "<!-- BEGIN MANAGED: System A-System B DEVELOPMENT HANDOFF -->"
OBSOLETE_MANAGED_END = "<!-- END MANAGED: System A-System B DEVELOPMENT HANDOFF -->"
INSTRUCTION_FILES = ("AGENTS.md", "CLAUDE.md")
IGNORE_RULE = "/.knowledge-os-handoffs/"
STORE_NAME = ".knowledge-os-handoffs"
ACTIVE_NAME = "ACTIVE.yaml"
LOCK_NAME = ".ACTIVE.lock"
TRANSACTION_NAME = ".APPLY.transaction"
TRANSACTION_PREPARE_NAME = ".APPLY.transaction.prepare"
TRANSACTION_COMMITTED_NAME = "COMMITTED"
MANIFEST_NAME = "handoff.yaml"
START_NAME = "START.md"
UPDATES_NAME = "implementation-updates.md"
HANDOFF_STATES = {"active", "ready-for-production", "production"}
MAX_DOCUMENT_BYTES = 1_048_576
MAX_BUNDLE_BYTES = 3_145_728
MAX_AGENTS_BYTES = 32_768
MAX_UPDATES_BYTES = 1_048_576
MAX_CLOSURE_UNTRACKED_PATHS = 10_000
REVISION_RE = re.compile(r"v([0-9]{4})")
UPDATE_HEADING_RE = re.compile(r"^## UPD-([0-9]{3,}) — (.+)$")
TRACKER_NAME_RE = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*")
STORY_ID_RE = re.compile(r"S-[0-9]{3,}")
SAFE_NAME_RE = re.compile(r"[^a-z0-9]+")
REMOTE_CONFIG_RE = re.compile(r"^remote\.([^.]+)\.url$")
FAMILY_RE = re.compile(
    r"[a-z0-9]+(?:-[a-z0-9]+)*--[a-z0-9]+(?:-[a-z0-9]+)*"
)
HIGH_CONFIDENCE_SECRET_PATTERNS = (
    re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----"),
    re.compile(r"\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}\b"),
    re.compile(r"\bgithub_pat_[A-Za-z0-9_]{20,}\b"),
    re.compile(r"\bglpat-[A-Za-z0-9_-]{20,}\b"),
    re.compile(r"\bxox[baprs]-[A-Za-z0-9-]{20,}\b"),
    re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    re.compile(r"\bAIza[0-9A-Za-z_-]{30,}\b"),
    re.compile(r"https?://[^/\s:@]+:[^/\s@]+@"),
)
ASSIGNED_SECRET_RE = re.compile(
    r"""(?ix)
    \b(password|passwd|pwd|client[_-]?secret|api[_-]?key|access[_-]?token)
    \b\s*[:=]\s*
    ["']?([^\s"'#,;]+)
    """
)
SAFE_SECRET_VALUES = {
    "redacted",
    "<redacted>",
    "[redacted]",
    "***",
    "masked",
    "placeholder",
    "example",
    "none",
    "null",
}

SKILL_ROOT = Path(__file__).resolve().parents[1]
AGENTS_BLOCK_ASSET = SKILL_ROOT / "assets/agents-managed-block.md"
START_ASSET = SKILL_ROOT / "assets/start.md"
UPDATES_ASSET = SKILL_ROOT / "assets/implementation-updates.md"

UPDATE_REQUIRED_FIELDS = (
    "Recorded at",
    "Source or trigger",
    "Initial definition affected",
    "Update",
    "Status",
    "Reason or agreement",
    "Impact",
    "Evidence",
    "Related entries",
)
UPDATE_ALLOWED_FIELDS = {*UPDATE_REQUIRED_FIELDS, "Analysis"}
UPDATE_ALLOWED_STATUSES = {
    "proposed",
    "agreed",
    "implemented",
    "rejected",
    "superseded",
}

LOAD_YAML = YAML(typ="safe")
LOAD_YAML.allow_duplicate_keys = False


class HandoffError(Exception):
    """Expected, safely reportable contract or state error."""

    def __init__(
        self,
        code: str,
        message: str,
        *,
        exit_code: int = 2,
        **details: object,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.exit_code = exit_code
        self.details = details

    def payload(self) -> dict[str, object]:
        error: dict[str, object] = {"code": self.code, "message": self.message}
        error.update(self.details)
        return {"status": "error", "error": error}


@dataclass(frozen=True)
class Document:
    name: str
    raw: bytes
    text: str
    semantic_text: str
    sha256: str
    semantic_sha256: str


@dataclass(frozen=True)
class Bundle:
    root: Path
    raw_metadata: bytes
    source: dict[str, str]
    work_item: dict[str, str]
    repository_remote: str
    change_summary: str
    change_reasons: dict[str, str]
    documents: dict[str, Document]
    fingerprint: str


@dataclass(frozen=True)
class ExistingHandoff:
    manifest: dict[str, Any]
    revision: str
    manifest_path: Path
    family_path: Path


@dataclass
class ApplyPlan:
    output: dict[str, Any]
    target: Path
    normalized_remote: str
    bundle: Bundle
    family: str
    handoff_id: str
    active: dict[str, Any] | None
    existing: ExistingHandoff | None
    changed_documents: list[str]
    unchanged_documents: list[str]
    desired_instructions: dict[str, bytes]
    desired_ignore: bytes
    instruction_actions: dict[str, str]
    ignore_action: str
    updates_action: str


@dataclass
class CompleteHandoffPlan:
    output: dict[str, Any]
    worktree: WorktreePlan
    materialization: ApplyPlan


@dataclass
class WorktreePlan:
    output: dict[str, Any]
    bundle: Bundle
    source: Path
    normalized_remote: str
    remote_name: str
    base_branch: str
    base_source: str
    selected_commit: str
    branch: str
    path: Path
    fetch_required: bool
    remote_tracking_commit: str | None
    tracking_update_required: bool


def sha256(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def now_utc() -> str:
    return (
        datetime.now(timezone.utc)
        .replace(microsecond=0)
        .isoformat()
        .replace("+00:00", "Z")
    )


def canonical_json(value: object) -> bytes:
    return json.dumps(
        value,
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")


def emit_json(value: object, *, stream: Any = sys.stdout) -> None:
    json.dump(value, stream, ensure_ascii=False, sort_keys=True)
    stream.write("\n")


def yaml_bytes(value: object) -> bytes:
    yaml = YAML()
    yaml.default_flow_style = False
    yaml.allow_unicode = True
    yaml.indent(mapping=2, sequence=4, offset=2)
    stream = io.StringIO()
    yaml.dump(value, stream)
    return stream.getvalue().encode("utf-8")


def load_yaml_mapping(path: Path, *, label: str) -> tuple[dict[str, Any], bytes]:
    if path.is_symlink():
        raise HandoffError(
            "unsafe_symlink",
            f"{label} must not be a symbolic link",
            path=str(path),
        )
    try:
        raw = path.read_bytes()
    except FileNotFoundError as error:
        raise HandoffError(
            "missing_file",
            f"{label} is required",
            path=str(path),
        ) from error
    except OSError as error:
        raise HandoffError(
            "read_failed",
            f"{label} could not be read",
            path=str(path),
            error_type=type(error).__name__,
        ) from error
    try:
        value = LOAD_YAML.load(raw.decode("utf-8"))
    except (UnicodeDecodeError, YAMLError, ValueError) as error:
        raise HandoffError(
            "invalid_yaml",
            f"{label} must contain valid UTF-8 YAML",
            path=str(path),
            error_type=type(error).__name__,
        ) from error
    if not isinstance(value, dict):
        raise HandoffError(
            "invalid_type",
            f"{label} must contain a mapping",
            path=str(path),
        )
    return value, raw


def expect_exact_keys(
    value: dict[str, Any],
    allowed: set[str],
    *,
    field: str,
    required: set[str] | None = None,
) -> None:
    required = allowed if required is None else required
    unknown = sorted(str(key) for key in value if key not in allowed)
    missing = sorted(key for key in required if key not in value)
    if unknown:
        raise HandoffError(
            "unknown_bundle_key",
            f"{field} contains unsupported keys",
            field=field,
            keys=unknown,
        )
    if missing:
        raise HandoffError(
            "missing_bundle_key",
            f"{field} is missing required keys",
            field=field,
            keys=missing,
        )


def expect_mapping(value: object, *, field: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise HandoffError(
            "invalid_type",
            f"{field} must be a mapping",
            field=field,
        )
    return value


def expect_string(
    value: object,
    *,
    field: str,
    maximum: int = 2_000,
) -> str:
    if not isinstance(value, str) or not value.strip():
        raise HandoffError(
            "invalid_string",
            f"{field} must be a non-empty string",
            field=field,
        )
    result = value.strip()
    if len(result) > maximum:
        raise HandoffError(
            "value_too_large",
            f"{field} exceeds its maximum length",
            field=field,
            maximum=maximum,
        )
    return result


def expect_timestamp(value: object, *, field: str) -> str:
    raw = expect_string(value, field=field, maximum=64)
    try:
        parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError as error:
        raise HandoffError(
            "invalid_timestamp",
            f"{field} must be an ISO-8601 timestamp",
            field=field,
        ) from error
    if parsed.tzinfo is None or parsed.utcoffset() is None:
        raise HandoffError(
            "invalid_timestamp",
            f"{field} must include a UTC offset",
            field=field,
        )
    return raw


def normalize_tracker_url(value: object) -> str:
    raw = expect_string(value, field="work-item.tracker-url", maximum=512)
    parsed = urlparse(raw)
    try:
        port = parsed.port
    except ValueError as error:
        raise HandoffError(
            "invalid_tracker_url",
            "work-item.tracker-url contains an invalid port",
            field="work-item.tracker-url",
        ) from error
    if (
        parsed.scheme.casefold() != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        raise HandoffError(
            "invalid_tracker_url",
            "work-item.tracker-url must be a credential-free HTTPS base URL",
            field="work-item.tracker-url",
        )
    host = parsed.hostname.casefold()
    if port:
        host = f"{host}:{port}"
    path = parsed.path.rstrip("/")
    return f"https://{host}{path}"


def validate_work_item_url(value: object, *, tracker_url: str) -> str:
    raw = expect_string(value, field="work-item.url", maximum=1_024)
    parsed = urlparse(raw)
    tracker_parsed = urlparse(tracker_url)
    try:
        port = parsed.port
        tracker_port = tracker_parsed.port
    except ValueError as error:
        raise HandoffError(
            "invalid_work_item_url",
            "work-item.url contains an invalid port",
            field="work-item.url",
        ) from error
    if (
        parsed.scheme.casefold() != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.fragment
        or parsed.hostname.casefold() != (tracker_parsed.hostname or "").casefold()
        or (port or 443) != (tracker_port or 443)
    ):
        raise HandoffError(
            "invalid_work_item_url",
            "work-item.url must be a credential-free HTTPS URL on its tracker host",
            field="work-item.url",
        )
    return raw


def semantic_text(text: str) -> str:
    normalized = unicodedata.normalize("NFC", text)
    normalized = normalized.replace("\r\n", "\n").replace("\r", "\n")
    lines = [line.rstrip(" \t") for line in normalized.split("\n")]
    while lines and not lines[0]:
        lines.pop(0)
    while lines and not lines[-1]:
        lines.pop()
    return "\n".join(lines) + "\n"


def scan_for_secrets(name: str, text: str) -> None:
    for pattern in HIGH_CONFIDENCE_SECRET_PATTERNS:
        match = pattern.search(text)
        if match:
            line = text.count("\n", 0, match.start()) + 1
            raise HandoffError(
                "secret_detected",
                "The handoff bundle contains a probable secret value",
                document=name,
                line=line,
            )
    for match in ASSIGNED_SECRET_RE.finditer(text):
        value = match.group(2).strip().casefold()
        if (
            value in SAFE_SECRET_VALUES
            or value.startswith("${")
            or value.startswith("{{")
        ):
            continue
        line = text.count("\n", 0, match.start()) + 1
        raise HandoffError(
            "secret_detected",
            "The handoff bundle contains a probable assigned secret value",
            document=name,
            line=line,
        )


def read_document(bundle_root: Path, name: str) -> Document:
    path = bundle_root / name
    if path.is_symlink():
        raise HandoffError(
            "unsafe_symlink",
            "Bundle documents must not be symbolic links",
            document=name,
        )
    try:
        raw = path.read_bytes()
    except FileNotFoundError as error:
        raise HandoffError(
            "missing_bundle_document",
            "The handoff bundle is incomplete",
            document=name,
        ) from error
    if not raw or len(raw) > MAX_DOCUMENT_BYTES:
        raise HandoffError(
            "invalid_bundle_document_size",
            "Bundle documents must be non-empty and within the size limit",
            document=name,
            maximum=MAX_DOCUMENT_BYTES,
        )
    try:
        text = raw.decode("utf-8-sig")
    except UnicodeDecodeError as error:
        raise HandoffError(
            "invalid_bundle_encoding",
            "Bundle documents must be UTF-8 text",
            document=name,
        ) from error
    scan_for_secrets(name, text)
    normalized = semantic_text(text)
    return Document(
        name=name,
        raw=raw,
        text=text,
        semantic_text=normalized,
        sha256=sha256(raw),
        semantic_sha256=sha256(normalized.encode("utf-8")),
    )


def load_bundle(raw_root: str) -> Bundle:
    root = Path(raw_root)
    if root.is_symlink():
        raise HandoffError(
            "unsafe_symlink",
            "The bundle directory must not be a symbolic link",
            path=str(root),
        )
    try:
        root = root.resolve(strict=True)
    except OSError as error:
        raise HandoffError(
            "invalid_bundle_path",
            "The bundle directory could not be resolved",
            path=str(root),
        ) from error
    if not root.is_dir():
        raise HandoffError(
            "invalid_bundle_path",
            "The bundle path must be a directory",
            path=str(root),
        )

    metadata, raw_metadata = load_yaml_mapping(
        root / "bundle.yaml",
        label="bundle.yaml",
    )
    expect_exact_keys(
        metadata,
        {"schema-version", "source", "work-item", "repository", "change"},
        field="bundle",
    )
    if metadata["schema-version"] != BUNDLE_SCHEMA_VERSION:
        raise HandoffError(
            "unsupported_bundle_schema",
            "The bundle schema version is unsupported",
            expected=BUNDLE_SCHEMA_VERSION,
        )

    source_value = expect_mapping(metadata["source"], field="source")
    expect_exact_keys(
        source_value,
        {"investigation-id", "investigation-updated-at", "story-id"},
        field="source",
    )
    source = {
        "investigation-id": expect_string(
            source_value["investigation-id"],
            field="source.investigation-id",
            maximum=200,
        ),
        "investigation-updated-at": expect_timestamp(
            source_value["investigation-updated-at"],
            field="source.investigation-updated-at",
        ),
        "story-id": expect_string(
            source_value["story-id"],
            field="source.story-id",
            maximum=32,
        ),
    }
    if not STORY_ID_RE.fullmatch(source["story-id"]):
        raise HandoffError(
            "invalid_story_id",
            "source.story-id must use the canonical S-NNN form",
            field="source.story-id",
        )

    work_item_value = expect_mapping(metadata["work-item"], field="work-item")
    expect_exact_keys(
        work_item_value,
        {
            "tracker-id",
            "provider",
            "tracker-url",
            "reference",
            "url",
            "updated-at",
            "captured-at",
            "freshness",
            "snapshot-source",
        },
        field="work-item",
    )
    tracker_id = expect_string(
        work_item_value["tracker-id"], field="work-item.tracker-id", maximum=64
    )
    provider = expect_string(
        work_item_value["provider"], field="work-item.provider", maximum=64
    )
    if TRACKER_NAME_RE.fullmatch(tracker_id) is None or TRACKER_NAME_RE.fullmatch(provider) is None:
        raise HandoffError(
            "invalid_work_item_identity",
            "work-item tracker-id and provider must use canonical kebab-case",
            field="work-item",
        )
    reference = expect_string(
        work_item_value["reference"], field="work-item.reference", maximum=200
    )
    tracker_url = normalize_tracker_url(work_item_value["tracker-url"])
    freshness = expect_string(
        work_item_value["freshness"],
        field="work-item.freshness",
        maximum=16,
    )
    if freshness != "current":
        raise HandoffError(
            "stale_work_item_snapshot",
            "The copied work-item snapshot must be verified current before handoff",
            field="work-item.freshness",
        )
    snapshot_source = expect_string(
        work_item_value["snapshot-source"],
        field="work-item.snapshot-source",
        maximum=32,
    )
    if snapshot_source not in {"connected-readback", "user-supplied-export"}:
        raise HandoffError(
            "invalid_work_item_snapshot_source",
            "work-item.snapshot-source must identify a supported exact-copy provenance",
            field="work-item.snapshot-source",
        )
    work_item = {
        "tracker-id": tracker_id,
        "provider": provider,
        "tracker-url": tracker_url,
        "reference": reference,
        "url": validate_work_item_url(
            work_item_value["url"],
            tracker_url=tracker_url,
        ),
        "updated-at": expect_timestamp(
            work_item_value["updated-at"],
            field="work-item.updated-at",
        ),
        "captured-at": expect_timestamp(
            work_item_value["captured-at"],
            field="work-item.captured-at",
        ),
        "freshness": freshness,
        "snapshot-source": snapshot_source,
    }

    repository_value = expect_mapping(metadata["repository"], field="repository")
    expect_exact_keys(repository_value, {"remote"}, field="repository")
    repository_remote = expect_string(
        repository_value["remote"],
        field="repository.remote",
        maximum=1_024,
    )
    parsed_remote = urlparse(repository_remote)
    if "://" in repository_remote and (
        parsed_remote.username or parsed_remote.password
    ):
        raise HandoffError(
            "credentialed_repository_remote",
            "repository.remote must not contain userinfo or credentials",
            field="repository.remote",
        )

    change_value = expect_mapping(metadata["change"], field="change")
    expect_exact_keys(
        change_value,
        {"summary", "reasons"},
        field="change",
        required={"summary"},
    )
    change_summary = expect_string(
        change_value["summary"],
        field="change.summary",
        maximum=500,
    )
    reasons_value = change_value.get("reasons", {})
    if reasons_value is None:
        reasons_value = {}
    reasons_mapping = expect_mapping(reasons_value, field="change.reasons")
    unknown_reasons = sorted(str(key) for key in reasons_mapping if key not in DOCUMENTS)
    if unknown_reasons:
        raise HandoffError(
            "unknown_change_reason",
            "change.reasons may reference only canonical handoff documents",
            keys=unknown_reasons,
        )
    change_reasons = {
        str(name): expect_string(
            value,
            field=f"change.reasons.{name}",
            maximum=500,
        )
        for name, value in reasons_mapping.items()
    }

    documents = {name: read_document(root, name) for name in DOCUMENTS}
    if reference.casefold() not in documents["work-item.md"].text.casefold():
        raise HandoffError(
            "work_item_snapshot_identity_mismatch",
            "work-item.md must identify the copied work item",
            document="work-item.md",
        )
    total_bytes = len(raw_metadata) + sum(
        len(document.raw) for document in documents.values()
    )
    if total_bytes > MAX_BUNDLE_BYTES:
        raise HandoffError(
            "bundle_too_large",
            "The handoff bundle exceeds the total size limit",
            maximum=MAX_BUNDLE_BYTES,
        )
    fingerprint = sha256(
        canonical_json(
            {
                "metadata": sha256(raw_metadata),
                "documents": {
                    name: document.sha256
                    for name, document in sorted(documents.items())
                },
            }
        )
    )
    return Bundle(
        root=root,
        raw_metadata=raw_metadata,
        source=source,
        work_item=work_item,
        repository_remote=repository_remote,
        change_summary=change_summary,
        change_reasons=change_reasons,
        documents=documents,
        fingerprint=fingerprint,
    )


def resolve_vault(raw_root: str) -> Path:
    try:
        root = Path(raw_root).resolve(strict=True)
    except OSError as error:
        raise HandoffError(
            "invalid_vault_root",
            "The vault root could not be resolved",
        ) from error
    helper = root / "90-Meta/workspace-config.py"
    if not helper.is_file() or helper.is_symlink():
        raise HandoffError(
            "missing_configuration_helper",
            "The vault does not expose the versioned configuration helper",
        )
    return root


def validate_tracker_binding(vault_root: Path, bundle: Bundle) -> None:
    instance_path = vault_root / "instance.yaml"
    if not instance_path.is_file():
        raise HandoffError(
            "invalid_tracker_configuration",
            "The vault must contain a valid instance.yaml tracker registry",
            path=str(instance_path),
        )
    instance_helper = vault_root / "90-Meta/instance.py"
    if not instance_helper.is_file() or instance_helper.is_symlink():
        raise HandoffError(
            "invalid_tracker_configuration",
            "The vault does not expose the managed instance validator",
            path=str(instance_helper),
        )
    spec = importlib.util.spec_from_file_location(
        "_knowledge_os_instance", instance_helper
    )
    if spec is None or spec.loader is None:
        raise HandoffError(
            "invalid_tracker_configuration",
            "The managed instance validator could not be loaded",
            path=str(instance_helper),
        )
    instance_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(instance_module)
    try:
        instance = instance_module.load_instance(instance_path)
    except instance_module.InstanceError as error:
        raise HandoffError(
            "invalid_tracker_configuration",
            "The vault must contain a valid instance.yaml tracker registry",
            path=str(instance_path),
            reason=str(error),
        ) from error
    trackers = instance.get("trackers", [])
    tracker_id = bundle.work_item["tracker-id"]
    matches = [
        item
        for item in trackers
        if isinstance(item, dict) and item.get("id") == tracker_id
    ]
    if len(matches) != 1:
        raise HandoffError(
            "unknown_tracker",
            "The work item must reference exactly one configured tracker",
            tracker_id=tracker_id,
        )
    configured = matches[0]
    if (
        configured.get("provider") != bundle.work_item["provider"]
        or normalize_tracker_url(configured.get("url"))
        != bundle.work_item["tracker-url"]
    ):
        raise HandoffError(
            "tracker_binding_mismatch",
            "The work-item tracker binding does not match instance.yaml",
            tracker_id=tracker_id,
        )


def resolve_repository(
    vault_root: Path,
    remote: str,
) -> tuple[Path, str]:
    command = [
        sys.executable,
        "-B",
        str(vault_root / "90-Meta/workspace-config.py"),
        "--vault-root",
        str(vault_root),
        "locate-repository",
        remote,
        "--format",
        "json",
    ]
    result = subprocess.run(
        command,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        cause = "configuration_error"
        try:
            payload = json.loads(result.stderr or result.stdout)
            if isinstance(payload, dict):
                error = payload.get("error")
                if isinstance(error, dict) and isinstance(error.get("code"), str):
                    cause = error["code"]
                elif isinstance(payload.get("status"), str):
                    cause = f"repository_{payload['status']}"
        except json.JSONDecodeError:
            pass
        raise HandoffError(
            "repository_resolution_failed",
            "The repository remote did not resolve uniquely through workspace configuration",
            exit_code=3,
            cause=cause,
        )
    try:
        payload = json.loads(result.stdout)
        target = Path(payload["path"]).resolve(strict=True)
        normalized_remote = str(payload["remote"])
    except (json.JSONDecodeError, KeyError, TypeError, OSError) as error:
        raise HandoffError(
            "invalid_repository_resolution",
            "The workspace configuration helper returned an invalid repository view",
            exit_code=3,
        ) from error
    if not target.is_dir():
        raise HandoffError(
            "invalid_repository_resolution",
            "The resolved repository path is not a directory",
            exit_code=3,
        )
    git_root = subprocess.run(
        ["git", "-C", str(target), "rev-parse", "--show-toplevel"],
        text=True,
        capture_output=True,
        check=False,
    )
    if git_root.returncode != 0:
        raise HandoffError(
            "invalid_target_repository",
            "The resolved target is not a Git worktree",
            exit_code=3,
        )
    try:
        observed_root = Path(git_root.stdout.strip()).resolve(strict=True)
    except OSError as error:
        raise HandoffError(
            "invalid_target_repository",
            "The target Git root could not be resolved",
            exit_code=3,
        ) from error
    if observed_root != target:
        raise HandoffError(
            "repository_root_mismatch",
            "The configured target must be the exact Git worktree root",
            exit_code=3,
        )
    return target, normalized_remote


def resolve_worktree_root(vault_root: Path) -> Path:
    command = [
        sys.executable,
        "-B",
        str(vault_root / "90-Meta/workspace-config.py"),
        "--vault-root",
        str(vault_root),
        "development-worktree-root",
        "--format",
        "json",
    ]
    result = subprocess.run(
        command,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        cause = "configuration_error"
        try:
            payload = json.loads(result.stderr or result.stdout)
            if isinstance(payload, dict):
                error = payload.get("error")
                if isinstance(error, dict) and isinstance(error.get("code"), str):
                    cause = error["code"]
                elif payload.get("worktree_root") is None:
                    cause = "worktree_root_not_configured"
        except json.JSONDecodeError:
            pass
        raise HandoffError(
            "worktree_root_resolution_failed",
            "The development worktree root is not configured or available",
            exit_code=3,
            cause=cause,
        )
    try:
        payload = json.loads(result.stdout)
        worktree_root = Path(payload["worktree_root"]).resolve(strict=True)
    except (json.JSONDecodeError, KeyError, TypeError, OSError) as error:
        raise HandoffError(
            "invalid_worktree_root_resolution",
            "The workspace configuration helper returned an invalid worktree root",
            exit_code=3,
        ) from error
    if not worktree_root.is_dir():
        raise HandoffError(
            "invalid_worktree_root_resolution",
            "The configured development worktree root is not a directory",
            exit_code=3,
        )
    return worktree_root


def normalize_remote(remote: str) -> str | None:
    value = remote.strip()
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
    return f"{host.casefold()}/{path.casefold()}" if host and path else None


def repository_basename(remote: str) -> str:
    value = remote.strip()
    scp = re.fullmatch(r"(?:[^@]+@)?([^:]+):(.+)", value)
    if scp and "://" not in value:
        _, path = scp.groups()
    else:
        parsed = urlparse(value if "://" in value else f"https://{value}")
        path = parsed.path
    path = path.strip("/")
    if path.casefold().endswith(".git"):
        path = path[:-4]
    name = path.rsplit("/", 1)[-1]
    if (
        name in {"", ".", ".."}
        or "/" in name
        or "\\" in name
        or re.fullmatch(r"[A-Za-z0-9._-]+", name) is None
    ):
        raise HandoffError(
            "invalid_repository_basename",
            "The repository remote does not provide a safe canonical directory name",
            exit_code=3,
        )
    return name


def matching_remote_name(source: Path, normalized_remote: str) -> str:
    result = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(source),
            "config",
            "--get-regexp",
            r"^remote\..*\.url$",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    matches: set[str] = set()
    if result.returncode in {0, 1}:
        for line in result.stdout.splitlines():
            parts = line.split(None, 1)
            if len(parts) != 2:
                continue
            key, remote = parts
            match = REMOTE_CONFIG_RE.fullmatch(key)
            if match and normalize_remote(remote) == normalized_remote:
                matches.add(match.group(1))
    if not matches:
        raise HandoffError(
            "repository_remote_missing",
            "The source checkout has no Git remote matching the configured repository",
            exit_code=3,
        )
    if "origin" in matches:
        selected = "origin"
    elif len(matches) == 1:
        selected = next(iter(matches))
    else:
        raise HandoffError(
            "ambiguous_repository_remote",
            "More than one Git remote matches the configured repository",
            exit_code=3,
            remotes=sorted(matches),
        )
    if re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", selected) is None:
        raise HandoffError(
            "unsafe_repository_remote",
            "The matching Git remote name is not safe for automated use",
            exit_code=3,
        )
    return selected


def observed_repository_basename(source: Path, normalized_remote: str) -> str:
    remote_name = matching_remote_name(source, normalized_remote)
    result = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(source),
            "config",
            "--get",
            f"remote.{remote_name}.url",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    observed_remote = result.stdout.strip()
    if result.returncode != 0 or not observed_remote:
        raise HandoffError(
            "repository_remote_url_missing",
            "The matching Git remote does not expose its configured URL",
            exit_code=3,
            remote=remote_name,
        )
    if normalize_remote(observed_remote) != normalized_remote:
        raise HandoffError(
            "repository_remote_changed",
            "The matching Git remote changed while resolving its repository name",
            exit_code=3,
            remote=remote_name,
        )
    return repository_basename(observed_remote)


def rev_parse_optional(repository: Path, ref: str) -> str | None:
    result = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(repository),
            "rev-parse",
            "--verify",
            "--quiet",
            f"{ref}^{{commit}}",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    return result.stdout.strip() if result.returncode == 0 else None


def remote_branch_commit(
    source: Path,
    remote_name: str,
    base_branch: str,
) -> str:
    environment = os.environ.copy()
    environment["GIT_TERMINAL_PROMPT"] = "0"
    result = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(source),
            "ls-remote",
            "--exit-code",
            remote_name,
            f"refs/heads/{base_branch}",
        ],
        text=True,
        capture_output=True,
        env=environment,
        timeout=30,
        check=False,
    )
    if result.returncode != 0:
        raise HandoffError(
            "base_branch_remote_unavailable",
            "The selected base branch could not be verified on its matching remote",
            exit_code=3,
            base_branch=base_branch,
            remote=remote_name,
        )
    lines = [line.split()[0] for line in result.stdout.splitlines() if line.split()]
    if len(lines) != 1 or re.fullmatch(r"[0-9a-fA-F]{40,64}", lines[0]) is None:
        raise HandoffError(
            "invalid_remote_branch_resolution",
            "The selected remote base branch did not resolve to one exact commit",
            exit_code=3,
        )
    return lines[0].casefold()


def current_branch(repository: Path) -> str:
    result = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(repository),
            "symbolic-ref",
            "--quiet",
            "--short",
            "HEAD",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0 or not result.stdout.strip():
        raise HandoffError(
            "detached_worktree",
            "A development handoff requires a worktree attached to a branch",
            exit_code=3,
            path=str(repository),
        )
    return result.stdout.strip()


def git_common_directory(repository: Path) -> Path:
    result = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(repository),
            "rev-parse",
            "--path-format=absolute",
            "--git-common-dir",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise HandoffError(
            "invalid_target_repository",
            "The target does not expose Git worktree metadata",
            exit_code=3,
        )
    try:
        return Path(result.stdout.strip()).resolve(strict=True)
    except OSError as error:
        raise HandoffError(
            "invalid_target_repository",
            "The target Git common directory could not be resolved",
            exit_code=3,
        ) from error


def resolve_explicit_worktree(
    vault_root: Path,
    remote: str,
    raw_path: str,
) -> tuple[Path, str, str]:
    source, normalized_remote = resolve_repository(vault_root, remote)
    worktree_root = resolve_worktree_root(vault_root)
    requested = Path(raw_path)
    if not requested.is_absolute():
        raise HandoffError(
            "invalid_worktree_path",
            "The worktree path must be absolute",
            exit_code=3,
        )
    try:
        target = requested.resolve(strict=True)
    except OSError as error:
        raise HandoffError(
            "worktree_path_missing",
            "The requested worktree path does not exist",
            exit_code=3,
        ) from error
    if os.path.normcase(str(requested)) != os.path.normcase(str(target)):
        raise HandoffError(
            "invalid_worktree_path",
            "The worktree path must be normalized and contain no symlink aliases",
            exit_code=3,
        )
    if worktree_root not in target.parents:
        raise HandoffError(
            "worktree_outside_configured_root",
            "The requested worktree is outside the configured development worktree root",
            exit_code=3,
        )
    relative_parts = target.relative_to(worktree_root).parts
    if len(relative_parts) != 2:
        raise HandoffError(
            "invalid_worktree_layout",
            "The worktree path must use <root>/<repository>/<work-item-description>",
            exit_code=3,
        )
    expected_repository = observed_repository_basename(source, normalized_remote)
    if relative_parts[0] != expected_repository:
        raise HandoffError(
            "worktree_repository_directory_mismatch",
            "The worktree must be inside the canonical repository directory",
            exit_code=3,
            expected=expected_repository,
        )
    git_root = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(target),
            "rev-parse",
            "--show-toplevel",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if git_root.returncode != 0:
        raise HandoffError(
            "invalid_target_repository",
            "The requested target is not a Git worktree",
            exit_code=3,
        )
    try:
        observed_root = Path(git_root.stdout.strip()).resolve(strict=True)
    except OSError as error:
        raise HandoffError(
            "invalid_target_repository",
            "The target Git root could not be resolved",
            exit_code=3,
        ) from error
    if observed_root != target or git_common_directory(target) != git_common_directory(source):
        raise HandoffError(
            "worktree_repository_mismatch",
            "The requested target is not a registered worktree of the resolved repository",
            exit_code=3,
        )
    return target, normalized_remote, current_branch(target)


def resolve_handoff_target(
    vault_root: Path,
    remote: str,
    worktree_path: str,
) -> tuple[Path, str, str]:
    return resolve_explicit_worktree(vault_root, remote, worktree_path)


def resolve_branch_target(
    vault_root: Path,
    remote: str,
    branch: str,
) -> dict[str, Any]:
    """Resolve portable repository/branch identity to a verified local worktree."""
    branch = expect_string(branch, field="branch", maximum=255)
    check = subprocess.run(
        ["git", "check-ref-format", "--branch", branch],
        text=True,
        capture_output=True,
        check=False,
    )
    if check.returncode != 0:
        raise HandoffError("invalid_worktree_branch", "The branch is not a valid Git branch")
    source, normalized_remote = resolve_repository(vault_root, remote)
    worktree_root = resolve_worktree_root(vault_root)
    target = (
        worktree_root
        / observed_repository_basename(source, normalized_remote)
        / branch.rsplit("/", 1)[-1]
    )
    if not target.exists():
        return {
            "status": "unavailable",
            "availability": "not-local",
            "repository_remote": normalized_remote,
            "branch": branch,
            "derived_path": str(target),
        }
    resolved, observed_remote, observed_branch = resolve_explicit_worktree(
        vault_root, normalized_remote, str(target)
    )
    if observed_branch != branch:
        raise HandoffError(
            "worktree_branch_mismatch",
            "The derived worktree is attached to a different branch",
            exit_code=3,
            expected_branch=branch,
            observed_branch=observed_branch,
        )
    return {
        "status": "available",
        "availability": "local-verified",
        "repository_remote": observed_remote,
        "branch": observed_branch,
        "worktree_path": str(resolved),
    }


def slug(value: str) -> str:
    normalized = unicodedata.normalize("NFKD", value.casefold())
    without_marks = "".join(
        character for character in normalized if not unicodedata.combining(character)
    )
    result = SAFE_NAME_RE.sub("-", without_marks).strip("-")
    if not result:
        raise HandoffError(
            "invalid_handoff_identity",
            "The handoff identity could not produce a canonical name",
        )
    return result


def work_item_identity(
    work_item: dict[str, str],
    normalized_remote: str,
) -> tuple[str, str, str]:
    repository_name = normalized_remote.rsplit("/", 1)[-1]
    identity = "|".join(
        (
            work_item["tracker-id"],
            work_item["reference"],
            normalized_remote,
        )
    )
    handoff_id = sha256(identity.encode("utf-8"))
    readable = slug(
        f"{work_item['tracker-id']}-{work_item['reference']}"
    )[:80].rstrip("-")
    token = f"{readable}-{handoff_id[:10]}"
    family = f"{token}--{slug(repository_name)}"
    return family, handoff_id, token


def handoff_identity(
    bundle: Bundle,
    normalized_remote: str,
) -> tuple[str, str, str]:
    return work_item_identity(bundle.work_item, normalized_remote)


def read_asset(path: Path) -> bytes:
    if not path.is_file() or path.is_symlink():
        raise HandoffError(
            "missing_skill_asset",
            "A required development-handoff asset is unavailable",
            asset=path.name,
        )
    raw = path.read_bytes()
    try:
        raw.decode("utf-8")
    except UnicodeDecodeError as error:
        raise HandoffError(
            "invalid_skill_asset",
            "A required development-handoff asset is not UTF-8",
            asset=path.name,
        ) from error
    return raw


def read_optional_file(path: Path, *, label: str) -> bytes | None:
    if path.is_symlink():
        raise HandoffError(
            "unsafe_symlink",
            f"{label} must not be a symbolic link",
            path=str(path),
        )
    try:
        return path.read_bytes()
    except FileNotFoundError:
        return None
    except IsADirectoryError as error:
        raise HandoffError(
            "invalid_file_type",
            f"{label} must be a regular file",
            path=str(path),
        ) from error


def decode_managed_text(raw: bytes | None, *, label: str) -> tuple[str, bool, str]:
    if raw is None:
        return "", False, "\n"
    bom = raw.startswith(codecs.BOM_UTF8)
    try:
        text = raw.decode("utf-8-sig")
    except UnicodeDecodeError as error:
        raise HandoffError(
            "invalid_text_encoding",
            f"{label} must be UTF-8 text",
            path=label,
        ) from error
    crlf = text.count("\r\n")
    bare_lf = text.count("\n") - crlf
    newline = "\r\n" if crlf > bare_lf else "\n"
    normalized = text.replace("\r\n", "\n").replace("\r", "\n")
    return normalized, bom, newline


def encode_managed_text(text: str, *, bom: bool, newline: str) -> bytes:
    converted = text.replace("\n", newline)
    raw = converted.encode("utf-8")
    return codecs.BOM_UTF8 + raw if bom else raw


def managed_section_bounds(text: str, *, label: str) -> tuple[int, int] | None:
    if OBSOLETE_MANAGED_BEGIN in text or OBSOLETE_MANAGED_END in text:
        raise HandoffError(
            "unsupported_managed_block_version",
            f"{label} contains an unsupported managed handoff block",
            path=label,
        )
    begin_count = text.count(MANAGED_BEGIN)
    end_count = text.count(MANAGED_END)
    if begin_count == 0 and end_count == 0:
        return None
    if begin_count != 1 or end_count != 1:
        raise HandoffError(
            "invalid_managed_block",
            f"{label} must contain zero or one complete managed handoff block",
            path=label,
        )
    begin = text.index(MANAGED_BEGIN)
    end_start = text.index(MANAGED_END)
    if end_start < begin:
        raise HandoffError(
            "invalid_managed_block",
            f"The managed handoff markers in {label} are inverted",
            path=label,
        )
    return begin, end_start + len(MANAGED_END)


def instruction_insertion_offset(text: str) -> int:
    lines = text.splitlines(keepends=True)
    index = 0
    if lines and lines[0].rstrip("\n") == "---":
        for candidate in range(1, len(lines)):
            if lines[candidate].rstrip("\n") == "---":
                index = candidate + 1
                break
    while index < len(lines) and not lines[index].strip():
        index += 1
    if index < len(lines) and lines[index].startswith("# "):
        index += 1
    return sum(len(line) for line in lines[:index])


def join_managed_block(block: str, after: str) -> str:
    if not after.strip():
        return block + after
    leading_newlines = len(after) - len(after.lstrip("\n"))
    return block + "\n" * max(0, 2 - leading_newlines) + after


def insert_managed_block(text: str, block: str) -> str:
    offset = instruction_insertion_offset(text)
    before = text[:offset]
    after = text[offset:]
    if before and not before.endswith("\n"):
        before += "\n"
    if before and not before.endswith("\n\n"):
        before += "\n"
    result = before + join_managed_block(block, after)
    if not result.endswith("\n"):
        result += "\n"
    return result


def root_instruction_files(target: Path) -> tuple[str, ...]:
    physical: set[str] = set()
    linked: set[str] = set()
    for name in INSTRUCTION_FILES:
        path = target / name
        if path.is_symlink():
            linked.add(name)
        elif path.exists():
            if not path.is_file():
                raise HandoffError(
                    "invalid_file_type",
                    f"{name} must be a regular file or a symlink to its root counterpart",
                    path=str(path),
                )
            physical.add(name)

    for name in linked:
        path = target / name
        counterpart = "CLAUDE.md" if name == "AGENTS.md" else "AGENTS.md"
        raw_target = os.readlink(path)
        candidate = Path(raw_target)
        if not candidate.is_absolute():
            candidate = path.parent / candidate
        candidate_key = os.path.normcase(os.path.abspath(os.path.normpath(candidate)))
        counterpart_key = os.path.normcase(
            os.path.abspath(os.path.normpath(target / counterpart))
        )
        if candidate_key != counterpart_key or counterpart not in physical:
            raise HandoffError(
                "unsafe_instruction_symlink",
                f"{name} must point directly to the physical root {counterpart}",
                path=str(path),
                target=raw_target,
            )

    if not physical:
        if linked:
            raise HandoffError(
                "unsafe_instruction_symlink",
                "The root instruction files do not resolve to one physical file",
            )
        return ("AGENTS.md",)
    return tuple(name for name in INSTRUCTION_FILES if name in physical)


def prepare_instruction_file(path: Path) -> tuple[bytes, str]:
    label = path.name
    current = read_optional_file(path, label=label)
    text, bom, newline = decode_managed_text(current, label=label)
    block = read_asset(AGENTS_BLOCK_ASSET).decode("utf-8").strip("\n")
    bounds = managed_section_bounds(text, label=label)
    if bounds is None:
        desired_text = insert_managed_block(text, block)
    else:
        begin, end = bounds
        desired_text = text[:begin] + join_managed_block(block, text[end:])
    desired = encode_managed_text(desired_text, bom=bom, newline=newline)
    if len(desired) > MAX_AGENTS_BYTES:
        raise HandoffError(
            "instruction_file_too_large",
            f"The managed block would exceed the supported {label} size",
            path=str(path),
            maximum=MAX_AGENTS_BYTES,
        )
    if current == desired:
        return desired, "none"
    return desired, "create" if current is None else "update"


def prepare_instructions(target: Path) -> tuple[dict[str, bytes], dict[str, str]]:
    override = read_optional_file(
        target / "AGENTS.override.md",
        label="AGENTS.override.md",
    )
    if override is not None and override.strip():
        raise HandoffError(
            "effective_instruction_override",
            "A non-empty root AGENTS.override.md hides the managed AGENTS.md block",
            path=str(target / "AGENTS.override.md"),
        )
    desired: dict[str, bytes] = {}
    actions: dict[str, str] = {}
    for name in root_instruction_files(target):
        desired[name], actions[name] = prepare_instruction_file(target / name)
    return desired, actions


def prepare_gitignore(target: Path) -> tuple[bytes, str]:
    path = target / ".gitignore"
    current = read_optional_file(path, label=".gitignore")
    text, bom, newline = decode_managed_text(current, label=".gitignore")
    lines = text.splitlines()
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("!") and STORE_NAME in stripped:
            raise HandoffError(
                "gitignore_conflict",
                "A .gitignore negation conflicts with the local handoff store",
                path=str(path),
            )
    if IGNORE_RULE in lines:
        return current or b"", "none"
    desired_text = text
    if desired_text and not desired_text.endswith("\n"):
        desired_text += "\n"
    desired_text += IGNORE_RULE + "\n"
    desired = encode_managed_text(desired_text, bom=bom, newline=newline)
    return desired, "create" if current is None else "update"


def invalid_implementation_updates(
    message: str,
    *,
    path: Path,
    entry: str | None = None,
) -> HandoffError:
    details: dict[str, object] = {"path": str(path)}
    if entry is not None:
        details["entry"] = entry
    return HandoffError(
        "invalid_implementation_updates",
        message,
        **details,
    )


def validate_implementation_updates(path: Path) -> dict[str, object]:
    raw = read_optional_file(path, label=UPDATES_NAME)
    if raw is None:
        raise HandoffError(
            "implementation_updates_missing",
            "The registered handoff has no implementation update changelog",
            path=str(path),
        )
    if len(raw) > MAX_UPDATES_BYTES:
        raise invalid_implementation_updates(
            "implementation-updates.md exceeds its maximum size",
            path=path,
        )
    try:
        text = raw.decode("utf-8-sig")
    except UnicodeDecodeError as error:
        raise invalid_implementation_updates(
            "implementation-updates.md must be UTF-8 text",
            path=path,
        ) from error
    if "\x00" in text:
        raise invalid_implementation_updates(
            "implementation-updates.md contains a NUL byte",
            path=path,
        )
    scan_for_secrets(UPDATES_NAME, text)
    normalized = text.replace("\r\n", "\n").replace("\r", "\n")
    lines = normalized.splitlines()
    first_content = next((line for line in lines if line.strip()), "")
    if first_content != "# Implementation updates":
        raise invalid_implementation_updates(
            "implementation-updates.md must start with its canonical H1",
            path=path,
        )

    headings: list[tuple[int, int, str]] = []
    for line_number, line in enumerate(lines):
        if not line.startswith("## "):
            continue
        match = UPDATE_HEADING_RE.fullmatch(line)
        if match is None:
            raise invalid_implementation_updates(
                "Every H2 must be a canonical UPD-NNN changelog entry",
                path=path,
            )
        headings.append((line_number, int(match.group(1)), match.group(2).strip()))

    for position, (start, number, title) in enumerate(headings, start=1):
        entry_id = f"UPD-{number:03d}"
        if number != position or not title:
            raise invalid_implementation_updates(
                "Update IDs must be unique, contiguous, and start at UPD-001",
                path=path,
                entry=entry_id,
            )
        end = headings[position][0] if position < len(headings) else len(lines)
        fields: dict[str, str] = {}
        field_order: list[str] = []
        for line in lines[start + 1 : end]:
            if not line.startswith("- "):
                continue
            field_match = re.fullmatch(r"- ([A-Za-z][A-Za-z ]+):\s*(.*)", line)
            if field_match is None:
                raise invalid_implementation_updates(
                    "Every entry bullet must use a canonical field label",
                    path=path,
                    entry=entry_id,
                )
            label = field_match.group(1)
            value = field_match.group(2).strip()
            if label not in UPDATE_ALLOWED_FIELDS or label in fields:
                raise invalid_implementation_updates(
                    "An update contains an unknown or duplicate field",
                    path=path,
                    entry=entry_id,
                )
            if not value:
                raise invalid_implementation_updates(
                    "Every update field must have a value on the same line",
                    path=path,
                    entry=entry_id,
                )
            fields[label] = value
            field_order.append(label)
        missing = [field for field in UPDATE_REQUIRED_FIELDS if field not in fields]
        if missing:
            raise invalid_implementation_updates(
                "An update is missing one or more required fields",
                path=path,
                entry=entry_id,
            )
        expected_order = list(UPDATE_REQUIRED_FIELDS)
        if "Analysis" in fields:
            expected_order.append("Analysis")
        if field_order != expected_order:
            raise invalid_implementation_updates(
                "Update fields must follow the canonical linear order",
                path=path,
                entry=entry_id,
            )
        try:
            expect_timestamp(
                fields["Recorded at"],
                field=f"{entry_id}.Recorded at",
            )
        except HandoffError as error:
            raise invalid_implementation_updates(
                "Recorded at must be an ISO-8601 timestamp with a UTC offset",
                path=path,
                entry=entry_id,
            ) from error
        if fields["Status"] not in UPDATE_ALLOWED_STATUSES:
            raise invalid_implementation_updates(
                "Status must use one of the allowed changelog states",
                path=path,
                entry=entry_id,
            )
        related = fields["Related entries"]
        related_ids = re.findall(r"\bUPD-([0-9]{3,})\b", related)
        if related.casefold() != "none" and not related_ids:
            raise invalid_implementation_updates(
                "Related entries must be `none` or reference earlier UPD IDs",
                path=path,
                entry=entry_id,
            )
        if any(int(related_id) >= number for related_id in related_ids):
            raise invalid_implementation_updates(
                "Related entries may reference only earlier updates",
                path=path,
                entry=entry_id,
            )
        analysis = fields.get("Analysis")
        if analysis is not None and analysis.casefold() in {
            "none",
            "n/a",
            "not applicable",
            "no analysis",
        }:
            raise invalid_implementation_updates(
                "Omit Analysis when no analysis occurred",
                path=path,
                entry=entry_id,
            )

    return {
        "path": UPDATES_NAME,
        "entries": len(headings),
        "latest": (
            None
            if not headings
            else f"UPD-{headings[-1][1]:03d}"
        ),
    }


def prepare_implementation_updates(
    family_path: Path,
) -> tuple[str, dict[str, object]]:
    path = family_path / UPDATES_NAME
    raw = read_optional_file(path, label=UPDATES_NAME)
    if raw is None:
        return "create", {
            "path": UPDATES_NAME,
            "entries": 0,
            "latest": None,
        }
    return "none", validate_implementation_updates(path)


def ensure_safe_directory(path: Path, *, label: str) -> None:
    if path.is_symlink():
        raise HandoffError(
            "unsafe_symlink",
            f"{label} must not be a symbolic link",
            path=str(path),
        )
    if path.exists() and not path.is_dir():
        raise HandoffError(
            "invalid_file_type",
            f"{label} must be a directory",
            path=str(path),
        )


def validate_relative_path(value: object, *, field: str) -> str:
    raw = expect_string(value, field=field, maximum=256)
    path = Path(raw)
    if path.is_absolute() or ".." in path.parts or str(path) != raw:
        raise HandoffError(
            "unsafe_relative_path",
            f"{field} must be a normalized relative path",
            field=field,
        )
    return raw


def validate_family_files(
    family_path: Path,
    manifest: dict[str, Any],
) -> None:
    files = expect_mapping(manifest.get("files"), field="manifest.files")
    expected_names = {START_NAME, *DOCUMENTS}
    if set(files) != expected_names:
        raise HandoffError(
            "invalid_handoff_manifest",
            "handoff.yaml must index every current handoff document exactly once",
        )
    for name in sorted(expected_names):
        record = expect_mapping(files[name], field=f"manifest.files.{name}")
        allowed = {"sha256"}
        if name in DOCUMENTS:
            allowed.add("semantic-sha256")
        expect_exact_keys(record, allowed, field=f"manifest.files.{name}")
        path = family_path / name
        raw = read_optional_file(path, label=name)
        if raw is None or sha256(raw) != record["sha256"]:
            raise HandoffError(
                "handoff_integrity_error",
                "A materialized handoff file does not match handoff.yaml",
                path=str(path),
            )
        if name in DOCUMENTS:
            try:
                observed_semantic = sha256(
                    semantic_text(raw.decode("utf-8-sig")).encode("utf-8")
                )
            except UnicodeDecodeError as error:
                raise HandoffError(
                    "handoff_integrity_error",
                    "A materialized handoff file is not UTF-8",
                    path=str(path),
                ) from error
            if observed_semantic != record["semantic-sha256"]:
                raise HandoffError(
                    "handoff_integrity_error",
                    "A materialized handoff semantic hash does not match handoff.yaml",
                    path=str(path),
                )

    history = expect_mapping(manifest.get("history"), field="manifest.history")
    expect_exact_keys(history, {"path", "sha256"}, field="manifest.history")
    history_relative = validate_relative_path(
        history["path"],
        field="manifest.history.path",
    )
    expected_history = f"history/{manifest.get('revision')}.md"
    if history_relative != expected_history:
        raise HandoffError(
            "invalid_handoff_manifest",
            "handoff.yaml must reference the event for its current revision",
        )
    history_path = family_path / history_relative
    history_raw = read_optional_file(history_path, label="revision history event")
    if history_raw is None or sha256(history_raw) != history["sha256"]:
        raise HandoffError(
            "handoff_integrity_error",
            "The current history event does not match handoff.yaml",
            path=str(history_path),
        )
    validate_history_chain(
        family_path,
        manifest=manifest,
        current_sha256=str(history["sha256"]),
    )


def markdown_frontmatter(raw: bytes, *, label: str) -> dict[str, Any]:
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as error:
        raise HandoffError(
            "handoff_history_integrity_error",
            "A handoff history event is not UTF-8",
            path=label,
        ) from error
    if not text.startswith("---\n"):
        raise HandoffError(
            "handoff_history_integrity_error",
            "A handoff history event has no YAML frontmatter",
            path=label,
        )
    frontmatter_text, separator, _ = text[4:].partition("\n---\n")
    if not separator:
        raise HandoffError(
            "handoff_history_integrity_error",
            "A handoff history event has incomplete YAML frontmatter",
            path=label,
        )
    try:
        value = LOAD_YAML.load(frontmatter_text)
    except (YAMLError, ValueError) as error:
        raise HandoffError(
            "handoff_history_integrity_error",
            "A handoff history event has invalid YAML frontmatter",
            path=label,
        ) from error
    if not isinstance(value, dict):
        raise HandoffError(
            "handoff_history_integrity_error",
            "A handoff history event frontmatter is not a mapping",
            path=label,
        )
    return value


def validate_history_chain(
    family_path: Path,
    *,
    manifest: dict[str, Any],
    current_sha256: str,
) -> None:
    revision = str(manifest["revision"])
    match = REVISION_RE.fullmatch(revision)
    if match is None:
        raise HandoffError(
            "invalid_handoff_manifest",
            "handoff.yaml contains an invalid revision",
        )
    current_number = int(match.group(1))
    expected_hash: str | None = current_sha256
    handoff_id = str(manifest["handoff-id"])
    for number in range(current_number, 0, -1):
        event_revision = f"v{number:04d}"
        event_path = family_path / "history" / f"{event_revision}.md"
        if event_path.is_symlink() or not event_path.is_file():
            raise HandoffError(
                "handoff_history_integrity_error",
                "A handoff history event is missing or unsafe",
                path=str(event_path),
            )
        raw = event_path.read_bytes()
        if expected_hash is None or sha256(raw) != expected_hash:
            raise HandoffError(
                "handoff_history_integrity_error",
                "A handoff history event breaks the hash chain",
                path=str(event_path),
            )
        event = markdown_frontmatter(raw, label=str(event_path))
        allowed = {
            "schema-version",
            "handoff-id",
            "revision",
            "previous",
            "previous-event-sha256",
            "created-at",
            "reason",
            "changed",
            "unchanged",
        }
        try:
            expect_exact_keys(event, allowed, field=f"history.{event_revision}")
        except HandoffError as error:
            raise HandoffError(
                "handoff_history_integrity_error",
                "A handoff history event has an invalid schema",
                path=str(event_path),
                cause=error.code,
            ) from error
        expected_previous = f"v{number - 1:04d}" if number > 1 else None
        if (
            event["schema-version"] != HISTORY_SCHEMA_VERSION
            or event["handoff-id"] != handoff_id
            or event["revision"] != event_revision
            or event["previous"] != expected_previous
            or not isinstance(event["changed"], list)
            or not isinstance(event["unchanged"], list)
        ):
            raise HandoffError(
                "handoff_history_integrity_error",
                "A handoff history event does not match its chain position",
                path=str(event_path),
            )
        previous_hash = event["previous-event-sha256"]
        if number == 1:
            if previous_hash is not None:
                raise HandoffError(
                    "handoff_history_integrity_error",
                    "The baseline history event must terminate the hash chain",
                    path=str(event_path),
                )
            expected_hash = None
        elif (
            not isinstance(previous_hash, str)
            or re.fullmatch(r"[0-9a-f]{64}", previous_hash) is None
        ):
            raise HandoffError(
                "handoff_history_integrity_error",
                "A handoff history event has an invalid previous-event hash",
                path=str(event_path),
            )
        else:
            expected_hash = previous_hash


def read_existing_handoff(
    family_path: Path,
    *,
    family: str,
    handoff_id: str,
    normalized_remote: str,
) -> ExistingHandoff | None:
    ensure_safe_directory(family_path.parent, label="handoff store")
    ensure_safe_directory(family_path, label="handoff family")
    if not family_path.exists():
        return None
    required_family_entries = {
        MANIFEST_NAME,
        START_NAME,
        *DOCUMENTS,
        "history",
    }
    observed_family_entries = {path.name for path in family_path.iterdir()}
    if (
        not required_family_entries.issubset(observed_family_entries)
        or observed_family_entries
        - required_family_entries
        - {UPDATES_NAME}
    ):
        raise HandoffError(
            "invalid_handoff_layout",
            "The handoff family contains missing or unmanaged paths",
            family=family,
        )
    manifest_path = family_path / MANIFEST_NAME
    manifest, _ = load_yaml_mapping(manifest_path, label=MANIFEST_NAME)
    if manifest.get("schema-version") != HANDOFF_SCHEMA_VERSION:
        raise HandoffError(
            "unsupported_handoff_schema",
            "The materialized handoff schema is unsupported; export a schema-2 work-item package",
            expected=HANDOFF_SCHEMA_VERSION,
            family=family,
        )
    if (
        manifest.get("handoff-id") != handoff_id
        or manifest.get("family") != family
    ):
        raise HandoffError(
            "handoff_family_collision",
            "The canonical handoff family already belongs to another identity",
            family=family,
        )
    repository = expect_mapping(
        manifest.get("repository"),
        field="manifest.repository",
    )
    if repository.get("remote") != normalized_remote:
        raise HandoffError(
            "handoff_family_collision",
            "The canonical handoff family targets a different repository remote",
            family=family,
        )
    source = expect_mapping(manifest.get("source"), field="manifest.source")
    expect_exact_keys(
        source,
        {"investigation-id", "investigation-updated-at", "story-id"},
        field="manifest.source",
    )
    source["investigation-id"] = expect_string(
        source["investigation-id"],
        field="manifest.source.investigation-id",
        maximum=200,
    )
    source["investigation-updated-at"] = expect_timestamp(
        source["investigation-updated-at"],
        field="manifest.source.investigation-updated-at",
    )
    source["story-id"] = expect_string(
        source["story-id"], field="manifest.source.story-id", maximum=32
    )
    if STORY_ID_RE.fullmatch(source["story-id"]) is None:
        raise HandoffError(
            "invalid_handoff_manifest",
            "handoff.yaml contains an invalid source story ID",
        )
    work_item = expect_mapping(
        manifest.get("work-item"), field="manifest.work-item"
    )
    expect_exact_keys(
        work_item,
        {
            "tracker-id",
            "provider",
            "tracker-url",
            "reference",
            "url",
            "updated-at",
            "captured-at",
            "freshness",
            "snapshot-source",
        },
        field="manifest.work-item",
    )
    work_item["tracker-id"] = expect_string(
        work_item["tracker-id"],
        field="manifest.work-item.tracker-id",
        maximum=64,
    )
    work_item["provider"] = expect_string(
        work_item["provider"], field="manifest.work-item.provider", maximum=64
    )
    work_item["reference"] = expect_string(
        work_item["reference"], field="manifest.work-item.reference", maximum=200
    )
    if (
        TRACKER_NAME_RE.fullmatch(work_item["tracker-id"]) is None
        or TRACKER_NAME_RE.fullmatch(work_item["provider"]) is None
    ):
        raise HandoffError(
            "invalid_handoff_manifest",
            "handoff.yaml contains an invalid work-item identity",
        )
    work_item["tracker-url"] = normalize_tracker_url(work_item["tracker-url"])
    work_item["url"] = validate_work_item_url(
        work_item["url"], tracker_url=work_item["tracker-url"]
    )
    revision = manifest.get("revision")
    if not isinstance(revision, str) or REVISION_RE.fullmatch(revision) is None:
        raise HandoffError(
            "invalid_handoff_manifest",
            "handoff.yaml contains an invalid revision",
        )
    history_path = family_path / "history"
    ensure_safe_directory(history_path, label="handoff history")
    if not history_path.is_dir():
        raise HandoffError(
            "invalid_handoff_manifest",
            "The handoff history directory is missing",
        )
    revision_number = int(REVISION_RE.fullmatch(revision).group(1))
    history_entries = list(history_path.iterdir())
    if any(path.is_symlink() or not path.is_file() for path in history_entries):
        raise HandoffError(
            "invalid_handoff_layout",
            "The handoff history contains an unmanaged or unsafe path",
            family=family,
        )
    observed_events = sorted(path.name for path in history_entries)
    expected_events = [f"v{number:04d}.md" for number in range(1, revision_number + 1)]
    if observed_events != expected_events:
        raise HandoffError(
            "invalid_handoff_history",
            "The handoff history must be contiguous and contain no extra files",
            family=family,
        )
    validate_family_files(family_path, manifest)
    return ExistingHandoff(
        manifest=manifest,
        revision=revision,
        manifest_path=manifest_path,
        family_path=family_path,
    )


def read_active(store: Path) -> tuple[dict[str, Any] | None, bytes | None]:
    path = store / ACTIVE_NAME
    if not path.exists() and not path.is_symlink():
        return None, None
    active, raw = load_yaml_mapping(path, label=ACTIVE_NAME)
    schema_version = active.get("schema-version")
    if schema_version == LEGACY_ACTIVE_SCHEMA_VERSION:
        expected = {
            "schema-version",
            "handoff-id",
            "family",
            "revision",
            "manifest",
            "activated-at",
        }
        expect_exact_keys(active, expected, field="ACTIVE.yaml")
        handoff_id = expect_string(
            active["handoff-id"], field="ACTIVE.yaml.handoff-id", maximum=64
        )
        family = expect_string(
            active["family"], field="ACTIVE.yaml.family", maximum=200
        )
        revision = expect_string(
            active["revision"], field="ACTIVE.yaml.revision", maximum=16
        )
        if (
            re.fullmatch(r"[a-f0-9]{64}", handoff_id) is None
            or FAMILY_RE.fullmatch(family) is None
            or REVISION_RE.fullmatch(revision) is None
        ):
            raise HandoffError(
                "invalid_active_pointer",
                "Legacy ACTIVE.yaml contains a non-canonical identity",
            )
        manifest = validate_relative_path(
            active["manifest"], field="ACTIVE.yaml.manifest"
        )
        if manifest != f"{family}/{MANIFEST_NAME}":
            raise HandoffError(
                "invalid_active_pointer",
                "Legacy ACTIVE.yaml must point to its stable family manifest",
            )
        active["handoff-id"] = handoff_id
        active["family"] = family
        active["revision"] = revision
        active["manifest"] = manifest
        active["activated-at"] = expect_timestamp(
            active["activated-at"], field="ACTIVE.yaml.activated-at"
        )
        return active, raw
    if schema_version != ACTIVE_SCHEMA_VERSION:
        raise HandoffError(
            "unsupported_active_schema",
            "ACTIVE.yaml uses an unsupported schema version",
        )
    expected = {"schema-version", "investigation-id", "handoffs"}
    expect_exact_keys(active, expected, field="ACTIVE.yaml")
    investigation_id = expect_string(
        active["investigation-id"],
        field="ACTIVE.yaml.investigation-id",
        maximum=200,
    )
    raw_handoffs = active["handoffs"]
    if not isinstance(raw_handoffs, list) or not raw_handoffs:
        raise HandoffError(
            "invalid_active_pointer",
            "ACTIVE.yaml must contain at least one handoff",
        )
    seen_ids: set[str] = set()
    seen_families: set[str] = set()
    normalized_handoffs: list[dict[str, str]] = []
    entry_keys = {
        "handoff-id",
        "family",
        "revision",
        "manifest",
        "activated-at",
        "state",
    }
    for index, raw_entry in enumerate(raw_handoffs):
        field = f"ACTIVE.yaml.handoffs[{index}]"
        entry = expect_mapping(raw_entry, field=field)
        expect_exact_keys(entry, entry_keys, field=field)
        handoff_id = expect_string(
            entry["handoff-id"], field=f"{field}.handoff-id", maximum=64
        )
        if re.fullmatch(r"[a-f0-9]{64}", handoff_id) is None:
            raise HandoffError(
                "invalid_active_pointer",
                "ACTIVE.yaml contains an invalid handoff ID",
            )
        family = expect_string(entry["family"], field=f"{field}.family", maximum=200)
        revision = expect_string(
            entry["revision"], field=f"{field}.revision", maximum=16
        )
        state = expect_string(entry["state"], field=f"{field}.state", maximum=32)
        if FAMILY_RE.fullmatch(family) is None or REVISION_RE.fullmatch(revision) is None:
            raise HandoffError(
                "invalid_active_pointer",
                "ACTIVE.yaml contains a non-canonical family or revision",
            )
        if state not in HANDOFF_STATES:
            raise HandoffError(
                "invalid_handoff_state",
                "ACTIVE.yaml contains an unsupported handoff state",
                state=state,
            )
        expected_manifest = f"{family}/{MANIFEST_NAME}"
        manifest = validate_relative_path(entry["manifest"], field=f"{field}.manifest")
        if manifest != expected_manifest:
            raise HandoffError(
                "invalid_active_pointer",
                "ACTIVE.yaml must point to each stable family manifest",
            )
        if handoff_id in seen_ids or family in seen_families:
            raise HandoffError(
                "duplicate_active_handoff",
                "ACTIVE.yaml contains a duplicate handoff identity",
            )
        seen_ids.add(handoff_id)
        seen_families.add(family)
        normalized_handoffs.append(
            {
                "handoff-id": handoff_id,
                "family": family,
                "revision": revision,
                "manifest": manifest,
                "activated-at": expect_timestamp(
                    entry["activated-at"], field=f"{field}.activated-at"
                ),
                "state": state,
            }
        )
    active["investigation-id"] = investigation_id
    active["handoffs"] = normalized_handoffs
    return active, raw


def normalize_active_registry(
    store: Path,
    active: dict[str, Any],
    normalized_remote: str,
) -> dict[str, Any]:
    if active["schema-version"] == ACTIVE_SCHEMA_VERSION:
        return active
    family = str(active["family"])
    handoff_id = str(active["handoff-id"])
    existing = read_existing_handoff(
        store / family,
        family=family,
        handoff_id=handoff_id,
        normalized_remote=normalized_remote,
    )
    if existing is None or existing.revision != active["revision"]:
        raise HandoffError(
            "invalid_active_pointer",
            "Legacy ACTIVE.yaml does not resolve to the exact handoff revision",
            handoff_id=handoff_id,
        )
    return {
        "schema-version": ACTIVE_SCHEMA_VERSION,
        "investigation-id": existing.manifest["source"]["investigation-id"],
        "handoffs": [
            {
                "handoff-id": handoff_id,
                "family": family,
                "revision": str(active["revision"]),
                "manifest": str(active["manifest"]),
                "activated-at": str(active["activated-at"]),
                "state": "active",
            }
        ],
    }


def active_entry(active: dict[str, Any], handoff_id: str) -> dict[str, str] | None:
    return next(
        (
            entry
            for entry in active["handoffs"]
            if entry["handoff-id"] == handoff_id
        ),
        None,
    )


def validate_active_entries(
    store: Path,
    active: dict[str, Any],
    normalized_remote: str,
    branch: str,
) -> list[tuple[dict[str, str], ExistingHandoff, dict[str, object]]]:
    validated = []
    for entry in active["handoffs"]:
        handoff_id = entry["handoff-id"]
        existing = read_existing_handoff(
            store / entry["family"],
            family=entry["family"],
            handoff_id=handoff_id,
            normalized_remote=normalized_remote,
        )
        if existing is None or existing.revision != entry["revision"]:
            raise HandoffError(
                "invalid_active_pointer",
                "ACTIVE.yaml does not resolve to the exact handoff revision",
                handoff_id=handoff_id,
            )
        if existing.manifest["source"]["investigation-id"] != active["investigation-id"]:
            raise HandoffError(
                "worktree_investigation_mismatch",
                "Every handoff in a shared worktree must belong to its investigation",
                handoff_id=handoff_id,
            )
        updates = validate_implementation_updates(existing.family_path / UPDATES_NAME)
        validated.append((entry, existing, updates))
    anchor_work_item = validated[0][1].manifest["work-item"]
    _family, _handoff_id, anchor_token = work_item_identity(
        anchor_work_item, normalized_remote
    )
    branch_leaf = branch.rsplit("/", 1)[-1]
    if "/" not in branch or not branch_leaf.startswith(f"{anchor_token}-"):
        raise HandoffError(
            "worktree_branch_mismatch",
            "The worktree branch no longer matches its first handoff",
            exit_code=3,
            branch=branch,
            work_item_token=anchor_token,
        )
    return validated


@contextmanager
def handoff_store_lock(target: Path):
    store = target / STORE_NAME
    ensure_safe_directory(store, label="handoff store")
    store.mkdir(parents=True, exist_ok=True)
    lock_path = store / LOCK_NAME
    if lock_path.is_symlink() or (lock_path.exists() and not lock_path.is_file()):
        raise HandoffError(
            "unsafe_lock_target",
            "The handoff lock must be a regular file",
            path=str(lock_path),
        )
    flags = os.O_RDWR | os.O_CREAT
    flags |= getattr(os, "O_NOFOLLOW", 0)
    descriptor = os.open(lock_path, flags, 0o600)
    locked = False
    try:
        if os.fstat(descriptor).st_size == 0:
            os.write(descriptor, b"\0")
        os.lseek(descriptor, 0, os.SEEK_SET)
        if os.name == "nt":
            msvcrt.locking(descriptor, msvcrt.LK_LOCK, 1)
        else:
            fcntl.flock(descriptor, fcntl.LOCK_EX)
        locked = True
        yield
    finally:
        try:
            if locked:
                os.lseek(descriptor, 0, os.SEEK_SET)
                if os.name == "nt":
                    msvcrt.locking(descriptor, msvcrt.LK_UNLCK, 1)
                else:
                    fcntl.flock(descriptor, fcntl.LOCK_UN)
        finally:
            os.close(descriptor)


def ensure_no_pending_transaction(store: Path) -> None:
    transaction = store / TRANSACTION_NAME
    if transaction.is_symlink() or (transaction.exists() and not transaction.is_dir()):
        raise HandoffError(
            "unsafe_transaction_state",
            "The handoff transaction state must be a directory",
            path=str(transaction),
        )
    committed = transaction / TRANSACTION_COMMITTED_NAME
    if committed.is_symlink():
        raise HandoffError(
            "unsafe_transaction_state",
            "The handoff transaction commit marker is unsafe",
            path=str(committed),
        )
    if transaction.is_dir() and not committed.is_file():
        raise HandoffError(
            "handoff_recovery_required",
            "A previous handoff apply was interrupted; retry its authorized apply before planning new work",
            exit_code=3,
            path=str(transaction),
        )


def apply_mutation_paths(plan: ApplyPlan, revision: str) -> list[str]:
    prefix = f"{STORE_NAME}/{plan.family}/"
    paths: list[str] = []
    if plan.ignore_action != "none":
        paths.append(".gitignore")
    paths.extend(
        name
        for name, action in plan.instruction_actions.items()
        if action != "none"
    )
    if plan.updates_action == "create":
        paths.append(f"{prefix}{UPDATES_NAME}")
    if plan.existing is None:
        paths.append(f"{prefix}{START_NAME}")
    paths.extend(f"{prefix}{name}" for name in plan.changed_documents)
    if plan.output["action"] in {"create", "update"}:
        paths.extend(
            (
                f"{prefix}history/{revision}.md",
                f"{prefix}{MANIFEST_NAME}",
            )
        )
    if plan.output["action"] in {"create", "update", "activate"}:
        paths.append(f"{STORE_NAME}/{ACTIVE_NAME}")
    return list(dict.fromkeys(paths))


def remove_transaction_directory(path: Path) -> None:
    if path.is_symlink() or (path.exists() and not path.is_dir()):
        raise HandoffError(
            "unsafe_transaction_state",
            "The handoff transaction state is unsafe",
            path=str(path),
        )
    if path.exists():
        shutil.rmtree(path)


def ensure_safe_transaction_target(store: Path, relative: str) -> None:
    ensure_safe_directory(store, label="handoff store")
    current = store
    for part in Path(relative).parts[:-1]:
        current /= part
        if current.is_symlink() or (current.exists() and not current.is_dir()):
            raise HandoffError(
                "unsafe_transaction_target",
                "A handoff transaction target has an unsafe directory ancestor",
                path=str(current),
            )


def start_apply_transaction(plan: ApplyPlan, revision: str) -> bool:
    store = plan.target / STORE_NAME
    transaction = store / TRANSACTION_NAME
    prepare = store / TRANSACTION_PREPARE_NAME
    remove_transaction_directory(prepare)
    if transaction.exists() or transaction.is_symlink():
        raise HandoffError(
            "handoff_recovery_required",
            "A previous handoff apply must be recovered before a new write",
            exit_code=3,
            path=str(transaction),
        )
    relative_paths = apply_mutation_paths(plan, revision)
    if not relative_paths:
        return False
    prepare.mkdir()
    records: list[dict[str, object]] = []
    try:
        for index, relative in enumerate(relative_paths):
            normalized = validate_relative_path(
                relative, field="transaction.path"
            )
            ensure_safe_transaction_target(plan.target, normalized)
            target = plan.target / normalized
            raw = read_optional_file(target, label=normalized)
            backup = None
            if raw is not None:
                backup = f"{index:04d}.bak"
                atomic_write(prepare / backup, raw)
            records.append(
                {
                    "path": normalized,
                    "existed": raw is not None,
                    "backup": backup,
                    "postimage-sha256": None,
                }
            )
        metadata = {
            "schema-version": CONTROL_SCHEMA_VERSION,
            "plan-token": str(plan.output["plan_token"]),
            "bundle-fingerprint": plan.bundle.fingerprint,
            "handoff-id": plan.handoff_id,
            "family": plan.family,
            "family-existed": (store / plan.family).is_dir(),
            "paths": records,
        }
        atomic_write(
            prepare / "transaction.json",
            canonical_json(metadata) + b"\n",
        )
        os.replace(prepare, transaction)
    except Exception:
        remove_transaction_directory(prepare)
        raise
    return True


def transactional_write(target: Path, path: Path, raw: bytes) -> None:
    transaction = target / STORE_NAME / TRANSACTION_NAME
    metadata = load_apply_transaction(transaction)
    try:
        relative = path.relative_to(target).as_posix()
    except ValueError as error:
        raise HandoffError(
            "invalid_transaction_state",
            "A transactional write is outside the target worktree",
            path=str(path),
        ) from error
    matched = False
    for record in metadata["paths"]:
        if record["path"] == relative:
            record["postimage-sha256"] = sha256(raw)
            matched = True
            break
    if not matched:
        raise HandoffError(
            "invalid_transaction_state",
            "A transactional write was not declared by the apply plan",
            path=str(path),
        )
    ensure_safe_transaction_target(target, relative)
    atomic_write(
        transaction / "transaction.json",
        canonical_json(metadata) + b"\n",
    )
    atomic_write(path, raw)


def load_apply_transaction(transaction: Path) -> dict[str, Any]:
    metadata_path = transaction / "transaction.json"
    if metadata_path.is_symlink() or not metadata_path.is_file():
        raise HandoffError(
            "invalid_transaction_state",
            "The handoff transaction metadata is missing or unsafe",
            path=str(metadata_path),
        )
    try:
        metadata = json.loads(metadata_path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise HandoffError(
            "invalid_transaction_state",
            "The handoff transaction metadata is invalid",
            path=str(metadata_path),
        ) from error
    metadata = expect_mapping(metadata, field="transaction")
    expect_exact_keys(
        metadata,
        {
            "schema-version",
            "plan-token",
            "bundle-fingerprint",
            "handoff-id",
            "family",
            "family-existed",
            "paths",
        },
        field="transaction",
    )
    family = expect_string(metadata["family"], field="transaction.family", maximum=200)
    plan_token = expect_string(
        metadata["plan-token"], field="transaction.plan-token", maximum=64
    )
    bundle_fingerprint = expect_string(
        metadata["bundle-fingerprint"],
        field="transaction.bundle-fingerprint",
        maximum=64,
    )
    handoff_id = expect_string(
        metadata["handoff-id"], field="transaction.handoff-id", maximum=64
    )
    if (
        metadata["schema-version"] != CONTROL_SCHEMA_VERSION
        or FAMILY_RE.fullmatch(family) is None
        or re.fullmatch(r"[a-f0-9]{64}", plan_token) is None
        or re.fullmatch(r"[a-f0-9]{64}", bundle_fingerprint) is None
        or re.fullmatch(r"[a-f0-9]{64}", handoff_id) is None
    ):
        raise HandoffError(
            "invalid_transaction_state",
            "The handoff transaction identity is invalid",
        )
    if not isinstance(metadata["family-existed"], bool) or not isinstance(
        metadata["paths"], list
    ):
        raise HandoffError(
            "invalid_transaction_state",
            "The handoff transaction snapshot is invalid",
        )
    return metadata


def cleanup_committed_transaction(target: Path) -> bool:
    transaction = target / STORE_NAME / TRANSACTION_NAME
    if not transaction.exists() and not transaction.is_symlink():
        return False
    if transaction.is_symlink() or not transaction.is_dir():
        raise HandoffError(
            "unsafe_transaction_state",
            "The handoff transaction state is unsafe",
            path=str(transaction),
        )
    committed = transaction / TRANSACTION_COMMITTED_NAME
    if committed.is_symlink():
        raise HandoffError(
            "unsafe_transaction_state",
            "The handoff transaction commit marker is unsafe",
            path=str(committed),
        )
    if not committed.is_file():
        return False
    remove_transaction_directory(transaction)
    return True


def restore_apply_transaction(
    target: Path,
    plan_token: str,
    bundle_fingerprint: str,
    handoff_id: str,
) -> bool:
    store = target / STORE_NAME
    prepare = store / TRANSACTION_PREPARE_NAME
    remove_transaction_directory(prepare)
    transaction = store / TRANSACTION_NAME
    if not transaction.exists() and not transaction.is_symlink():
        return False
    if transaction.is_symlink() or not transaction.is_dir():
        raise HandoffError(
            "unsafe_transaction_state",
            "The handoff transaction state is unsafe",
            path=str(transaction),
        )
    if cleanup_committed_transaction(target):
        return False
    metadata = load_apply_transaction(transaction)
    if (
        not hmac.compare_digest(str(metadata["plan-token"]), plan_token)
        or not hmac.compare_digest(
            str(metadata["bundle-fingerprint"]), bundle_fingerprint
        )
        or not hmac.compare_digest(str(metadata["handoff-id"]), handoff_id)
    ):
        raise HandoffError(
            "plan_stale",
            "Only the exact interrupted apply may recover this worktree",
            exit_code=3,
        )
    family = str(metadata["family"])
    family_existed = bool(metadata["family-existed"])
    raw_records = metadata["paths"]
    records: list[tuple[str, bool, str | None, str | None]] = []
    seen: set[str] = set()
    seen_backups: set[str] = set()
    for index, raw_record in enumerate(raw_records):
        record = expect_mapping(raw_record, field=f"transaction.paths[{index}]")
        expect_exact_keys(
            record,
            {"path", "existed", "backup", "postimage-sha256"},
            field=f"transaction.paths[{index}]",
        )
        relative = validate_relative_path(
            record["path"], field=f"transaction.paths[{index}].path"
        )
        existed = record["existed"]
        backup = record["backup"]
        postimage = record["postimage-sha256"]
        if (
            relative in seen
            or not isinstance(existed, bool)
            or (
                relative not in {".gitignore", *INSTRUCTION_FILES}
                and relative != f"{STORE_NAME}/{ACTIVE_NAME}"
                and not relative.startswith(f"{STORE_NAME}/{family}/")
            )
        ):
            raise HandoffError(
                "invalid_transaction_state",
                "The handoff transaction contains invalid path records",
            )
        if backup is not None and (
            not isinstance(backup, str)
            or re.fullmatch(r"[0-9]{4}\.bak", backup) is None
        ):
            raise HandoffError(
                "invalid_transaction_state",
                "The handoff transaction contains an invalid backup",
            )
        if postimage is not None and (
            not isinstance(postimage, str)
            or re.fullmatch(r"[a-f0-9]{64}", postimage) is None
        ):
            raise HandoffError(
                "invalid_transaction_state",
                "The handoff transaction contains an invalid postimage",
            )
        if existed != (backup is not None):
            raise HandoffError(
                "invalid_transaction_state",
                "The handoff transaction backup does not match its path state",
            )
        if (
            not family_existed
            and relative.startswith(f"{STORE_NAME}/{family}/")
            and existed
        ):
            raise HandoffError(
                "invalid_transaction_state",
                "A new-family transaction contains an unexpected backup",
            )
        if backup is not None and backup in seen_backups:
            raise HandoffError(
                "invalid_transaction_state",
                "The handoff transaction reuses a backup",
            )
        seen.add(relative)
        if backup is not None:
            seen_backups.add(backup)
        records.append((relative, existed, backup, postimage))

    for relative, _existed, _backup, _postimage in records:
        ensure_safe_transaction_target(target, relative)

    family_path = store / family
    planned_family_files = {
        relative
        for relative, _existed, _backup, _postimage in records
        if relative.startswith(f"{STORE_NAME}/{family}/")
    }
    allowed_family_directories: set[str] = set()
    for relative in planned_family_files:
        family_relative = Path(relative).relative_to(STORE_NAME, family)
        for parent in family_relative.parents:
            if parent != Path("."):
                allowed_family_directories.add(parent.as_posix())
    if not family_existed:
        ensure_safe_directory(family_path, label="handoff family")
        if family_path.exists():
            for child in family_path.rglob("*"):
                child_relative = child.relative_to(family_path).as_posix()
                store_relative = f"{STORE_NAME}/{family}/{child_relative}"
                known = (
                    child.is_file() and store_relative in planned_family_files
                ) or (
                    child.is_dir()
                    and child_relative in allowed_family_directories
                )
                if child.is_symlink() or not known:
                    raise HandoffError(
                        "handoff_recovery_conflict",
                        "Interrupted apply recovery found unplanned family content",
                        exit_code=3,
                        path=str(child),
                    )

    preimages: dict[str, bytes | None] = {}

    def ensure_recoverable_content(
        relative: str,
        existed: bool,
        postimage: str | None,
    ) -> None:
        path = target / relative
        if path.is_symlink() or (path.exists() and not path.is_file()):
            raise HandoffError(
                "unsafe_transaction_target",
                "A handoff transaction restore target is unsafe",
                path=str(path),
            )
        current = read_optional_file(path, label=relative)
        original = preimages[relative]
        if current is None:
            matches = not existed
        else:
            current_sha256 = sha256(current)
            matches = (
                (original is not None and current_sha256 == sha256(original))
                or (postimage is not None and current_sha256 == postimage)
            )
        if not matches:
            raise HandoffError(
                "handoff_recovery_conflict",
                "Interrupted apply recovery found changed planned content",
                exit_code=3,
                path=str(path),
            )

    for relative, existed, backup, postimage in records:
        original = None
        if existed:
            if backup is None:
                raise HandoffError(
                    "invalid_transaction_state",
                    "An existing handoff transaction path requires a backup",
                )
            backup_path = transaction / backup
            original = read_optional_file(backup_path, label=backup)
            if original is None:
                raise HandoffError(
                    "invalid_transaction_state",
                    "A required handoff transaction backup is missing",
                    path=str(backup_path),
                )
        preimages[relative] = original
        ensure_recoverable_content(relative, existed, postimage)

    for relative, existed, backup, postimage in sorted(
        records, key=lambda item: item[0] == f"{STORE_NAME}/{ACTIVE_NAME}"
    ):
        ensure_safe_transaction_target(target, relative)
        ensure_recoverable_content(relative, existed, postimage)
        path = target / relative
        if existed:
            raw = preimages[relative]
            if raw is None:
                raise HandoffError(
                    "invalid_transaction_state",
                    "A required handoff transaction preimage is missing",
                    path=relative,
                )
            atomic_write(path, raw)
        elif path.exists() or path.is_symlink():
            if path.is_symlink() or not path.is_file():
                raise HandoffError(
                    "unsafe_transaction_target",
                    "A handoff transaction restore target is unsafe",
                    path=str(path),
                )
            path.unlink()
    if not family_existed and family_path.exists():
        for relative in sorted(
            allowed_family_directories,
            key=lambda value: len(Path(value).parts),
            reverse=True,
        ):
            directory = family_path / relative
            if directory.exists():
                directory.rmdir()
        family_path.rmdir()
    remove_transaction_directory(transaction)
    return True


def commit_apply_transaction(target: Path) -> None:
    transaction = target / STORE_NAME / TRANSACTION_NAME
    atomic_write(transaction / TRANSACTION_COMMITTED_NAME, b"committed\n")
    try:
        remove_transaction_directory(transaction)
    except OSError:
        pass


def path_fingerprint(path: Path) -> object:
    if path.is_symlink():
        return {"type": "symlink"}
    if not path.exists():
        return {"type": "absent"}
    if path.is_file():
        return {"type": "file", "sha256": sha256(path.read_bytes())}
    if path.is_dir():
        entries: list[dict[str, str]] = []
        for index, child in enumerate(sorted(path.rglob("*"))):
            if index >= 1_000:
                raise HandoffError(
                    "handoff_store_too_large",
                    "The handoff family contains too many paths to validate safely",
                    maximum=1_000,
                )
            relative = child.relative_to(path).as_posix()
            if child.is_symlink():
                kind = "symlink"
                digest = ""
            elif child.is_file():
                kind = "file"
                digest = sha256(child.read_bytes())
            elif child.is_dir():
                kind = "directory"
                digest = ""
            else:
                kind = "other"
                digest = ""
            entries.append({"path": relative, "type": kind, "sha256": digest})
        return {"type": "directory", "entries": entries}
    return {"type": "other"}


def git_snapshot_bytes(target: Path, *arguments: str) -> bytes:
    result = subprocess.run(
        ["git", "--no-optional-locks", "-C", str(target), *arguments],
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise HandoffError(
            "repository_snapshot_failed",
            "Git could not produce an exact repository closure snapshot",
            exit_code=3,
            operation=arguments[0] if arguments else "git",
        )
    return result.stdout


def untracked_path_fingerprint(path: Path) -> dict[str, object]:
    try:
        before = path.lstat()
    except OSError as error:
        raise HandoffError(
            "repository_state_unstable",
            "An untracked path changed while the closure snapshot was being read",
            exit_code=3,
        ) from error
    observed = (
        before.st_mode,
        before.st_size,
        before.st_mtime_ns,
        before.st_ctime_ns,
    )
    if stat.S_ISLNK(before.st_mode):
        try:
            value: dict[str, object] = {
                "type": "symlink",
                "sha256": sha256(os.fsencode(os.readlink(path))),
            }
        except OSError as error:
            raise HandoffError(
                "repository_state_unstable",
                "An untracked symlink changed while the closure snapshot was being read",
                exit_code=3,
            ) from error
    elif stat.S_ISREG(before.st_mode):
        digest = hashlib.sha256()
        try:
            with path.open("rb") as stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                    digest.update(chunk)
        except OSError as error:
            raise HandoffError(
                "repository_state_unstable",
                "An untracked file changed while the closure snapshot was being read",
                exit_code=3,
            ) from error
        value = {"type": "file", "sha256": digest.hexdigest()}
    else:
        value = {
            "type": "other",
            "mode": stat.S_IFMT(before.st_mode),
            "size": before.st_size,
        }
    try:
        after = path.lstat()
    except OSError as error:
        raise HandoffError(
            "repository_state_unstable",
            "An untracked path changed while the closure snapshot was being read",
            exit_code=3,
        ) from error
    if observed != (
        after.st_mode,
        after.st_size,
        after.st_mtime_ns,
        after.st_ctime_ns,
    ):
        raise HandoffError(
            "repository_state_unstable",
            "An untracked path changed while the closure snapshot was being read",
            exit_code=3,
        )
    return value


def closure_fingerprint(target: Path, family_path: Path, active_path: Path) -> str:
    head = git_snapshot_bytes(target, "rev-parse", "--verify", "HEAD").strip()
    status = git_snapshot_bytes(
        target,
        "status",
        "--porcelain=v1",
        "-z",
        "--branch",
        "--untracked-files=all",
        "--ignore-submodules=none",
    )
    diff = git_snapshot_bytes(
        target,
        "diff",
        "--no-ext-diff",
        "--binary",
        "--submodule=diff",
        "HEAD",
        "--",
    )
    raw_paths = git_snapshot_bytes(
        target,
        "ls-files",
        "--others",
        "--exclude-standard",
        "-z",
    ).split(b"\0")
    if raw_paths and raw_paths[-1] == b"":
        raw_paths.pop()
    if len(raw_paths) > MAX_CLOSURE_UNTRACKED_PATHS:
        raise HandoffError(
            "repository_snapshot_too_large",
            "The repository has too many untracked paths for an exact closure snapshot",
            exit_code=3,
            maximum=MAX_CLOSURE_UNTRACKED_PATHS,
        )
    untracked: list[dict[str, object]] = []
    for raw_path in raw_paths:
        relative = Path(os.fsdecode(raw_path))
        if relative.is_absolute() or ".." in relative.parts:
            raise HandoffError(
                "unsafe_repository_path",
                "Git returned an unsafe untracked repository path",
                exit_code=3,
            )
        untracked.append(
            {
                "path": raw_path.hex(),
                **untracked_path_fingerprint(target / relative),
            }
        )
    return sha256(
        canonical_json(
            {
                "schema_version": CONTROL_SCHEMA_VERSION,
                "handoff_family": path_fingerprint(family_path),
                "active": path_fingerprint(active_path),
                "repository": {
                    "head": head.hex(),
                    "status_sha256": sha256(status),
                    "diff_sha256": sha256(diff),
                    "untracked": untracked,
                },
            }
        )
    )


def shallow_path_state(path: Path) -> dict[str, str]:
    if path.is_symlink():
        kind = "symlink"
    elif not path.exists():
        kind = "absent"
    elif path.is_dir():
        kind = "directory"
    elif path.is_file():
        kind = "file"
    else:
        kind = "other"
    return {"type": kind}


def is_tracked(target: Path, relative: str) -> bool:
    result = subprocess.run(
        ["git", "-C", str(target), "ls-files", "--error-unmatch", "--", relative],
        text=True,
        capture_output=True,
        check=False,
    )
    return result.returncode == 0


def effect(
    target: Path,
    relative: str,
    action: str,
    *,
    tracked: bool | None = None,
) -> dict[str, object]:
    if tracked is None:
        tracked = is_tracked(target, relative)
    return {"path": relative, "action": action, "tracked": tracked}


def next_revision(previous: str | None) -> str:
    if previous is None:
        return "v0001"
    match = REVISION_RE.fullmatch(previous)
    if match is None or int(match.group(1)) >= 9_999:
        raise HandoffError(
            "revision_exhausted",
            "The handoff revision cannot be incremented",
        )
    return f"v{int(match.group(1)) + 1:04d}"


def build_worktree_plan(
    vault_root: Path,
    bundle_path: str,
    base_branch: str,
    base_source: str,
    description: str,
    branch_prefix: str = "issue",
) -> WorktreePlan:
    bundle = load_bundle(bundle_path)
    validate_tracker_binding(vault_root, bundle)
    source, normalized_remote = resolve_repository(
        vault_root,
        bundle.repository_remote,
    )
    worktree_root = resolve_worktree_root(vault_root)
    base_branch = expect_string(
        base_branch,
        field="base_branch",
        maximum=200,
    )
    branch_check = subprocess.run(
        ["git", "check-ref-format", "--branch", base_branch],
        text=True,
        capture_output=True,
        check=False,
    )
    if branch_check.returncode != 0:
        raise HandoffError(
            "invalid_base_branch",
            "The selected base branch is not a valid Git branch name",
            field="base_branch",
        )
    if base_source not in {"local", "remote"}:
        raise HandoffError(
            "invalid_base_source",
            "base_source must be local or remote",
            field="base_source",
        )
    description_slug = slug(
        expect_string(
            description,
            field="description",
            maximum=120,
        )
    )
    branch_prefix = expect_string(
        branch_prefix,
        field="branch_prefix",
        maximum=100,
    ).rstrip("/")
    if not branch_prefix or branch_prefix.startswith("/"):
        raise HandoffError(
            "invalid_branch_prefix",
            "The branch prefix must be a non-empty relative Git ref prefix",
            field="branch_prefix",
        )
    _family, _handoff_id, work_item_token = handoff_identity(
        bundle, normalized_remote
    )
    branch = f"{branch_prefix}/{work_item_token}-{description_slug}"
    branch_check = subprocess.run(
        ["git", "check-ref-format", "--branch", branch],
        text=True,
        capture_output=True,
        check=False,
    )
    if branch_check.returncode != 0:
        raise HandoffError(
            "invalid_worktree_branch",
            "The work-item identity and description did not produce a valid Git branch",
        )

    repository_directory = worktree_root / observed_repository_basename(
        source,
        normalized_remote,
    )
    path = repository_directory / f"{work_item_token}-{description_slug}"
    if repository_directory.is_symlink() or (
        repository_directory.exists() and not repository_directory.is_dir()
    ):
        raise HandoffError(
            "unsafe_worktree_parent",
            "The repository worktree directory is not a safe directory",
            path=str(repository_directory),
        )
    if repository_directory.exists():
        parent_git_context = subprocess.run(
            [
                "git",
                "--no-optional-locks",
                "-C",
                str(repository_directory),
                "rev-parse",
                "--is-inside-work-tree",
            ],
            text=True,
            capture_output=True,
            check=False,
        )
        if parent_git_context.returncode == 0:
            raise HandoffError(
                "unsafe_worktree_parent",
                "The repository worktree directory cannot be inside a Git worktree",
                path=str(repository_directory),
            )
    if path.exists() or path.is_symlink():
        raise HandoffError(
            "worktree_path_exists",
            "The proposed worktree path already exists",
            exit_code=3,
            path=str(path),
        )
    if rev_parse_optional(source, f"refs/heads/{branch}") is not None:
        raise HandoffError(
            "worktree_branch_exists",
            "The proposed worktree branch already exists",
            exit_code=3,
            branch=branch,
        )
    listed = subprocess.run(
        [
            "git",
            "--no-optional-locks",
            "-C",
            str(source),
            "worktree",
            "list",
            "--porcelain",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if listed.returncode != 0:
        raise HandoffError(
            "worktree_metadata_unavailable",
            "Git worktree registration could not be inspected",
            exit_code=3,
        )
    registered_paths = {
        Path(line.removeprefix("worktree ")).resolve(strict=False)
        for line in listed.stdout.splitlines()
        if line.startswith("worktree ")
    }
    if path.resolve(strict=False) in registered_paths:
        raise HandoffError(
            "worktree_path_registered",
            "The proposed path is already registered as a Git worktree",
            exit_code=3,
            path=str(path),
        )

    remote_name = matching_remote_name(source, normalized_remote)
    local_commit = rev_parse_optional(source, f"refs/heads/{base_branch}")
    remote_commit = remote_branch_commit(source, remote_name, base_branch)
    if local_commit is None and base_source == "local":
        raise HandoffError(
            "local_base_branch_missing",
            "The selected local base branch does not exist",
            exit_code=3,
            base_branch=base_branch,
        )
    selected_commit = remote_commit if base_source == "remote" else str(local_commit)
    freshness = (
        "local_missing"
        if local_commit is None
        else "current"
        if local_commit == remote_commit
        else "local_differs_from_remote"
    )
    remote_tracking_ref = f"refs/remotes/{remote_name}/{base_branch}"
    remote_tracking_commit = rev_parse_optional(source, remote_tracking_ref)
    remote_object_present = rev_parse_optional(source, remote_commit) == remote_commit
    fetch_required = base_source == "remote" and not remote_object_present
    tracking_update_required = (
        base_source == "remote" and remote_tracking_commit != remote_commit
    )

    effects: list[dict[str, object]] = []
    if fetch_required:
        effects.append(
            {
                "path": f"git-object:{remote_commit}",
                "action": "fetch",
                "tracked": False,
            }
        )
    if tracking_update_required:
        effects.append(
            {
                "path": remote_tracking_ref,
                "action": "create" if remote_tracking_commit is None else "update",
                "tracked": False,
            }
        )
    if not repository_directory.exists():
        effects.append(
            {
                "path": str(repository_directory),
                "action": "create-directory",
                "tracked": False,
            }
        )
    effects.extend(
        (
            {
                "path": f"refs/heads/{branch}",
                "action": "create",
                "tracked": False,
            },
            {
                "path": str(path),
                "action": "create-worktree",
                "tracked": False,
            },
        )
    )
    token_payload = {
        "schema_version": CONTROL_SCHEMA_VERSION,
        "operation": "create-worktree",
        "bundle": bundle.fingerprint,
        "source": str(source),
        "remote": normalized_remote,
        "remote_name": remote_name,
        "worktree_root": str(worktree_root),
        "base_branch": base_branch,
        "base_source": base_source,
        "local_commit": local_commit,
        "remote_commit": remote_commit,
        "remote_tracking_commit": remote_tracking_commit,
        "selected_commit": selected_commit,
        "branch": branch,
        "path": str(path),
        "parent_state": shallow_path_state(repository_directory),
        "path_state": path_fingerprint(path),
        "branch_state": rev_parse_optional(source, f"refs/heads/{branch}"),
        "effects": effects,
    }
    plan_token = sha256(canonical_json(token_payload))
    output = {
        "status": "planned",
        "operation": "create-worktree",
        "source": {
            "path": str(source),
            "remote": normalized_remote,
            "remote_name": remote_name,
        },
        "base": {
            "branch": base_branch,
            "local_commit": local_commit,
            "remote_commit": remote_commit,
            "freshness": freshness,
            "selected_source": base_source,
            "selected_commit": selected_commit,
            "fetch_required": fetch_required,
            "tracking_update_required": tracking_update_required,
        },
        "worktree": {
            "root": str(worktree_root),
            "repository_directory": str(repository_directory),
            "path": str(path),
            "branch": branch,
        },
        "effects": effects,
        "plan_token": plan_token,
    }
    return WorktreePlan(
        output=output,
        bundle=bundle,
        source=source,
        normalized_remote=normalized_remote,
        remote_name=remote_name,
        base_branch=base_branch,
        base_source=base_source,
        selected_commit=selected_commit,
        branch=branch,
        path=path,
        fetch_required=fetch_required,
        remote_tracking_commit=remote_tracking_commit,
        tracking_update_required=tracking_update_required,
    )


def create_worktree(
    vault_root: Path,
    bundle_path: str,
    base_branch: str,
    base_source: str,
    description: str,
    branch_prefix: str,
    plan_token: str,
) -> dict[str, Any]:
    plan = build_worktree_plan(
        vault_root,
        bundle_path,
        base_branch,
        base_source,
        description,
        branch_prefix,
    )
    if not hmac.compare_digest(str(plan.output["plan_token"]), plan_token):
        raise HandoffError(
            "plan_stale",
            "The repository, remote base, or destination changed after planning",
            exit_code=3,
        )
    if plan.fetch_required:
        environment = os.environ.copy()
        environment["GIT_TERMINAL_PROMPT"] = "0"
        fetched = subprocess.run(
            [
                "git",
                "-C",
                str(plan.source),
                "fetch",
                "--no-tags",
                "--no-write-fetch-head",
                plan.remote_name,
                plan.selected_commit,
            ],
            text=True,
            capture_output=True,
            env=environment,
            check=False,
        )
        if fetched.returncode != 0:
            raise HandoffError(
                "base_branch_fetch_failed",
                "The selected remote base branch could not be fetched",
                exit_code=4,
                base_branch=plan.base_branch,
                remote=plan.remote_name,
            )
        if rev_parse_optional(plan.source, plan.selected_commit) != plan.selected_commit:
            raise HandoffError(
                "base_commit_fetch_failed",
                "The fetched Git object does not match the approved base commit",
                exit_code=4,
            )
    if plan.tracking_update_required:
        remote_tracking_ref = (
            f"refs/remotes/{plan.remote_name}/{plan.base_branch}"
        )
        old_commit = plan.remote_tracking_commit or (
            "0" * len(plan.selected_commit)
        )
        updated_ref = subprocess.run(
            [
                "git",
                "-C",
                str(plan.source),
                "update-ref",
                remote_tracking_ref,
                plan.selected_commit,
                old_commit,
            ],
            text=True,
            capture_output=True,
            check=False,
        )
        if updated_ref.returncode != 0:
            raise HandoffError(
                "base_tracking_ref_changed",
                "The remote-tracking base ref changed after planning",
                exit_code=3,
                base_branch=plan.base_branch,
                remote=plan.remote_name,
            )

    plan.path.parent.mkdir(parents=True, exist_ok=True)
    added = subprocess.run(
        [
            "git",
            "-C",
            str(plan.source),
            "worktree",
            "add",
            "-b",
            plan.branch,
            str(plan.path),
            plan.selected_commit,
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if added.returncode != 0:
        raise HandoffError(
            "worktree_creation_failed",
            "Git could not create the approved worktree and branch",
            exit_code=4,
            path=str(plan.path),
            branch=plan.branch,
        )
    observed_head = rev_parse_optional(plan.path, "HEAD")
    observed_branch = current_branch(plan.path)
    if observed_head != plan.selected_commit or observed_branch != plan.branch:
        raise HandoffError(
            "worktree_postcondition_failed",
            "The created worktree does not match the approved branch and base commit",
            exit_code=4,
            path=str(plan.path),
        )
    return {
        "status": "created",
        "source": plan.output["source"],
        "base": plan.output["base"],
        "worktree": plan.output["worktree"],
    }


def build_apply_plan_from_state(
    bundle: Bundle,
    *,
    state_root: Path,
    target: Path,
    normalized_remote: str,
    branch: str,
    explicit_worktree: bool,
    tracked_paths: set[str] | None = None,
    head: str | None = None,
    verify_effective_ignore: bool = True,
) -> ApplyPlan:
    store = state_root / STORE_NAME
    ensure_safe_directory(store, label="handoff store")
    ensure_no_pending_transaction(store)
    active, _ = read_active(store)
    legacy_active = (
        active is not None
        and active["schema-version"] == LEGACY_ACTIVE_SCHEMA_VERSION
    )
    if active is not None:
        active = normalize_active_registry(store, active, normalized_remote)
    branch_leaf = branch.rsplit("/", 1)[-1]
    if explicit_worktree and active is None and (
        "/" not in branch
        or not branch_leaf.startswith(
            f"{handoff_identity(bundle, normalized_remote)[2]}-"
        )
    ):
        raise HandoffError(
            "worktree_branch_mismatch",
            "The selected worktree branch does not match the handoff work item",
            exit_code=3,
            branch=branch,
            work_item_reference=bundle.work_item["reference"],
        )
    investigation_id = bundle.source["investigation-id"]
    if active is not None and active["investigation-id"] != investigation_id:
        raise HandoffError(
            "worktree_investigation_mismatch",
            "A shared worktree may contain handoffs from only one investigation",
            active_investigation=active["investigation-id"],
            requested_investigation=investigation_id,
        )
    if active is not None:
        validate_active_entries(store, active, normalized_remote, branch)
    family, handoff_id, _work_item_token = handoff_identity(
        bundle, normalized_remote
    )
    family_path = store / family
    existing = read_existing_handoff(
        family_path,
        family=family,
        handoff_id=handoff_id,
        normalized_remote=normalized_remote,
    )
    if existing is not None:
        stable_work_item = existing.manifest["work-item"]
        stable_keys = ("tracker-id", "provider", "tracker-url", "reference")
        if all(
            key in stable_work_item and key in bundle.work_item
            for key in stable_keys
        ) and any(
            stable_work_item[key] != bundle.work_item[key] for key in stable_keys
        ):
            raise HandoffError(
                "tracker_binding_mismatch",
                "An existing handoff cannot change its tracker binding or work-item reference",
                family=family,
            )
    if (
        existing is not None
        and existing.manifest["source"]["investigation-id"] != investigation_id
    ):
        raise HandoffError(
            "worktree_investigation_mismatch",
            "The existing handoff family belongs to another investigation",
            existing_investigation=existing.manifest["source"]["investigation-id"],
            requested_investigation=investigation_id,
        )
    desired_instructions, instruction_actions = prepare_instructions(state_root)
    instructions_changed = any(
        action != "none" for action in instruction_actions.values()
    )
    desired_ignore, ignore_action = prepare_gitignore(state_root)
    updates_action, updates_summary = prepare_implementation_updates(family_path)
    if existing is not None and updates_action == "create":
        raise HandoffError(
            "missing_implementation_updates",
            "An existing handoff must contain implementation-updates.md",
            path=str(family_path / UPDATES_NAME),
        )
    if ignore_action == "none" and verify_effective_ignore:
        verify_ignored(state_root)

    if existing is None:
        changed_documents = list(DOCUMENTS)
        unchanged_documents: list[str] = []
        revision = next_revision(None)
        previous_revision = None
        action = "create"
    else:
        manifest_files = existing.manifest["files"]
        changed_documents = sorted(
            name
            for name in DOCUMENTS
            if manifest_files[name]["semantic-sha256"]
            != bundle.documents[name].semantic_sha256
        )
        unchanged_documents = sorted(set(DOCUMENTS) - set(changed_documents))
        previous_revision = existing.revision
        if changed_documents:
            revision = next_revision(existing.revision)
            action = "update"
        else:
            revision = existing.revision
            entry = None if active is None else active_entry(active, handoff_id)
            active_matches = (
                entry is not None
                and entry["family"] == family
                and entry["revision"] == revision
                and entry["state"] == "active"
            )
            if legacy_active or not active_matches:
                action = "activate"
            elif (
                instructions_changed
                or ignore_action != "none"
                or updates_action != "none"
            ):
                action = "bootstrap"
            else:
                action = "noop"

    def planned_effect(
        relative: str,
        planned_action: str,
        *,
        tracked: bool | None = None,
    ) -> dict[str, object]:
        if tracked is None and tracked_paths is not None:
            tracked = relative in tracked_paths
        return effect(state_root, relative, planned_action, tracked=tracked)

    effects: list[dict[str, object]] = []
    if ignore_action != "none":
        effects.append(planned_effect(".gitignore", ignore_action))
    for name, instruction_action in instruction_actions.items():
        if instruction_action != "none":
            effects.append(planned_effect(name, instruction_action))
    prefix = f"{STORE_NAME}/{family}"
    if existing is None:
        effects.append(planned_effect(f"{prefix}/{START_NAME}", "create", tracked=False))
    if updates_action != "none":
        effects.append(
            planned_effect(
                f"{prefix}/{UPDATES_NAME}",
                updates_action,
                tracked=False,
            )
        )
    for name in changed_documents:
        effects.append(
            planned_effect(
                f"{prefix}/{name}",
                "create" if existing is None else "update",
                tracked=False,
            )
        )
    if action in {"create", "update"}:
        effects.extend(
            (
                planned_effect(
                    f"{prefix}/history/{revision}.md",
                    "create",
                    tracked=False,
                ),
                planned_effect(
                    f"{prefix}/{MANIFEST_NAME}",
                    "create" if existing is None else "update",
                    tracked=False,
                ),
            )
        )
    if action in {"create", "update", "activate"}:
        effects.append(
            planned_effect(
                f"{STORE_NAME}/{ACTIVE_NAME}",
                "create" if active is None else "update",
                tracked=False,
            )
        )

    state = {
        "instructions": {
            name: path_fingerprint(state_root / name) for name in INSTRUCTION_FILES
        },
        "agents_override": path_fingerprint(state_root / "AGENTS.override.md"),
        "gitignore": path_fingerprint(state_root / ".gitignore"),
        "active": path_fingerprint(store / ACTIVE_NAME),
        "family": path_fingerprint(family_path),
    }
    token_payload = {
        "schema_version": CONTROL_SCHEMA_VERSION,
        "operation": "apply",
        "bundle": bundle.fingerprint,
        "target": str(target),
        "remote": normalized_remote,
        "family": family,
        "handoff_id": handoff_id,
        "revision": revision,
        "action": action,
        "state": state,
        "branch": branch,
        "head": head if head is not None else rev_parse_optional(state_root, "HEAD"),
        "effects": effects,
        "assets": {
            "agents": sha256(read_asset(AGENTS_BLOCK_ASSET)),
            "start": sha256(read_asset(START_ASSET)),
            "implementation_updates": sha256(read_asset(UPDATES_ASSET)),
        },
    }
    plan_token = sha256(canonical_json(token_payload))
    output = {
        "status": "planned",
        "operation": "apply",
        "action": action,
        "target": {
            "path": str(target),
            "remote": normalized_remote,
            "branch": branch,
        },
        "handoff": {
            "id": handoff_id,
            "family": family,
            "revision": revision,
            "previous_revision": previous_revision,
            "changed_documents": changed_documents,
            "unchanged_documents": unchanged_documents,
            "implementation_updates": {
                **updates_summary,
                "action": updates_action,
            },
        },
        "effects": effects,
        "plan_token": plan_token,
    }
    return ApplyPlan(
        output=output,
        target=target,
        normalized_remote=normalized_remote,
        bundle=bundle,
        family=family,
        handoff_id=handoff_id,
        active=active,
        existing=existing,
        changed_documents=changed_documents,
        unchanged_documents=unchanged_documents,
        desired_instructions=desired_instructions,
        desired_ignore=desired_ignore,
        instruction_actions=instruction_actions,
        ignore_action=ignore_action,
        updates_action=updates_action,
    )


def build_apply_plan(
    vault_root: Path,
    bundle_path: str,
    worktree_path: str,
) -> ApplyPlan:
    bundle = load_bundle(bundle_path)
    validate_tracker_binding(vault_root, bundle)
    target, normalized_remote, branch = resolve_handoff_target(
        vault_root,
        bundle.repository_remote,
        worktree_path,
    )
    return build_apply_plan_from_state(
        bundle,
        state_root=target,
        target=target,
        normalized_remote=normalized_remote,
        branch=branch,
        explicit_worktree=True,
    )


def tracked_blob_at_commit(
    source: Path,
    commit: str,
    relative: str,
) -> tuple[bytes, str] | None:
    listed = subprocess.run(
        ["git", "-C", str(source), "ls-tree", "-z", commit, "--", relative],
        capture_output=True,
        check=False,
    )
    if listed.returncode != 0:
        raise HandoffError(
            "base_tree_unavailable",
            "The selected base tree could not be inspected for handoff planning",
            exit_code=3,
            path=relative,
        )
    entries = [entry for entry in listed.stdout.split(b"\0") if entry]
    if not entries:
        return None
    if len(entries) != 1 or b"\t" not in entries[0]:
        raise HandoffError(
            "base_tree_ambiguous",
            "The selected base tree contains an ambiguous managed path",
            path=relative,
        )
    metadata, observed_name = entries[0].split(b"\t", 1)
    parts = metadata.split()
    if len(parts) != 3:
        raise HandoffError(
            "base_tree_invalid",
            "The selected base tree returned invalid Git metadata",
            path=relative,
        )
    mode, object_type, object_id = parts
    if observed_name.decode("utf-8", errors="strict") != relative:
        raise HandoffError(
            "base_tree_ambiguous",
            "The selected base tree resolved a different managed path",
            path=relative,
        )
    if object_type != b"blob" or mode not in {b"100644", b"100755", b"120000"}:
        raise HandoffError(
            "unsafe_managed_path",
            "A managed root path in the selected base has an unsupported Git mode",
            path=relative,
        )
    loaded = subprocess.run(
        ["git", "-C", str(source), "cat-file", "blob", object_id.decode("ascii")],
        capture_output=True,
        check=False,
    )
    if loaded.returncode != 0:
        raise HandoffError(
            "base_blob_unavailable",
            "A managed root file in the selected base could not be read",
            exit_code=3,
            path=relative,
        )
    return loaded.stdout, mode.decode("ascii")


def checkout_uses_symlinks(source: Path) -> bool:
    configured = subprocess.run(
        ["git", "-C", str(source), "config", "--bool", "--get", "core.symlinks"],
        text=True,
        capture_output=True,
        check=False,
    )
    if configured.returncode == 0:
        return configured.stdout.strip() == "true"
    if configured.returncode == 1:
        return os.name != "nt"
    raise HandoffError(
        "git_config_unavailable",
        "Git could not determine whether the target checkout preserves symlinks",
        exit_code=3,
    )


def reject_tracked_handoff_store(source: Path, commit: str) -> None:
    listed = subprocess.run(
        ["git", "-C", str(source), "ls-tree", commit, "--", STORE_NAME],
        text=True,
        capture_output=True,
        check=False,
    )
    if listed.returncode != 0:
        raise HandoffError(
            "base_tree_unavailable",
            "The selected base tree could not be inspected for handoff planning",
            exit_code=3,
            path=STORE_NAME,
        )
    if listed.stdout.strip():
        raise HandoffError(
            "tracked_handoff_store",
            "A fresh handoff worktree cannot start from a commit that tracks "
            "the local handoff store",
            path=STORE_NAME,
        )


def projection_repository(worktree: WorktreePlan, temporary_root: Path) -> Path:
    if not worktree.fetch_required:
        return worktree.source
    repository = temporary_root / "projection.git"
    initialized = subprocess.run(
        ["git", "init", "--bare", "-q", str(repository)],
        text=True,
        capture_output=True,
        check=False,
    )
    if initialized.returncode != 0:
        raise HandoffError(
            "projection_repository_failed",
            "A temporary repository for complete handoff planning could not be created",
            exit_code=4,
        )
    remote_url = subprocess.run(
        [
            "git",
            "-C",
            str(worktree.source),
            "remote",
            "get-url",
            worktree.remote_name,
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if remote_url.returncode != 0 or not remote_url.stdout.strip():
        raise HandoffError(
            "projection_remote_unavailable",
            "The selected remote could not be resolved for complete handoff planning",
            exit_code=3,
        )
    environment = os.environ.copy()
    environment["GIT_TERMINAL_PROMPT"] = "0"
    fetched = subprocess.run(
        [
            "git",
            "-C",
            str(repository),
            "fetch",
            "--no-tags",
            "--no-write-fetch-head",
            "--depth=1",
            remote_url.stdout.strip(),
            worktree.selected_commit,
        ],
        text=True,
        capture_output=True,
        env=environment,
        check=False,
    )
    if (
        fetched.returncode != 0
        or rev_parse_optional(repository, worktree.selected_commit)
        != worktree.selected_commit
    ):
        raise HandoffError(
            "projection_fetch_failed",
            "The exact remote base commit could not be inspected in temporary storage",
            exit_code=4,
            commit=worktree.selected_commit,
        )
    return repository


def build_complete_handoff_plan(
    vault_root: Path,
    bundle_path: str,
    base_branch: str,
    base_source: str,
    description: str,
    branch_prefix: str = "issue",
) -> CompleteHandoffPlan:
    worktree = build_worktree_plan(
        vault_root,
        bundle_path,
        base_branch,
        base_source,
        description,
        branch_prefix,
    )
    tracked_paths: set[str] = set()
    with tempfile.TemporaryDirectory(prefix="knowledge-os-handoff-plan-") as raw_projection:
        temporary_root = Path(raw_projection)
        projection_source = projection_repository(worktree, temporary_root)
        reject_tracked_handoff_store(projection_source, worktree.selected_commit)
        projection = temporary_root / "worktree"
        projection.mkdir()
        preserve_symlinks = checkout_uses_symlinks(worktree.source)
        for relative in (
            "AGENTS.md",
            "CLAUDE.md",
            "AGENTS.override.md",
            ".gitignore",
        ):
            entry = tracked_blob_at_commit(
                projection_source,
                worktree.selected_commit,
                relative,
            )
            if entry is None:
                continue
            raw, mode = entry
            tracked_paths.add(relative)
            projected_path = projection / relative
            if mode == "120000" and preserve_symlinks:
                try:
                    link_target = raw.decode("utf-8")
                    os.symlink(link_target, projected_path)
                except (OSError, UnicodeDecodeError) as error:
                    raise HandoffError(
                        "projection_symlink_failed",
                        "The selected base symlink could not be projected safely",
                        exit_code=3,
                        path=relative,
                    ) from error
            else:
                projected_path.write_bytes(raw)
        materialization = build_apply_plan_from_state(
            worktree.bundle,
            state_root=projection,
            target=worktree.path,
            normalized_remote=worktree.normalized_remote,
            branch=worktree.branch,
            explicit_worktree=True,
            tracked_paths=tracked_paths,
            head=worktree.selected_commit,
            verify_effective_ignore=False,
        )

    token_payload = {
        "schema_version": CONTROL_SCHEMA_VERSION,
        "operation": "prepare-handoff",
        "worktree_plan_token": worktree.output["plan_token"],
        "materialization_plan_token": materialization.output["plan_token"],
    }
    plan_token = sha256(canonical_json(token_payload))
    output = {
        "status": "planned",
        "operation": "prepare-handoff",
        "authorization": "single-complete-plan",
        "source": worktree.output["source"],
        "base": worktree.output["base"],
        "worktree": worktree.output["worktree"],
        "handoff": {
            "action": materialization.output["action"],
            **materialization.output["handoff"],
        },
        "effects": {
            "git": worktree.output["effects"],
            "files": materialization.output["effects"],
        },
        "excluded_effects": ["source-code", "commit", "push", "pull-request"],
        "failure_boundary": (
            "Execution is sequential; a materialization failure preserves the "
            "created worktree, while a validation failure preserves the applied "
            "handoff as the exact resume boundary."
        ),
        "plan_token": plan_token,
    }
    return CompleteHandoffPlan(
        output=output,
        worktree=worktree,
        materialization=materialization,
    )


def prepare_complete_handoff(
    vault_root: Path,
    bundle_path: str,
    base_branch: str,
    base_source: str,
    description: str,
    branch_prefix: str,
    plan_token: str,
) -> dict[str, Any]:
    plan = build_complete_handoff_plan(
        vault_root,
        bundle_path,
        base_branch,
        base_source,
        description,
        branch_prefix,
    )
    if not hmac.compare_digest(str(plan.output["plan_token"]), plan_token):
        raise HandoffError(
            "plan_stale",
            "The complete handoff plan changed after authorization",
            exit_code=3,
        )
    created = create_worktree(
        vault_root,
        bundle_path,
        base_branch,
        base_source,
        description,
        branch_prefix,
        str(plan.worktree.output["plan_token"]),
    )
    worktree_path = str(plan.worktree.path)
    observed = build_apply_plan(vault_root, bundle_path, worktree_path)
    expected_token = str(plan.materialization.output["plan_token"])
    observed_token = str(observed.output["plan_token"])
    if not hmac.compare_digest(expected_token, observed_token):
        raise HandoffError(
            "materialization_plan_changed",
            "The created worktree does not match the authorized materialization plan",
            exit_code=3,
            worktree=created["worktree"],
            resume_boundary="worktree-created",
        )
    try:
        applied = apply_plan(
            vault_root,
            bundle_path,
            observed_token,
            worktree_path,
        )
    except HandoffError as error:
        raise HandoffError(
            "handoff_materialization_failed",
            "The worktree was created but handoff materialization did not complete",
            exit_code=error.exit_code,
            cause=error.code,
            worktree=created["worktree"],
            resume_boundary="worktree-created",
        ) from error
    try:
        validation = validate_repository(
            vault_root,
            plan.worktree.bundle.repository_remote,
            worktree_path=worktree_path,
        )
    except HandoffError as error:
        raise HandoffError(
            "handoff_validation_failed",
            "The handoff was materialized but did not pass validation",
            exit_code=error.exit_code,
            cause=error.code,
            worktree=created["worktree"],
            resume_boundary="handoff-applied",
        ) from error
    return {
        "status": "prepared",
        "operation": "prepare-handoff",
        "source": created["source"],
        "base": created["base"],
        "worktree": created["worktree"],
        "handoff": applied["handoff"],
        "validation": validation,
    }


def atomic_write(path: Path, raw: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    mode = 0o644
    if path.exists():
        if path.is_symlink() or not path.is_file():
            raise HandoffError(
                "unsafe_write_target",
                "A managed write target must be a regular file",
                path=str(path),
            )
        mode = path.stat().st_mode & 0o777
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.",
        dir=str(path.parent),
    )
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary, mode)
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def history_event_bytes(
    *,
    plan: ApplyPlan,
    revision: str,
    previous: str | None,
    previous_event_sha256: str | None,
    created_at: str,
) -> bytes:
    changed: list[dict[str, object]] = []
    for name in plan.changed_documents:
        previous_hash: str | None = None
        if plan.existing is not None:
            previous_hash = plan.existing.manifest["files"][name][
                "semantic-sha256"
            ]
        changed.append(
            {
                "path": name,
                "previous-semantic-sha256": previous_hash,
                "semantic-sha256": plan.bundle.documents[name].semantic_sha256,
                "reason": plan.bundle.change_reasons.get(
                    name,
                    plan.bundle.change_summary,
                ),
            }
        )
    frontmatter = {
        "schema-version": HISTORY_SCHEMA_VERSION,
        "handoff-id": plan.handoff_id,
        "revision": revision,
        "previous": previous,
        "previous-event-sha256": previous_event_sha256,
        "created-at": created_at,
        "reason": plan.bundle.change_summary,
        "changed": changed,
        "unchanged": plan.unchanged_documents,
    }
    parts = [
        "---\n",
        yaml_bytes(frontmatter).decode("utf-8"),
        "---\n",
        f"# Revision {revision}\n\n",
        f"{plan.bundle.change_summary}\n\n",
        "## Changed files\n\n",
    ]
    for item in changed:
        parts.append(f"- `{item['path']}`: {item['reason']}\n")
    if previous is None:
        parts.extend(
            (
                "\n## Baseline\n\n",
                "This revision establishes the materialized baseline. "
                "Later events contain only changed-file diffs.\n",
            )
        )
    else:
        parts.append("\n## Patch\n\n")
        for name in plan.changed_documents:
            old_raw = (plan.existing.family_path / name).read_bytes()
            old_text = semantic_text(old_raw.decode("utf-8-sig"))
            new_text = plan.bundle.documents[name].semantic_text
            patch = difflib.unified_diff(
                old_text.splitlines(),
                new_text.splitlines(),
                fromfile=f"a/{name}",
                tofile=f"b/{name}",
                lineterm="",
            )
            parts.append(f"### `{name}`\n\n")
            for line in patch:
                parts.append(f"    {line}\n")
            parts.append("\n")
    return "".join(parts).encode("utf-8")


def manifest_bytes(
    *,
    plan: ApplyPlan,
    revision: str,
    previous: str | None,
    history_path: str,
    history_raw: bytes,
    created_at: str,
    updated_at: str,
) -> bytes:
    family_path = plan.target / STORE_NAME / plan.family
    file_records: dict[str, dict[str, str]] = {}
    start_raw = (
        read_asset(START_ASSET)
        if plan.existing is None
        else (family_path / START_NAME).read_bytes()
    )
    file_records[START_NAME] = {"sha256": sha256(start_raw)}
    for name in DOCUMENTS:
        raw = (
            plan.bundle.documents[name].raw
            if name in plan.changed_documents
            else (family_path / name).read_bytes()
        )
        normalized = semantic_text(raw.decode("utf-8-sig"))
        file_records[name] = {
            "sha256": sha256(raw),
            "semantic-sha256": sha256(normalized.encode("utf-8")),
        }
    manifest = {
        "schema-version": HANDOFF_SCHEMA_VERSION,
        "handoff-id": plan.handoff_id,
        "family": plan.family,
        "revision": revision,
        "previous": previous,
        "created-at": created_at,
        "updated-at": updated_at,
        "repository": {"remote": plan.normalized_remote},
        "work-item": plan.bundle.work_item,
        "source": plan.bundle.source,
        "files": file_records,
        "history": {
            "path": history_path,
            "sha256": sha256(history_raw),
        },
    }
    return yaml_bytes(manifest)


def active_bytes(
    *,
    plan: ApplyPlan,
    revision: str,
    activated_at: str,
) -> bytes:
    entry = {
        "handoff-id": plan.handoff_id,
        "family": plan.family,
        "revision": revision,
        "manifest": f"{plan.family}/{MANIFEST_NAME}",
        "activated-at": activated_at,
        "state": "active",
    }
    handoffs = [] if plan.active is None else list(plan.active["handoffs"])
    for index, current in enumerate(handoffs):
        if current["handoff-id"] == plan.handoff_id:
            handoffs[index] = entry
            break
    else:
        handoffs.append(entry)
    return yaml_bytes(
        {
            "schema-version": ACTIVE_SCHEMA_VERSION,
            "investigation-id": plan.bundle.source["investigation-id"],
            "handoffs": handoffs,
        }
    )


def verify_ignored(target: Path) -> None:
    result = subprocess.run(
        [
            "git",
            "-C",
            str(target),
            "check-ignore",
            "--no-index",
            "-q",
            f"{STORE_NAME}/probe",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise HandoffError(
            "handoff_store_not_ignored",
            "Git does not ignore the root-local handoff store",
            path=str(target / STORE_NAME),
        )


def apply_plan(
    vault_root: Path,
    bundle_path: str,
    plan_token: str,
    worktree_path: str,
) -> dict[str, Any]:
    bundle = load_bundle(bundle_path)
    validate_tracker_binding(vault_root, bundle)
    target, normalized_remote, branch = resolve_handoff_target(
        vault_root,
        bundle.repository_remote,
        worktree_path,
    )
    _requested_family, requested_handoff_id, _requested_token = handoff_identity(
        bundle, normalized_remote
    )
    with handoff_store_lock(target):
        cleanup_committed_transaction(target)
        restore_apply_transaction(
            target,
            plan_token,
            bundle.fingerprint,
            requested_handoff_id,
        )
        plan = build_apply_plan_from_state(
            bundle,
            state_root=target,
            target=target,
            normalized_remote=normalized_remote,
            branch=branch,
            explicit_worktree=True,
        )
        if not hmac.compare_digest(str(plan.output["plan_token"]), plan_token):
            raise HandoffError(
                "plan_stale",
                "The bundle or target repository changed after planning; create a new plan",
                exit_code=3,
            )
        action = plan.output["action"]
        if action == "noop":
            return {
                "status": "unchanged",
                "action": "noop",
                "target": plan.output["target"],
                "handoff": plan.output["handoff"],
            }
        store = plan.target / STORE_NAME
        family_path = store / plan.family
        history_directory = family_path / "history"
        revision = plan.output["handoff"]["revision"]
        previous = plan.output["handoff"]["previous_revision"]
        transaction_started = start_apply_transaction(plan, revision)
        try:
            if plan.ignore_action != "none":
                transactional_write(
                    plan.target,
                    plan.target / ".gitignore",
                    plan.desired_ignore,
                )
            verify_ignored(plan.target)
            for name, instruction_action in plan.instruction_actions.items():
                if instruction_action != "none":
                    transactional_write(
                        plan.target,
                        plan.target / name,
                        plan.desired_instructions[name],
                    )

            for path, label in (
                (store, "handoff store"),
                (family_path, "handoff family"),
                (history_directory, "handoff history"),
            ):
                ensure_safe_directory(path, label=label)
                path.mkdir(parents=True, exist_ok=True)

            if plan.updates_action == "create":
                transactional_write(
                    plan.target,
                    family_path / UPDATES_NAME,
                    read_asset(UPDATES_ASSET),
                )

            timestamp = now_utc()
            if action in {"create", "update"}:
                if plan.existing is None:
                    transactional_write(
                        plan.target,
                        family_path / START_NAME,
                        read_asset(START_ASSET),
                    )
                event_raw = history_event_bytes(
                    plan=plan,
                    revision=revision,
                    previous=previous,
                    previous_event_sha256=(
                        None
                        if plan.existing is None
                        else str(plan.existing.manifest["history"]["sha256"])
                    ),
                    created_at=timestamp,
                )
                event_path = history_directory / f"{revision}.md"
                if event_path.exists() or event_path.is_symlink():
                    raise HandoffError(
                        "revision_already_exists",
                        "The planned immutable history event already exists",
                        path=str(event_path),
                    )
                transactional_write(plan.target, event_path, event_raw)
                for name in plan.changed_documents:
                    transactional_write(
                        plan.target,
                        family_path / name,
                        plan.bundle.documents[name].raw,
                    )
                created_at = (
                    timestamp
                    if plan.existing is None
                    else str(plan.existing.manifest["created-at"])
                )
                manifest_raw = manifest_bytes(
                    plan=plan,
                    revision=revision,
                    previous=previous,
                    history_path=f"history/{revision}.md",
                    history_raw=event_raw,
                    created_at=created_at,
                    updated_at=timestamp,
                )
                transactional_write(
                    plan.target,
                    family_path / MANIFEST_NAME,
                    manifest_raw,
                )

            if action in {"create", "update", "activate"}:
                transactional_write(
                    plan.target,
                    store / ACTIVE_NAME,
                    active_bytes(
                        plan=plan,
                        revision=revision,
                        activated_at=timestamp,
                    ),
                )

            validation = validate_repository(
                vault_root,
                plan.bundle.repository_remote,
                worktree_path=worktree_path,
                allow_inactive=False,
                allow_pending_transaction=True,
            )
            if transaction_started:
                commit_apply_transaction(plan.target)
        except Exception as error:
            if transaction_started:
                try:
                    restore_apply_transaction(
                        plan.target,
                        plan_token,
                        plan.bundle.fingerprint,
                        plan.handoff_id,
                    )
                except Exception as recovery_error:
                    raise HandoffError(
                        "handoff_recovery_failed",
                        "The handoff apply failed and its original state could not be restored",
                        exit_code=4,
                        cause=(
                            error.code
                            if isinstance(error, HandoffError)
                            else type(error).__name__
                        ),
                        recovery_error=(
                            recovery_error.code
                            if isinstance(recovery_error, HandoffError)
                            else type(recovery_error).__name__
                        ),
                    ) from recovery_error
            raise
    return {
        "status": "applied",
        "action": action,
        "target": plan.output["target"],
        "handoff": plan.output["handoff"],
        "validation": validation,
    }


def validate_repository(
    vault_root: Path,
    remote: str,
    *,
    worktree_path: str,
    allow_inactive: bool = True,
    allow_pending_transaction: bool = False,
) -> dict[str, Any]:
    target, normalized_remote, branch = resolve_handoff_target(
        vault_root,
        remote,
        worktree_path,
    )
    _desired_instructions, instruction_actions = prepare_instructions(target)
    stale_instructions = [
        name for name, action in instruction_actions.items() if action != "none"
    ]
    if stale_instructions:
        raise HandoffError(
            "managed_instruction_block_outdated",
            "The root instruction files do not contain the current exact managed handoff policy",
            paths=stale_instructions,
        )
    desired_ignore, ignore_action = prepare_gitignore(target)
    del desired_ignore
    if ignore_action != "none":
        raise HandoffError(
            "handoff_ignore_rule_missing",
            "The exact root-local handoff ignore rule is missing",
        )
    verify_ignored(target)
    store = target / STORE_NAME
    ensure_safe_directory(store, label="handoff store")
    if not allow_pending_transaction:
        ensure_no_pending_transaction(store)
    active, _ = read_active(store)
    if active is None:
        if allow_inactive:
            return {
                "status": "inactive",
                "target": {
                    "path": str(target),
                    "remote": normalized_remote,
                    "branch": branch,
                },
            }
        raise HandoffError(
            "no_active_handoff",
            "The repository has no active development handoff",
        )
    active = normalize_active_registry(store, active, normalized_remote)
    handoffs: list[dict[str, Any]] = []
    for entry, existing, updates in validate_active_entries(
        store, active, normalized_remote, branch
    ):
        handoff_id = entry["handoff-id"]
        handoffs.append(
            {
                "handoff_id": handoff_id,
                "family": entry["family"],
                "revision": existing.revision,
                "state": entry["state"],
                "materialized_at": existing.manifest["updated-at"],
                "manifest": str(existing.manifest_path),
                "history": existing.manifest["history"]["path"],
                "implementation_updates": updates,
                "closure_fingerprint": closure_fingerprint(
                    target,
                    existing.family_path,
                    store / ACTIVE_NAME,
                ),
            }
        )
    return {
        "status": "valid",
        "target": {
            "path": str(target),
            "remote": normalized_remote,
            "branch": branch,
        },
        "investigation_id": active["investigation-id"],
        "handoffs": handoffs,
    }


def build_set_state_plan(
    vault_root: Path,
    remote: str,
    worktree_path: str,
    handoff_id: str,
    state: str,
    closure_fingerprint_value: str | None = None,
) -> dict[str, Any]:
    if state not in HANDOFF_STATES:
        raise HandoffError(
            "invalid_handoff_state",
            "The requested handoff state is not supported",
            state=state,
        )
    target, normalized_remote, branch = resolve_handoff_target(
        vault_root,
        remote,
        worktree_path,
    )
    _desired_instructions, instruction_actions = prepare_instructions(target)
    instructions_changed = any(
        action != "none" for action in instruction_actions.values()
    )
    store = target / STORE_NAME
    ensure_safe_directory(store, label="handoff store")
    ensure_no_pending_transaction(store)
    active, _ = read_active(store)
    if active is None:
        raise HandoffError(
            "no_active_handoff",
            "A state update requires an active worktree registry",
        )
    active = normalize_active_registry(store, active, normalized_remote)
    entry = active_entry(active, handoff_id)
    if entry is None:
        raise HandoffError(
            "handoff_not_found",
            "The requested handoff is not registered in this worktree",
            handoff_id=handoff_id,
        )
    validated = validate_active_entries(store, active, normalized_remote, branch)
    existing = next(
        existing
        for current, existing, _updates in validated
        if current["handoff-id"] == handoff_id
    )
    if state == "ready-for-production" and entry["state"] not in {
        "active",
        "ready-for-production",
    }:
        raise HandoffError(
            "invalid_state_transition",
            "ready-for-production requires an active handoff",
            current_state=entry["state"],
        )
    if state == "ready-for-production" and entry["state"] == "active":
        if (
            closure_fingerprint_value is None
            or re.fullmatch(r"[a-f0-9]{64}", closure_fingerprint_value) is None
        ):
            raise HandoffError(
                "reconciliation_fingerprint_required",
                "ready-for-production requires the current closure fingerprint",
            )
        observed_fingerprint = closure_fingerprint(
            target,
            existing.family_path,
            store / ACTIVE_NAME,
        )
        if not hmac.compare_digest(
            observed_fingerprint,
            closure_fingerprint_value,
        ):
            raise HandoffError(
                "reconciliation_snapshot_mismatch",
                "The local evidence changed after the reconciliation snapshot was validated",
                exit_code=3,
            )
    elif state != "ready-for-production" and closure_fingerprint_value is not None:
        raise HandoffError(
            "unexpected_reconciliation_fingerprint",
            "Only ready-for-production accepts a closure fingerprint",
        )
    if state == "production" and entry["state"] not in {
        "ready-for-production",
        "production",
    }:
        raise HandoffError(
            "invalid_state_transition",
            "production requires a ready-for-production handoff",
            current_state=entry["state"],
        )
    if entry["state"] != state:
        action = "set-state"
    elif instructions_changed:
        action = "refresh-policy"
    else:
        action = "noop"
    effects: list[dict[str, object]] = []
    for name, instruction_action in instruction_actions.items():
        if instruction_action != "none":
            effects.append(effect(target, name, instruction_action))
    if action == "set-state":
        effects.append(
            effect(
                target,
                f"{STORE_NAME}/{ACTIVE_NAME}",
                "update",
                tracked=False,
            )
        )
    token = sha256(
        canonical_json(
            {
                "schema_version": CONTROL_SCHEMA_VERSION,
                "operation": "set-state",
                "handoff_id": handoff_id,
                "state": state,
                "closure_fingerprint": closure_fingerprint_value,
                "target": str(target),
                "remote": normalized_remote,
                "active": path_fingerprint(store / ACTIVE_NAME),
                "instructions": {
                    name: path_fingerprint(target / name)
                    for name in INSTRUCTION_FILES
                },
                "agents_override": path_fingerprint(
                    target / "AGENTS.override.md"
                ),
                "agents_asset": sha256(read_asset(AGENTS_BLOCK_ASSET)),
                "effects": effects,
            }
        )
    )
    return {
        "status": "planned",
        "operation": "set-state",
        "handoff_id": handoff_id,
        "state": state,
        "action": action,
        "target": {
            "path": str(target),
            "remote": normalized_remote,
            "branch": branch,
        },
        "active": active,
        "policy": instruction_actions,
        "effects": effects,
        "plan_token": token,
    }


def set_state(
    vault_root: Path,
    remote: str,
    plan_token: str | None,
    worktree_path: str,
    handoff_id: str,
    state: str,
    closure_fingerprint_value: str | None = None,
) -> dict[str, Any]:
    if plan_token is None:
        return build_set_state_plan(
            vault_root,
            remote,
            worktree_path,
            handoff_id,
            state,
            closure_fingerprint_value,
        )
    target, _normalized_remote, _branch = resolve_handoff_target(
        vault_root,
        remote,
        worktree_path,
    )
    with handoff_store_lock(target):
        cleanup_committed_transaction(target)
        plan = build_set_state_plan(
            vault_root,
            remote,
            worktree_path,
            handoff_id,
            state,
            closure_fingerprint_value,
        )
        if not hmac.compare_digest(plan["plan_token"], plan_token):
            raise HandoffError(
                "plan_stale",
                "The state-update inputs changed after planning; create a new plan",
                exit_code=3,
            )
        if plan["action"] == "noop":
            return {
                "status": "unchanged",
                "action": "noop",
                "handoff_id": plan["handoff_id"],
                "state": plan["state"],
                "target": plan["target"],
            }
        if any(action != "none" for action in plan["policy"].values()):
            desired_instructions, instruction_actions = prepare_instructions(target)
            if instruction_actions != plan["policy"]:
                raise HandoffError(
                    "plan_stale",
                    "The managed instruction policy changed after planning",
                    exit_code=3,
                )
            for name, instruction_action in instruction_actions.items():
                if instruction_action != "none":
                    atomic_write(target / name, desired_instructions[name])
        if plan["action"] == "refresh-policy":
            return {
                "status": "policy-refreshed",
                "action": "refresh-policy",
                "handoff_id": plan["handoff_id"],
                "state": plan["state"],
                "target": plan["target"],
            }
        active_path = target / STORE_NAME / ACTIVE_NAME
        if active_path.is_symlink() or not active_path.is_file():
            raise HandoffError(
                "unsafe_write_target",
                "ACTIVE.yaml is not a writable regular file",
                path=str(active_path),
            )
        active = plan["active"]
        entry = active_entry(active, handoff_id)
        if entry is None:
            raise HandoffError(
                "plan_stale",
                "The handoff was removed after the state update was planned",
                exit_code=3,
                handoff_id=handoff_id,
            )
        entry["state"] = state
        if state == "active":
            entry["activated-at"] = now_utc()
        atomic_write(active_path, yaml_bytes(active))
    return {
        "status": "state-updated",
        "action": "set-state",
        "handoff_id": handoff_id,
        "state": state,
        "target": plan["target"],
    }


class ArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise HandoffError("usage_error", message)


def build_parser() -> ArgumentParser:
    parser = ArgumentParser(description=__doc__)
    parser.add_argument("--vault-root", required=True)
    commands = parser.add_subparsers(
        dest="command",
        required=True,
        parser_class=ArgumentParser,
    )

    plan_worktree = commands.add_parser(
        "plan-worktree",
        help="Plan one persistent work-item worktree without writing",
    )
    plan_worktree.add_argument("--bundle", required=True)
    plan_worktree.add_argument("--base-branch", required=True)
    plan_worktree.add_argument(
        "--base-source",
        choices=("local", "remote"),
        required=True,
    )
    plan_worktree.add_argument("--description", required=True)
    plan_worktree.add_argument("--branch-prefix", default="issue")

    create_worktree_parser = commands.add_parser(
        "create-worktree",
        help="Create one unchanged approved work-item worktree plan",
    )
    create_worktree_parser.add_argument("--bundle", required=True)
    create_worktree_parser.add_argument("--base-branch", required=True)
    create_worktree_parser.add_argument(
        "--base-source",
        choices=("local", "remote"),
        required=True,
    )
    create_worktree_parser.add_argument("--description", required=True)
    create_worktree_parser.add_argument("--branch-prefix", default="issue")
    create_worktree_parser.add_argument("--plan-token", required=True)

    plan_handoff = commands.add_parser(
        "plan-handoff",
        help="Plan worktree creation and initial handoff materialization together",
    )
    plan_handoff.add_argument("--bundle", required=True)
    plan_handoff.add_argument("--base-branch", required=True)
    plan_handoff.add_argument(
        "--base-source",
        choices=("local", "remote"),
        required=True,
    )
    plan_handoff.add_argument("--description", required=True)
    plan_handoff.add_argument("--branch-prefix", default="issue")

    prepare_handoff = commands.add_parser(
        "prepare-handoff",
        help="Create and materialize one unchanged complete handoff plan",
    )
    prepare_handoff.add_argument("--bundle", required=True)
    prepare_handoff.add_argument("--base-branch", required=True)
    prepare_handoff.add_argument(
        "--base-source",
        choices=("local", "remote"),
        required=True,
    )
    prepare_handoff.add_argument("--description", required=True)
    prepare_handoff.add_argument("--branch-prefix", default="issue")
    prepare_handoff.add_argument("--plan-token", required=True)

    plan = commands.add_parser("plan", help="Create a read-only handoff effect plan")
    plan.add_argument("--bundle", required=True)
    plan.add_argument("--worktree-path", required=True)

    apply = commands.add_parser("apply", help="Apply one unchanged approved plan")
    apply.add_argument("--bundle", required=True)
    apply.add_argument("--worktree-path", required=True)
    apply.add_argument("--plan-token", required=True)

    validate = commands.add_parser("validate", help="Validate a worktree handoff registry")
    validate.add_argument("--repository-remote", required=True)
    validate.add_argument("--worktree-path", required=True)

    resolve_branch = commands.add_parser(
        "resolve-branch",
        help="Resolve portable repository and branch identity to a verified local worktree",
    )
    resolve_branch.add_argument("--repository-remote", required=True)
    resolve_branch.add_argument("--branch", required=True)

    state_parser = commands.add_parser(
        "set-state",
        help="Plan or apply one handoff state update",
    )
    state_parser.add_argument("--repository-remote", required=True)
    state_parser.add_argument("--worktree-path", required=True)
    state_parser.add_argument("--handoff-id", required=True)
    state_parser.add_argument(
        "--state",
        choices=tuple(sorted(HANDOFF_STATES)),
        required=True,
    )
    state_parser.add_argument("--closure-fingerprint")
    state_parser.add_argument("--plan-token")
    return parser


def main(argv: list[str] | None = None) -> int:
    try:
        args = build_parser().parse_args(argv)
        vault_root = resolve_vault(args.vault_root)
        if args.command == "plan-worktree":
            emit_json(
                build_worktree_plan(
                    vault_root,
                    args.bundle,
                    args.base_branch,
                    args.base_source,
                    args.description,
                    args.branch_prefix,
                ).output
            )
            return 0
        if args.command == "create-worktree":
            emit_json(
                create_worktree(
                    vault_root,
                    args.bundle,
                    args.base_branch,
                    args.base_source,
                    args.description,
                    args.branch_prefix,
                    args.plan_token,
                )
            )
            return 0
        if args.command == "plan-handoff":
            emit_json(
                build_complete_handoff_plan(
                    vault_root,
                    args.bundle,
                    args.base_branch,
                    args.base_source,
                    args.description,
                    args.branch_prefix,
                ).output
            )
            return 0
        if args.command == "prepare-handoff":
            emit_json(
                prepare_complete_handoff(
                    vault_root,
                    args.bundle,
                    args.base_branch,
                    args.base_source,
                    args.description,
                    args.branch_prefix,
                    args.plan_token,
                )
            )
            return 0
        if args.command == "plan":
            emit_json(
                build_apply_plan(
                    vault_root,
                    args.bundle,
                    args.worktree_path,
                ).output
            )
            return 0
        if args.command == "apply":
            emit_json(
                apply_plan(
                    vault_root,
                    args.bundle,
                    args.plan_token,
                    args.worktree_path,
                )
            )
            return 0
        if args.command == "validate":
            emit_json(
                validate_repository(
                    vault_root,
                    args.repository_remote,
                    worktree_path=args.worktree_path,
                )
            )
            return 0
        if args.command == "resolve-branch":
            emit_json(
                resolve_branch_target(
                    vault_root,
                    args.repository_remote,
                    args.branch,
                )
            )
            return 0
        if args.command == "set-state":
            emit_json(
                set_state(
                    vault_root,
                    args.repository_remote,
                    args.plan_token,
                    args.worktree_path,
                    args.handoff_id,
                    args.state,
                    args.closure_fingerprint,
                )
            )
            return 0
        raise HandoffError("usage_error", "Unknown command")
    except HandoffError as error:
        emit_json(error.payload(), stream=sys.stderr)
        return error.exit_code
    except (OSError, subprocess.SubprocessError) as error:
        emit_json(
            HandoffError(
                "operational_error",
                "The development-handoff operation failed",
                exit_code=4,
                error_type=type(error).__name__,
            ).payload(),
            stream=sys.stderr,
        )
        return 4


if __name__ == "__main__":
    raise SystemExit(main())
