"""Provider-neutral catalog/configuration and retirement behavior."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "kernel/90-Meta"))
from instance import dump_instance, load_instance, validate_instance


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


config = load_module("neutral_config", ROOT / "kernel/90-Meta/cell-config.py")
installer = load_module("neutral_installer", ROOT / "scripts/knowledge_os.py")


class ProviderNeutralityTests(unittest.TestCase):
    def test_reports_do_not_depend_on_legacy_recipe_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subprocess.run([sys.executable, "-B", str(ROOT / "scripts/knowledge_os.py"),
                            "init", "--dest", tmp, "--cell-name", "Test", "--purpose",
                            "Test", "--system", "test:Test", "--yes"],
                           check=True, capture_output=True)
            audit = load_module("neutral_audit", root / "90-Meta/audit-vault.py")
            operations = root / "60-Operacion"
            area = operations / "Delivery"
            area.mkdir(parents=True)
            (operations / "Operacion.md").write_text("# Operations\n")
            report = area / "Snapshot.md"
            report.write_text("---\ntipo: operacional\nclase: reporte\nreport-id: snapshot\n---\n")
            paths = [report]
            self.assertEqual(audit.audit_operational_topology(root, paths), [])
            # A consumer-owned recipe directory must not turn unrelated reports
            # into invalid notes or impose the retired engine's JSON schema.
            recipes = root / ".agents/skills/generate-reports/scripts/reports/custom"
            recipes.mkdir(parents=True)
            (recipes / "report.json").write_text('{"consumer_format": true}')
            self.assertEqual(audit.audit_operational_topology(root, paths), [])
            duplicate = area / "Duplicate.md"
            duplicate.write_text(report.read_text())
            self.assertTrue(any("duplicate report-id" in issue for issue in
                                audit.audit_operational_topology(root, paths + [duplicate])))

    def test_unknown_provider_uses_destination_procedure(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            instance = validate_instance({
                "cell": {"name": "Test", "purpose": "Test"},
                "systems": [{"id": "delivery", "name": "Delivery"}],
                "trackers": [{"id": "delivery", "provider": "custom-tracker",
                              "url": "https://tracker.example.org/team"}],
            })
            (root / "instance.yaml").write_text(dump_instance(instance))
            area = root / "60-Operacion/Delivery"
            area.mkdir(parents=True)
            (area / "Tracker access.md").write_text(
                "---\ntipo: operacional\nclase: procedimiento\n---\n"
                "Read-only executor for delivery. Preserve observed relationship direction.\n")
            config.bind(root, "work-item-evidence", ["Tracker access"])
            reread = load_instance(root / "instance.yaml")
            self.assertEqual(reread["trackers"], instance["trackers"])
            resolved = config.resolve(root, reread, "work-item-evidence")
            self.assertEqual(resolved["status"], "configured")
            self.assertEqual(len(resolved["procedures"]), 1)

    def test_retired_report_engine_preserves_local_recipe_and_note(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            command = [sys.executable, "-B", str(ROOT / "scripts/knowledge_os.py")]
            subprocess.run(command + ["init", "--dest", tmp, "--cell-name", "Test",
                           "--purpose", "Test", "--system", "test:Test", "--adapter",
                           "reports", "--yes"], check=True, capture_output=True)
            lock = installer.load_lock(root / installer.LOCK_NAME)
            retired_paths = [
                ".agents/skills/generate-reports/scripts/run_report.py",
                ".agents/skills/generate-reports/scripts/bigquery_adapter.py",
                "90-Meta/jira-evidence.md",
                ".agents/skills/manage-operational-workflow/scripts/prepare-jira-blocks-links.py",
            ]
            for relative in retired_paths:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("previous managed implementation")
                lock["managed_hashes"][relative] = installer.sha256_file(path)
            (root / installer.LOCK_NAME).write_text(installer.dump_lock(lock))
            recipe = root / ".agents/skills/generate-reports/scripts/reports/local/report.json"
            recipe.parent.mkdir(parents=True)
            recipe.write_text('{"owned_by": "consumer"}')
            note = root / "60-Operacion/Report access.md"
            note.write_text("consumer runbook")
            result = subprocess.run(command + ["update", "--dest", tmp],
                                    check=True, capture_output=True, text=True)
            self.assertEqual(set(json.loads(result.stdout)["retired_managed_files"]),
                             set(retired_paths))
            self.assertEqual(recipe.read_text(), '{"owned_by": "consumer"}')
            self.assertEqual(note.read_text(), "consumer runbook")
            self.assertTrue((root / ".agents/skills/generate-reports/SKILL.md").is_file())
            self.assertFalse((root / retired_paths[0]).exists())


if __name__ == "__main__":
    unittest.main()
