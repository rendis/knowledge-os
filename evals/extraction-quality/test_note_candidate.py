"""Regression tests for the final full-note review binding."""
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "review_note_candidate", ROOT / "kernel/90-Meta/review-note-candidate.py"
)
tool = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(tool)


class NoteCandidateReview(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        root = Path(self.temporary.name)
        self.vault = root / "vault"
        self.candidate = root / "candidate"
        self.evidence = root / "evidence"
        for path in (self.vault, self.candidate, self.evidence):
            path.mkdir()
        relative = Path("20-Repos/demo/service.md")
        (self.vault / relative).parent.mkdir(parents=True)
        (self.candidate / relative).parent.mkdir(parents=True)
        (self.vault / relative).write_text(
            "# Service\n\nOld prose.\n<!-- connection:connection.orders -->\n",
            encoding="utf-8",
        )
        (self.candidate / relative).write_text(
            "# Service\n\nFinal prose.\n<!-- connection:connection.orders -->\n"
            "<!-- connection:connection.audit -->\n",
            encoding="utf-8",
        )
        (self.evidence / "record.json").write_text('{"observed":true}\n', encoding="utf-8")
        self.manifest_path = root / "manifest.json"
        self.review_path = root / "review.json"
        self.manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"]
        )
        self.manifest_path.write_text(json.dumps(self.manifest), encoding="utf-8")
        self.review = {
            "version": 1,
            "manifest_digest": tool.canonical_digest(self.manifest),
            "verdict": "accept",
            "findings": [],
            "connection_decisions": {
                "20-Repos/demo/service.md#connection.orders": {
                    "action": "update", "reason": "The final prose clarifies this flow."
                },
                "20-Repos/demo/service.md#connection.audit": {
                    "action": "create", "reason": "The evidence establishes the audit flow."
                },
            },
        }
        self.write_review()

    def write_review(self):
        self.review_path.write_text(json.dumps(self.review), encoding="utf-8")

    def check(self):
        return tool.check_candidate(
            self.vault, self.candidate, self.evidence,
            self.manifest_path, self.review_path,
        )

    def test_accepted_review_binds_final_prose(self):
        self.assertEqual(self.check()["status"], "pass")
        note = self.candidate / "20-Repos/demo/service.md"
        note.write_text(note.read_text() + "stale change\n", encoding="utf-8")
        with self.assertRaisesRegex(tool.NoteCandidateError, "stale"):
            self.check()

    def test_evidence_drift_is_refused(self):
        (self.evidence / "record.json").write_text('{"observed":false}\n', encoding="utf-8")
        with self.assertRaisesRegex(tool.NoteCandidateError, "stale"):
            self.check()

    def test_stale_diagnostics_identify_each_changed_source(self):
        relative = "20-Repos/demo/service.md"
        for category, root, path in (
            ("candidate", self.candidate, relative),
            ("base", self.vault, relative),
            ("evidence", self.evidence, "record.json"),
        ):
            with self.subTest(category=category):
                target = root / path
                original = target.read_bytes()
                target.write_bytes(original + b"\nChanged.\n")
                try:
                    with self.assertRaises(tool.NoteCandidateError) as caught:
                        self.check()
                    self.assertEqual(str(caught.exception), "candidate-base-or-evidence-stale")
                    self.assertEqual(caught.exception.changes, [{"category": category, "paths": [path]}])
                finally:
                    target.write_bytes(original)

    def test_cli_reports_all_drift_without_mutating_artifacts(self):
        relative = "20-Repos/demo/service.md"
        note = self.vault / relative
        note.write_text(note.read_text() + "Concurrent update.\n")
        (self.evidence / "record.json").write_text('{"observed":false}\n')
        before = {path: path.read_bytes() for path in Path(self.temporary.name).rglob("*") if path.is_file()}
        result = subprocess.run([
            sys.executable, "-B", str(ROOT / "kernel/90-Meta/review-note-candidate.py"),
            "check", "--vault", str(self.vault), "--candidate", str(self.candidate),
            "--evidence-root", str(self.evidence), "--manifest", str(self.manifest_path),
            "--review", str(self.review_path),
        ], capture_output=True, text=True, check=False)
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual(json.loads(result.stdout), {
            "status": "blocked", "code": "candidate-base-or-evidence-stale",
            "changes": [
                {"category": "base", "paths": [relative]},
                {"category": "evidence", "paths": ["record.json"]},
            ],
        })
        self.assertEqual(before, {path: path.read_bytes() for path in before})

    def test_empty_base_removal_reports_presence_drift(self):
        relative = "20-Repos/demo/service.md"
        note = self.vault / relative
        note.write_text("")
        manifest = tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])
        self.manifest_path.write_text(json.dumps(manifest))
        note.unlink()
        with self.assertRaises(tool.NoteCandidateError) as caught:
            self.check()
        self.assertEqual(str(caught.exception), "candidate-base-or-evidence-stale")
        self.assertEqual(caught.exception.changes, [{"category": "base", "paths": [relative]}])

    def test_published_drift_and_later_review(self):
        pairs = [[self.manifest_path, self.review_path]]
        self.assertEqual(tool.check_published(self.vault, pairs)["status"], "blocked")
        relative = "20-Repos/demo/service.md"
        note = self.vault / relative
        shutil.copyfile(self.candidate / relative, note)
        self.assertEqual(tool.check_published(self.vault, pairs)["status"], "pass")
        (self.candidate / relative).write_text(note.read_text() + "\nReviewed addition.\n")
        later = tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])
        manifest_path = self.manifest_path.with_name("later-manifest.json")
        review_path = self.review_path.with_name("later-review.json")
        manifest_path.write_text(json.dumps(later))
        review = dict(self.review, manifest_digest=tool.canonical_digest(later))
        review["connection_decisions"] = {
            key: {"action": "preserve", "reason": "Existing connection preserved."}
            for key in self.review["connection_decisions"]
        }
        review_path.write_text(json.dumps(review))
        shutil.copyfile(self.candidate / relative, note)
        self.assertEqual(tool.check_published(self.vault, pairs)["mismatches"], [relative])
        pairs.append([manifest_path, review_path])
        self.assertEqual(tool.check_published(self.vault, pairs)["status"], "pass")
        review["verdict"] = "revise"
        review["findings"] = [{"reason": "Not accepted."}]
        review_path.write_text(json.dumps(review))
        with self.assertRaises(tool.NoteCandidateError):
            tool.check_published(self.vault, pairs)

    def test_published_deletion_requires_absence(self):
        relative = "20-Repos/demo/service.md"
        (self.candidate / relative).unlink()
        manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"],
            deleted_paths=[relative],
        )
        self.manifest_path.write_text(json.dumps(manifest))
        review = dict(self.review, manifest_digest=tool.canonical_digest(manifest))
        review["connection_decisions"] = {
            relative + "#connection.orders": {"action": "retire", "reason": "Retired."}
        }
        self.review_path.write_text(json.dumps(review))
        pairs = [[self.manifest_path, self.review_path]]
        (self.vault / relative).write_text("")
        self.assertEqual(tool.check_published(self.vault, pairs)["status"], "blocked")
        (self.vault / relative).unlink()
        self.assertEqual(tool.check_published(self.vault, pairs)["status"], "pass")

    def test_base_drift_is_refused(self):
        note = self.vault / "20-Repos/demo/service.md"
        note.write_text(note.read_text().replace("Old", "Concurrent"), encoding="utf-8")
        with self.assertRaisesRegex(tool.NoteCandidateError, "stale"):
            self.check()

    def test_reviewer_revise_is_refused(self):
        self.review.update(verdict="revise", findings=[{"reason": "Unsupported claim."}])
        self.write_review()
        with self.assertRaisesRegex(tool.NoteCandidateError, "requires-revision"):
            self.check()

    def test_connection_decisions_require_exact_union_and_valid_transition(self):
        del self.review["connection_decisions"]["20-Repos/demo/service.md#connection.orders"]
        self.write_review()
        with self.assertRaisesRegex(tool.NoteCandidateError, "connection-decisions-invalid"):
            self.check()
        self.review["connection_decisions"]["20-Repos/demo/service.md#connection.orders"] = {
            "action": "retire", "reason": "Wrong transition."
        }
        self.write_review()
        with self.assertRaisesRegex(tool.NoteCandidateError, "connection-decisions-invalid"):
            self.check()

    def test_duplicate_connection_anchors_are_refused_in_candidate_and_base(self):
        candidate = self.candidate / "20-Repos/demo/service.md"
        candidate.write_text(
            candidate.read_text(encoding="utf-8")
            + "<!-- connection:connection.orders -->\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(tool.NoteCandidateError, "duplicate-connection-anchor"):
            tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])

        candidate.write_text(
            "# Service\n<!-- connection:connection.orders -->\n", encoding="utf-8"
        )
        base = self.vault / "20-Repos/demo/service.md"
        base.write_text(
            base.read_text(encoding="utf-8")
            + "<!-- connection:connection.orders -->\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(tool.NoteCandidateError, "duplicate-connection-anchor"):
            tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])

    def test_rejects_noncanonical_and_symlink_candidate_paths(self):
        extra = self.candidate / "90-Meta/unsafe.md"
        extra.parent.mkdir()
        extra.write_text("unsafe\n", encoding="utf-8")
        with self.assertRaisesRegex(tool.NoteCandidateError, "not-authorized"):
            tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])
        extra.unlink()
        extra.parent.rmdir()
        link = self.candidate / "20-Repos/demo/link.md"
        link.symlink_to(self.candidate / "20-Repos/demo/service.md")
        with self.assertRaisesRegex(tool.NoteCandidateError, "unsafe-path"):
            tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])

    def test_rejects_symlink_ancestors_and_missing_target_below_symlink(self):
        root = self.vault.parent
        vault_link = root / "vault-link"
        vault_link.symlink_to(self.vault, target_is_directory=True)
        with self.assertRaisesRegex(tool.NoteCandidateError, "unsafe-path"):
            tool.freeze_candidate(vault_link, self.candidate, self.evidence, ["record.json"])

        candidate_note = self.candidate / "20-Repos/linked/new.md"
        candidate_note.parent.mkdir(parents=True)
        candidate_note.write_text("# New\n", encoding="utf-8")
        outside = root / "outside"
        outside.mkdir()
        (self.vault / "20-Repos/linked").symlink_to(outside, target_is_directory=True)
        with self.assertRaisesRegex(tool.NoteCandidateError, "unsafe-path"):
            tool.freeze_candidate(self.vault, self.candidate, self.evidence, ["record.json"])
        output_parent = root / "real-output"
        output_parent.mkdir()
        output_link = root / "output-link"
        output_link.symlink_to(output_parent, target_is_directory=True)
        with self.assertRaisesRegex(tool.NoteCandidateError, "unsafe-path"):
            tool.write_json(output_link / "manifest.json", {"value": 1})

    def test_simulated_pending_to_closed_lifecycle_keeps_connection_anchor(self):
        """The caller applies accepted images; the helper itself never writes the vault."""
        root = self.vault.parent
        path = Path("30-Flujos/Fulfilment.md")
        image = self.candidate / path
        image.parent.mkdir(exist_ok=True)
        anchor = "<!-- connection:connection.fulfilment -->"
        image.write_text(f"# Fulfilment\n\nStatus: pending\n{anchor}\n", encoding="utf-8")
        source_metadata = {
            "repository": "fixture-service", "revision": "abc123", "observed_on": "2026-09-13"
        }
        (self.evidence / "record.json").write_text(json.dumps({
            "source": source_metadata, "simulated_external_status": "pending",
            "destination": "queue-v1",
        }), encoding="utf-8")

        first_manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"]
        )
        first_manifest_path = root / "first-manifest.json"
        first_review_path = root / "first-review.json"
        first_manifest_path.write_text(json.dumps(first_manifest), encoding="utf-8")
        first_review = dict(self.review)
        first_review["manifest_digest"] = tool.canonical_digest(first_manifest)
        first_review["connection_decisions"] = dict(first_review["connection_decisions"])
        first_review["connection_decisions"][f"{path.as_posix()}#connection.fulfilment"] = {
            "action": "create", "reason": "The simulated fixture supports a pending connection."
        }
        first_review_path.write_text(json.dumps(first_review), encoding="utf-8")
        self.assertEqual(tool.check_candidate(
            self.vault, self.candidate, self.evidence,
            first_manifest_path, first_review_path,
        )["status"], "pass")
        (self.vault / path).parent.mkdir(exist_ok=True)
        shutil.copyfile(image, self.vault / path)

        closed_evidence = {
            "source": source_metadata, "simulated_external_status": "closed",
            "destination": "queue-v1",
        }
        (self.evidence / "record.json").write_text(
            json.dumps(closed_evidence), encoding="utf-8"
        )
        self.assertEqual(closed_evidence["source"], source_metadata)
        image.write_text(
            f"# Fulfilment\n\nStatus: closed\nStale: still pending\n{anchor}\n",
            encoding="utf-8",
        )
        stale_manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"]
        )
        stale_manifest_path = root / "stale-manifest.json"
        stale_review_path = root / "stale-review.json"
        stale_manifest_path.write_text(json.dumps(stale_manifest), encoding="utf-8")
        stale_review = {
            "version": 1,
            "manifest_digest": tool.canonical_digest(stale_manifest),
            "verdict": "revise",
            "findings": [{"reason": "Simulated reviewer found stale retained prose."}],
            "connection_decisions": {
                key: {
                    "action": "update" if key.endswith("connection.fulfilment") else value["action"],
                    "reason": "The simulated source change was reviewed."
                }
                for key, value in first_review["connection_decisions"].items()
            },
        }
        stale_review_path.write_text(json.dumps(stale_review), encoding="utf-8")
        with self.assertRaisesRegex(tool.NoteCandidateError, "requires-revision"):
            tool.check_candidate(
                self.vault, self.candidate, self.evidence,
                stale_manifest_path, stale_review_path,
            )

        image.write_text(f"# Fulfilment\n\nStatus: closed\n{anchor}\n", encoding="utf-8")
        final_manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"]
        )
        final_manifest_path = root / "final-manifest.json"
        final_review_path = root / "final-review.json"
        final_manifest_path.write_text(json.dumps(final_manifest), encoding="utf-8")
        final_review = dict(stale_review, verdict="accept", findings=[])
        final_review["manifest_digest"] = tool.canonical_digest(final_manifest)
        final_review_path.write_text(json.dumps(final_review), encoding="utf-8")
        self.assertEqual(tool.check_candidate(
            self.vault, self.candidate, self.evidence,
            final_manifest_path, final_review_path,
        )["status"], "pass")
        shutil.copyfile(image, self.vault / path)
        final_text = (self.vault / path).read_text(encoding="utf-8")
        self.assertIn("Status: closed", final_text)
        self.assertIn(anchor, final_text)
        self.assertNotIn("still pending", final_text)

        destination_evidence = dict(closed_evidence, destination="archive-v2")
        (self.evidence / "record.json").write_text(
            json.dumps(destination_evidence), encoding="utf-8"
        )
        self.assertEqual(destination_evidence["source"], source_metadata)
        image.write_text(
            f"# Fulfilment\n\nStatus: closed\nDestination: archive-v2\n{anchor}\n",
            encoding="utf-8",
        )
        destination_manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"]
        )
        destination_manifest_path = root / "destination-manifest.json"
        destination_review_path = root / "destination-review.json"
        destination_manifest_path.write_text(json.dumps(destination_manifest), encoding="utf-8")
        destination_review = dict(final_review)
        destination_review["manifest_digest"] = tool.canonical_digest(destination_manifest)
        destination_review_path.write_text(json.dumps(destination_review), encoding="utf-8")
        self.assertEqual(tool.check_candidate(
            self.vault, self.candidate, self.evidence,
            destination_manifest_path, destination_review_path,
        )["status"], "pass")
        shutil.copyfile(image, self.vault / path)
        destination_text = (self.vault / path).read_text(encoding="utf-8")
        self.assertIn("Destination: archive-v2", destination_text)
        self.assertEqual(destination_text.count(anchor), 1)

    def test_projection_binds_base_and_result_images(self):
        projection = {
            "base_files": self.manifest["base_files"],
            "result_files": self.manifest["candidate_files"],
        }
        projection_path = self.manifest_path.parent / "projection.json"
        projection_path.write_text(json.dumps(projection), encoding="utf-8")
        bound = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"], projection_path
        )
        self.assertEqual(bound["projection_digest"], tool.canonical_digest(projection))
        projection["result_files"]["20-Repos/demo/service.md"] = "0" * 64
        projection_path.write_text(json.dumps(projection), encoding="utf-8")
        with self.assertRaisesRegex(tool.NoteCandidateError, "projection-file-binding-invalid"):
            tool.freeze_candidate(
                self.vault, self.candidate, self.evidence, ["record.json"], projection_path
            )

    def test_projected_check_reports_source_or_projection_drift(self):
        relative = "20-Repos/demo/service.md"
        projection = {
            "base_files": dict(self.manifest["base_files"]),
            "result_files": dict(self.manifest["candidate_files"]),
        }
        projection_path = self.manifest_path.parent / "projection.json"
        projection_path.write_text(json.dumps(projection))
        bound = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"], projection_path
        )
        self.manifest_path.write_text(json.dumps(bound))
        self.review["manifest_digest"] = tool.canonical_digest(bound)
        self.write_review()
        for category, target, reported_path in (
            ("candidate", self.candidate / relative, relative),
            ("base", self.vault / relative, relative),
            ("projection", projection_path, str(projection_path)),
        ):
            with self.subTest(category=category):
                original = target.read_bytes()
                if category == "projection":
                    changed = json.loads(original)
                    changed["result_files"][relative] = "0" * 64
                    target.write_text(json.dumps(changed))
                else:
                    target.write_bytes(original + b"\nConcurrent change.\n")
                try:
                    with self.assertRaises(tool.NoteCandidateError) as caught:
                        tool.check_candidate(
                            self.vault, self.candidate, self.evidence,
                            self.manifest_path, self.review_path, projection_path,
                        )
                    self.assertEqual(str(caught.exception), "projection-file-binding-invalid")
                    self.assertEqual(caught.exception.changes, [{
                        "category": category, "paths": [reported_path],
                    }])
                finally:
                    target.write_bytes(original)

    def test_deletion_requires_retirement_decisions(self):
        deleted = "20-Repos/demo/removed.md"
        removed = self.vault / deleted
        removed.write_text(
            "# Removed\n<!-- connection:connection.legacy -->\n", encoding="utf-8"
        )
        manifest = tool.freeze_candidate(
            self.vault, self.candidate, self.evidence, ["record.json"],
            deleted_paths=[deleted],
        )
        self.assertEqual(manifest["candidate_files"][deleted], tool.EMPTY_DIGEST)
        self.assertEqual(manifest["connections"][deleted]["candidate"], [])
        review = dict(self.review)
        review["manifest_digest"] = tool.canonical_digest(manifest)
        review["connection_decisions"] = dict(review["connection_decisions"])
        review["connection_decisions"][f"{deleted}#connection.legacy"] = {
            "action": "retire", "reason": "The reviewed candidate removes the obsolete note."
        }
        tool.validate_review(review, manifest)


if __name__ == "__main__":
    unittest.main()
