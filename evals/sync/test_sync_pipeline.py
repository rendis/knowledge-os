#!/usr/bin/env python3
"""Red public-CLI contracts for resumable, claim-aware synchronization."""
from __future__ import annotations

import json
import hashlib
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from fixtures import (
    NODE,
    SOURCE_CLAIM,
    SOURCE_REPOSITORY,
    TARGET_CLAIM,
    TARGET_REPOSITORY,
    gate_v2,
    make_repository_pair,
    package_artifacts,
    projection,
    write_json,
)


DIST = Path(__file__).resolve().parents[2]
MANIFEST = DIST / "kernel" / "90-Meta" / "git-change-manifest.py"
SYNC_RUN = DIST / "kernel" / "90-Meta" / "sync-run.py"


class ResumableSyncEval(unittest.TestCase):
    def run_manifest(self, *arguments: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-B", str(MANIFEST), *arguments],
            cwd=str(DIST), text=True, capture_output=True, check=False,
        )

    def run_sync(self, *arguments: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-B", str(SYNC_RUN), *arguments],
            cwd=str(DIST), text=True, capture_output=True, check=False,
        )

    def projection_result(
        self,
        root: Path,
        gate: dict[str, object],
        candidate: dict[str, object],
    ) -> dict[str, object]:
        root.mkdir(parents=True, exist_ok=True)
        gate_path, projection_path, patch_path = (
            root / "gate.json", root / "projection.json", root / "unit.patch"
        )
        write_json(gate_path, gate)
        patch_path.write_text(str(candidate.pop("patch")), encoding="utf-8")
        write_json(projection_path, candidate)
        result = self.run_manifest(
            "validate-projection", "--gate", str(gate_path),
            "--projection", str(projection_path), "--patch", str(patch_path),
        )
        self.assertTrue(result.stdout, result.stderr)
        return json.loads(result.stdout)

    def gate_from_generic_packages(
        self,
        root: Path,
        source: Path,
        target: Path,
    ) -> dict[str, object]:
        source_artifacts = package_artifacts(
            root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM,
            requested_nodes=(NODE,),
        )
        target_artifacts = package_artifacts(
            root, target, TARGET_REPOSITORY, claim_id=TARGET_CLAIM,
            rejected_review=True,
        )
        closed_packages = []
        for repository_path, artifacts in (
            (source, source_artifacts), (target, target_artifacts),
        ):
            package_path = root / f"closed-{repository_path.name}.json"
            closed = self.run_manifest(
                "close-package", "--repo", str(repository_path),
                "--manifest", str(artifacts[0]),
                "--scaffold", str(artifacts[1]),
                "--analysis", str(artifacts[2]),
                "--review", str(artifacts[3]),
                "--production-ref", "refs/heads/main",
                "--analysis-date", "2026-08-22",
                "--output", str(package_path),
            )
            self.assertEqual(closed.returncode, 0, closed.stdout + closed.stderr)
            closed_packages.append(package_path)
        gate_path = root / "generated-gate.json"
        result = self.run_manifest(
            "gate-batch", "--output", str(gate_path),
            "--expected-repository", SOURCE_REPOSITORY,
            "--expected-repository", TARGET_REPOSITORY,
            "--package", str(closed_packages[0]),
            "--package", str(closed_packages[1]),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return json.loads(result.stdout)

    def test_accepted_cross_repository_grant_survives_rejected_target_review(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, target, oids = make_repository_pair(root)
            self.assertTrue((source / ".git").exists())
            self.assertTrue((target / ".git").exists())
            gate = self.gate_from_generic_packages(root, source, target)
            self.assertEqual(gate["version"], 2, gate)
            grant = gate["write_groups"][0]["grants"][0]
            self.assertEqual(grant, {
                "repository": SOURCE_REPOSITORY,
                "claim_id": SOURCE_CLAIM,
                "nodes": [NODE],
            })
            self.assertEqual(gate["repositories"][1]["disposition"], "cursor-ready")
            candidate = projection(gate)
            payload = self.projection_result(root, gate, candidate)
            self.assertEqual(payload["status"], "pass", payload)

    def test_projection_invalid_for_rejected_or_invented_grants(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            invalid_grants = (
                [{"repository": TARGET_REPOSITORY, "claim_id": TARGET_CLAIM, "nodes": [NODE]}],
                [{"repository": SOURCE_REPOSITORY, "claim_id": "claim-invented-001", "nodes": [NODE]}],
            )
            for index, grants in enumerate(invalid_grants):
                with self.subTest(grants=grants):
                    candidate = projection(gate, grants=grants)
                    payload = self.projection_result(root / str(index), gate, candidate)
                    self.assertEqual(payload["code"], "projection-invalid", payload)
                    self.assertTrue(payload["retryable"], payload)
                    self.assertEqual(payload["resume_from"], "projection", payload)

    def test_acknowledgement_and_write_group_validate_as_independent_units(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            acknowledgement = projection(
                gate, unit_id="acknowledgements", unit_type="acknowledgements", grants=[]
            )
            group = projection(gate)
            acknowledgement_result = self.projection_result(root / "ack", gate, acknowledgement)
            group_result = self.projection_result(root / "group", gate, group)
            self.assertEqual(acknowledgement_result["status"], "pass", acknowledgement_result)
            self.assertEqual(group_result["status"], "pass", group_result)

    def test_validator_rejects_patch_syntax_the_applier_cannot_apply(self) -> None:
        """A validated projection must never later fail only on patch parsing."""
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            valid_patch = (
                "--- a/10-Sistemas/target-service.md\n"
                "+++ b/10-Sistemas/target-service.md\n"
                "@@ -1 +1 @@\n"
                "-before\n"
                "+after\n"
            )
            candidates = {
                "missing-hunk": (
                    "--- a/10-Sistemas/target-service.md\n"
                    "+++ b/10-Sistemas/target-service.md\n"
                    "+after\n"
                ),
                "trailing-garbage": valid_patch + "not-a-patch-line\n",
            }
            for name, patch in candidates.items():
                with self.subTest(name=name):
                    candidate = projection(gate)
                    candidate["patch_digest"] = hashlib.sha256(
                        patch.encode("utf-8")
                    ).hexdigest()
                    candidate["patch"] = patch
                    payload = self.projection_result(root / name, gate, candidate)
                    self.assertEqual(payload["code"], "projection-invalid", payload)
                    self.assertTrue(payload["retryable"], payload)
                    self.assertEqual(payload["resume_from"], "projection", payload)

    def test_acknowledgement_projection_uses_the_gate_canonical_date_and_branch(self) -> None:
        """Equivalent acknowledgement authority must have one durable encoding."""
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            candidate = projection(
                gate, unit_id="acknowledgements", unit_type="acknowledgements", grants=[],
            )
            alternate = json.dumps({
                "version": 1,
                "repositories": [{
                    "repository": TARGET_REPOSITORY,
                    "branch": "master",
                    "analyzed_sha": oids[TARGET_REPOSITORY][:12],
                    "decision": "review-rejected",
                    "analysis_date": "2026-08-23",
                }],
            }, ensure_ascii=True, sort_keys=True, separators=(",", ":")) + "\n"
            patch = (
                "--- /dev/null\n+++ b/90-Meta/.sync-acknowledgements.json\n"
                "@@ -0,0 +1 @@\n"
                f"+{alternate}"
            )
            candidate["patch"] = patch
            candidate["patch_digest"] = hashlib.sha256(patch.encode("utf-8")).hexdigest()
            candidate["result_files"] = {
                "90-Meta/.sync-acknowledgements.json": hashlib.sha256(
                    alternate.encode("utf-8")
                ).hexdigest(),
            }
            payload = self.projection_result(root, gate, candidate)
            self.assertEqual(payload["code"], "projection-invalid", payload)
            self.assertTrue(payload["retryable"], payload)
            self.assertEqual(payload["resume_from"], "projection", payload)

    def test_write_group_projection_covers_every_granted_node(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            source = gate["repositories"][0]
            source["declared_nodes"].append("unpublished-node")
            source["write_nodes"].append("unpublished-node")
            gate["write_groups"][0]["nodes"].append("unpublished-node")
            gate["write_groups"][0]["grants"][0]["nodes"].append("unpublished-node")
            payload = self.projection_result(root, gate, projection(gate))
            self.assertEqual(payload["code"], "projection-invalid", payload)
            self.assertTrue(payload["retryable"], payload)
            self.assertEqual(payload["resume_from"], "projection", payload)

    def test_write_group_cannot_target_agent_instruction_paths(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            candidate = projection(gate)
            original = "10-Sistemas/target-service.md"
            forbidden = ".agents/skills/unrelated/target-service.md"
            patch = str(candidate["patch"]).replace(original, forbidden)
            candidate["patch"] = patch
            candidate["patch_digest"] = hashlib.sha256(patch.encode("utf-8")).hexdigest()
            candidate["base_files"] = {forbidden: candidate["base_files"][original]}
            candidate["result_files"] = {forbidden: candidate["result_files"][original]}
            payload = self.projection_result(root, gate, candidate)
            self.assertEqual(payload["code"], "projection-invalid", payload)
            self.assertTrue(payload["retryable"], payload)
            self.assertEqual(payload["resume_from"], "projection", payload)

    def test_close_package_rechecks_the_declared_production_ref_oid(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, _ = make_repository_pair(root)
            artifacts = package_artifacts(
                root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM,
                requested_nodes=(NODE,),
            )
            checked = self.run_manifest(
                "check", "--repo", str(source), "--manifest", str(artifacts[0]),
                "--scaffold", str(artifacts[1]), "--analysis", str(artifacts[2]),
                "--current-ref", "refs/heads/main",
            )
            self.assertEqual(checked.returncode, 0, checked.stdout + checked.stderr)
            (source / "component.txt").write_text("branch advanced\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(source), "add", "component.txt"], check=True)
            subprocess.run(["git", "-C", str(source), "commit", "-qm", "advance main"], check=True)
            closed = self.run_manifest(
                "close-package", "--repo", str(source), "--manifest", str(artifacts[0]),
                "--scaffold", str(artifacts[1]), "--analysis", str(artifacts[2]),
                "--review", str(artifacts[3]),
                "--production-ref", "refs/heads/main",
                "--analysis-date", "2026-08-22", "--output", str(root / "closed.json"),
            )
            self.assertEqual(closed.returncode, 2, closed.stdout + closed.stderr)

    def test_close_package_accepts_frozen_remote_ref_when_local_branch_is_stale(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, _ = make_repository_pair(root)
            artifacts = package_artifacts(
                root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM,
                requested_nodes=(NODE,),
            )
            analyzed_oid = json.loads(artifacts[0].read_text(encoding="utf-8"))["new_oid"]
            subprocess.run(
                ["git", "-C", str(source), "update-ref", "refs/remotes/origin/main", analyzed_oid],
                check=True,
            )
            (source / "component.txt").write_text("local branch advanced\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(source), "add", "component.txt"], check=True)
            subprocess.run(["git", "-C", str(source), "commit", "-qm", "advance local main"], check=True)
            closed = self.run_manifest(
                "close-package", "--repo", str(source), "--manifest", str(artifacts[0]),
                "--scaffold", str(artifacts[1]), "--analysis", str(artifacts[2]),
                "--review", str(artifacts[3]),
                "--production-ref", "refs/remotes/origin/main",
                "--analysis-date", "2026-08-22", "--output", str(root / "closed.json"),
            )
            self.assertEqual(closed.returncode, 0, closed.stdout + closed.stderr)

    def test_close_package_rejects_nested_branch_masquerading_as_main(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, _ = make_repository_pair(root)
            artifacts = package_artifacts(
                root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM,
                requested_nodes=(NODE,),
            )
            analyzed_oid = json.loads(artifacts[0].read_text(encoding="utf-8"))["new_oid"]
            subprocess.run(
                ["git", "-C", str(source), "update-ref", "refs/heads/archive/main", analyzed_oid],
                check=True,
            )
            closed = self.run_manifest(
                "close-package", "--repo", str(source), "--manifest", str(artifacts[0]),
                "--scaffold", str(artifacts[1]), "--analysis", str(artifacts[2]),
                "--review", str(artifacts[3]),
                "--production-ref", "refs/heads/archive/main",
                "--analysis-date", "2026-08-22", "--output", str(root / "closed.json"),
            )
            self.assertEqual(closed.returncode, 2, closed.stdout + closed.stderr)

    def test_write_projection_binds_declared_postimage_to_patch_bytes(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            candidate = projection(gate)
            candidate["result_files"] = {
                "10-Sistemas/target-service.md": "f" * 64,
            }
            payload = self.projection_result(root, gate, candidate)
            self.assertEqual(payload["code"], "projection-invalid", payload)
            self.assertTrue(payload["retryable"], payload)
            self.assertEqual(payload["resume_from"], "projection", payload)

    def test_begin_is_deterministic_and_resume_requires_durable_run_state(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            arguments = (
                "begin", "--state-root", str(root / "state"),
                "--tool-digest", "a" * 64, "--inventory-digest", "b" * 64,
                "--package", SOURCE_REPOSITORY, oids[SOURCE_REPOSITORY],
                "--package", TARGET_REPOSITORY, oids[TARGET_REPOSITORY],
            )
            first, second = self.run_sync(*arguments), self.run_sync(*arguments)
            self.assertEqual(first.returncode, 0, first.stdout + first.stderr)
            self.assertEqual(second.returncode, 0, second.stdout + second.stderr)
            first_payload, second_payload = json.loads(first.stdout), json.loads(second.stdout)
            self.assertEqual(first_payload["run_id"], second_payload["run_id"])
            run_id = first_payload["run_id"]
            run_json = root / "state" / "active" / run_id / "run.json"
            self.assertTrue(run_json.is_file())
            run_json.unlink()
            resumed = self.run_sync("resume", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(resumed.returncode, 2, resumed.stdout + resumed.stderr)
            payload = json.loads(resumed.stdout)
            self.assertEqual(payload["code"], "run-state-missing", payload)


if __name__ == "__main__":
    unittest.main()
