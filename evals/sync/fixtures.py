"""Generic fixtures for public resumable-sync CLI contracts."""
from __future__ import annotations

import hashlib
import json
import subprocess
from pathlib import Path
from typing import Any


SOURCE_REPOSITORY = "APP00000-source-adapter"
TARGET_REPOSITORY = "APP00000-target-service"
SOURCE_CLAIM = "claim-source-001"
TARGET_CLAIM = "claim-target-001"
NODE = "target-service"


def write_json(path: Path, value: dict[str, Any]) -> None:
    path.write_text(
        json.dumps(value, ensure_ascii=True, sort_keys=True, separators=(",", ":")),
        encoding="utf-8",
    )


def digest(value: Any) -> str:
    return hashlib.sha256(
        json.dumps(value, ensure_ascii=True, sort_keys=True, separators=(",", ":")).encode("utf-8")
    ).hexdigest()


def make_repository_pair(root: Path) -> tuple[Path, Path, dict[str, str]]:
    """Create two independent, generically named repositories with exact OIDs."""
    repositories: list[Path] = []
    oids: dict[str, str] = {}
    for name in (SOURCE_REPOSITORY, TARGET_REPOSITORY):
        repo = root / name
        subprocess.run(["git", "init", "-q", str(repo)], check=True)
        subprocess.run(
            ["git", "-C", str(repo), "remote", "add", "origin", f"https://example.invalid/{name}.git"],
            check=True,
        )
        subprocess.run(["git", "-C", str(repo), "config", "user.email", "eval@example.invalid"], check=True)
        subprocess.run(["git", "-C", str(repo), "config", "user.name", "Sync Eval"], check=True)
        (repo / "component.txt").write_text(f"{name} baseline\n", encoding="utf-8")
        subprocess.run(["git", "-C", str(repo), "add", "."], check=True)
        subprocess.run(["git", "-C", str(repo), "commit", "-qm", "baseline"], check=True)
        (repo / "component.txt").write_text(f"{name} changed\n", encoding="utf-8")
        subprocess.run(["git", "-C", str(repo), "commit", "-am", "change", "-q"], check=True)
        oids[name] = subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "HEAD"],
            check=True,
            text=True,
            capture_output=True,
        ).stdout.strip()
        repositories.append(repo)
    return repositories[0], repositories[1], oids


def package_artifacts(
    root: Path,
    repo: Path,
    repository: str,
    *,
    claim_id: str,
    requested_nodes: tuple[str, ...] = (),
    rejected_review: bool = False,
) -> tuple[Path, Path, Path, Path]:
    """Create one valid generic package for the current public gate command."""
    dist = Path(__file__).resolve().parents[2]
    manifest_cli = dist / "kernel" / "90-Meta" / "git-change-manifest.py"

    def run(*arguments: str) -> dict[str, Any]:
        result = subprocess.run(
            ["python3", "-B", str(manifest_cli), *arguments],
            cwd=str(dist), text=True, capture_output=True, check=False,
        )
        if result.returncode:
            raise RuntimeError(result.stdout + result.stderr)
        return json.loads(result.stdout)

    package = root / f"package-{repository}"
    package.mkdir()
    manifest = run(
        "build", "--repo", str(repo), "--old",
        subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "HEAD~1"],
            text=True, capture_output=True, check=True,
        ).stdout.strip(),
        "--new", subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "HEAD"],
            text=True, capture_output=True, check=True,
        ).stdout.strip(),
    )
    manifest_path = package / "manifest.json"
    write_json(manifest_path, manifest)
    arguments = ["init-analysis", "--manifest", str(manifest_path), "--repository", repository]
    for node in requested_nodes:
        arguments.extend(("--node", node))
    scaffold = run(*arguments)
    analysis = json.loads(json.dumps(scaffold))
    changed_path = manifest["paths"][0]["path"]
    for decision in analysis["paths"]:
        decision.update({
            "disposition": "relevant",
            "reason": "generic fixture evidence",
            "claim_ids": [claim_id],
        })
    for checklist in analysis["checklist"].values():
        checklist["status"] = "checked"
        checklist["reason"] = "generic fixture review"
        checklist["evidence"] = [{"path": changed_path, "anchor": "exact commit evidence"}]
        for question in checklist["questions"]:
            checklist["questions"][question] = "not-observed"
    analysis["claims"] = [{
        "claim_id": claim_id,
        "statement": "Generic fixture claim.",
        "evidence": [{"path": changed_path, "anchor": "exact commit evidence"}],
    }]
    for node in analysis["nodes"]:
        node.update({
            "action": "update" if node["basename"] == NODE else "no-change",
            "reason": "generic fixture node decision",
            "claim_ids": [claim_id] if node["basename"] == NODE else [],
        })
    analysis["result"] = "documentation-change"
    analysis["blockers"] = []
    review: dict[str, Any] = {
        "version": 3,
        "repository": repository,
        "manifest_digest": digest(manifest),
        "scaffold_digest": digest(scaffold),
        "analysis_digest": digest(analysis),
        "verdict": "accept",
        "findings": [],
    }
    if rejected_review:
        review["verdict"] = "revise"
        review["findings"] = [{
            "target": f"claims.{claim_id}",
            "category": "generic-fixture",
            "reason": "Reject the target repository's own claim.",
            "nodes": [NODE],
            "evidence": {"path": changed_path, "anchor": "exact commit evidence"},
        }]
    paths = (manifest_path, package / "scaffold.json", package / "analysis.json", package / "review.json")
    for path, value in zip(paths[1:], (scaffold, analysis, review)):
        path.write_text(
            json.dumps(value, ensure_ascii=True, separators=(",", ":")),
            encoding="utf-8",
        )
    return paths


def gate_v2(oids: dict[str, str]) -> dict[str, Any]:
    """A sealed gate where an accepted source claim writes a rejected target node."""
    source = {
        "repository": SOURCE_REPOSITORY,
        "verdict": "accept",
        "review_disposition": "accept",
        "result": "documentation-change",
        "new_oid": oids[SOURCE_REPOSITORY],
        "is_new": False,
        "declared_nodes": ["source-adapter", NODE],
        "affected_nodes": [],
        "write_nodes": [NODE],
        "accepted_claim_ids": [SOURCE_CLAIM],
        "rejected_claim_ids": [],
        "partial_accept": False,
        "analysis_fallback": False,
        "fallback_reason": "",
        "cursor_decision": "",
        "disposition": "write-ready",
    }
    target = {
        "repository": TARGET_REPOSITORY,
        "verdict": "revise",
        "review_disposition": "revise",
        "result": "documentation-change",
        "new_oid": oids[TARGET_REPOSITORY],
        "is_new": False,
        "declared_nodes": [NODE],
        "affected_nodes": [NODE],
        "write_nodes": [],
        "accepted_claim_ids": [],
        "rejected_claim_ids": [TARGET_CLAIM],
        "partial_accept": False,
        "analysis_fallback": False,
        "fallback_reason": "review-revise",
        "cursor_decision": "review-rejected",
        "disposition": "cursor-ready",
    }
    return {
        "version": 2,
        "code": "batch-gated",
        "status": "all-ready",
        "repositories": [source, target],
        "write_groups": [{
            "group_id": "group-001",
            "repositories": [SOURCE_REPOSITORY],
            "nodes": [NODE],
            "grants": [{
                "repository": SOURCE_REPOSITORY,
                "claim_id": SOURCE_CLAIM,
                "nodes": [NODE],
            }],
        }],
        "acknowledgements": [{
            "repository": TARGET_REPOSITORY,
            "new_oid": oids[TARGET_REPOSITORY],
            "decision": "review-rejected",
            "branch": "main",
            "analysis_date": "2026-08-22",
        }],
        "fallback_repositories": [TARGET_REPOSITORY],
        "pending_repositories": [],
    }


def projection(
    gate: dict[str, Any],
    *,
    unit_id: str = "group-001",
    unit_type: str = "write-group",
    grants: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    before_files = {"10-Sistemas/target-service.md": hashlib.sha256(b"before\n").hexdigest()}
    result_files = {
        "10-Sistemas/target-service.md": hashlib.sha256(
            b"Adapter behavior is documented through explicit source lineage.\n"
        ).hexdigest()
    }
    patch = (
        "--- a/10-Sistemas/target-service.md\n"
        "+++ b/10-Sistemas/target-service.md\n"
        "@@ -1 +1 @@\n"
        "-before\n"
        "+Adapter behavior is documented through explicit source lineage.\n"
    )
    if unit_type == "acknowledgements":
        acknowledgement = json.dumps(
            {
                "version": 1,
                "repositories": [
                    {
                        "repository": item["repository"],
                        "branch": item["branch"],
                        "analyzed_sha": item["new_oid"][:12],
                        "decision": item["decision"],
                        "analysis_date": item["analysis_date"],
                    }
                    for item in gate["acknowledgements"]
                ],
            },
            ensure_ascii=True,
            sort_keys=True,
            separators=(",", ":"),
        ) + "\n"
        patch = (
            "--- /dev/null\n"
            "+++ b/90-Meta/.sync-acknowledgements.json\n"
            "@@ -0,0 +1 @@\n"
            f"+{acknowledgement}"
        )
        before_files = {
            "90-Meta/.sync-acknowledgements.json": hashlib.sha256(b"").hexdigest()
        }
        result_files = {
            "90-Meta/.sync-acknowledgements.json": hashlib.sha256(
                acknowledgement.encode("utf-8")
            ).hexdigest()
        }
    return {
        "version": 1,
        "run_id": "run-sync-eval-001",
        "gate_digest": digest(gate),
        "unit_id": unit_id,
        "unit_type": unit_type,
        "patch_digest": hashlib.sha256(patch.encode("utf-8")).hexdigest(),
        "base_files": before_files,
        "result_files": result_files,
        "grants": grants if grants is not None else gate["write_groups"][0]["grants"],
        "patch": patch,
    }
