#!/usr/bin/env python3
"""Public-CLI red contracts for resumable synchronization state and recovery."""
from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Any

from fixtures import (
    NODE,
    SOURCE_CLAIM,
    SOURCE_REPOSITORY,
    TARGET_CLAIM,
    TARGET_REPOSITORY,
    digest,
    gate_v2,
    make_repository_pair,
    package_artifacts,
    write_json,
)


DIST = Path(__file__).resolve().parents[2]
SYNC_RUN = DIST / "kernel" / "90-Meta" / "sync-run.py"
MANIFEST = DIST / "kernel" / "90-Meta" / "git-change-manifest.py"
NOTE_REVIEW = DIST / "kernel" / "90-Meta" / "review-note-candidate.py"
ACK_PATH = "90-Meta/.sync-acknowledgements.json"
SECOND_REPOSITORY = "APP00000-zobserver-service"
SECOND_NODE = "observer-service"

_manifest_spec = importlib.util.spec_from_file_location("sync_state_manifest", MANIFEST)
manifest_tool = importlib.util.module_from_spec(_manifest_spec)
_manifest_spec.loader.exec_module(manifest_tool)


def bytes_digest(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


class SyncRunStateEval(unittest.TestCase):
    def run_sync(self, *arguments: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-B", str(SYNC_RUN), *arguments],
            cwd=str(DIST), text=True, capture_output=True, check=False,
        )

    def run_manifest(self, *arguments: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-B", str(MANIFEST), *arguments],
            cwd=str(DIST), text=True, capture_output=True, check=False,
        )

    def close_fixture_package(
        self,
        root: Path,
        repo: Path,
        repository: str,
        *,
        claim_id: str,
        requested_nodes: tuple[str, ...] = (),
        rejected_review: bool = False,
    ) -> Path:
        artifacts = package_artifacts(
            root, repo, repository, claim_id=claim_id,
            requested_nodes=requested_nodes, rejected_review=rejected_review,
        )
        closed_path = root / f"closed-{repository}.json"
        result = self.run_manifest(
            "close-package", "--repo", str(repo), "--manifest", str(artifacts[0]),
            "--scaffold", str(artifacts[1]), "--analysis", str(artifacts[2]),
            "--review", str(artifacts[3]),
            "--production-ref", "refs/heads/main",
            "--analysis-date", "2026-08-22", "--output", str(closed_path),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return closed_path

    def payload(self, result: subprocess.CompletedProcess[str]) -> dict[str, Any]:
        self.assertTrue(result.stdout, result.stderr)
        return json.loads(result.stdout)

    def begin(
        self,
        root: Path,
        oids: dict[str, str],
        *,
        packages: tuple[tuple[str, str], ...] | None = None,
    ) -> str:
        package_set = packages or (
            (SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),
            (TARGET_REPOSITORY, oids[TARGET_REPOSITORY]),
        )
        arguments = [
            "begin", "--state-root", str(root / "state"),
            "--tool-digest", "a" * 64, "--inventory-digest", "b" * 64,
        ]
        for repository, oid in package_set:
            arguments.extend(("--package", repository, oid))
        result = self.run_sync(*arguments)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        run_id = self.payload(result)["run_id"]
        if not hasattr(self, "run_packages"):
            self.run_packages: dict[str, dict[str, str]] = {}
        self.run_packages[run_id] = dict(package_set)
        return run_id

    def closed_receipt(self, repository: str, oid: str) -> dict[str, Any]:
        return {
            "version": 1,
            "code": "analysis-closed",
            "status": "complete",
            "repository": repository,
            "new_oid": oid,
            "artifact_digest": bytes_digest(f"{repository}:{oid}"),
            "finalize_digest": bytes_digest(f"finalized:{repository}:{oid}"),
        }

    def semantic_package(
        self,
        repository: str,
        oid: str,
        gate: dict[str, Any],
    ) -> dict[str, Any]:
        record = copy.deepcopy(next(
            item for item in gate["repositories"]
            if item["repository"] == repository
        ))
        grants = [
            copy.deepcopy(grant)
            for group in gate["write_groups"]
            for grant in group["grants"]
            if grant["repository"] == repository
        ]
        acknowledgements = [
            copy.deepcopy(item) for item in gate["acknowledgements"]
            if item["repository"] == repository
        ]
        package_gate = {
            "version": 2,
            "code": "batch-gated",
            "status": "all-ready" if grants or acknowledgements else "complete-no-write",
            "repositories": [record],
            "write_groups": ([{
                "group_id": "group-001",
                "repositories": [repository],
                "nodes": record["write_nodes"],
                "grants": grants,
            }] if record["disposition"] == "write-ready" else []),
            "acknowledgements": acknowledgements,
            "fallback_repositories": (
                [repository] if record["fallback_reason"] else []
            ),
            "pending_repositories": [],
        }
        manifest = {
            "version": 1, "old_oid": "a" * 40, "new_oid": oid,
            "paths": [], "environment_configs": [], "credential_suspects": [],
        }
        scaffold = {
            "version": 2, "repository": repository,
            "old_oid": "a" * 40, "new_oid": oid, "paths": [],
            "checklist": {}, "claims": [], "nodes": [],
            "result": "no-change", "blockers": [],
        }
        analysis = {
            "version": 2,
            "repository": repository,
            "old_oid": "a" * 40,
            "new_oid": oid,
            "paths": [], "checklist": {},
            "claims": [
                {
                    "claim_id": claim_id,
                    "statement": "generic finalized package claim",
                    "evidence": [{"path": "component.txt", "anchor": "exact fixture evidence"}],
                }
                for claim_id in record["accepted_claim_ids"]
            ],
            "nodes": [
                {
                    "basename": node, "action": "update",
                    "reason": "generic finalized node",
                    "claim_ids": record["accepted_claim_ids"],
                }
                for node in record["write_nodes"]
            ],
            "result": record["result"], "blockers": [],
        }
        review = {
            "version": 3, "repository": repository,
            "manifest_digest": digest(manifest),
            "scaffold_digest": digest(scaffold),
            "analysis_digest": digest(analysis),
            "verdict": record["verdict"],
            "findings": ([] if record["verdict"] == "accept" else [{
                "target": "analysis", "category": "generic-fixture",
                "reason": "generic closed review limitation",
                "nodes": record["declared_nodes"][:1],
                "evidence": {"path": "component.txt", "anchor": "fixture"},
            }]),
        }
        validation = {
            "manifest_digest": digest(manifest),
            "scaffold_digest": digest(scaffold),
            "analysis_digest": digest(analysis),
            "review_digest": digest(review),
            "gate_digest": digest(package_gate),
        }
        return {
            "version": 1,
            "code": "package-closed",
            "status": "complete",
            "repository": repository,
            "new_oid": oid,
            "manifest": manifest,
            "scaffold": scaffold,
            "analysis": analysis,
            "review": review,
            "gate": package_gate,
            "validation": validation,
        }

    def checkpoint_packages(
        self, root: Path, run_id: str, gate: dict[str, Any]
    ) -> None:
        for repository, oid in sorted(self.run_packages[run_id].items()):
            artifact_path = root / f"closed-{repository}.json"
            write_json(artifact_path, self.semantic_package(repository, oid, gate))
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"),
                "--run-id", run_id, "--repository", repository,
                "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def seal(
        self,
        root: Path,
        run_id: str,
        gate: dict[str, Any],
        *,
        checkpoint: bool = True,
        require_coverage: bool = True,
    ) -> None:
        if require_coverage:
            self.assertEqual(
                {item["repository"] for item in gate["repositories"]},
                set(self.run_packages[run_id]),
            )
        if checkpoint:
            self.checkpoint_packages(root, run_id, gate)
        gate_path = root / "gate.json"
        write_json(gate_path, gate)
        result = self.run_sync(
            "seal-gate", "--state-root", str(root / "state"),
            "--run-id", run_id, "--gate", str(gate_path),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def acknowledgement_payload(self, gate: dict[str, Any]) -> str:
        return json.dumps(
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

    def status(self, root: Path, run_id: str) -> dict[str, Any]:
        result = self.run_sync(
            "status", "--state-root", str(root / "state"), "--run-id", run_id,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return self.payload(result)

    def unit_files(
        self,
        root: Path,
        gate: dict[str, Any],
        *,
        run_id: str,
        unit_id: str,
        unit_type: str,
        path: str,
        before: str,
        after: str,
        grants: list[dict[str, Any]],
    ) -> tuple[Path, Path, Path]:
        vault = root / "vault"
        target = vault / path
        target.parent.mkdir(parents=True, exist_ok=True)
        if before:
            target.write_text(before, encoding="utf-8")
        patch = (
            f"--- a/{path}\n+++ b/{path}\n@@ -1 +1 @@\n-{before}+{after}"
            if before else f"--- /dev/null\n+++ b/{path}\n@@ -0,0 +1 @@\n+{after}"
        )
        projection = {
            "version": 1,
            "run_id": run_id,
            "gate_digest": digest(gate),
            "unit_id": unit_id,
            "unit_type": unit_type,
            "patch_digest": bytes_digest(patch),
            "base_files": {path: bytes_digest(before)},
            "result_files": {path: bytes_digest(after)},
            "grants": grants,
        }
        projection_path, patch_path = root / f"{unit_id}.json", root / f"{unit_id}.patch"
        write_json(projection_path, projection)
        patch_path.write_text(patch, encoding="utf-8")
        return vault, projection_path, patch_path

    def validate_unit(
        self,
        root: Path,
        run_id: str,
        unit_id: str,
        projection_path: Path,
        patch_path: Path,
        *,
        review: bool = True,
    ) -> subprocess.CompletedProcess[str]:
        result = self.run_sync(
            "validate-unit", "--state-root", str(root / "state"), "--run-id", run_id,
            "--unit-id", unit_id, "--projection", str(projection_path),
            "--patch", str(patch_path),
        )
        if result.returncode == 0 and review:
            projection = json.loads(projection_path.read_text(encoding="utf-8"))
            if projection["unit_type"] == "write-group":
                reviewed = self.review_unit(root, run_id, unit_id, projection_path, patch_path)
                self.assertEqual(reviewed.returncode, 0, reviewed.stdout + reviewed.stderr)
        return result

    def review_unit(
        self,
        root: Path,
        run_id: str,
        unit_id: str,
        projection_path: Path,
        patch_path: Path,
        *,
        invert_result_kind: bool = False,
    ) -> subprocess.CompletedProcess[str]:
        candidate = root / f"candidate-{unit_id}"
        evidence = root / f"evidence-{unit_id}"
        shutil.rmtree(candidate, ignore_errors=True)
        shutil.rmtree(evidence, ignore_errors=True)
        candidate.mkdir()
        evidence.mkdir()
        (root / "vault").mkdir(parents=True, exist_ok=True)
        patch_text = patch_path.read_text(encoding="utf-8")
        sections, issues = manifest_tool.projected_patch_sections(patch_text)
        self.assertEqual(issues, [])
        deleted = []
        for relative, section in sections.items():
            images = manifest_tool.complete_patch_images(section)
            self.assertIsNotNone(images)
            _, after = images
            if invert_result_kind:
                self.assertEqual(after, b"")
            if (section["new_path"] is None) != invert_result_kind:
                deleted.append(relative)
            else:
                target = candidate / relative
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(after)
        (evidence / "record.json").write_text(
            '{"kind":"synthetic-final-note-evidence"}\n', encoding="utf-8"
        )
        manifest_path = root / f"note-manifest-{unit_id}.json"
        freeze_arguments = [
                sys.executable, "-B", str(NOTE_REVIEW), "freeze",
                "--vault", str(root / "vault"),
                "--candidate", str(candidate),
                "--evidence-root", str(evidence),
                "--evidence", "record.json",
                "--projection", str(projection_path),
                "--output", str(manifest_path),
        ]
        for relative in deleted:
            freeze_arguments.extend(("--delete", relative))
        frozen = subprocess.run(
            freeze_arguments,
            cwd=str(DIST), text=True, capture_output=True, check=False,
        )
        self.assertEqual(frozen.returncode, 0, frozen.stdout + frozen.stderr)
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        decisions = {}
        for relative, images in manifest["connections"].items():
            before, after = set(images["base"]), set(images["candidate"])
            for connection_id in before | after:
                action = (
                    "preserve" if connection_id in before and connection_id in after
                    else "create" if connection_id in after else "retire"
                )
                decisions[f"{relative}#{connection_id}"] = {
                    "action": action,
                    "reason": "Synthetic fixture transition was reviewed.",
                }
        review_path = root / f"note-review-{unit_id}.json"
        write_json(review_path, {
            "version": 1,
            "manifest_digest": digest(manifest),
            "verdict": "accept",
            "findings": [],
            "connection_decisions": decisions,
        })
        return self.run_sync(
            "review-unit", "--state-root", str(root / "state"), "--run-id", run_id,
            "--unit-id", unit_id, "--vault", str(root / "vault"),
            "--candidate", str(candidate), "--evidence-root", str(evidence),
            "--manifest", str(manifest_path), "--review", str(review_path),
        )

    def test_documentation_apply_requires_bound_final_note_review(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001",
                unit_type="write-group", path="10-Sistemas/target-service.md",
                before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            validated = self.validate_unit(
                root, run_id, "group-001", projection_path, patch_path, review=False,
            )
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            _, ack_projection, ack_patch = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements",
                unit_type="acknowledgements", path=ACK_PATH, before="",
                after=self.acknowledgement_payload(gate), grants=[],
            )
            ack_validated = self.validate_unit(
                root, run_id, "acknowledgements", ack_projection, ack_patch,
            )
            self.assertEqual(
                ack_validated.returncode, 0, ack_validated.stdout + ack_validated.stderr,
            )
            self.apply_unit(root, run_id, "acknowledgements", vault)
            status = self.status(root, run_id)
            self.assertEqual(status["next_command"], "review-unit")
            blocked = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"),
                "--run-id", run_id, "--unit-id", "group-001", "--vault", str(vault),
            )
            self.assertEqual(blocked.returncode, 2, blocked.stdout + blocked.stderr)
            self.assertEqual(self.payload(blocked)["code"], "final-note-review-required")
            target = vault / "10-Sistemas/target-service.md"
            target.write_text("after\n", encoding="utf-8")
            resumed = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
            )
            self.assertEqual(resumed.returncode, 2, resumed.stdout + resumed.stderr)
            self.assertEqual(self.payload(resumed)["code"], "final-note-review-required")
            target.write_text("before\n", encoding="utf-8")
            reviewed = self.review_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(reviewed.returncode, 0, reviewed.stdout + reviewed.stderr)
            self.assertTrue(self.payload(reviewed)["receipt_digest"])
            self.apply_unit(root, run_id, "group-001", vault)

    def apply_unit(self, root: Path, run_id: str, unit_id: str, vault: Path) -> dict[str, Any]:
        result = self.run_sync(
            "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
            "--unit-id", unit_id, "--vault", str(vault),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return self.payload(result)

    def two_group_gate(self, oids: dict[str, str]) -> dict[str, Any]:
        gate = copy.deepcopy(gate_v2(oids))
        gate["repositories"] = [
            item for item in gate["repositories"]
            if item["repository"] == SOURCE_REPOSITORY
        ]
        gate["acknowledgements"] = []
        second_oid = "c" * 40
        gate["repositories"].append({
            "repository": SECOND_REPOSITORY,
            "verdict": "accept",
            "review_disposition": "accept",
            "result": "documentation-change",
            "new_oid": second_oid,
            "is_new": False,
            "declared_nodes": [SECOND_NODE],
            "affected_nodes": [],
            "write_nodes": [SECOND_NODE],
            "accepted_claim_ids": ["claim-observer-001"],
            "rejected_claim_ids": [],
            "partial_accept": False,
            "analysis_fallback": False,
            "fallback_reason": "",
            "cursor_decision": "",
            "disposition": "write-ready",
        })
        gate["repositories"].sort(key=lambda item: item["repository"])
        gate["write_groups"].append({
            "group_id": "group-002",
            "repositories": [SECOND_REPOSITORY],
            "nodes": [SECOND_NODE],
            "grants": [{
                "repository": SECOND_REPOSITORY,
                "claim_id": "claim-observer-001",
                "nodes": [SECOND_NODE],
            }],
        })
        gate["fallback_repositories"] = []
        return gate

    def acknowledgement_only_gate(self, oids: dict[str, str]) -> dict[str, Any]:
        gate = copy.deepcopy(gate_v2(oids))
        gate["repositories"] = [
            item for item in gate["repositories"]
            if item["repository"] == TARGET_REPOSITORY
        ]
        gate["write_groups"] = []
        return gate

    def test_acknowledgement_applies_when_group_projection_is_invalid(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, ack_projection, ack_patch = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(gate), grants=[],
            )
            invalid_projection = json.loads(ack_projection.read_text(encoding="utf-8"))
            invalid_projection["unit_id"] = "group-001"
            invalid_projection["unit_type"] = "write-group"
            invalid_projection["grants"] = [{
                "repository": TARGET_REPOSITORY,
                "claim_id": "claim-target-001",
                "nodes": [NODE],
            }]
            group_projection = root / "invalid-group.json"
            write_json(group_projection, invalid_projection)
            ack_validated = self.validate_unit(root, run_id, "acknowledgements", ack_projection, ack_patch)
            self.assertEqual(ack_validated.returncode, 0, ack_validated.stdout + ack_validated.stderr)
            self.apply_unit(root, run_id, "acknowledgements", vault)
            group_validated = self.validate_unit(root, run_id, "group-001", group_projection, ack_patch)
            self.assertEqual(group_validated.returncode, 2, group_validated.stdout + group_validated.stderr)
            self.assertEqual(self.payload(group_validated)["code"], "projection-invalid")
            units = {unit["unit_id"]: unit for unit in self.status(root, run_id)["units"]}
            self.assertEqual(units["acknowledgements"]["status"], "applied")
            self.assertEqual(units["group-001"]["status"], "projection-invalid")

    def test_disjoint_groups_apply_independently_in_stable_order(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            run_id = self.begin(root, oids, packages=(
                (SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),
                (SECOND_REPOSITORY, "c" * 40),
            ))
            self.seal(root, run_id, gate)
            before = self.status(root, run_id)
            self.assertEqual([unit["unit_id"] for unit in before["units"]], ["group-001", "group-002"])
            first = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="first\n",
                grants=gate["write_groups"][0]["grants"],
            )
            second = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-002", unit_type="write-group",
                path="10-Sistemas/observer-service.md", before="before\n", after="second\n",
                grants=gate["write_groups"][1]["grants"],
            )
            for unit_id, (vault, projection_path, patch_path) in (("group-001", first), ("group-002", second)):
                validated = self.validate_unit(root, run_id, unit_id, projection_path, patch_path)
                self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
                self.apply_unit(root, run_id, unit_id, vault)
            after = self.status(root, run_id)
            self.assertEqual([unit["unit_id"] for unit in after["units"]], ["group-001", "group-002"])
            self.assertEqual({unit["status"] for unit in after["units"]}, {"applied"})

    def test_resume_reuses_sealed_digests_after_projection_invalid(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=[{"repository": TARGET_REPOSITORY, "claim_id": "claim-target-001", "nodes": [NODE]}],
            )
            rejected = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(rejected.returncode, 2, rejected.stdout + rejected.stderr)
            before = self.status(root, run_id)
            resumed = self.run_sync("resume", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            after = self.payload(resumed)
            self.assertEqual(after["gate_digest"], before["gate_digest"])
            self.assertEqual(after["package_counters"], before["package_counters"])
            self.assertEqual(after["next_command"], "validate-unit")

    def test_rejects_repeated_failed_patch_digest_but_accepts_corrected_digest(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, invalid_projection, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=[{"repository": TARGET_REPOSITORY, "claim_id": "claim-target-001", "nodes": [NODE]}],
            )
            first = self.validate_unit(root, run_id, "group-001", invalid_projection, patch_path)
            self.assertEqual(first.returncode, 2, first.stdout + first.stderr)
            repeated = self.validate_unit(root, run_id, "group-001", invalid_projection, patch_path)
            self.assertEqual(repeated.returncode, 2, repeated.stdout + repeated.stderr)
            self.assertEqual(self.payload(repeated)["code"], "failed-patch-digest-reused")
            corrected = copy.deepcopy(json.loads(invalid_projection.read_text(encoding="utf-8")))
            corrected["grants"] = gate["write_groups"][0]["grants"]
            corrected["patch_digest"] = bytes_digest(patch_path.read_text(encoding="utf-8") + "corrected")
            corrected_patch = root / "corrected.patch"
            corrected_patch.write_text(patch_path.read_text(encoding="utf-8") + "\n", encoding="utf-8")
            corrected["patch_digest"] = bytes_digest(corrected_patch.read_text(encoding="utf-8"))
            corrected_projection = root / "corrected.json"
            write_json(corrected_projection, corrected)
            validated = self.validate_unit(root, run_id, "group-001", corrected_projection, corrected_patch)
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            self.apply_unit(root, run_id, "group-001", vault)

    def test_source_oid_change_invalidates_only_connected_unit(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            run_id = self.begin(root, oids, packages=(
                (SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),
                (SECOND_REPOSITORY, "c" * 40),
            ))
            self.seal(root, run_id, gate)
            resumed = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            payload = self.payload(resumed)
            self.assertEqual(payload["code"], "source-stale")
            self.assertEqual(payload["invalidated_packages"], [SOURCE_REPOSITORY])
            self.assertEqual(payload["invalidated_units"], ["group-001"])
            self.assertEqual(payload["reused_units"], ["group-002"])

    def test_destination_baseline_change_invalidates_only_its_unit(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            run_id = self.begin(root, oids, packages=(
                (SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),
                (SECOND_REPOSITORY, "c" * 40),
            ))
            self.seal(root, run_id, gate)
            resumed = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--destination-digest", "10-Sistemas/target-service.md", "d" * 64,
            )
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            payload = self.payload(resumed)
            self.assertEqual(payload["code"], "vault-baseline-stale")
            self.assertEqual(payload["invalidated_packages"], [])
            self.assertEqual(payload["invalidated_units"], ["group-001"])
            self.assertEqual(payload["reused_units"], ["group-002"])

    def test_resume_reconciles_after_bytes_without_receipt(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            validated = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            crashed = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault), "--crash-after-bytes",
            )
            self.assertEqual(crashed.returncode, 1, crashed.stdout + crashed.stderr)
            target = vault / "10-Sistemas/target-service.md"
            self.assertEqual(target.read_text(encoding="utf-8"), "after\n")
            resumed = self.run_sync("resume", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            self.assertEqual(self.payload(resumed)["reconciled_units"], ["group-001"])
            self.assertEqual(target.read_text(encoding="utf-8"), "after\n")

    def test_checkpoints_and_receipts_exclude_sensitive_worker_material(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),))
            root.joinpath("raw-worker.json").write_text(json.dumps({
                "claim": "secret claim statement",
                "finding": "secret finding text",
                "credential": "sync-eval-secret-value",
                "path": "/private/tmp/absolute-source-path",
            }), encoding="utf-8")
            checkpoint = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(root / "raw-worker.json"),
            )
            self.assertEqual(checkpoint.returncode, 2, checkpoint.stdout + checkpoint.stderr)
            self.assertEqual(self.payload(checkpoint)["code"], "checkpoint-sensitive-content")
            root = root / "acknowledgements"
            root.mkdir()
            run_id = self.begin(
                root,
                oids,
                packages=((TARGET_REPOSITORY, oids[TARGET_REPOSITORY]),),
            )
            gate = self.acknowledgement_only_gate(oids)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(gate), grants=[],
            )
            validated = self.validate_unit(
                root, run_id, "acknowledgements", projection_path, patch_path,
            )
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            self.apply_unit(root, run_id, "acknowledgements", vault)
            closed = self.run_sync("close", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(closed.returncode, 0, closed.stdout + closed.stderr)
            state_text = "\n".join(
                path.read_text(encoding="utf-8")
                for path in (root / "state").glob("**/*.json")
            )
            for forbidden in ("secret claim statement", "secret finding text", "sync-eval-secret-value", "/private/tmp/absolute-source-path"):
                with self.subTest(forbidden=forbidden):
                    self.assertNotIn(forbidden, state_text)

    def test_identical_inputs_keep_run_gate_unit_order_and_receipts_stable(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            packages = (
                (SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),
                (SECOND_REPOSITORY, "c" * 40),
            )
            first_run = self.begin(root, oids, packages=packages)
            self.seal(root, first_run, gate)
            first_status = self.status(root, first_run)
            second_run = self.begin(root, oids, packages=packages)
            second_status = self.status(root, second_run)
            self.assertEqual(first_run, second_run)
            self.assertEqual(first_status["gate_digest"], second_status["gate_digest"])
            self.assertEqual(first_status["units"], second_status["units"])
            self.assertEqual(first_status["receipt_digest"], second_status["receipt_digest"])

    def test_traceability_only_cannot_create_a_markdown_write_group(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            source = next(item for item in gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)
            source["result"] = "traceability-only"
            source["accepted_claim_ids"] = []
            source["write_nodes"] = ["source-adapter"]
            gate["repositories"] = [source]
            gate["write_groups"][0]["nodes"] = ["source-adapter"]
            gate["write_groups"][0]["grants"] = []
            gate["acknowledgements"] = []
            gate["fallback_repositories"] = []
            run_id = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),))
            package_gate = copy.deepcopy(gate)
            package_source = package_gate["repositories"][0]
            package_source["write_nodes"] = []
            package_source["cursor_decision"] = "no-durable-node"
            package_source["disposition"] = "cursor-ready"
            package_gate["write_groups"] = []
            package_gate["acknowledgements"] = [{
                "repository": SOURCE_REPOSITORY,
                "new_oid": oids[SOURCE_REPOSITORY],
                "decision": "no-durable-node",
                "branch": "main",
                "analysis_date": "2026-08-22",
            }]
            self.checkpoint_packages(root, run_id, package_gate)
            gate_path = root / "traceability-write-gate.json"
            write_json(gate_path, gate)
            sealed = self.run_sync(
                "seal-gate", "--state-root", str(root / "state"), "--run-id", run_id,
                "--gate", str(gate_path),
            )
            self.assertEqual(sealed.returncode, 2, sealed.stdout + sealed.stderr)

    def test_seal_requires_a_finalized_checkpoint_and_complete_gate_inventory(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(root, oids)
            gate = {
                "version": 2,
                "code": "batch-gated",
                "status": "complete-no-write",
                "repositories": [],
                "write_groups": [],
                "acknowledgements": [],
                "fallback_repositories": [],
                "pending_repositories": [],
            }
            gate_path = root / "incomplete-gate.json"
            write_json(gate_path, gate)
            sealed = self.run_sync(
                "seal-gate", "--state-root", str(root / "state"),
                "--run-id", run_id, "--gate", str(gate_path),
            )
            self.assertEqual(sealed.returncode, 2, sealed.stdout + sealed.stderr)
            self.assertEqual(self.payload(sealed)["code"], "package-checkpoint-required")

    def test_begin_reuses_an_integrity_checked_closed_receipt(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            group = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            acknowledgement = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(gate), grants=[],
            )
            for unit_id, (vault, projection_path, patch_path) in (
                ("acknowledgements", acknowledgement),
                ("group-001", group),
            ):
                validated = self.validate_unit(root, run_id, unit_id, projection_path, patch_path)
                self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
                self.apply_unit(root, run_id, unit_id, vault)
            closed = self.run_sync(
                "close", "--state-root", str(root / "state"), "--run-id", run_id,
            )
            self.assertEqual(closed.returncode, 0, closed.stdout + closed.stderr)
            reused = self.run_sync(
                "begin", "--state-root", str(root / "state"),
                "--tool-digest", "a" * 64, "--inventory-digest", "b" * 64,
                "--package", SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY],
                "--package", TARGET_REPOSITORY, oids[TARGET_REPOSITORY],
            )
            self.assertEqual(reused.returncode, 0, reused.stdout + reused.stderr)
            payload = self.payload(reused)
            self.assertTrue(payload.get("reused"), payload)
            self.assertEqual(payload["status"], "complete", payload)

    def test_validate_unit_requires_the_exact_supervised_run_id(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            projection = json.loads(projection_path.read_text(encoding="utf-8"))
            projection["run_id"] = "run-provided-by-worker"
            write_json(projection_path, projection)
            result = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "projection-invalid")

    def test_applied_unit_is_invalidated_when_authority_or_postimage_drifts(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            validated = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            self.apply_unit(root, run_id, "group-001", vault)
            for name, arguments in (
                ("source", ("--source-oid", SOURCE_REPOSITORY, "d" * 40)),
                ("destination", ("--destination-digest", "10-Sistemas/target-service.md", "d" * 64)),
            ):
                with self.subTest(name=name):
                    resumed = self.run_sync(
                        "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                        *arguments,
                    )
                    self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
                    payload = self.payload(resumed)
                    self.assertIn("group-001", payload["invalidated_units"], payload)
                    self.assertNotEqual(
                        {unit["unit_id"]: unit for unit in payload["units"]}["group-001"]["status"],
                        "applied",
                        payload,
                    )

    def test_acknowledgement_projection_binds_its_result_to_the_gate(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.acknowledgement_only_gate(oids)
            run_id = self.begin(root, oids, packages=((TARGET_REPOSITORY, oids[TARGET_REPOSITORY]),))
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after="{\"version\":1}\n", grants=[],
            )
            rejected = self.validate_unit(
                root, run_id, "acknowledgements", projection_path, patch_path,
            )
            self.assertEqual(rejected.returncode, 2, rejected.stdout + rejected.stderr)
            self.assertEqual(self.payload(rejected)["code"], "projection-invalid")

    def test_write_error_between_files_leaves_no_partial_publication_and_is_recoverable(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault = root / "vault"
            first = vault / "10-Sistemas" / "target-service.md"
            first.parent.mkdir(parents=True)
            first.write_text("before-one\n", encoding="utf-8")
            second_parent = vault / "20-Repos"
            second_parent.mkdir()
            first_path = "10-Sistemas/target-service.md"
            second_path = "20-Repos/target-service.md"
            patch = (
                f"--- a/{first_path}\n+++ b/{first_path}\n@@ -1 +1 @@\n"
                "-before-one\n+after-one\n"
                f"--- /dev/null\n+++ b/{second_path}\n@@ -0,0 +1 @@\n+after-two\n"
            )
            projection = {
                "version": 1,
                "run_id": run_id,
                "gate_digest": digest(gate),
                "unit_id": "group-001",
                "unit_type": "write-group",
                "patch_digest": bytes_digest(patch),
                "base_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("")},
                "result_files": {first_path: bytes_digest("after-one\n"), second_path: bytes_digest("after-two\n")},
                "grants": gate["write_groups"][0]["grants"],
            }
            projection_path, patch_path = root / "two-path.json", root / "two-path.patch"
            write_json(projection_path, projection)
            patch_path.write_text(patch, encoding="utf-8")
            validated = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            failed = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault),
                "--fail-after-writes", "1",
            )
            self.assertEqual(failed.returncode, 1, failed.stdout + failed.stderr)
            self.assertEqual(first.read_text(encoding="utf-8"), "before-one\n")
            self.assertFalse((vault / second_path).exists())
            status = self.status(root, run_id)
            self.assertEqual(
                {unit["unit_id"]: unit for unit in status["units"]}["group-001"]["status"],
                "apply-failed",
            )

    def test_reseal_preserves_an_unaffected_validated_unit(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            run_id = self.begin(root, oids, packages=(
                (SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),
                (SECOND_REPOSITORY, "c" * 40),
            ))
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-002", unit_type="write-group",
                path="10-Sistemas/observer-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][1]["grants"],
            )
            validated = self.validate_unit(root, run_id, "group-002", projection_path, patch_path)
            self.assertEqual(validated.returncode, 0, validated.stdout + validated.stderr)
            stale = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(stale.returncode, 0, stale.stdout + stale.stderr)
            source_record = next(item for item in gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)
            source_record["new_oid"] = "d" * 40
            artifact_path = root / "recheckpoint.json"
            write_json(
                artifact_path,
                self.semantic_package(SOURCE_REPOSITORY, "d" * 40, gate),
            )
            checkpoint = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"),
                "--run-id", run_id, "--repository", SOURCE_REPOSITORY,
                "--artifact", str(artifact_path),
            )
            self.assertEqual(checkpoint.returncode, 0, checkpoint.stdout + checkpoint.stderr)
            resealed_path = root / "resealed.json"
            write_json(resealed_path, gate)
            resealed = self.run_sync(
                "seal-gate", "--state-root", str(root / "state"), "--run-id", run_id,
                "--gate", str(resealed_path),
            )
            self.assertEqual(resealed.returncode, 0, resealed.stdout + resealed.stderr)
            units = {unit["unit_id"]: unit for unit in self.payload(resealed)["units"]}
            self.assertEqual(units["group-002"]["status"], "validated")
            self.assertTrue(units["group-002"]["projection_digest"])

    def test_nested_state_symlink_is_rejected_before_checkpoint_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            outside = root / "outside"
            outside.mkdir()
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(root, oids)
            package_dir = root / "state" / "active" / run_id / "packages" / SOURCE_REPOSITORY
            package_dir.parent.mkdir(exist_ok=True)
            package_dir.symlink_to(outside, target_is_directory=True)
            artifact_path = root / "artifact.json"
            write_json(
                artifact_path,
                self.semantic_package(SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY], gate_v2(oids)),
            )
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"),
                "--run-id", run_id, "--repository", SOURCE_REPOSITORY,
                "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "state-path-invalid")
            self.assertFalse((outside / "artifact.json").exists())

    def test_oidc_permission_checkpoint_and_patch_preserve_sensitive_checks(self) -> None:
        cases = [
            ("/status responde 200.", True),
            ("GET /home/document-summary devuelve un resumen.", True),
            ("Archivo /home/alice/private/config.yaml", False),
            ("Archivo /Users/alice/private/config.yaml", False),
            ("GitHub permissions `id-token: write`.", True),
            ("GitHub permissions id-token: read", True),
            ("GitHub permissions id-token: none", True),
            ("token: write", False),
            ("token: synthetic-secret", False),
            ("password=id-token: write", False),
            ("secret: id-token: none", False),
            ("id-token: write,password=synthetic-secret", False),
            ("id-token: none`secret:synthetic-secret", False),
            ("id-token: nonstandard", False),
            ("id-token: write-secret", False),
            ("custom-id-token: write", False),
            ("id-token: write; token: synthetic-secret", False),
            ("id-token: write ghp_FAKEVALUE1234567890", False),
            ("id-token: write password=synthetic-secret", False),
        ]
        for statement, accepted in cases:
            with self.subTest(statement=statement), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                _, _, oids = make_repository_pair(root)
                gate = gate_v2(oids)
                run_id = self.begin(root, oids)
                artifact = self.semantic_package(SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY], gate)
                artifact['analysis']['claims'][0]['statement'] = statement
                artifact['review']['analysis_digest'] = digest(artifact['analysis'])
                artifact['validation']['analysis_digest'] = digest(artifact['analysis'])
                artifact['validation']['review_digest'] = digest(artifact['review'])
                artifact_path = root / 'oidc-package.json'
                write_json(artifact_path, artifact)
                result = self.run_sync(
                    'checkpoint-package', '--state-root', str(root / 'state'),
                    '--run-id', run_id, '--repository', SOURCE_REPOSITORY,
                    '--artifact', str(artifact_path),
                )
                self.assertEqual(result.returncode, 0 if accepted else 2, result.stdout + result.stderr)
                if accepted:
                    persisted = json.loads(Path(self.payload(result)['artifact_path']).read_text())
                    self.assertEqual(persisted, artifact)
                else:
                    self.assertEqual(self.payload(result)['code'], 'checkpoint-sensitive-content')
                patch_root = root / 'patch-case'
                patch_root.mkdir()
                patch_run_id = self.begin(patch_root, oids)
                self.seal(patch_root, patch_run_id, gate)
                _, projection_path, patch_path = self.unit_files(
                    patch_root, gate, run_id=patch_run_id, unit_id='group-001', unit_type='write-group',
                    path='10-Sistemas/target-service.md', before='before\n',
                    after=statement + '\n', grants=gate['write_groups'][0]['grants'],
                )
                result = self.validate_unit(patch_root, patch_run_id, 'group-001', projection_path, patch_path)
                self.assertEqual(result.returncode, 0 if accepted else 2, result.stdout + result.stderr)
                if not accepted:
                    self.assertEqual(self.payload(result)['code'], 'checkpoint-sensitive-content')


    def test_checkpoint_rejects_raw_token_like_claim_material(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(root, oids)
            artifact = {
                "version": 2, "repository": SOURCE_REPOSITORY,
                "old_oid": "a" * 40, "new_oid": oids[SOURCE_REPOSITORY],
                "paths": [], "checklist": {},
                "claims": [{"claim_id": SOURCE_CLAIM, "statement": "ghp_FAKEVALUE1234567890"}],
                "nodes": [], "result": "documentation-change", "blockers": [],
            }
            artifact_path = root / "raw-token-claim.json"
            write_json(artifact_path, artifact)
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"),
                "--run-id", run_id, "--repository", SOURCE_REPOSITORY,
                "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "checkpoint-sensitive-content")

    def test_closed_receipt_requires_its_exact_run_identity_and_digest(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            receipts = root / "state" / "receipts"
            receipts.mkdir(parents=True)
            forged_run_id = "run-forged-receipt"
            write_json(receipts / f"{forged_run_id}.json", {
                "version": 1,
                "code": "run-closed",
                "status": "complete",
            })
            result = self.run_sync(
                "status", "--state-root", str(root / "state"), "--run-id", forged_run_id,
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "run-receipt-invalid")

    def test_recheckpointed_stale_run_advances_to_seal_gate(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            stale = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(stale.returncode, 0, stale.stdout + stale.stderr)
            next_gate = copy.deepcopy(gate)
            next(item for item in next_gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)["new_oid"] = "d" * 40
            artifact = root / "recheckpoint.json"
            write_json(
                artifact,
                self.semantic_package(SOURCE_REPOSITORY, "d" * 40, next_gate),
            )
            checkpoint = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"),
                "--run-id", run_id, "--repository", SOURCE_REPOSITORY,
                "--artifact", str(artifact),
            )
            self.assertEqual(checkpoint.returncode, 0, checkpoint.stdout + checkpoint.stderr)
            self.assertEqual(self.status(root, run_id)["next_command"], "seal-gate")

    def test_closed_receipt_never_reuses_a_run_with_changed_source_identity(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            next_gate = copy.deepcopy(gate)
            next(item for item in next_gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)["new_oid"] = "d" * 40
            artifact = root / "recheckpoint.json"
            write_json(
                artifact,
                self.semantic_package(SOURCE_REPOSITORY, "d" * 40, next_gate),
            )
            self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"),
                "--run-id", run_id, "--repository", SOURCE_REPOSITORY,
                "--artifact", str(artifact),
            )
            self.seal(root, run_id, next_gate, checkpoint=False)
            group = self.unit_files(
                root, next_gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=next_gate["write_groups"][0]["grants"],
            )
            acknowledgement = self.unit_files(
                root, next_gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(next_gate), grants=[],
            )
            for unit_id, (vault, projection_path, patch_path) in (("acknowledgements", acknowledgement), ("group-001", group)):
                self.assertEqual(self.validate_unit(root, run_id, unit_id, projection_path, patch_path).returncode, 0)
                self.apply_unit(root, run_id, unit_id, vault)
            self.assertEqual(self.run_sync("close", "--state-root", str(root / "state"), "--run-id", run_id).returncode, 0)
            original = self.run_sync(
                "begin", "--state-root", str(root / "state"), "--tool-digest", "a" * 64,
                "--inventory-digest", "b" * 64, "--package", SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY],
                "--package", TARGET_REPOSITORY, oids[TARGET_REPOSITORY],
            )
            self.assertEqual(original.returncode, 0, original.stdout + original.stderr)
            receipt = self.payload(original)
            self.assertEqual(
                next(item for item in receipt["packages"] if item["repository"] == SOURCE_REPOSITORY)["oid"],
                oids[SOURCE_REPOSITORY],
                receipt,
            )

    def test_checkpoint_rejects_unverifiable_closed_digest_receipt(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(root, oids)
            artifact = root / "forged-closed.json"
            write_json(artifact, self.closed_receipt(SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]))
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "checkpoint-lineage-invalid")

    def test_checkpoint_preserves_finalized_package_material_for_gate_recovery(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, oids = make_repository_pair(root)
            run_id = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),))
            manifest = root / "finalized-package.json"
            write_json(
                manifest,
                self.semantic_package(
                    SOURCE_REPOSITORY,
                    oids[SOURCE_REPOSITORY],
                    gate_v2(oids),
                ),
            )
            del source
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(manifest),
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            checkpoint = root / "state" / "active" / run_id / "packages" / SOURCE_REPOSITORY / "artifact.json"
            persisted = json.loads(checkpoint.read_text(encoding="utf-8"))
            self.assertIn("analysis", persisted)
            self.assertIn("review", persisted)

    def test_seal_rejects_claim_authority_not_bound_to_checkpoint(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.checkpoint_packages(root, run_id, gate)
            gate["repositories"][0]["accepted_claim_ids"] = ["claim-invented-001"]
            gate["write_groups"][0]["grants"][0]["claim_id"] = "claim-invented-001"
            gate_path = root / "invented-authority-gate.json"
            write_json(gate_path, gate)
            sealed = self.run_sync(
                "seal-gate", "--state-root", str(root / "state"), "--run-id", run_id,
                "--gate", str(gate_path),
            )
            self.assertEqual(sealed.returncode, 2, sealed.stdout + sealed.stderr)
            self.assertEqual(self.payload(sealed)["code"], "gate-authority-unbound")

    def test_acknowledgement_projection_preserves_prior_cursor_records(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.acknowledgement_only_gate(oids)
            run_id = self.begin(root, oids, packages=((TARGET_REPOSITORY, oids[TARGET_REPOSITORY]),))
            self.seal(root, run_id, gate)
            prior = {
                "repository": "APP00000-prior-service", "branch": "main", "analyzed_sha": "a" * 12,
                "decision": "inspection-limited", "analysis_date": "2026-08-21",
            }
            before = json.dumps({"version": 1, "repositories": [prior]}, sort_keys=True, separators=(",", ":")) + "\n"
            current = json.loads(self.acknowledgement_payload(gate))["repositories"]
            after = json.dumps({"version": 1, "repositories": [prior, *current]}, sort_keys=True, separators=(",", ":")) + "\n"
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before=before, after=after, grants=[],
            )
            result = self.validate_unit(root, run_id, "acknowledgements", projection_path, patch_path)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_empty_file_create_and_delete_change_path_kind(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            path = "10-Sistemas/target-service.md"
            vault = root / "vault"
            patch = f"--- /dev/null\n+++ b/{path}\n@@ -0,0 +0,0 @@\n"
            projection = {
                "version": 1, "run_id": run_id, "gate_digest": digest(gate), "unit_id": "group-001",
                "unit_type": "write-group", "patch_digest": bytes_digest(patch),
                "base_files": {path: bytes_digest("")}, "result_files": {path: bytes_digest("")},
                "grants": gate["write_groups"][0]["grants"],
            }
            projection_path, patch_path = root / "empty-create.json", root / "empty-create.patch"
            write_json(projection_path, projection)
            patch_path.write_text(patch, encoding="utf-8")
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            self.apply_unit(root, run_id, "group-001", vault)
            self.assertTrue((vault / path).is_file())

    def test_review_rejects_empty_file_deletion_kind_mismatch(self) -> None:
        for delete in (True, False):
            with self.subTest(delete=delete), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                _, _, oids = make_repository_pair(root)
                gate = gate_v2(oids)
                run_id = self.begin(root, oids)
                self.seal(root, run_id, gate)
                path = "10-Sistemas/target-service.md"
                target = root / "vault" / path
                target.parent.mkdir(parents=True)
                target.write_bytes(b"")
                new_path = "/dev/null" if delete else f"b/{path}"
                patch = f"--- a/{path}\n+++ {new_path}\n@@ -0,0 +0,0 @@\n"
                projection = {
                    "version": 1, "run_id": run_id, "gate_digest": digest(gate),
                    "unit_id": "group-001", "unit_type": "write-group",
                    "patch_digest": bytes_digest(patch),
                    "base_files": {path: bytes_digest("")},
                    "result_files": {path: bytes_digest("")},
                    "grants": gate["write_groups"][0]["grants"],
                }
                projection_path, patch_path = root / "kind.json", root / "kind.patch"
                write_json(projection_path, projection)
                patch_path.write_text(patch, encoding="utf-8")
                validated = self.validate_unit(
                    root, run_id, "group-001", projection_path, patch_path, review=False,
                )
                self.assertEqual(validated.returncode, 0, validated.stdout)
                rejected = self.review_unit(
                    root, run_id, "group-001", projection_path, patch_path,
                    invert_result_kind=True,
                )
                self.assertEqual(rejected.returncode, 2, rejected.stdout)
                self.assertEqual(self.payload(rejected)["code"], "final-note-review-kind-mismatch")
                blocked = self.run_sync(
                    "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                    "--unit-id", "group-001", "--vault", str(root / "vault"),
                )
                self.assertEqual(self.payload(blocked)["code"], "final-note-review-required")
                self.assertTrue(target.is_file())
                accepted = self.review_unit(root, run_id, "group-001", projection_path, patch_path)
                self.assertEqual(accepted.returncode, 0, accepted.stdout)
                self.apply_unit(root, run_id, "group-001", root / "vault")
                self.assertEqual(target.exists(), not delete)

    def test_malformed_unit_inputs_use_retryable_projection_envelope(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            for name, projection_bytes, patch_bytes in (
                ("invalid-json", b"{", b"--- a/x\n+++ b/x\n"),
                ("non-utf8", json.dumps({"unit_id": "group-001"}).encode(), b"\xff"),
            ):
                with self.subTest(name=name):
                    case = root / name
                    case.mkdir()
                    run_id = self.begin(case, oids)
                    self.seal(case, run_id, gate)
                    projection_path, patch_path = case / "projection.json", case / "unit.patch"
                    projection_path.write_bytes(projection_bytes)
                    patch_path.write_bytes(patch_bytes)
                    result = self.validate_unit(case, run_id, "group-001", projection_path, patch_path)
                    payload = self.payload(result)
                    self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
                    self.assertEqual(payload["code"], "projection-invalid")
                    self.assertTrue(payload.get("retryable"), payload)
                    self.assertEqual(payload["resume_from"], "projection", payload)

    def test_token_like_patch_is_not_checkpointed_as_a_validated_unit(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n",
                after="Documented value ghp_FAKEVALUE1234567890\n", grants=gate["write_groups"][0]["grants"],
            )
            result = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "checkpoint-sensitive-content")

    def test_matching_applied_postimage_digest_is_reused(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            self.apply_unit(root, run_id, "group-001", vault)
            resumed = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--destination-digest", "10-Sistemas/target-service.md", bytes_digest("after\n"),
            )
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            payload = self.payload(resumed)
            self.assertNotIn("group-001", payload["invalidated_units"], payload)

    def test_missing_checkpoint_after_source_invalidation_remains_recoverable(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            checkpoint = root / "state" / "active" / run_id / "packages" / SOURCE_REPOSITORY / "artifact.json"
            checkpoint.unlink()
            result = self.run_sync("status", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["next_command"], "checkpoint-package")

    def test_vault_symlink_parent_is_rejected_before_apply(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            target = vault / "10-Sistemas" / "target-service.md"
            target.unlink()
            target.parent.rmdir()
            redirected = vault / "50-Glosario"
            redirected.mkdir()
            physical = redirected / "target-service.md"
            physical.write_text("before\n", encoding="utf-8")
            (vault / "10-Sistemas").symlink_to(redirected, target_is_directory=True)
            applied = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault),
            )
            self.assertEqual(applied.returncode, 2, applied.stdout + applied.stderr)
            self.assertEqual(self.payload(applied)["code"], "projection-path-invalid")
            self.assertEqual(physical.read_text(encoding="utf-8"), "before\n")

    def test_new_run_advertises_checkpoint_before_seal_gate(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            status = self.status(root, run_id)
            self.assertEqual(status["next_command"], "checkpoint-package", status)

    def test_begin_reuses_active_run_after_source_fingerprint_refresh(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            stale = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(stale.returncode, 0, stale.stdout + stale.stderr)
            reused = self.run_sync(
                "begin", "--state-root", str(root / "state"),
                "--tool-digest", "a" * 64, "--inventory-digest", "b" * 64,
                "--package", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(reused.returncode, 0, reused.stdout + reused.stderr)
            payload = self.payload(reused)
            self.assertTrue(payload["reused"], payload)
            self.assertEqual(payload["run_id"], run_id, payload)

    def test_checkpoint_rejects_self_digested_inconsistent_package_gate(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            artifact = self.semantic_package(SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY], gate)
            artifact["gate"]["fallback_repositories"] = [SOURCE_REPOSITORY]
            artifact["validation"]["gate_digest"] = digest(artifact["gate"])
            artifact_path = root / "inconsistent-closed-package.json"
            write_json(artifact_path, artifact)
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_checkpoint_rejects_self_digested_fabricated_analysis(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            artifact = self.semantic_package(SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY], gate)
            artifact["analysis"] = {
                "repository": SOURCE_REPOSITORY,
                "new_oid": oids[SOURCE_REPOSITORY],
                "claims": [{
                    "claim_id": SOURCE_CLAIM,
                    "statement": "fabricated without repository evidence",
                }],
            }
            artifact["validation"]["analysis_digest"] = digest(artifact["analysis"])
            artifact_path = root / "fabricated-closed-package.json"
            write_json(artifact_path, artifact)
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_wrong_projection_unit_uses_retryable_projection_envelope(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            projection = json.loads(projection_path.read_text(encoding="utf-8"))
            projection["unit_id"] = "acknowledgements"
            write_json(projection_path, projection)
            result = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            payload = self.payload(result)
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(payload["code"], "projection-invalid", payload)
            self.assertTrue(payload["retryable"], payload)
            self.assertEqual(payload["resume_from"], "projection", payload)

    def test_partial_postimage_between_files_blocks_or_recovers_atomically(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault = root / "vault"
            first_path, second_path = "10-Sistemas/target-service.md", "20-Repos/target-service.md"
            first = vault / first_path
            first.parent.mkdir(parents=True)
            first.write_text("before-one\n", encoding="utf-8")
            patch = (
                f"--- a/{first_path}\n+++ b/{first_path}\n@@ -1 +1 @@\n-before-one\n+after-one\n"
                f"--- /dev/null\n+++ b/{second_path}\n@@ -0,0 +1 @@\n+after-two\n"
            )
            projection = {
                "version": 1, "run_id": run_id, "gate_digest": digest(gate),
                "unit_id": "group-001", "unit_type": "write-group",
                "patch_digest": bytes_digest(patch),
                "base_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("")},
                "result_files": {first_path: bytes_digest("after-one\n"), second_path: bytes_digest("after-two\n")},
                "grants": gate["write_groups"][0]["grants"],
            }
            projection_path, patch_path = root / "partial.json", root / "partial.patch"
            write_json(projection_path, projection)
            patch_path.write_text(patch, encoding="utf-8")
            self.assertEqual(
                self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode,
                0,
            )
            crashed = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"),
                "--run-id", run_id, "--unit-id", "group-001",
                "--vault", str(vault), "--crash-after-writes", "1",
            )
            self.assertEqual(crashed.returncode, 99, crashed.stdout + crashed.stderr)
            resumed = self.run_sync("resume", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(resumed.returncode, 2, resumed.stdout + resumed.stderr)

    def test_reseal_preserves_unaffected_apply_failed_projection(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            packages = ((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]), (SECOND_REPOSITORY, "c" * 40))
            run_id = self.begin(root, oids, packages=packages)
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-002", unit_type="write-group",
                path="10-Sistemas/observer-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][1]["grants"],
            )
            self.assertEqual(
                self.validate_unit(root, run_id, "group-002", projection_path, patch_path).returncode, 0,
            )
            (vault / "10-Sistemas/observer-service.md").write_text("conflict\n", encoding="utf-8")
            failed = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-002", "--vault", str(vault),
            )
            self.assertEqual(failed.returncode, 2, failed.stdout + failed.stderr)
            stale = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(stale.returncode, 0, stale.stdout + stale.stderr)
            next_gate = copy.deepcopy(gate)
            next(item for item in next_gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)["new_oid"] = "d" * 40
            artifact_path = root / "source-recheckpoint.json"
            write_json(artifact_path, self.semantic_package(SOURCE_REPOSITORY, "d" * 40, next_gate))
            checkpoint = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact_path),
            )
            self.assertEqual(checkpoint.returncode, 0, checkpoint.stdout + checkpoint.stderr)
            gate_path = root / "resealed-after-failure.json"
            write_json(gate_path, next_gate)
            resealed = self.run_sync(
                "seal-gate", "--state-root", str(root / "state"), "--run-id", run_id,
                "--gate", str(gate_path),
            )
            self.assertEqual(resealed.returncode, 0, resealed.stdout + resealed.stderr)
            unit = {item["unit_id"]: item for item in self.payload(resealed)["units"]}["group-002"]
            self.assertEqual(unit["status"], "apply-failed", unit)
            self.assertTrue(unit["projection_digest"], unit)

    def test_corrected_projection_metadata_reuses_the_same_patch(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, invalid_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="after\n", grants=[],
            )
            first = self.validate_unit(root, run_id, "group-001", invalid_path, patch_path)
            self.assertEqual(first.returncode, 2, first.stdout + first.stderr)
            corrected = json.loads(invalid_path.read_text(encoding="utf-8"))
            corrected["grants"] = gate["write_groups"][0]["grants"]
            corrected_path = root / "corrected-metadata.json"
            write_json(corrected_path, corrected)
            result = self.validate_unit(root, run_id, "group-001", corrected_path, patch_path)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_sk_projection_token_is_not_checkpointed_in_a_validated_patch(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n",
                after="Credential sk-proj-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n",
                grants=gate["write_groups"][0]["grants"],
            )
            result = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "checkpoint-sensitive-content")

    def test_checkpoint_requires_review_or_fallback_for_write_authority(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            gate["repositories"] = [gate["repositories"][0]]
            gate["acknowledgements"] = []
            gate["fallback_repositories"] = []
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            artifact = self.semantic_package(SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY], gate)
            artifact["review"] = None
            artifact["validation"]["review_digest"] = digest(None)
            artifact_path = root / "unreviewed-write-package.json"
            write_json(artifact_path, artifact)
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_sealed_spaced_node_name_remains_resumable(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            for record in gate["repositories"]:
                for field in ("declared_nodes", "affected_nodes", "write_nodes"):
                    record[field] = sorted("Target Service" if item == NODE else item for item in record[field])
            for group in gate["write_groups"]:
                group["nodes"] = sorted("Target Service" if item == NODE else item for item in group["nodes"])
                for grant in group["grants"]:
                    grant["nodes"] = sorted("Target Service" if item == NODE else item for item in grant["nodes"])
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            status = self.status(root, run_id)
            self.assertEqual(status["run_id"], run_id, status)

    def test_direct_apply_retry_rejects_mixed_crash_postimage(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault = root / "vault"
            first_path, second_path = "10-Sistemas/target-service.md", "20-Repos/target-service.md"
            first = vault / first_path
            first.parent.mkdir(parents=True)
            first.write_text("before-one\n", encoding="utf-8")
            patch = (
                f"--- a/{first_path}\n+++ b/{first_path}\n@@ -1 +1 @@\n-before-one\n+after-one\n"
                f"--- /dev/null\n+++ b/{second_path}\n@@ -0,0 +1 @@\n+after-two\n"
            )
            projection = {
                "version": 1, "run_id": run_id, "gate_digest": digest(gate),
                "unit_id": "group-001", "unit_type": "write-group", "patch_digest": bytes_digest(patch),
                "base_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("")},
                "result_files": {first_path: bytes_digest("after-one\n"), second_path: bytes_digest("after-two\n")},
                "grants": gate["write_groups"][0]["grants"],
            }
            projection_path, patch_path = root / "crash.json", root / "crash.patch"
            write_json(projection_path, projection)
            patch_path.write_text(patch, encoding="utf-8")
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            crashed = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault), "--crash-after-writes", "1",
            )
            self.assertEqual(crashed.returncode, 99, crashed.stdout + crashed.stderr)
            retried = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault),
            )
            self.assertEqual(retried.returncode, 2, retried.stdout + retried.stderr)

    def test_backslash_projection_path_uses_retryable_invalid_projection_envelope(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="nested\\segment/target-service.md", before="", after="after\n",
                grants=gate["write_groups"][0]["grants"],
            )
            result = self.validate_unit(root, run_id, "group-001", projection_path, patch_path)
            payload = self.payload(result)
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertEqual(payload["code"], "projection-invalid", payload)
            self.assertTrue(payload["retryable"], payload)
            self.assertEqual(payload["resume_from"], "projection", payload)

    def test_checkpoint_does_not_claim_same_principal_authentication(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, oids = make_repository_pair(root)
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            artifact_path = self.close_fixture_package(
                root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM,
                requested_nodes=(NODE,),
            )
            artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
            artifact["analysis"]["claims"][0]["statement"] = "fabricated claim with plausible exact evidence"
            artifact["review"]["analysis_digest"] = digest(artifact["analysis"])
            artifact["validation"]["analysis_digest"] = digest(artifact["analysis"])
            artifact["validation"]["review_digest"] = digest(artifact["review"])
            write_json(artifact_path, artifact)
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(self.payload(result)["code"], "package-checkpointed")

    def test_checkpoint_rejects_embedded_absolute_source_path(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, oids = make_repository_pair(root)
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            artifact_path = self.close_fixture_package(
                root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM,
                requested_nodes=(NODE,),
            )
            artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
            artifact["analysis"]["claims"][0]["statement"] = "Generic fixture claim from /private/tmp/operator-path."
            artifact["review"]["analysis_digest"] = digest(artifact["analysis"])
            artifact["validation"]["analysis_digest"] = digest(artifact["analysis"])
            artifact["validation"]["review_digest"] = digest(artifact["review"])
            write_json(artifact_path, artifact)
            result = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact_path),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_source_drift_reuses_matching_closed_receipt(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.acknowledgement_only_gate(oids)
            closed_run = self.begin(
                root, oids, packages=((TARGET_REPOSITORY, oids[TARGET_REPOSITORY]),),
            )
            self.seal(root, closed_run, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=closed_run, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(gate), grants=[],
            )
            self.assertEqual(
                self.validate_unit(root, closed_run, "acknowledgements", projection_path, patch_path).returncode,
                0,
            )
            self.apply_unit(root, closed_run, "acknowledgements", vault)
            self.assertEqual(self.run_sync("close", "--state-root", str(root / "state"), "--run-id", closed_run).returncode, 0)
            active = self.begin(
                root, oids, packages=((TARGET_REPOSITORY, "c" * 40),),
            )
            drifted = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", active,
                "--source-oid", TARGET_REPOSITORY, oids[TARGET_REPOSITORY],
            )
            self.assertEqual(drifted.returncode, 0, drifted.stdout + drifted.stderr)
            payload = self.payload(drifted)
            self.assertTrue(payload.get("reused"), payload)
            self.assertEqual(payload["run_id"], closed_run, payload)

    def test_status_prefers_valid_closed_receipt_after_interrupted_cleanup(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.acknowledgement_only_gate(oids)
            run_id = self.begin(
                root, oids, packages=((TARGET_REPOSITORY, oids[TARGET_REPOSITORY]),),
            )
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(gate), grants=[],
            )
            self.assertEqual(
                self.validate_unit(root, run_id, "acknowledgements", projection_path, patch_path).returncode,
                0,
            )
            self.apply_unit(root, run_id, "acknowledgements", vault)
            active = root / "state" / "active" / run_id
            interrupted = root / "interrupted-active"
            shutil.copytree(active, interrupted)
            self.assertEqual(self.run_sync("close", "--state-root", str(root / "state"), "--run-id", run_id).returncode, 0)
            shutil.copytree(interrupted, active)
            (active / "units" / "acknowledgements" / "projection.json").unlink()
            status = self.run_sync("status", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(status.returncode, 0, status.stdout + status.stderr)
            self.assertEqual(self.payload(status)["status"], "complete")
            retried_close = self.run_sync(
                "close", "--state-root", str(root / "state"), "--run-id", run_id,
            )
            self.assertEqual(
                retried_close.returncode, 0, retried_close.stdout + retried_close.stderr,
            )
            self.assertFalse(active.exists())
            next_run = self.run_sync(
                "begin", "--state-root", str(root / "state"),
                "--tool-digest", "a" * 64, "--inventory-digest", "c" * 64,
                "--package", TARGET_REPOSITORY, oids[TARGET_REPOSITORY],
            )
            self.assertEqual(next_run.returncode, 0, next_run.stdout + next_run.stderr)

    def test_reseal_cannot_close_while_withdrawn_applied_postimage_remains(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            gate["repositories"] = [gate["repositories"][0]]
            gate["acknowledgements"] = []
            gate["fallback_repositories"] = []
            run_id = self.begin(
                root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),),
            )
            self.seal(root, run_id, gate)
            vault, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="stale-from-old-oid\n",
                grants=gate["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            self.apply_unit(root, run_id, "group-001", vault)
            next_oid = "d" * 40
            stale = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, next_oid,
            )
            self.assertEqual(stale.returncode, 0, stale.stdout + stale.stderr)
            stale_payload = self.payload(stale)
            self.assertEqual(stale_payload["retracted_units"], ["group-001"], stale_payload)
            target = vault / "10-Sistemas/target-service.md"
            self.assertEqual(target.read_text(encoding="utf-8"), "before\n")
            next_gate = copy.deepcopy(gate)
            source = next(item for item in next_gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)
            source.update({"new_oid": next_oid, "result": "traceability-only", "write_nodes": [], "accepted_claim_ids": [], "cursor_decision": "no-durable-node", "disposition": "cursor-ready"})
            next_gate["write_groups"] = []
            next_gate["acknowledgements"] = [{"repository": SOURCE_REPOSITORY, "new_oid": next_oid, "decision": "no-durable-node", "branch": "main", "analysis_date": "2026-08-22"}]
            next_gate["fallback_repositories"] = []
            artifact = root / "fresh-source-package.json"
            write_json(artifact, self.semantic_package(SOURCE_REPOSITORY, next_oid, next_gate))
            checkpoint = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact),
            )
            self.assertEqual(checkpoint.returncode, 0, checkpoint.stdout + checkpoint.stderr)
            next_gate_path = root / "withdrawn-gate.json"
            gated = self.run_manifest(
                "gate-batch", "--output", str(next_gate_path),
                "--expected-repository", SOURCE_REPOSITORY,
                "--package", self.payload(checkpoint)["artifact_path"],
            )
            self.assertEqual(gated.returncode, 0, gated.stdout + gated.stderr)
            next_gate = json.loads(next_gate_path.read_text(encoding="utf-8"))
            self.assertEqual(
                self.run_sync("seal-gate", "--state-root", str(root / "state"), "--run-id", run_id, "--gate", str(next_gate_path)).returncode,
                0,
            )
            _, ack_projection, ack_patch = self.unit_files(
                root, next_gate, run_id=run_id, unit_id="acknowledgements", unit_type="acknowledgements",
                path=ACK_PATH, before="", after=self.acknowledgement_payload(next_gate), grants=[],
            )
            self.assertEqual(self.validate_unit(root, run_id, "acknowledgements", ack_projection, ack_patch).returncode, 0)
            self.apply_unit(root, run_id, "acknowledgements", vault)
            closed = self.run_sync("close", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(closed.returncode, 0, closed.stdout + closed.stderr)
            self.assertEqual(target.read_text(encoding="utf-8"), "before\n")

    def test_reseal_preserves_validated_group_when_ordinal_changes(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = self.two_group_gate(oids)
            packages = ((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]), (SECOND_REPOSITORY, "c" * 40))
            run_id = self.begin(root, oids, packages=packages)
            self.seal(root, run_id, gate)
            _, projection_path, patch_path = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-002", unit_type="write-group",
                path="10-Sistemas/observer-service.md", before="before\n", after="after\n",
                grants=gate["write_groups"][1]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_id, "group-002", projection_path, patch_path).returncode, 0)
            stale = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, "d" * 40,
            )
            self.assertEqual(stale.returncode, 0, stale.stdout + stale.stderr)
            next_gate = copy.deepcopy(gate)
            source = next(item for item in next_gate["repositories"] if item["repository"] == SOURCE_REPOSITORY)
            source.update({"new_oid": "d" * 40, "result": "traceability-only", "write_nodes": [], "accepted_claim_ids": [], "cursor_decision": "no-durable-node", "disposition": "cursor-ready"})
            next_gate["write_groups"] = [group for group in next_gate["write_groups"] if SOURCE_REPOSITORY not in group["repositories"]]
            next_gate["acknowledgements"] = [{"repository": SOURCE_REPOSITORY, "new_oid": "d" * 40, "decision": "no-durable-node", "branch": "main", "analysis_date": "2026-08-22"}]
            next_gate["fallback_repositories"] = []
            artifact = root / "renumbered-source.json"
            write_json(artifact, self.semantic_package(SOURCE_REPOSITORY, "d" * 40, next_gate))
            checkpoint = self.run_sync(
                "checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id,
                "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact),
            )
            self.assertEqual(checkpoint.returncode, 0, checkpoint.stdout + checkpoint.stderr)
            next_gate_path = root / "renumbered-gate.json"
            run_root = root / "state" / "active" / run_id
            gated = self.run_manifest(
                "gate-batch", "--output", str(next_gate_path),
                "--expected-repository", SOURCE_REPOSITORY, "--expected-repository", SECOND_REPOSITORY,
                "--package", self.payload(checkpoint)["artifact_path"],
                "--package", str(run_root / "packages" / SECOND_REPOSITORY / "artifact.json"),
            )
            self.assertEqual(gated.returncode, 0, gated.stdout + gated.stderr)
            resealed = self.run_sync(
                "seal-gate", "--state-root", str(root / "state"), "--run-id", run_id,
                "--gate", str(next_gate_path),
            )
            self.assertEqual(resealed.returncode, 0, resealed.stdout + resealed.stderr)
            unit = {item["unit_id"]: item for item in self.payload(resealed)["units"]}["group-001"]
            self.assertEqual(unit["status"], "validated", unit)
            self.assertTrue(unit["projection_digest"], unit)

    def test_destination_drift_resume_does_not_start_source_retraction(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            gate["repositories"] = [gate["repositories"][0]]
            gate["acknowledgements"] = []
            gate["fallback_repositories"] = []
            run_id = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),))
            self.seal(root, run_id, gate)
            vault, projection, patch = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="applied\n",
                grants=gate["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection, patch).returncode, 0)
            self.apply_unit(root, run_id, "group-001", vault)
            target = vault / "10-Sistemas/target-service.md"
            target.write_text("user destination edit\n", encoding="utf-8")
            closed = self.run_sync("close", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(closed.returncode, 2, closed.stdout + closed.stderr)
            resumed = self.run_sync("resume", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            payload = self.payload(resumed)
            self.assertEqual(payload["code"], "vault-baseline-stale", payload)
            self.assertEqual(target.read_text(encoding="utf-8"), "user destination edit\n")

    def test_convergent_receipt_restores_receipt_postimage_before_retiring_run(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate_a = gate_v2(oids)
            gate_a["repositories"] = [gate_a["repositories"][0]]
            gate_a["acknowledgements"] = []
            gate_a["fallback_repositories"] = []
            run_a = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),))
            self.seal(root, run_a, gate_a)
            vault, a_projection, a_patch = self.unit_files(
                root, gate_a, run_id=run_a, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="from-a\n",
                grants=gate_a["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_a, "group-001", a_projection, a_patch).returncode, 0)
            self.apply_unit(root, run_a, "group-001", vault)
            self.assertEqual(self.run_sync("close", "--state-root", str(root / "state"), "--run-id", run_a).returncode, 0)
            oid_b = "d" * 40
            gate_b = copy.deepcopy(gate_a)
            gate_b["repositories"][0]["new_oid"] = oid_b
            run_b = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oid_b),))
            self.seal(root, run_b, gate_b)
            _, b_projection, b_patch = self.unit_files(
                root, gate_b, run_id=run_b, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="from-a\n", after="from-b\n",
                grants=gate_b["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_b, "group-001", b_projection, b_patch).returncode, 0)
            self.apply_unit(root, run_b, "group-001", vault)
            resumed = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_b,
                "--source-oid", SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY],
            )
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            payload = self.payload(resumed)
            self.assertTrue(payload.get("reused"), payload)
            self.assertEqual(payload["run_id"], run_a, payload)
            self.assertEqual((vault / "10-Sistemas/target-service.md").read_text(encoding="utf-8"), "from-a\n")

    def test_reseal_keeps_failed_source_retraction_as_a_close_prerequisite(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            gate["repositories"] = [gate["repositories"][0]]
            gate["acknowledgements"] = []
            gate["fallback_repositories"] = []
            run_id = self.begin(root, oids, packages=((SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY]),))
            self.seal(root, run_id, gate)
            vault, projection, patch = self.unit_files(
                root, gate, run_id=run_id, unit_id="group-001", unit_type="write-group",
                path="10-Sistemas/target-service.md", before="before\n", after="from-a\n",
                grants=gate["write_groups"][0]["grants"],
            )
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection, patch).returncode, 0)
            self.apply_unit(root, run_id, "group-001", vault)
            target = vault / "10-Sistemas/target-service.md"
            target.write_text("user edit\n", encoding="utf-8")
            oid_b = "d" * 40
            conflict = self.run_sync(
                "resume", "--state-root", str(root / "state"), "--run-id", run_id,
                "--source-oid", SOURCE_REPOSITORY, oid_b,
            )
            self.assertEqual(conflict.returncode, 2, conflict.stdout + conflict.stderr)
            target.write_text("from-a\n", encoding="utf-8")
            cursor_gate = copy.deepcopy(gate)
            record = cursor_gate["repositories"][0]
            record.update({"new_oid": oid_b, "result": "traceability-only", "write_nodes": [], "accepted_claim_ids": [], "cursor_decision": "no-durable-node", "disposition": "cursor-ready"})
            cursor_gate["write_groups"] = []
            cursor_gate["acknowledgements"] = [{"repository": SOURCE_REPOSITORY, "new_oid": oid_b, "decision": "no-durable-node", "branch": "main", "analysis_date": "2026-08-22"}]
            artifact = root / "cursor.json"
            write_json(artifact, self.semantic_package(SOURCE_REPOSITORY, oid_b, cursor_gate))
            checkpoint = self.run_sync("checkpoint-package", "--state-root", str(root / "state"), "--run-id", run_id, "--repository", SOURCE_REPOSITORY, "--artifact", str(artifact))
            self.assertEqual(checkpoint.returncode, 0, checkpoint.stdout + checkpoint.stderr)
            gate_path = root / "cursor-gate.json"
            gated = self.run_manifest("gate-batch", "--output", str(gate_path), "--expected-repository", SOURCE_REPOSITORY, "--package", self.payload(checkpoint)["artifact_path"])
            self.assertEqual(gated.returncode, 0, gated.stdout + gated.stderr)
            resealed = self.run_sync("seal-gate", "--state-root", str(root / "state"), "--run-id", run_id, "--gate", str(gate_path))
            self.assertEqual(resealed.returncode, 2, resealed.stdout + resealed.stderr)


if __name__ == "__main__":
    unittest.main()
