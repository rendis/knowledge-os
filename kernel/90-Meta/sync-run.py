#!/usr/bin/env python3
"""Persist and resume independently projected synchronization units."""
from __future__ import annotations

import argparse
import copy
import hashlib
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path, PurePosixPath
from typing import Any, NoReturn


RUN_VERSION = 1
DEFAULT_STATE_ROOT = Path(".agents/state/map-ecosystem/sync")
DIGEST_RE = re.compile(r"^[0-9a-f]{64}$")
OID_RE = re.compile(r"^(?:[0-9a-f]{40}|[0-9a-f]{64})$")
NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
HUNK_RE = re.compile(
    r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?: .*)?(?:\n)?$"
)
SENSITIVE_PATCH_RE = re.compile(
    r"(?i)(?:password|passwd|secret|token|api[_-]?key|private[_-]?key)"
    r"\s*[:=]\s*['\"]?[^\s'\"]+"
)
TOKEN_LIKE_RE = re.compile(
    r"(?:gh[pousr]_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,}|"
    r"AKIA[0-9A-Z]{16}|Bearer\s+[A-Za-z0-9._~-]{10,}|"
    r"sk-(?:proj-)?[A-Za-z0-9_-]{20,})"
)
LOCAL_ABSOLUTE_PATH_RE = re.compile(
    r"(?<![A-Za-z0-9._-])/(?:Users|home|private/(?:tmp|var)|tmp|var/folders|Volumes)/"
    r"[^\s'\"`,;)}\]]+"
)
PACKAGE_RECEIPT_FIELDS = {
    "version", "code", "status", "repository", "new_oid", "manifest",
    "scaffold", "analysis", "review", "gate", "validation",
}


class SyncError(Exception):
    exit_code = 1

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


class ContractError(SyncError):
    exit_code = 2


class OperationalError(SyncError):
    pass


class StableArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise ContractError("usage-error", "invalid command arguments")


class DuplicateJsonKey(ValueError):
    pass


def reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    value: dict[str, Any] = {}
    for key, item in pairs:
        if key in value:
            raise DuplicateJsonKey(key)
        value[key] = item
    return value


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=True, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def canonical_digest(value: Any) -> str:
    return hashlib.sha256(canonical_bytes(value)).hexdigest()


def bytes_digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def emit(value: dict[str, Any]) -> None:
    sys.stdout.write(canonical_bytes(value).decode("ascii") + "\n")


def read_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=reject_duplicate_keys,
        )
    except FileNotFoundError as error:
        raise ContractError("input-missing", "required JSON input is missing") from error
    except (OSError, UnicodeError) as error:
        raise OperationalError("file-read-error", "JSON input could not be read") from error
    except (json.JSONDecodeError, DuplicateJsonKey) as error:
        raise ContractError("invalid-json", "input is not valid JSON") from error
    if not isinstance(value, dict):
        raise ContractError("invalid-json", "JSON input must be an object")
    return value


def atomic_bytes(path: Path, content: bytes, *, mode: int = 0o600) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary: Path | None = None
    try:
        descriptor, raw = tempfile.mkstemp(
            prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
        )
        temporary = Path(raw)
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temporary, mode)
        os.replace(temporary, path)
        temporary = None
    except OSError as error:
        raise OperationalError(
            "file-write-error", "state could not be persisted safely"
        ) from error
    finally:
        if temporary is not None:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass


def atomic_json(path: Path, value: dict[str, Any]) -> None:
    atomic_bytes(path, canonical_bytes(value) + b"\n")


def require_state_write_path(state_root: Path, target: Path) -> None:
    lexical_root = Path(os.path.abspath(state_root))
    lexical_target = Path(os.path.abspath(target))
    try:
        relative = lexical_target.relative_to(lexical_root)
    except ValueError as error:
        raise ContractError("state-path-invalid", "state write escapes its root") from error
    current = lexical_root
    if current.is_symlink():
        raise ContractError("state-path-invalid", "state paths cannot be symbolic links")
    for part in relative.parts:
        current = current / part
        if current.is_symlink():
            raise ContractError("state-path-invalid", "state paths cannot be symbolic links")
    try:
        lexical_target.parent.resolve().relative_to(lexical_root.resolve())
    except ValueError as error:
        raise ContractError("state-path-invalid", "state write resolves outside its root") from error


def atomic_state_bytes(
    state_root: Path,
    path: Path,
    content: bytes,
    *,
    mode: int = 0o600,
) -> None:
    require_state_write_path(state_root, path)
    atomic_bytes(path, content, mode=mode)


def atomic_state_json(state_root: Path, path: Path, value: dict[str, Any]) -> None:
    atomic_state_bytes(state_root, path, canonical_bytes(value) + b"\n")


def valid_name(value: Any) -> bool:
    return isinstance(value, str) and NAME_RE.fullmatch(value) is not None


def valid_node_name(value: Any) -> bool:
    return (
        isinstance(value, str)
        and bool(value.strip())
        and value == value.strip()
        and value not in {".", ".."}
        and "/" not in value
        and "\\" not in value
        and not any(ord(character) < 32 or ord(character) == 127 for character in value)
    )


def valid_path(value: Any) -> bool:
    if not isinstance(value, str) or not value or "\x00" in value or "\\" in value:
        return False
    path = PurePosixPath(value)
    return not path.is_absolute() and all(part not in {"", ".", ".."} for part in path.parts)


def require_digest(value: str, field: str) -> None:
    if DIGEST_RE.fullmatch(value) is None:
        raise ContractError("invalid-digest", f"{field} must be a SHA-256 digest")


def require_run_id(run_id: str) -> None:
    if not valid_name(run_id):
        raise ContractError("invalid-run-id", "run_id is invalid")


def require_state_container(state_root: Path, child: str) -> None:
    if state_root.is_symlink() or (state_root / child).is_symlink():
        raise ContractError("state-path-invalid", "state paths cannot be symbolic links")


def active_dir(state_root: Path, run_id: str) -> Path:
    require_run_id(run_id)
    require_state_container(state_root, "active")
    return state_root / "active" / run_id


def receipt_path(state_root: Path, run_id: str) -> Path:
    require_run_id(run_id)
    require_state_container(state_root, "receipts")
    return state_root / "receipts" / f"{run_id}.json"


def fingerprint_receipt_path(state_root: Path, fingerprint: str) -> Path:
    require_digest(fingerprint, "fingerprint")
    require_state_container(state_root, "receipts")
    return state_root / "receipts" / "fingerprints" / f"{fingerprint}.json"


def run_path(state_root: Path, run_id: str) -> Path:
    return active_dir(state_root, run_id) / "run.json"


def unit_dir(state_root: Path, run_id: str, unit_id: str) -> Path:
    if not valid_name(unit_id):
        raise ContractError("invalid-unit-id", "unit_id is invalid")
    return active_dir(state_root, run_id) / "units" / unit_id


def load_run(state_root: Path, run_id: str) -> dict[str, Any]:
    path = run_path(state_root, run_id)
    if path.is_symlink() or path.parent.is_symlink():
        raise ContractError("state-path-invalid", "run state cannot be a symbolic link")
    if not path.is_file():
        raise ContractError("run-state-missing", "active run state is missing")
    run = read_json(path)
    if run.get("version") != RUN_VERSION:
        raise ContractError(
            "run-version-mismatch", "run state uses an unsupported schema version"
        )
    if run.get("run_id") != run_id:
        raise ContractError("run-state-invalid", "run identity does not match its path")
    required = {
        "version", "run_id", "fingerprint", "tool_digest", "inventory_digest",
        "status", "packages", "gate_digest", "gate_stale", "units", "vault_locator",
    }
    if set(run) != required:
        raise ContractError("run-state-invalid", "run state fields are invalid")
    if (
        not isinstance(run["packages"], list)
        or not isinstance(run["units"], list)
        or type(run["gate_stale"]) is not bool
        or not isinstance(run["vault_locator"], str)
        or run["status"] not in {"packages", "gated", "projecting", "closing"}
        or DIGEST_RE.fullmatch(run["fingerprint"]) is None
        or DIGEST_RE.fullmatch(run["tool_digest"]) is None
        or DIGEST_RE.fullmatch(run["inventory_digest"]) is None
        or run["gate_digest"] and DIGEST_RE.fullmatch(run["gate_digest"]) is None
    ):
        raise ContractError("run-state-invalid", "run state values are invalid")
    package_fields = {
        "repository", "oid", "status", "artifact_digest", "checkpoint_count",
    }
    package_names = []
    repaired_missing_packages: set[str] = set()
    for package in run["packages"]:
        if (
            not isinstance(package, dict)
            or set(package) != package_fields
            or not valid_name(package.get("repository"))
            or not isinstance(package.get("oid"), str)
            or OID_RE.fullmatch(package["oid"]) is None
            or package.get("status") not in {"pending", "checkpointed", "stale"}
            or package.get("artifact_digest") and DIGEST_RE.fullmatch(package["artifact_digest"]) is None
            or type(package.get("checkpoint_count")) is not int
            or package["checkpoint_count"] < 0
        ):
            raise ContractError("run-state-invalid", "package state is invalid")
        package_names.append(package["repository"])
    if package_names != sorted(set(package_names)):
        raise ContractError("run-state-invalid", "package state order is invalid")
    if run["fingerprint"] != run_fingerprint(
        run["tool_digest"], run["inventory_digest"],
        [(item["repository"], item["oid"]) for item in run["packages"]],
    ):
        raise ContractError("run-state-invalid", "run fingerprint differs from package identity")
    unit_fields = {
        "unit_id", "unit_type", "status", "repositories", "nodes",
        "stale_reason",
        "failed_patch_digests", "gate_digest", "projection_digest", "patch_digest",
        "base_files", "result_files", "base_kinds", "result_kinds",
        "receipt_digest",
    }
    unit_names = []
    for unit in run["units"]:
        if (
            not isinstance(unit, dict)
            or set(unit) != unit_fields
            or not valid_name(unit.get("unit_id"))
            or unit.get("unit_type") not in {"acknowledgements", "write-group"}
            or unit.get("status") not in {
                "pending", "validated", "applied", "projection-invalid", "apply-failed", "stale"
            }
            or unit.get("stale_reason") not in {
                "", "source", "source-retracted", "destination"
            }
            or not isinstance(unit.get("repositories"), list)
            or unit["repositories"] != sorted(set(unit["repositories"]))
            or any(not valid_name(item) for item in unit["repositories"])
            or not isinstance(unit.get("nodes"), list)
            or unit["nodes"] != sorted(set(unit["nodes"]))
            or any(not valid_node_name(item) for item in unit["nodes"])
            or not isinstance(unit.get("failed_patch_digests"), list)
            or unit["failed_patch_digests"] != sorted(set(unit["failed_patch_digests"]))
            or any(DIGEST_RE.fullmatch(item) is None for item in unit["failed_patch_digests"])
            or any(
                not isinstance(value, str)
                or value and DIGEST_RE.fullmatch(value) is None
                for value in (
                    unit.get("gate_digest"), unit.get("projection_digest"),
                    unit.get("patch_digest"), unit.get("receipt_digest"),
                )
            )
            or not all(
                isinstance(unit.get(field), dict)
                and all(valid_path(path) and isinstance(digest, str) and DIGEST_RE.fullmatch(digest) is not None for path, digest in unit[field].items())
                for field in ("base_files", "result_files")
            )
            or not all(
                isinstance(unit.get(field), dict)
                and set(unit[field]) == set(unit["base_files"])
                and all(kind in {"file", "missing"} for kind in unit[field].values())
                for field in ("base_kinds", "result_kinds")
            )
        ):
            raise ContractError("run-state-invalid", "unit state is invalid")
        unit_names.append(unit["unit_id"])
    if len(unit_names) != len(set(unit_names)):
        raise ContractError("run-state-invalid", "unit identities are duplicated")
    root = active_dir(state_root, run_id)
    for package in run["packages"]:
        if package["status"] != "checkpointed":
            continue
        artifact_path = root / "packages" / package["repository"] / "artifact.json"
        try:
            artifact = read_json(artifact_path)
        except SyncError as error:
            if error.code == "input-missing":
                package["status"] = "stale"
                package["artifact_digest"] = ""
                repaired_missing_packages.add(package["repository"])
                continue
            raise ContractError(
                "package-checkpoint-invalid", "checkpointed package artifact is missing or invalid"
            ) from error
        if canonical_digest(artifact) != package["artifact_digest"]:
            raise ContractError(
                "package-checkpoint-invalid", "checkpointed package digest does not match"
            )
        if (
            sensitive_artifact(artifact)
            or not checkpoint_artifact_valid(artifact)
            or artifact["repository"] != package["repository"]
            or artifact["new_oid"] != package["oid"]
        ):
            raise ContractError(
                "package-checkpoint-invalid", "checkpointed package receipt is invalid"
            )
    if repaired_missing_packages:
        run["gate_stale"] = True
        run["status"] = "packages"
        for unit in run["units"]:
            if repaired_missing_packages.intersection(unit["repositories"]):
                unit["status"] = "stale"
                unit["stale_reason"] = "source"
        atomic_state_json(state_root, path, run)
    if run["gate_digest"]:
        history_path = root / "gates" / f"{run['gate_digest']}.json"
        try:
            gate = read_json(root / "gate.json")
        except SyncError as error:
            try:
                gate = read_json(history_path)
            except SyncError as history_error:
                raise ContractError(
                    "gate-checkpoint-invalid", "sealed gate is missing or invalid"
                ) from history_error
            if canonical_digest(gate) == run["gate_digest"]:
                atomic_state_json(state_root, root / "gate.json", gate)
        if canonical_digest(gate) != run["gate_digest"]:
            try:
                gate = read_json(history_path)
            except SyncError as error:
                raise ContractError(
                    "gate-checkpoint-invalid", "sealed gate digest does not match"
                ) from error
            if canonical_digest(gate) != run["gate_digest"]:
                raise ContractError("gate-checkpoint-invalid", "sealed gate digest does not match")
            atomic_state_json(state_root, root / "gate.json", gate)
    for unit in run["units"]:
        if unit["status"] not in {"validated", "apply-failed", "applied"}:
            continue
        directory = root / "units" / unit["unit_id"]
        try:
            projection = read_json(directory / "projection.json")
            patch_digest = bytes_digest((directory / "unit.patch").read_bytes())
        except (SyncError, OSError) as error:
            raise ContractError(
                "unit-checkpoint-invalid", "validated unit checkpoint is missing or invalid"
            ) from error
        if (
            canonical_digest(projection) != unit["projection_digest"]
            or patch_digest != unit["patch_digest"]
            or projection.get("gate_digest") != unit["gate_digest"]
        ):
            raise ContractError("unit-checkpoint-invalid", "validated unit digest does not match")
        try:
            unit_gate = read_json(root / "gates" / f"{unit['gate_digest']}.json")
        except SyncError as error:
            raise ContractError(
                "unit-checkpoint-invalid", "unit gate history is missing or invalid"
            ) from error
        if canonical_digest(unit_gate) != unit["gate_digest"]:
            raise ContractError("unit-checkpoint-invalid", "unit gate history digest differs")
        if unit["status"] == "applied":
            try:
                receipt = read_json(directory / "receipt.json")
            except SyncError as error:
                raise ContractError(
                    "unit-checkpoint-invalid", "applied unit receipt is missing or invalid"
                ) from error
            if canonical_digest(receipt) != unit["receipt_digest"]:
                raise ContractError("unit-checkpoint-invalid", "applied receipt digest does not match")
    return run


def save_run(state_root: Path, run: dict[str, Any]) -> None:
    atomic_state_json(state_root, run_path(state_root, run["run_id"]), run)


def find_unit(run: dict[str, Any], unit_id: str) -> dict[str, Any]:
    for unit in run["units"]:
        if unit["unit_id"] == unit_id:
            return unit
    raise ContractError("unit-not-found", "unit is not part of this run")


def package_counters(run: dict[str, Any]) -> dict[str, int]:
    packages = run["packages"]
    return {
        "checkpointed": sum(item["status"] == "checkpointed" for item in packages),
        "expected": len(packages),
        "invalidated": sum(item["status"] == "stale" for item in packages),
    }


def run_fingerprint(
    tool_digest: str,
    inventory_digest: str,
    packages: list[tuple[str, str]],
) -> str:
    return canonical_digest({
        "version": RUN_VERSION,
        "tool_digest": tool_digest,
        "inventory_digest": inventory_digest,
        "packages": [[repository, oid] for repository, oid in sorted(packages)],
    })


def next_command(run: dict[str, Any]) -> str:
    if any(
        unit["status"] == "stale"
        and unit["stale_reason"] == "source"
        and unit["receipt_digest"]
        for unit in run["units"]
    ):
        return "resume"
    if any(item["status"] in {"pending", "stale"} for item in run["packages"]):
        return "checkpoint-package"
    if run.get("gate_stale"):
        return "seal-gate"
    if not run.get("gate_digest"):
        return "seal-gate"
    if any(
        unit["status"] in {"pending", "projection-invalid", "stale"}
        for unit in run["units"]
    ):
        return "validate-unit"
    if any(unit["status"] in {"validated", "apply-failed"} for unit in run["units"]):
        return "apply-unit"
    if run["units"] and all(unit["status"] == "applied" for unit in run["units"]):
        return "close"
    return "close" if not run["units"] and run.get("gate_digest") else "seal-gate"


def receipt_view(run: dict[str, Any]) -> dict[str, Any]:
    return {
        "version": RUN_VERSION,
        "run_id": run["run_id"],
        "fingerprint": run["fingerprint"],
        "tool_digest": run["tool_digest"],
        "inventory_digest": run["inventory_digest"],
        "status": run["status"],
        "gate_digest": run["gate_digest"],
        "packages": [
            {
                "repository": item["repository"],
                "oid": item["oid"],
                "status": item["status"],
                "artifact_digest": item["artifact_digest"],
            }
            for item in run["packages"]
        ],
        "units": [
            {
                "unit_id": unit["unit_id"],
                "unit_type": unit["unit_type"],
                "status": unit["status"],
                "gate_digest": unit["gate_digest"],
                "projection_digest": unit["projection_digest"],
                "patch_digest": unit["patch_digest"],
                "receipt_digest": unit["receipt_digest"],
            }
            for unit in run["units"]
        ],
    }


def validate_closed_receipt(
    state_root: Path,
    path: Path,
    run_id: str,
    *,
    expected_fingerprint: str | None = None,
) -> dict[str, Any]:
    require_state_write_path(state_root, path)
    try:
        receipt = read_json(path)
    except SyncError as error:
        raise ContractError("run-receipt-invalid", "closed run receipt is invalid") from error
    fields = {
        "version", "code", "status", "run_id", "fingerprint", "tool_digest",
        "inventory_digest", "gate_digest", "packages", "units", "receipt_digest",
    }
    if set(receipt) != fields:
        raise ContractError("run-receipt-invalid", "closed run receipt fields are invalid")
    fingerprint = receipt.get("fingerprint")
    if (
        receipt.get("version") != RUN_VERSION
        or receipt.get("code") != "run-closed"
        or receipt.get("status") != "complete"
        or receipt.get("run_id") != run_id
        or not isinstance(fingerprint, str)
        or DIGEST_RE.fullmatch(fingerprint) is None
        or expected_fingerprint is not None and fingerprint != expected_fingerprint
        or any(
            not isinstance(receipt.get(field), str)
            or DIGEST_RE.fullmatch(receipt[field]) is None
            for field in ("tool_digest", "inventory_digest", "gate_digest", "receipt_digest")
        )
    ):
        raise ContractError("run-receipt-invalid", "closed run receipt identity is invalid")
    packages = receipt.get("packages")
    if not isinstance(packages, list) or any(
        not isinstance(item, dict)
        or set(item) != {"repository", "oid", "status", "artifact_digest"}
        or not valid_name(item.get("repository"))
        or not isinstance(item.get("oid"), str)
        or OID_RE.fullmatch(item["oid"]) is None
        or item.get("status") != "checkpointed"
        or not isinstance(item.get("artifact_digest"), str)
        or DIGEST_RE.fullmatch(item["artifact_digest"]) is None
        for item in packages
    ):
        raise ContractError("run-receipt-invalid", "closed package receipts are invalid")
    if [item["repository"] for item in packages] != sorted({item["repository"] for item in packages}):
        raise ContractError("run-receipt-invalid", "closed package order is invalid")
    if fingerprint != run_fingerprint(
        receipt["tool_digest"],
        receipt["inventory_digest"],
        [(item["repository"], item["oid"]) for item in packages],
    ):
        raise ContractError("run-receipt-invalid", "closed package identity differs from its fingerprint")
    units = receipt.get("units")
    unit_fields = {
        "unit_id", "unit_type", "status", "gate_digest", "projection_digest",
        "patch_digest", "receipt_digest",
    }
    if not isinstance(units, list) or any(
        not isinstance(item, dict)
        or set(item) != unit_fields
        or not valid_name(item.get("unit_id"))
        or item.get("unit_type") not in {"acknowledgements", "write-group"}
        or item.get("status") != "applied"
        or any(
            not isinstance(item.get(field), str)
            or DIGEST_RE.fullmatch(item[field]) is None
            for field in ("gate_digest", "projection_digest", "patch_digest", "receipt_digest")
        )
        for item in units
    ):
        raise ContractError("run-receipt-invalid", "closed unit receipts are invalid")
    if len({item["unit_id"] for item in units}) != len(units):
        raise ContractError("run-receipt-invalid", "closed unit identities are duplicated")
    expected_digest = receipt["receipt_digest"]
    unsigned = {key: value for key, value in receipt.items() if key != "receipt_digest"}
    if canonical_digest(unsigned) != expected_digest:
        raise ContractError("run-receipt-invalid", "closed receipt digest does not match")
    return receipt


def public_status(state_root: Path, run: dict[str, Any]) -> dict[str, Any]:
    run_id = run["run_id"]
    units = []
    for unit in run["units"]:
        directory = unit_dir(state_root, run_id, unit["unit_id"])
        units.append({
            "unit_id": unit["unit_id"],
            "unit_type": unit["unit_type"],
            "status": unit["status"],
            "stale_reason": unit["stale_reason"],
            "repositories": unit["repositories"],
            "nodes": unit["nodes"],
            "gate_digest": unit["gate_digest"],
            "projection_digest": unit["projection_digest"],
            "patch_digest": unit["patch_digest"],
            "receipt_digest": unit["receipt_digest"],
            "projection_path": str(directory / "projection.json"),
            "patch_path": str(directory / "unit.patch"),
        })
    return {
        "version": RUN_VERSION,
        "code": "run-status",
        "status": run["status"],
        "run_id": run_id,
        "gate_digest": run["gate_digest"],
        "packages": [
            {
                "repository": item["repository"],
                "oid": item["oid"],
                "status": item["status"],
                "artifact_digest": item["artifact_digest"],
            }
            for item in run["packages"]
        ],
        "units": units,
        "package_counters": package_counters(run),
        "receipt_digest": canonical_digest(receipt_view(run)),
        "next_command": next_command(run),
    }


def begin(args: argparse.Namespace) -> dict[str, Any]:
    require_digest(args.tool_digest, "tool_digest")
    require_digest(args.inventory_digest, "inventory_digest")
    packages = []
    for repository, oid in args.package:
        if not valid_name(repository) or OID_RE.fullmatch(oid) is None:
            raise ContractError("package-contract-invalid", "package identity is invalid")
        packages.append((repository, oid))
    packages.sort()
    if not packages or len(packages) != len({item[0] for item in packages}):
        raise ContractError("package-contract-invalid", "packages must be unique")
    fingerprint = run_fingerprint(args.tool_digest, args.inventory_digest, packages)
    indexed = fingerprint_receipt_path(args.state_root, fingerprint)
    if indexed.is_symlink() or indexed.is_file():
        indexed_value = read_json(indexed)
        indexed_run_id = indexed_value.get("run_id")
        if not valid_name(indexed_run_id):
            raise ContractError("run-receipt-invalid", "fingerprint receipt identity is invalid")
        receipt = validate_closed_receipt(
            args.state_root, indexed, indexed_run_id,
            expected_fingerprint=fingerprint,
        )
        return {**receipt, "reused": True}
    run_id = f"run-{fingerprint[:24]}"
    path = run_path(args.state_root, run_id)
    closed = receipt_path(args.state_root, run_id)
    if closed.is_symlink() or closed.is_file():
        receipt = validate_closed_receipt(
            args.state_root,
            closed,
            run_id,
        )
        if receipt["fingerprint"] == fingerprint:
            return {**receipt, "reused": True}
        generation = 2
        while True:
            candidate = f"run-{fingerprint[:24]}-{generation}"
            candidate_active = run_path(args.state_root, candidate)
            candidate_closed = receipt_path(args.state_root, candidate)
            if candidate_active.is_file():
                candidate_run = load_run(args.state_root, candidate)
                if candidate_run["fingerprint"] == fingerprint:
                    return {**public_status(args.state_root, candidate_run), "reused": True}
            if candidate_closed.is_file() or candidate_closed.is_symlink():
                candidate_receipt = validate_closed_receipt(
                    args.state_root, candidate_closed, candidate,
                )
                if candidate_receipt["fingerprint"] == fingerprint:
                    return {**candidate_receipt, "reused": True}
            if not candidate_active.exists() and not candidate_closed.exists():
                run_id, path, closed = candidate, candidate_active, candidate_closed
                break
            generation += 1
    if path.is_file():
        run = load_run(args.state_root, run_id)
        if run["fingerprint"] != fingerprint:
            raise ContractError("run-fingerprint-mismatch", "existing run has different inputs")
        return {**public_status(args.state_root, run), "reused": True}
    if path.parent.exists():
            raise ContractError("run-state-missing", "existing run directory has no run state")
    active = args.state_root / "active"
    if active.is_dir():
        for candidate in sorted(active.iterdir()):
            if not candidate.is_dir() or not (candidate / "run.json").is_file():
                continue
            candidate_closed = receipt_path(args.state_root, candidate.name)
            if candidate_closed.is_symlink() or candidate_closed.is_file():
                candidate_receipt = validate_closed_receipt(
                    args.state_root,
                    candidate_closed,
                    candidate.name,
                )
                retire_closed_active(args.state_root, candidate_receipt)
                if candidate_receipt["fingerprint"] == fingerprint:
                    return {**candidate_receipt, "reused": True}
                continue
            candidate_run = load_run(args.state_root, candidate.name)
            if candidate_run["fingerprint"] == fingerprint:
                return {**public_status(args.state_root, candidate_run), "reused": True}
            if candidate.name != run_id:
                raise ContractError("active-run-conflict", "another active run must be resumed or closed")
    run = {
        "version": RUN_VERSION,
        "run_id": run_id,
        "fingerprint": fingerprint,
        "tool_digest": args.tool_digest,
        "inventory_digest": args.inventory_digest,
        "status": "packages",
        "packages": [
            {
                "repository": repository,
                "oid": oid,
                "status": "pending",
                "artifact_digest": "",
                "checkpoint_count": 0,
            }
            for repository, oid in packages
        ],
        "gate_digest": "",
        "gate_stale": False,
        "units": [],
        "vault_locator": "",
    }
    save_run(args.state_root, run)
    return {**public_status(args.state_root, run), "reused": False}


def sensitive_artifact(value: Any) -> bool:
    if isinstance(value, dict):
        for item in value.values():
            if sensitive_artifact(item):
                return True
        return False
    if isinstance(value, list):
        return any(sensitive_artifact(item) for item in value)
    if isinstance(value, str):
        return (
            value.startswith("/")
            or LOCAL_ABSOLUTE_PATH_RE.search(value) is not None
            or "-----BEGIN " in value
            or SENSITIVE_PATCH_RE.search(value) is not None
            or TOKEN_LIKE_RE.search(value) is not None
        )
    return False


def checkpoint_artifact_valid(value: dict[str, Any]) -> bool:
    if not (
        set(value) == PACKAGE_RECEIPT_FIELDS
        and value.get("version") == 1
        and value.get("code") == "package-closed"
        and value.get("status") == "complete"
        and valid_name(value.get("repository"))
        and isinstance(value.get("new_oid"), str)
        and OID_RE.fullmatch(value["new_oid"]) is not None
        and all(isinstance(value.get(field), dict) for field in (
            "manifest", "scaffold", "analysis", "gate", "validation",
        ))
        and (value.get("review") is None or isinstance(value["review"], dict))
    ):
        return False
    validation = value["validation"]
    if set(validation) != {
        "manifest_digest", "scaffold_digest", "analysis_digest",
        "review_digest", "gate_digest",
    }:
        return False
    for field, artifact_field in (
        ("manifest_digest", "manifest"),
        ("scaffold_digest", "scaffold"),
        ("analysis_digest", "analysis"),
        ("review_digest", "review"),
        ("gate_digest", "gate"),
    ):
        if (
            not isinstance(validation.get(field), str)
            or validation[field] != canonical_digest(value[artifact_field])
        ):
            return False
    basic_valid = (
        value["manifest"].get("new_oid") == value["new_oid"]
        and value["analysis"].get("repository") == value["repository"]
        and value["gate"].get("code") == "batch-gated"
        and value["gate"].get("status") in {"all-ready", "complete-no-write"}
        and [
            item.get("repository")
            for item in value["gate"].get("repositories", [])
            if isinstance(item, dict)
        ] == [value["repository"]]
    )
    if not basic_valid:
        return False
    helper = Path(__file__).with_name("git-change-manifest.py")
    spec = importlib.util.spec_from_file_location("sync_package_validator", helper)
    if spec is None or spec.loader is None:
        return False
    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
        return not module.validate_closed_package_value(value)
    except Exception:
        return False


def checkpoint_package(args: argparse.Namespace) -> dict[str, Any]:
    run = load_run(args.state_root, args.run_id)
    package = next(
        (item for item in run["packages"] if item["repository"] == args.repository), None
    )
    if package is None:
        raise ContractError("package-not-found", "repository is not part of this run")
    artifact = read_json(args.artifact)
    if sensitive_artifact(artifact):
        raise ContractError(
            "checkpoint-sensitive-content", "package artifact contains non-checkpoint-safe material"
        )
    if not checkpoint_artifact_valid(artifact):
        raise ContractError(
            "checkpoint-lineage-invalid",
            "package artifact must be an exact finalized semantic package",
        )
    if artifact.get("repository", args.repository) != args.repository:
        raise ContractError("package-identity-mismatch", "package receipt repository differs")
    observed_oid = artifact.get("new_oid")
    if observed_oid is not None and observed_oid != package["oid"]:
        raise ContractError("package-identity-mismatch", "package receipt OID differs")
    canonical = canonical_bytes(artifact)
    digest = bytes_digest(canonical)
    checkpoint_path = (
        active_dir(args.state_root, args.run_id)
        / "packages" / args.repository / "artifact.json"
    )
    if package["status"] == "checkpointed" and package["artifact_digest"] == digest:
        return {"version": RUN_VERSION, "code": "package-checkpointed", "status": "pass", "run_id": args.run_id, "repository": args.repository, "artifact_digest": digest, "artifact_path": str(checkpoint_path), "reused": True}
    if package["status"] == "checkpointed":
        raise ContractError("package-checkpoint-mismatch", "checkpointed package cannot be overwritten")
    atomic_state_bytes(args.state_root, checkpoint_path, canonical + b"\n")
    package["status"] = "checkpointed"
    package["artifact_digest"] = digest
    package["checkpoint_count"] += 1
    save_run(args.state_root, run)
    return {"version": RUN_VERSION, "code": "package-checkpointed", "status": "pass", "run_id": args.run_id, "repository": args.repository, "artifact_digest": digest, "artifact_path": str(checkpoint_path), "reused": False}


def package_authority(artifact: dict[str, Any]) -> tuple[dict[str, Any], list[dict[str, Any]], list[dict[str, Any]]]:
    gate = artifact["gate"]
    repository = gate["repositories"][0]
    grants = sorted(
        (
            grant
            for group in gate.get("write_groups", [])
            if isinstance(group, dict)
            for grant in group.get("grants", [])
            if isinstance(grant, dict)
            and grant.get("repository") == artifact["repository"]
        ),
        key=lambda item: (
            item.get("repository", ""), item.get("claim_id", ""),
            tuple(item.get("nodes", [])),
        ),
    )
    acknowledgements = [
        item for item in gate.get("acknowledgements", [])
        if isinstance(item, dict)
        and item.get("repository") == artifact["repository"]
    ]
    return repository, grants, acknowledgements


def validate_gate_package_authority(
    state_root: Path,
    run: dict[str, Any],
    gate: dict[str, Any],
) -> None:
    run_root = active_dir(state_root, run["run_id"])
    gate_repositories = {
        item["repository"]: item for item in gate["repositories"]
    }
    gate_grants: dict[str, list[dict[str, Any]]] = {}
    for group in gate["write_groups"]:
        for grant in group["grants"]:
            gate_grants.setdefault(grant["repository"], []).append(grant)
    gate_acknowledgements = {
        item["repository"]: item for item in gate["acknowledgements"]
    }
    for package in run["packages"]:
        artifact = read_json(
            run_root / "packages" / package["repository"] / "artifact.json"
        )
        repository_record, grants, acknowledgements = package_authority(artifact)
        observed_grants = sorted(
            gate_grants.get(package["repository"], []),
            key=lambda item: (
                item["repository"], item["claim_id"], tuple(item["nodes"]),
            ),
        )
        observed_acknowledgements = (
            [gate_acknowledgements[package["repository"]]]
            if package["repository"] in gate_acknowledgements else []
        )
        if (
            gate_repositories.get(package["repository"]) != repository_record
            or observed_grants != grants
            or observed_acknowledgements != acknowledgements
        ):
            raise ContractError(
                "gate-authority-unbound",
                "gate authority differs from a finalized package checkpoint",
            )


def validate_gate_shape(gate: dict[str, Any]) -> None:
    helper = Path(__file__).with_name("git-change-manifest.py")
    spec = importlib.util.spec_from_file_location("sync_gate_validator", helper)
    if spec is None or spec.loader is None:
        raise OperationalError("validator-unavailable", "gate validator could not load")
    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
        _, _, issues = module.validate_gate_for_write(gate)
    except Exception as error:
        raise OperationalError("validator-unavailable", "gate validator could not run") from error
    if issues:
        raise ContractError("gate-contract-invalid", "gate does not satisfy schema v2")


def new_unit(unit_id: str, unit_type: str, repositories: list[str], nodes: list[str]) -> dict[str, Any]:
    return {
        "unit_id": unit_id,
        "unit_type": unit_type,
        "status": "pending",
        "stale_reason": "",
        "repositories": repositories,
        "nodes": nodes,
        "failed_patch_digests": [],
        "gate_digest": "",
        "projection_digest": "",
        "patch_digest": "",
        "base_files": {},
        "result_files": {},
        "base_kinds": {},
        "result_kinds": {},
        "receipt_digest": "",
    }


def unit_authority(gate: dict[str, Any], unit: dict[str, Any]) -> Any:
    if unit["unit_type"] == "write-group":
        return next(
            (
                item for item in gate["write_groups"]
                if item["group_id"] == unit["unit_id"]
            ),
            None,
        )
    return [
        item for item in gate["acknowledgements"]
        if item["repository"] in unit["repositories"]
    ]


def unit_authority_key(gate: dict[str, Any], unit: dict[str, Any]) -> tuple[Any, ...]:
    authority = unit_authority(gate, unit)
    if unit["unit_type"] == "write-group" and isinstance(authority, dict):
        authority = {
            key: value
            for key, value in authority.items()
            if key != "group_id"
        }
    return (
        unit["unit_type"],
        tuple(unit["repositories"]),
        tuple(unit["nodes"]),
        canonical_digest(authority),
    )


def rebind_unit_checkpoint(
    state_root: Path,
    run: dict[str, Any],
    previous: dict[str, Any],
    fresh: dict[str, Any],
    gate_digest: str,
) -> dict[str, Any]:
    source = unit_dir(state_root, run["run_id"], previous["unit_id"])
    projection = read_json(source / "projection.json")
    try:
        patch = (source / "unit.patch").read_bytes()
    except OSError as error:
        raise ContractError(
            "unit-checkpoint-invalid",
            "preserved unit patch is unavailable",
        ) from error
    if (
        canonical_digest(projection) != previous["projection_digest"]
        or bytes_digest(patch) != previous["patch_digest"]
    ):
        raise ContractError(
            "unit-checkpoint-invalid",
            "preserved unit checkpoint has changed",
        )
    rebound = copy.deepcopy(previous)
    rebound["unit_id"] = fresh["unit_id"]
    rebound["gate_digest"] = gate_digest
    rebound["receipt_digest"] = ""
    projection["unit_id"] = fresh["unit_id"]
    projection["gate_digest"] = gate_digest
    rebound["projection_digest"] = canonical_digest(projection)
    target = unit_dir(state_root, run["run_id"], fresh["unit_id"])
    atomic_state_json(state_root, target / "projection.json", projection)
    atomic_state_bytes(state_root, target / "unit.patch", patch)
    return rebound


def seal_gate(args: argparse.Namespace) -> dict[str, Any]:
    run = load_run(args.state_root, args.run_id)
    if any(item["status"] != "checkpointed" for item in run["packages"]):
        raise ContractError(
            "package-checkpoint-required", "every package must have a closed checkpoint"
        )
    if any(
        unit["status"] == "stale"
        and unit["stale_reason"] == "source"
        and unit["receipt_digest"]
        for unit in run["units"]
    ):
        raise ContractError(
            "stale-retraction-required",
            "applied source-stale units must be retracted by resume before reseal",
        )
    gate = read_json(args.gate)
    validate_gate_shape(gate)
    digest = canonical_digest(gate)
    if run["gate_digest"]:
        if run["gate_digest"] == digest and not run["gate_stale"]:
            return {**public_status(args.state_root, run), "code": "gate-sealed", "reused": True}
        if not run["gate_stale"]:
            raise ContractError("gate-already-sealed", "sealed gate cannot be replaced")
    expected = {item["repository"]: item["oid"] for item in run["packages"]}
    observed = {
        item["repository"]: item["new_oid"]
        for item in gate["repositories"]
    }
    if observed != expected:
        raise ContractError(
            "gate-inventory-mismatch", "gate repository/OID coverage differs from the run"
        )
    validate_gate_package_authority(args.state_root, run, gate)
    old_units = list(run["units"])
    old_gate = (
        read_json(
            active_dir(args.state_root, args.run_id)
            / "gates" / f"{run['gate_digest']}.json"
        )
        if run["gate_digest"]
        else None
    )
    units = []
    if gate["acknowledgements"]:
        units.append(new_unit(
            "acknowledgements", "acknowledgements",
            sorted(item["repository"] for item in gate["acknowledgements"]), [],
        ))
    for group in gate["write_groups"]:
        if not isinstance(group, dict) or not isinstance(group.get("repositories"), list) or not isinstance(group.get("nodes"), list):
            raise ContractError("gate-contract-invalid", "gate write group is invalid")
        units.append(new_unit(
            group["group_id"], "write-group",
            list(group["repositories"]), list(group["nodes"]),
        ))
    if run["gate_stale"]:
        for index, fresh in enumerate(units):
            candidates = (
                []
                if old_gate is None
                else [
                    previous
                    for previous in old_units
                    if previous["status"] in {"validated", "apply-failed", "applied"}
                    and unit_authority_key(old_gate, previous)
                    == unit_authority_key(gate, fresh)
                ]
            )
            if len(candidates) == 1:
                units[index] = rebind_unit_checkpoint(
                    args.state_root,
                    run,
                    candidates[0],
                    fresh,
                    digest,
                )
    run_root = active_dir(args.state_root, args.run_id)
    gate_target = run_root / "gate.json"
    atomic_state_json(args.state_root, run_root / "gates" / f"{digest}.json", gate)
    run["gate_digest"] = digest
    run["gate_stale"] = False
    run["units"] = units
    run["status"] = "gated"
    for unit in run["units"]:
        if unit["status"] == "applied" and not unit["receipt_digest"]:
            write_unit_receipt(args.state_root, run, unit)
    save_run(args.state_root, run)
    atomic_state_json(args.state_root, gate_target, gate)
    return {**public_status(args.state_root, run), "code": "gate-sealed", "reused": False}


def status(args: argparse.Namespace) -> dict[str, Any]:
    active = run_path(args.state_root, args.run_id)
    closed = receipt_path(args.state_root, args.run_id)
    if closed.is_symlink() or closed.is_file():
        return validate_closed_receipt(args.state_root, closed, args.run_id)
    return public_status(args.state_root, load_run(args.state_root, args.run_id))


def manifest_validation(gate: Path, projection: Path, patch: Path) -> tuple[int, dict[str, Any]]:
    helper = Path(__file__).with_name("git-change-manifest.py")
    try:
        result = subprocess.run(
            [sys.executable, "-B", str(helper), "validate-projection", "--gate", str(gate), "--projection", str(projection), "--patch", str(patch)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=False,
        )
    except OSError as error:
        raise OperationalError("validator-unavailable", "projection validator could not run") from error
    try:
        payload = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise OperationalError("validator-output-invalid", "projection validator returned invalid output") from error
    if not isinstance(payload, dict):
        raise OperationalError("validator-output-invalid", "projection validator returned invalid output")
    return result.returncode, payload


def validate_unit(args: argparse.Namespace) -> dict[str, Any]:
    run = load_run(args.state_root, args.run_id)
    if not run["gate_digest"] or run["gate_stale"]:
        raise ContractError("gate-not-sealed", "a current sealed gate is required")
    unit = find_unit(run, args.unit_id)
    if unit["status"] == "applied":
        raise ContractError("unit-already-applied", "applied unit cannot be revalidated")
    try:
        projection = read_json(args.projection)
    except SyncError:
        projection = {}
        patch = b""
        patch_digest = bytes_digest(patch)
        unit["status"] = "projection-invalid"
        run["status"] = "projecting"
        save_run(args.state_root, run)
        return {
            "version": RUN_VERSION, "code": "projection-invalid",
            "status": "blocked", "retryable": True,
            "resume_from": "projection", "run_id": args.run_id,
            "unit_id": args.unit_id,
            "issues": [{"code": "projection-contract-invalid", "field": "projection"}],
        }
    try:
        patch = args.patch.read_bytes()
    except OSError:
        patch = b""
    patch_digest = bytes_digest(patch)
    try:
        patch_text = patch.decode("utf-8")
    except UnicodeError:
        patch_text = ""
        unit["status"] = "projection-invalid"
        if patch_digest not in unit["failed_patch_digests"]:
            unit["failed_patch_digests"].append(patch_digest)
            unit["failed_patch_digests"].sort()
        run["status"] = "projecting"
        save_run(args.state_root, run)
        return {
            "version": RUN_VERSION, "code": "projection-invalid",
            "status": "blocked", "retryable": True,
            "resume_from": "projection", "run_id": args.run_id,
            "unit_id": args.unit_id,
            "issues": [{"code": "patch-encoding-invalid", "field": "patch"}],
        }
    if (
        "-----BEGIN " in patch_text
        or SENSITIVE_PATCH_RE.search(patch_text)
        or TOKEN_LIKE_RE.search(patch_text)
    ):
        raise ContractError(
            "checkpoint-sensitive-content", "unit patch contains credential-like material"
        )
    projection_digest = canonical_digest(projection)
    attempt_digest = canonical_digest({
        "projection_digest": projection_digest,
        "patch_digest": patch_digest,
    })
    if attempt_digest in unit["failed_patch_digests"]:
        raise ContractError("failed-patch-digest-reused", "unchanged failed patch must not be revalidated")
    if projection.get("unit_id") != args.unit_id:
        unit["status"] = "projection-invalid"
        unit["failed_patch_digests"].append(attempt_digest)
        unit["failed_patch_digests"].sort()
        run["status"] = "projecting"
        save_run(args.state_root, run)
        return {
            "version": RUN_VERSION, "code": "projection-invalid",
            "status": "blocked", "retryable": True,
            "resume_from": "projection", "run_id": args.run_id,
            "unit_id": args.unit_id,
            "issues": [{"code": "projection-unit-mismatch", "field": "projection.unit_id"}],
        }
    if (
        projection.get("run_id") != args.run_id
        or projection.get("gate_digest") != run["gate_digest"]
        or projection.get("patch_digest") != patch_digest
    ):
        unit["status"] = "projection-invalid"
        if attempt_digest not in unit["failed_patch_digests"]:
            unit["failed_patch_digests"].append(attempt_digest)
            unit["failed_patch_digests"].sort()
        run["status"] = "projecting"
        save_run(args.state_root, run)
        return {"version": RUN_VERSION, "code": "projection-invalid", "status": "blocked", "retryable": True, "resume_from": "projection", "run_id": args.run_id, "unit_id": args.unit_id, "issues": [{"code": "projection-binding-invalid", "field": "projection"}]}
    if (
        unit["status"] == "stale"
        and unit["projection_digest"]
        and unit["projection_digest"] == projection_digest
    ):
        raise ContractError(
            "stale-projection-digest-reused",
            "destination-stale unit requires a newly bound projection",
        )
    if (
        unit["status"] == "validated"
        and unit["patch_digest"] == patch_digest
        and unit["projection_digest"] == projection_digest
    ):
        return {
            "version": RUN_VERSION,
            "code": "projection-valid",
            "status": "pass",
            "issues": [],
            "run_id": args.run_id,
            "unit_id": args.unit_id,
            "reused": True,
        }
    code, payload = manifest_validation(
        active_dir(args.state_root, args.run_id) / "gate.json", args.projection, args.patch
    )
    if code != 0 or payload.get("status") != "pass":
        unit["status"] = "projection-invalid"
        if attempt_digest not in unit["failed_patch_digests"]:
            unit["failed_patch_digests"].append(attempt_digest)
            unit["failed_patch_digests"].sort()
        run["status"] = "projecting"
        save_run(args.state_root, run)
        return {**payload, "run_id": args.run_id, "unit_id": args.unit_id}
    directory = unit_dir(args.state_root, args.run_id, args.unit_id)
    atomic_state_json(args.state_root, directory / "projection.json", projection)
    atomic_state_bytes(args.state_root, directory / "unit.patch", patch)
    unit["status"] = "validated"
    unit["stale_reason"] = ""
    unit["gate_digest"] = run["gate_digest"]
    unit["projection_digest"] = projection_digest
    unit["patch_digest"] = patch_digest
    unit["base_files"] = projection["base_files"]
    unit["result_files"] = projection["result_files"]
    sections = patch_sections(patch)
    unit["base_kinds"] = {
        path: "missing" if section[0] else "file"
        for path, section in sections.items()
    }
    unit["result_kinds"] = {
        path: "missing" if section[1] else "file"
        for path, section in sections.items()
    }
    unit["receipt_digest"] = ""
    run["status"] = "projecting"
    save_run(args.state_root, run)
    return {**payload, "run_id": args.run_id, "unit_id": args.unit_id, "reused": False}


def safe_target(vault: Path, relative: str) -> Path:
    if not valid_path(relative):
        raise ContractError("projection-path-invalid", "projection path is unsafe")
    root = vault.resolve()
    target = vault / relative
    current = Path(os.path.abspath(vault))
    if current.is_symlink():
        raise ContractError("projection-path-invalid", "vault path cannot be a symbolic link")
    for part in PurePosixPath(relative).parts:
        current = current / part
        if current.is_symlink():
            raise ContractError(
                "projection-path-invalid", "projection path cannot traverse a symbolic link"
            )
    try:
        target.resolve().relative_to(root)
    except ValueError as error:
        raise ContractError("projection-path-invalid", "projection path escapes vault") from error
    return target


def actual_digest(path: Path) -> str:
    if not path.exists():
        return bytes_digest(b"")
    if not path.is_file() or path.is_symlink():
        raise ContractError("vault-path-invalid", "projection target is not a regular file")
    try:
        return bytes_digest(path.read_bytes())
    except OSError as error:
        raise OperationalError("file-read-error", "vault target could not be read") from error


def actual_record(path: Path) -> tuple[str, str]:
    if not path.exists():
        return "missing", bytes_digest(b"")
    return "file", actual_digest(path)


def matches_unit_image(
    path: Path,
    relative: str,
    digests: dict[str, str],
    kinds: dict[str, str],
) -> bool:
    return actual_record(path) == (kinds[relative], digests[relative])


def patch_lines(raw: bytes) -> list[str]:
    try:
        lines = raw.decode("utf-8").splitlines(keepends=True)
    except UnicodeError as error:
        raise ContractError("patch-invalid", "unit patch is not UTF-8 text") from error
    cooked: list[str] = []
    for line in lines:
        if line.startswith("\\ No newline at end of file"):
            if not cooked:
                raise ContractError("patch-invalid", "orphan no-newline marker")
            cooked[-1] = cooked[-1].rstrip("\r\n")
        else:
            cooked.append(line)
    return cooked


def header_path(line: str) -> str | None:
    value = line[4:].rstrip("\r\n").split("\t", 1)[0]
    if value == "/dev/null":
        return None
    return value[2:] if value.startswith(("a/", "b/")) else value


def patch_sections(raw: bytes) -> dict[str, tuple[bool, bool, list[str]]]:
    lines = patch_lines(raw)
    sections: dict[str, tuple[bool, bool, list[str]]] = {}
    index = 0
    while index < len(lines):
        if not lines[index].startswith("--- "):
            index += 1
            continue
        old = header_path(lines[index])
        index += 1
        if index >= len(lines) or not lines[index].startswith("+++ "):
            raise ContractError("patch-invalid", "patch headers are incomplete")
        new = header_path(lines[index])
        index += 1
        path = new or old
        if path is None or not valid_path(path) or path in sections:
            raise ContractError("patch-invalid", "patch path is invalid or duplicated")
        body: list[str] = []
        old_remaining = 0
        new_remaining = 0
        saw_hunk = False
        while index < len(lines):
            line = lines[index]
            if old_remaining == 0 and new_remaining == 0:
                match = HUNK_RE.match(line)
                if match is not None:
                    old_remaining = int(match.group(2) or "1")
                    new_remaining = int(match.group(4) or "1")
                    saw_hunk = True
                    body.append(line)
                    index += 1
                    continue
                if (
                    line.startswith("--- ")
                    and index + 1 < len(lines)
                    and lines[index + 1].startswith("+++ ")
                ):
                    break
                if not line.strip():
                    body.append(line)
                    index += 1
                    continue
                raise ContractError("patch-invalid", "content appears outside a patch hunk")
            prefix = line[:1]
            if prefix == " ":
                old_remaining -= 1
                new_remaining -= 1
            elif prefix == "-":
                old_remaining -= 1
            elif prefix == "+":
                new_remaining -= 1
            else:
                raise ContractError("patch-invalid", "patch hunk line has an invalid prefix")
            if old_remaining < 0 or new_remaining < 0:
                raise ContractError("patch-invalid", "patch hunk exceeds its declared size")
            body.append(line)
            index += 1
        if not saw_hunk or old_remaining or new_remaining:
            raise ContractError("patch-invalid", "patch hunk is incomplete")
        sections[path] = (old is None, new is None, body)
    return sections


def apply_hunks(base: bytes, body: list[str]) -> bytes:
    try:
        source = base.decode("utf-8").splitlines(keepends=True)
    except UnicodeError as error:
        raise ContractError("patch-invalid", "base file is not UTF-8 text") from error
    output: list[str] = []
    cursor = 0
    index = 0
    while index < len(body):
        match = HUNK_RE.match(body[index])
        if match is None:
            if body[index].strip():
                raise ContractError("patch-invalid", "unexpected patch content")
            index += 1
            continue
        old_start = int(match.group(1))
        start = 0 if old_start == 0 else old_start - 1
        if start < cursor or start > len(source):
            raise ContractError("patch-does-not-apply", "patch hunk offset is invalid")
        output.extend(source[cursor:start])
        cursor = start
        index += 1
        while index < len(body) and HUNK_RE.match(body[index]) is None:
            line = body[index]
            if not line:
                index += 1
                continue
            prefix, content = line[0], line[1:]
            if prefix == " ":
                if cursor >= len(source) or source[cursor] != content:
                    raise ContractError("patch-does-not-apply", "patch context does not match")
                output.append(source[cursor])
                cursor += 1
            elif prefix == "-":
                if cursor >= len(source) or source[cursor] != content:
                    raise ContractError("patch-does-not-apply", "patch deletion does not match")
                cursor += 1
            elif prefix == "+":
                output.append(content)
            elif line.strip():
                raise ContractError("patch-invalid", "patch line has an invalid prefix")
            index += 1
    output.extend(source[cursor:])
    return "".join(output).encode("utf-8")


def write_unit_receipt(state_root: Path, run: dict[str, Any], unit: dict[str, Any]) -> None:
    receipt = {
        "version": RUN_VERSION,
        "run_id": run["run_id"],
        "unit_id": unit["unit_id"],
        "gate_digest": unit["gate_digest"],
        "projection_digest": unit["projection_digest"],
        "patch_digest": unit["patch_digest"],
        "base_files": unit["base_files"],
        "result_files": unit["result_files"],
        "base_kinds": unit["base_kinds"],
        "result_kinds": unit["result_kinds"],
        "status": "applied",
    }
    unit["receipt_digest"] = canonical_digest(receipt)
    atomic_state_json(
        state_root,
        unit_dir(state_root, run["run_id"], unit["unit_id"]) / "receipt.json",
        receipt,
    )


def resolve_vault(state_root: Path, locator: str) -> Path | None:
    if not locator:
        return None
    return (state_root.resolve() / locator).resolve()


def checkpoint_patch_images(
    state_root: Path,
    run_id: str,
    unit: dict[str, Any],
) -> dict[str, tuple[bytes, bytes]]:
    directory = unit_dir(state_root, run_id, unit["unit_id"])
    try:
        patch = (directory / "unit.patch").read_bytes()
        patch_text = patch.decode("utf-8", errors="strict")
    except (OSError, UnicodeError) as error:
        raise ContractError(
            "unit-checkpoint-invalid",
            "stale unit patch is unavailable",
        ) from error
    if bytes_digest(patch) != unit["patch_digest"]:
        raise ContractError(
            "unit-checkpoint-invalid",
            "stale unit patch has changed",
        )
    helper = Path(__file__).with_name("git-change-manifest.py")
    spec = importlib.util.spec_from_file_location("sync_patch_images", helper)
    if spec is None or spec.loader is None:
        raise OperationalError("validator-unavailable", "patch image validator could not load")
    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
        sections, issues = module.projected_patch_sections(patch_text)
        images = {
            path: module.complete_patch_images(section)
            for path, section in sections.items()
        }
    except Exception as error:
        raise OperationalError("validator-unavailable", "patch images could not be reconstructed") from error
    if issues or set(images) != set(unit["base_files"]) or any(
        value is None for value in images.values()
    ):
        raise ContractError(
            "unit-checkpoint-invalid",
            "stale unit patch does not contain complete bound images",
        )
    complete = {path: value for path, value in images.items() if value is not None}
    if any(
        bytes_digest(old) != unit["base_files"][path]
        or bytes_digest(new) != unit["result_files"][path]
        for path, (old, new) in complete.items()
    ):
        raise ContractError(
            "unit-checkpoint-invalid",
            "stale unit images differ from their recorded digests",
        )
    return complete


def retract_stale_unit(
    state_root: Path,
    run: dict[str, Any],
    unit: dict[str, Any],
    vault: Path,
) -> None:
    images = checkpoint_patch_images(state_root, run["run_id"], unit)
    observed = {
        path: actual_record(safe_target(vault, path))
        for path in sorted(unit["result_files"])
    }
    base_records = {
        path: (unit["base_kinds"][path], unit["base_files"][path])
        for path in observed
    }
    result_records = {
        path: (unit["result_kinds"][path], unit["result_files"][path])
        for path in observed
    }
    if any(
        observed[path] not in {base_records[path], result_records[path]}
        for path in observed
    ):
        raise ContractError(
            "stale-publication-conflict",
            "published bytes changed and cannot be retracted automatically",
        )
    staged = [
        path for path in sorted(observed)
        if observed[path] == result_records[path] and result_records[path] != base_records[path]
    ]
    written: list[str] = []
    try:
        for path in staged:
            target = safe_target(vault, path)
            old_bytes, _ = images[path]
            if unit["base_kinds"][path] == "missing":
                target.unlink(missing_ok=True)
            else:
                prepare_vault_parent(vault, target)
                atomic_bytes(target, old_bytes, mode=0o644)
            written.append(path)
    except (OSError, SyncError) as error:
        rollback_failed = False
        for path in reversed(written):
            target = safe_target(vault, path)
            _, new_bytes = images[path]
            try:
                if unit["result_kinds"][path] == "missing":
                    target.unlink(missing_ok=True)
                else:
                    prepare_vault_parent(vault, target)
                    atomic_bytes(target, new_bytes, mode=0o644)
            except (OSError, SyncError):
                rollback_failed = True
        if rollback_failed:
            raise OperationalError(
                "rollback-failed",
                "stale publication rollback could not restore its postimage",
            ) from error
        raise OperationalError(
            "publication-retraction-failed",
            "stale publication could not be retracted",
        ) from error
    if any(actual_record(safe_target(vault, path)) != base_records[path] for path in observed):
        raise OperationalError(
            "publication-retraction-failed",
            "stale publication preimage verification failed",
        )


def apply_journal_path(state_root: Path, run_id: str, unit_id: str) -> Path:
    return unit_dir(state_root, run_id, unit_id) / "apply-journal.json"


def remove_state_file(state_root: Path, path: Path) -> None:
    require_state_write_path(state_root, path)
    try:
        path.unlink()
    except FileNotFoundError:
        pass
    except OSError as error:
        raise OperationalError("file-write-error", "state journal could not be removed") from error


def detach_active_run(state_root: Path, run_id: str, reason: str) -> Path:
    source = active_dir(state_root, run_id)
    retired_root = state_root / "retired"
    if retired_root.is_symlink():
        raise ContractError("state-path-invalid", "retired run state cannot be a symbolic link")
    try:
        retired_root.mkdir(parents=True, exist_ok=True)
    except OSError as error:
        raise OperationalError("run-cleanup-failed", "retired run state cannot be prepared") from error
    target = retired_root / f"{run_id}-{reason}"
    require_state_write_path(state_root, target)
    if target.exists() or target.is_symlink():
        raise ContractError("state-path-invalid", "retired run target already exists")
    try:
        os.replace(source, target)
    except OSError as error:
        raise OperationalError("run-cleanup-failed", "active run could not be detached") from error
    return target


def retire_closed_active(state_root: Path, receipt: dict[str, Any]) -> None:
    run_id = receipt["run_id"]
    source = active_dir(state_root, run_id)
    if not source.exists() and not source.is_symlink():
        return
    if source.is_symlink():
        raise ContractError(
            "state-path-invalid",
            "closed active run cannot be a symbolic link",
        )
    state_file = source / "run.json"
    if state_file.is_file():
        try:
            active_value = read_json(state_file)
        except SyncError:
            active_value = {}
        if active_value and (
            active_value.get("run_id") != run_id
            or active_value.get("fingerprint") != receipt["fingerprint"]
        ):
            raise ContractError(
                "active-receipt-conflict",
                "active run identity differs from its durable closed receipt",
            )
    retired = detach_active_run(state_root, run_id, "closed-orphan")
    try:
        shutil.rmtree(retired)
    except OSError:
        pass


def prepare_vault_parent(vault: Path, target: Path) -> None:
    try:
        target.parent.mkdir(parents=True, exist_ok=True)
    except OSError as error:
        raise OperationalError(
            "file-write-error", "projection target parent cannot be prepared"
        ) from error
    if not target.parent.is_dir():
        raise OperationalError(
            "file-write-error", "projection target parent is not a directory"
        )


def apply_unit(args: argparse.Namespace) -> dict[str, Any]:
    run = load_run(args.state_root, args.run_id)
    unit = find_unit(run, args.unit_id)
    if unit["status"] == "applied":
        locator = os.path.relpath(args.vault.resolve(), args.state_root.resolve())
        if run["vault_locator"] and run["vault_locator"] != locator:
            raise ContractError("vault-mismatch", "run units must apply to one vault")
        if not all(
            matches_unit_image(
                safe_target(args.vault, path), path,
                unit["result_files"], unit["result_kinds"],
            )
            for path in unit["result_files"]
        ):
            raise ContractError(
                "applied-unit-stale", "applied unit no longer has its verified postimage"
            )
        return {"version": RUN_VERSION, "code": "unit-applied", "status": "pass", "run_id": args.run_id, "unit_id": args.unit_id, "receipt_digest": unit["receipt_digest"], "reused": True}
    if unit["status"] not in {"validated", "apply-failed"}:
        raise ContractError("unit-not-validated", "unit must validate before apply")
    directory = unit_dir(args.state_root, args.run_id, args.unit_id)
    projection = read_json(directory / "projection.json")
    try:
        patch = (directory / "unit.patch").read_bytes()
    except OSError as error:
        raise OperationalError("file-read-error", "checkpointed patch could not be read") from error
    if canonical_digest(projection) != unit["projection_digest"] or bytes_digest(patch) != unit["patch_digest"] or projection.get("gate_digest") != unit["gate_digest"]:
        raise ContractError("unit-checkpoint-invalid", "validated unit checkpoint has changed")
    locator = os.path.relpath(args.vault.resolve(), args.state_root.resolve())
    if run["vault_locator"] and run["vault_locator"] != locator:
        raise ContractError("vault-mismatch", "run units must apply to one vault")
    run["vault_locator"] = locator
    save_run(args.state_root, run)
    sections = patch_sections(patch)
    expected_paths = set(unit["base_files"])
    if set(sections) != expected_paths or expected_paths != set(unit["result_files"]):
        raise ContractError("unit-checkpoint-invalid", "patch paths do not match projection")
    staged: dict[str, bytes | None] = {}
    originals: dict[str, bytes | None] = {}
    observed = {}
    for relative in sorted(expected_paths):
        target = safe_target(args.vault, relative)
        observed[relative] = actual_record(target)
        before, after = unit["base_files"][relative], unit["result_files"][relative]
        before_record = (unit["base_kinds"][relative], before)
        after_record = (unit["result_kinds"][relative], after)
        if observed[relative] == after_record:
            continue
        if observed[relative] != before_record:
            unit["status"] = "apply-failed"
            save_run(args.state_root, run)
            raise ContractError("apply-failed", "vault bytes do not match projection preimage")
        created, deleted, body = sections[relative]
        base = b"" if unit["base_kinds"][relative] == "missing" else target.read_bytes()
        result = None if deleted else apply_hunks(base, body)
        result_record = (
            "missing" if result is None else "file",
            bytes_digest(b"" if result is None else result),
        )
        if result_record != after_record:
            unit["status"] = "apply-failed"
            save_run(args.state_root, run)
            raise ContractError("apply-failed", "patched bytes do not match projection postimage")
        staged[relative] = result
        originals[relative] = None if observed[relative][0] == "missing" else base
    preimage_paths = {
        path
        for path, record in observed.items()
        if record == (unit["base_kinds"][path], unit["base_files"][path])
    }
    postimage_paths = {
        path
        for path, record in observed.items()
        if record == (unit["result_kinds"][path], unit["result_files"][path])
    }
    if (preimage_paths - postimage_paths) and (postimage_paths - preimage_paths):
        unit["status"] = "apply-failed"
        save_run(args.state_root, run)
        raise ContractError(
            "atomic-unit-interrupted",
            "unit has a mixed preimage/postimage and cannot roll forward",
        )
    try:
        for relative in sorted(staged):
            prepare_vault_parent(args.vault, safe_target(args.vault, relative))
    except OperationalError:
        unit["status"] = "apply-failed"
        save_run(args.state_root, run)
        raise
    journal = {
        "version": RUN_VERSION,
        "run_id": args.run_id,
        "unit_id": args.unit_id,
        "patch_digest": unit["patch_digest"],
        "status": "prepared",
        "entries": [
            {
                "path": relative,
                "before_digest": unit["base_files"][relative],
                "before_kind": unit["base_kinds"][relative],
                "after_digest": unit["result_files"][relative],
                "after_kind": unit["result_kinds"][relative],
                "status": "already-applied" if relative not in staged else "pending",
            }
            for relative in sorted(expected_paths)
        ],
    }
    journal_path = apply_journal_path(args.state_root, args.run_id, args.unit_id)
    atomic_state_json(args.state_root, journal_path, journal)
    written: list[str] = []

    def rollback_written() -> bool:
        failed = False
        for relative in reversed(written):
            target = safe_target(args.vault, relative)
            original = originals[relative]
            try:
                if original is None:
                    target.unlink(missing_ok=True)
                else:
                    atomic_bytes(target, original, mode=0o644)
            except (OSError, SyncError):
                failed = True
        if not failed:
            remove_state_file(args.state_root, journal_path)
        return not failed

    try:
        for relative, content in staged.items():
            target = safe_target(args.vault, relative)
            if content is None:
                try:
                    target.unlink()
                except FileNotFoundError:
                    pass
                except OSError as error:
                    raise OperationalError("file-write-error", "projected deletion failed") from error
            else:
                atomic_bytes(target, content, mode=0o644)
            written.append(relative)
            for entry in journal["entries"]:
                if entry["path"] == relative:
                    entry["status"] = "applied"
                    break
            journal["status"] = "applying"
            atomic_state_json(args.state_root, journal_path, journal)
            if args.crash_after_writes == len(written):
                os._exit(99)
            if args.fail_after_writes == len(written):
                raise OperationalError(
                    "simulated-write-failure",
                    "process encountered a write failure during unit application",
                )
    except (ContractError, OperationalError) as error:
        restored = rollback_written()
        unit["status"] = "apply-failed"
        save_run(args.state_root, run)
        if not restored:
            raise OperationalError(
                "rollback-failed", "unit rollback could not restore every preimage"
            ) from error
        raise error
    for relative in unit["result_files"]:
        if not matches_unit_image(
            safe_target(args.vault, relative), relative,
            unit["result_files"], unit["result_kinds"],
        ):
            restored = rollback_written()
            unit["status"] = "apply-failed"
            save_run(args.state_root, run)
            if not restored:
                raise OperationalError(
                    "rollback-failed", "unit rollback could not restore every preimage"
                )
            raise OperationalError("apply-failed", "vault postimage verification failed")
    if args.crash_after_bytes:
        raise OperationalError("simulated-crash", "process stopped after applying bytes")
    unit["status"] = "applied"
    write_unit_receipt(args.state_root, run, unit)
    if run["units"] and all(item["status"] == "applied" for item in run["units"]):
        run["status"] = "closing"
    save_run(args.state_root, run)
    remove_state_file(args.state_root, journal_path)
    return {"version": RUN_VERSION, "code": "unit-applied", "status": "pass", "run_id": args.run_id, "unit_id": args.unit_id, "receipt_digest": unit["receipt_digest"], "reused": not bool(staged)}


def unit_matches_destination(unit: dict[str, Any], path: str) -> bool:
    if path in unit["base_files"]:
        return True
    if unit["unit_type"] == "acknowledgements":
        return path == "90-Meta/.sync-acknowledgements.json"
    return valid_path(path) and PurePosixPath(path).stem in unit["nodes"]


def resume(args: argparse.Namespace) -> dict[str, Any]:
    run = load_run(args.state_root, args.run_id)
    if args.tool_digest is not None:
        require_digest(args.tool_digest, "tool_digest")
        if args.tool_digest != run["tool_digest"]:
            raise ContractError("run-version-mismatch", "tool digest changed; run cannot migrate")
    invalidated_packages: set[str] = set()
    invalidated_units: set[str] = {
        unit["unit_id"]
        for unit in run["units"]
        if unit["status"] == "stale"
    }
    for repository, oid in args.source_oid:
        if not valid_name(repository) or OID_RE.fullmatch(oid) is None:
            raise ContractError("package-contract-invalid", "source identity is invalid")
        package = next((item for item in run["packages"] if item["repository"] == repository), None)
        if package is None:
            raise ContractError("package-not-found", "source is not part of this run")
        if oid != package["oid"]:
            package["oid"] = oid
            package["status"] = "stale"
            package["artifact_digest"] = ""
            checkpoint = (
                active_dir(args.state_root, args.run_id)
                / "packages" / repository / "artifact.json"
            )
            invalidated_packages.add(repository)
            for unit in run["units"]:
                if repository in unit["repositories"]:
                    unit["status"] = "stale"
                    unit["stale_reason"] = "source"
                    invalidated_units.add(unit["unit_id"])
            run["gate_stale"] = True
            run["status"] = "packages"
            run["fingerprint"] = run_fingerprint(
                run["tool_digest"], run["inventory_digest"],
                [(item["repository"], item["oid"]) for item in run["packages"]],
            )
            save_run(args.state_root, run)
            remove_state_file(args.state_root, checkpoint)
    retracted_units: list[str] = []
    stale_vault = resolve_vault(args.state_root, run["vault_locator"])
    for unit in run["units"]:
        if (
            unit["status"] != "stale"
            or unit["stale_reason"] != "source"
            or not unit["receipt_digest"]
        ):
            continue
        if stale_vault is None:
            raise ContractError(
                "stale-publication-unavailable",
                "published stale unit has no recoverable vault locator",
            )
        retract_stale_unit(args.state_root, run, unit, stale_vault)
        unit["stale_reason"] = "source-retracted"
        retracted_units.append(unit["unit_id"])
        save_run(args.state_root, run)
    if invalidated_packages:
        indexed = fingerprint_receipt_path(args.state_root, run["fingerprint"])
        if indexed.is_symlink() or indexed.is_file():
            indexed_value = read_json(indexed)
            indexed_run_id = indexed_value.get("run_id")
            if not valid_name(indexed_run_id):
                raise ContractError(
                    "run-receipt-invalid",
                    "fingerprint receipt identity is invalid",
                )
            receipt = validate_closed_receipt(
                args.state_root,
                indexed,
                indexed_run_id,
                expected_fingerprint=run["fingerprint"],
            )
            retired = detach_active_run(
                args.state_root,
                args.run_id,
                f"superseded-{run['fingerprint'][:12]}",
            )
            try:
                shutil.rmtree(retired)
            except OSError:
                pass
            return {
                **receipt,
                "reused": True,
                "retracted_units": sorted(retracted_units),
            }
    for path, digest in args.destination_digest:
        if not valid_path(path) or DIGEST_RE.fullmatch(digest) is None:
            raise ContractError("destination-contract-invalid", "destination identity is invalid")
        for unit in run["units"]:
            if unit_matches_destination(unit, path):
                expected = (
                    unit["result_files"].get(path)
                    if unit["status"] == "applied"
                    else unit["base_files"].get(path)
                )
                if expected is None or expected != digest:
                    unit["status"] = "stale"
                    unit["stale_reason"] = "destination"
                    invalidated_units.add(unit["unit_id"])
    reconciled: list[str] = []
    vault = resolve_vault(args.state_root, run["vault_locator"])
    if vault is not None:
        for unit in run["units"]:
            if unit["unit_id"] in invalidated_units or unit["status"] not in {"validated", "apply-failed"} or not unit["result_files"]:
                continue
            after_matches = {
                path: matches_unit_image(
                    safe_target(vault, path), path,
                    unit["result_files"], unit["result_kinds"],
                )
                for path in unit["result_files"]
            }
            changed_matches = {
                path: matches for path, matches in after_matches.items()
                if (unit["base_kinds"][path], unit["base_files"][path])
                != (unit["result_kinds"][path], unit["result_files"][path])
            }
            if any(changed_matches.values()) and not all(changed_matches.values()):
                unit["status"] = "apply-failed"
                save_run(args.state_root, run)
                raise ContractError(
                    "atomic-unit-interrupted",
                    "unit has a mixed preimage/postimage and cannot roll forward",
                )
            if all(after_matches.values()):
                unit["status"] = "applied"
                write_unit_receipt(args.state_root, run, unit)
                reconciled.append(unit["unit_id"])
    if run["units"] and all(unit["status"] == "applied" for unit in run["units"]):
        run["status"] = "closing"
    elif run["gate_digest"] and not run["gate_stale"]:
        run["status"] = "projecting"
    save_run(args.state_root, run)
    for unit_id in reconciled:
        remove_state_file(
            args.state_root,
            apply_journal_path(args.state_root, args.run_id, unit_id),
        )
    reused = sorted(
        unit["unit_id"] for unit in run["units"] if unit["unit_id"] not in invalidated_units
    )
    code = "source-stale" if invalidated_packages else "vault-baseline-stale" if invalidated_units else "run-resumable"
    return {
        **public_status(args.state_root, run),
        "code": code,
        "invalidated_packages": sorted(invalidated_packages),
        "invalidated_units": sorted(invalidated_units),
        "reused_units": reused,
        "reconciled_units": sorted(reconciled),
        "retracted_units": sorted(retracted_units),
    }


def close(args: argparse.Namespace) -> dict[str, Any]:
    closed_path = receipt_path(args.state_root, args.run_id)
    if closed_path.is_symlink() or closed_path.is_file():
        receipt = validate_closed_receipt(
            args.state_root,
            closed_path,
            args.run_id,
        )
        retire_closed_active(args.state_root, receipt)
        return receipt
    run = load_run(args.state_root, args.run_id)
    if (
        not run["gate_digest"]
        or run["gate_stale"]
        or any(unit["status"] != "applied" for unit in run["units"])
    ):
        raise ContractError("run-not-closable", "all units must be applied before close")
    vault = resolve_vault(args.state_root, run["vault_locator"])
    if run["units"] and vault is None:
        raise ContractError("run-not-closable", "applied unit vault is unavailable")
    stale_units = []
    if vault is not None:
        for unit in run["units"]:
            if not all(
                matches_unit_image(
                    safe_target(vault, path), path,
                    unit["result_files"], unit["result_kinds"],
                )
                for path in unit["result_files"]
            ):
                unit["status"] = "stale"
                unit["stale_reason"] = "destination"
                stale_units.append(unit["unit_id"])
    if stale_units:
        run["status"] = "projecting"
        save_run(args.state_root, run)
        raise ContractError(
            "vault-baseline-stale", "applied unit postimage changed before close"
        )
    run["status"] = "complete"
    receipt = receipt_view(run)
    receipt["code"] = "run-closed"
    receipt["receipt_digest"] = canonical_digest(receipt)
    atomic_state_json(args.state_root, closed_path, receipt)
    atomic_state_json(
        args.state_root,
        fingerprint_receipt_path(args.state_root, run["fingerprint"]),
        receipt,
    )
    retired = detach_active_run(args.state_root, args.run_id, "closed")
    try:
        shutil.rmtree(retired)
    except OSError as error:
        raise OperationalError("run-cleanup-failed", "closed active run could not be removed") from error
    return receipt


def parser() -> StableArgumentParser:
    root = StableArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    begin_command = commands.add_parser("begin")
    begin_command.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    begin_command.add_argument("--tool-digest", required=True)
    begin_command.add_argument("--inventory-digest", required=True)
    begin_command.add_argument("--package", nargs=2, action="append", default=[], metavar=("REPOSITORY", "OID"))
    checkpoint = commands.add_parser("checkpoint-package")
    checkpoint.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    checkpoint.add_argument("--run-id", required=True)
    checkpoint.add_argument("--repository", required=True)
    checkpoint.add_argument("--artifact", type=Path, required=True)
    seal = commands.add_parser("seal-gate")
    seal.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    seal.add_argument("--run-id", required=True)
    seal.add_argument("--gate", type=Path, required=True)
    status_command = commands.add_parser("status")
    status_command.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    status_command.add_argument("--run-id", required=True)
    validate = commands.add_parser("validate-unit")
    validate.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    validate.add_argument("--run-id", required=True)
    validate.add_argument("--unit-id", required=True)
    validate.add_argument("--projection", type=Path, required=True)
    validate.add_argument("--patch", type=Path, required=True)
    apply = commands.add_parser("apply-unit")
    apply.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    apply.add_argument("--run-id", required=True)
    apply.add_argument("--unit-id", required=True)
    apply.add_argument("--vault", type=Path, required=True)
    apply.add_argument("--crash-after-bytes", action="store_true", help=argparse.SUPPRESS)
    apply.add_argument("--fail-after-writes", type=int, default=0, help=argparse.SUPPRESS)
    apply.add_argument("--crash-after-writes", type=int, default=0, help=argparse.SUPPRESS)
    resume_command = commands.add_parser("resume")
    resume_command.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    resume_command.add_argument("--run-id", required=True)
    resume_command.add_argument("--source-oid", nargs=2, action="append", default=[], metavar=("REPOSITORY", "OID"))
    resume_command.add_argument("--destination-digest", nargs=2, action="append", default=[], metavar=("PATH", "SHA256"))
    resume_command.add_argument("--tool-digest")
    close_command = commands.add_parser("close")
    close_command.add_argument("--state-root", type=Path, default=DEFAULT_STATE_ROOT)
    close_command.add_argument("--run-id", required=True)
    return root


def main(argv: list[str] | None = None) -> int:
    try:
        args = parser().parse_args(argv)
        handlers = {
            "begin": begin,
            "checkpoint-package": checkpoint_package,
            "seal-gate": seal_gate,
            "status": status,
            "validate-unit": validate_unit,
            "apply-unit": apply_unit,
            "resume": resume,
            "close": close,
        }
        payload = handlers[args.command](args)
        emit(payload)
        if payload.get("status") == "blocked":
            return 2
        if payload.get("status") == "error":
            return 1
        return 0
    except SyncError as error:
        emit({
            "version": RUN_VERSION,
            "code": error.code,
            "status": "blocked" if error.exit_code == 2 else "error",
            "message": str(error),
        })
        return error.exit_code
    except Exception:
        emit({
            "version": RUN_VERSION,
            "code": "internal-error",
            "status": "error",
            "message": "unexpected internal failure",
        })
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
