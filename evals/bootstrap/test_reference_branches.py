"""Configured source branches flow through inventory, notes and sealed packages."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

DIST = Path(__file__).resolve().parents[2]
META = DIST / "kernel/90-Meta"
sys.path.insert(0, str(META))
SOURCE_REPOSITORY = "FIX-source-service"
from instance import dump_instance, load_instance


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


class ReferenceBranchTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixture = tempfile.TemporaryDirectory()
        cls.vault = Path(cls.fixture.name) / "vault"
        subprocess.run([str(DIST / "install.sh"), "init", "--dest", str(cls.vault),
                        "--cell-name", "Fixture", "--purpose", "Branch tests",
                        "--system", "fixture:Fixture", "--yes"], check=True, capture_output=True)
        instance = load_instance(cls.vault / "instance.yaml")
        instance["sources"]["reference_branches"] = {SOURCE_REPOSITORY: "release/stable"}
        (cls.vault / "instance.yaml").write_text(dump_instance(instance))
        cls.inventory = load("branch_inventory", cls.vault / "90-Meta/vault-inventory.py")
        cls.audit = load("branch_audit", cls.vault / "90-Meta/audit-vault.py")

    @classmethod
    def tearDownClass(cls):
        cls.fixture.cleanup()

    def test_inventory_uses_explicit_branch_and_never_falls_back(self):
        tool = self.inventory
        repository = {"name": SOURCE_REPOSITORY, "main": {"target": {"oid": "a" * 40}}}
        listing = {"data": {"organization": {"repositories": {"nodes": [repository], "pageInfo": {"hasNextPage": False}}}}}
        github = tool.GitHubContext(login="test", source="test", token="unused")
        for reference, expected in (({"target": {"oid": "b" * 40}}, "release/stable"), (None, None)):
            replies = [subprocess.CompletedProcess([], 0, json.dumps(listing)),
                       subprocess.CompletedProcess([], 0, json.dumps({"data": {"repository": {"ref": reference}}}))]
            with patch.object(tool, "is_tracked_repository", return_value=True), patch.object(tool, "run", side_effect=replies) as run:
                result = tool.load_org("test", github)
            self.assertEqual(result[0]["branch"], expected)
            self.assertIn("ref=refs/heads/release/stable", run.call_args.args[0])
            self.assertEqual(result[0]["sha"], "b" * 12 if reference else None)

    def test_note_branch_history_remains_readable(self):
        note = self.vault / "20-Repos/Fixture/display-name.md"
        note.parent.mkdir(parents=True, exist_ok=True)
        for branch, allowed in (("release/stable", True), ("main", True), ("HEAD~1", False)):
            note.write_text(f"---\naliases: [{SOURCE_REPOSITORY}]\nrama-analizada: {branch}\n---\n")
            issues = self.audit.audit_repo(note, self.vault)
            self.assertEqual(not any("rama-analizada" in issue for issue in issues), allowed)

    def test_learning_rejects_scratch_as_durable_evidence(self):
        note = self.vault / "70-Aprendizajes/Aprendizaje - scratch source.md"
        note.write_text(
            "---\ntipo: aprendizaje\n---\n"
            "## Evidencia acumulada\n### EV-001\n"
            "- Fuentes durables: `.scratch/report/method.py`\n",
            encoding="utf-8",
        )
        issues = self.audit.audit_learning(note, self.vault)
        self.assertTrue(any("reference local ignored workspaces" in issue for issue in issues))
        note.write_text(
            note.read_text(encoding="utf-8").replace(
                ".scratch/report/method.py", "https://example.test/method.py"
            ),
            encoding="utf-8",
        )
        issues = self.audit.audit_learning(note, self.vault)
        self.assertFalse(any("reference local ignored workspaces" in issue for issue in issues))

    def test_branch_change_with_same_sha_needs_reanalysis(self):
        tool = self.inventory
        note = {"repo": SOURCE_REPOSITORY, "note": "display-name", "recorded_branch": "main", "recorded_sha": "a" * 12, "valid": True}
        remote = {"name": SOURCE_REPOSITORY, "branch": "release/stable", "sha": "a" * 12}
        github = tool.GitHubContext(login="test", source="test", token="unused")
        with patch.object(tool, "load_notes", return_value=[note]), patch.object(tool, "load_acknowledgements", return_value=[]), patch.object(tool, "load_org", return_value=[remote]), patch.object(tool, "is_tracked_repository", return_value=True):
            result = tool.build_inventory(self.vault, "test", github)
        self.assertEqual(result[0]["status"], "changed")
        acknowledgement = {"repository": SOURCE_REPOSITORY, "branch": "release/stable", "analyzed_sha": "a" * 12, "decision": "no-documentation-change", "analysis_date": "2026-09-14"}
        with patch.object(tool, "load_notes", return_value=[note]), patch.object(tool, "load_acknowledgements", return_value=[acknowledgement]), patch.object(tool, "load_org", return_value=[remote]), patch.object(tool, "is_tracked_repository", return_value=True):
            result = tool.build_inventory(self.vault, "test", github)
        self.assertEqual(result[0]["status"], "acknowledged-no-change")


if __name__ == "__main__":
    unittest.main()
