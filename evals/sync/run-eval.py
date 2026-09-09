#!/usr/bin/env python3
"""Build and close deterministic blind-review bundles for synchronization."""
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
import tempfile
from pathlib import Path, PurePosixPath
from typing import Any, NoReturn


TEST_COMMANDS = (
    ("sync-pipeline", "python3", "-B", "evals/sync/test_sync_pipeline.py"),
    ("sync-semantics", "python3", "-B", "evals/sync/test_sync_semantics.py"),
    ("sync-state", "python3", "-B", "evals/sync/test_sync_state.py"),
    ("eval-harness", "python3", "-B", "evals/sync/test_run_eval.py"),
    ("sync-tooling", "python3", "-B", "evals/bootstrap/test_sync_tooling.py"),
    ("bootstrap", "python3", "-B", "evals/bootstrap/test_bootstrap.py"),
    ("instance", "python3", "-B", "kernel/90-Meta/test_instance.py"),
)
REVIEW_FIELDS = {
    "role", "agent_id", "model", "reasoning_effort", "fork_turns",
    "input_digest", "findings",
}
FINDING_FIELDS = {
    "finding_id", "severity", "requirement", "file", "line", "claim",
    "evidence", "disposition",
}
DIGEST_RE = re.compile(r"^[0-9a-f]{64}$")
FINDING_ID_RE = re.compile(r"^[A-Z]+-[0-9]{3}$")
REQUIREMENT_RE = re.compile(r"^(REQ|SEC|CON|GUD|PAT)-[0-9]{3}$")


class EvalError(Exception):
    pass


class StableParser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise EvalError("invalid command arguments")


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(
        value, ensure_ascii=True, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def digest_json(value: Any) -> str:
    return digest_bytes(canonical_bytes(value))


def atomic_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, raw = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temporary = Path(raw)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(canonical_bytes(value) + b"\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary.exists():
            temporary.unlink()


def git(root: Path, *arguments: str) -> bytes:
    result = subprocess.run(
        ("git", *arguments), cwd=root, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, check=False,
    )
    if result.returncode:
        raise EvalError("git command failed")
    return result.stdout


def candidate_paths(root: Path) -> list[str]:
    raw = git(root, "ls-files", "-co", "--exclude-standard", "-z")
    paths = sorted({item.decode("utf-8") for item in raw.split(b"\0") if item})
    for path in paths:
        candidate = PurePosixPath(path)
        if candidate.is_absolute() or ".." in candidate.parts:
            raise EvalError("candidate path is unsafe")
    return paths


def file_record(root: Path, relative: str) -> dict[str, Any]:
    path = root / relative
    if not path.exists() and not path.is_symlink():
        return {
            "path": relative,
            "kind": "deleted",
            "executable": False,
            "digest": digest_bytes(b""),
            "size": 0,
        }
    observed = path.lstat()
    if stat.S_ISLNK(observed.st_mode):
        content = os.readlink(path).encode("utf-8")
        kind = "symlink"
    elif stat.S_ISREG(observed.st_mode):
        content = path.read_bytes()
        kind = "file"
    else:
        raise EvalError("candidate contains an unsupported file type")
    return {
        "path": relative,
        "kind": kind,
        "executable": bool(observed.st_mode & stat.S_IXUSR),
        "digest": digest_bytes(content),
        "size": len(content),
    }


def tree_manifest(root: Path) -> list[dict[str, Any]]:
    return [file_record(root, path) for path in candidate_paths(root)]


def copy_snapshot(root: Path, target: Path, manifest: list[dict[str, Any]]) -> None:
    if target.exists():
        return
    for record in manifest:
        if record["kind"] == "deleted":
            continue
        source = root / record["path"]
        destination = target / record["path"]
        destination.parent.mkdir(parents=True, exist_ok=True)
        if record["kind"] == "symlink":
            destination.symlink_to(os.readlink(source))
        else:
            shutil.copyfile(source, destination)
            destination.chmod(0o755 if record["executable"] else 0o644)


def test_count(output: str) -> int | None:
    matches = re.findall(r"^Ran ([0-9]+) tests? in ", output, flags=re.MULTILINE)
    return int(matches[-1]) if matches else None


def run_tests(root: Path) -> tuple[dict[str, Any], dict[str, str]]:
    records = []
    outputs: dict[str, str] = {}
    for name, *command in TEST_COMMANDS:
        result = subprocess.run(
            command, cwd=root, text=True, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, check=False,
        )
        combined = result.stdout + result.stderr
        outputs[name] = combined
        records.append({
            "name": name,
            "command": command,
            "returncode": result.returncode,
            "status": "pass" if result.returncode == 0 else "fail",
            "test_count": test_count(combined),
        })
    return {
        "version": 1,
        "status": "pass" if all(item["returncode"] == 0 for item in records) else "fail",
        "tests": records,
    }, outputs


def existing_round(eval_root: Path, tree_digest: str) -> int | None:
    bundle = eval_root / tree_digest / "bundle.json"
    if not bundle.is_file():
        return None
    value = json.loads(bundle.read_text(encoding="utf-8"))
    return value.get("round") if isinstance(value, dict) else None


def next_round(eval_root: Path) -> int:
    rounds = []
    if eval_root.is_dir():
        for path in eval_root.glob("*/bundle.json"):
            try:
                value = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError):
                continue
            if isinstance(value, dict) and type(value.get("round")) is int:
                rounds.append(value["round"])
    return max(rounds, default=-1) + 1


def prepare(args: argparse.Namespace) -> dict[str, Any]:
    root = args.repo_root.resolve()
    criteria = root / "evals/sync/criteria.md"
    manifest = tree_manifest(root)
    tree_digest = digest_json(manifest)
    bundle_root = args.eval_root.resolve() / tree_digest
    prior_round = existing_round(args.eval_root.resolve(), tree_digest)
    if prior_round is not None:
        bundle = json.loads((bundle_root / "bundle.json").read_text(encoding="utf-8"))
        review_input = json.loads((bundle_root / "review-input.json").read_text(encoding="utf-8"))
        test_receipt = json.loads((bundle_root / "test-receipt.json").read_text(encoding="utf-8"))
        return {
            "version": 1,
            "code": "eval-bundle-ready",
            "status": test_receipt["status"],
            "round": prior_round,
            "tree_digest": tree_digest,
            "bundle_root": str(bundle_root),
            "input_digest": review_input["input_digest"],
            "test_receipt_digest": bundle["test_receipt_digest"],
            "reused": True,
        }
    round_number = prior_round if prior_round is not None else next_round(args.eval_root.resolve())
    test_receipt, outputs = run_tests(root)
    test_receipt_digest = digest_json(test_receipt)
    base_sha = git(root, "rev-parse", "HEAD").decode("ascii").strip()
    changed = sorted(
        item.decode("utf-8")
        for item in git(root, "status", "--porcelain=v1", "-z").split(b"\0")
        if item
    )
    bundle = {
        "version": 1,
        "round": round_number,
        "base_sha": base_sha,
        "candidate_tree_digest": tree_digest,
        "criteria_digest": digest_bytes(criteria.read_bytes()),
        "test_receipt_digest": test_receipt_digest,
        "files": manifest,
        "changed_status": changed,
    }
    review_input = {
        "version": 1,
        "round": round_number,
        "base_sha": base_sha,
        "candidate_tree_digest": tree_digest,
        "criteria_digest": bundle["criteria_digest"],
        "test_receipt_digest": test_receipt_digest,
        "files": [item["path"] for item in manifest],
    }
    review_input["input_digest"] = digest_json(review_input)
    bundle_root.mkdir(parents=True, exist_ok=True)
    copy_snapshot(root, bundle_root / "snapshot", manifest)
    atomic_json(bundle_root / "bundle.json", bundle)
    atomic_json(bundle_root / "test-receipt.json", test_receipt)
    atomic_json(bundle_root / "review-input.json", review_input)
    for name, output in outputs.items():
        (bundle_root / "test-output").mkdir(parents=True, exist_ok=True)
        (bundle_root / "test-output" / f"{name}.txt").write_text(output, encoding="utf-8")
    (bundle_root / "candidate.diff").write_bytes(git(root, "diff", "--binary", "HEAD", "--"))
    result = {
        "version": 1,
        "code": "eval-bundle-ready",
        "status": test_receipt["status"],
        "round": round_number,
        "tree_digest": tree_digest,
        "bundle_root": str(bundle_root),
        "input_digest": review_input["input_digest"],
        "test_receipt_digest": test_receipt_digest,
        "reused": prior_round is not None,
    }
    return result


def safe_relative_file(value: Any) -> bool:
    if not isinstance(value, str) or not value or "\\" in value:
        return False
    path = PurePosixPath(value)
    return not path.is_absolute() and ".." not in path.parts


def validate_review(value: Any, role: str, input_digest: str) -> list[str]:
    issues = []
    if not isinstance(value, dict) or set(value) != REVIEW_FIELDS:
        return ["review-fields-invalid"]
    if value["role"] != role:
        issues.append("review-role-invalid")
    for field in ("agent_id", "model", "reasoning_effort", "fork_turns"):
        if not isinstance(value[field], str) or not value[field].strip():
            issues.append(f"review-{field}-invalid")
    if (
        not isinstance(value["input_digest"], str)
        or value["input_digest"] != input_digest
        or DIGEST_RE.fullmatch(value["input_digest"]) is None
    ):
        issues.append("review-input-digest-invalid")
    findings = value["findings"]
    if not isinstance(findings, list):
        return issues + ["review-findings-invalid"]
    ids = []
    for index, finding in enumerate(findings):
        prefix = f"finding-{index}"
        if not isinstance(finding, dict) or set(finding) != FINDING_FIELDS:
            issues.append(f"{prefix}-fields-invalid")
            continue
        if isinstance(finding["finding_id"], str):
            ids.append(finding["finding_id"])
        if not isinstance(finding["finding_id"], str) or FINDING_ID_RE.fullmatch(finding["finding_id"]) is None:
            issues.append(f"{prefix}-id-invalid")
        if finding["severity"] not in {"P0", "P1", "P2", "P3"}:
            issues.append(f"{prefix}-severity-invalid")
        if not isinstance(finding["requirement"], str) or REQUIREMENT_RE.fullmatch(finding["requirement"]) is None:
            issues.append(f"{prefix}-requirement-invalid")
        if not safe_relative_file(finding["file"]):
            issues.append(f"{prefix}-file-invalid")
        if type(finding["line"]) is not int or finding["line"] < 1:
            issues.append(f"{prefix}-line-invalid")
        for field in ("claim", "evidence"):
            if not isinstance(finding[field], str) or not finding[field].strip():
                issues.append(f"{prefix}-{field}-invalid")
        if finding["disposition"] != "reported":
            issues.append(f"{prefix}-disposition-invalid")
    if ids != sorted(ids) or len(ids) != len(set(ids)):
        issues.append("review-finding-order-invalid")
    return sorted(set(issues))


def load_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise EvalError("review JSON could not be read") from error


def finalize(args: argparse.Namespace) -> dict[str, Any]:
    bundle_root = args.bundle_root.resolve()
    bundle = load_json(bundle_root / "bundle.json")
    review_input = load_json(bundle_root / "review-input.json")
    if not isinstance(bundle, dict) or not isinstance(review_input, dict):
        raise EvalError("evaluation bundle is invalid")
    expected = review_input.get("input_digest")
    if not isinstance(expected, str) or not DIGEST_RE.fullmatch(expected):
        raise EvalError("review input digest is invalid")
    reviews = {
        "functional-review": load_json(args.functional_review),
        "recovery-review": load_json(args.recovery_review),
    }
    issues = {
        role: validate_review(value, role, expected)
        for role, value in reviews.items()
    }
    if any(issues.values()):
        return {
            "version": 1, "code": "eval-review-invalid", "status": "blocked",
            "tree_digest": bundle.get("candidate_tree_digest", ""), "issues": issues,
        }
    review_receipts = []
    finding_count = 0
    for role, value in sorted(reviews.items()):
        finding_count += len(value["findings"])
        review_receipts.append({
            "role": role,
            "agent_id": value["agent_id"],
            "model": value["model"],
            "reasoning_effort": value["reasoning_effort"],
            "fork_turns": value["fork_turns"],
            "input_digest": value["input_digest"],
            "output_digest": digest_json(value),
            "finding_count": len(value["findings"]),
            "status": "complete",
        })
    receipt = {
        "version": 1,
        "code": "eval-reviewed",
        "status": "approved" if finding_count == 0 else "findings",
        "round": bundle["round"],
        "tree_digest": bundle["candidate_tree_digest"],
        "criteria_digest": bundle["criteria_digest"],
        "test_receipt_digest": bundle["test_receipt_digest"],
        "finding_count": finding_count,
        "reviews": review_receipts,
    }
    receipt["receipt_digest"] = digest_json(receipt)
    atomic_json(bundle_root / "eval-receipt.json", receipt)
    return receipt


def parser() -> StableParser:
    root = StableParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    prepare_command = commands.add_parser("prepare")
    prepare_command.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[2])
    prepare_command.add_argument("--eval-root", type=Path, default=Path("/private/tmp/documentation-vault-sync-eval"))
    finalize_command = commands.add_parser("finalize")
    finalize_command.add_argument("--bundle-root", type=Path, required=True)
    finalize_command.add_argument("--functional-review", type=Path, required=True)
    finalize_command.add_argument("--recovery-review", type=Path, required=True)
    return root


def main() -> int:
    try:
        args = parser().parse_args()
        result = prepare(args) if args.command == "prepare" else finalize(args)
        sys.stdout.write(canonical_bytes(result).decode("ascii") + "\n")
        if args.command == "finalize" and result.get("code") == "eval-reviewed":
            return 0
        return 0 if result.get("status") == "pass" else 2
    except EvalError as error:
        sys.stdout.write(canonical_bytes({
            "version": 1, "code": "eval-contract-invalid", "status": "blocked",
            "message": str(error),
        }).decode("ascii") + "\n")
        return 2
    except Exception:
        sys.stdout.write(canonical_bytes({
            "version": 1, "code": "eval-internal-error", "status": "error",
            "message": "unexpected evaluation failure",
        }).decode("ascii") + "\n")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
