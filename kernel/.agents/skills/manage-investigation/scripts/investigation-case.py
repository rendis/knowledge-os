#!/usr/bin/env python3
"""Transactional filesystem mechanics for investigation case files."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import time
import unicodedata
import uuid
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any, Iterator
from urllib.parse import unquote, urlparse


ID_PATTERN = re.compile(r"^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$")
KEY_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
REGISTER_ID_PATTERN = re.compile(r"^(?:E|A|Q|D|AC|S|DH)-[0-9]{3,}$")
SECRET_PATTERN = re.compile(
    r"PRIVATE KEY-----|"
    r"\b(?:password|passwd|pwd|secret|token|api[-_]?key)\s*[:=]\s*\S+|"
    r"\b(?:gh[pousr]_|github_pat_|sk-(?:proj-|svcacct-)?|xox[baprs]-)\S+",
    re.IGNORECASE,
)
LOCAL_PATH_PATTERN = re.compile(
    r"(?<![A-Za-z0-9])(?:/(?:Users|home|tmp|var/tmp)/[^\s`]+|[A-Za-z]:\\[^\s`]+)"
)
UNPUBLISHED_ROOT_NAME = ".investigations"
PRIVATE_ROOT_NAME = ".investigations-private"
ORIGINAL_REQUEST_HEADINGS = {"Original request", "Solicitud original"}
GENERIC_SOURCE_REFS = {"", "unknown", "message", "customer conversation", "conversation"}
PURPOSES = {"knowledge", "development", "mixed", "undecided"}
VAULT_OUTCOMES = {
    "not-evaluated",
    "none",
    "deferred-until-production",
    "candidate-for-audit",
    "documented",
}
LEARNING_OUTCOMES = {
    "not-evaluated",
    "no-learning",
    "already-covered",
    "insufficient-evidence",
    "candidate",
    "documented",
}
STATUSES = {"investigating", "blocked", "closed"}
CLOSURE_OUTCOMES = {"completed", "abandoned"}
OBSOLETE_STATUSES = {
    "intake",
    "scoped",
    "validating",
    "ready-to-export",
    "exported",
}
CURRENT_PRODUCTIVE_STATE_ALIASES = (
    "Current productive state",
    "Estado productivo actual",
)
FUTURE_PROPOSED_STATE_ALIASES = (
    "Future/proposed state",
    "Estado futuro/propuesto",
    "Estado futuro o propuesto",
)
DEVELOPMENT_HANDOFF_SECTION_ALIASES = (
    "Development handoffs",
    "Handoffs de desarrollo",
)
HISTORY_SECTION_ALIASES = ("History", "Historial")
REGISTER_SECTION_ALIASES = {
    "E": ("Evidence", "Evidencia"),
    "A": ("References and attachments", "Referencias y adjuntos"),
    "Q": ("Open questions", "Preguntas abiertas"),
    "D": ("Decisions", "Decisiones"),
    "AC": ("Acceptance criteria", "Criterios de aceptación"),
    "DH": DEVELOPMENT_HANDOFF_SECTION_ALIASES,
}
DEVELOPMENT_HANDOFF_FIELDS = (
    "Story ID",
    "Tracker ID",
    "Provider",
    "Tracker URL",
    "Work item reference",
    "Repository remote",
    "Branch",
    "Handoff ID",
    "Family",
    "Revision",
    "Materialized at",
)
DEVELOPMENT_HANDOFF_HEADING_RE = re.compile(
    r"^### (DH-([0-9]{3,})) — (.+)$"
)
DEVELOPMENT_HANDOFF_FIELD_RE = re.compile(r"^- ([A-Za-z ]+): (.+)$")
DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN = (
    "knowledge-os:development-handoff-binding"
)
DEVELOPMENT_HANDOFF_HISTORY_MARKER_RE = re.compile(
    r"^  <!-- "
    + re.escape(DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN)
    + r" (\{.*\}) -->$"
)
DEVELOPMENT_HANDOFF_HISTORY_KEYS = (
    "dh",
    "story-id",
    "tracker-id",
    "provider",
    "tracker-url",
    "work-item-reference",
    "repository-remote",
    "branch",
    "handoff-id",
    "family",
    "revision",
    "materialized-at",
)
STORY_ID_RE = re.compile(r"^S-[0-9]{3,}$")
TRACKER_NAME_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
HANDOFF_ID_RE = re.compile(r"^[a-f0-9]{64}$")
HANDOFF_FAMILY_RE = re.compile(
    r"^[a-z0-9]+(?:-[a-z0-9]+)*--[a-z0-9]+(?:-[a-z0-9]+)*$"
)
HANDOFF_REVISION_RE = re.compile(r"^v[0-9]{4}$")
REQUIRED_FIELDS = (
    "id",
    "title",
    "dedupe-key",
    "status",
    "created-at",
    "updated-at",
    "source-type",
    "source-ref",
    "requester-role",
    "export-intent",
    "purpose",
    "vault-outcome",
    "learning-outcome",
)
REQUIRED_SECTIONS = (
    ("Request summary", "Resumen de la solicitud"),
    ("Current state", "Estado actual", "Estado vigente"),
    ("References and attachments", "Referencias y adjuntos"),
    ("Evidence", "Evidencia"),
    ("Affected surfaces", "Superficies afectadas"),
    DEVELOPMENT_HANDOFF_SECTION_ALIASES,
    ("Open questions", "Preguntas abiertas"),
    ("Decisions", "Decisiones"),
    ("Acceptance criteria", "Criterios de aceptación"),
    ("Readiness", "Preparación"),
    ("History", "Historial"),
)


class CaseError(Exception):
    def __init__(self, code: str, message: str, *, exit_code: int = 2, **details: Any):
        super().__init__(message)
        self.code = code
        self.message = message
        self.exit_code = exit_code
        self.details = details

    def payload(self) -> dict[str, Any]:
        return {
            "status": "error",
            "error": {"code": self.code, "message": self.message, **self.details},
        }


@dataclass(frozen=True)
class CaseRecord:
    path: Path
    text: str
    fields: dict[str, Any]

    @property
    def case_id(self) -> str:
        return str(self.fields.get("id", ""))

    @property
    def dedupe_key(self) -> str:
        return str(self.fields.get("dedupe-key", ""))

    @property
    def source_ref(self) -> str:
        return str(self.fields.get("source-ref", ""))

    @property
    def title(self) -> str:
        return str(self.fields.get("title", ""))

    @property
    def consolidated_from(self) -> list[str]:
        value = self.fields.get("consolidated-from", [])
        return [str(item) for item in value] if isinstance(value, list) else []


@dataclass(frozen=True)
class LocatedCase:
    record: CaseRecord
    visibility: str
    store: Path


@dataclass(frozen=True)
class GitIdentity:
    name: str
    email: str

    @property
    def display(self) -> str:
        return f"{self.name} <{self.email}>"


def emit(payload: dict[str, Any], *, stream: Any = sys.stdout) -> None:
    print(json.dumps(payload, indent=2, sort_keys=True), file=stream)


def normalize_text(value: str) -> str:
    normalized = unicodedata.normalize("NFKD", value)
    ascii_value = normalized.encode("ascii", "ignore").decode("ascii").casefold()
    return "-".join(re.findall(r"[a-z0-9]+", ascii_value))


def normalized_source_ref(value: str) -> str:
    return " ".join(value.casefold().split())


def reject_secret_input(*values: str) -> None:
    if any(SECRET_PATTERN.search(value) for value in values):
        raise CaseError(
            "secret_input",
            "Secret-bearing input cannot be persisted in an investigation",
        )


def effective_git_identity(vault_root: Path) -> GitIdentity:
    values: dict[str, str] = {}
    for field in ("user.name", "user.email"):
        result = subprocess.run(
            ["git", "-C", str(vault_root), "config", "--get", field],
            text=True,
            capture_output=True,
            check=False,
        )
        value = result.stdout.strip()
        if result.returncode != 0 or not value or "\n" in value or "\r" in value:
            raise CaseError(
                "git_identity_missing",
                "Attributed investigation writes require effective Git user.name and user.email",
                field=field,
            )
        values[field] = value
    reject_secret_input(values["user.name"], values["user.email"])
    return GitIdentity(values["user.name"], values["user.email"])


def notes_locale(vault_root: Path) -> str:
    instance = vault_root / "instance.yaml"
    try:
        text = instance.read_text(encoding="utf-8")
    except OSError:
        return "en"
    match = re.search(r"(?m)^locale:\s*$\n(?:^[ \t].*\n)*?^  notes:\s*([^#\s]+)", text)
    return match.group(1).casefold() if match else "en"


def record_locale(text: str) -> str:
    return "es" if re.search(r"(?m)^## Historial\s*$", text) else "en"


def append_history_event(text: str, event: str) -> str:
    history = re.search(
        r"(?m)^## (?:" + "|".join(map(re.escape, HISTORY_SECTION_ALIASES)) + r")\s*$",
        text,
    )
    if history is None:
        raise CaseError("history_missing", "Case is missing History")
    position = text.find("\n## ", history.end())
    if position < 0:
        position = len(text)
    suffix = text[position:].lstrip("\n")
    return text[:position].rstrip() + "\n\n" + event.rstrip() + ("\n\n" + suffix if suffix else "\n")


def attributed_event(
    timestamp: str,
    action: str,
    identity: GitIdentity,
    source: str,
    locale: str = "en",
) -> str:
    recorder_label = "registrado por" if locale.startswith("es") else "recorded by"
    source_label = "fuente" if locale.startswith("es") else "source"
    return (
        f"- {timestamp} — {action}; {recorder_label} {identity.display}; "
        f"{source_label}: {source}."
    )


def public_content_errors(case_dir: Path) -> list[str]:
    errors: list[str] = []
    for path in sorted(case_dir.rglob("*")):
        if path.is_symlink():
            errors.append(f"{path.name}: shareable case content must not be a symlink")
            continue
        if not path.is_file():
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        relative = path.relative_to(case_dir).as_posix()
        if SECRET_PATTERN.search(text):
            errors.append(f"{relative}: contains a credential-like value")
        if LOCAL_PATH_PATTERN.search(text):
            errors.append(f"{relative}: contains an absolute local path")
    return errors


def case_tree_snapshot(case_dir: Path) -> tuple[str, list[dict[str, Any]]]:
    """Bind reviewed case bytes, paths, and file and directory permissions."""
    if case_dir.is_symlink() or not case_dir.is_dir():
        raise CaseError("case_tree_invalid", "Case snapshot must be a real directory")
    entries: list[dict[str, Any]] = []
    for path in sorted(case_dir.rglob("*")):
        relative = path.relative_to(case_dir).as_posix()
        if path.is_symlink():
            raise CaseError("case_symlink", "Case snapshot must not contain symlinks", path=relative)
        if path.is_dir():
            entries.append({"path": relative, "type": "directory", "mode": stat.S_IMODE(path.stat().st_mode)})
            continue
        if not path.is_file():
            raise CaseError("case_tree_invalid", "Case snapshot contains a special file", path=relative)
        digest = hashlib.sha256()
        with path.open("rb") as handle:
            for block in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(block)
        entries.append({
            "path": relative,
            "type": "file",
            "mode": stat.S_IMODE(path.stat().st_mode),
            "sha256": digest.hexdigest(),
        })
    encoded = json.dumps(entries, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest(), entries


def unpublished_root(public_root: Path) -> Path:
    return public_root.parent / UNPUBLISHED_ROOT_NAME


def private_store_root(public_root: Path) -> Path:
    return public_root.parent / PRIVATE_ROOT_NAME


def local_working_dir(public_root: Path, case_id: str) -> Path:
    return private_store_root(public_root) / case_id / "local"


def is_legacy_case_text(text: str) -> bool:
    try:
        fields = parse_frontmatter(text)
    except CaseError:
        return True
    if any(not fields.get(field) for field in REQUIRED_FIELDS):
        return True
    headings = {
        line[3:].strip()
        for line in text.splitlines()
        if line.startswith("## ")
    }
    return bool(
        headings.intersection(ORIGINAL_REQUEST_HEADINGS)
        and not headings.intersection(REQUIRED_SECTIONS[0])
    )


def ensure_unpublished_root(public_root: Path) -> Path:
    path = unpublished_root(public_root)
    if path.is_symlink():
        raise CaseError("root_symlink", "The unpublished investigation root must not be a symlink")
    if not path.exists():
        path.mkdir(mode=0o700)
    elif not path.is_dir():
        raise CaseError("invalid_root", "The unpublished investigation root is not a directory")
    return path


def vault_lock_root(public_root: Path) -> Path:
    return ensure_unpublished_root(public_root)


def live_record_items(public_root: Path) -> list[tuple[str, CaseRecord]]:
    items = [("published", record) for record in load_records(public_root)]
    unpublished = unpublished_root(public_root)
    if unpublished.is_dir():
        items.extend(
            ("unpublished", record)
            for record in load_records(unpublished, skip_legacy=True, retirement_root=public_root)
        )
    seen_ids: dict[str, str] = {}
    seen_keys: dict[str, str] = {}
    for visibility, record in items:
        previous = seen_ids.get(record.case_id)
        if previous is not None and previous != visibility:
            raise CaseError(
                "visibility_conflict",
                "An investigation ID cannot exist as both published and unpublished",
                case_id=record.case_id,
            )
        seen_ids[record.case_id] = visibility
        if record.dedupe_key:
            previous_key = seen_keys.get(record.dedupe_key)
            if previous_key is not None and previous_key != record.case_id:
                raise CaseError(
                    "visibility_conflict",
                    "A dedupe key cannot identify both a published and an unpublished case",
                    dedupe_key=record.dedupe_key,
                )
            seen_keys[record.dedupe_key] = record.case_id
    return items


def locate_live_case(public_root: Path, case_id: str) -> LocatedCase:
    matches = [
        (visibility, record)
        for visibility, record in live_record_items(public_root)
        if case_id == record.case_id or case_id in record.consolidated_from
    ]
    if len(matches) != 1:
        raise CaseError(
            "case_not_found" if not matches else "case_ambiguous",
            "Exactly one live investigation must resolve from the supplied ID",
            matches=[record.case_id for _, record in matches],
        )
    visibility, record = matches[0]
    store = public_root if visibility == "published" else unpublished_root(public_root)
    return LocatedCase(record=record, visibility=visibility, store=store)


def locate_legacy_unpublished(public_root: Path, case_id: str) -> Path | None:
    path = unpublished_root(public_root) / case_id / "investigation.md"
    if not path.is_file():
        return None
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        return None
    return path if is_legacy_case_text(text) else None


def resolve_root(raw_root: Path, *, create: bool = False) -> Path:
    if raw_root.is_symlink():
        raise CaseError("root_symlink", "The investigation root must not be a symlink")
    if raw_root.name != "investigations":
        raise CaseError("invalid_root", "The investigation root must be named investigations")
    if create and not raw_root.exists():
        parent = raw_root.parent.resolve(strict=True)
        if not parent.is_dir():
            raise CaseError("root_missing", "The investigation parent does not exist")
        raw_root.mkdir(mode=0o755)
    try:
        root = raw_root.resolve(strict=True)
    except OSError as error:
        raise CaseError("root_missing", "The investigation root does not exist") from error
    if root.name != "investigations" or not root.is_dir():
        raise CaseError(
            "invalid_root",
            "The investigation root must be an existing investigations directory",
            root=str(root),
        )
    return root


def require_child(root: Path, path: Path, *, must_exist: bool = False) -> Path:
    try:
        resolved = path.resolve(strict=must_exist)
    except OSError as error:
        raise CaseError("path_missing", "A required investigation path is missing") from error
    if resolved == root or root not in resolved.parents:
        raise CaseError(
            "path_escape",
            "Investigation paths must remain beneath the configured root",
            path=str(path),
        )
    return resolved


def scalar(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
        if value[0] == '"':
            try:
                parsed = json.loads(value)
                return str(parsed)
            except json.JSONDecodeError:
                pass
        return value[1:-1]
    return value


def parse_frontmatter(text: str) -> dict[str, Any]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        raise CaseError("frontmatter_missing", "investigation.md must start with frontmatter")
    try:
        end = lines.index("---", 1)
    except ValueError as error:
        raise CaseError("frontmatter_invalid", "investigation.md frontmatter is not closed") from error

    fields: dict[str, Any] = {}
    index = 1
    while index < end:
        line = lines[index]
        match = re.match(r"^([a-z][a-z0-9-]*):(?:\s*(.*))?$", line)
        if match is None:
            if not line.strip():
                index += 1
                continue
            raise CaseError(
                "frontmatter_invalid",
                "Frontmatter must use one canonical unquoted key per line",
                line=index + 1,
            )
        key, raw_value = match.group(1), match.group(2) or ""
        if key in fields:
            raise CaseError(
                "frontmatter_invalid",
                f"investigation.md frontmatter repeats {key}",
                field=key,
            )
        if raw_value:
            fields[key] = scalar(raw_value)
            index += 1
            continue
        values: list[str] = []
        index += 1
        while index < end:
            item = re.match(r"^\s{2}-\s+(.+?)\s*$", lines[index])
            if item is None:
                break
            values.append(scalar(item.group(1)))
            index += 1
        fields[key] = values
    return fields


def load_records(
    root: Path,
    *,
    skip_legacy: bool = False,
    retirement_root: Path | None = None,
) -> list[CaseRecord]:
    records: list[CaseRecord] = []
    if not root.exists():
        return records
    for case_dir in sorted(root.iterdir()):
        if case_dir.name.startswith("."):
            continue
        if case_dir.is_symlink():
            raise CaseError(
                "case_symlink",
                "Investigation case directories must not be symlinks",
                path=str(case_dir),
            )
        record_path = case_dir / "investigation.md"
        if not record_path.is_file():
            continue
        require_child(root, record_path, must_exist=True)
        text = record_path.read_text(encoding="utf-8")
        if skip_legacy and is_legacy_case_text(text):
            continue
        records.append(CaseRecord(record_path, text, parse_frontmatter(text)))
    retired = retirement_records(retirement_root or root)
    retired_ids = {value for entry in retired.values()
                   for value in [entry["id"], *entry["consolidated-from"]]}
    for record in records:
        if record.case_id in retired_ids or any(value in retired_ids for value in record.consolidated_from):
            raise CaseError("retirement_conflict", "A retired ID still resolves to a live case")
    return records


RETIREMENT_HEADER = "# Retired investigations\n\n"
RETIREMENT_FIELDS = {
    "id", "title", "snapshot-commit", "public-sha256", "retired-at", "recorded-by",
    "reason", "source", "dependency-review", "absorption-review", "summary",
    "destinations", "consolidated-from",
}


def retirement_records(root: Path) -> dict[str, dict[str, Any]]:
    path = root / "retired.md"
    if path.is_symlink():
        raise CaseError("case_symlink", "Retirement register must not be a symlink")
    if not path.exists():
        return {}
    text = path.read_text(encoding="utf-8")
    if not text.startswith(RETIREMENT_HEADER):
        raise CaseError("retirement_register_invalid", "Invalid retirement register header")
    records: dict[str, dict[str, Any]] = {}
    for line in text[len(RETIREMENT_HEADER):].splitlines():
        try:
            entry = json.loads(line.removeprefix("- "))
        except ValueError as error:
            raise CaseError("retirement_register_invalid", "Invalid retirement record JSON") from error
        if (not line.startswith("- ") or not isinstance(entry, dict)
                or set(entry) != RETIREMENT_FIELDS
                or any(not isinstance(value, str) for key, value in entry.items()
                       if key not in {"destinations", "consolidated-from"})
                or any(not isinstance(entry[key], list) or
                       any(not isinstance(value, str) for value in entry[key])
                       for key in ("destinations", "consolidated-from"))
                or line != retirement_line(entry).rstrip("\n")):
            raise CaseError("retirement_register_invalid", "Invalid retirement record fields")
        validate_event_inputs("retirement_register_invalid", **{
            key: value for key, value in entry.items() if isinstance(value, str)})
        for value in entry["destinations"] + entry["consolidated-from"]:
            validate_event_inputs("retirement_register_invalid", reference=value)
        if (ID_PATTERN.fullmatch(entry["id"]) is None
                or re.fullmatch(r"[a-f0-9]{40}|[a-f0-9]{64}", entry["snapshot-commit"]) is None
                or re.fullmatch(r"[a-f0-9]{64}", entry["public-sha256"]) is None
                or not has_offset_timestamp(entry["retired-at"])
                or entry["id"] in records):
            raise CaseError("retirement_register_invalid", "Invalid or repeated retirement identity")
        if (root / entry["id"]).exists() or (root / entry["id"]).is_symlink():
            raise CaseError("retirement_conflict", "Retired ID still has a public directory")
        records[entry["id"]] = entry
    identities: set[str] = set()
    for entry in records.values():
        for value in [entry["id"], *entry["consolidated-from"]]:
            if ID_PATTERN.fullmatch(value) is None or value in identities:
                raise CaseError("retirement_register_invalid", "Repeated retirement lineage identity")
            if (root / value).exists() or (root / value).is_symlink():
                raise CaseError("retirement_conflict", "Retired lineage has a public directory")
            identities.add(value)
    return records


def retirement_line(entry: dict[str, Any]) -> str:
    return "- " + json.dumps(entry, ensure_ascii=False, sort_keys=True) + "\n"


def retirement_git(root: Path, *arguments: str, required: bool = True) -> bytes:
    result = subprocess.run(
        ["git", "-C", str(root.parent), *arguments], capture_output=True, check=False,
    )
    if required and result.returncode:
        raise CaseError("retirement_git_failed", "Required retirement Git evidence is unavailable")
    return result.stdout if result.returncode == 0 else b""


def retirement_git_path(root: Path, path: Path) -> str:
    top = Path(os.fsdecode(retirement_git(root, "rev-parse", "--show-toplevel")).strip()).resolve()
    try:
        return path.relative_to(top).as_posix()
    except ValueError as error:
        raise CaseError("retirement_git_failed", "Published case is outside its Git repository") from error


def retirement_view(root: Path, entry: dict[str, Any]) -> dict[str, Any]:
    """Derive completion from a reachable commit that added this record and deleted the case."""
    ledger = retirement_git_path(root, root / "retired.md")
    case = retirement_git_path(root, root / entry["id"])
    marker = retirement_line(entry).encode("utf-8")
    retirement_commit = None
    snapshot_tree = retirement_git(root, "rev-parse", f'{entry["snapshot-commit"]}:{case}', required=False)
    for commit in retirement_git(root, "log", "--format=%H", "--", f":(top,literal){ledger}", required=False).decode().splitlines():
        content = retirement_git(root, "show", f"{commit}:{ledger}", required=False)
        parent_content = retirement_git(root, "show", f"{commit}^:{ledger}", required=False)
        parent_tree = retirement_git(root, "rev-parse", f"{commit}^:{case}", required=False)
        if (marker in content.splitlines(keepends=True) and marker not in parent_content.splitlines(keepends=True)
                and snapshot_tree and parent_tree == snapshot_tree
                and not retirement_git(root, "ls-tree", "--full-tree", commit, "--", case)):
            retirement_commit = commit
            break
    return {**entry, "retirement_commit": retirement_commit,
            "commit_state": "committed" if retirement_commit else "pending"}


def list_legacy_unpublished(public_root: Path) -> list[dict[str, str]]:
    entries: list[dict[str, str]] = []
    uroot = unpublished_root(public_root)
    if not uroot.is_dir():
        return entries
    for case_dir in sorted(uroot.iterdir()):
        if case_dir.name.startswith(".") or not case_dir.is_dir():
            continue
        record_path = case_dir / "investigation.md"
        if not record_path.is_file():
            continue
        text = record_path.read_text(encoding="utf-8")
        if is_legacy_case_text(text):
            entries.append({"id": case_dir.name, "path": str(record_path)})
    return entries


def list_cases(root: Path) -> int:
    with locked(root):
        cases = []
        for visibility, record in live_record_items(root):
            cases.append(
                {
                    "id": record.case_id,
                    "title": record.title,
                    "status": record.fields.get("status"),
                    "visibility": visibility,
                    "summary": "\n".join(
                        named_section(record.text, REQUIRED_SECTIONS[0])[0] or []
                    ).strip(),
                    "updated-at": record.fields.get("updated-at"),
                }
            )
        retired = [retirement_view(root, entry) for entry in retirement_records(root).values()]
        legacy = list_legacy_unpublished(root)
    emit({"status": "listed", "cases": cases, "retired": retired, "legacy": legacy})
    return 0


def retire_case(args: argparse.Namespace, root: Path) -> int:
    if not args.authorized:
        raise CaseError("retirement_not_authorized", "Retirement requires explicit --authorized")
    validate_event_inputs("retirement_invalid", reason=args.reason, source=args.source,
                          dependency_review=args.dependency_review,
                          absorption_review=args.absorption_review,
                          recorded_by=args.identity.display, summary=args.summary)
    for destination in args.destination:
        validate_event_inputs("retirement_invalid", destination=destination)
    timestamp = timestamp_value(args.timestamp)
    with locked(root):
        located = locate_live_case(root, args.id)
        record = located.record
        if record.fields.get("status") != "closed":
            raise CaseError("retirement_not_closed", "Only a closed investigation may be retired")
        if digest_bytes(record.path.read_bytes()) != args.expected_public_sha256:
            raise CaseError("stale_public_snapshot", "Investigation changed after review", exit_code=3)
        validate_event_inputs("retirement_invalid", title=record.title)
        case_dir = record.path.parent
        if located.visibility == "unpublished":
            errors, _ = validate_store(located.store, public_root=root, ignore_lock=True)
            if errors:
                raise CaseError("retirement_validation_failed", "Unpublished cases are invalid", errors=errors)
            shutil.rmtree(case_dir)
            emit({"status": "retired", "id": args.id, "visibility": "unpublished"})
            return 0
        if args.snapshot_commit is None or re.fullmatch(
            r"[a-f0-9]{40}|[a-f0-9]{64}", args.snapshot_commit
        ) is None:
            raise CaseError("retirement_snapshot_invalid", "Snapshot must be an exact full Git commit ID")
        errors, _ = validate_store(root, public_root=root, ignore_lock=True)
        if errors:
            raise CaseError("retirement_validation_failed", "Published cases are invalid", errors=errors)
        if any(path.is_symlink() or not (path.is_dir() or path.is_file())
               for path in case_dir.rglob("*")):
            raise CaseError("retirement_snapshot_invalid", "Published case contains non-regular filesystem entries")
        relative = retirement_git_path(root, case_dir)
        commit = retirement_git(root, "rev-parse", "--verify", f"{args.snapshot_commit}^{{commit}}").decode().strip()
        if commit != args.snapshot_commit:
            raise CaseError("retirement_snapshot_invalid", "Snapshot is not a commit")
        retirement_git(root, "merge-base", "--is-ancestor", commit, "HEAD")
        if retirement_git(root, "status", "--porcelain=v1", "--untracked-files=all", "--", f":(top,literal){relative}"):
            raise CaseError("retirement_snapshot_changed", "Case has uncommitted changes")
        tree = retirement_git(root, "ls-tree", "--full-tree", "-rz", commit, "--", relative)
        originals: dict[Path, tuple[bytes, int]] = {}
        for item in tree.split(b"\0"):
            if not item:
                continue
            metadata, name = item.split(b"\t", 1)
            mode, kind, blob = metadata.split()
            if kind != b"blob" or mode not in {b"100644", b"100755"}:
                raise CaseError("retirement_snapshot_invalid", "Snapshot contains a non-regular file")
            suffix = Path(os.fsdecode(name)).relative_to(relative)
            target = case_dir / suffix
            content = retirement_git(root, "cat-file", "blob", blob.decode())
            if (target.is_symlink() or not target.is_file() or target.read_bytes() != content
                    or bool(target.stat().st_mode & 0o111) != (mode == b"100755")):
                raise CaseError("retirement_snapshot_changed", "Snapshot does not match the entire published case")
            originals[suffix] = (content, target.stat().st_mode & 0o777)
        actual = {path.relative_to(case_dir) for path in case_dir.rglob("*") if path.is_file()}
        if not originals or actual != set(originals):
            raise CaseError("retirement_snapshot_changed", "Snapshot omits published files, including ignored files")
        entry = {"id": args.id, "title": record.title, "snapshot-commit": commit,
                 "public-sha256": args.expected_public_sha256, "retired-at": timestamp,
                 "recorded-by": args.identity.display, "reason": args.reason, "source": args.source,
                 "dependency-review": args.dependency_review, "absorption-review": args.absorption_review,
                 "summary": args.summary, "destinations": args.destination,
                 "consolidated-from": record.consolidated_from}
        ledger = root / "retired.md"
        before = ledger.read_bytes() if ledger.exists() else None
        content = (before if before is not None else RETIREMENT_HEADER.encode()) + retirement_line(entry).encode()
        backup = root / f".retire-{uuid.uuid4().hex}.tmp"
        directories = [path.relative_to(case_dir) for path in case_dir.rglob("*") if path.is_dir()]
        os.replace(case_dir, backup)
        try:
            atomic_write(ledger, content)
            shutil.rmtree(backup)
        except Exception:
            if backup.exists():
                os.replace(backup, case_dir)
            else:
                case_dir.mkdir(mode=0o700)
            for directory in directories:
                (case_dir / directory).mkdir(parents=True, exist_ok=True)
            for suffix, (data, mode) in originals.items():
                target = case_dir / suffix
                if not target.exists():
                    atomic_write(target, data)
                    target.chmod(mode)
            if before is None:
                if ledger.exists():
                    ledger.unlink()
            elif not ledger.exists() or ledger.read_bytes() != before:
                atomic_write(ledger, before)
            raise
    emit({"status": "retired", **retirement_view(root, entry)})
    return 0


def acquire_lock(root: Path, timeout_seconds: float = 2.0) -> Path:
    lock = vault_lock_root(root) / ".open.lock"
    deadline = time.monotonic() + timeout_seconds
    while True:
        try:
            os.mkdir(lock, 0o700)
            return lock
        except FileExistsError:
            if time.monotonic() >= deadline:
                raise CaseError(
                    "case_root_locked",
                    "Another investigation mutation holds the open gate",
                    exit_code=3,
                    lock=str(lock),
                )
            time.sleep(0.01)


def locked(root: Path) -> Iterator[None]:
    class LockContext:
        def __enter__(self) -> None:
            self.path = acquire_lock(root)

        def __exit__(self, exc_type: Any, exc: Any, traceback: Any) -> None:
            try:
                self.path.rmdir()
            except OSError as error:
                if exc is None:
                    raise CaseError(
                        "lock_cleanup_failed",
                        "The investigation open gate could not be released",
                        exit_code=4,
                        lock=str(self.path),
                    ) from error

    return LockContext()  # type: ignore[return-value]


def atomic_write(path: Path, content: bytes) -> None:
    temporary = path.with_name(f".{path.name}.tmp-{uuid.uuid4().hex}")
    try:
        descriptor = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def match_records(
    records: list[CaseRecord],
    *,
    case_id: str,
    dedupe_key: str,
    title: str,
    source_refs: list[str],
) -> list[CaseRecord]:
    definite: dict[str, CaseRecord] = {}
    normalized_refs = {
        normalized_source_ref(value)
        for value in source_refs
        if normalized_source_ref(value) not in GENERIC_SOURCE_REFS
    }
    title_key = normalize_text(title)

    for record in records:
        if case_id == record.case_id or case_id in record.consolidated_from:
            definite[record.case_id] = record
        if record.dedupe_key and record.dedupe_key == dedupe_key:
            definite[record.case_id] = record
        record_source = normalized_source_ref(record.source_ref)
        if record_source not in GENERIC_SOURCE_REFS and record_source in normalized_refs:
            definite[record.case_id] = record
        record_title_key = normalize_text(record.title)
        if record_title_key and record_title_key == title_key:
            definite[record.case_id] = record
    return list(definite.values())


def render_case(args: argparse.Namespace, timestamp: str, identity: GitIdentity) -> str:
    template = (
        Path(__file__).resolve().parents[1] / "assets" / "investigation-template.md"
    ).read_text(encoding="utf-8")
    primary_source = args.source_ref[0] if args.source_ref else "direct user request"
    replacements = {
        "id: <investigation-id>": f"id: {args.id}",
        "title: <title>": f"title: {json.dumps(args.title, ensure_ascii=False)}",
        "dedupe-key: <dedupe-key>": f"dedupe-key: {args.dedupe_key}",
        "created-at: <ISO-8601 timestamp>": f"created-at: {timestamp}",
        "updated-at: <ISO-8601 timestamp>": f"updated-at: {timestamp}",
        "source-type: <message|bug|ticket|issue|attachment|other>": (
            f"source-type: {args.source_type}"
        ),
        "source-ref: <source reference>": (
            f"source-ref: {json.dumps(primary_source, ensure_ascii=False)}"
        ),
        "requester-role: <role|unknown>": (
            f"requester-role: {json.dumps(args.requester_role, ensure_ascii=False)}"
        ),
        "export-intent: <technical-stories|user-stories|mixed|undecided>": (
            f"export-intent: {args.export_intent}"
        ),
        "purpose: <knowledge|development|mixed|undecided>": (
            f"purpose: {args.purpose}"
        ),
        (
            "vault-outcome: "
            "<not-evaluated|none|deferred-until-production|candidate-for-audit|documented>"
        ): f"vault-outcome: {args.vault_outcome}",
        (
            "learning-outcome: "
            "<not-evaluated|no-learning|already-covered|insufficient-evidence|"
            "candidate|documented>"
        ): f"learning-outcome: {args.learning_outcome}",
        "# <Title>": f"# {args.title}",
        "<Formalize the relevant request without preserving the conversation transcript.>": args.request_summary,
        "- <timestamp> — Case file created.": attributed_event(
            timestamp,
            "Expediente creado" if args.note_locale.startswith("es") else "Case file created",
            identity,
            primary_source,
            args.note_locale,
        ),
    }
    for old, new in replacements.items():
        template = template.replace(old, new)
    template = template.replace(
        "### Objective\n\n",
        f"### Objective\n\n{args.objective}\n\n",
        1,
    )
    if args.source_ref:
        references = "\n".join(f"- `{value}`" for value in args.source_ref)
        template = template.replace(
            "## References and attachments\n",
            f"## References and attachments\n\n{references}\n",
            1,
        )
    if args.note_locale.startswith("es"):
        headings = {
            "## Request summary": "## Resumen de la solicitud",
            "## Current state": "## Estado actual",
            "### Objective": "### Objetivo",
            "### Scope": "### Alcance",
            "### Out of scope": "### Fuera de alcance",
            "### Current productive state": "### Estado productivo actual",
            "### Future/proposed state": "### Estado futuro o propuesto",
            "## References and attachments": "## Referencias y adjuntos",
            "## Evidence": "## Evidencia",
            "### Facts": "### Hechos",
            "### Inferences": "### Inferencias",
            "### Contradictions": "### Contradicciones",
            "## Affected surfaces": "## Superficies afectadas",
            "## Development handoffs": "## Handoffs de desarrollo",
            "## Open questions": "## Preguntas abiertas",
            "## Decisions": "## Decisiones",
            "## Acceptance criteria": "## Criterios de aceptación",
            "## Readiness": "## Preparación",
            "## History": "## Historial",
        }
        for original, translated in headings.items():
            template = template.replace(original, translated)
    return template


def timestamp_value(raw: str | None) -> str:
    if raw is not None:
        try:
            parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
        except ValueError as error:
            raise CaseError("invalid_timestamp", "Timestamp must be ISO-8601") from error
        if parsed.tzinfo is None or parsed.utcoffset() is None:
            raise CaseError(
                "invalid_timestamp",
                "Timestamp must be ISO-8601 with a UTC offset",
            )
        return raw
    return datetime.now().astimezone().isoformat(timespec="seconds")


def open_case(args: argparse.Namespace, root: Path) -> int:
    if ID_PATTERN.fullmatch(args.id) is None:
        raise CaseError("invalid_id", "Investigation ID is not canonical", case_id=args.id)
    if KEY_PATTERN.fullmatch(args.dedupe_key) is None:
        raise CaseError(
            "invalid_dedupe_key",
            "Dedupe key must use lowercase ASCII hyphenated words",
        )
    reject_secret_input(
        args.title,
        args.objective,
        args.request_summary,
        *args.source_ref,
    )
    timestamp = timestamp_value(args.timestamp)
    visibility = getattr(args, "visibility", "unpublished") or "unpublished"
    if visibility not in {"unpublished", "published"}:
        raise CaseError("invalid_visibility", "Visibility must be unpublished or published")
    store = root if visibility == "published" else ensure_unpublished_root(root)

    with locked(root):
        live = live_record_items(root)
        records = [record for _, record in live]
        visibility_by_id = {record.case_id: vis for vis, record in live}
        if any(args.id == entry["id"] or args.id in entry["consolidated-from"]
               for entry in retirement_records(root).values()):
            raise CaseError("case_retired", "A retired investigation ID cannot be reused")
        if locate_legacy_unpublished(root, args.id) is not None:
            raise CaseError(
                "legacy_unpublished",
                "A legacy unpublished investigation with this ID requires Migrate",
                case_id=args.id,
            )
        definite = match_records(
            records,
            case_id=args.id,
            dedupe_key=args.dedupe_key,
            title=args.title,
            source_refs=args.source_ref,
        )
        if definite:
            emit(
                {
                    "status": "definite_match",
                    "cases": [
                        {
                            "id": record.case_id,
                            "path": str(record.path.parent),
                            "visibility": visibility_by_id.get(record.case_id),
                        }
                        for record in sorted(definite, key=lambda item: item.case_id)
                    ],
                }
            )
            return 0
        target = require_child(store, store / args.id)
        if target.exists():
            raise CaseError("case_exists", "The investigation directory already exists")
        staging = store / f".open-{uuid.uuid4().hex}.tmp"
        require_child(store, staging)
        try:
            staging.mkdir(mode=0o700)
            (staging / "artifacts").mkdir(mode=0o700)
            (staging / "exports").mkdir(mode=0o700)
            (staging / "handoffs").mkdir(mode=0o700)
            atomic_write(
                staging / "investigation.md",
                render_case(args, timestamp, args.identity).encode("utf-8"),
            )
            os.replace(staging, target)
        finally:
            if staging.exists():
                shutil.rmtree(staging)

    emit({"status": "created", "id": args.id, "path": str(target), "visibility": visibility})
    return 0


def load_case(args: argparse.Namespace, root: Path) -> int:
    with locked(root):
        return load_case_unlocked(args, root)


def load_case_unlocked(args: argparse.Namespace, root: Path) -> int:
    for entry in retirement_records(root).values():
        if args.id == entry["id"] or args.id in entry["consolidated-from"]:
            emit({"status": "retired", **retirement_view(root, entry)})
            return 0
    legacy = locate_legacy_unpublished(root, args.id)
    if legacy is not None and not (root / args.id / "investigation.md").is_file():
        emit(
            {
                "status": "legacy",
                "id": args.id,
                "path": str(legacy),
                "visibility": "unpublished",
            }
        )
        return 0
    located = locate_live_case(root, args.id)
    record = located.record
    public_errors = public_content_errors(record.path.parent)
    if public_errors:
        raise CaseError(
            "public_validation_failed",
            "Investigation is unsafe to load",
            errors=public_errors,
        )
    private_path = private_store_root(root) / record.case_id / "private.md"
    if private_path.is_symlink() or private_path.parent.is_symlink():
        raise CaseError("case_symlink", "Private overlay paths must not be symlinks")
    private_exists = private_path.is_file()
    private_bytes = private_path.read_bytes() if private_exists else None
    if private_bytes is not None:
        try:
            private_text = private_bytes.decode("utf-8")
        except UnicodeDecodeError as error:
            raise CaseError("private_validation_failed", "Private overlay must be UTF-8") from error
        private_errors = validate_private_text(private_text, record.case_id)
        if SECRET_PATTERN.search(private_text):
            private_errors.append("private overlay contains a credential-like value")
        if private_errors:
            raise CaseError(
                "private_validation_failed",
                "Private overlay is unsafe or invalid",
                errors=private_errors,
            )
    local_path = local_working_dir(root, record.case_id)
    local_available = local_path.is_dir() and any(local_path.rglob("*"))
    tree_sha256, _ = case_tree_snapshot(record.path.parent)
    emit(
        {
            "status": "loaded",
            "id": record.case_id,
            "visibility": located.visibility,
            "public": {
                "path": str(record.path),
                "sha256": digest_bytes(record.path.read_bytes()),
                "tree_sha256": tree_sha256,
            },
            "private": {
                "available": private_exists,
                "path": str(private_path) if private_exists else None,
                "sha256": digest_bytes(private_bytes),
            },
            "local": {
                "available": local_available,
                "path": str(local_path) if local_available else None,
            },
        }
    )
    return 0


def snapshot_case(args: argparse.Namespace) -> int:
    digest, entries = case_tree_snapshot(args.case_dir)
    emit({"status": "snapshotted", "tree_sha256": digest, "files": entries})
    return 0


def section_names(text: str) -> list[str]:
    return [
        line[3:].strip()
        for line in text.splitlines()
        if line.startswith("## ")
    ]


def subsection_names(
    text: str,
    *,
    parent_aliases: tuple[str, ...] | None = None,
) -> list[str]:
    parent: str | None = None
    names: list[str] = []
    for line in text.splitlines():
        if line.startswith("## "):
            parent = line[3:].strip()
        elif line.startswith("### ") and (
            parent_aliases is None or parent in parent_aliases
        ):
            names.append(line[4:].strip())
    return names


def canonical_tracker_url(value: str) -> str | None:
    parsed = urlparse(value)
    if (
        parsed.scheme.casefold() != "https"
        or not parsed.hostname
        or parsed.username
        or parsed.password
        or parsed.query
        or parsed.fragment
    ):
        return None
    host = parsed.hostname.casefold()
    try:
        port = parsed.port
    except ValueError:
        return None
    if port:
        host = f"{host}:{port}"
    return f"https://{host}{parsed.path.rstrip('/')}"


def canonical_slug(value: str) -> str:
    return "-".join(re.findall(r"[a-z0-9]+", value.casefold()))


def valid_branch(value: str) -> bool:
    if not value or value != value.strip() or len(value) > 255:
        return False
    result = subprocess.run(
        ["git", "check-ref-format", "--branch", value],
        text=True,
        capture_output=True,
        check=False,
    )
    return result.returncode == 0


def named_section(
    text: str,
    aliases: tuple[str, ...],
) -> tuple[list[str] | None, int]:
    lines = text.splitlines()
    starts = [
        index
        for index, line in enumerate(lines)
        if line.startswith("## ")
        and line[3:].strip() in aliases
    ]
    if len(starts) != 1:
        return None, len(starts)
    start = starts[0] + 1
    end = next(
        (
            index
            for index in range(start, len(lines))
            if lines[index].startswith("## ")
        ),
        len(lines),
    )
    return lines[start:end], 1


def development_handoff_section(text: str) -> tuple[list[str] | None, int]:
    return named_section(text, DEVELOPMENT_HANDOFF_SECTION_ALIASES)


def offset_timestamp(value: str) -> datetime | None:
    try:
        observed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if observed.tzinfo is None or observed.utcoffset() is None:
        return None
    return observed


def has_offset_timestamp(value: str) -> bool:
    return offset_timestamp(value) is not None


def handoff_history_binding(
    entry_id: str,
    fields: dict[str, str],
) -> dict[str, str]:
    return {
        "dh": entry_id,
        "story-id": fields["Story ID"],
        "tracker-id": fields["Tracker ID"],
        "provider": fields["Provider"],
        "tracker-url": fields["Tracker URL"],
        "work-item-reference": fields["Work item reference"],
        "repository-remote": fields["Repository remote"],
        "branch": fields["Branch"],
        "handoff-id": fields["Handoff ID"],
        "family": fields["Family"],
        "revision": fields["Revision"],
        "materialized-at": fields["Materialized at"],
    }


def validate_development_handoff_history(
    text: str,
    entries: list[tuple[str, int, str, dict[str, str]]],
) -> list[str]:
    lines, section_count = named_section(text, HISTORY_SECTION_ALIASES)
    if section_count == 0:
        if DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN in text:
            return [
                "Development handoff binding markers must appear only in History"
            ]
        return []
    if section_count != 1 or lines is None:
        return ["History section must appear exactly once"]

    errors: list[str] = []
    marker_occurrences = text.count(DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN)
    history_marker_occurrences = sum(
        line.count(DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN) for line in lines
    )
    if marker_occurrences != history_marker_occurrences:
        errors.append(
            "Development handoff binding markers must appear only in History"
        )
    markers: list[tuple[dict[str, str], str | None]] = []
    current_event: str | None = None
    current_event_has_binding = False
    for line in lines:
        if line.startswith("- "):
            current_event = line
            current_event_has_binding = False
            continue
        if DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN not in line:
            if line and not line[0].isspace():
                current_event = None
                current_event_has_binding = False
            continue
        event = current_event
        if event is None:
            errors.append(
                "Development handoff binding must belong to one human-readable "
                "History event"
            )
        elif current_event_has_binding:
            errors.append(
                "A History event must contain at most one development handoff binding"
            )
        else:
            current_event_has_binding = True
        match = DEVELOPMENT_HANDOFF_HISTORY_MARKER_RE.fullmatch(line)
        if match is None:
            errors.append("History contains an invalid development handoff binding marker")
            continue
        duplicate_keys: set[str] = set()

        def history_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
            parsed: dict[str, Any] = {}
            for key, item in pairs:
                if key in parsed:
                    duplicate_keys.add(key)
                parsed[key] = item
            return parsed

        raw_marker = match.group(1)
        try:
            value = json.loads(
                raw_marker,
                object_pairs_hook=history_object,
            )
        except json.JSONDecodeError:
            errors.append("History contains invalid development handoff binding JSON")
            continue
        if duplicate_keys:
            errors.append(
                "History development handoff binding contains duplicate keys: "
                + ", ".join(sorted(duplicate_keys))
            )
            continue
        if (
            not isinstance(value, dict)
            or tuple(sorted(value)) != tuple(sorted(DEVELOPMENT_HANDOFF_HISTORY_KEYS))
            or any(not isinstance(item, str) or not item for item in value.values())
        ):
            errors.append("History development handoff binding has invalid fields")
            continue
        canonical_marker = json.dumps(
            value,
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )
        if raw_marker != canonical_marker:
            errors.append(
                "History development handoff binding must use canonical compact JSON"
            )
        markers.append((value, event))

    entry_fields = {
        entry_id: fields
        for entry_id, _, _, fields in entries
        if all(field in fields for field in DEVELOPMENT_HANDOFF_FIELDS)
    }
    revisions: dict[str, list[int]] = {entry_id: [] for entry_id in entry_fields}
    materialized_times: dict[str, list[datetime]] = {
        entry_id: [] for entry_id in entry_fields
    }
    current_matches: dict[str, int] = {entry_id: 0 for entry_id in entry_fields}
    for marker, event in markers:
        entry_id = marker["dh"]
        fields = entry_fields.get(entry_id)
        if fields is None:
            errors.append(f"History binding references unknown {entry_id}")
            continue
        stable_pairs = (
            ("story-id", "Story ID"),
            ("tracker-id", "Tracker ID"),
            ("provider", "Provider"),
            ("tracker-url", "Tracker URL"),
            ("work-item-reference", "Work item reference"),
            ("repository-remote", "Repository remote"),
            ("handoff-id", "Handoff ID"),
            ("family", "Family"),
        )
        if any(marker[key] != fields[field] for key, field in stable_pairs):
            errors.append(f"History binding for {entry_id} changes its stable identity")
            continue
        revision_match = HANDOFF_REVISION_RE.fullmatch(marker["revision"])
        current_match = HANDOFF_REVISION_RE.fullmatch(fields["Revision"])
        if revision_match is None or current_match is None:
            errors.append(f"History binding for {entry_id} has invalid Revision")
            continue
        revision_number = int(revision_match.group(0)[1:])
        current_number = int(current_match.group(0)[1:])
        if revision_number < 1 or revision_number > current_number:
            errors.append(f"History binding for {entry_id} exceeds its current Revision")
            continue
        branch = marker["branch"]
        if not valid_branch(branch):
            errors.append(f"History binding for {entry_id} has invalid Branch")
            continue
        materialized_at = offset_timestamp(marker["materialized-at"])
        if materialized_at is None:
            errors.append(f"History binding for {entry_id} has invalid Materialized at")
            continue
        if event is not None:
            event_prefix = f"- {marker['materialized-at']} — "
            target_fragment = f"development handoff `{entry_id}`"
            if not event.startswith(event_prefix):
                errors.append(
                    f"History event for {entry_id} must start with its Materialized at"
                )
            else:
                action, separator, _ = event[len(event_prefix) :].partition(
                    f" {target_fragment};"
                )
                if not separator or not any(character.isalnum() for character in action):
                    errors.append(
                        f"History event for {entry_id} must name an action before "
                        "its development handoff target"
                    )
            visible_fragments = (
                (entry_id, target_fragment),
                (marker["story-id"], f"story `{marker['story-id']}`"),
                (
                    marker["work-item-reference"],
                    f"work item `{marker['tracker-id']}:{marker['work-item-reference']}`",
                ),
                (
                    marker["repository-remote"],
                    f"repository `{marker['repository-remote']}`",
                ),
                (
                    marker["branch"],
                    f"branch `{marker['branch']}`",
                ),
                (marker["handoff-id"], f"handoff `{marker['handoff-id']}`"),
                (marker["revision"], f"revision `{marker['revision']}`"),
            )
            missing_values = [
                value
                for value, fragment in visible_fragments
                if fragment not in event
            ]
            if missing_values:
                errors.append(
                    f"History event for {entry_id} does not name its exact binding target: "
                    + ", ".join(missing_values)
                )
        revisions[entry_id].append(revision_number)
        materialized_times[entry_id].append(materialized_at)
        if marker == handoff_history_binding(entry_id, fields):
            current_matches[entry_id] += 1

    for entry_id, fields in entry_fields.items():
        current_match = HANDOFF_REVISION_RE.fullmatch(fields["Revision"])
        if current_match is None:
            continue
        current_number = int(current_match.group(0)[1:])
        expected_revisions = list(range(1, current_number + 1))
        if sorted(revisions[entry_id]) != expected_revisions:
            errors.append(
                f"History bindings for {entry_id} must cover each Revision from v0001"
            )
        elif revisions[entry_id] != expected_revisions:
            errors.append(
                f"History bindings for {entry_id} must appear in Revision order from v0001"
            )
        elif any(
            current < previous
            for previous, current in zip(
                materialized_times[entry_id],
                materialized_times[entry_id][1:],
            )
        ):
            errors.append(
                f"History bindings for {entry_id} Materialized at timestamps must "
                "preserve Revision chronology"
            )
        if current_matches[entry_id] != 1:
            errors.append(
                f"History must contain exactly one current binding for {entry_id}"
            )
    return errors


def validate_development_handoffs(text: str) -> list[str]:
    lines, section_count = development_handoff_section(text)
    if section_count == 0:
        return []
    if section_count != 1 or lines is None:
        return ["Development handoffs section must appear exactly once"]

    errors: list[str] = []
    entries: list[tuple[str, int, str, dict[str, str]]] = []
    index = 0
    while index < len(lines):
        if not lines[index].strip():
            index += 1
            continue
        heading = DEVELOPMENT_HANDOFF_HEADING_RE.fullmatch(lines[index])
        if heading is None:
            errors.append(
                "Development handoffs contains content outside an exact DH-NNN entry"
            )
            break
        entry_id, number, title = heading.groups()
        index += 1
        fields: dict[str, str] = {}
        field_order: list[str] = []
        while index < len(lines) and not lines[index].startswith("### "):
            line = lines[index]
            index += 1
            if not line.strip():
                continue
            field_match = DEVELOPMENT_HANDOFF_FIELD_RE.fullmatch(line)
            if field_match is None:
                errors.append(f"{entry_id} contains invalid content")
                continue
            field, value = field_match.groups()
            if field not in DEVELOPMENT_HANDOFF_FIELDS:
                errors.append(f"{entry_id} contains unknown field {field}")
                continue
            if field in fields:
                errors.append(f"{entry_id} repeats field {field}")
                continue
            fields[field] = value.strip()
            field_order.append(field)
        missing = [field for field in DEVELOPMENT_HANDOFF_FIELDS if not fields.get(field)]
        if missing:
            errors.append(f"{entry_id} is missing fields {', '.join(missing)}")
        if not missing and tuple(field_order) != DEVELOPMENT_HANDOFF_FIELDS:
            errors.append(f"{entry_id} fields are out of order")
        entries.append((entry_id, int(number), title.strip(), fields))

    observed_ids = [entry_id for entry_id, _, _, _ in entries]
    if len(set(observed_ids)) != len(observed_ids):
        errors.append("Development handoffs contains duplicate DH-NNN identifiers")
    observed_numbers = [number for _, number, _, _ in entries]
    if observed_numbers != list(range(1, len(entries) + 1)):
        errors.append("Development handoff identifiers must be contiguous from DH-001")

    targets: set[tuple[str, str]] = set()
    branches: dict[str, str] = {}
    handoff_ids: set[str] = set()
    for entry_id, _, title, fields in entries:
        if any(field not in fields for field in DEVELOPMENT_HANDOFF_FIELDS):
            continue
        story_id = fields["Story ID"]
        tracker_id = fields["Tracker ID"]
        provider = fields["Provider"]
        tracker_url = fields["Tracker URL"]
        work_item_reference = fields["Work item reference"]
        remote = fields["Repository remote"]
        branch = fields["Branch"]
        handoff_id = fields["Handoff ID"]
        family = fields["Family"]
        revision = fields["Revision"]
        materialized_at = fields["Materialized at"]

        if STORY_ID_RE.fullmatch(story_id) is None:
            errors.append(f"{entry_id} has invalid Story ID")
        canonical_url = canonical_tracker_url(tracker_url)
        if canonical_url is None or tracker_url != canonical_url:
            errors.append(f"{entry_id} has invalid canonical Tracker URL")
        if TRACKER_NAME_RE.fullmatch(tracker_id) is None:
            errors.append(f"{entry_id} has invalid Tracker ID")
        if TRACKER_NAME_RE.fullmatch(provider) is None:
            errors.append(f"{entry_id} has invalid Provider")
        if not work_item_reference or len(work_item_reference) > 200:
            errors.append(f"{entry_id} has invalid Work item reference")
        remote_segments = remote.split("/")
        if (
            remote != remote.casefold()
            or "://" in remote
            or "\\" in remote
            or remote.endswith(".git")
            or len(remote_segments) < 2
            or any(not segment or segment in {".", ".."} for segment in remote_segments)
            or any(character.isspace() for character in remote)
        ):
            errors.append(f"{entry_id} has invalid normalized Repository remote")
        if not valid_branch(branch):
            errors.append(f"{entry_id} has invalid Branch")
        if HANDOFF_ID_RE.fullmatch(handoff_id) is None:
            errors.append(f"{entry_id} has invalid Handoff ID")
        if HANDOFF_FAMILY_RE.fullmatch(family) is None:
            errors.append(f"{entry_id} has invalid Family")
        if HANDOFF_REVISION_RE.fullmatch(revision) is None:
            errors.append(f"{entry_id} has invalid Revision")
        if not has_offset_timestamp(materialized_at):
            errors.append(f"{entry_id} has invalid Materialized at timestamp")

        repository_name = remote.rsplit("/", 1)[-1]
        expected_title = f"{tracker_id}:{work_item_reference} / {repository_name}"
        if title != expected_title:
            errors.append(f"{entry_id} heading does not match work item and repository")
        expected_handoff_id = hashlib.sha256(
            "|".join((tracker_id, work_item_reference, remote)).encode("utf-8")
        ).hexdigest()
        readable = canonical_slug(f"{tracker_id}-{work_item_reference}")[:80].rstrip("-")
        token = f"{readable}-{expected_handoff_id[:10]}"
        expected_family = f"{token}--{canonical_slug(repository_name)}"
        if family != expected_family:
            errors.append(f"{entry_id} Family does not match work item and repository")
        if handoff_id != expected_handoff_id:
            errors.append(f"{entry_id} Handoff ID does not match its identity")

        target = (story_id, remote)
        if target in targets:
            errors.append(f"{entry_id} duplicates a story-and-repository target")
        targets.add(target)
        registered_remote = branches.get(branch)
        if registered_remote is not None and registered_remote != remote:
            errors.append(
                f"{entry_id} reuses a Branch for a different Repository remote"
            )
        branches.setdefault(branch, remote)
        if handoff_id in handoff_ids:
            errors.append(f"{entry_id} reuses a Handoff ID")
        handoff_ids.add(handoff_id)
    errors.extend(validate_development_handoff_history(text, entries))
    return errors


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def validate_archive(archive: Path) -> list[str]:
    errors: list[str] = []
    manifest_path = archive / "manifest.json"
    if not manifest_path.is_file():
        return [f"{archive}: missing manifest.json"]
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError):
        return [f"{manifest_path}: invalid manifest"]
    files = manifest.get("files")
    if not isinstance(files, list):
        return [f"{manifest_path}: files must be a list"]
    for item in files:
        if not isinstance(item, dict) or set(item) != {"path", "sha256"}:
            errors.append(f"{manifest_path}: invalid file entry")
            continue
        candidate = archive / str(item["path"])
        try:
            resolved = candidate.resolve(strict=True)
        except OSError:
            errors.append(f"{manifest_path}: missing archived file {item['path']}")
            continue
        if archive not in resolved.parents or not resolved.is_file():
            errors.append(f"{manifest_path}: archived path escapes {item['path']}")
        elif sha256(resolved) != item["sha256"]:
            errors.append(f"{manifest_path}: hash mismatch {item['path']}")
    return errors


def validate_root(
    root: Path,
    *,
    ignore_lock: bool = False,
    skip_legacy: bool = False,
    retirement_root: Path | None = None,
    lock_root: Path | None = None,
) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []
    if not root.exists():
        return errors, warnings
    lock_dir = vault_lock_root(lock_root or root) if (lock_root or root).name == "investigations" else root
    if not ignore_lock and (lock_dir / ".open.lock").exists():
        errors.append("live mutation lock exists")
    for path in root.iterdir():
        if path.name.startswith((".open-", ".retire-", ".publish-")) and path.name.endswith(".tmp"):
            errors.append(f"incomplete staging directory: {path.name}")

    records = load_records(root, skip_legacy=skip_legacy, retirement_root=retirement_root)
    ids: dict[str, Path] = {}
    keys: dict[str, Path] = {}
    lineage: dict[str, Path] = {}
    active_ids = {record.case_id for record in records}
    for record in records:
        relative = record.path.relative_to(root)
        for field in REQUIRED_FIELDS:
            if not record.fields.get(field):
                errors.append(f"{relative}: missing {field}")
        for field in ("created-at", "updated-at"):
            value = str(record.fields.get(field, ""))
            if value and not has_offset_timestamp(value):
                errors.append(f"{relative}: {field} must be an ISO-8601 timestamp with offset")
        for line in record.text.splitlines():
            if "; recorded by " not in line and "; registrado por " not in line:
                continue
            timestamp, separator, _ = line.removeprefix("- ").partition(" — ")
            if not separator or not has_offset_timestamp(timestamp):
                errors.append(
                    f"{relative}: attributed History events require an ISO-8601 timestamp with offset"
                )
        if record.case_id in ids:
            errors.append(f"duplicate id: {record.case_id}")
        ids[record.case_id] = record.path
        if record.path.parent.name != record.case_id:
            errors.append(f"{relative}: directory does not match id")
        if ID_PATTERN.fullmatch(record.case_id) is None:
            errors.append(f"{relative}: invalid id")

        if record.dedupe_key:
            if KEY_PATTERN.fullmatch(record.dedupe_key) is None:
                errors.append(f"{relative}: invalid dedupe-key")
            elif record.dedupe_key in keys:
                errors.append(f"duplicate dedupe-key: {record.dedupe_key}")
            keys[record.dedupe_key] = record.path

        purpose = str(record.fields.get("purpose", ""))
        if purpose and purpose not in PURPOSES:
            errors.append(f"{relative}: invalid purpose")

        vault_outcome = str(record.fields.get("vault-outcome", ""))
        if vault_outcome and vault_outcome not in VAULT_OUTCOMES:
            errors.append(f"{relative}: invalid vault-outcome")

        learning_outcome = str(record.fields.get("learning-outcome", ""))
        if learning_outcome and learning_outcome not in LEARNING_OUTCOMES:
            errors.append(f"{relative}: invalid learning-outcome")

        status = str(record.fields.get("status", ""))
        if status and status not in STATUSES:
            errors.append(f"{relative}: invalid status")
        blocked_on = str(record.fields.get("blocked-on", ""))
        closure_outcome = str(record.fields.get("closure-outcome", ""))
        if "resume-to" in record.fields:
            errors.append(f"{relative}: resume-to is legacy lifecycle metadata")
        if status == "blocked" and not blocked_on:
            errors.append(f"{relative}: blocked status requires blocked-on")
        if status != "blocked" and blocked_on:
            errors.append(f"{relative}: blocked-on is allowed only while blocked")
        if status == "closed" and closure_outcome not in CLOSURE_OUTCOMES:
            errors.append(f"{relative}: closed status requires a valid closure-outcome")
        if status != "closed" and closure_outcome:
            errors.append(f"{relative}: closure-outcome is allowed only while closed")
        if (
            purpose == "undecided"
            and status == "closed"
            and closure_outcome == "completed"
        ):
            errors.append(f"{relative}: a completed case requires a resolved purpose")
        if vault_outcome == "not-evaluated" and (
            status == "closed" and closure_outcome == "completed"
        ):
            errors.append(
                f"{relative}: a completed case requires an evaluated vault-outcome"
            )

        if purpose in {"development", "mixed"}:
            observed_subsections = subsection_names(
                record.text,
                parent_aliases=REQUIRED_SECTIONS[1],
            )
            missing_current_state = not any(
                alias in observed_subsections
                for alias in CURRENT_PRODUCTIVE_STATE_ALIASES
            )
            missing_future_state = not any(
                alias in observed_subsections
                for alias in FUTURE_PROPOSED_STATE_ALIASES
            )
            state_messages = []
            if missing_current_state:
                state_messages.append("missing Current productive state subsection")
            if missing_future_state:
                state_messages.append("missing Future/proposed state subsection")
            for message in state_messages:
                errors.append(f"{relative}: {message}")

        observed_sections = section_names(record.text)
        required_positions: list[int] = []
        missing_sections: list[str] = []
        for aliases in REQUIRED_SECTIONS:
            positions = [
                observed_sections.index(alias)
                for alias in aliases
                if alias in observed_sections
            ]
            if positions:
                required_positions.append(min(positions))
            else:
                missing_sections.append(aliases[0])
        if missing_sections:
            errors.append(f"{relative}: missing sections {', '.join(missing_sections)}")
        elif required_positions != sorted(required_positions):
            errors.append(f"{relative}: required sections are out of order")

        errors.extend(
            f"{relative}: {message}"
            for message in validate_development_handoffs(record.text)
        )
        errors.extend(
            f"{relative}: {message}"
            for message in public_content_errors(record.path.parent)
        )
        if record.fields.get("artifact-integrity") not in (None, "sha256-v1"):
            errors.append(f"{relative}: invalid artifact-integrity")
        try:
            entries = case_tree_snapshot(record.path.parent)[1]
        except CaseError as error:
            errors.append(f"{relative}: {error.message}")
        else:
            errors.extend(
                f"{relative}: {message}"
                for message in artifact_integrity_errors(
                    entries, record.text,
                    strict=record.fields.get("artifact-integrity") == "sha256-v1",
                )
            )

        for retired_id in record.consolidated_from:
            if retired_id in lineage:
                errors.append(f"duplicate consolidated-from id: {retired_id}")
            lineage[retired_id] = record.path
            if retired_id in active_ids:
                errors.append(f"consolidated-from id still has an active directory: {retired_id}")

        archive_root = record.path.parent / "artifacts" / "consolidated"
        if archive_root.is_dir():
            for archive in sorted(archive_root.iterdir()):
                if archive.is_dir() and not archive.name.startswith("."):
                    errors.extend(validate_archive(archive))
    return errors, warnings


def validate_store(
    store: Path,
    *,
    public_root: Path,
    ignore_lock: bool = False,
) -> tuple[list[str], list[str]]:
    skip_legacy = store.name == UNPUBLISHED_ROOT_NAME
    return validate_root(
        store,
        ignore_lock=ignore_lock,
        skip_legacy=skip_legacy,
        retirement_root=public_root,
        lock_root=public_root,
    )


def validate_vault(public_root: Path, *, ignore_lock: bool = False) -> tuple[list[str], list[str]]:
    errors, warnings = validate_store(public_root, public_root=public_root, ignore_lock=ignore_lock)
    unpublished = unpublished_root(public_root)
    extra_errors, extra_warnings = validate_store(
        unpublished, public_root=public_root, ignore_lock=True
    )
    errors.extend(extra_errors)
    warnings.extend(extra_warnings)
    try:
        live_record_items(public_root)
    except CaseError as error:
        errors.append(error.message)
    return errors, warnings


def validate_command(root: Path) -> int:
    errors, warnings = validate_vault(root)
    if errors:
        emit(
            {"status": "invalid", "errors": errors, "warnings": warnings},
            stream=sys.stderr,
        )
        return 2
    emit(
        {
            "status": "valid",
            "cases": len(live_record_items(root)),
            "warnings": warnings,
        }
    )
    return 0


def digest_bytes(content: bytes | None) -> str:
    if content is None:
        return "absent"
    return hashlib.sha256(content).hexdigest()


def validate_private_text(text: str, case_id: str) -> list[str]:
    errors: list[str] = []
    try:
        fields = parse_frontmatter(text)
    except CaseError as error:
        return [error.message]
    if set(fields) != {"id", "authority", "updated-at"}:
        errors.append("private overlay frontmatter must contain only id, authority, and updated-at")
    if fields.get("id") != case_id:
        errors.append("private overlay id must match the investigation")
    if fields.get("authority") != "private-overlay":
        errors.append("private overlay authority must be private-overlay")
    if not has_offset_timestamp(str(fields.get("updated-at", ""))):
        errors.append("private overlay updated-at must be an ISO-8601 timestamp with offset")
    observed = section_names(text)
    required_aliases = (
        ("Sensitive context", "Contexto sensible"),
        ("Private references", "Referencias privadas"),
        ("History", "Historial"),
    )
    if len(observed) != len(required_aliases) or any(
        observed[index] not in aliases
        for index, aliases in enumerate(required_aliases)
    ):
        errors.append("private overlay must contain the required sections once and in order")
    for line in text.splitlines():
        if "; recorded by " not in line and "; registrado por " not in line:
            continue
        timestamp, separator, _ = line.removeprefix("- ").partition(" — ")
        if not separator or not has_offset_timestamp(timestamp):
            errors.append(
                "private attributed History events require an ISO-8601 timestamp with offset"
            )
    return errors


def declares_register(text: str, register_id: str) -> bool:
    """Require a declaration in the register's authoritative case section."""
    prefix = register_id.rsplit("-", 1)[0]
    aliases = REGISTER_SECTION_ALIASES.get(prefix)
    if aliases is None:
        return False
    lines, count = named_section(text, aliases)
    if count != 1 or lines is None:
        return False
    return re.search(
        rf"(?m)^(?:-\s+|###\s+)`?{re.escape(register_id)}`?(?=$|[\s:—(])",
        "\n".join(lines),
    ) is not None


def artifact_unit_id(relative: str) -> str | None:
    parts = Path(relative).parts
    if len(parts) < 2 or parts[0] != "artifacts":
        return None
    match = re.match(r"^(A-[0-9]{3,})(?=$|[-.])", parts[1])
    return match.group(1) if match else None


def artifact_entry(text: str, register_id: str) -> str:
    lines, count = named_section(text, REGISTER_SECTION_ALIASES["A"])
    if count != 1 or lines is None:
        return ""
    header = re.compile(r"^(?:-\s+|###\s+)`?(A-[0-9]{3,})`?(?=$|[\s:—(])")
    start = next((i for i, line in enumerate(lines)
                  if (match := header.match(line)) and match.group(1) == register_id), None)
    if start is None:
        return ""
    end = next((i for i in range(start + 1, len(lines)) if header.match(lines[i])), len(lines))
    return "\n".join(lines[start:end])


def artifact_integrity_errors(entries: list[dict[str, Any]], text: str,
                              targets: set[str] | None = None,
                              *, strict: bool = True) -> list[str]:
    """Check every selected case artifact against its register's file inventory."""
    errors: list[str] = []
    for item in entries:
        if item.get("type") != "file":
            continue
        relative = str(item["path"])
        unit = artifact_unit_id(relative)
        if not relative.startswith("artifacts/"):
            continue
        if unit is None:
            if strict and targets is None:
                errors.append(f"{relative}: artifact is outside an A-NNN unit")
            continue
        if targets is not None and unit not in targets:
            continue
        entry = artifact_entry(text, unit)
        if not strict and not any(
            re.search(re.escape(relative) + r".{0,24}SHA-256\s+`?[a-fA-F0-9]{64}", line)
            for line in entry.splitlines()
        ):
            continue  # Historical units without a per-file inventory remain legacy debt.
        if not entry:
            errors.append(f"{relative}: missing {unit} register entry")
            continue
        if not any(relative in line and item["sha256"] in line for line in entry.splitlines()):
            errors.append(f"{relative}: {unit} must record path and SHA-256 on one line")
    return errors


def retained_link_errors(case_dir: Path, removed: set[str]) -> list[str]:
    """Reject local links in published documents that point at retained files."""
    errors: list[str] = []
    markdown = re.compile(r"(?<!!)\[[^\]]*\]\(([^)]+)\)|!\[[^\]]*\]\(([^)]+)\)")
    html = re.compile(r"\b(?:href|src)\s*=\s*['\"]([^'\"]+)['\"]", re.I)
    for document in case_dir.rglob("*"):
        if not document.is_file() or document.suffix.lower() not in {".md", ".html", ".htm"}:
            continue
        relative_document = document.relative_to(case_dir)
        try:
            content = document.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        links = [next(group for group in match.groups() if group is not None)
                 for match in markdown.finditer(content)]
        links.extend(match.group(1) for match in html.finditer(content))
        for raw in links:
            target = unquote(urlparse(raw.split(maxsplit=1)[0].strip("<> ")).path)
            if not target or target.startswith("/") or urlparse(raw).scheme:
                continue
            resolved = (document.parent / target).resolve()
            try:
                candidate = resolved.relative_to(case_dir.resolve()).as_posix()
            except ValueError:
                continue
            if candidate in removed:
                errors.append(f"{relative_document}: link to retained resource {candidate}")
    return errors


def without_targeted_artifact_entries(text: str, targets: set[str]) -> str:
    """Remove only declared A entries for a scoped resource-note comparison."""
    lines = text.splitlines(keepends=True)
    starts = [i for i, line in enumerate(lines)
              if line.startswith("## ") and line[3:].strip() in REGISTER_SECTION_ALIASES["A"]]
    if len(starts) != 1:
        raise CaseError("resource_scope_invalid", "Case requires one artifact register")
    start = starts[0] + 1
    end = next((i for i in range(start, len(lines)) if lines[i].startswith("## ")), len(lines))
    header = re.compile(r"^(?:-\s+|###\s+)`?(A-[0-9]{3,})`?(?=$|[\s:—(])")
    boundaries = [i for i in range(start, end) if header.match(lines[i])]
    boundaries.append(end)
    kept: list[str] = []
    cursor = start
    for pos, following in zip(boundaries, boundaries[1:]):
        kept.extend(lines[cursor:pos])
        if header.match(lines[pos]).group(1) not in targets:
            kept.extend(lines[pos:following])
        cursor = following
    kept.extend(lines[cursor:end])
    # An added or removed targeted entry can leave one more separator line.
    section = "".join(kept).strip("\n")
    return "".join(lines[:start]) + section + "\n" + "".join(lines[end:])


def declared_export_ids(case_dir: Path, case_id: str) -> set[str]:
    story_ids: set[str] = set()
    for draft in (case_dir / "exports").glob("*.md"):
        if draft.is_symlink() or not draft.is_file():
            continue
        try:
            fields = parse_frontmatter(draft.read_text(encoding="utf-8"))
        except (CaseError, OSError, UnicodeDecodeError):
            continue
        story_id = str(fields.get("story-id", ""))
        if (
            STORY_ID_RE.fullmatch(story_id)
            and fields.get("source-investigation") == case_id
            and draft.name.startswith(f"{story_id}-")
        ):
            story_ids.add(story_id)
    return story_ids


def save_case(args: argparse.Namespace, root: Path) -> int:
    """Atomically replace a reviewed case snapshot and optional private overlay."""
    candidate = args.public_candidate.read_bytes()
    try:
        public_text = candidate.decode("utf-8")
    except UnicodeDecodeError as error:
        raise CaseError("candidate_invalid", "Public candidate must be UTF-8") from error
    reject_secret_input(public_text)
    if not args.source.strip() or "\n" in args.source or "\r" in args.source:
        raise CaseError("source_invalid", "Save source must be one nonempty line")
    reject_secret_input(args.source)
    if LOCAL_PATH_PATTERN.search(args.source):
        raise CaseError("source_invalid", "Save source must be portable")
    all_targets = args.target + args.private_target
    invalid_targets = [target for target in all_targets if REGISTER_ID_PATTERN.fullmatch(target) is None]
    if invalid_targets:
        raise CaseError("target_invalid", "Save targets must be existing register IDs", targets=invalid_targets)

    private_candidate: bytes | None = None
    private_text: str | None = None
    if args.private_candidate is not None and args.delete_private:
        raise CaseError("usage_error", "--private-candidate and --delete-private are mutually exclusive")
    if args.private_candidate is not None:
        private_candidate = args.private_candidate.read_bytes()
        try:
            private_text = private_candidate.decode("utf-8")
        except UnicodeDecodeError as error:
            raise CaseError("candidate_invalid", "Private candidate must be UTF-8") from error
        reject_secret_input(private_text)
        private_errors = validate_private_text(private_text, args.id)
        if private_errors:
            raise CaseError("private_validation_failed", "Private overlay is invalid", errors=private_errors)

    private_root = args.private_root
    if private_root is not None:
        if private_root.is_symlink():
            raise CaseError("root_symlink", "The private investigation root must not be a symlink")
        if private_root.name != ".investigations-private":
            raise CaseError("invalid_private_root", "Private root must be named .investigations-private")
        if private_root.parent.resolve(strict=True) != root.parent:
            raise CaseError("invalid_private_root", "Private overlay and investigation roots must share the vault root")
    if (private_candidate is not None or args.delete_private) and private_root is None:
        raise CaseError("private_root_required", "A private mutation requires --private-root")

    with locked(root):
        located = locate_live_case(root, args.id)
        record = located.record
        original_public = record.path.read_bytes()
        if digest_bytes(original_public) != args.expected_public_sha256:
            raise CaseError(
                "stale_public_snapshot",
                "Investigation changed after it was read",
                exit_code=3,
                current_sha256=digest_bytes(original_public),
            )

        private_path = private_root / args.id / "private.md" if private_root else None
        if private_path is not None and (
            private_path.is_symlink() or private_path.parent.is_symlink()
        ):
            raise CaseError("case_symlink", "Private overlay paths must not be symlinks")
        original_private = private_path.read_bytes() if private_path and private_path.is_file() else None
        if digest_bytes(original_private) != args.expected_private_sha256:
            raise CaseError(
                "stale_private_snapshot",
                "Private overlay changed after it was read",
                exit_code=3,
                current_sha256=digest_bytes(original_private),
            )

        public_changed = candidate != original_public
        private_changed = (
            (private_candidate is not None and private_candidate != original_private)
            or (args.delete_private and original_private is not None)
        )
        if not public_changed and not private_changed:
            emit(
                {
                    "status": "unchanged",
                    "id": args.id,
                    "public_sha256": digest_bytes(original_public),
                    "private_sha256": digest_bytes(original_private),
                }
            )
            return 0
        if public_changed and not args.target:
            raise CaseError(
                "traceability_target_missing",
                "A material public write requires at least one affected public register ID",
            )
        if private_changed and not args.private_target:
            raise CaseError(
                "traceability_target_missing",
                "A material private write requires at least one affected private register ID",
            )

        candidate_fields = parse_frontmatter(public_text)
        if candidate_fields.get("id") != args.id:
            raise CaseError("candidate_invalid", "Public candidate id does not match the target")
        if candidate_fields.get("artifact-integrity") != record.fields.get("artifact-integrity"):
            raise CaseError("artifact_integrity_invalid", "Artifact integrity mode cannot change through save")
        lifecycle_fields = ("status", "blocked-on", "closure-outcome", "resume-to")
        changed_lifecycle = [
            field
            for field in lifecycle_fields
            if candidate_fields.get(field) != record.fields.get(field)
        ]
        if changed_lifecycle:
            raise CaseError(
                "status_transition_required",
                "Lifecycle fields must change through transition or close",
                fields=changed_lifecycle,
            )
        export_ids = declared_export_ids(record.path.parent, args.id)
        missing_targets = [
            target
            for target in all_targets
            if not declares_register(public_text, target) and target not in export_ids
        ]
        if missing_targets:
            raise CaseError(
                "traceability_target_missing",
                "Every affected register ID must exist in the public candidate",
                targets=missing_targets,
            )

        timestamp = timestamp_value(args.timestamp)
        locale = record_locale(public_text)
        if public_changed or (args.delete_private and original_private is not None):
            public_text = replace_frontmatter_scalar(public_text, "updated-at", timestamp)
            targets = args.private_target if args.delete_private else args.target
            if locale == "es":
                verb = "Complemento privado eliminado para" if args.delete_private else "Registros actualizados"
            else:
                verb = "Removed private overlay linked to" if args.delete_private else "Updated registers"
            action = verb + " " + ", ".join(f"`{target}`" for target in targets)
            public_text = append_history_event(
                public_text,
                attributed_event(timestamp, action, args.identity, args.source, locale),
            )
            candidate = public_text.encode("utf-8")
        if private_candidate is not None and private_candidate != original_private:
            if private_text is None:
                raise CaseError("candidate_invalid", "Private candidate must be UTF-8")
            private_text = replace_frontmatter_scalar(private_text, "updated-at", timestamp)
            private_locale = record_locale(private_text)
            action = (
                "Contexto privado actualizado para "
                if private_locale == "es"
                else "Updated private context for "
            ) + ", ".join(
                f"`{target}`" for target in args.private_target
            )
            private_text = append_history_event(
                private_text,
                attributed_event(timestamp, action, args.identity, args.source, private_locale),
            )
            private_candidate = private_text.encode("utf-8")

        created_private_root = False
        created_private_dir = False
        try:
            atomic_write(record.path, candidate)
            if args.delete_private and private_path is not None and private_path.exists():
                private_path.unlink()
                if not any(private_path.parent.iterdir()):
                    private_path.parent.rmdir()
            elif private_candidate is not None and private_root is not None and private_path is not None:
                if not private_root.exists():
                    private_root.mkdir(mode=0o700)
                    created_private_root = True
                elif not private_root.is_dir():
                    raise CaseError("invalid_private_root", "Private root is not a directory")
                private_dir = private_path.parent
                if not private_dir.exists():
                    private_dir.mkdir(mode=0o700)
                    created_private_dir = True
                atomic_write(private_path, private_candidate)
            errors, warnings = validate_vault(root, ignore_lock=True)
            if errors:
                raise CaseError("save_validation_failed", "Candidate failed validation", errors=errors)
        except Exception:
            atomic_write(record.path, original_public)
            if private_path is not None:
                if original_private is None and private_path.exists():
                    private_path.unlink()
                elif original_private is not None:
                    private_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                    atomic_write(private_path, original_private)
                if created_private_dir and private_path.parent.exists():
                    private_path.parent.rmdir()
                if created_private_root and private_root is not None and private_root.exists():
                    private_root.rmdir()
            raise

    emit(
        {
            "status": "saved",
            "id": args.id,
            "public_sha256": digest_bytes(candidate),
            "private": private_candidate is not None,
            "private_deleted": bool(args.delete_private),
            "warnings": warnings,
        }
    )
    return 0


def save_resources(args: argparse.Namespace, root: Path) -> int:
    """Replace a reviewed case resource unit and its register entry together."""
    if not args.source.strip() or "\n" in args.source or "\r" in args.source:
        raise CaseError("source_invalid", "Resource source must be one portable line")
    reject_secret_input(args.source)
    if LOCAL_PATH_PATTERN.search(args.source):
        raise CaseError("source_invalid", "Resource source must be portable")
    targets = set(args.target)
    if not targets or any(re.fullmatch(r"A-[0-9]{3,}", target) is None for target in targets):
        raise CaseError("target_invalid", "Resource changes require A-NNN targets")
    candidate_dir = args.candidate_dir
    candidate_sha256, candidate_entries = case_tree_snapshot(candidate_dir)
    if candidate_sha256 != args.expected_candidate_tree_sha256:
        raise CaseError("stale_candidate_tree", "Reviewed resource candidate changed", exit_code=3)
    candidate_path = candidate_dir / "investigation.md"
    if not candidate_path.is_file():
        raise CaseError("candidate_invalid", "Resource candidate requires investigation.md")
    candidate_text = candidate_path.read_text(encoding="utf-8")
    reject_secret_input(candidate_text)
    candidate_fields = parse_frontmatter(candidate_text)
    if candidate_fields.get("id") != args.id:
        raise CaseError("candidate_invalid", "Resource candidate id does not match the target")
    for target in targets:
        if not declares_register(candidate_text, target):
            raise CaseError("traceability_target_missing", "Resource target is absent from case", target=target)

    with locked(root):
        located = locate_live_case(root, args.id)
        record = located.record
        if candidate_fields.get("artifact-integrity") != record.fields.get("artifact-integrity"):
            raise CaseError("artifact_integrity_invalid", "Artifact integrity mode cannot change through save-resources")
        source_dir = record.path.parent
        if candidate_dir.resolve() == source_dir.resolve() or source_dir.resolve() in candidate_dir.resolve().parents:
            raise CaseError("candidate_invalid", "Stage resource candidates outside the live case")
        source_sha256, source_entries = case_tree_snapshot(source_dir)
        if source_sha256 != args.expected_tree_sha256:
            raise CaseError("stale_case_tree", "Case resources changed after review", exit_code=3,
                            current_sha256=source_sha256)
        if candidate_sha256 == source_sha256:
            emit({"status": "unchanged", "id": args.id, "tree_sha256": source_sha256})
            return 0
        for field in ("status", "blocked-on", "closure-outcome", "resume-to"):
            if candidate_fields.get(field) != record.fields.get(field):
                raise CaseError("status_transition_required", "Lifecycle changes require transition or close", field=field)
        if without_targeted_artifact_entries(candidate_text, targets) != without_targeted_artifact_entries(record.text, targets):
            raise CaseError("resource_scope_invalid", "Case note changes must stay within targeted A-NNN entries")
        before = {entry["path"]: entry for entry in source_entries}
        after = {entry["path"]: entry for entry in candidate_entries}
        changed = [path for path in before.keys() | after.keys() if before.get(path) != after.get(path)]
        for path in changed:
            if path == "investigation.md":
                continue
            parts = Path(path).parts
            if path == "artifacts" and after.get(path, before.get(path)).get("type") == "directory":
                continue
            if len(parts) < 2 or parts[0] != "artifacts" or not any(
                parts[1] == target or parts[1].startswith(target + "-") for target in targets
            ):
                raise CaseError("resource_scope_invalid", "Only targeted A-NNN units may change", path=path)
        if not changed or changed == ["investigation.md"]:
            raise CaseError("resource_scope_invalid", "Resource update requires an artifact change")
        integrity_errors = artifact_integrity_errors(candidate_entries, candidate_text, targets)
        if integrity_errors:
            raise CaseError("artifact_integrity_invalid", "Resource register does not inventory the reviewed unit",
                            errors=integrity_errors)

        transaction_id = uuid.uuid4().hex
        staging = located.store / f".resource-{transaction_id}.tmp"
        backup = located.store / f".resource-{transaction_id}.backup"
        try:
            shutil.copytree(candidate_dir, staging, symlinks=False)
            if case_tree_snapshot(staging)[0] != candidate_sha256:
                raise CaseError("stale_candidate_tree", "Resource candidate changed while staging", exit_code=3)
            errors = public_content_errors(staging)
            if errors:
                raise CaseError("resource_unsafe", "Resource candidate failed content checks", errors=errors)
            timestamp = timestamp_value(args.timestamp)
            locale = record_locale(candidate_text)
            updated = replace_frontmatter_scalar(candidate_text, "updated-at", timestamp)
            action = ("Artefactos revisados " if locale == "es" else "Reviewed resources ") + ", ".join(
                f"`{target}`" for target in sorted(targets)
            )
            updated = append_history_event(
                updated, attributed_event(timestamp, action, args.identity, args.source, locale)
            )
            atomic_write(staging / "investigation.md", updated.encode("utf-8"))
            if case_tree_snapshot(source_dir)[0] != source_sha256:
                raise CaseError("stale_case_tree", "Case changed during resource staging", exit_code=3)
            os.replace(source_dir, backup)
            os.replace(staging, source_dir)
            validation_errors, warnings = validate_vault(root, ignore_lock=True)
            if validation_errors:
                raise CaseError("resource_validation_failed", "Resource candidate failed validation",
                                errors=validation_errors)
        except Exception:
            if backup.exists():
                if source_dir.exists():
                    shutil.rmtree(source_dir)
                os.replace(backup, source_dir)
            if staging.exists():
                shutil.rmtree(staging)
            raise
        try:
            shutil.rmtree(backup)
        except OSError:
            warnings.append(f"Committed resource backup remains at {backup}")
    emit({"status": "resources_saved", "id": args.id,
          "tree_sha256": case_tree_snapshot(source_dir)[0], "changed": sorted(changed),
          "warnings": warnings})
    return 0


def add_consolidated_from(text: str, retired_id: str) -> str:
    fields = parse_frontmatter(text)
    if retired_id in fields.get("consolidated-from", []):
        return text
    lines = text.splitlines()
    frontmatter_end = lines.index("---", 1)
    for index in range(1, frontmatter_end):
        if lines[index] != "consolidated-from:":
            continue
        insertion = index + 1
        while insertion < frontmatter_end and lines[insertion].startswith("  - "):
            insertion += 1
        lines.insert(insertion, f"  - {retired_id}")
        return "\n".join(lines) + ("\n" if text.endswith("\n") else "")
    lines[frontmatter_end:frontmatter_end] = [
        "consolidated-from:",
        f"  - {retired_id}",
    ]
    return "\n".join(lines) + ("\n" if text.endswith("\n") else "")


def replace_frontmatter_scalar(text: str, field: str, value: str) -> str:
    lines = text.splitlines()
    frontmatter_end = lines.index("---", 1)
    prefix = f"{field}:"
    for index in range(1, frontmatter_end):
        if lines[index].startswith(prefix):
            lines[index] = f"{field}: {value}"
            return "\n".join(lines) + ("\n" if text.endswith("\n") else "")
    raise CaseError(
        "frontmatter_field_missing",
        f"The canonical case has no {field} field",
        field=field,
    )


def upsert_frontmatter_scalar(text: str, field: str, value: str) -> str:
    try:
        return replace_frontmatter_scalar(text, field, value)
    except CaseError as error:
        if error.code != "frontmatter_field_missing":
            raise
    lines = text.splitlines()
    frontmatter_end = lines.index("---", 1)
    lines.insert(frontmatter_end, f"{field}: {value}")
    return "\n".join(lines) + ("\n" if text.endswith("\n") else "")


def remove_frontmatter_fields(text: str, fields: set[str]) -> str:
    lines = text.splitlines()
    frontmatter_end = lines.index("---", 1)
    prefixes = tuple(f"{field}:" for field in fields)
    kept = [
        line
        for index, line in enumerate(lines)
        if index >= frontmatter_end or not line.startswith(prefixes)
    ]
    return "\n".join(kept) + ("\n" if text.endswith("\n") else "")


def archive_case(retired: Path, archive: Path, timestamp: str) -> None:
    temporary = archive.with_name(f".{archive.name}.tmp-{uuid.uuid4().hex}")
    require_child(archive.parents[3], temporary)
    try:
        temporary.mkdir(parents=True, mode=0o700)
        manifest_files: list[dict[str, str]] = []
        for source in sorted(retired.rglob("*")):
            if source.is_symlink():
                raise CaseError(
                    "case_symlink",
                    "Retiring cases must not contain symlinks",
                    path=str(source),
                )
            relative = source.relative_to(retired)
            destination = temporary / relative
            if source.is_dir():
                destination.mkdir(parents=True, exist_ok=True, mode=0o700)
                continue
            destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            shutil.copyfile(source, destination)
            os.chmod(destination, 0o600)
            manifest_files.append({"path": relative.as_posix(), "sha256": sha256(destination)})
        atomic_write(
            temporary / "manifest.json",
            json.dumps(
                {
                    "retired-id": retired.name,
                    "captured-at": timestamp,
                    "files": manifest_files,
                },
                indent=2,
                sort_keys=True,
            ).encode("utf-8")
            + b"\n",
        )
        archive.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        os.replace(temporary, archive)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)


def restore_retired(archive: Path, retired: Path) -> None:
    retired.mkdir(mode=0o700)
    for source in sorted(archive.rglob("*")):
        if source.name == "manifest.json":
            continue
        relative = source.relative_to(archive)
        destination = retired / relative
        if source.is_dir():
            destination.mkdir(parents=True, exist_ok=True, mode=0o700)
        else:
            destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            shutil.copyfile(source, destination)
            os.chmod(destination, 0o600)


def consolidate_case(args: argparse.Namespace, root: Path) -> int:
    if args.canonical == args.retire:
        raise CaseError("same_case", "Canonical and retiring IDs must differ")
    timestamp = timestamp_value(args.timestamp)

    with locked(root):
        canonical_located = locate_live_case(root, args.canonical)
        retiring_located = locate_live_case(root, args.retire)
        if canonical_located.visibility != retiring_located.visibility:
            raise CaseError(
                "visibility_conflict",
                "Canonical and retiring investigations must share the same visibility",
            )
        canonical = canonical_located.record
        retiring = retiring_located.record
        canonical_snapshot = canonical.path.read_bytes()
        retiring_snapshot = retiring.path.read_bytes()
        if digest_bytes(canonical_snapshot) != args.expected_canonical_sha256:
            raise CaseError(
                "stale_canonical_snapshot",
                "Canonical investigation changed after reconciliation",
                exit_code=3,
                current_sha256=digest_bytes(canonical_snapshot),
            )
        if digest_bytes(retiring_snapshot) != args.expected_retire_sha256:
            raise CaseError(
                "stale_retiring_snapshot",
                "Retiring investigation changed after reconciliation",
                exit_code=3,
                current_sha256=digest_bytes(retiring_snapshot),
            )
        canonical_dir = canonical.path.parent
        retiring_dir = retiring.path.parent
        mapping = canonical_dir / "artifacts" / f"consolidation-{args.retire}-mapping.md"
        if not mapping.is_file():
            raise CaseError(
                "semantic_mapping_missing",
                "Consolidation requires the agent-reconciled mapping artifact",
                mapping=str(mapping),
            )
        mapping_text = mapping.read_text(encoding="utf-8")
        learning_reconciliation = re.search(
            r"(?m)^learning-assessment: (preserved|reset)$",
            mapping_text,
        )
        if (
            f"retired-id: {args.retire}" not in mapping_text
            or re.search(r"(?m)^drafts: (?:none|reconciled|stale)$", mapping_text) is None
            or learning_reconciliation is None
        ):
            raise CaseError(
                "semantic_mapping_invalid",
                "The mapping must name the retired ID, draft reconciliation state, "
                "and learning assessment reconciliation",
                mapping=str(mapping),
            )

        archive = canonical_dir / "artifacts" / "consolidated" / args.retire
        if archive.exists():
            raise CaseError("archive_exists", "The consolidation archive already exists")
        original_canonical = canonical.path.read_bytes()
        removed = False
        try:
            archive_case(retiring_dir, archive, timestamp)
            archive_errors = validate_archive(archive)
            if archive_errors:
                raise CaseError(
                    "archive_invalid",
                    "The consolidation archive failed validation",
                    errors=archive_errors,
                )
            updated = add_consolidated_from(
                original_canonical.decode("utf-8"),
                args.retire,
            )
            updated = replace_frontmatter_scalar(
                updated,
                "updated-at",
                timestamp,
            )
            prior_learning_outcome = str(
                canonical.fields.get("learning-outcome", "missing")
            )
            if learning_reconciliation.group(1) == "reset":
                updated = replace_frontmatter_scalar(
                    updated,
                    "learning-outcome",
                    "not-evaluated",
                )
                learning_event = (
                    f"learning assessment reset from `{prior_learning_outcome}` "
                    "to `not-evaluated`"
                )
            else:
                learning_event = (
                    f"learning assessment preserved as `{prior_learning_outcome}`"
                )
            locale = record_locale(updated)
            consolidation_action = (
                f"Consolidado `{args.retire}`; archivo "
                if locale == "es"
                else f"Consolidated `{args.retire}`; archive "
            )
            updated = append_history_event(
                updated,
                attributed_event(
                    timestamp,
                    consolidation_action
                    +
                    f"`artifacts/consolidated/{args.retire}/`; mapping "
                    f"`artifacts/{mapping.name}`; {learning_event}",
                    args.identity,
                    f"semantic mapping `artifacts/{mapping.name}`",
                    locale,
                ),
            )
            atomic_write(canonical.path, updated.encode("utf-8"))
            shutil.rmtree(retiring_dir)
            removed = True
            errors, warnings = validate_vault(root, ignore_lock=True)
            if errors:
                raise CaseError(
                    "consolidation_validation_failed",
                    "The consolidated case failed validation",
                    errors=errors,
                )
        except Exception:
            if removed and not retiring_dir.exists():
                restore_retired(archive, retiring_dir)
            atomic_write(canonical.path, original_canonical)
            if archive.exists():
                shutil.rmtree(archive)
            raise

    emit(
        {
            "status": "consolidated",
            "canonical": args.canonical,
            "retired": args.retire,
            "warnings": warnings,
        }
    )
    return 0


def validate_event_inputs(code: str, **values: str) -> None:
    reject_secret_input(*values.values())
    for field, value in values.items():
        if not value.strip() or "\n" in value or "\r" in value:
            raise CaseError(code, f"{field} must be one nonempty line")
        if LOCAL_PATH_PATTERN.search(value):
            raise CaseError(code, f"{field} must be portable")


def transition_case(args: argparse.Namespace, root: Path) -> int:
    values = {"reason": args.reason, "source": args.source}
    if args.blocked_on is not None:
        values["blocked-on"] = args.blocked_on
    validate_event_inputs("transition_invalid", **values)
    timestamp = timestamp_value(args.timestamp)
    with locked(root):
        record = locate_live_case(root, args.id).record
        original = record.path.read_bytes()
        if digest_bytes(original) != args.expected_public_sha256:
            raise CaseError(
                "stale_public_snapshot",
                "Investigation changed after it was read",
                exit_code=3,
                current_sha256=digest_bytes(original),
            )
        current = str(record.fields.get("status", ""))
        migration_mode = (
            current in OBSOLETE_STATUSES
            or "resume-to" in record.fields
            or (
                current == "closed"
                and record.fields.get("closure-outcome") not in CLOSURE_OUTCOMES
            )
        )
        preexisting_errors, _ = validate_vault(root, ignore_lock=True)
        allowed = {
            ("investigating", "blocked"),
            ("blocked", "investigating"),
            ("closed", "investigating"),
        }
        allowed.update((status, "investigating") for status in OBSOLETE_STATUSES)
        if migration_mode:
            allowed.add((current, "investigating"))
        if (current, args.to) not in allowed:
            raise CaseError(
                "transition_invalid",
                "Unsupported investigation lifecycle transition",
                current=current,
                requested=args.to,
            )
        if migration_mode and args.to != "investigating":
            raise CaseError(
                "transition_invalid",
                "Prior lifecycle conversion must end in investigating",
                current=current,
                requested=args.to,
            )
        if args.to == "blocked" and not args.blocked_on:
            raise CaseError("transition_invalid", "Blocking requires --blocked-on")
        if args.to != "blocked" and args.blocked_on:
            raise CaseError("transition_invalid", "--blocked-on is valid only when blocking")

        updated = replace_frontmatter_scalar(record.text, "status", args.to)
        updated = replace_frontmatter_scalar(updated, "updated-at", timestamp)
        if args.to == "blocked":
            updated = upsert_frontmatter_scalar(
                updated,
                "blocked-on",
                json.dumps(args.blocked_on, ensure_ascii=False),
            )
        else:
            updated = remove_frontmatter_fields(
                updated,
                {"blocked-on", "closure-outcome", "resume-to"},
            )
        locale = record_locale(updated)
        if migration_mode:
            subject = (
                f"estado anterior `{current}`"
                if locale == "es"
                else f"prior state `{current}`"
            )
            action = (
                f"{subject.capitalize()} migrado a `investigating`; razón: {args.reason}"
                if locale == "es"
                else f"Migrated {subject} to `investigating`; reason: {args.reason}"
            )
        elif current == "investigating":
            action = (
                f"Investigación bloqueada por {args.blocked_on}; razón: {args.reason}"
                if locale == "es"
                else f"Investigation blocked by {args.blocked_on}; reason: {args.reason}"
            )
        elif current == "blocked":
            action = (
                f"Investigación desbloqueada; razón: {args.reason}"
                if locale == "es"
                else f"Investigation unblocked; reason: {args.reason}"
            )
        else:
            action = (
                f"Investigación reabierta; razón: {args.reason}"
                if locale == "es"
                else f"Investigation reopened; reason: {args.reason}"
            )
        updated = append_history_event(
            updated,
            attributed_event(timestamp, action, args.identity, args.source, locale),
        )
        try:
            atomic_write(record.path, updated.encode("utf-8"))
            errors, warnings = validate_vault(root, ignore_lock=True)
            new_errors = sorted(set(errors) - set(preexisting_errors))
            if errors and (not migration_mode or new_errors):
                raise CaseError(
                    "transition_validation_failed",
                    "Lifecycle transition failed validation",
                    errors=new_errors or errors,
                )
            if errors:
                warnings.append(
                    "Other pre-existing case errors remain; complete the requested "
                    "public lifecycle migration before normal mutations"
                )
        except Exception:
            atomic_write(record.path, original)
            raise
    emit(
        {
            "status": "transitioned",
            "id": args.id,
            "from": current,
            "to": args.to,
            "migration": migration_mode,
            "warnings": warnings,
        }
    )
    return 0


def close_case(args: argparse.Namespace, root: Path) -> int:
    validate_event_inputs(
        "closure_invalid",
        reason=args.reason,
        limitations=args.limitations,
        source=args.source,
    )
    timestamp = timestamp_value(args.timestamp)
    with locked(root):
        record = locate_live_case(root, args.id).record
        current_status = str(record.fields.get("status", ""))
        if current_status == "closed":
            raise CaseError("closure_gate_failed", "Investigation is already closed")
        lifecycle_metadata_valid = (
            current_status in {"investigating", "blocked"}
            and "resume-to" not in record.fields
            and not record.fields.get("closure-outcome")
            and (
                (current_status == "blocked" and bool(record.fields.get("blocked-on")))
                or (current_status == "investigating" and not record.fields.get("blocked-on"))
            )
        )
        if not lifecycle_metadata_valid:
            raise CaseError(
                "closure_gate_failed",
                "Investigation lifecycle must be valid and prior published states must be "
                "migrated to investigating before closure",
                current=current_status,
            )
        original = record.path.read_bytes()
        if digest_bytes(original) != args.expected_public_sha256:
            raise CaseError(
                "stale_public_snapshot",
                "Investigation changed after it was read",
                exit_code=3,
                current_sha256=digest_bytes(original),
            )
        if args.decision == "complete":
            if record.fields.get("purpose") == "undecided":
                raise CaseError("closure_gate_failed", "Completion requires a resolved purpose")
            if record.fields.get("vault-outcome") == "not-evaluated":
                raise CaseError("closure_gate_failed", "Completion requires an evaluated vault outcome")
            if not args.evidence:
                raise CaseError("closure_gate_failed", "Completion requires closure evidence IDs")
        declared_exports = declared_export_ids(record.path.parent, args.id)
        invalid_evidence = [
            value
            for value in args.evidence
            if REGISTER_ID_PATTERN.fullmatch(value) is None
            or (
                not declares_register(record.text, value)
                and value not in declared_exports
            )
        ]
        if invalid_evidence:
            raise CaseError(
                "closure_gate_failed",
                "Closure evidence must reference existing register IDs",
                evidence=invalid_evidence,
            )
        updated = replace_frontmatter_scalar(record.text, "status", "closed")
        updated = replace_frontmatter_scalar(updated, "updated-at", timestamp)
        outcome = "completed" if args.decision == "complete" else "abandoned"
        updated = upsert_frontmatter_scalar(updated, "closure-outcome", outcome)
        updated = remove_frontmatter_fields(updated, {"blocked-on", "resume-to"})
        if args.decision == "abandoned" and record.fields.get("vault-outcome") == "not-evaluated":
            updated = replace_frontmatter_scalar(updated, "vault-outcome", "none")
        locale = record_locale(updated)
        evidence = ", ".join(f"`{value}`" for value in args.evidence) or "none"
        closure_action = (
            f"Investigación cerrada como `{outcome}`; razón: {args.reason}; "
            f"evidencia: {evidence}; limitaciones pendientes: {args.limitations}"
            if locale == "es"
            else f"Investigation closed as `{outcome}`; reason: {args.reason}; "
            f"evidence: {evidence}; outstanding limitations: {args.limitations}"
        )
        event = attributed_event(
            timestamp,
            closure_action,
            args.identity,
            args.source,
            locale,
        )
        updated = append_history_event(updated, event)
        try:
            atomic_write(record.path, updated.encode("utf-8"))
            errors, warnings = validate_vault(root, ignore_lock=True)
            if errors:
                raise CaseError("closure_validation_failed", "Closure failed validation", errors=errors)
        except Exception:
            atomic_write(record.path, original)
            raise
    emit({"status": "closed", "id": args.id, "outcome": outcome, "warnings": warnings})
    return 0


def bind_case(args: argparse.Namespace, root: Path) -> int:
    """Persist a validated vault-side observation; never access the worktree."""
    try:
        observation = json.loads(args.observation.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise CaseError("binding_observation_invalid", str(error)) from error
    keys = DEVELOPMENT_HANDOFF_HISTORY_KEYS[1:]
    if (not isinstance(observation, dict) or set(observation) != set(keys)
            or any(not isinstance(v, str) or not v or "\n" in v or "\r" in v
                   for v in observation.values())):
        raise CaseError("binding_observation_invalid", "Expected exact binding fields excluding dh")
    reject_secret_input(*(str(value) for value in observation.values()))
    fields = dict(zip(DEVELOPMENT_HANDOFF_FIELDS, (observation[k] for k in keys)))
    with locked(root):
        record = locate_live_case(root, args.id).record
        original = record.path.read_bytes()
        if digest_bytes(original) != args.expected_public_sha256:
            raise CaseError(
                "stale_public_snapshot", "Investigation changed after binding review",
                exit_code=3, current_sha256=digest_bytes(original),
            )
        text = original.decode("utf-8")
        errors, warnings = validate_vault(root, ignore_lock=True)
        if errors:
            raise CaseError("binding_validation_failed", "Source case is invalid", errors=errors)
        stories = []
        for draft in (record.path.parent / "exports").glob("*.md"):
            if draft.is_symlink():
                raise CaseError("case_symlink", "Story drafts must not be symlinks")
            metadata = parse_frontmatter(draft.read_text(encoding="utf-8"))
            if metadata.get("story-id") == fields["Story ID"]:
                stories.append((metadata, draft))
        if len(stories) != 1 or stories[0][0].get("source-investigation") != args.id:
            raise CaseError("binding_story_missing", "Source case must contain one exact story draft")
        story_digest = sha256(stories[0][1])
        if story_digest != args.expected_story_sha256:
            raise CaseError(
                "stale_story_snapshot", "Source story changed after binding review",
                exit_code=3, current_sha256=story_digest,
            )
        lines, count = development_handoff_section(text)
        if count != 1 or lines is None:
            raise CaseError("binding_section_missing", "Source case needs Development handoffs section")
        content = "\n".join(lines)
        entries = list(re.finditer(r"(?ms)^### (DH-[0-9]{3,}) — .*?(?=^### |\Z)", content))
        target = None
        old = None
        for entry in entries:
            values = dict(match.groups() for line in entry.group(0).splitlines()
                          if (match := DEVELOPMENT_HANDOFF_FIELD_RE.fullmatch(line)))
            if (values.get("Story ID"), values.get("Repository remote")) == (fields["Story ID"], fields["Repository remote"]):
                target, old = entry, values
        entry_id = target.group(1) if target else f"DH-{len(entries)+1:03d}"
        if old == fields:
            emit({"status": "unchanged", "id": args.id, "dh": entry_id, "warnings": warnings})
            return 0
        if old:
            stable = set(DEVELOPMENT_HANDOFF_FIELDS) - {"Revision", "Materialized at"}
            if any(old[k] != fields[k] for k in stable):
                raise CaseError("binding_identity_mismatch", "Immutable binding coordinates changed")
        expected = int(old["Revision"][1:]) + 1 if old else 1
        if fields["Revision"] != f"v{expected:04d}":
            raise CaseError("binding_revision_invalid", "Binding requires the exact next revision")
        title = f'{fields["Tracker ID"]}:{fields["Work item reference"]} / {fields["Repository remote"].rsplit("/", 1)[-1]}'
        block = f"### {entry_id} — {title}\n\n" + "\n".join(f"- {k}: {fields[k]}" for k in DEVELOPMENT_HANDOFF_FIELDS) + "\n\n"
        content = (content[:target.start()] + block + content[target.end():]) if target else content.rstrip() + "\n\n" + block
        all_lines = text.splitlines(keepends=True)
        start = next(i for i, line in enumerate(all_lines) if line.startswith("## ") and line[3:].strip() in DEVELOPMENT_HANDOFF_SECTION_ALIASES) + 1
        end = next((i for i in range(start, len(all_lines)) if all_lines[i].startswith("## ")), len(all_lines))
        all_lines[start:end] = ["\n" + content.strip() + "\n\n"]
        updated = "".join(all_lines)
        marker = json.dumps(handoff_history_binding(entry_id, fields), ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        locale = record_locale(updated)
        bound_action = "Vinculado" if locale == "es" else "Bound"
        recorder_label = "registrado por" if locale == "es" else "recorded by"
        source_label = "fuente" if locale == "es" else "source"
        event = (f'- {fields["Materialized at"]} — {bound_action} development handoff `{entry_id}`; '
                 f'story `{fields["Story ID"]}`; work item `{fields["Tracker ID"]}:{fields["Work item reference"]}`; '
                 f'repository `{fields["Repository remote"]}`; branch `{fields["Branch"]}`; '
                 f'handoff `{fields["Handoff ID"]}`; revision `{fields["Revision"]}`; '
                 f'{recorder_label} {args.identity.display}; {source_label}: validated handoff observation.\n'
                 f'  <!-- {DEVELOPMENT_HANDOFF_HISTORY_MARKER_TOKEN} {marker} -->\n')
        history = re.search(r"(?m)^## (?:" + "|".join(map(re.escape, HISTORY_SECTION_ALIASES)) + r")\s*$", updated)
        position = updated.find("\n## ", history.end())
        if position < 0:
            position = len(updated)
        updated = updated[:position].rstrip() + "\n\n" + event + "\n" + updated[position:]
        try:
            atomic_write(record.path, updated.encode("utf-8"))
            errors, warnings = validate_vault(root, ignore_lock=True)
            if errors:
                raise CaseError("binding_validation_failed", "Binding failed validation", errors=errors)
        except Exception:
            atomic_write(record.path, original)
            raise
    emit({"status": "bound", "id": args.id, "dh": entry_id, "warnings": warnings})
    return 0


def publish_case(args: argparse.Namespace, root: Path) -> int:
    timestamp = timestamp_value(args.timestamp)
    if not args.source.strip() or "\n" in args.source or "\r" in args.source:
        raise CaseError("source_invalid", "Publish source must be one nonempty line")
    reject_secret_input(args.source)
    if LOCAL_PATH_PATTERN.search(args.source):
        raise CaseError("source_invalid", "Publish source must be portable")
    with locked(root):
        located = locate_live_case(root, args.id)
        if located.visibility != "unpublished":
            raise CaseError("already_published", "Investigation is already published")
        record = located.record
        original = record.path.read_bytes()
        if digest_bytes(original) != args.expected_public_sha256:
            raise CaseError(
                "stale_public_snapshot",
                "Unpublished investigation changed after it was read",
                exit_code=3,
                current_sha256=digest_bytes(original),
            )
        current_tree_sha256, current_entries = case_tree_snapshot(record.path.parent)
        if current_tree_sha256 != args.expected_tree_sha256:
            raise CaseError(
                "stale_case_tree",
                "Case resources changed after publication review",
                exit_code=3,
                current_sha256=current_tree_sha256,
            )
        source_dir = record.path.parent
        dest = require_child(root, root / args.id)
        if dest.exists():
            raise CaseError(
                "visibility_conflict",
                "A published directory already exists for this investigation",
            )
        transaction_id = uuid.uuid4().hex
        staging = root / f".publish-{transaction_id}.tmp"
        backup = source_dir.parent / f".publish-{transaction_id}.backup"
        require_child(root, staging)
        retained: list[Path] = []
        for raw in args.retain_local:
            relative = Path(raw)
            if relative.is_absolute() or not relative.parts or any(part in {".", ".."} for part in relative.parts):
                raise CaseError("retain_path_invalid", "Retained paths must be case-relative", path=raw)
            if relative.as_posix() == "investigation.md":
                raise CaseError("retain_path_invalid", "The case file must be published")
            if not (source_dir / relative).exists():
                raise CaseError("retain_path_missing", "Retained source does not exist", path=raw)
            if any(relative == previous or previous in relative.parents or relative in previous.parents for previous in retained):
                raise CaseError("retain_path_duplicate", "Retained paths overlap", path=raw)
            retained.append(relative)
        for relative in retained:
            selected = [entry["path"] for entry in current_entries
                        if entry.get("type") == "file" and
                        (Path(entry["path"]) == relative or relative in Path(entry["path"]).parents)]
            for path in selected:
                unit = artifact_unit_id(path)
                if (unit and declares_register(record.text, unit)) or path in record.text:
                    raise CaseError("retain_referenced_resource",
                                    "A registered or referenced resource must remain with the published case",
                                    path=path)
        retained_target: Path | None = None
        if retained:
            local_root = local_working_dir(root, args.id)
            if any(path.is_symlink() for path in (local_root, local_root.parent, local_root.parent.parent)):
                raise CaseError("case_symlink", "Local retention path must not be a symlink")
            retained_target = local_root / "publish-retained" / current_tree_sha256
            if retained_target.exists():
                raise CaseError("retain_conflict", "Reviewed case snapshot was already retained locally")
        published_bytes = original
        try:
            shutil.copytree(source_dir, staging, symlinks=False)
            staged_tree_sha256, _ = case_tree_snapshot(staging)
            if staged_tree_sha256 != current_tree_sha256:
                raise CaseError("stale_case_tree", "Case changed during publication staging", exit_code=3)
            if retained_target is not None:
                retained_target.mkdir(parents=True)
            for relative in retained:
                staged_path = staging / relative
                retained_path = retained_target / relative
                retained_path.parent.mkdir(parents=True, exist_ok=True)
                if staged_path.is_dir():
                    shutil.copytree(staged_path, retained_path)
                    if case_tree_snapshot(staged_path) != case_tree_snapshot(retained_path):
                        raise CaseError("retain_copy_failed", "Retained directory differs from reviewed bytes", path=str(relative))
                    shutil.rmtree(staged_path)
                else:
                    shutil.copy2(staged_path, retained_path)
                    if staged_path.read_bytes() != retained_path.read_bytes() or stat.S_IMODE(staged_path.stat().st_mode) != stat.S_IMODE(retained_path.stat().st_mode):
                        raise CaseError("retain_copy_failed", "Retained file differs from reviewed bytes", path=str(relative))
                    staged_path.unlink()
            removed = {entry["path"] for entry in current_entries
                       if entry.get("type") == "file" and
                       any(Path(entry["path"]) == item or item in Path(entry["path"]).parents
                           for item in retained)}
            link_errors = retained_link_errors(staging, removed)
            if link_errors:
                raise CaseError("retain_referenced_resource",
                                "Published resources link to locally retained files",
                                errors=link_errors)
            integrity_errors = artifact_integrity_errors(case_tree_snapshot(staging)[1], record.text)
            if integrity_errors:
                raise CaseError("artifact_integrity_invalid",
                                "Published artifacts require a path and SHA-256 in their A-NNN entries",
                                errors=integrity_errors)
            content_errors = public_content_errors(staging)
            if content_errors:
                raise CaseError(
                    "publish_unsafe",
                    "Selected investigation content is not safe to publish",
                    errors=content_errors,
                )
            published_path = staging / "investigation.md"
            locale = record_locale(record.text)
            action = (
                "Investigación publicada"
                if locale == "es"
                else "Published investigation"
            )
            published_text = replace_frontmatter_scalar(
                published_path.read_text(encoding="utf-8"),
                "updated-at",
                timestamp,
            )
            published_text = upsert_frontmatter_scalar(
                published_text, "artifact-integrity", "sha256-v1"
            )
            published_text = append_history_event(
                published_text,
                attributed_event(timestamp, action, args.identity, args.source, locale),
            )
            atomic_write(published_path, published_text.encode("utf-8"))
            if case_tree_snapshot(source_dir)[0] != current_tree_sha256:
                raise CaseError("stale_case_tree", "Case changed during publication", exit_code=3)
            os.replace(source_dir, backup)
            os.replace(staging, dest)
            errors, warnings = validate_vault(root, ignore_lock=True)
            if errors:
                raise CaseError(
                    "publish_validation_failed",
                    "Published investigation failed validation",
                    errors=errors,
                )
            published_bytes = (dest / "investigation.md").read_bytes()
        except Exception:
            if dest.exists():
                shutil.rmtree(dest)
            if backup.exists():
                os.replace(backup, source_dir)
            if staging.exists():
                shutil.rmtree(staging)
            if retained_target is not None and retained_target.exists():
                shutil.rmtree(retained_target)
            raise
        try:
            shutil.rmtree(backup)
        except OSError:
            warnings.append(f"Committed publication backup remains at {backup}")
    emit(
        {
            "status": "published",
            "id": args.id,
            "path": str(dest / "investigation.md"),
            "visibility": "published",
            "public_sha256": digest_bytes(published_bytes),
            "tree_sha256": case_tree_snapshot(dest)[0],
            "retained_local": [path.as_posix() for path in retained],
            "warnings": warnings,
        }
    )
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True, type=Path)
    commands = parser.add_subparsers(dest="command", required=True)

    open_parser = commands.add_parser("open", help="check and create one case atomically")
    open_parser.add_argument("--id", required=True)
    open_parser.add_argument("--title", required=True)
    open_parser.add_argument("--objective", required=True)
    open_parser.add_argument("--dedupe-key", required=True)
    open_parser.add_argument("--source-ref", action="append", default=[])
    open_parser.add_argument(
        "--source-type",
        choices=("message", "bug", "ticket", "issue", "attachment", "other"),
        default="message",
    )
    open_parser.add_argument("--requester-role", default="unknown")
    open_parser.add_argument(
        "--export-intent",
        choices=("technical-stories", "user-stories", "mixed", "undecided"),
        default="undecided",
    )
    open_parser.add_argument(
        "--purpose",
        choices=tuple(sorted(PURPOSES)),
        required=True,
    )
    open_parser.add_argument(
        "--vault-outcome",
        choices=tuple(sorted(VAULT_OUTCOMES)),
        required=True,
    )
    open_parser.add_argument(
        "--learning-outcome",
        choices=tuple(sorted(LEARNING_OUTCOMES)),
        required=True,
    )
    open_parser.add_argument("--request-summary", required=True)
    open_parser.add_argument("--timestamp")
    open_parser.add_argument(
        "--visibility",
        choices=("unpublished", "published"),
        default="unpublished",
    )

    load_parser = commands.add_parser(
        "load",
        help="resolve one unpublished or published case and discover overlay and local working store",
    )
    load_parser.add_argument("--id", required=True)

    snapshot_parser = commands.add_parser("snapshot", help="inventory one reviewed case tree")
    snapshot_parser.add_argument("--case-dir", required=True, type=Path)

    commands.add_parser("list", help="derive active cases and retired entries")
    retire_parser = commands.add_parser(
        "retire",
        help="delete a closed unpublished case, or retire a closed published case with a Git snapshot",
    )
    for option in ("id", "expected-public-sha256", "reason", "source",
                   "dependency-review", "absorption-review", "summary"):
        retire_parser.add_argument("--" + option, required=True)
    retire_parser.add_argument("--snapshot-commit")
    retire_parser.add_argument("--destination", action="append", default=[])
    retire_parser.add_argument("--authorized", action="store_true")
    retire_parser.add_argument("--timestamp")

    publish_parser = commands.add_parser(
        "publish",
        help="move one unpublished case into versioned investigations/",
    )
    publish_parser.add_argument("--id", required=True)
    publish_parser.add_argument(
        "--expected-public-sha256",
        required=True,
        help="unpublished investigation.md SHA-256 from load public.sha256",
    )
    publish_parser.add_argument("--expected-tree-sha256", required=True)
    publish_parser.add_argument("--retain-local", action="append", default=[])
    publish_parser.add_argument("--source", required=True)
    publish_parser.add_argument("--timestamp")

    consolidate_parser = commands.add_parser(
        "consolidate",
        help="archive and retire one semantically reconciled duplicate",
    )
    consolidate_parser.add_argument("--canonical", required=True)
    consolidate_parser.add_argument("--retire", required=True)
    consolidate_parser.add_argument("--expected-canonical-sha256", required=True)
    consolidate_parser.add_argument("--expected-retire-sha256", required=True)
    consolidate_parser.add_argument("--timestamp")

    transition_parser = commands.add_parser(
        "transition",
        help="block, unblock, reopen, or convert one prior status with compare-and-swap",
    )
    transition_parser.add_argument("--id", required=True)
    transition_parser.add_argument("--to", required=True, choices=("investigating", "blocked"))
    transition_parser.add_argument("--reason", required=True)
    transition_parser.add_argument("--source", required=True)
    transition_parser.add_argument("--blocked-on")
    transition_parser.add_argument("--expected-public-sha256", required=True)
    transition_parser.add_argument("--timestamp")

    close_parser = commands.add_parser("close", help="close one investigation with an explicit outcome")
    close_parser.add_argument("--id", required=True)
    close_parser.add_argument("--decision", required=True, choices=("complete", "abandoned"))
    close_parser.add_argument("--reason", required=True)
    close_parser.add_argument("--limitations", required=True)
    close_parser.add_argument("--source", required=True)
    close_parser.add_argument("--evidence", action="append", default=[])
    close_parser.add_argument("--expected-public-sha256", required=True)
    close_parser.add_argument("--timestamp")

    bind_parser = commands.add_parser("bind", help="transactionally bind one validated handoff observation")
    bind_parser.add_argument("--id", required=True)
    bind_parser.add_argument("--observation", required=True, type=Path)
    bind_parser.add_argument("--expected-public-sha256", required=True)
    bind_parser.add_argument("--expected-story-sha256", required=True)

    save_parser = commands.add_parser(
        "save",
        help="atomically save one reviewed case snapshot and optional private overlay",
    )
    save_parser.add_argument("--id", required=True)
    save_parser.add_argument("--public-candidate", required=True, type=Path)
    save_parser.add_argument("--expected-public-sha256", required=True)
    save_parser.add_argument("--private-root", required=True, type=Path)
    save_parser.add_argument("--private-candidate", type=Path)
    save_parser.add_argument("--delete-private", action="store_true")
    save_parser.add_argument("--expected-private-sha256", default="absent")
    save_parser.add_argument("--source", required=True)
    save_parser.add_argument("--target", action="append", default=[])
    save_parser.add_argument("--private-target", action="append", default=[])
    save_parser.add_argument("--timestamp")

    resources_parser = commands.add_parser(
        "save-resources", help="replace reviewed A-NNN resource units and case record together"
    )
    resources_parser.add_argument("--id", required=True)
    resources_parser.add_argument("--candidate-dir", required=True, type=Path)
    resources_parser.add_argument("--expected-tree-sha256", required=True)
    resources_parser.add_argument("--expected-candidate-tree-sha256", required=True)
    resources_parser.add_argument("--target", action="append", required=True)
    resources_parser.add_argument("--source", required=True)
    resources_parser.add_argument("--timestamp")

    commands.add_parser("validate", help="validate case-file mechanics")
    return parser


def main(argv: list[str] | None = None) -> int:
    try:
        args = build_parser().parse_args(argv)
        root = resolve_root(args.root, create=args.command == "open")
        if args.command in {"open", "save", "save-resources", "consolidate", "transition", "close", "bind", "retire", "publish"}:
            args.identity = effective_git_identity(root.parent)
            args.note_locale = notes_locale(root.parent)
        if args.command == "open":
            return open_case(args, root)
        if args.command == "load":
            return load_case(args, root)
        if args.command == "snapshot":
            return snapshot_case(args)
        if args.command == "list":
            return list_cases(root)
        if args.command == "retire":
            return retire_case(args, root)
        if args.command == "publish":
            return publish_case(args, root)
        if args.command == "transition":
            return transition_case(args, root)
        if args.command == "close":
            return close_case(args, root)
        if args.command == "bind":
            return bind_case(args, root)
        if args.command == "save":
            return save_case(args, root)
        if args.command == "save-resources":
            return save_resources(args, root)
        if args.command == "consolidate":
            return consolidate_case(args, root)
        if args.command == "validate":
            return validate_command(root)
        raise CaseError("unknown_command", "Unknown investigation command")
    except CaseError as error:
        emit(error.payload(), stream=sys.stderr)
        return error.exit_code
    except (OSError, UnicodeError, ValueError) as error:
        emit(
            CaseError(
                "operational_error",
                "The investigation filesystem operation failed",
                exit_code=4,
                error_type=type(error).__name__,
            ).payload(),
            stream=sys.stderr,
        )
        return 4


if __name__ == "__main__":
    raise SystemExit(main())
