#!/usr/bin/env python3
"""Plan, prepare, apply, validate, or deactivate Jira development handoffs."""
from __future__ import annotations

import argparse
import codecs
import difflib
import hashlib
import hmac
import io
import json
import os
import re
import subprocess
import sys
import tempfile
import unicodedata
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from ruamel.yaml import YAML
from ruamel.yaml.error import YAMLError


SCHEMA_VERSION = 1
DOCUMENTS = ("context.md", "jira.md", "scope.md")
MANAGED_BEGIN = '<!-- knowledge-os:managed:start id="development handoff" -->'
MANAGED_END = '<!-- knowledge-os:managed:end id="development handoff" -->'
LEGACY_MANAGED_BEGIN = "<!-- BEGIN MANAGED: System A-System B DEVELOPMENT HANDOFF -->"
LEGACY_MANAGED_END = "<!-- END MANAGED: System A-System B DEVELOPMENT HANDOFF -->"
INSTRUCTION_FILES = ("AGENTS.md", "CLAUDE.md")
IGNORE_RULE = "/.knowledge-os-handoffs/"
STORE_NAME = ".knowledge-os-handoffs"
ACTIVE_NAME = "ACTIVE.yaml"
MANIFEST_NAME = "handoff.yaml"
START_NAME = "START.md"
UPDATES_NAME = "implementation-updates.md"
MAX_DOCUMENT_BYTES = 1_048_576
MAX_BUNDLE_BYTES = 3_145_728
MAX_AGENTS_BYTES = 32_768
MAX_UPDATES_BYTES = 1_048_576
REVISION_RE = re.compile(r"v([0-9]{4})")
UPDATE_HEADING_RE = re.compile(r"^## UPD-([0-9]{3,}) — (.+)$")
ISSUE_KEY_RE = re.compile(r"[A-Z][A-Z0-9]+-[0-9]+")
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
    jira: dict[str, str]
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


def normalize_site(value: object) -> str:
    raw = expect_string(value, field="jira.site", maximum=512)
    parsed = urlparse(raw)
    if (
        parsed.scheme.casefold() != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        raise HandoffError(
            "invalid_jira_site",
            "jira.site must be a credential-free HTTPS base URL",
            field="jira.site",
        )
    host = parsed.hostname.casefold()
    if parsed.port:
        host = f"{host}:{parsed.port}"
    path = parsed.path.rstrip("/")
    return f"https://{host}{path}"


def validate_jira_url(value: object, *, site: str, issue_key: str) -> str:
    raw = expect_string(value, field="jira.url", maximum=1_024)
    parsed = urlparse(raw)
    site_parsed = urlparse(site)
    if (
        parsed.scheme.casefold() != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.hostname.casefold() != (site_parsed.hostname or "").casefold()
        or issue_key.casefold() not in parsed.path.casefold()
    ):
        raise HandoffError(
            "invalid_jira_url",
            "jira.url must be a credential-free HTTPS URL for the copied issue",
            field="jira.url",
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
        {"schema-version", "source", "jira", "repository", "change"},
        field="bundle",
    )
    if metadata["schema-version"] != SCHEMA_VERSION:
        raise HandoffError(
            "unsupported_bundle_schema",
            "The bundle schema version is unsupported",
            expected=SCHEMA_VERSION,
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

    jira_value = expect_mapping(metadata["jira"], field="jira")
    expect_exact_keys(
        jira_value,
        {
            "site",
            "issue-key",
            "url",
            "updated-at",
            "captured-at",
            "freshness",
            "snapshot-source",
        },
        field="jira",
    )
    issue_key = expect_string(
        jira_value["issue-key"],
        field="jira.issue-key",
        maximum=64,
    )
    if not ISSUE_KEY_RE.fullmatch(issue_key):
        raise HandoffError(
            "invalid_issue_key",
            "jira.issue-key must use the canonical uppercase Jira key",
            field="jira.issue-key",
        )
    site = normalize_site(jira_value["site"])
    freshness = expect_string(
        jira_value["freshness"],
        field="jira.freshness",
        maximum=16,
    )
    if freshness != "current":
        raise HandoffError(
            "stale_jira_snapshot",
            "The copied Jira snapshot must be verified current before handoff",
            field="jira.freshness",
        )
    snapshot_source = expect_string(
        jira_value["snapshot-source"],
        field="jira.snapshot-source",
        maximum=32,
    )
    if snapshot_source not in {"connected-readback", "user-supplied-export"}:
        raise HandoffError(
            "invalid_jira_snapshot_source",
            "jira.snapshot-source must identify a supported exact-copy provenance",
            field="jira.snapshot-source",
        )
    jira = {
        "site": site,
        "issue-key": issue_key,
        "url": validate_jira_url(
            jira_value["url"],
            site=site,
            issue_key=issue_key,
        ),
        "updated-at": expect_timestamp(
            jira_value["updated-at"],
            field="jira.updated-at",
        ),
        "captured-at": expect_timestamp(
            jira_value["captured-at"],
            field="jira.captured-at",
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
    if issue_key.casefold() not in documents["jira.md"].text.casefold():
        raise HandoffError(
            "jira_snapshot_identity_mismatch",
            "jira.md must identify the copied Jira issue",
            document="jira.md",
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
        jira=jira,
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
            payload = json.loads(result.stderr)
            if isinstance(payload, dict):
                error = payload.get("error")
                if isinstance(error, dict) and isinstance(error.get("code"), str):
                    cause = error["code"]
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
            payload = json.loads(result.stderr)
            if isinstance(payload, dict):
                error = payload.get("error")
                if isinstance(error, dict) and isinstance(error.get("code"), str):
                    cause = error["code"]
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
            "The worktree path must use <root>/<repository>/<jira-description>",
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
    worktree_path: str | None,
) -> tuple[Path, str, str]:
    if worktree_path is not None:
        return resolve_explicit_worktree(vault_root, remote, worktree_path)
    target, normalized_remote = resolve_repository(vault_root, remote)
    return target, normalized_remote, current_branch(target)


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


def handoff_identity(
    bundle: Bundle,
    normalized_remote: str,
) -> tuple[str, str]:
    repository_name = normalized_remote.rsplit("/", 1)[-1]
    family = f"{slug(bundle.jira['issue-key'])}--{slug(repository_name)}"
    identity = "|".join(
        (
            bundle.jira["site"].casefold(),
            bundle.jira["issue-key"],
            normalized_remote,
        )
    )
    return family, sha256(identity.encode("utf-8"))


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
    observed: list[tuple[str, str]] = []
    for begin_marker, end_marker in (
        (MANAGED_BEGIN, MANAGED_END),
        (LEGACY_MANAGED_BEGIN, LEGACY_MANAGED_END),
    ):
        begin_count = text.count(begin_marker)
        end_count = text.count(end_marker)
        if begin_count == 0 and end_count == 0:
            continue
        if begin_count != 1 or end_count != 1:
            raise HandoffError(
                "invalid_managed_block",
                f"{label} must contain zero or one complete managed handoff block",
                path=label,
            )
        observed.append((begin_marker, end_marker))
    if not observed:
        return None
    if len(observed) != 1:
        raise HandoffError(
            "invalid_managed_block",
            f"{label} contains more than one managed handoff block",
            path=label,
        )
    begin_marker, end_marker = observed[0]
    begin = text.index(begin_marker)
    end_start = text.index(end_marker)
    if end_start < begin:
        raise HandoffError(
            "invalid_managed_block",
            f"The managed handoff markers in {label} are inverted",
            path=label,
        )
    return begin, end_start + len(end_marker)


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
            "The active handoff has no implementation update changelog",
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
            event["schema-version"] != SCHEMA_VERSION
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
    if (
        manifest.get("schema-version") != SCHEMA_VERSION
        or manifest.get("handoff-id") != handoff_id
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
    expected = {
        "schema-version",
        "handoff-id",
        "family",
        "revision",
        "manifest",
        "activated-at",
    }
    expect_exact_keys(active, expected, field="ACTIVE.yaml")
    if active["schema-version"] != SCHEMA_VERSION:
        raise HandoffError(
            "unsupported_active_schema",
            "ACTIVE.yaml uses an unsupported schema version",
        )
    family = expect_string(active["family"], field="ACTIVE.yaml.family", maximum=200)
    if FAMILY_RE.fullmatch(family) is None:
        raise HandoffError(
            "invalid_active_pointer",
            "ACTIVE.yaml contains a non-canonical family",
        )
    revision = expect_string(
        active["revision"],
        field="ACTIVE.yaml.revision",
        maximum=16,
    )
    if REVISION_RE.fullmatch(revision) is None:
        raise HandoffError(
            "invalid_active_pointer",
            "ACTIVE.yaml contains an invalid revision",
        )
    expected_manifest = f"{family}/{MANIFEST_NAME}"
    if validate_relative_path(
        active["manifest"],
        field="ACTIVE.yaml.manifest",
    ) != expected_manifest:
        raise HandoffError(
            "invalid_active_pointer",
            "ACTIVE.yaml must point to the stable family manifest",
        )
    return active, raw


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
    issue_key = bundle.jira["issue-key"]
    branch = f"{branch_prefix}/{issue_key}-{description_slug}"
    branch_check = subprocess.run(
        ["git", "check-ref-format", "--branch", branch],
        text=True,
        capture_output=True,
        check=False,
    )
    if branch_check.returncode != 0:
        raise HandoffError(
            "invalid_worktree_branch",
            "The Jira issue and description did not produce a valid Git branch",
        )

    repository_directory = worktree_root / observed_repository_basename(
        source,
        normalized_remote,
    )
    path = repository_directory / f"{issue_key}-{description_slug}"
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
        "schema_version": SCHEMA_VERSION,
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
    branch_leaf = branch.rsplit("/", 1)[-1]
    if explicit_worktree and (
        "/" not in branch
        or not branch_leaf.startswith(f"{bundle.jira['issue-key']}-")
    ):
        raise HandoffError(
            "worktree_branch_mismatch",
            "The selected worktree branch does not match the handoff Jira issue",
            exit_code=3,
            branch=branch,
            issue_key=bundle.jira["issue-key"],
        )
    family, handoff_id = handoff_identity(bundle, normalized_remote)
    store = state_root / STORE_NAME
    family_path = store / family
    ensure_safe_directory(store, label="handoff store")
    existing = read_existing_handoff(
        family_path,
        family=family,
        handoff_id=handoff_id,
        normalized_remote=normalized_remote,
    )
    active, _ = read_active(store)
    desired_instructions, instruction_actions = prepare_instructions(state_root)
    instructions_changed = any(
        action != "none" for action in instruction_actions.values()
    )
    desired_ignore, ignore_action = prepare_gitignore(state_root)
    updates_action, updates_summary = prepare_implementation_updates(family_path)
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
            active_matches = (
                active is not None
                and active["handoff-id"] == handoff_id
                and active["family"] == family
                and active["revision"] == revision
            )
            if not active_matches:
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
        "schema_version": SCHEMA_VERSION,
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
    worktree_path: str | None = None,
) -> ApplyPlan:
    bundle = load_bundle(bundle_path)
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
        explicit_worktree=worktree_path is not None,
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
        "schema_version": SCHEMA_VERSION,
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
        old_semantic: str | None = None
        previous_hash: str | None = None
        if plan.existing is not None:
            old_raw = (plan.existing.family_path / name).read_bytes()
            old_semantic = semantic_text(old_raw.decode("utf-8-sig"))
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
        "schema-version": SCHEMA_VERSION,
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
        "schema-version": SCHEMA_VERSION,
        "handoff-id": plan.handoff_id,
        "family": plan.family,
        "revision": revision,
        "previous": previous,
        "created-at": created_at,
        "updated-at": updated_at,
        "repository": {"remote": plan.normalized_remote},
        "story": plan.bundle.jira,
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
    return yaml_bytes(
        {
            "schema-version": SCHEMA_VERSION,
            "handoff-id": plan.handoff_id,
            "family": plan.family,
            "revision": revision,
            "manifest": f"{plan.family}/{MANIFEST_NAME}",
            "activated-at": activated_at,
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
    worktree_path: str | None = None,
) -> dict[str, Any]:
    plan = build_apply_plan(vault_root, bundle_path, worktree_path)
    observed_token = plan.output["plan_token"]
    if not hmac.compare_digest(str(observed_token), plan_token):
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

    if plan.ignore_action != "none":
        atomic_write(plan.target / ".gitignore", plan.desired_ignore)
    verify_ignored(plan.target)
    for name, instruction_action in plan.instruction_actions.items():
        if instruction_action != "none":
            atomic_write(plan.target / name, plan.desired_instructions[name])

    store = plan.target / STORE_NAME
    family_path = store / plan.family
    history_directory = family_path / "history"
    for path, label in (
        (store, "handoff store"),
        (family_path, "handoff family"),
        (history_directory, "handoff history"),
    ):
        ensure_safe_directory(path, label=label)
        path.mkdir(parents=True, exist_ok=True)

    if plan.updates_action == "create":
        atomic_write(family_path / UPDATES_NAME, read_asset(UPDATES_ASSET))

    revision = plan.output["handoff"]["revision"]
    previous = plan.output["handoff"]["previous_revision"]
    timestamp = now_utc()
    if action in {"create", "update"}:
        if plan.existing is None:
            atomic_write(family_path / START_NAME, read_asset(START_ASSET))
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
        atomic_write(event_path, event_raw)
        for name in plan.changed_documents:
            atomic_write(family_path / name, plan.bundle.documents[name].raw)
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
        atomic_write(family_path / MANIFEST_NAME, manifest_raw)

    if action in {"create", "update", "activate"}:
        atomic_write(
            store / ACTIVE_NAME,
            active_bytes(plan=plan, revision=revision, activated_at=timestamp),
        )

    validation = validate_repository(
        vault_root,
        plan.bundle.repository_remote,
        worktree_path=worktree_path,
        allow_inactive=False,
    )
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
    worktree_path: str | None = None,
    allow_inactive: bool = True,
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
    family = str(active["family"])
    handoff_id = str(active["handoff-id"])
    existing = read_existing_handoff(
        store / family,
        family=family,
        handoff_id=handoff_id,
        normalized_remote=normalized_remote,
    )
    if existing is None:
        raise HandoffError(
            "invalid_active_pointer",
            "ACTIVE.yaml points to a missing handoff family",
        )
    if existing.revision != active["revision"]:
        raise HandoffError(
            "invalid_active_pointer",
            "ACTIVE.yaml and handoff.yaml disagree on the active revision",
        )
    updates = validate_implementation_updates(existing.family_path / UPDATES_NAME)
    return {
        "status": "valid",
        "target": {
            "path": str(target),
            "remote": normalized_remote,
            "branch": branch,
        },
        "handoff_id": handoff_id,
        "family": family,
        "revision": existing.revision,
        "manifest": str(existing.manifest_path),
        "history": existing.manifest["history"]["path"],
        "implementation_updates": updates,
    }


def build_deactivate_plan(
    vault_root: Path,
    remote: str,
    worktree_path: str | None = None,
) -> dict[str, Any]:
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
    active, _ = read_active(store)
    if active is not None:
        existing = read_existing_handoff(
            store / str(active["family"]),
            family=str(active["family"]),
            handoff_id=str(active["handoff-id"]),
            normalized_remote=normalized_remote,
        )
        if existing is None or existing.revision != active["revision"]:
            raise HandoffError(
                "invalid_active_pointer",
                "ACTIVE.yaml does not resolve to its exact handoff revision",
            )
        validate_implementation_updates(existing.family_path / UPDATES_NAME)
    if active is not None:
        action = "deactivate"
    elif instructions_changed:
        action = "refresh-policy"
    else:
        action = "noop"
    effects: list[dict[str, object]] = []
    for name, instruction_action in instruction_actions.items():
        if instruction_action != "none":
            effects.append(effect(target, name, instruction_action))
    if active is not None:
        effects.append(
            effect(
                target,
                f"{STORE_NAME}/{ACTIVE_NAME}",
                "delete",
                tracked=False,
            )
        )
    token = sha256(
        canonical_json(
            {
                "schema_version": SCHEMA_VERSION,
                "operation": "deactivate",
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
        "operation": "deactivate",
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


def deactivate(
    vault_root: Path,
    remote: str,
    plan_token: str | None,
    worktree_path: str | None = None,
) -> dict[str, Any]:
    plan = build_deactivate_plan(vault_root, remote, worktree_path)
    if plan_token is None:
        return plan
    if not hmac.compare_digest(plan["plan_token"], plan_token):
        raise HandoffError(
            "plan_stale",
            "The active pointer changed after planning; create a new plan",
            exit_code=3,
        )
    if plan["action"] == "noop":
        return {
            "status": "unchanged",
            "action": "noop",
            "target": plan["target"],
        }
    target = Path(plan["target"]["path"])
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
            "target": plan["target"],
        }
    active_path = target / STORE_NAME / ACTIVE_NAME
    if active_path.is_symlink() or not active_path.is_file():
        raise HandoffError(
            "unsafe_write_target",
            "ACTIVE.yaml is not a removable regular file",
            path=str(active_path),
        )
    active_path.unlink()
    return {
        "status": "deactivated",
        "action": "deactivate",
        "target": plan["target"],
        "preserved_family": plan["active"]["family"],
        "preserved_revision": plan["active"]["revision"],
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
        help="Plan one persistent Jira worktree without writing",
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
        help="Create one unchanged approved Jira worktree plan",
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
    plan.add_argument("--worktree-path")

    apply = commands.add_parser("apply", help="Apply one unchanged approved plan")
    apply.add_argument("--bundle", required=True)
    apply.add_argument("--worktree-path")
    apply.add_argument("--plan-token", required=True)

    validate = commands.add_parser("validate", help="Validate an active handoff")
    validate.add_argument("--repository-remote", required=True)
    validate.add_argument("--worktree-path")

    deactivate_parser = commands.add_parser(
        "deactivate",
        help="Plan or apply removal of only the active pointer",
    )
    deactivate_parser.add_argument("--repository-remote", required=True)
    deactivate_parser.add_argument("--worktree-path")
    deactivate_parser.add_argument("--plan-token")
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
        if args.command == "deactivate":
            emit_json(
                deactivate(
                    vault_root,
                    args.repository_remote,
                    args.plan_token,
                    args.worktree_path,
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
