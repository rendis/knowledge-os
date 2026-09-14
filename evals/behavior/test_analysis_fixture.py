"""Check blind-fixture isolation and source behavior, not agent conclusions."""
from pathlib import Path
import hashlib
import importlib.util
import json
import subprocess
import tempfile
import unittest


DIST = Path(__file__).resolve().parents[2]


class AnalysisFixtureTests(unittest.TestCase):
    def prepare(self, root, evaluation):
        return subprocess.run([
            "python3", "-B", str(DIST / "evals/behavior/prepare.py"),
            "--distribution", str(DIST), "--output", str(root),
            "--scenario", "analysis", "--evaluation-output", str(evaluation),
        ], capture_output=True, text=True)

    def test_excludes_evaluator_and_loads_overlay(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            run, evaluation = base / "run", base / "evaluation"
            result = self.prepare(run, evaluation)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((run / "rubric.json").exists())
            self.assertEqual(len(list((run / "tasks").glob("*.txt"))), 14)
            hashes = json.loads((evaluation / "baseline-hashes.json").read_text())
            for name, digest in hashes.items():
                self.assertEqual(hashlib.sha256((run / name).read_bytes()).hexdigest(), digest)
            helper = run / "vault/.agents/skills/manage-investigation/scripts/investigation-case.py"
            loaded = subprocess.run(["python3", "-B", str(helper), "--root", str(run / "vault/investigations"), "load", "--id", "20260908-090000-reader-units"], capture_output=True, text=True, check=True)
            self.assertTrue(json.loads(loaded.stdout)["private"]["available"])
            results = {}
            for name in ("availability", "batch", "preview"):
                spec = importlib.util.spec_from_file_location(name, run / f"repos/reader/{name}.py")
                module = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(module)
                results[name] = module.preview(10, 1.5) if name == "preview" else module.available(10, 1500)
            self.assertEqual(results, {"availability": 8.5, "batch": 0, "preview": 8.5})
            resolved = subprocess.run(["python3", "-B", str(run / "vault/90-Meta/workspace-config.py"), "--vault-root", str(run / "vault"), "locate-repository", "https://example.invalid/cedar/reader.git"], capture_output=True, text=True, check=True)
            self.assertIn(str((run / "repos/reader").resolve()), resolved.stdout)
            procedure = subprocess.run(["python3", "-B", str(run / "vault/90-Meta/operational-catalog.py"), "resolve", "--basename", "Cedar - Offline processing audit"], capture_output=True, text=True, check=True)
            self.assertIn("Cedar - Offline processing audit.md", procedure.stdout)

    def test_rejects_evaluator_inside_run_before_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            run = Path(directory) / "run"
            result = self.prepare(run, run / "evaluation")
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(run.exists())

    def test_prompts_identical_across_fresh_runs(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            for name in ("a", "b"):
                result = self.prepare(base / name, base / f"eval-{name}")
                self.assertEqual(result.returncode, 0, result.stderr)
            for task in (base / "a/tasks").glob("*.txt"):
                self.assertEqual(task.read_bytes(), (base / "b/tasks" / task.name).read_bytes())


if __name__ == "__main__":
    unittest.main()
