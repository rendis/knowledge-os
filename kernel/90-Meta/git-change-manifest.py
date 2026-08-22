#!/usr/bin/env python3
"""Build and validate deterministic per-path Git change manifests."""
from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from datetime import date
from pathlib import Path, PurePosixPath
from typing import Any, NoReturn


MANIFEST_FIELDS = {
    "version", "old_oid", "new_oid", "paths", "environment_configs", "credential_suspects",
}
ANALYSIS_FIELDS = {
    "version", "repository", "old_oid", "new_oid", "paths", "checklist",
    "claims", "nodes", "result", "blockers",
}
PATH_STATUSES = {"A": "added", "D": "deleted", "M": "modified", "T": "type-changed"}
DISPOSITIONS = {"relevant", "not-documentable", "blocked"}
DIMENSIONS = (
    "inputs", "outputs", "data", "business-behavior", "infrastructure", "deployment",
)
MINIMUM_SWEEP_QUESTIONS = {
    "inputs": (
        "http-openapi", "pubsub-subscriptions", "cron-cronjobs", "scheduler",
        "functions", "eventarc", "cloud-run", "application", "user",
    ),
    "outputs": (
        "publications", "outgoing-http", "data-writes", "integrations",
        "fire-and-forget", "retries", "dlq",
    ),
    "data": (
        "engine", "database-schema-dataset-bucket-collection", "read-targets",
        "write-targets", "migrations-entities", "upsert", "truncation",
        "bulk-deletion", "synchronize",
    ),
    "business-behavior": (
        "validations", "transformations", "states",
        "country-store-bu-brand-category-vendor", "flags", "deduplication",
        "idempotency",
    ),
    "infrastructure": (
        "runtime-project-by-environment", "deployment",
        "schedulers-subscriptions", "config-maps", "service-accounts",
        "secret-references",
    ),
    "deployment": (
        "event-input", "workflow-job", "environment-condition",
        "build-artifact", "deploy-action", "gcp-project",
        "platform-resource", "location", "namespace-workload",
        "manifest-overlay-values",
    ),
}
CHECK_STATUSES = {"checked", "not-applicable", "blocked"}
QUESTION_STATUSES = {"observed", "not-observed", "not-applicable", "blocked"}
NODE_ACTIONS = {"create", "update", "consolidate", "retire", "no-change"}
RESULTS = {"documentation-change", "traceability-only", "blocked", "no-change"}
REVIEW_FIELDS = {
    "version", "repository", "manifest_digest", "scaffold_digest",
    "analysis_digest", "verdict", "findings",
}
FINDING_FIELDS = {"target", "category", "reason", "nodes", "evidence"}
REVIEW_VERDICTS = {"accept", "revise", "blocked"}
GATE_WRITE_FIELDS = {
    "version", "code", "status", "repositories", "write_groups",
    "acknowledgements", "fallback_repositories", "pending_repositories",
}
GATE_REPOSITORY_FIELDS = {
    "repository", "verdict", "review_disposition", "result", "new_oid",
    "is_new", "declared_nodes", "affected_nodes", "write_nodes",
    "accepted_claim_ids", "rejected_claim_ids", "partial_accept",
    "analysis_fallback", "fallback_reason", "cursor_decision", "disposition",
}
GATE_WRITE_GROUP_FIELDS = {"group_id", "repositories", "nodes", "grants"}
GATE_GRANT_FIELDS = {"repository", "claim_id", "nodes"}
GATE_ACKNOWLEDGEMENT_FIELDS = {
    "repository", "new_oid", "decision", "branch", "analysis_date",
}
PROJECTION_FIELDS = {
    "version", "run_id", "gate_digest", "unit_id", "unit_type",
    "patch_digest", "base_files", "result_files", "grants",
}
CLOSED_PACKAGE_FIELDS = {
    "version", "code", "status", "repository", "new_oid", "manifest",
    "scaffold", "analysis", "review", "gate", "validation",
}
SYNC_PROCESS_PATTERN = re.compile(
    r"\b(?:la sync|la review|claims? (?:aceptad|rechazad)|por el gate|"
    r"conocimiento durable adicional publicado)\b",
    re.IGNORECASE,
)
CONTENT_KINDS = {"text", "binary", "gitlink"}
MANIFEST_VERSION = 1
ANALYSIS_VERSION = 2
REVIEW_VERSION = 3
GATE_VERSION = 2
PROJECTION_VERSION = 1
KNOWLEDGE_NODE_ROOTS = {
    "10-Sistemas", "15-Arquitectura", "20-Repos", "25-Topics",
    "30-Flujos", "40-Integraciones", "50-Glosario", "60-Operacion",
    "70-Aprendizajes",
}
OID_PATTERN = re.compile(r"^(?:[0-9a-f]{40}|[0-9a-f]{64})$")
DIGEST_PATTERN = re.compile(r"^[0-9a-f]{64}$")
PATCH_HUNK_PATTERN = re.compile(
    r"^@@ -(?P<old_start>[0-9]+)(?:,(?P<old_count>[0-9]+))? "
    r"\+(?P<new_start>[0-9]+)(?:,(?P<new_count>[0-9]+))? "
    r"@@(?: .*)?(?:\r?\n)?$"
)
EMPTY_TREE_OIDS = {
    "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
    hashlib.sha256(b"tree 0\0").hexdigest(),
}
ENV_PATTERN = re.compile(r"^cloudrun/(?P<environment>[^/]+)/\.env\.ya?ml$")
KUSTOMIZE_PATTERN = re.compile(r"^kustomization/(?P<environment>[^/]+)/.+$")
CREDENTIAL_PATTERN = re.compile(
    r"(?<![A-Za-z0-9_.-])['\"]?(?P<name>[A-Za-z0-9_.-]*"
    r"(?:password|passwd|pass|secret|token|api[_-]?key|private[_-]?key)"
    r"[A-Za-z0-9_.-]*)['\"]?\s*[:=]\s*(?P<rhs>[^,};\n]+)",
    re.IGNORECASE,
)
QUOTED_VALUE = re.compile(r"(?P<quote>['\"])(?P<value>.*?)(?P=quote)")
PYTHON_GETENV_PATTERN = re.compile(
    r"(?:os\.getenv|os\.environ\.get)\(\s*['\"](?P<name>[^'\"]+)['\"]"
    r"\s*,\s*(?P<default>['\"][^'\"]*['\"])\s*\)"
)
YAML_ENV_NAME_PATTERN = re.compile(
    r"^(?P<indent>\s*)-\s*name\s*:\s*['\"]?(?P<name>[^'\"#\s]+)['\"]?\s*$",
    re.IGNORECASE,
)
YAML_VALUE_PATTERN = re.compile(
    r"^\s*value\s*:\s*(?P<value>.+?)\s*$",
    re.IGNORECASE,
)
REFERENCE_MARKERS = (
    "process.env", "os.getenv", "system.getenv", "secretkeyref", "valuefrom",
    "secretmanager", "getsecret", "getenv", "vault:",
)
PLACEHOLDERS = {
    "changeme", "change-me", "example", "inherit", "placeholder",
    "redacted", "replace-me", "todo", "xxx", "undefined", "null", "none",
}
PLACEHOLDER_PREFIXES = ("dummy-", "fake-", "fixture-", "fixture_", "mock-", "test-")
SOURCE_SUFFIXES = {".cs", ".go", ".java", ".js", ".jsx", ".kt", ".py", ".rb", ".ts", ".tsx"}
REDACTION_MARKER = "[redacted]"
REDACTION_REASON = "Credential-bearing content was excluded from synchronization evidence."
NONBLOCKING_REASON = "Unavailable content was recorded as not observed; safe evidence remains eligible."
FALLBACK_REASON = "No safe qualifying claim remained after deterministic finalization."

class ManifestError(Exception):
    exit_code = 1

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code

class ContractError(ManifestError):
    exit_code = 2

class OperationalError(ManifestError):
    pass

class StableArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise ContractError("usage-error", "invalid command arguments")

def emit(payload: dict[str, Any]) -> None:
    text = json.dumps(payload, ensure_ascii=True, separators=(",", ":"))
    sys.stdout.write(text + "\n")

def run_git(
    repo: Path,
    *args: str,
    input_data: bytes | None = None,
) -> subprocess.CompletedProcess[bytes]:
    try:
        return subprocess.run(
            ["git", "-C", str(repo), *args],
            input=input_data,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
    except OSError as error:
        raise OperationalError("git-unavailable", "git could not be executed") from error

def require_repository(repo: Path) -> None:
    if not repo.is_dir() or run_git(repo, "rev-parse", "--git-dir").returncode:
        raise OperationalError("not-a-repository", "repository is not an available Git checkout")

def remote_repository_names(repo: Path) -> set[str]:
    require_repository(repo)
    remotes = run_git(repo, "remote")
    if remotes.returncode:
        raise OperationalError("git-failure", "git could not list repository remotes")
    try:
        remote_names = remotes.stdout.decode("utf-8", errors="strict").splitlines()
    except UnicodeDecodeError as error:
        raise OperationalError("invalid-git-output", "git returned invalid remote metadata") from error

    repository_names = set()
    for remote in remote_names:
        urls = run_git(repo, "remote", "get-url", "--all", remote)
        if urls.returncode:
            continue
        try:
            decoded_urls = urls.stdout.decode("utf-8", errors="strict").splitlines()
        except UnicodeDecodeError as error:
            raise OperationalError("invalid-git-output", "git returned invalid remote metadata") from error
        for url in decoded_urls:
            tail = re.split(r"[/\\:]", url.strip().rstrip("/"))[-1]
            if tail.casefold().endswith(".git"):
                tail = tail[:-4]
            if tail:
                repository_names.add(tail.casefold())
    return repository_names

def repository_checkout_issues(
    repo: Path,
    repository: Any,
) -> list[dict[str, str]]:
    if not isinstance(repository, str) or not repository.strip():
        return []
    declared = repository.strip()
    expected = {
        declared.casefold(),
        re.sub(r"^APP[0-9]{5}-", "", declared, flags=re.IGNORECASE).casefold(),
    }
    if remote_repository_names(repo).isdisjoint(expected):
        return [issue("repository-checkout-mismatch", "repository")]
    return []

def resolve_commit(repo: Path, candidate: str) -> str:
    result = run_git(repo, "rev-parse", "--verify", "--end-of-options", f"{candidate}^{{object}}")
    if result.returncode:
        raise ContractError("invalid-commit", "requested commit is unavailable")
    try:
        oid = result.stdout.decode("ascii", errors="strict").strip()
        object_type = run_git(repo, "cat-file", "-t", oid)
        kind = object_type.stdout.decode("ascii", errors="strict").strip()
    except UnicodeDecodeError as error:
        raise OperationalError("invalid-git-output", "git returned invalid object metadata") from error
    if object_type.returncode or kind != "commit":
        raise ContractError("invalid-commit", "requested object is not a commit")
    if OID_PATTERN.fullmatch(oid) is None:
        raise OperationalError("invalid-git-output", "git returned an invalid object id")
    return oid

def resolve_current_ref(repo: Path, current_ref: str) -> str:
    if (
        not isinstance(current_ref, str)
        or OID_PATTERN.fullmatch(current_ref) is not None
        or not current_ref.startswith(("refs/heads/", "refs/remotes/"))
        or run_git(repo, "check-ref-format", current_ref).returncode
    ):
        raise ContractError(
            "invalid-current-ref",
            "current_ref must be an explicit local or remote branch ref",
        )
    try:
        return resolve_commit(repo, current_ref)
    except ContractError as error:
        raise ContractError(
            "invalid-current-ref",
            "current_ref is not an available commit ref",
        ) from error

def parse_raw_diff(raw: bytes) -> list[dict[str, str]]:
    fields = raw.split(b"\0")
    if fields and fields[-1] == b"":
        fields.pop()
    if len(fields) % 2:
        raise OperationalError("invalid-git-output", "git returned an invalid raw diff")
    paths = []
    for index in range(0, len(fields), 2):
        header, encoded_path = fields[index:index + 2]
        if not header.startswith(b":") or b" " not in header:
            raise OperationalError("invalid-git-output", "git returned an invalid raw diff")
        status = header.rsplit(b" ", 1)[-1].decode("ascii", errors="strict")[:1]
        if status not in PATH_STATUSES:
            raise ContractError("unsupported-change-status", "git returned an unsupported status")
        try:
            path = encoded_path.decode("utf-8", errors="strict")
        except UnicodeDecodeError as error:
            raise ContractError("invalid-path-encoding", "a changed path is not valid UTF-8") from error
        paths.append({"path": path, "status": PATH_STATUSES[status]})
    paths.sort(key=lambda item: item["path"])
    if len(paths) != len({item["path"] for item in paths}):
        raise OperationalError("duplicate-git-path", "git returned duplicate changed paths")
    return paths

def empty_tree_oid(repo: Path) -> str:
    result = run_git(
        repo,
        "hash-object",
        "-t",
        "tree",
        "--stdin",
        input_data=b"",
    )
    if result.returncode:
        raise OperationalError("git-failure", "git could not identify the empty tree")
    try:
        oid = result.stdout.decode("ascii", errors="strict").strip()
    except UnicodeDecodeError as error:
        raise OperationalError("invalid-git-output", "git returned an invalid object id") from error
    if OID_PATTERN.fullmatch(oid) is None:
        raise OperationalError("invalid-git-output", "git returned an invalid object id")
    return oid

def new_repository_paths(repo: Path, new_oid: str) -> list[dict[str, str]]:
    result = run_git(repo, "ls-tree", "-r", "-z", "--full-tree", new_oid)
    if result.returncode:
        raise OperationalError("git-failure", "git could not list the new repository tree")
    records = result.stdout.split(b"\0")
    if records and records[-1] == b"":
        records.pop()
    paths = []
    for record_value in records:
        try:
            _, encoded_path = record_value.split(b"\t", 1)
            path = encoded_path.decode("utf-8", errors="strict")
        except (ValueError, UnicodeDecodeError) as error:
            raise ContractError(
                "invalid-path-encoding",
                "a repository path is not valid UTF-8",
            ) from error
        paths.append({
            "path": path,
            "status": "added",
            "content_kind": content_kind(repo, new_oid, path),
        })
    paths.sort(key=lambda item: item["path"])
    if len(paths) != len({item["path"] for item in paths}):
        raise OperationalError("duplicate-git-path", "git returned duplicate repository paths")
    return paths

def content_kind(repo: Path, oid: str, path: str) -> str:
    spec = f"{oid}:{path}"
    result = run_git(repo, "cat-file", "-t", spec)
    if result.returncode:
        raise OperationalError("invalid-git-output", "git could not resolve a changed object")
    kind = result.stdout.decode("ascii", errors="strict").strip()
    if kind == "commit":
        return "gitlink"
    if kind != "blob":
        raise ContractError("unsupported-path-object", "a changed path has an unsupported type")
    blob = run_git(repo, "show", spec)
    if blob.returncode:
        raise OperationalError("invalid-git-output", "git could not read a changed object")
    if b"\0" in blob.stdout:
        return "binary"
    try:
        blob.stdout.decode("utf-8", errors="strict")
    except UnicodeDecodeError:
        return "binary"
    return "text"

def environment_for(path: str) -> str | None:
    match = ENV_PATTERN.fullmatch(path) or KUSTOMIZE_PATTERN.fullmatch(path)
    if match and (
        ENV_PATTERN.fullmatch(path)
        or match.group("environment").casefold() != "base"
    ):
        return match.group("environment")
    return None

def environment_configs(paths: list[dict[str, str]]) -> list[dict[str, str]]:
    records = [
        {"environment": environment, "path": item["path"]}
        for item in paths
        if (environment := environment_for(item["path"])) is not None
    ]
    return sorted(records, key=lambda item: (item["path"], item["environment"]))

def is_placeholder(value: str) -> bool:
    normalized = value.strip().strip("'\"").strip().casefold()
    return (
        not normalized
        or normalized in PLACEHOLDERS
        or normalized.startswith(PLACEHOLDER_PREFIXES)
        or normalized.startswith("<") and normalized.endswith(">")
    )

def credential_name(value: str) -> bool:
    normalized = value.casefold()
    compact = re.sub(r"[_.-]", "", normalized)
    if compact.endswith(("url", "uri")):
        return False
    markers = ("password", "passwd", "secret", "token", "apikey", "privatekey")
    return any(marker in compact for marker in markers) or bool(
        re.search(r"(?:^|[_.-])pass(?:$|[_.-])", normalized)
    )

def literal_rhs(rhs: str, *, bare_reference: bool) -> bool:
    return bool(literal_values(rhs, bare_reference=bare_reference))

def literal_values(rhs: str, *, bare_reference: bool) -> set[str]:
    value = rhs.strip().rstrip(";, ")
    positions = [
        position
        for token in ("||", "??", "?:")
        if (position := value.find(token)) >= 0
    ]
    if positions:
        return literal_values(value[min(positions) + 2:], bare_reference=False)
    lowered = value.casefold()
    if "$" + "{" in lowered or any(marker in lowered for marker in REFERENCE_MARKERS):
        return set()
    if re.fullmatch(r"\$[A-Za-z_][A-Za-z0-9_]*", value):
        return set()
    if re.fullmatch(r"['\"]?\s*\{\{.+\}\}\s*['\"]?", value):
        return set()
    quoted = [match.group("value") for match in QUOTED_VALUE.finditer(value)]
    if quoted:
        return {item for item in quoted if not is_placeholder(item)}
    value = value.split(" #", 1)[0].strip()
    if is_placeholder(value):
        return set()
    if bare_reference and re.fullmatch(
        r"[A-Za-z_$][A-Za-z0-9_$.[\]()<>? |&]*", value
    ):
        return set()
    return {value} if value else set()

def safe_id_token_permission(name: str, rhs: str) -> bool:
    if name.casefold() != "id-token":
        return False
    value = rhs.split("#", 1)[0].strip().rstrip(";, ").strip("'\"").casefold()
    return value in {"read", "write", "none"}

def text_blob(repo: Path, oid: str, path: str) -> str | None:
    object_type = run_git(repo, "cat-file", "-t", f"{oid}:{path}")
    if object_type.returncode or object_type.stdout.strip() != b"blob":
        return None
    blob = run_git(repo, "show", f"{oid}:{path}")
    if blob.returncode or b"\0" in blob.stdout:
        return None
    try:
        return blob.stdout.decode("utf-8", errors="strict")
    except UnicodeDecodeError:
        return None

def scan_credential_text(path: str, text: str) -> set[tuple[str, int]]:
    records: set[tuple[str, int]] = set()
    pure_path = PurePosixPath(path)
    bare_reference = pure_path.suffix.casefold() in SOURCE_SUFFIXES
    lines = text.splitlines()
    if not bare_reference and credential_name(pure_path.name):
        for line_number, line in enumerate(lines, start=1):
            if (
                line.strip()
                and not line.lstrip().startswith(("#", "//", "*"))
                and literal_rhs(line, bare_reference=False)
            ):
                records.add((path, line_number))
                break
    for line_number, line in enumerate(lines, start=1):
        if line.lstrip().startswith(("#", "//", "*")):
            continue
        for match in PYTHON_GETENV_PATTERN.finditer(line):
            if (
                credential_name(match.group("name"))
                and literal_rhs(match.group("default"), bare_reference=False)
            ):
                records.add((path, line_number))
                break
        for match in CREDENTIAL_PATTERN.finditer(line):
            name = match.group("name")
            if (
                credential_name(name)
                and not safe_id_token_permission(name, match.group("rhs"))
                and literal_rhs(match.group("rhs"), bare_reference=bare_reference)
            ):
                records.add((path, line_number))
                break
    for index, line in enumerate(lines):
        name_match = YAML_ENV_NAME_PATTERN.match(line)
        if name_match is None or not credential_name(name_match.group("name")):
            continue
        base_indent = len(name_match.group("indent"))
        for candidate_index in range(index + 1, len(lines)):
            candidate = lines[candidate_index]
            if not candidate.strip() or candidate.lstrip().startswith("#"):
                continue
            if YAML_ENV_NAME_PATTERN.match(candidate):
                break
            if candidate.strip() and len(candidate) - len(candidate.lstrip()) <= base_indent:
                break
            if re.match(r"^\s*valueFrom\s*:", candidate, re.IGNORECASE):
                break
            value_match = YAML_VALUE_PATTERN.match(candidate)
            if value_match is not None:
                if (
                    literal_rhs(
                        value_match.group("value"),
                        bare_reference=False,
                    )
                ):
                    records.add((path, candidate_index + 1))
                break
    return records

def credential_suspects(
    repo: Path,
    old_oid: str,
    new_oid: str,
    paths: list[dict[str, str]],
) -> list[dict[str, Any]]:
    records: set[tuple[str, int]] = set()
    for item in paths:
        revisions = []
        if item["status"] in {"deleted", "modified", "type-changed"}:
            revisions.append(old_oid)
        if item["status"] in {"added", "modified", "type-changed"}:
            revisions.append(new_oid)
        for oid in revisions:
            text = text_blob(repo, oid, item["path"])
            if text is not None:
                records.update(scan_credential_text(item["path"], text))
    return [{"path": path, "line": line} for path, line in sorted(records)]

def credential_values_from_line(path: str, line: str) -> set[str]:
    values: set[str] = set()
    bare_reference = PurePosixPath(path).suffix.casefold() in SOURCE_SUFFIXES
    for match in PYTHON_GETENV_PATTERN.finditer(line):
        if credential_name(match.group("name")):
            values.update(literal_values(
                match.group("default"),
                bare_reference=False,
            ))
    for match in CREDENTIAL_PATTERN.finditer(line):
        name = match.group("name")
        if credential_name(name) and not safe_id_token_permission(
            name,
            match.group("rhs"),
        ):
            values.update(literal_values(
                match.group("rhs"),
                bare_reference=bare_reference,
            ))
    yaml_value = YAML_VALUE_PATTERN.match(line)
    if yaml_value is not None:
        values.update(literal_values(
            yaml_value.group("value"),
            bare_reference=False,
        ))
    if credential_name(PurePosixPath(path).name):
        values.update(literal_values(line, bare_reference=False))
    return values

def credential_literals(
    repo: Path,
    manifest: dict[str, Any],
) -> set[str]:
    values: set[str] = set()
    for suspect in manifest["credential_suspects"]:
        path, line_number = suspect["path"], suspect["line"]
        for oid in (manifest["old_oid"], manifest["new_oid"]):
            text = text_blob(repo, oid, path)
            if text is None:
                continue
            lines = text.splitlines()
            if line_number <= len(lines):
                values.update(credential_values_from_line(
                    path,
                    lines[line_number - 1],
                ))
    return values

def credential_exposure_issues(
    value: Any,
    literals: set[str],
    field: str = "$",
) -> list[dict[str, str]]:
    issues: list[dict[str, str]] = []
    if isinstance(value, str):
        if any(credential_literal_occurs(value, literal) for literal in literals):
            issues.append(issue("credential-value-exposed", field))
    elif isinstance(value, list):
        for index, item in enumerate(value):
            issues.extend(credential_exposure_issues(
                item,
                literals,
                f"{field}[{index}]",
            ))
    elif isinstance(value, dict):
        for key, item in value.items():
            issues.extend(credential_exposure_issues(
                item,
                literals,
                f"{field}.{key}",
            ))
    return issues

def credential_literal_occurs(value: str, literal: str) -> bool:
    if not literal:
        return False
    if len(literal) < 8:
        return re.search(
            rf"(?<![A-Za-z0-9]){re.escape(literal)}(?![A-Za-z0-9])",
            value,
        ) is not None
    return literal in value

def contains_credential_literal(value: Any, literals: set[str]) -> bool:
    return bool(credential_exposure_issues(value, literals))

def redact_credential_literals(value: Any, literals: set[str]) -> Any:
    if isinstance(value, str):
        for literal in sorted(literals, key=lambda item: (-len(item), item)):
            if not literal:
                continue
            if len(literal) < 8:
                value = re.sub(
                    rf"(?<![A-Za-z0-9]){re.escape(literal)}(?![A-Za-z0-9])",
                    REDACTION_MARKER,
                    value,
                )
            else:
                value = value.replace(literal, REDACTION_MARKER)
        return value
    if isinstance(value, list):
        return [redact_credential_literals(item, literals) for item in value]
    if isinstance(value, dict):
        return {
            key: redact_credential_literals(item, literals)
            for key, item in value.items()
        }
    return value

def claim_credential_surface(claim: Any) -> Any:
    if not isinstance(claim, dict):
        return claim
    evidence_items = claim.get("evidence")
    return {
        "claim_id": claim.get("claim_id"),
        "statement": claim.get("statement"),
        "evidence_anchors": [
            item.get("anchor")
            for item in evidence_items
            if isinstance(item, dict)
        ] if isinstance(evidence_items, list) else evidence_items,
    }

def analysis_credential_surface(analysis: dict[str, Any]) -> dict[str, Any]:
    paths = analysis.get("paths")
    checklist = analysis.get("checklist")
    nodes = analysis.get("nodes")
    claims = analysis.get("claims")
    return {
        "path_reasons": [
            item.get("reason") for item in paths if isinstance(item, dict)
        ] if isinstance(paths, list) else paths,
        "checklist": {
            dimension: {
                "reason": item.get("reason"),
                "evidence_anchors": [
                    evidence_item.get("anchor")
                    for evidence_item in item.get("evidence", [])
                    if isinstance(evidence_item, dict)
                ],
            }
            for dimension, item in checklist.items()
            if isinstance(item, dict)
        } if isinstance(checklist, dict) else checklist,
        "claims": [
            claim_credential_surface(item) for item in claims
        ] if isinstance(claims, list) else claims,
        "nodes": [
            {"reason": item.get("reason")}
            for item in nodes
            if isinstance(item, dict)
        ] if isinstance(nodes, list) else nodes,
        "blockers": analysis.get("blockers"),
    }

def review_credential_surface(review: dict[str, Any]) -> dict[str, Any]:
    findings = review.get("findings")
    return {
        "findings": [
            {
                "reason": item.get("reason"),
                "evidence_anchor": (
                    item.get("evidence", {}).get("anchor")
                    if isinstance(item.get("evidence"), dict)
                    else item.get("evidence")
                ),
            }
            for item in findings
            if isinstance(item, dict)
        ] if isinstance(findings, list) else findings,
    }

def redact_analysis_text(
    analysis: dict[str, Any],
    literals: set[str],
) -> dict[str, Any]:
    finalized = copy.deepcopy(analysis)
    paths = finalized.get("paths")
    if isinstance(paths, list):
        for item in paths:
            if isinstance(item, dict) and "reason" in item:
                item["reason"] = redact_credential_literals(
                    item["reason"], literals
                )
    checklist = finalized.get("checklist")
    if isinstance(checklist, dict):
        for item in checklist.values():
            if not isinstance(item, dict):
                continue
            if "reason" in item:
                item["reason"] = redact_credential_literals(
                    item["reason"], literals
                )
            evidence_items = item.get("evidence")
            if isinstance(evidence_items, list):
                for evidence_item in evidence_items:
                    if isinstance(evidence_item, dict) and "anchor" in evidence_item:
                        evidence_item["anchor"] = redact_credential_literals(
                            evidence_item["anchor"], literals
                        )
    nodes = finalized.get("nodes")
    if isinstance(nodes, list):
        for item in nodes:
            if isinstance(item, dict) and "reason" in item:
                item["reason"] = redact_credential_literals(
                    item["reason"], literals
                )
    return finalized

def claim_evidence_paths(claim: Any) -> set[str]:
    if not isinstance(claim, dict) or not isinstance(claim.get("evidence"), list):
        return set()
    return {
        item["path"]
        for item in claim["evidence"]
        if isinstance(item, dict) and valid_path(item.get("path"))
    }

def materialize_analysis_payload(
    candidate: dict[str, Any],
    scaffold: dict[str, Any],
) -> dict[str, Any]:
    """Project agent-owned semantic decisions onto the canonical scaffold."""
    materialized = copy.deepcopy(scaffold)

    candidate_paths = candidate.get("paths")
    if isinstance(candidate_paths, list):
        by_path = {
            item["path"]: item
            for item in candidate_paths
            if isinstance(item, dict) and valid_path(item.get("path"))
        }
        for target in materialized["paths"]:
            source = by_path.get(target["path"])
            if source is None:
                continue
            for field in ("disposition", "reason", "claim_ids"):
                if field in source:
                    target[field] = copy.deepcopy(source[field])

    candidate_checklist = candidate.get("checklist")
    if isinstance(candidate_checklist, dict):
        for dimension, target in materialized["checklist"].items():
            source = candidate_checklist.get(dimension)
            if not isinstance(source, dict):
                continue
            questions = source.get("questions")
            if isinstance(questions, dict):
                for question in target["questions"]:
                    if question in questions:
                        target["questions"][question] = copy.deepcopy(
                            questions[question]
                        )
            statuses = list(target["questions"].values())
            if statuses and all(status in QUESTION_STATUSES for status in statuses):
                target["status"] = (
                    "blocked"
                    if "blocked" in statuses
                    else "not-applicable"
                    if all(status == "not-applicable" for status in statuses)
                    else "checked"
                )
            for field in ("reason", "evidence"):
                if field in source:
                    target[field] = copy.deepcopy(source[field])

    candidate_claims = candidate.get("claims")
    if isinstance(candidate_claims, list):
        materialized["claims"] = [
            {
                field: copy.deepcopy(item[field])
                for field in ("claim_id", "statement", "evidence")
                if field in item
            }
            for item in candidate_claims
            if isinstance(item, dict)
        ]

    candidate_nodes = candidate.get("nodes")
    if isinstance(candidate_nodes, list):
        by_basename = {
            item["basename"]: item
            for item in candidate_nodes
            if isinstance(item, dict) and valid_basename(item.get("basename"))
        }
        seed_nodes = {
            item["basename"]: item for item in materialized["nodes"]
        }
        nodes = []
        for basename in sorted(set(seed_nodes).union(by_basename)):
            target = copy.deepcopy(seed_nodes.get(basename, {
                "basename": basename,
                "action": "no-change",
                "reason": "",
                "claim_ids": [],
            }))
            source = by_basename.get(basename)
            if source is not None:
                for field in ("action", "reason", "claim_ids"):
                    if field in source:
                        target[field] = copy.deepcopy(source[field])
            nodes.append(target)
        materialized["nodes"] = nodes

    materialized["result"] = copy.deepcopy(candidate.get("result", ""))
    materialized["blockers"] = copy.deepcopy(candidate.get("blockers", []))
    return materialized

def unique_issues(items: list[dict[str, str]]) -> list[dict[str, str]]:
    return [
        {"code": code, "field": field}
        for code, field in sorted({(item["code"], item["field"]) for item in items})
    ]

def finalize_analysis_payload(
    analysis: dict[str, Any],
    manifest: dict[str, Any],
    sensitive_literals: set[str],
) -> tuple[dict[str, Any], int]:
    suspect_paths = {
        item["path"] for item in manifest["credential_suspects"]
    }
    raw_claims = analysis.get("claims")
    safe_claims = []
    if isinstance(raw_claims, list):
        for claim in raw_claims:
            if (
                contains_credential_literal(
                    claim_credential_surface(claim), sensitive_literals
                )
                or claim_evidence_paths(claim).intersection(suspect_paths)
            ):
                continue
            safe_claims.append(copy.deepcopy(claim))

    finalized = redact_analysis_text(analysis, sensitive_literals)
    finalized["claims"] = safe_claims
    safe_claim_ids = {
        item["claim_id"]
        for item in safe_claims
        if isinstance(item, dict)
        and isinstance(item.get("claim_id"), str)
        and item["claim_id"].strip()
    }

    decisions = finalized.get("paths")
    if isinstance(decisions, list):
        for item in decisions:
            if not isinstance(item, dict):
                continue
            references = item.get("claim_ids")
            if isinstance(references, list):
                item["claim_ids"] = sorted({
                    claim_id for claim_id in references
                    if claim_id in safe_claim_ids
                })
            if item.get("path") in suspect_paths:
                item["disposition"] = "not-documentable"
                item["reason"] = REDACTION_REASON
                item["claim_ids"] = []
            elif item.get("disposition") == "blocked":
                item["disposition"] = "not-documentable"
                item["reason"] = NONBLOCKING_REASON
                item["claim_ids"] = []

    checklist = finalized.get("checklist")
    if isinstance(checklist, dict):
        environment_paths = [
            item["path"] for item in manifest["environment_configs"]
        ]
        required_evidence = {
            "infrastructure": environment_paths,
            "deployment": [
                path for path in environment_paths
                if KUSTOMIZE_PATTERN.fullmatch(path)
            ],
        }
        for dimension, item in checklist.items():
            if not isinstance(item, dict):
                continue
            questions = item.get("questions")
            changed = False
            if isinstance(questions, dict):
                for question, status in questions.items():
                    if status == "blocked":
                        questions[question] = "not-observed"
                        changed = True
            statuses = (
                list(questions.values())
                if isinstance(questions, dict)
                else []
            )
            if statuses and all(status in QUESTION_STATUSES for status in statuses):
                item["status"] = (
                    "not-applicable"
                    if statuses and all(status == "not-applicable" for status in statuses)
                    else "checked"
                )
            if changed:
                item["reason"] = NONBLOCKING_REASON
            paths = required_evidence.get(dimension, [])
            if paths:
                evidence_by_path = {
                    evidence_item["path"]: evidence_item
                    for evidence_item in item.get("evidence", [])
                    if isinstance(evidence_item, dict)
                    and isinstance(evidence_item.get("path"), str)
                    and isinstance(evidence_item.get("anchor"), str)
                    and evidence_item["path"]
                    and evidence_item["anchor"]
                }
                for path in paths:
                    evidence_by_path.setdefault(
                        path,
                        {"path": path, "anchor": "exact commit path"},
                    )
                item["evidence"] = [
                    evidence_by_path[path] for path in sorted(evidence_by_path)
                ]

    nodes = finalized.get("nodes")
    if isinstance(nodes, list):
        for item in nodes:
            if not isinstance(item, dict):
                continue
            references = item.get("claim_ids")
            if isinstance(references, list):
                item["claim_ids"] = sorted({
                    claim_id for claim_id in references
                    if claim_id in safe_claim_ids
                })
            if item.get("action") == "no-change":
                item["claim_ids"] = []
            elif not item.get("claim_ids"):
                item["action"] = "no-change"
                item["reason"] = REDACTION_REASON

    write_claim_ids: set[str] = set()
    if isinstance(nodes, list):
        for item in nodes:
            if (
                isinstance(item, dict)
                and item.get("action") != "no-change"
                and isinstance(item.get("claim_ids"), list)
            ):
                write_claim_ids.update(item["claim_ids"])
    safe_claims = [
        item for item in safe_claims
        if isinstance(item, dict) and item.get("claim_id") in write_claim_ids
    ]
    safe_claim_ids = {item["claim_id"] for item in safe_claims}
    finalized["claims"] = safe_claims
    for collection in (finalized.get("paths"), finalized.get("nodes")):
        if not isinstance(collection, list):
            continue
        for item in collection:
            if isinstance(item, dict) and isinstance(item.get("claim_ids"), list):
                item["claim_ids"] = sorted({
                    claim_id for claim_id in item["claim_ids"]
                    if claim_id in safe_claim_ids
                })

    finalized["blockers"] = []
    if safe_claims:
        finalized["result"] = "documentation-change"
    else:
        repository = finalized.get("repository")
        local_basename = (
            repository_basename(repository)
            if isinstance(repository, str) and valid_repository_name(repository)
            else None
        )
        new_repository = is_new_manifest(manifest)
        finalized["result"] = "no-change" if new_repository else "traceability-only"
        if isinstance(nodes, list):
            for item in nodes:
                if not isinstance(item, dict):
                    continue
                item["action"] = (
                    "update"
                    if not new_repository and item.get("basename") == local_basename
                    else "no-change"
                )
                item["reason"] = REDACTION_REASON
                item["claim_ids"] = []
        if isinstance(decisions, list):
            for item in decisions:
                if isinstance(item, dict):
                    item["claim_ids"] = []

    original_claim_count = len(raw_claims) if isinstance(raw_claims, list) else 0
    return finalized, original_claim_count - len(safe_claims)

def fallback_analysis_payload(
    scaffold: dict[str, Any],
    manifest: dict[str, Any],
) -> dict[str, Any]:
    fallback = redact_credential_literals(scaffold, set())
    if is_empty_new_manifest(manifest):
        return fallback

    manifest_paths = [item["path"] for item in manifest["paths"]]
    no_changed_paths = not manifest_paths
    first_path = manifest_paths[0] if manifest_paths else ""
    environment_paths = [
        item["path"] for item in manifest["environment_configs"]
    ]
    kustomize_paths = [
        path for path in environment_paths
        if KUSTOMIZE_PATTERN.fullmatch(path)
    ]
    for item in fallback["paths"]:
        item["disposition"] = "not-documentable"
        item["reason"] = FALLBACK_REASON
        item["claim_ids"] = []
    for dimension, item in fallback["checklist"].items():
        for question in item["questions"]:
            item["questions"][question] = (
                "not-applicable" if no_changed_paths else "not-observed"
            )
        item["status"] = "not-applicable" if no_changed_paths else "checked"
        item["reason"] = FALLBACK_REASON
        evidence_paths = (
            []
            if no_changed_paths
            else environment_paths
            if dimension == "infrastructure" and environment_paths
            else kustomize_paths
            if dimension == "deployment" and kustomize_paths
            else [first_path]
        )
        item["evidence"] = [
            {"path": path, "anchor": "exact commit path"}
            for path in evidence_paths
        ]
    fallback["claims"] = []
    repository = fallback["repository"]
    local_basename = repository_basename(repository)
    new_repository = is_new_manifest(manifest)
    for item in fallback["nodes"]:
        item["action"] = (
            "update"
            if not new_repository and item["basename"] == local_basename
            else "no-change"
        )
        item["reason"] = FALLBACK_REASON
        item["claim_ids"] = []
    fallback["result"] = "no-change" if new_repository else "traceability-only"
    fallback["blockers"] = []
    return fallback

def replace_json(path: Path, value: dict[str, Any]) -> None:
    temporary_path: Path | None = None
    try:
        descriptor, raw_path = tempfile.mkstemp(
            prefix=f".{path.name}.",
            suffix=".tmp",
            dir=path.parent,
        )
        temporary_path = Path(raw_path)
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            json.dump(value, handle, ensure_ascii=True, separators=(",", ":"))
            handle.write("\n")
        os.replace(temporary_path, path)
        temporary_path = None
    except OSError as error:
        raise OperationalError(
            "file-write-error",
            "JSON result could not be persisted",
        ) from error
    finally:
        if temporary_path is not None:
            try:
                temporary_path.unlink()
            except FileNotFoundError:
                pass

def path_is_within(path: Path, root: Path) -> bool:
    try:
        path.resolve().relative_to(root.resolve())
        return True
    except ValueError:
        return False

def validate_closed_gate_output(output: Path, packages: list[Path]) -> None:
    if path_is_within(output, Path(__file__).resolve().parent.parent):
        raise ContractError(
            "gate-output-protected",
            "gate output must remain outside the installed kernel",
        )
    if output.resolve() in {path.resolve() for path in packages}:
        raise ContractError(
            "gate-output-overlap",
            "gate output must not replace a closed package",
        )

def validate_package_output(
    output: Path,
    repo: Path,
    artifacts: list[Path],
) -> None:
    if (
        path_is_within(output, Path(__file__).resolve().parent.parent)
        or path_is_within(output, repo)
    ):
        raise ContractError(
            "package-output-protected",
            "closed package output must remain outside the kernel and source repository",
        )
    if output.resolve() in {path.resolve() for path in artifacts}:
        raise ContractError(
            "package-output-overlap",
            "closed package output must not replace a semantic input",
        )

def validate_finalize_output(output: Path, analysis: Path) -> None:
    if output.name != "finalize-result.json" or output.parent.resolve() != analysis.parent.resolve():
        raise ContractError(
            "finalize-output-invalid",
            "finalize output must be finalize-result.json beside analysis.json",
        )
    if output.exists():
        raise ContractError(
            "finalize-already-recorded",
            "finalization receipt already exists; do not invoke the finalizer again",
        )

def stable_string_list(value: Any, validator: Any) -> bool:
    return (
        isinstance(value, list)
        and all(validator(item) for item in value)
        and value == sorted(set(value))
    )

def validate_gate_for_write(
    gate: dict[str, Any],
) -> tuple[set[str], bool, list[dict[str, str]]]:
    issues: list[dict[str, str]] = []
    if set(gate) != GATE_WRITE_FIELDS:
        return set(), False, [issue("gate-contract-invalid", "gate")]
    if (
        type(gate.get("version")) is not int
        or gate.get("version") != GATE_VERSION
    ):
        issues.append(issue("gate-contract-invalid", "gate.version"))
    if gate.get("code") != "batch-gated":
        issues.append(issue("gate-not-write-ready", "gate.code"))
    if gate.get("status") not in {"all-ready", "complete-no-write"}:
        issues.append(issue("gate-not-write-ready", "gate.status"))
    if gate.get("pending_repositories") != []:
        issues.append(issue("gate-not-write-ready", "gate.pending_repositories"))

    repositories = gate.get("repositories")
    ready_records: list[dict[str, Any]] = []
    repository_records: dict[str, dict[str, Any]] = {}
    if not isinstance(repositories, list):
        issues.append(issue("gate-contract-invalid", "gate.repositories"))
    else:
        names = []
        for index, item in enumerate(repositories):
            field = f"gate.repositories[{index}]"
            if not isinstance(item, dict) or set(item) != GATE_REPOSITORY_FIELDS:
                issues.append(issue("gate-contract-invalid", field))
                continue
            repository = item.get("repository")
            accepted_claim_ids = item.get("accepted_claim_ids")
            rejected_claim_ids = item.get("rejected_claim_ids")
            disposition = item.get("disposition")
            declared_nodes = item.get("declared_nodes")
            affected_nodes = item.get("affected_nodes")
            write_nodes = item.get("write_nodes")
            accepted_claim_ids_valid = stable_string_list(
                accepted_claim_ids,
                lambda value: isinstance(value, str) and bool(value.strip()),
            )
            rejected_claim_ids_valid = stable_string_list(
                rejected_claim_ids,
                lambda value: isinstance(value, str) and bool(value.strip()),
            )
            declared_nodes_valid = stable_string_list(
                declared_nodes,
                valid_basename,
            )
            affected_nodes_valid = stable_string_list(
                affected_nodes,
                valid_basename,
            )
            write_nodes_valid = stable_string_list(write_nodes, valid_basename)
            if not valid_repository_name(repository):
                issues.append(issue("gate-contract-invalid", f"{field}.repository"))
            else:
                names.append(repository)
                repository_records[repository] = item
            if not accepted_claim_ids_valid:
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.accepted_claim_ids",
                ))
            if not rejected_claim_ids_valid:
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.rejected_claim_ids",
                ))
            elif (
                accepted_claim_ids_valid
                and set(accepted_claim_ids).intersection(rejected_claim_ids)
            ):
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.accepted_claim_ids",
                ))
            if disposition not in {"write-ready", "cursor-ready"}:
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.disposition",
                ))
            for name, valid in (
                ("declared_nodes", declared_nodes_valid),
                ("affected_nodes", affected_nodes_valid),
                ("write_nodes", write_nodes_valid),
            ):
                if not valid:
                    issues.append(issue(
                        "gate-contract-invalid",
                        f"{field}.{name}",
                    ))
            if (
                write_nodes_valid
                and declared_nodes_valid
                and not set(write_nodes).issubset(declared_nodes)
            ):
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.write_nodes",
                ))
            if disposition == "cursor-ready" and write_nodes:
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.write_nodes",
                ))
            if disposition == "write-ready" and not write_nodes:
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.write_nodes",
                ))
            for name in ("is_new", "partial_accept", "analysis_fallback"):
                if type(item.get(name)) is not bool:
                    issues.append(issue(
                        "gate-contract-invalid",
                        f"{field}.{name}",
                    ))
            for name in ("fallback_reason", "cursor_decision"):
                if not isinstance(item.get(name), str):
                    issues.append(issue(
                        "gate-contract-invalid",
                        f"{field}.{name}",
                    ))
            if not valid_oid(item.get("new_oid")):
                issues.append(issue("gate-contract-invalid", f"{field}.new_oid"))
            if item.get("verdict") not in REVIEW_VERDICTS:
                issues.append(issue("gate-contract-invalid", f"{field}.verdict"))
            if item.get("result") not in RESULTS:
                issues.append(issue("gate-contract-invalid", f"{field}.result"))
            if not isinstance(item.get("review_disposition"), str):
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.review_disposition",
                ))
            if (
                disposition == "write-ready"
                and valid_repository_name(repository)
                and write_nodes_valid
                and bool(write_nodes)
            ):
                ready_records.append({
                    "repository": repository,
                    "write_nodes": write_nodes,
                    "grants": [],
                })
        if names != sorted(names) or len(names) != len(set(names)):
            issues.append(issue("gate-contract-invalid", "gate.repositories"))

    authorized_nodes: set[str] = set()
    groups = gate.get("write_groups")
    if not isinstance(groups, list):
        issues.append(issue("gate-contract-invalid", "gate.write_groups"))
    else:
        for index, item in enumerate(groups):
            field = f"gate.write_groups[{index}]"
            if not isinstance(item, dict) or set(item) != GATE_WRITE_GROUP_FIELDS:
                issues.append(issue("gate-contract-invalid", field))
                continue
            nodes = item["nodes"]
            group_repositories = item["repositories"]
            grants = item["grants"]
            if (
                not isinstance(nodes, list)
                or not nodes
                or any(not valid_basename(node) for node in nodes)
                or nodes != sorted(set(nodes))
            ):
                issues.append(issue("gate-contract-invalid", f"{field}.nodes"))
            else:
                authorized_nodes.update(nodes)
            if (
                not isinstance(group_repositories, list)
                or not group_repositories
                or any(
                    not valid_repository_name(repository)
                    for repository in group_repositories
                )
                or group_repositories != sorted(set(group_repositories))
            ):
                issues.append(issue(
                    "gate-contract-invalid",
                    f"{field}.repositories",
                ))
            valid_grants: list[dict[str, Any]] = []
            if not isinstance(grants, list):
                issues.append(issue("gate-contract-invalid", f"{field}.grants"))
            else:
                for grant_index, grant in enumerate(grants):
                    grant_field = f"{field}.grants[{grant_index}]"
                    if not isinstance(grant, dict) or set(grant) != GATE_GRANT_FIELDS:
                        issues.append(issue("gate-contract-invalid", grant_field))
                        continue
                    grant_repository = grant["repository"]
                    claim_id = grant["claim_id"]
                    grant_nodes = grant["nodes"]
                    record_item = repository_records.get(grant_repository)
                    record_accepted = (
                        record_item.get("accepted_claim_ids", [])
                        if record_item is not None
                        and isinstance(record_item.get("accepted_claim_ids"), list)
                        else []
                    )
                    record_rejected = (
                        record_item.get("rejected_claim_ids", [])
                        if record_item is not None
                        and isinstance(record_item.get("rejected_claim_ids"), list)
                        else []
                    )
                    record_write_nodes = (
                        record_item.get("write_nodes", [])
                        if record_item is not None
                        and isinstance(record_item.get("write_nodes"), list)
                        else []
                    )
                    repository_valid = not (
                        not valid_repository_name(grant_repository)
                        or not isinstance(group_repositories, list)
                        or grant_repository not in group_repositories
                        or record_item is None
                    )
                    if not repository_valid:
                        issues.append(issue(
                            "gate-grant-invalid",
                            f"{grant_field}.repository",
                        ))
                    claim_valid = not (
                        not isinstance(claim_id, str)
                        or not claim_id.strip()
                        or record_item is None
                        or claim_id not in record_accepted
                        or claim_id in record_rejected
                    )
                    if not claim_valid:
                        issues.append(issue(
                            "gate-grant-invalid",
                            f"{grant_field}.claim_id",
                        ))
                    nodes_valid = not (
                        not isinstance(grant_nodes, list)
                        or not grant_nodes
                        or any(not valid_basename(node) for node in grant_nodes)
                        or grant_nodes != sorted(set(grant_nodes))
                        or not isinstance(nodes, list)
                        or not set(grant_nodes).issubset(nodes)
                        or record_item is None
                        or not set(grant_nodes).issubset(record_write_nodes)
                    )
                    if not nodes_valid:
                        issues.append(issue(
                            "gate-grant-invalid",
                            f"{grant_field}.nodes",
                        ))
                    if repository_valid and claim_valid and nodes_valid:
                        valid_grants.append(grant)
                ordered_grants = sorted(
                    valid_grants,
                    key=lambda value: (
                        value.get("repository", ""),
                        value.get("claim_id", ""),
                        tuple(value.get("nodes", [])),
                    ),
                )
                if grants != ordered_grants or len(ordered_grants) != len({
                    (item.get("repository"), item.get("claim_id"))
                    for item in ordered_grants
                }):
                    issues.append(issue("gate-contract-invalid", f"{field}.grants"))
                if not grants:
                    issues.append(issue(
                        "gate-grant-invalid",
                        f"{field}.grants",
                    ))
                if isinstance(nodes, list) and {
                    node
                    for grant in valid_grants
                    for node in grant.get("nodes", [])
                } != set(nodes):
                    issues.append(issue("gate-grant-invalid", f"{field}.nodes"))
                if isinstance(group_repositories, list) and {
                    grant.get("repository") for grant in valid_grants
                } != set(group_repositories):
                    issues.append(issue(
                        "gate-grant-invalid",
                        f"{field}.repositories",
                    ))
        expected_groups = [
            {
                "group_id": item["group_id"],
                "repositories": item["repositories"],
                "nodes": item["nodes"],
            }
            for item in write_groups(ready_records)
        ]
        observed_groups = [
            {
                "group_id": item.get("group_id"),
                "repositories": item.get("repositories"),
                "nodes": item.get("nodes"),
            }
            for item in groups
            if isinstance(item, dict)
        ]
        if observed_groups != expected_groups:
            issues.append(issue("gate-contract-invalid", "gate.write_groups"))

    acknowledgements = gate.get("acknowledgements")
    has_acknowledgements = isinstance(acknowledgements, list) and bool(acknowledgements)
    if not isinstance(acknowledgements, list):
        issues.append(issue("gate-contract-invalid", "gate.acknowledgements"))
    else:
        expected_acknowledgements = {
            item["repository"]: {
                "new_oid": item["new_oid"],
                "decision": item["cursor_decision"],
            }
            for item in repositories
            if (
                isinstance(item, dict)
                and set(item) == GATE_REPOSITORY_FIELDS
                and item["disposition"] == "cursor-ready"
            )
        } if isinstance(repositories, list) else {}
        acknowledgement_names: list[str] = []
        for item in acknowledgements:
            if not isinstance(item, dict) or set(item) != GATE_ACKNOWLEDGEMENT_FIELDS:
                issues.append(issue("gate-contract-invalid", "gate.acknowledgements"))
                continue
            repository = item["repository"]
            acknowledgement_names.append(repository)
            expected_item = expected_acknowledgements.get(repository)
            try:
                parsed_date = date.fromisoformat(item["analysis_date"])
            except (TypeError, ValueError):
                parsed_date = None
            if (
                expected_item is None
                or item["new_oid"] != expected_item["new_oid"]
                or item["decision"] != expected_item["decision"]
                or item["branch"] not in {"main", "master"}
                or parsed_date is None
                or parsed_date.isoformat() != item["analysis_date"]
            ):
                issues.append(issue("gate-contract-invalid", "gate.acknowledgements"))
        if (
            acknowledgement_names != sorted(expected_acknowledgements)
            or len(acknowledgement_names) != len(set(acknowledgement_names))
        ):
            issues.append(issue("gate-contract-invalid", "gate.acknowledgements"))
    fallback_repositories = gate.get("fallback_repositories")
    if (
        not isinstance(fallback_repositories, list)
        or any(
            not valid_repository_name(repository)
            for repository in fallback_repositories
        )
        or fallback_repositories != sorted(set(fallback_repositories))
    ):
        issues.append(issue("gate-contract-invalid", "gate.fallback_repositories"))
    elif isinstance(repositories, list):
        expected_fallbacks = sorted(
            item["repository"]
            for item in repositories
            if (
                isinstance(item, dict)
                and set(item) == GATE_REPOSITORY_FIELDS
                and valid_repository_name(item["repository"])
                and isinstance(item["fallback_reason"], str)
                and item["fallback_reason"]
            )
        )
        if fallback_repositories != expected_fallbacks:
            issues.append(issue(
                "gate-contract-invalid",
                "gate.fallback_repositories",
            ))
    if (
        gate.get("status") == "all-ready"
        and not authorized_nodes
        and not has_acknowledgements
    ):
        issues.append(issue("gate-not-write-ready", "gate.status"))
    if (
        gate.get("status") == "complete-no-write"
        and (authorized_nodes or has_acknowledgements)
    ):
        issues.append(issue("gate-contract-invalid", "gate.status"))
    return authorized_nodes, has_acknowledgements, unique_issues(issues)

def patch_header_path(line: str) -> str | None:
    value = line[4:].rstrip("\r\n").split("\t", 1)[0]
    if value == "/dev/null":
        return None
    return value[2:] if value.startswith(("a/", "b/")) else value

def projection_invalid(issues: list[dict[str, str]]) -> dict[str, Any]:
    return {
        "code": "projection-invalid",
        "status": "blocked",
        "retryable": True,
        "resume_from": "projection",
        "issues": unique_issues(issues),
    }

def projection_file_digests(
    value: Any,
    field: str,
    issues: list[dict[str, str]],
) -> set[str]:
    paths: set[str] = set()
    if not isinstance(value, dict) or not value:
        issues.append(issue("projection-contract-invalid", field))
        return paths
    for path, digest in value.items():
        if not valid_path(path):
            issues.append(issue("projection-contract-invalid", f"{field}.{path}"))
        else:
            paths.add(path)
        if not isinstance(digest, str) or DIGEST_PATTERN.fullmatch(digest) is None:
            issues.append(issue("projection-contract-invalid", f"{field}.{path}"))
    return paths

def projected_patch_sections(
    patch_text: str,
) -> tuple[dict[str, dict[str, Any]], list[dict[str, str]]]:
    raw_lines = list(enumerate(patch_text.splitlines(keepends=True), start=1))
    lines: list[tuple[int, str]] = []
    issues: list[dict[str, str]] = []
    for line_number, line in raw_lines:
        if line.startswith("\\ No newline at end of file"):
            if not lines:
                issues.append(issue(
                    "patch-structure-invalid",
                    f"patch[{line_number}]",
                ))
            else:
                previous_number, previous = lines[-1]
                lines[-1] = (previous_number, previous.rstrip("\r\n"))
            continue
        lines.append((line_number, line))

    sections: dict[str, dict[str, Any]] = {}
    index = 0
    while index < len(lines):
        line_number, line = lines[index]
        if not line.strip():
            index += 1
            continue
        if not line.startswith("--- "):
            issues.append(issue(
                "patch-structure-invalid",
                f"patch[{line_number}]",
            ))
            index += 1
            continue
        old_path = patch_header_path(line)
        index += 1
        if index >= len(lines) or not lines[index][1].startswith("+++ "):
            issues.append(issue(
                "patch-structure-invalid",
                f"patch[{line_number}]",
            ))
            continue
        new_path = patch_header_path(lines[index][1])
        path = new_path or old_path
        index += 1
        if (
            path is None
            or not valid_path(path)
            or path in sections
            or old_path is not None
            and new_path is not None
            and old_path != new_path
        ):
            issues.append(issue(
                "patch-structure-invalid",
                f"patch[{line_number}]",
            ))

        hunks: list[dict[str, Any]] = []
        while index < len(lines):
            hunk_line_number, hunk_line = lines[index]
            if (
                hunk_line.startswith("--- ")
                and index + 1 < len(lines)
                and lines[index + 1][1].startswith("+++ ")
            ):
                break
            if not hunk_line.strip():
                index += 1
                continue
            match = PATCH_HUNK_PATTERN.fullmatch(hunk_line)
            if match is None:
                issues.append(issue(
                    "patch-structure-invalid",
                    f"patch[{hunk_line_number}]",
                ))
                index += 1
                continue
            hunk = {
                "old_start": int(match.group("old_start")),
                "old_count": int(match.group("old_count") or "1"),
                "new_start": int(match.group("new_start")),
                "new_count": int(match.group("new_count") or "1"),
                "lines": [],
            }
            old_remaining = hunk["old_count"]
            new_remaining = hunk["new_count"]
            index += 1
            while old_remaining or new_remaining:
                if index >= len(lines):
                    break
                content_line_number, content_line = lines[index]
                prefix = content_line[:1]
                if prefix == " ":
                    old_remaining -= 1
                    new_remaining -= 1
                elif prefix == "-":
                    old_remaining -= 1
                elif prefix == "+":
                    new_remaining -= 1
                    if SYNC_PROCESS_PATTERN.search(content_line[1:]):
                        issues.append(issue(
                            "sync-process-state-added",
                            f"patch[{content_line_number}]",
                        ))
                else:
                    issues.append(issue(
                        "patch-structure-invalid",
                        f"patch[{content_line_number}]",
                    ))
                    break
                if old_remaining < 0 or new_remaining < 0:
                    issues.append(issue(
                        "patch-structure-invalid",
                        f"patch[{content_line_number}]",
                    ))
                    break
                hunk["lines"].append(content_line)
                index += 1
            if old_remaining or new_remaining:
                issues.append(issue(
                    "patch-structure-invalid",
                    f"patch[{hunk_line_number}]",
                ))
            hunks.append(hunk)
        if not hunks:
            issues.append(issue("patch-structure-invalid", f"patch[{line_number}]"))
        if path is not None and valid_path(path) and path not in sections:
            sections[path] = {
                "old_path": old_path,
                "new_path": new_path,
                "hunks": hunks,
            }
    if not sections:
        issues.append(issue("patch-structure-invalid", "patch"))
    return sections, issues

def complete_patch_images(
    section: dict[str, Any],
) -> tuple[bytes, bytes] | None:
    hunks = section.get("hunks")
    if (
        section.get("old_path") is None and section.get("new_path") is None
        or not isinstance(hunks, list)
        or len(hunks) != 1
    ):
        return None
    hunk = hunks[0]
    old_count = hunk.get("old_count")
    new_count = hunk.get("new_count")
    if (
        type(old_count) is not int
        or type(new_count) is not int
        or hunk.get("old_start") != (0 if old_count == 0 else 1)
        or hunk.get("new_start") != (0 if new_count == 0 else 1)
        or not isinstance(hunk.get("lines"), list)
    ):
        return None
    old_lines: list[str] = []
    new_lines: list[str] = []
    for line in hunk["lines"]:
        if not isinstance(line, str) or not line:
            return None
        if line.startswith(" "):
            old_lines.append(line[1:])
            new_lines.append(line[1:])
        elif line.startswith("-"):
            old_lines.append(line[1:])
        elif line.startswith("+"):
            new_lines.append(line[1:])
        else:
            return None
    if len(old_lines) != old_count or len(new_lines) != new_count:
        return None
    return (
        "".join(old_lines).encode("utf-8"),
        "".join(new_lines).encode("utf-8"),
    )

def acknowledgement_document(
    raw: bytes,
) -> tuple[dict[str, dict[str, Any]], bool]:
    if not raw:
        return {}, True
    try:
        value = json.loads(
            raw.decode("utf-8", errors="strict"),
            object_pairs_hook=reject_duplicate_keys,
        )
    except (json.JSONDecodeError, UnicodeError, DuplicateJsonKey):
        return {}, False
    if (
        not isinstance(value, dict)
        or set(value) != {"version", "repositories"}
        or type(value.get("version")) is not int
        or value["version"] != 1
        or not isinstance(value.get("repositories"), list)
    ):
        return {}, False
    records: dict[str, dict[str, Any]] = {}
    names: list[str] = []
    for record_value in value["repositories"]:
        if (
            not isinstance(record_value, dict)
            or set(record_value) != {
                "repository", "branch", "analyzed_sha", "decision",
                "analysis_date",
            }
            or not valid_repository_name(record_value.get("repository"))
            or record_value.get("branch") not in {"main", "master"}
            or not isinstance(record_value.get("analyzed_sha"), str)
            or re.fullmatch(r"[0-9a-f]{12}", record_value["analyzed_sha"]) is None
            or not isinstance(record_value.get("decision"), str)
            or not record_value["decision"].strip()
        ):
            return {}, False
        try:
            parsed_date = date.fromisoformat(record_value.get("analysis_date"))
        except (TypeError, ValueError):
            return {}, False
        if parsed_date.isoformat() != record_value["analysis_date"]:
            return {}, False
        repository = record_value["repository"]
        names.append(repository)
        records[repository] = record_value
    canonical = json.dumps(
        value, ensure_ascii=True, sort_keys=True, separators=(",", ":"),
    ).encode("utf-8") + b"\n"
    return records, (
        names == sorted(names)
        and len(names) == len(set(names))
        and raw == canonical
    )

def validate_acknowledgement_projection(
    gate: dict[str, Any],
    projection: dict[str, Any],
    sections: dict[str, dict[str, Any]],
    issues: list[dict[str, str]],
) -> None:
    field = "projection.acknowledgements"
    section = sections.get("90-Meta/.sync-acknowledgements.json")
    images = complete_patch_images(section) if section is not None else None
    if images is None:
        issues.append(issue("acknowledgement-content-invalid", field))
        return
    old_bytes, new_bytes = images
    base_files = projection.get("base_files")
    result_files = projection.get("result_files")
    if (
        not isinstance(base_files, dict)
        or base_files.get("90-Meta/.sync-acknowledgements.json")
        != hashlib.sha256(old_bytes).hexdigest()
        or not isinstance(result_files, dict)
        or result_files.get("90-Meta/.sync-acknowledgements.json")
        != hashlib.sha256(new_bytes).hexdigest()
    ):
        issues.append(issue("acknowledgement-content-invalid", field))
        return
    prior, prior_valid = acknowledgement_document(old_bytes)
    observed, observed_valid = acknowledgement_document(new_bytes)
    if not prior_valid or not observed_valid:
        issues.append(issue("acknowledgement-content-invalid", field))
        return
    expected = dict(prior)
    for item in gate["acknowledgements"]:
        record_value = observed.get(item["repository"])
        if (
            record_value is None
            or record_value["branch"] != item["branch"]
            or record_value["analyzed_sha"] != item["new_oid"][:12]
            or record_value["decision"] != item["decision"]
            or record_value["analysis_date"] != item["analysis_date"]
        ):
            issues.append(issue("acknowledgement-content-invalid", field))
            return
        expected[item["repository"]] = record_value
    if observed != expected:
        issues.append(issue("acknowledgement-content-invalid", field))

def validate_projection(
    gate_path: Path,
    projection_path: Path,
    patch_path: Path,
) -> dict[str, Any]:
    gate = read_json(gate_path)
    try:
        projection = read_json(projection_path)
    except ContractError as error:
        if error.code != "invalid-json":
            raise
        return projection_invalid([
            issue("projection-contract-invalid", "projection"),
        ])
    try:
        patch_bytes = patch_path.read_bytes()
        patch_text = patch_bytes.decode("utf-8", errors="strict")
    except (OSError, UnicodeError) as error:
        raise OperationalError(
            "file-read-error",
            "projected patch could not be read",
        ) from error
    _, has_acknowledgements, gate_issues = validate_gate_for_write(gate)
    issues = prefix_issues(gate_issues, "gate")
    if gate_issues:
        return projection_invalid(issues)
    if set(projection) != PROJECTION_FIELDS:
        issues.append(issue("projection-contract-invalid", "projection"))
        return projection_invalid(issues)
    if (
        type(projection.get("version")) is not int
        or projection["version"] != PROJECTION_VERSION
    ):
        issues.append(issue("projection-contract-invalid", "projection.version"))
    if not valid_basename(projection.get("run_id")):
        issues.append(issue("projection-contract-invalid", "projection.run_id"))
    if (
        not isinstance(projection.get("gate_digest"), str)
        or projection["gate_digest"] != canonical_digest(gate)
    ):
        issues.append(issue("projection-gate-digest-mismatch", "projection.gate_digest"))
    if (
        not isinstance(projection.get("patch_digest"), str)
        or projection["patch_digest"] != hashlib.sha256(patch_bytes).hexdigest()
    ):
        issues.append(issue("projection-patch-digest-mismatch", "projection.patch_digest"))

    unit_type = projection.get("unit_type")
    unit_id = projection.get("unit_id")
    projection_grants = projection.get("grants")
    authorized_nodes: set[str] = set()
    allowed_acknowledgement_path = False
    if unit_type == "acknowledgements":
        if unit_id != "acknowledgements" or not has_acknowledgements:
            issues.append(issue("projection-unit-invalid", "projection.unit_id"))
        if projection_grants != []:
            issues.append(issue("projection-grants-invalid", "projection.grants"))
        allowed_acknowledgement_path = True
    elif unit_type == "write-group":
        group = next(
            (
                item
                for item in gate.get("write_groups", [])
                if isinstance(item, dict) and item.get("group_id") == unit_id
            ),
            None,
        )
        if group is None:
            issues.append(issue("projection-unit-invalid", "projection.unit_id"))
        else:
            authorized_nodes = set(group["nodes"])
            if projection_grants != group["grants"]:
                issues.append(issue("projection-grants-invalid", "projection.grants"))
    else:
        issues.append(issue("projection-unit-invalid", "projection.unit_type"))

    base_paths = projection_file_digests(
        projection.get("base_files"),
        "projection.base_files",
        issues,
    )
    result_paths = projection_file_digests(
        projection.get("result_files"),
        "projection.result_files",
        issues,
    )
    patch_sections, patch_issues = projected_patch_sections(patch_text)
    patch_paths = set(patch_sections)
    issues.extend(patch_issues)
    if base_paths != result_paths or patch_paths != result_paths:
        issues.append(issue("projection-file-binding-invalid", "projection.result_files"))
    for path, section in sorted(patch_sections.items()):
        images = complete_patch_images(section)
        if images is None:
            issues.append(issue("projection-file-binding-invalid", f"patch[{path}]"))
            continue
        old_bytes, new_bytes = images
        if (
            not isinstance(projection.get("base_files"), dict)
            or projection["base_files"].get(path)
            != hashlib.sha256(old_bytes).hexdigest()
            or not isinstance(projection.get("result_files"), dict)
            or projection["result_files"].get(path)
            != hashlib.sha256(new_bytes).hexdigest()
        ):
            issues.append(issue("projection-file-binding-invalid", f"patch[{path}]"))
    for path in sorted(patch_paths):
        parsed_path = PurePosixPath(path)
        allowed = (
            allowed_acknowledgement_path
            and path == "90-Meta/.sync-acknowledgements.json"
            or unit_type == "write-group"
            and path.endswith(".md")
            and len(parsed_path.parts) >= 2
            and parsed_path.parts[0] in KNOWLEDGE_NODE_ROOTS
            and parsed_path.stem in authorized_nodes
        )
        if not valid_path(path) or not allowed:
            issues.append(issue("projection-path-not-authorized", f"patch[{path}]"))
    if unit_type == "write-group" and {
        PurePosixPath(path).stem for path in patch_paths
    } != authorized_nodes:
        issues.append(issue(
            "projection-node-coverage-invalid",
            "projection.result_files",
        ))
    if unit_type == "acknowledgements":
        validate_acknowledgement_projection(
            gate,
            projection,
            patch_sections,
            issues,
        )
    return {
        "code": "projection-valid" if not issues else "projection-invalid",
        "status": "pass" if not issues else "blocked",
        **({} if not issues else {
            "retryable": True,
            "resume_from": "projection",
        }),
        "issues": unique_issues(issues),
    }

def build_manifest(repo: Path, old: str, new: str) -> dict[str, Any]:
    require_repository(repo)
    old_oid, new_oid = resolve_commit(repo, old), resolve_commit(repo, new)
    ancestry = run_git(repo, "merge-base", "--is-ancestor", old_oid, new_oid)
    if ancestry.returncode == 1:
        raise ContractError("non-ancestor", "old_oid is not an ancestor of new_oid")
    if ancestry.returncode:
        raise OperationalError("git-failure", "git could not validate ancestry")
    diff = run_git(
        repo, "diff", "--raw", "-z", "--no-renames", "--abbrev=64",
        old_oid, new_oid, "--",
    )
    if diff.returncode:
        raise OperationalError("git-failure", "git could not produce the raw diff")
    paths = parse_raw_diff(diff.stdout)
    paths = [
        {
            **item,
            "content_kind": content_kind(
                repo,
                old_oid if item["status"] == "deleted" else new_oid,
                item["path"],
            ),
        }
        for item in paths
    ]
    return {
        "version": 1,
        "old_oid": old_oid,
        "new_oid": new_oid,
        "paths": paths,
        "environment_configs": environment_configs(paths),
        "credential_suspects": credential_suspects(
            repo,
            old_oid,
            new_oid,
            paths,
        ),
    }

def build_new_manifest(repo: Path, new: str) -> dict[str, Any]:
    require_repository(repo)
    new_oid = resolve_commit(repo, new)
    paths = new_repository_paths(repo, new_oid)
    return {
        "version": 1,
        "old_oid": empty_tree_oid(repo),
        "new_oid": new_oid,
        "paths": paths,
        "environment_configs": environment_configs(paths),
        "credential_suspects": credential_suspects(
            repo,
            empty_tree_oid(repo),
            new_oid,
            paths,
        ),
    }

class DuplicateJsonKey(ValueError):
    pass

def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    value: dict[str, Any] = {}
    for key, item in pairs:
        if key in value:
            raise DuplicateJsonKey(key)
        value[key] = item
    return value

def read_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=reject_duplicate_keys,
        )
    except OSError as error:
        raise OperationalError("file-read-error", "input file could not be read") from error
    except (json.JSONDecodeError, UnicodeError, DuplicateJsonKey) as error:
        raise ContractError("invalid-json", "input is not valid UTF-8 JSON") from error
    if not isinstance(value, dict):
        raise ContractError("invalid-json", "input JSON must be an object")
    return value

def canonical_digest(value: dict[str, Any]) -> str:
    encoded = json.dumps(
        value,
        ensure_ascii=True,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()

def source_manifest_issues(
    repo: Path,
    manifest: dict[str, Any],
) -> list[dict[str, str]]:
    require_repository(repo)
    expected = (
        build_new_manifest(repo, manifest["new_oid"])
        if manifest["old_oid"] == empty_tree_oid(repo)
        else build_manifest(repo, manifest["old_oid"], manifest["new_oid"])
    )
    return (
        []
        if manifest == expected
        else [issue("manifest-source-mismatch", "manifest")]
    )

def path_exists_in_range(
    repo: Path,
    manifest: dict[str, Any],
    path: str,
) -> bool:
    return any(
        run_git(repo, "cat-file", "-e", f"{oid}:{path}").returncode == 0
        for oid in (manifest["old_oid"], manifest["new_oid"])
    )

def source_evidence_issues(
    repo: Path,
    manifest: dict[str, Any],
    analysis: dict[str, Any],
    review: dict[str, Any] | None = None,
) -> list[dict[str, str]]:
    candidates: list[tuple[str, Any]] = []
    checklist = analysis.get("checklist")
    if isinstance(checklist, dict):
        for dimension, value in checklist.items():
            if isinstance(value, dict) and isinstance(value.get("evidence"), list):
                for index, item in enumerate(value["evidence"]):
                    candidates.append((
                        f"checklist.{dimension}.evidence[{index}].path",
                        item.get("path") if isinstance(item, dict) else None,
                    ))
    claims = analysis.get("claims")
    if isinstance(claims, list):
        for claim_index, claim in enumerate(claims):
            if not isinstance(claim, dict) or not isinstance(claim.get("evidence"), list):
                continue
            for evidence_index, item in enumerate(claim["evidence"]):
                candidates.append((
                    f"claims[{claim_index}].evidence[{evidence_index}].path",
                    item.get("path") if isinstance(item, dict) else None,
                ))
    if isinstance(review, dict) and isinstance(review.get("findings"), list):
        for finding_index, finding in enumerate(review["findings"]):
            observed = finding.get("evidence") if isinstance(finding, dict) else None
            candidates.append((
                f"review.findings[{finding_index}].evidence.path",
                observed.get("path") if isinstance(observed, dict) else None,
            ))
    return [
        issue("evidence-path-unavailable", field)
        for field, path in candidates
        if valid_path(path) and not path_exists_in_range(repo, manifest, path)
    ]

def issue(code: str, field: str) -> dict[str, str]:
    return {"code": code, "field": field}

def record(
    value: Any,
    fields: set[str],
    code: str,
    field: str,
    issues: list[dict[str, str]],
) -> dict[str, Any] | None:
    if not isinstance(value, dict) or set(value) != fields:
        issues.append(issue(code, field))
        return None
    return value

def valid_oid(value: Any) -> bool:
    return isinstance(value, str) and OID_PATTERN.fullmatch(value) is not None

def valid_path(value: Any) -> bool:
    if (
        not isinstance(value, str)
        or not value
        or "\x00" in value
        or "\\" in value
    ):
        return False
    path = PurePosixPath(value)
    return (
        re.match(r"^[A-Za-z]:[\\/]", value) is None
        and not path.is_absolute()
        and ".." not in path.parts
    )

def valid_basename(value: Any) -> bool:
    return (
        isinstance(value, str)
        and bool(value.strip())
        and value == value.strip()
        and value not in {".", ".."}
        and "/" not in value
        and "\\" not in value
        and not any(ord(character) < 32 or ord(character) == 127 for character in value)
    )

def repository_basename(repository: str) -> str:
    return re.sub(r"^APP[0-9]{5}-", "", repository)

def valid_repository_name(repository: Any) -> bool:
    return (
        isinstance(repository, str)
        and valid_basename(repository)
        and valid_basename(repository_basename(repository))
    )

def evidence(
    value: Any,
    field: str,
    issues: list[dict[str, str]],
    *,
    allow_empty: bool = False,
) -> set[str]:
    paths = set()
    if not isinstance(value, list) or (not value and not allow_empty):
        issues.append(issue("evidence-required", field))
        return paths
    for index, raw in enumerate(value):
        item_field = f"{field}[{index}]"
        item = record(raw, {"path", "anchor"}, "invalid-evidence", item_field, issues)
        if item is None:
            continue
        if not valid_path(item["path"]):
            issues.append(issue("invalid-evidence-path", f"{item_field}.path"))
        else:
            paths.add(item["path"])
        if not isinstance(item["anchor"], str) or not item["anchor"].strip():
            issues.append(issue("invalid-evidence-anchor", f"{item_field}.anchor"))
    return paths

def claim_references(
    value: Any,
    field: str,
    references: list[tuple[str, str]],
    issues: list[dict[str, str]],
) -> None:
    if not isinstance(value, list) or any(not isinstance(item, str) for item in value):
        issues.append(issue("invalid-claim-references", field))
        return
    if len(value) != len(set(value)):
        issues.append(issue("duplicate-claim-reference", field))
    if value != sorted(value):
        issues.append(issue("claim-references-not-sorted", field))
    references.extend((claim_id, field) for claim_id in value)

def validate_manifest(
    manifest: dict[str, Any],
) -> tuple[set[str], set[str], set[str], list[dict[str, str]]]:
    issues = []
    if set(manifest) != MANIFEST_FIELDS:
        issues.append(issue("manifest-fields-invalid", "$"))
    if (
        type(manifest.get("version")) is not int
        or manifest.get("version") != MANIFEST_VERSION
    ):
        issues.append(issue("version-unsupported", "version"))
    for field in ("old_oid", "new_oid"):
        if not valid_oid(manifest.get(field)):
            issues.append(issue("invalid-oid", field))

    paths, statuses = [], {}
    raw_paths = manifest.get("paths")
    if not isinstance(raw_paths, list):
        issues.append(issue("invalid-paths", "paths"))
    else:
        for index, raw in enumerate(raw_paths):
            field = f"paths[{index}]"
            item = record(
                raw, {"path", "status", "content_kind"},
                "invalid-path-record", field, issues,
            )
            if item is None:
                continue
            path = item["path"]
            if not valid_path(path):
                issues.append(issue("invalid-path", f"{field}.path"))
            else:
                paths.append(path)
                statuses[path] = item["status"]
            if item["status"] not in PATH_STATUSES.values():
                issues.append(issue("invalid-path-status", f"{field}.status"))
            if not isinstance(item["content_kind"], str) or item["content_kind"] not in CONTENT_KINDS:
                issues.append(issue("invalid-content-kind", f"{field}.content_kind"))
        if len(paths) != len(set(paths)):
            issues.append(issue("duplicate-path", "paths"))
        if paths != sorted(paths):
            issues.append(issue("paths-not-sorted", "paths"))

    configs = []
    raw_configs = manifest.get("environment_configs")
    if not isinstance(raw_configs, list):
        issues.append(issue("invalid-environment-configs", "environment_configs"))
    else:
        for index, raw in enumerate(raw_configs):
            field = f"environment_configs[{index}]"
            item = record(
                raw, {"environment", "path"},
                "invalid-environment-config", field, issues,
            )
            if item is None:
                continue
            path, environment = item["path"], item["environment"]
            if not valid_path(path) or path not in statuses:
                issues.append(issue("invalid-environment-path", f"{field}.path"))
            if not isinstance(path, str) or not isinstance(environment, str) or environment_for(path) != environment:
                issues.append(issue("environment-config-contract-mismatch", field))
            if isinstance(path, str) and isinstance(environment, str):
                configs.append((path, environment))
        if len(configs) != len(set(configs)):
            issues.append(issue("duplicate-environment-config", "environment_configs"))
        if configs != sorted(configs):
            issues.append(issue("environment-configs-not-sorted", "environment_configs"))
        expected = {
            (item["path"], item["environment"])
            for item in environment_configs([{"path": path} for path in statuses])
        }
        if set(configs) != expected:
            issues.append(
                issue("environment-config-coverage-mismatch", "environment_configs")
            )

    suspects = []
    raw_suspects = manifest.get("credential_suspects")
    if not isinstance(raw_suspects, list):
        issues.append(issue("invalid-credential-suspects", "credential_suspects"))
    else:
        for index, raw in enumerate(raw_suspects):
            field = f"credential_suspects[{index}]"
            item = record(
                raw, {"path", "line"},
                "invalid-credential-suspect", field, issues,
            )
            if item is None:
                continue
            path, line = item["path"], item["line"]
            if not valid_path(path) or path not in statuses:
                issues.append(issue("invalid-credential-suspect-path", f"{field}.path"))
            if not isinstance(line, int) or isinstance(line, bool) or line < 1:
                issues.append(issue("invalid-credential-suspect-line", f"{field}.line"))
            if isinstance(path, str) and isinstance(line, int):
                suspects.append((path, line))
        if len(suspects) != len(set(suspects)):
            issues.append(issue("duplicate-credential-suspect", "credential_suspects"))
        if suspects != sorted(suspects):
            issues.append(issue("credential-suspects-not-sorted", "credential_suspects"))
    return (
        set(paths),
        {path for path, _ in configs},
        {path for path, _ in suspects},
        issues,
    )

def analysis_evidence(paths: list[str]) -> list[dict[str, str]]:
    selected = paths or [""]
    return [{"path": path, "anchor": ""} for path in selected]

def is_empty_new_manifest(manifest: dict[str, Any]) -> bool:
    return (
        not manifest["paths"]
        and manifest["old_oid"] in EMPTY_TREE_OIDS
    )

def is_new_manifest(manifest: dict[str, Any]) -> bool:
    return manifest["old_oid"] in EMPTY_TREE_OIDS

def build_analysis_scaffold(
    manifest: dict[str, Any],
    repository: str,
    requested_nodes: list[str],
) -> dict[str, Any]:
    repository = repository.strip()
    if not repository:
        raise ContractError("repository-required", "repository must be a non-empty name")
    if not valid_repository_name(repository):
        raise ContractError("invalid-repository-name", "repository must be a local name")
    local_repository_basename = repository_basename(repository)
    nodes = {local_repository_basename}
    for candidate in requested_nodes:
        basename = candidate.strip()
        if not valid_basename(basename):
            raise ContractError("invalid-node-basename", "node basenames must be local names")
        nodes.add(basename)

    environment_paths = [
        item["path"] for item in manifest["environment_configs"]
    ]
    kustomize_paths = [
        path for path in environment_paths if KUSTOMIZE_PATTERN.fullmatch(path)
    ]
    empty = is_empty_new_manifest(manifest)
    empty_reason = "repository tree contains no paths"
    checklist = {
        dimension: {
            "status": "not-applicable" if empty else "",
            "questions": {
                question: "not-applicable" if empty else ""
                for question in MINIMUM_SWEEP_QUESTIONS[dimension]
            },
            "reason": empty_reason if empty else "",
            "evidence": (
                []
                if empty
                else analysis_evidence(
                    environment_paths
                    if dimension == "infrastructure"
                    else kustomize_paths
                    if dimension == "deployment"
                    else []
                )
            ),
        }
        for dimension in DIMENSIONS
    }
    claims = [] if empty else [
        {
            "claim_id": "",
            "statement": "",
            "evidence": [{"path": "", "anchor": ""}],
        }
    ]
    return {
        "version": ANALYSIS_VERSION,
        "repository": repository,
        "old_oid": manifest["old_oid"],
        "new_oid": manifest["new_oid"],
        "paths": [
            {
                "path": item["path"],
                "disposition": "",
                "reason": "",
                "claim_ids": [],
            }
            for item in manifest["paths"]
        ],
        "checklist": checklist,
        "claims": claims,
        "nodes": [
            {
                "basename": basename,
                "action": "no-change",
                "reason": empty_reason if empty else "",
                "claim_ids": [],
            }
            for basename in sorted(nodes)
        ],
        "result": "no-change" if empty else "",
        "blockers": [],
    }

def initialize_analysis(
    manifest_path: Path,
    repository: str,
    requested_nodes: list[str],
) -> dict[str, Any]:
    manifest = read_json(manifest_path)
    _, _, _, issues = validate_manifest(manifest)
    if issues:
        raise ContractError("manifest-invalid", "manifest failed contract validation")
    return build_analysis_scaffold(manifest, repository, requested_nodes)

def validate_scaffold(
    scaffold: dict[str, Any],
    manifest: dict[str, Any],
) -> tuple[set[str], list[dict[str, str]]]:
    issues: list[dict[str, str]] = []
    if set(scaffold) != ANALYSIS_FIELDS:
        return set(), [issue("scaffold-fields-invalid", "scaffold")]
    repository = scaffold.get("repository")
    nodes = scaffold.get("nodes")
    if not isinstance(repository, str) or not repository.strip():
        return set(), [issue("scaffold-repository-invalid", "scaffold.repository")]
    if not isinstance(nodes, list):
        return set(), [issue("scaffold-nodes-invalid", "scaffold.nodes")]
    basenames: list[str] = []
    for item in nodes:
        if not isinstance(item, dict):
            return set(), [issue("scaffold-nodes-invalid", "scaffold.nodes")]
        basename = item.get("basename")
        if not isinstance(basename, str) or not basename.strip():
            return set(), [issue("scaffold-nodes-invalid", "scaffold.nodes")]
        basenames.append(basename)
    repository_basename = re.sub(r"^APP[0-9]{5}-", "", repository)
    requested_nodes = [
        basename for basename in basenames if basename != repository_basename
    ]
    try:
        expected = build_analysis_scaffold(
            manifest,
            repository,
            requested_nodes,
        )
    except ManifestError:
        return set(), [issue("scaffold-invalid", "scaffold")]
    if scaffold != expected:
        issues.append(issue("scaffold-mismatch", "scaffold"))
    return set(basenames), issues

def validate_analysis(
    analysis: dict[str, Any],
    manifest: dict[str, Any],
    manifest_paths: set[str],
    environment_paths: set[str],
    suspect_paths: set[str],
    sensitive_literals: set[str],
    scaffold: dict[str, Any],
    seed_nodes: set[str],
) -> list[dict[str, str]]:
    issues = []
    issues.extend(credential_exposure_issues(
        analysis_credential_surface(analysis),
        sensitive_literals,
    ))
    if set(analysis) != ANALYSIS_FIELDS:
        issues.append(issue("analysis-fields-invalid", "$"))
    if (
        type(analysis.get("version")) is not int
        or analysis.get("version") != ANALYSIS_VERSION
    ):
        issues.append(issue("version-unsupported", "version"))
    repository = analysis.get("repository")
    if not isinstance(repository, str) or not repository.strip():
        issues.append(issue("repository-required", "repository"))
    elif repository != scaffold.get("repository"):
        issues.append(issue("scaffold-identity-mismatch", "repository"))
    for field in ("old_oid", "new_oid"):
        if analysis.get(field) != manifest.get(field):
            issues.append(issue("identity-mismatch", field))

    references, paths, dispositions = [], [], {}
    decisions = analysis.get("paths")
    if not isinstance(decisions, list):
        issues.append(issue("invalid-paths", "paths"))
    else:
        for index, raw in enumerate(decisions):
            field = f"paths[{index}]"
            item = record(
                raw, {"path", "disposition", "reason", "claim_ids"},
                "invalid-path-decision", field, issues,
            )
            if item is None:
                continue
            path = item["path"]
            if not valid_path(path):
                issues.append(issue("invalid-path", f"{field}.path"))
            else:
                paths.append(path)
                dispositions[path] = item["disposition"]
            if not isinstance(item["disposition"], str) or item["disposition"] not in DISPOSITIONS:
                issues.append(issue("invalid-disposition", f"{field}.disposition"))
            if not isinstance(item["reason"], str) or not item["reason"].strip():
                issues.append(issue("reason-required", f"{field}.reason"))
            claim_references(
                item["claim_ids"], f"{field}.claim_ids", references, issues
            )
            if (
                isinstance(item["disposition"], str)
                and item["disposition"] in DISPOSITIONS
                and item["disposition"] != "relevant"
                and isinstance(item["claim_ids"], list)
                and item["claim_ids"]
            ):
                issues.append(issue(
                    "claim-path-disposition-mismatch",
                    f"{field}.claim_ids",
                ))
        if len(paths) != len(set(paths)):
            issues.append(issue("duplicate-path", "paths"))
        if paths != sorted(paths):
            issues.append(issue("analysis-paths-not-sorted", "paths"))
        if set(paths) != manifest_paths:
            issues.append(issue("path-coverage-mismatch", "paths"))
    infrastructure, deployment = set(), set()
    checklist = analysis.get("checklist")
    if not isinstance(checklist, dict) or set(checklist) != set(DIMENSIONS):
        issues.append(issue("checklist-incomplete", "checklist"))
    if isinstance(checklist, dict):
        for dimension in DIMENSIONS:
            field = f"checklist.{dimension}"
            item = record(
                checklist.get(dimension),
                {"status", "questions", "reason", "evidence"},
                "invalid-checklist-entry", field, issues,
            )
            if item is None:
                continue
            if not isinstance(item["status"], str) or item["status"] not in CHECK_STATUSES:
                issues.append(issue("invalid-check-status", f"{field}.status"))
            questions = item["questions"]
            expected_questions = MINIMUM_SWEEP_QUESTIONS[dimension]
            question_statuses = []
            questions_complete = (
                isinstance(questions, dict)
                and tuple(questions) == expected_questions
            )
            if not questions_complete:
                issues.append(issue(
                    "minimum-sweep-questions-incomplete",
                    f"{field}.questions",
                ))
            if isinstance(questions, dict):
                for question in expected_questions:
                    if question not in questions:
                        continue
                    status = questions[question]
                    if (
                        not isinstance(status, str)
                        or status not in QUESTION_STATUSES
                    ):
                        issues.append(issue(
                            "invalid-question-status",
                            f"{field}.questions.{question}",
                        ))
                    else:
                        question_statuses.append(status)
            if (
                questions_complete
                and len(question_statuses) == len(expected_questions)
                and isinstance(item["status"], str)
                and item["status"] in CHECK_STATUSES
            ):
                has_blocked = "blocked" in question_statuses
                all_not_applicable = all(
                    status == "not-applicable" for status in question_statuses
                )
                consistent = (
                    item["status"] == "blocked" and has_blocked
                    or item["status"] == "not-applicable" and all_not_applicable
                    or item["status"] == "checked"
                    and not has_blocked
                    and not all_not_applicable
                )
                if not consistent:
                    issues.append(issue(
                        "dimension-status-mismatch",
                        f"{field}.status",
                    ))
            if not isinstance(item["reason"], str) or not item["reason"].strip():
                issues.append(issue("reason-required", f"{field}.reason"))
            observed = evidence(
                item["evidence"],
                f"{field}.evidence",
                issues,
                allow_empty=(not manifest_paths and item.get("status") == "not-applicable"),
            )
            if dimension == "infrastructure":
                infrastructure = observed
            elif dimension == "deployment":
                deployment = observed
    for path in sorted(environment_paths - infrastructure):
        issues.append(issue(
            "environment-evidence-missing",
            f"checklist.infrastructure.evidence[{path}]",
        ))
    kustomize = {path for path in environment_paths if KUSTOMIZE_PATTERN.fullmatch(path)}
    for path in sorted(kustomize - deployment):
        issues.append(issue(
            "environment-deployment-evidence-missing",
            f"checklist.deployment.evidence[{path}]",
        ))

    claim_ids = []
    claims = analysis.get("claims")
    if not isinstance(claims, list):
        issues.append(issue("invalid-claims", "claims"))
    else:
        for index, raw in enumerate(claims):
            field = f"claims[{index}]"
            item = record(
                raw, {"claim_id", "statement", "evidence"},
                "invalid-claim", field, issues,
            )
            if item is None:
                continue
            claim_id = item["claim_id"]
            if not isinstance(claim_id, str) or not claim_id.strip():
                issues.append(issue("claim-id-required", f"{field}.claim_id"))
            else:
                claim_ids.append(claim_id)
            if not isinstance(item["statement"], str) or not item["statement"].strip():
                issues.append(issue("statement-required", f"{field}.statement"))
            claim_evidence = evidence(
                item["evidence"],
                f"{field}.evidence",
                issues,
            )
            for path in sorted(claim_evidence):
                if path in suspect_paths:
                    issues.append(issue(
                        "credential-suspect-claim-evidence",
                        f"{field}.evidence[{path}]",
                    ))
                if path in dispositions and dispositions[path] != "relevant":
                    issues.append(issue(
                        "claim-path-disposition-mismatch",
                        f"{field}.evidence[{path}]",
                    ))
        if len(claim_ids) != len(set(claim_ids)):
            issues.append(issue("duplicate-claim-id", "claims"))

    basenames, actions = [], []
    write_node_references: set[str] = set()
    write_nodes_without_claims: list[int] = []
    no_change_nodes_with_claims: list[int] = []
    nodes = analysis.get("nodes")
    if not isinstance(nodes, list) or not nodes:
        issues.append(issue("node-decision-required", "nodes"))
    else:
        for index, raw in enumerate(nodes):
            field = f"nodes[{index}]"
            item = record(
                raw, {"basename", "action", "reason", "claim_ids"},
                "invalid-node-decision", field, issues,
            )
            if item is None:
                continue
            basename, action = item["basename"], item["action"]
            if not valid_basename(basename):
                issues.append(issue("invalid-node-basename", f"{field}.basename"))
            else:
                basenames.append(basename)
            if not isinstance(action, str) or action not in NODE_ACTIONS:
                issues.append(issue("invalid-node-action", f"{field}.action"))
            else:
                actions.append(action)
            if not isinstance(item["reason"], str) or not item["reason"].strip():
                issues.append(issue("reason-required", f"{field}.reason"))
            claim_references(
                item["claim_ids"], f"{field}.claim_ids", references, issues
            )
            if isinstance(item["claim_ids"], list) and all(
                isinstance(claim_id, str) for claim_id in item["claim_ids"]
            ):
                if action == "no-change" and item["claim_ids"]:
                    no_change_nodes_with_claims.append(index)
                elif (
                    isinstance(action, str)
                    and action in NODE_ACTIONS
                    and action != "no-change"
                ):
                    if item["claim_ids"]:
                        write_node_references.update(item["claim_ids"])
                    else:
                        write_nodes_without_claims.append(index)
        if len(basenames) != len(set(basenames)):
            issues.append(issue("duplicate-node-decision", "nodes"))
        if basenames != sorted(basenames):
            issues.append(issue("node-decisions-not-sorted", "nodes"))
        if not seed_nodes.issubset(set(basenames)):
            issues.append(issue("scaffold-seed-node-missing", "nodes"))
    repository_basename = None
    if isinstance(repository, str) and repository.strip():
        repository_basename = re.sub(r"^APP[0-9]{5}-", "", repository)
        if repository_basename not in basenames:
            issues.append(issue("repository-node-missing", "nodes"))
    for claim_id, field in references:
        if claim_id not in set(claim_ids):
            issues.append(issue("unknown-claim-reference", field))
    for index in no_change_nodes_with_claims:
        issues.append(issue(
            "no-change-node-has-claims",
            f"nodes[{index}].claim_ids",
        ))

    result = analysis.get("result")
    if not isinstance(result, str) or result not in RESULTS:
        issues.append(issue("invalid-result", "result"))
    blocked = (
        any(
            isinstance(item, dict) and item.get("disposition") == "blocked"
            for item in decisions
        )
        if isinstance(decisions, list)
        else False
    )
    if isinstance(checklist, dict):
        blocked = blocked or any(
            isinstance(item, dict) and item.get("status") == "blocked"
            for item in checklist.values()
        )
    if blocked != (result == "blocked"):
        issues.append(issue("blocked-state-mismatch", "result"))
    if result == "documentation-change" and not claim_ids:
        issues.append(issue("result-claim-mismatch", "result"))
    if result in ("traceability-only", "blocked", "no-change") and claim_ids:
        issues.append(issue("result-claim-mismatch", "result"))
    if result == "documentation-change":
        if not any(action != "no-change" for action in actions):
            issues.append(issue("documentation-write-node-required", "nodes"))
        for index in write_nodes_without_claims:
            issues.append(issue(
                "write-node-claim-required",
                f"nodes[{index}].claim_ids",
            ))
        for claim_id in sorted(set(claim_ids) - write_node_references):
            issues.append(issue(
                "claim-write-node-reference-missing",
                f"claims.{claim_id}",
            ))
    if result == "blocked" and any(action != "no-change" for action in actions):
        issues.append(issue("blocked-node-action", "nodes"))
    if result == "no-change" and any(action != "no-change" for action in actions):
        issues.append(issue("no-change-node-action", "nodes"))
    if result == "no-change" and not is_new_manifest(manifest):
        issues.append(issue("no-change-requires-new-repository", "result"))
    if result == "traceability-only" and is_new_manifest(manifest):
        issues.append(issue("traceability-requires-existing-repository", "result"))
    if result == "traceability-only" and repository_basename is not None:
        for index, raw in enumerate(nodes if isinstance(nodes, list) else []):
            if not isinstance(raw, dict):
                continue
            basename, action = raw.get("basename"), raw.get("action")
            expected = "update" if basename == repository_basename else "no-change"
            if isinstance(action, str) and action in NODE_ACTIONS and action != expected:
                issues.append(issue(
                    "traceability-node-action-mismatch",
                    f"nodes[{index}].action",
                ))

    blockers = analysis.get("blockers")
    if not isinstance(blockers, list) or any(
        not isinstance(item, str) or not item.strip() for item in blockers
    ):
        issues.append(issue("invalid-blockers", "blockers"))
    elif result == "blocked" and not blockers:
        issues.append(issue("blocker-required", "blockers"))
    elif result in ("documentation-change", "traceability-only", "no-change") and blockers:
        issues.append(issue("unexpected-blockers", "blockers"))
    if is_empty_new_manifest(manifest) and analysis != scaffold:
        issues.append(issue("terminal-analysis-mismatch", "$"))
    return issues

def validate_review(
    review: dict[str, Any],
    repository: str,
    manifest: dict[str, Any],
    scaffold: dict[str, Any],
    analysis: dict[str, Any],
    sensitive_literals: set[str],
) -> list[dict[str, str]]:
    issues = []
    issues.extend(credential_exposure_issues(
        review_credential_surface(review),
        sensitive_literals,
    ))
    if set(review) != REVIEW_FIELDS:
        issues.append(issue("review-fields-invalid", "$"))
    if (
        type(review.get("version")) is not int
        or review.get("version") != REVIEW_VERSION
    ):
        issues.append(issue("review-version-unsupported", "version"))
    if review.get("repository") != repository:
        issues.append(issue("review-repository-mismatch", "repository"))
    expected_digests = {
        "manifest_digest": canonical_digest(manifest),
        "scaffold_digest": canonical_digest(scaffold),
        "analysis_digest": canonical_digest(analysis),
    }
    for field, expected in expected_digests.items():
        value = review.get(field)
        if not isinstance(value, str) or DIGEST_PATTERN.fullmatch(value) is None:
            issues.append(issue("invalid-review-digest", field))
        elif value != expected:
            issues.append(issue("review-artifact-mismatch", field))
    verdict = review.get("verdict")
    if not isinstance(verdict, str) or verdict not in REVIEW_VERDICTS:
        issues.append(issue("invalid-review-verdict", "verdict"))

    findings = review.get("findings")
    if not isinstance(findings, list):
        issues.append(issue("invalid-review-findings", "findings"))
        return issues
    for index, raw in enumerate(findings):
        field = f"findings[{index}]"
        item = record(raw, FINDING_FIELDS, "invalid-review-finding", field, issues)
        if item is None:
            continue
        for name in ("target", "category", "reason"):
            if not isinstance(item[name], str) or not item[name].strip():
                issues.append(issue(
                    "invalid-review-finding-value",
                    f"{field}.{name}",
                ))
        nodes = item["nodes"]
        if (
            not isinstance(nodes, list)
            or not nodes
            or any(not valid_basename(node) for node in nodes)
        ):
            issues.append(issue(
                "invalid-review-finding-nodes",
                f"{field}.nodes",
            ))
        elif len(nodes) != len(set(nodes)):
            issues.append(issue(
                "duplicate-review-finding-node",
                f"{field}.nodes",
            ))
        elif nodes != sorted(nodes):
            issues.append(issue(
                "review-finding-nodes-not-sorted",
                f"{field}.nodes",
            ))
        observed = record(
            item["evidence"],
            {"path", "anchor"},
            "invalid-review-evidence",
            f"{field}.evidence",
            issues,
        )
        if observed is None:
            continue
        if not valid_path(observed["path"]):
            issues.append(issue(
                "invalid-review-evidence-path",
                f"{field}.evidence.path",
            ))
        if (
            not isinstance(observed["anchor"], str)
            or not observed["anchor"].strip()
        ):
            issues.append(issue(
                "invalid-review-evidence-anchor",
                f"{field}.evidence.anchor",
            ))
    if verdict == "accept" and findings:
        issues.append(issue("accept-review-has-findings", "findings"))
    if verdict in {"revise", "blocked"} and not findings:
        issues.append(issue("nonaccept-review-needs-finding", "findings"))
    if analysis.get("result") == "blocked" and verdict == "accept":
        issues.append(issue("blocked-analysis-cannot-be-accepted", "verdict"))
    if is_empty_new_manifest(manifest) and (verdict != "accept" or findings):
        issues.append(issue("terminal-review-mismatch", "$"))
    return issues

def prefix_issues(
    issues: list[dict[str, str]],
    prefix: str,
) -> list[dict[str, str]]:
    return [
        {"code": item["code"], "field": f"{prefix}.{item['field']}"}
        for item in issues
    ]

def write_groups(
    records: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    remaining = sorted(records, key=lambda item: item["repository"])
    groups = []
    while remaining:
        seed = remaining.pop(0)
        repositories = {seed["repository"]}
        nodes = set(seed["write_nodes"])
        changed = True
        while changed:
            changed = False
            for candidate in list(remaining):
                if nodes.intersection(candidate["write_nodes"]):
                    remaining.remove(candidate)
                    repositories.add(candidate["repository"])
                    nodes.update(candidate["write_nodes"])
                    changed = True
        groups.append({
            "repositories": sorted(repositories),
            "nodes": sorted(nodes),
            "grants": sorted(
                (
                    grant
                    for record in records
                    if record["repository"] in repositories
                    for grant in record.get("grants", [])
                ),
                key=lambda item: (
                    item["repository"],
                    item["claim_id"],
                    tuple(item["nodes"]),
                ),
            ),
        })
    return [
        {
            "group_id": f"group-{index:03d}",
            **group,
        }
        for index, group in enumerate(groups, start=1)
    ]

def gate_batch_sources(
    items: list[list[Path]],
    expected_repositories: list[str],
    acknowledgement_metadata: dict[str, tuple[str, str]],
) -> dict[str, Any]:
    issues = []
    expected = []
    for index, raw in enumerate(expected_repositories):
        repository = raw.strip()
        if not repository or "\x00" in repository:
            issues.append(issue(
                "invalid-expected-repository",
                f"expected_repositories[{index}]",
            ))
        else:
            expected.append(repository)
    if len(expected) != len(set(expected)):
        issues.append(issue(
            "duplicate-expected-repository",
            "expected_repositories",
        ))
    records = []
    for index, item_paths in enumerate(items):
        repo_path, manifest_path, scaffold_path, analysis_path, review_path = item_paths
        manifest = read_json(manifest_path)
        scaffold = read_json(scaffold_path)
        analysis = read_json(analysis_path)
        review_failure = ""
        try:
            review = read_json(review_path)
        except ContractError as error:
            if error.code != "invalid-json":
                raise
            review = {}
            review_failure = "review-invalid"
        except OperationalError as error:
            if error.code != "file-read-error":
                raise
            review = {}
            review_failure = "review-missing"
        manifest_paths, environments, suspects, item_issues = validate_manifest(
            manifest
        )
        manifest_valid = not item_issues
        seed_nodes: set[str] = set()
        sensitive_literals: set[str] = set()
        item_issues += repository_checkout_issues(
            repo_path,
            analysis.get("repository"),
        )
        if not item_issues:
            item_issues += source_manifest_issues(repo_path, manifest)
            seed_nodes, scaffold_issues = validate_scaffold(scaffold, manifest)
            item_issues += scaffold_issues
            sensitive_literals = credential_literals(repo_path, manifest)
        if not item_issues:
            item_issues += validate_analysis(
                analysis,
                manifest,
                manifest_paths,
                environments,
                suspects,
                sensitive_literals,
                scaffold,
                seed_nodes,
            )
            item_issues += source_evidence_issues(
                repo_path,
                manifest,
                analysis,
            )
        analysis_fallback = (
            not item_issues
            and not is_empty_new_manifest(manifest)
            and analysis == fallback_analysis_payload(scaffold, manifest)
        )
        repository = analysis.get("repository")
        if manifest_valid and isinstance(repository, str) and repository.strip():
            if analysis_fallback:
                review_failure = "analysis-fallback"
            elif not review_failure:
                review_issues = validate_review(
                    review,
                    repository,
                    manifest,
                    scaffold,
                    analysis,
                    sensitive_literals,
                )
                review_issues += source_evidence_issues(
                    repo_path,
                    manifest,
                    analysis,
                    review,
                )
                if review_issues:
                    review_failure = "review-invalid"
        elif not isinstance(repository, str) or not repository.strip():
            item_issues.append(issue("repository-required", "repository"))
        issues.extend(prefix_issues(item_issues, f"items[{index}]"))
        if item_issues:
            continue
        nodes = analysis["nodes"]
        claim_ids = sorted(
            claim["claim_id"]
            for claim in analysis["claims"]
        )
        rejected_claim_ids: list[str] = []
        partial_accept = False
        if not review_failure and review["verdict"] == "revise":
            finding_claim_ids = []
            claim_targets_valid = bool(review["findings"])
            for finding in review["findings"]:
                target = finding["target"]
                if not target.startswith("claims."):
                    claim_targets_valid = False
                    break
                claim_id = target.removeprefix("claims.")
                if claim_id not in claim_ids:
                    claim_targets_valid = False
                    break
                finding_claim_ids.append(claim_id)
            if claim_targets_valid:
                rejected_claim_ids = sorted(set(finding_claim_ids))
                partial_accept = bool(
                    set(claim_ids).difference(rejected_claim_ids)
                )
        if review_failure or review.get("verdict") == "blocked":
            accepted_claim_ids = []
        elif review["verdict"] == "accept":
            accepted_claim_ids = claim_ids
        elif partial_accept:
            accepted_claim_ids = sorted(
                set(claim_ids).difference(rejected_claim_ids)
            )
        else:
            accepted_claim_ids = []
        accepted_claim_id_set = set(accepted_claim_ids)
        grant_nodes: dict[str, set[str]] = {}
        for node in nodes:
            if node["action"] == "no-change":
                continue
            for claim_id in accepted_claim_id_set.intersection(node["claim_ids"]):
                grant_nodes.setdefault(claim_id, set()).add(node["basename"])
        grants = [
            {
                "repository": repository,
                "claim_id": claim_id,
                "nodes": sorted(node_names),
            }
            for claim_id, node_names in sorted(grant_nodes.items())
        ]
        eligible_write_nodes = {
            node
            for grant in grants
            for node in grant["nodes"]
        }
        records.append({
            "repository": repository,
            "verdict": "blocked" if review_failure else review["verdict"],
            "review_disposition": (
                "blocked"
                if review_failure
                else "partial-accept"
                if partial_accept
                else review["verdict"]
            ),
            "result": analysis["result"],
            "new_oid": manifest["new_oid"],
            "is_new": is_new_manifest(manifest),
            "declared_nodes": sorted(item["basename"] for item in nodes),
            "affected_nodes": (
                []
                if review_failure
                else sorted({
                    basename
                    for finding in review["findings"]
                    for basename in finding["nodes"]
                })
            ),
            "write_nodes": sorted(eligible_write_nodes),
            "accepted_claim_ids": accepted_claim_ids,
            "rejected_claim_ids": rejected_claim_ids,
            "partial_accept": partial_accept,
            "analysis_fallback": analysis_fallback,
            "fallback_reason": review_failure,
            "grants": grants,
        })

    repositories = [item["repository"] for item in records]
    if len(repositories) != len(set(repositories)):
        issues.append(issue("duplicate-batch-repository", "items"))
    if set(repositories) != set(expected):
        issues.append(issue(
            "batch-repository-coverage-mismatch",
            "items",
        ))
    if issues:
        return {
            "version": GATE_VERSION,
            "code": "batch-invalid",
            "status": "blocked",
            "issues": issues,
        }

    repository_results = []
    ready = []
    for item in sorted(records, key=lambda value: value["repository"]):
        cursor_decision = ""
        if item["fallback_reason"]:
            cursor_decision = "inspection-limited"
        elif item["verdict"] == "revise" and not item["partial_accept"]:
            item["fallback_reason"] = "review-revise"
            cursor_decision = "review-rejected"
        elif item["verdict"] == "blocked":
            item["fallback_reason"] = "review-limited"
            cursor_decision = "inspection-limited"

        if cursor_decision:
            item["affected_nodes"] = []
            item["write_nodes"] = []
            disposition = "cursor-ready"
        elif (
            not item["write_nodes"]
        ):
            cursor_decision = "no-durable-node"
            disposition = "cursor-ready"
        else:
            disposition = "write-ready"
            ready.append(item)
        repository_results.append({
            **{
                key: value
                for key, value in item.items()
                if key != "grants"
            },
            "cursor_decision": cursor_decision,
            "disposition": disposition,
        })

    groups = write_groups(ready)
    acknowledgements = [
        {
            "repository": item["repository"],
            "new_oid": item["new_oid"],
            "decision": item["cursor_decision"],
            "branch": acknowledgement_metadata[item["repository"]][0],
            "analysis_date": acknowledgement_metadata[item["repository"]][1],
        }
        for item in repository_results
        if item["disposition"] == "cursor-ready"
    ]
    fallback_repositories = sorted(
        item["repository"]
        for item in repository_results
        if item["fallback_reason"]
    )
    status = (
        "all-ready"
        if groups or acknowledgements
        else "complete-no-write"
    )
    return {
        "version": GATE_VERSION,
        "code": "batch-gated",
        "status": status,
        "repositories": repository_results,
        "write_groups": groups,
        "acknowledgements": acknowledgements,
        "fallback_repositories": fallback_repositories,
        "pending_repositories": [],
    }

def validate_closed_package_value(
    package: dict[str, Any],
) -> list[dict[str, str]]:
    issues: list[dict[str, str]] = []
    if set(package) != CLOSED_PACKAGE_FIELDS:
        return [issue("package-fields-invalid", "package")]
    if (
        package.get("version") != 1
        or package.get("code") != "package-closed"
        or package.get("status") != "complete"
        or not valid_repository_name(package.get("repository"))
        or not valid_oid(package.get("new_oid"))
        or not all(isinstance(package.get(field), dict) for field in (
            "manifest", "scaffold", "analysis", "gate", "validation",
        ))
        or package.get("review") is not None
        and not isinstance(package["review"], dict)
    ):
        issues.append(issue("package-contract-invalid", "package"))
        return issues
    validation = package["validation"]
    expected_validation = {
        "manifest_digest": canonical_digest(package["manifest"]),
        "scaffold_digest": canonical_digest(package["scaffold"]),
        "analysis_digest": canonical_digest(package["analysis"]),
        "review_digest": canonical_digest(package["review"]),
        "gate_digest": canonical_digest(package["gate"]),
    }
    if validation != expected_validation:
        issues.append(issue("package-lineage-invalid", "package.validation"))
    gate = package["gate"]
    _, _, gate_issues = validate_gate_for_write(gate)
    issues.extend(prefix_issues(gate_issues, "package.gate"))
    repositories = gate.get("repositories")
    if (
        not isinstance(repositories, list)
        or len(repositories) != 1
        or not isinstance(repositories[0], dict)
        or repositories[0].get("repository") != package["repository"]
        or repositories[0].get("new_oid") != package["new_oid"]
        or package["manifest"].get("new_oid") != package["new_oid"]
        or package["analysis"].get("repository") != package["repository"]
    ):
        issues.append(issue("package-identity-invalid", "package"))
    if set(package["manifest"]) != MANIFEST_FIELDS:
        issues.append(issue("package-semantic-invalid", "package.manifest"))
    if set(package["scaffold"]) != ANALYSIS_FIELDS:
        issues.append(issue("package-semantic-invalid", "package.scaffold"))
    analysis = package["analysis"]
    if set(analysis) != ANALYSIS_FIELDS:
        issues.append(issue("package-semantic-invalid", "package.analysis"))
    review = package["review"]
    if review is not None and set(review) != REVIEW_FIELDS:
        issues.append(issue("package-semantic-invalid", "package.review"))
    claims = analysis.get("claims")
    claim_ids: set[str] = set()
    if not isinstance(claims, list):
        issues.append(issue("package-semantic-invalid", "package.analysis.claims"))
    else:
        for index, claim in enumerate(claims):
            if (
                not isinstance(claim, dict)
                or set(claim) != {"claim_id", "statement", "evidence"}
                or not isinstance(claim.get("claim_id"), str)
                or not claim["claim_id"].strip()
                or not isinstance(claim.get("statement"), str)
                or not claim["statement"].strip()
                or not isinstance(claim.get("evidence"), list)
                or not claim["evidence"]
            ):
                issues.append(issue(
                    "package-semantic-invalid",
                    f"package.analysis.claims[{index}]",
                ))
            else:
                claim_ids.add(claim["claim_id"])
    if isinstance(repositories, list) and len(repositories) == 1:
        accepted = repositories[0].get("accepted_claim_ids")
        if not isinstance(accepted, list) or not set(accepted).issubset(claim_ids):
            issues.append(issue(
                "package-authority-invalid",
                "package.gate.repositories[0].accepted_claim_ids",
            ))
    if review is not None and (
        review.get("repository") != package["repository"]
        or review.get("manifest_digest") != canonical_digest(package["manifest"])
        or review.get("scaffold_digest") != canonical_digest(package["scaffold"])
        or review.get("analysis_digest") != canonical_digest(analysis)
    ):
        issues.append(issue("package-lineage-invalid", "package.review"))
    has_write_authority = bool(gate.get("write_groups"))
    if has_write_authority and review is None:
        issues.append(issue(
            "package-review-required",
            "package.review",
        ))
    return unique_issues(issues)

def closed_package(path: Path) -> tuple[dict[str, Any], list[dict[str, str]]]:
    package = read_json(path)
    issues = validate_closed_package_value(package)
    return package, issues

def gate_batch(
    package_paths: list[Path],
    expected_repositories: list[str],
) -> dict[str, Any]:
    issues: list[dict[str, str]] = []
    expected = sorted(expected_repositories)
    if (
        not expected
        or len(expected) != len(set(expected))
        or any(not valid_repository_name(item) for item in expected)
    ):
        issues.append(issue("invalid-expected-repository", "expected_repositories"))
    records: list[dict[str, Any]] = []
    acknowledgements: list[dict[str, Any]] = []
    fallback_repositories: list[str] = []
    for index, path in enumerate(package_paths):
        try:
            package, package_issues = closed_package(path)
        except (ContractError, OperationalError) as error:
            package, package_issues = {}, [issue(error.code, "package")]
        issues.extend(prefix_issues(package_issues, f"packages[{index}]"))
        if package_issues:
            continue
        gate = package["gate"]
        record_value = copy.deepcopy(gate["repositories"][0])
        record_value["grants"] = sorted(
            (
                copy.deepcopy(grant)
                for group in gate["write_groups"]
                for grant in group["grants"]
                if grant["repository"] == package["repository"]
            ),
            key=lambda item: (
                item["repository"], item["claim_id"], tuple(item["nodes"]),
            ),
        )
        records.append(record_value)
        acknowledgements.extend(copy.deepcopy(gate["acknowledgements"]))
        fallback_repositories.extend(gate["fallback_repositories"])
    repositories = [item["repository"] for item in records]
    if sorted(repositories) != expected:
        issues.append(issue("batch-repository-coverage-mismatch", "packages"))
    if len(repositories) != len(set(repositories)):
        issues.append(issue("duplicate-batch-repository", "packages"))
    if issues:
        return {
            "version": GATE_VERSION,
            "code": "batch-invalid",
            "status": "blocked",
            "issues": unique_issues(issues),
        }
    ready = [item for item in records if item["disposition"] == "write-ready"]
    repository_results = [
        {key: value for key, value in item.items() if key != "grants"}
        for item in sorted(records, key=lambda value: value["repository"])
    ]
    acknowledgements = sorted(
        acknowledgements, key=lambda item: item["repository"]
    )
    result = {
        "version": GATE_VERSION,
        "code": "batch-gated",
        "status": "all-ready" if ready or acknowledgements else "complete-no-write",
        "repositories": repository_results,
        "write_groups": write_groups(ready),
        "acknowledgements": acknowledgements,
        "fallback_repositories": sorted(set(fallback_repositories)),
        "pending_repositories": [],
    }
    _, _, result_issues = validate_gate_for_write(result)
    if result_issues:
        return {
            "version": GATE_VERSION,
            "code": "batch-invalid",
            "status": "blocked",
            "issues": prefix_issues(result_issues, "gate"),
        }
    return result

def close_package(
    repo_path: Path,
    manifest_path: Path,
    scaffold_path: Path,
    analysis_path: Path,
    review_path: Path,
    branch: str,
    analysis_date: str,
) -> dict[str, Any]:
    manifest = read_json(manifest_path)
    scaffold = read_json(scaffold_path)
    analysis = read_json(analysis_path)
    try:
        review: dict[str, Any] | None = read_json(review_path)
    except (ContractError, OperationalError) as error:
        if error.code not in {"invalid-json", "file-read-error"}:
            raise
        review = None
    repository = analysis.get("repository")
    if not valid_repository_name(repository):
        raise ContractError("package-invalid", "package repository is invalid")
    try:
        parsed_date = date.fromisoformat(analysis_date)
    except (TypeError, ValueError) as error:
        raise ContractError("package-invalid", "analysis date is invalid") from error
    if branch not in {"main", "master"} or parsed_date.isoformat() != analysis_date:
        raise ContractError("package-invalid", "acknowledgement metadata is invalid")
    branch_oid = resolve_commit(repo_path, f"refs/heads/{branch}")
    if branch_oid != manifest.get("new_oid"):
        raise ContractError(
            "package-source-stale",
            "package branch no longer resolves to the analyzed source OID",
        )
    gate = gate_batch_sources(
        [[repo_path, manifest_path, scaffold_path, analysis_path, review_path]],
        [repository],
        {repository: (branch, analysis_date)},
    )
    if gate.get("status") == "blocked":
        return {
            "version": 1,
            "code": "package-invalid",
            "status": "blocked",
            "issues": gate.get("issues", []),
        }
    validation = {
        "manifest_digest": canonical_digest(manifest),
        "scaffold_digest": canonical_digest(scaffold),
        "analysis_digest": canonical_digest(analysis),
        "review_digest": canonical_digest(review),
        "gate_digest": canonical_digest(gate),
    }
    return {
        "version": 1,
        "code": "package-closed",
        "status": "complete",
        "repository": repository,
        "new_oid": manifest["new_oid"],
        "manifest": manifest,
        "scaffold": scaffold,
        "analysis": analysis,
        "review": review,
        "gate": gate,
        "validation": validation,
    }

def finalize_analysis(
    repo_path: Path,
    manifest_path: Path,
    scaffold_path: Path,
    analysis_path: Path,
) -> dict[str, Any]:
    manifest = read_json(manifest_path)
    scaffold = read_json(scaffold_path)
    invalid_agent_output = False
    input_issues: list[dict[str, str]] = []
    try:
        analysis = read_json(analysis_path)
    except (ContractError, OperationalError) as error:
        if error.code not in {"invalid-json", "file-read-error"}:
            raise
        analysis = {}
        invalid_agent_output = True
        input_issues.append(issue(error.code, "$"))
    paths, environments, suspects, issues = validate_manifest(manifest)
    issues += repository_checkout_issues(
        repo_path,
        scaffold.get("repository"),
    )
    seed_nodes: set[str] = set()
    sensitive_literals: set[str] = set()
    if not issues:
        issues += source_manifest_issues(repo_path, manifest)
        seed_nodes, scaffold_issues = validate_scaffold(scaffold, manifest)
        issues += scaffold_issues
        sensitive_literals = credential_literals(repo_path, manifest)
    if issues:
        return {
            "code": "analysis-finalization-invalid",
            "status": "blocked",
            "issues": issues,
        }

    binding_mismatch = any(
        field in analysis and analysis.get(field) != scaffold.get(field)
        for field in ("repository", "old_oid", "new_oid")
    )
    existing_fallback = (
        not invalid_agent_output
        and not is_empty_new_manifest(manifest)
        and analysis == fallback_analysis_payload(scaffold, manifest)
    )
    if not invalid_agent_output:
        input_issues += validate_analysis(
            analysis,
            manifest,
            paths,
            environments,
            suspects,
            sensitive_literals,
            scaffold,
            seed_nodes,
        )
        materialized = materialize_analysis_payload(analysis, scaffold)
    else:
        materialized = analysis

    ambiguous_candidate = any(
        item["code"] in {
            "duplicate-path",
            "duplicate-claim-id",
            "duplicate-node-decision",
            "path-coverage-mismatch",
        }
        for item in input_issues
    )
    if invalid_agent_output or binding_mismatch or ambiguous_candidate:
        finalized, dropped_claims = fallback_analysis_payload(scaffold, manifest), 0
    elif existing_fallback:
        finalized, dropped_claims = analysis, 0
    else:
        finalized, dropped_claims = finalize_analysis_payload(
            materialized,
            manifest,
            sensitive_literals,
        )
    candidate_issues = validate_analysis(
        finalized,
        manifest,
        paths,
        environments,
        suspects,
        sensitive_literals,
        scaffold,
        seed_nodes,
    )
    candidate_issues += source_evidence_issues(
        repo_path,
        manifest,
        finalized,
    )
    rejected_issues = list(candidate_issues)
    fallback_used = (
        invalid_agent_output
        or binding_mismatch
        or ambiguous_candidate
        or existing_fallback
        or bool(candidate_issues)
    )
    if (
        candidate_issues
        and not invalid_agent_output
        and not binding_mismatch
        and not ambiguous_candidate
    ):
        finalized = fallback_analysis_payload(scaffold, manifest)
        original_claims = analysis.get("claims")
        dropped_claims = (
            len(original_claims) if isinstance(original_claims, list) else 0
        )
        candidate_issues = validate_analysis(
            finalized,
            manifest,
            paths,
            environments,
            suspects,
            sensitive_literals,
            scaffold,
            seed_nodes,
        )
        candidate_issues += source_evidence_issues(
            repo_path,
            manifest,
            finalized,
        )
    issues += candidate_issues
    if issues:
        return {
            "code": "analysis-finalization-invalid",
            "status": "blocked",
            "issues": issues,
        }

    changed = invalid_agent_output or finalized != analysis
    if changed:
        replace_json(analysis_path, finalized)
    return {
        "code": "analysis-finalized",
        "status": "pass",
        "changed": changed,
        "dropped_claims": dropped_claims,
        "fallback_used": fallback_used,
        "input_issues": unique_issues(input_issues + rejected_issues),
    }

def check_analysis(
    repo_path: Path,
    manifest_path: Path,
    scaffold_path: Path,
    analysis_path: Path,
    current_ref: str,
) -> dict[str, Any]:
    manifest = read_json(manifest_path)
    scaffold = read_json(scaffold_path)
    analysis = read_json(analysis_path)
    paths, environments, suspects, issues = validate_manifest(manifest)
    issues += repository_checkout_issues(repo_path, analysis.get("repository"))
    seed_nodes: set[str] = set()
    sensitive_literals: set[str] = set()
    if not issues:
        issues += source_manifest_issues(repo_path, manifest)
        seed_nodes, scaffold_issues = validate_scaffold(scaffold, manifest)
        issues += scaffold_issues
        sensitive_literals = credential_literals(repo_path, manifest)
    if not issues:
        issues += validate_analysis(
            analysis,
            manifest,
            paths,
            environments,
            suspects,
            sensitive_literals,
            scaffold,
            seed_nodes,
        )
        issues += source_evidence_issues(repo_path, manifest, analysis)
    if issues:
        return {"code": "analysis-invalid", "status": "blocked", "issues": issues}
    current_oid = resolve_current_ref(repo_path, current_ref)
    if current_oid != manifest["new_oid"]:
        return {
            "code": "stale-head",
            "status": "blocked",
            "expected_oid": manifest["new_oid"],
            "current_oid": current_oid,
        }
    return {
        "code": "ok",
        "status": "pass",
        "manifest_digest": canonical_digest(manifest),
        "scaffold_digest": canonical_digest(scaffold),
        "analysis_digest": canonical_digest(analysis),
    }

def parser() -> StableArgumentParser:
    root = StableArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    build = commands.add_parser("build", help="build a deterministic manifest")
    build.add_argument("--repo", required=True, type=Path)
    build.add_argument("--old", required=True)
    build.add_argument("--new", required=True)
    build_new = commands.add_parser(
        "build-new",
        help="build a complete manifest from an empty tree",
    )
    build_new.add_argument("--repo", required=True, type=Path)
    build_new.add_argument("--new", required=True)
    initialize = commands.add_parser(
        "init-analysis",
        help="initialize the canonical analysis package",
    )
    initialize.add_argument("--manifest", required=True, type=Path)
    initialize.add_argument("--repository", required=True)
    initialize.add_argument("--node", action="append", default=[])
    finalize = commands.add_parser(
        "finalize-analysis",
        help="deterministically redact and finalize an analysis package",
    )
    finalize.add_argument("--repo", required=True, type=Path)
    finalize.add_argument("--manifest", required=True, type=Path)
    finalize.add_argument("--scaffold", required=True, type=Path)
    finalize.add_argument("--analysis", required=True, type=Path)
    finalize.add_argument("--output", type=Path)
    check = commands.add_parser("check", help="validate an analysis package")
    check.add_argument("--repo", required=True, type=Path)
    check.add_argument("--manifest", required=True, type=Path)
    check.add_argument("--scaffold", required=True, type=Path)
    check.add_argument("--analysis", required=True, type=Path)
    check.add_argument("--current-ref", required=True)
    gate = commands.add_parser(
        "gate-batch",
        help="scope reviewed packages into deterministic write groups",
    )
    gate.add_argument(
        "--expected-repository",
        action="append",
        required=True,
    )
    gate.add_argument(
        "--package",
        action="append",
        type=Path,
        required=True,
    )
    gate.add_argument(
        "--output",
        required=True,
        type=Path,
        help="persist the authoritative gate result as JSON",
    )
    close = commands.add_parser(
        "close-package",
        help="persist a finalized, reviewed, gate-recoverable semantic package",
    )
    close.add_argument("--repo", required=True, type=Path)
    close.add_argument("--manifest", required=True, type=Path)
    close.add_argument("--scaffold", required=True, type=Path)
    close.add_argument("--analysis", required=True, type=Path)
    close.add_argument("--review", required=True, type=Path)
    close.add_argument("--branch", required=True, choices=("main", "master"))
    close.add_argument("--analysis-date", required=True)
    close.add_argument("--output", required=True, type=Path)
    validate_projection_command = commands.add_parser(
        "validate-projection",
        help="validate one claim-aware projection unit against the sealed gate",
    )
    validate_projection_command.add_argument("--gate", required=True, type=Path)
    validate_projection_command.add_argument(
        "--projection",
        required=True,
        type=Path,
    )
    validate_projection_command.add_argument("--patch", required=True, type=Path)
    return root

def main(argv: list[str] | None = None) -> int:
    try:
        args = parser().parse_args(argv)
        if args.command == "build":
            emit(build_manifest(args.repo, args.old, args.new))
            return 0
        if args.command == "build-new":
            emit(build_new_manifest(args.repo, args.new))
            return 0
        if args.command == "init-analysis":
            emit(initialize_analysis(
                args.manifest,
                args.repository,
                args.node,
            ))
            return 0
        if args.command == "finalize-analysis":
            output = args.output or args.analysis.parent / "finalize-result.json"
            validate_finalize_output(output, args.analysis)
            payload = finalize_analysis(
                args.repo,
                args.manifest,
                args.scaffold,
                args.analysis,
            )
            replace_json(output, payload)
            emit(payload)
            return 0 if payload["status"] == "pass" else 2
        if args.command == "validate-projection":
            payload = validate_projection(
                args.gate,
                args.projection,
                args.patch,
            )
            emit(payload)
            return 0 if payload["status"] == "pass" else 2
        if args.command == "close-package":
            validate_package_output(
                args.output, args.repo,
                [args.manifest, args.scaffold, args.analysis, args.review],
            )
            payload = close_package(
                args.repo, args.manifest, args.scaffold,
                args.analysis, args.review, args.branch, args.analysis_date,
            )
            replace_json(args.output, payload)
            emit(payload)
            return 0 if payload["status"] == "complete" else 2
        if args.command == "gate-batch":
            validate_closed_gate_output(args.output, args.package)
            payload = gate_batch(
                args.package,
                args.expected_repository,
            )
            replace_json(args.output, payload)
            emit(payload)
            return 0 if payload["status"] != "blocked" else 2
        payload = check_analysis(
            args.repo,
            args.manifest,
            args.scaffold,
            args.analysis,
            args.current_ref,
        )
        emit(payload)
        return 0 if payload["status"] == "pass" else 2
    except ManifestError as error:
        status = "blocked" if error.exit_code == 2 else "error"
        emit({"code": error.code, "status": status, "message": str(error)})
        return error.exit_code
    except Exception:
        emit({
            "code": "internal-error",
            "status": "error",
            "message": "unexpected internal failure",
        })
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
