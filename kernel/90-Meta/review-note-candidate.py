#!/usr/bin/env python3
"""Freeze and verify independently reviewed full Markdown note images.

This helper binds artifacts; it does not perform semantic review or write notes
to a vault. Reviewer independence is enforced by the caller/orchestrator.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path, PurePosixPath
from typing import Iterable


KNOWLEDGE_ROOTS = {
    "10-Sistemas", "15-Arquitectura", "20-Repos", "25-Topics",
    "30-Flujos", "40-Integraciones", "50-Glosario", "60-Operacion",
    "70-Aprendizajes",
}
EMPTY_DIGEST = hashlib.sha256(b"").hexdigest()
DIGEST_RE = re.compile(r"^[0-9a-f]{64}$")
CONNECTION_RE = re.compile(
    r"<!--\s*connection:(connection\.[A-Za-z0-9][A-Za-z0-9._-]*)\s*-->"
)
CONNECTION_ID_RE = re.compile(r"^connection\.[A-Za-z0-9][A-Za-z0-9._-]*$")
REVIEW_FIELDS = {
    "version", "manifest_digest", "verdict", "findings",
    "connection_decisions",
}


class NoteCandidateError(ValueError):
    """A closed-gate contract violation."""


def canonical_digest(value: object) -> str:
    encoded = json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def file_digest(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def safe_path(path: Path, *, must_exist: bool = True) -> Path:
    if ".." in path.parts:
        raise NoteCandidateError("unsafe-path")
    absolute = path.absolute()
    # macOS exposes top-level aliases such as /var -> /private/var. Accept that
    # platform boundary, but reject caller-controlled symlinks below it.
    current = Path(absolute.anchor)
    for index, part in enumerate(absolute.parts[1:]):
        current /= part
        if index > 0 and current.is_symlink():
            raise NoteCandidateError("unsafe-path")
    if must_exist and not absolute.exists():
        raise NoteCandidateError("missing-path")
    return absolute


def relative_path(value: str) -> PurePosixPath:
    path = PurePosixPath(value)
    if path.is_absolute() or not path.parts or ".." in path.parts or "." in path.parts:
        raise NoteCandidateError("unsafe-relative-path")
    return path


def note_path(value: str) -> PurePosixPath:
    path = relative_path(value)
    if len(path.parts) < 2 or path.parts[0] not in KNOWLEDGE_ROOTS or path.suffix != ".md":
        raise NoteCandidateError("candidate-path-not-authorized")
    return path


def contained_file(root: Path, relative: PurePosixPath) -> Path:
    current = contained_path(root, relative)
    if not current.is_file():
        raise NoteCandidateError("missing-or-invalid-file")
    return current


def contained_path(root: Path, relative: PurePosixPath) -> Path:
    current = root
    for part in relative.parts:
        current = current / part
        if current.is_symlink():
            raise NoteCandidateError("unsafe-path")
    return current


def candidate_files(root: Path) -> dict[str, Path]:
    files: dict[str, Path] = {}
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root).as_posix()
        if path.is_symlink():
            raise NoteCandidateError("unsafe-path")
        if path.is_dir():
            continue
        if not path.is_file():
            raise NoteCandidateError("missing-or-invalid-file")
        validated = note_path(relative).as_posix()
        files[validated] = path
    return files


def connections(path: Path) -> list[str]:
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError as error:
        raise NoteCandidateError("candidate-not-utf8") from error
    found = CONNECTION_RE.findall(text)
    if len(found) != len(set(found)):
        raise NoteCandidateError("duplicate-connection-anchor")
    return sorted(found)


def read_json(path: Path) -> object:
    def unique_object(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise NoteCandidateError("invalid-json")
            value[key] = item
        return value
    try:
        return json.loads(
            path.read_text(encoding="utf-8"), object_pairs_hook=unique_object
        )
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise NoteCandidateError("invalid-json") from error


def projection_binding(projection: object) -> tuple[dict[str, str], dict[str, str]]:
    if not isinstance(projection, dict):
        raise NoteCandidateError("projection-invalid")
    base, result = projection.get("base_files"), projection.get("result_files")
    if not isinstance(base, dict) or not isinstance(result, dict):
        raise NoteCandidateError("projection-invalid")
    for mapping in (base, result):
        if any(
            not isinstance(key, str)
            or not isinstance(value, str)
            or not DIGEST_RE.fullmatch(value)
            for key, value in mapping.items()
        ):
            raise NoteCandidateError("projection-invalid")
    return dict(base), dict(result)


def freeze_candidate(
    vault: Path,
    candidate: Path,
    evidence_root: Path,
    evidence_paths: Iterable[str],
    projection_path: Path | None = None,
    deleted_paths: Iterable[str] = (),
) -> dict[str, object]:
    vault = safe_path(vault)
    candidate = safe_path(candidate)
    evidence_root = safe_path(evidence_root)
    if not vault.is_dir() or not candidate.is_dir() or not evidence_root.is_dir():
        raise NoteCandidateError("root-not-directory")

    candidates = candidate_files(candidate)
    deleted = sorted({note_path(raw).as_posix() for raw in deleted_paths})
    if set(deleted) & set(candidates):
        raise NoteCandidateError("candidate-delete-overlap")
    if not candidates and not deleted:
        raise NoteCandidateError("empty-candidate")
    candidate_hashes = {name: file_digest(path) for name, path in candidates.items()}
    candidate_hashes.update({name: EMPTY_DIGEST for name in deleted})
    base_hashes: dict[str, str] = {}
    base_present: list[str] = []
    connection_images: dict[str, dict[str, list[str]]] = {}
    for name in sorted(candidate_hashes):
        relative = note_path(name)
        target = contained_path(vault, relative)
        if target.exists():
            target = contained_file(vault, relative)
            base_hashes[name] = file_digest(target)
            base_present.append(name)
            before = connections(target)
        else:
            if name in deleted:
                raise NoteCandidateError("delete-base-missing")
            base_hashes[name] = EMPTY_DIGEST
            before = []
        after = [] if name in deleted else connections(candidates[name])
        connection_images[name] = {"base": before, "candidate": after}

    evidence_hashes: dict[str, str] = {}
    for raw in evidence_paths:
        relative = relative_path(raw)
        name = relative.as_posix()
        if name in evidence_hashes:
            raise NoteCandidateError("duplicate-evidence")
        evidence_hashes[name] = file_digest(contained_file(evidence_root, relative))
    if not evidence_hashes:
        raise NoteCandidateError("empty-evidence")

    manifest: dict[str, object] = {
        "version": 1,
        "candidate_files": candidate_hashes,
        "base_files": base_hashes,
        "base_present": base_present,
        "deleted_files": deleted,
        "evidence_files": evidence_hashes,
        "connections": connection_images,
    }
    if projection_path is not None:
        projection_path = safe_path(projection_path)
        projection = read_json(projection_path)
        projection_base, projection_result = projection_binding(projection)
        if projection_result != candidate_hashes or projection_base != base_hashes:
            raise NoteCandidateError("projection-file-binding-invalid")
        manifest["projection_digest"] = canonical_digest(projection)
    return manifest


def validate_manifest(value: object) -> dict[str, object]:
    required = {
        "version", "candidate_files", "base_files", "base_present", "deleted_files",
        "evidence_files", "connections",
    }
    if not isinstance(value, dict) or set(value) not in (required, required | {"projection_digest"}):
        raise NoteCandidateError("manifest-invalid")
    if value["version"] != 1:
        raise NoteCandidateError("manifest-invalid")
    for field in ("candidate_files", "base_files", "evidence_files"):
        mapping = value[field]
        if not isinstance(mapping, dict) or not mapping or any(
            not isinstance(key, str)
            or not isinstance(digest, str)
            or not DIGEST_RE.fullmatch(digest)
            for key, digest in mapping.items()
        ):
            raise NoteCandidateError("manifest-invalid")
    candidates = value["candidate_files"]
    if set(value["base_files"]) != set(candidates):
        raise NoteCandidateError("manifest-invalid")
    if not isinstance(value["base_present"], list) or any(
        not isinstance(name, str) for name in value["base_present"]
    ) or value["base_present"] != sorted(set(value["base_present"])):
        raise NoteCandidateError("manifest-invalid")
    if not set(value["base_present"]).issubset(candidates):
        raise NoteCandidateError("manifest-invalid")
    if (
        not isinstance(value["deleted_files"], list)
        or value["deleted_files"] != sorted(set(value["deleted_files"]))
        or not set(value["deleted_files"]).issubset(candidates)
        or not set(value["deleted_files"]).issubset(value["base_present"])
        or any(candidates[name] != EMPTY_DIGEST for name in value["deleted_files"])
    ):
        raise NoteCandidateError("manifest-invalid")
    if not isinstance(value["connections"], dict) or set(value["connections"]) != set(candidates):
        raise NoteCandidateError("manifest-invalid")
    for name in candidates:
        note_path(name)
        record = value["connections"][name]
        if not isinstance(record, dict) or set(record) != {"base", "candidate"}:
            raise NoteCandidateError("manifest-invalid")
        for side in ("base", "candidate"):
            ids = record[side]
            if not isinstance(ids, list) or ids != sorted(set(ids)) or any(
                not isinstance(item, str) or not CONNECTION_ID_RE.fullmatch(item) for item in ids
            ):
                raise NoteCandidateError("manifest-invalid")
    if "projection_digest" in value and (
        not isinstance(value["projection_digest"], str)
        or not DIGEST_RE.fullmatch(value["projection_digest"])
    ):
        raise NoteCandidateError("manifest-invalid")
    return value


def validate_review(review: object, manifest: dict[str, object]) -> None:
    if not isinstance(review, dict) or set(review) != REVIEW_FIELDS or review["version"] != 1:
        raise NoteCandidateError("review-invalid")
    if review["manifest_digest"] != canonical_digest(manifest):
        raise NoteCandidateError("review-stale")
    verdict, findings = review["verdict"], review["findings"]
    if verdict not in {"accept", "revise"} or not isinstance(findings, list):
        raise NoteCandidateError("review-invalid")
    valid_findings = all(
        isinstance(item, dict)
        and set(item) == {"reason"}
        and isinstance(item["reason"], str)
        and bool(item["reason"].strip())
        for item in findings
    )
    if not valid_findings or (verdict == "accept") != (findings == []):
        raise NoteCandidateError("review-invalid")

    expected: dict[str, set[str]] = {}
    for path, record in manifest["connections"].items():
        before, after = set(record["base"]), set(record["candidate"])
        for connection_id in before | after:
            key = f"{path}#{connection_id}"
            if connection_id in before and connection_id in after:
                expected[key] = {"preserve", "update"}
            elif connection_id in after:
                expected[key] = {"create"}
            else:
                expected[key] = {"retire"}
    decisions = review["connection_decisions"]
    if not isinstance(decisions, dict) or set(decisions) != set(expected):
        raise NoteCandidateError("connection-decisions-invalid")
    for key, allowed in expected.items():
        decision = decisions[key]
        if (
            not isinstance(decision, dict)
            or set(decision) != {"action", "reason"}
            or decision.get("action") not in allowed
            or not isinstance(decision.get("reason"), str)
            or not decision["reason"].strip()
        ):
            raise NoteCandidateError("connection-decisions-invalid")
    if verdict != "accept":
        raise NoteCandidateError("review-requires-revision")


def check_candidate(
    vault: Path,
    candidate: Path,
    evidence_root: Path,
    manifest_path: Path,
    review_path: Path,
    projection_path: Path | None = None,
) -> dict[str, object]:
    manifest_path = safe_path(manifest_path)
    review_path = safe_path(review_path)
    manifest = validate_manifest(read_json(manifest_path))
    current = freeze_candidate(
        vault,
        candidate,
        evidence_root,
        manifest["evidence_files"].keys(),
        projection_path,
        manifest["deleted_files"],
    )
    if current != manifest:
        raise NoteCandidateError("candidate-base-or-evidence-stale")
    validate_review(read_json(review_path), manifest)
    return {
        "status": "pass",
        "code": "note-candidate-reviewed",
        "manifest_digest": canonical_digest(manifest),
    }


def write_json(path: Path, value: object) -> None:
    path = safe_path(path, must_exist=False)
    if not path.parent.is_dir() or path.exists() and not path.is_file():
        raise NoteCandidateError("invalid-output")
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    freeze = commands.add_parser("freeze")
    check = commands.add_parser("check")
    for command in (freeze, check):
        command.add_argument("--vault", required=True, type=Path)
        command.add_argument("--candidate", required=True, type=Path)
        command.add_argument("--evidence-root", required=True, type=Path)
        command.add_argument("--projection", type=Path)
    freeze.add_argument("--evidence", action="append", required=True)
    freeze.add_argument("--delete", action="append", default=[])
    freeze.add_argument("--output", required=True, type=Path)
    check.add_argument("--manifest", required=True, type=Path)
    check.add_argument("--review", required=True, type=Path)
    args = parser.parse_args()
    try:
        if args.command == "freeze":
            result = freeze_candidate(
                args.vault, args.candidate, args.evidence_root,
                args.evidence, args.projection, args.delete,
            )
            write_json(args.output, result)
            output = {
                "status": "pass", "code": "note-candidate-frozen",
                "manifest": str(args.output.absolute()),
                "manifest_digest": canonical_digest(result),
            }
        else:
            output = check_candidate(
                args.vault, args.candidate, args.evidence_root,
                args.manifest, args.review, args.projection,
            )
    except (NoteCandidateError, OSError) as error:
        output = {"status": "blocked", "code": str(error)}
    print(json.dumps(output, sort_keys=True))
    return 0 if output["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
