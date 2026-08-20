#!/usr/bin/env python3
"""Transactional filesystem mechanics for local investigation case files."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import sys
import time
import unicodedata
import uuid
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any, Iterator


ID_PATTERN = re.compile(r"^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$")
KEY_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
SECRET_PATTERN = re.compile(
    r"PRIVATE KEY-----|"
    r"\b(?:password|passwd|pwd|secret|token|api[-_]?key)\s*[:=]\s*\S+|"
    r"\b(?:gh[pousr]_|github_pat_|sk-(?:proj-|svcacct-)?|xox[baprs]-)\S+",
    re.IGNORECASE,
)
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
STATUSES = {
    "intake",
    "investigating",
    "validating",
    "ready-to-export",
    "exported",
    "closed",
    "blocked",
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
REQUIRED_FIELDS = (
    "id",
    "title",
    "status",
    "created-at",
    "updated-at",
    "source-type",
    "source-ref",
    "requester-role",
    "export-intent",
)
REQUIRED_SECTIONS = (
    ("Original request", "Solicitud original"),
    ("Current state", "Estado actual", "Estado vigente"),
    ("References and attachments", "Referencias y adjuntos"),
    ("Evidence", "Evidencia"),
    ("Affected surfaces", "Superficies afectadas"),
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


def resolve_root(raw_root: Path) -> Path:
    if raw_root.is_symlink():
        raise CaseError("root_symlink", "The investigation root must not be a symlink")
    try:
        root = raw_root.resolve(strict=True)
    except OSError as error:
        raise CaseError("root_missing", "The investigation root does not exist") from error
    if root.name != ".investigations" or not root.is_dir():
        raise CaseError(
            "invalid_root",
            "The investigation root must be an existing .investigations directory",
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
            index += 1
            continue
        key, raw_value = match.group(1), match.group(2) or ""
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


def load_records(root: Path) -> list[CaseRecord]:
    records: list[CaseRecord] = []
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
        records.append(CaseRecord(record_path, text, parse_frontmatter(text)))
    return records


def acquire_lock(root: Path, timeout_seconds: float = 2.0) -> Path:
    lock = root / ".open.lock"
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


def render_case(args: argparse.Namespace, timestamp: str) -> str:
    template = (
        Path(__file__).resolve().parents[1] / "assets" / "investigation-template.md"
    ).read_text(encoding="utf-8")
    primary_source = args.source_ref[0] if args.source_ref else "unknown"
    request = args.request if args.request is not None else args.objective
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
        "<Preserve the safe original request verbatim.>": request,
        "- <timestamp> — Case file created.": f"- {timestamp} — Case file created.",
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
    return template


def timestamp_value(raw: str | None) -> str:
    if raw is not None:
        try:
            datetime.fromisoformat(raw)
        except ValueError as error:
            raise CaseError("invalid_timestamp", "Timestamp must be ISO-8601") from error
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
        args.request or "",
        *args.source_ref,
    )
    timestamp = timestamp_value(args.timestamp)

    with locked(root):
        records = load_records(root)
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
                        {"id": record.case_id, "path": str(record.path.parent)}
                        for record in sorted(definite, key=lambda item: item.case_id)
                    ],
                }
            )
            return 0
        target = require_child(root, root / args.id)
        if target.exists():
            raise CaseError("case_exists", "The investigation directory already exists")
        staging = root / f".open-{uuid.uuid4().hex}.tmp"
        require_child(root, staging)
        try:
            staging.mkdir(mode=0o700)
            (staging / "artifacts").mkdir(mode=0o700)
            (staging / "exports").mkdir(mode=0o700)
            (staging / "handoffs").mkdir(mode=0o700)
            atomic_write(
                staging / "investigation.md",
                render_case(args, timestamp).encode("utf-8"),
            )
            os.replace(staging, target)
        finally:
            if staging.exists():
                shutil.rmtree(staging)

    emit({"status": "created", "id": args.id, "path": str(target)})
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


def validate_root(root: Path, *, ignore_lock: bool = False) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []
    if not ignore_lock and (root / ".open.lock").exists():
        errors.append("live mutation lock exists")
    for path in root.iterdir():
        if path.name.startswith(".open-") and path.name.endswith(".tmp"):
            errors.append(f"incomplete staging directory: {path.name}")

    records = load_records(root)
    ids: dict[str, Path] = {}
    keys: dict[str, Path] = {}
    lineage: dict[str, Path] = {}
    active_ids = {record.case_id for record in records}
    for record in records:
        relative = record.path.relative_to(root)
        for field in REQUIRED_FIELDS:
            if not record.fields.get(field):
                errors.append(f"{relative}: missing {field}")
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
        else:
            warnings.append(f"{relative}: legacy record has no dedupe-key")

        purpose = str(record.fields.get("purpose", ""))
        if not purpose:
            warnings.append(f"{relative}: legacy record has no purpose")
        elif purpose not in PURPOSES:
            errors.append(f"{relative}: invalid purpose")

        vault_outcome = str(record.fields.get("vault-outcome", ""))
        if not vault_outcome:
            warnings.append(f"{relative}: legacy record has no vault-outcome")
        elif vault_outcome not in VAULT_OUTCOMES:
            errors.append(f"{relative}: invalid vault-outcome")

        learning_outcome = str(record.fields.get("learning-outcome", ""))
        if not learning_outcome:
            warnings.append(f"{relative}: legacy record has no learning-outcome")
        elif learning_outcome not in LEARNING_OUTCOMES:
            errors.append(f"{relative}: invalid learning-outcome")

        status = str(record.fields.get("status", ""))
        effective_status = status
        if status and status not in STATUSES:
            errors.append(f"{relative}: invalid status")
        elif status == "blocked":
            resume_to = str(record.fields.get("resume-to", ""))
            if resume_to not in STATUSES - {"blocked"}:
                errors.append(f"{relative}: blocked status requires a valid resume-to")
            else:
                effective_status = resume_to

        if purpose == "undecided" and effective_status != "intake":
            errors.append(f"{relative}: purpose undecided is allowed only during intake")
        if (
            vault_outcome == "not-evaluated"
            and effective_status not in {"intake", "investigating"}
        ):
            errors.append(
                f"{relative}: vault-outcome not-evaluated is allowed only during "
                "intake or investigating"
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
                if record.dedupe_key:
                    errors.append(f"{relative}: {message}")
                else:
                    warnings.append(f"{relative}: legacy record {message}")

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


def validate_command(root: Path) -> int:
    errors, warnings = validate_root(root)
    if errors:
        emit(
            {"status": "invalid", "errors": errors, "warnings": warnings},
            stream=sys.stderr,
        )
        return 2
    emit(
        {
            "status": "valid",
            "cases": len(load_records(root)),
            "warnings": warnings,
        }
    )
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
        records = {record.case_id: record for record in load_records(root)}
        if args.canonical not in records or args.retire not in records:
            raise CaseError(
                "case_not_found",
                "Canonical and retiring investigation IDs must both exist",
            )
        canonical = records[args.canonical]
        retiring = records[args.retire]
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
            updated = updated.rstrip()
            updated += (
                f"\n\n- {timestamp} — Consolidated `{args.retire}`; "
                f"archive `artifacts/consolidated/{args.retire}/`; "
                f"mapping `artifacts/{mapping.name}`; {learning_event}.\n"
            )
            atomic_write(canonical.path, updated.encode("utf-8"))
            shutil.rmtree(retiring_dir)
            removed = True
            errors, warnings = validate_root(root, ignore_lock=True)
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
    open_parser.add_argument("--request")
    open_parser.add_argument("--timestamp")

    consolidate_parser = commands.add_parser(
        "consolidate",
        help="archive and retire one semantically reconciled duplicate",
    )
    consolidate_parser.add_argument("--canonical", required=True)
    consolidate_parser.add_argument("--retire", required=True)
    consolidate_parser.add_argument("--timestamp")

    commands.add_parser("validate", help="validate case-file mechanics")
    return parser


def main(argv: list[str] | None = None) -> int:
    try:
        args = build_parser().parse_args(argv)
        root = resolve_root(args.root)
        if args.command == "open":
            return open_case(args, root)
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
