"""Exercise real fixture preparation while replacing only the Codex process."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import run_analysis


DIST = Path(__file__).resolve().parents[2]
REAL_RUN = subprocess.run


class AnalysisRunnerTests(unittest.TestCase):
    def execute(self, base, task, variant="original", fail=False):
        args = argparse.Namespace(distribution=DIST, output=base / "run",
                                  evaluation_output=base / "evaluator", task=task,
                                  model="synthetic-test-model", effort="low", variant=variant)
        calls = []

        def fake_codex(command, **kwargs):
            if command[0] != "codex":
                return REAL_RUN(command, **kwargs)
            if task == "diagnosis":
                # Assert isolation at launch, not only after the executor returns.
                for relative in ("sources/review-note.md", "sources/accepted-agreement.md",
                                 "tasks", "task.txt", "package.md", "worktree"):
                    self.assertFalse((Path(kwargs["cwd"]) / relative).exists(), relative)
            calls.append((command, kwargs["input"]))
            kwargs["stdout"].write(json.dumps({"type": "thread.started", "thread_id": "synthetic-session-17"}) + "\n")
            Path(command[command.index("--output-last-message") + 1]).write_text("Synthetic process result.\n")
            if fail:
                (Path(kwargs["cwd"]) / "vault/unauthorized-note.md").write_text("Unexpected write in a read-only task.\n")
            return subprocess.CompletedProcess(command, 7 if fail else 0)

        with patch.object(run_analysis.subprocess, "run", side_effect=fake_codex):
            if fail:
                with self.assertRaises(SystemExit) as caught:
                    run_analysis.run(args)
                self.assertEqual(caught.exception.code, 7)
            else:
                run_analysis.run(args)
        return args, calls

    def test_independent_ephemeral_and_no_diagnosis_answer_leak(self):
        with tempfile.TemporaryDirectory() as temporary:
            args, calls = self.execute(Path(temporary), "diagnosis")
            self.assertIn("--ephemeral", calls[0][0])
            self.assertNotIn("resume", calls[0][0])
            self.assertFalse((args.output / "sources/review-note.md").exists())
            self.assertFalse((args.output / "sources/accepted-agreement.md").exists())
            self.assertFalse((args.output / "package.md").exists())
            self.assertFalse((args.output / "worktree").exists())
            self.assertFalse((args.output / "tasks").exists())
            self.assertFalse((args.output / "task.txt").exists())
            self.assertEqual(list(args.output.rglob("*-events.jsonl")), [])
            self.assertFalse((args.output / "observed.json").exists())
            self.assertTrue((args.evaluation_output / "diagnosis-events.jsonl").is_file())
            self.assertTrue((args.evaluation_output / "diagnosis-result.md").is_file())
            observed = json.loads((args.evaluation_output / "observed.json").read_text())
            self.assertEqual(observed[0]["changed_vault_paths"], [])

    def test_continuity_resumes_returned_thread_with_only_new_prompt(self):
        with tempfile.TemporaryDirectory() as temporary:
            args, calls = self.execute(Path(temporary), "continuity")
            self.assertEqual(len(calls), 3)
            observed = json.loads((args.evaluation_output / "observed.json").read_text())
            for index, (command, prompt) in enumerate(calls, 1):
                self.assertNotIn("--ephemeral", command)
                self.assertEqual(hashlib.sha256(prompt.encode()).hexdigest(), observed[index - 1]["prompt_sha256"])
                self.assertNotIn('Synthetic process result', prompt)
                self.assertFalse((args.output / 'tasks').exists())
                if index == 1:
                    self.assertNotIn("resume", command)
                else:
                    self.assertEqual(command[-3:], ["resume", "synthetic-session-17", "-"])

    def test_alternate_changes_inputs_and_matching_incident(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            original, _ = self.execute(base / "original", "diagnosis")
            alternate, _ = self.execute(base / "alternate", "diagnosis", "alternate")
            first = json.loads((original.output / "sources/observations.json").read_text())
            second = json.loads((alternate.output / "sources/observations.json").read_text())
            self.assertNotEqual(first["request"], second["request"])
            self.assertEqual(second["request"], {"physical": 12, "reserved_milliunits": 2500})
            self.assertEqual(second["interactive_result"], 9.5)
            self.assertIn("9.5", (alternate.output / "sources/incident.md").read_text())
            self.assertIn("2500 milliunits", (alternate.evaluation_output / "rubric.json").read_text())

    def test_failed_process_preserves_observed_read_only_side_effect(self):
        with tempfile.TemporaryDirectory() as temporary:
            args, calls = self.execute(Path(temporary), "simple", fail=True)
            self.assertEqual(len(calls), 1)
            observed = json.loads((args.evaluation_output / "observed.json").read_text())
            self.assertEqual(observed[0]["exit_code"], 7)
            self.assertIn("unauthorized-note.md", observed[0]["changed_vault_paths"])


if __name__ == "__main__":
    unittest.main()
