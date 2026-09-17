#!/usr/bin/env python3
"""Real Git regression checks for public investigation retirement mechanics."""
from __future__ import annotations

import argparse
import importlib.util
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


DIST = Path(__file__).resolve().parents[2]
HELPER = DIST / "kernel/.agents/skills/manage-investigation/scripts/investigation-case.py"
SPEC = importlib.util.spec_from_file_location("retirement_helper", HELPER)
assert SPEC is not None and SPEC.loader is not None
M = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = M
SPEC.loader.exec_module(M)
CASE_ID = "20260915-120000-retirement"


class RetirementTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.vault = Path(temporary.name).resolve()
        self.root = self.vault / "investigations"
        self.root.mkdir()
        self.git("init", "-q")
        self.git("config", "user.name", "Case Maintainer")
        self.git("config", "user.email", "maintainer@example.org")
        self.case = self.root / CASE_ID
        args = argparse.Namespace(
            id=CASE_ID, title="Retirement sample", objective="Investigate sample",
            dedupe_key="retirement-sample", source_ref=["issue EX-12"], source_type="issue",
            requester_role="maintainer", export_intent="undecided", purpose="knowledge",
            vault_outcome="none", learning_outcome="no-learning", request_summary="Bounded case",
            timestamp=None, note_locale="en", identity=M.effective_git_identity(self.vault),
            visibility="published",
        )
        self.open_args = args
        with patch.object(M, "emit"):
            M.open_case(args, self.root)
        public = self.case / "investigation.md"
        text = M.replace_frontmatter_scalar(public.read_text(), "status", "closed")
        text = M.upsert_frontmatter_scalar(text, "closure-outcome", "completed")
        public.write_text(text)
        (self.case / "artifacts" / "attachment.bin").write_bytes(b"\xff\x00artifact")
        (self.case / "exports" / "draft.md").write_text("A shareable draft\n")
        self.commit()
        self.snapshot = self.git("rev-parse", "HEAD").strip()
        self.args = argparse.Namespace(
            id=CASE_ID, authorized=True, snapshot_commit=self.snapshot,
            expected_public_sha256=M.sha256(public), reason="Durable value absorbed",
            source="user request EX-12", dependency_review="No unresolved consumers after review",
            absorption_review="Useful results absorbed into system note",
            summary="Confirmed sample behavior and documented its limits",
            destination=["10-Sistemas/sample.md"], timestamp=None,
            identity=M.effective_git_identity(self.vault),
        )

    def git(self, *args):
        return subprocess.run(["git", "-C", str(self.vault), *args], check=True,
                              capture_output=True, text=True).stdout

    def commit(self):
        self.git("add", "investigations")
        self.git("commit", "-qm", "test: capture investigation")

    def retire(self):
        with patch.object(M, "emit") as emitted:
            M.retire_case(self.args, self.root)
        return emitted.call_args.args[0]

    def assert_rejected(self, code):
        before = {p.relative_to(self.case): p.read_bytes()
                  for p in self.case.rglob("*") if p.is_file()}
        with self.assertRaises(M.CaseError) as raised:
            self.retire()
        self.assertEqual(raised.exception.code, code)
        self.assertEqual(before, {p.relative_to(self.case): p.read_bytes()
                                for p in self.case.rglob("*") if p.is_file()})
        self.assertFalse((self.root / "retired.md").exists())

    def test_exact_snapshot_pending_then_committed_load_list_and_private_unchanged(self):
        private = self.vault / ".investigations-private" / CASE_ID
        private.mkdir(parents=True)
        (private / "private.md").write_text("Private data remains separately owned")
        config = (self.vault / ".git/config").read_bytes()
        output = self.retire()
        self.assertEqual(output["commit_state"], "pending")
        self.assertIsNone(output["retirement_commit"])
        self.assertFalse(self.case.exists())
        self.assertEqual(output["recorded-by"], "Case Maintainer <maintainer@example.org>")
        self.assertEqual(output["snapshot-commit"], self.snapshot)
        self.assertEqual(config, (self.vault / ".git/config").read_bytes())
        self.assertEqual((private / "private.md").read_text(), "Private data remains separately owned")
        self.assertEqual(self.git("rev-parse", "HEAD").strip(), self.snapshot)
        with patch.object(M, "emit") as emitted:
            M.load_case(self.args, self.root)
        self.assertEqual(emitted.call_args.args[0]["status"], "retired")
        with patch.object(M, "emit") as emitted:
            M.list_cases(self.root)
        self.assertEqual(emitted.call_args.args[0]["cases"], [])
        self.assertEqual(len(emitted.call_args.args[0]["retired"]), 1)
        self.commit()
        entry = M.retirement_records(self.root)[CASE_ID]
        view = M.retirement_view(self.root, entry)
        self.assertEqual(view["commit_state"], "committed")
        self.assertEqual(view["retirement_commit"], self.git("rev-parse", "HEAD").strip())
        with self.assertRaises(M.CaseError) as raised:
            M.open_case(self.open_args, self.root)
        self.assertEqual(raised.exception.code, "case_retired")

    def test_closed_gate(self):
        path = self.case / "investigation.md"
        path.write_text(M.replace_frontmatter_scalar(path.read_text(), "status", "investigating"))
        self.assert_rejected("retirement_not_closed")

    def test_nested_vault_uses_repository_relative_git_paths(self):
        nested = self.vault / "nested-vault"
        nested.mkdir()
        self.root.rename(nested / "investigations")
        self.root = nested / "investigations"
        self.case = self.root / CASE_ID
        self.git("add", "-A")
        self.git("commit", "-qm", "test: nest vault")
        self.args.snapshot_commit = self.git("rev-parse", "HEAD").strip()
        self.retire()
        self.git("add", "-A")
        self.git("commit", "-qm", "test: retire nested case")
        entry = M.retirement_records(self.root)[CASE_ID]
        self.assertEqual(M.retirement_view(self.root, entry)["commit_state"], "committed")

    def test_authorization_gate(self):
        self.args.authorized = False
        self.assert_rejected("retirement_not_authorized")

    def test_stale_public(self):
        self.args.expected_public_sha256 = "0" * 64
        self.assert_rejected("stale_public_snapshot")

    def test_changed_attachment(self):
        (self.case / "artifacts/attachment.bin").write_bytes(b"changed")
        self.assert_rejected("retirement_snapshot_changed")

    def test_untracked_attachment(self):
        (self.case / "artifacts/new.bin").write_bytes(b"new")
        self.assert_rejected("retirement_snapshot_changed")

    def test_ignored_attachment(self):
        (self.vault / ".git/info/exclude").write_text("*.ignored\n")
        (self.case / "artifacts/new.ignored").write_bytes(b"new")
        self.assert_rejected("retirement_snapshot_changed")

    def test_missing_attachment(self):
        (self.case / "exports/draft.md").unlink()
        self.assert_rejected("retirement_snapshot_changed")

    def test_missing_snapshot(self):
        self.args.snapshot_commit = "0" * 40
        self.assert_rejected("retirement_git_failed")

    def test_old_snapshot_omits_committed_attachment(self):
        (self.case / "artifacts/new.bin").write_bytes(b"new")
        self.commit()
        self.assert_rejected("retirement_snapshot_changed")

    def test_atomic_write_failure_rolls_back(self):
        original = M.atomic_write

        def fail_register(path, content):
            if path.name == "retired.md":
                raise OSError("injected write failure")
            original(path, content)

        before = (self.case / "investigation.md").read_bytes()
        with patch.object(M, "atomic_write", side_effect=fail_register):
            with self.assertRaises(OSError):
                self.retire()
        self.assertEqual(before, (self.case / "investigation.md").read_bytes())
        self.assertFalse((self.root / "retired.md").exists())
        self.assertEqual(list(self.root.glob(".retire-*")), [])
        self.assertEqual(self.git("status", "--porcelain"), "")

    def test_partial_cleanup_failure_rolls_back(self):
        def partial_delete(path):
            (path / "artifacts/attachment.bin").unlink()
            raise OSError("injected cleanup failure")

        with patch.object(M.shutil, "rmtree", side_effect=partial_delete):
            with self.assertRaises(OSError):
                self.retire()
        self.assertEqual((self.case / "artifacts/attachment.bin").read_bytes(), b"\xff\x00artifact")
        self.assertFalse((self.root / "retired.md").exists())
        self.assertEqual(self.git("status", "--porcelain"), "")

    def test_deletion_and_register_in_separate_commits_remains_pending(self):
        self.retire()
        self.git("add", "investigations/retired.md")
        self.git("commit", "-qm", "test: register only")
        self.commit()
        entry = M.retirement_records(self.root)[CASE_ID]
        self.assertEqual(M.retirement_view(self.root, entry)["commit_state"], "pending")

    def test_register_symlink_rejected(self):
        outside = self.vault / "outside.txt"
        outside.write_text("external")
        (self.root / "retired.md").symlink_to(outside)
        with self.assertRaises(M.CaseError) as raised:
            self.retire()
        self.assertEqual(raised.exception.code, "case_symlink")
        self.assertEqual(outside.read_text(), "external")

    def test_missing_identity_cli_fails_without_git_configuration_mutation(self):
        # Empty local identity overrides any global identity without changing it.
        self.git("config", "user.name", "")
        config = (self.vault / ".git/config").read_bytes()
        with self.assertRaises(M.CaseError) as raised:
            M.effective_git_identity(self.vault)
        self.assertEqual(raised.exception.code, "git_identity_missing")
        self.assertEqual(config, (self.vault / ".git/config").read_bytes())

    def test_symlink_rejected_without_following_target(self):
        outside = self.vault / "outside.txt"
        outside.write_text("outside")
        (self.case / "artifacts/link").symlink_to(outside)
        self.assert_rejected("retirement_validation_failed")
        self.assertEqual(outside.read_text(), "outside")

    def test_inputs_sanitized(self):
        for field, value in (("reason", "token=abc"), ("source", "/Users/someone/file"),
                             ("summary", "line one\nline two"),
                             ("destination", ["/home/person/file"])):
            original = getattr(self.args, field)
            setattr(self.args, field, value)
            with self.assertRaises(M.CaseError):
                self.retire()
            setattr(self.args, field, original)
        self.assertTrue(self.case.exists())

    def test_lineage_and_conflict(self):
        alias = "20260914-120000-previous"
        path = self.case / "investigation.md"
        path.write_text(M.add_consolidated_from(path.read_text(), alias))
        self.commit()
        self.args.snapshot_commit = self.git("rev-parse", "HEAD").strip()
        self.args.expected_public_sha256 = M.sha256(path)
        self.retire()
        with patch.object(M, "emit") as emitted:
            M.load_case(argparse.Namespace(id=alias), self.root)
        self.assertEqual(emitted.call_args.args[0]["id"], CASE_ID)
        self.open_args.id = alias
        with self.assertRaises(M.CaseError):
            M.open_case(self.open_args, self.root)
        self.case.mkdir()
        with self.assertRaises(M.CaseError) as raised:
            M.validate_root(self.root)
        self.assertEqual(raised.exception.code, "retirement_conflict")

    def test_cli_options(self):
        parsed = M.build_parser().parse_args([
            "--root", str(self.root), "retire", "--id", CASE_ID, "--authorized",
            "--snapshot-commit", self.snapshot, "--expected-public-sha256", self.args.expected_public_sha256,
            "--reason", "absorbed", "--source", "issue EX-12", "--summary", "Summary",
            "--dependency-review", "reviewed", "--absorption-review", "reviewed",
            "--destination", "10-Sistemas/sample.md"])
        self.assertEqual(parsed.command, "retire")
        self.assertTrue(parsed.authorized)


if __name__ == "__main__":
    unittest.main()
