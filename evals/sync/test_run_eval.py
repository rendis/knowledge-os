#!/usr/bin/env python3
"""Behavior-facing checks for the deterministic evaluation harness."""
from __future__ import annotations

import importlib.util
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock


SCRIPT = Path(__file__).with_name("run-eval.py")


def load_module():
    spec = importlib.util.spec_from_file_location("sync_run_eval", SCRIPT)
    if spec is None or spec.loader is None:
        raise RuntimeError("unable to load run-eval.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def git(root: Path, *args: str) -> None:
    result = subprocess.run(
        ("git", *args), cwd=root, text=True, capture_output=True, check=False,
    )
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)


class RunEvalTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.module = load_module()

    def review(self, role: str, input_digest: str) -> dict:
        return {
            "role": role,
            "agent_id": "agent-eval",
            "model": "gpt-test",
            "reasoning_effort": "high",
            "fork_turns": "none",
            "input_digest": input_digest,
            "findings": [],
        }

    def test_review_contract_accepts_empty_findings_and_rejects_wrong_binding(self) -> None:
        digest = "a" * 64
        review = self.review("functional-review", digest)
        self.assertEqual(
            self.module.validate_review(review, "functional-review", digest), []
        )
        review["input_digest"] = "b" * 64
        self.assertIn(
            "review-input-digest-invalid",
            self.module.validate_review(review, "functional-review", digest),
        )

    def test_prepare_reuses_unchanged_tree_without_reexecuting_tests(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "repo"
            eval_root = Path(temp) / "eval"
            (root / "evals" / "sync").mkdir(parents=True)
            (root / "evals" / "sync" / "criteria.md").write_text(
                "# Criteria\n", encoding="utf-8"
            )
            (root / "candidate.txt").write_text("one\n", encoding="utf-8")
            git(root, "init", "-b", "main")
            git(root, "add", ".")
            git(
                root, "-c", "user.name=Eval", "-c",
                "user.email=eval@example.invalid", "commit", "-m", "base",
            )
            args = SimpleNamespace(repo_root=root, eval_root=eval_root)
            receipt = {
                "version": 1,
                "status": "pass",
                "tests": [{
                    "name": "fixture", "command": ["true"], "returncode": 0,
                    "status": "pass", "test_count": 1,
                }],
            }
            with mock.patch.object(
                self.module, "run_tests", return_value=(receipt, {"fixture": "OK\n"})
            ) as executed:
                first = self.module.prepare(args)
                second = self.module.prepare(args)
            self.assertFalse(first["reused"])
            self.assertTrue(second["reused"])
            self.assertEqual(first["tree_digest"], second["tree_digest"])
            self.assertEqual(executed.call_count, 1)

    def test_changed_tree_advances_round(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "repo"
            eval_root = Path(temp) / "eval"
            (root / "evals" / "sync").mkdir(parents=True)
            (root / "evals" / "sync" / "criteria.md").write_text(
                "# Criteria\n", encoding="utf-8"
            )
            candidate = root / "candidate.txt"
            candidate.write_text("one\n", encoding="utf-8")
            git(root, "init", "-b", "main")
            git(root, "add", ".")
            git(
                root, "-c", "user.name=Eval", "-c",
                "user.email=eval@example.invalid", "commit", "-m", "base",
            )
            args = SimpleNamespace(repo_root=root, eval_root=eval_root)
            receipt = {"version": 1, "status": "pass", "tests": []}
            with mock.patch.object(
                self.module, "run_tests", return_value=(receipt, {})
            ):
                first = self.module.prepare(args)
                candidate.write_text("two\n", encoding="utf-8")
                second = self.module.prepare(args)
            self.assertEqual(first["round"], 0)
            self.assertEqual(second["round"], 1)
            self.assertNotEqual(first["tree_digest"], second["tree_digest"])

    def test_finalize_distinguishes_review_completion_from_approval(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            digest = "a" * 64
            bundle = {
                "round": 0,
                "candidate_tree_digest": "b" * 64,
                "criteria_digest": "c" * 64,
                "test_receipt_digest": "d" * 64,
            }
            review_input = {"input_digest": digest}
            (root / "bundle.json").write_text(json.dumps(bundle), encoding="utf-8")
            (root / "review-input.json").write_text(
                json.dumps(review_input), encoding="utf-8"
            )
            functional = self.review("functional-review", digest)
            functional["findings"] = [{
                "finding_id": "FUNC-001",
                "severity": "P1",
                "requirement": "REQ-001",
                "file": "kernel/example.py",
                "line": 1,
                "claim": "A relevant contract is broken.",
                "evidence": "The public behavior reproduces the mismatch.",
                "disposition": "reported",
            }]
            recovery = self.review("recovery-review", digest)
            functional_path, recovery_path = root / "functional.json", root / "recovery.json"
            functional_path.write_text(json.dumps(functional), encoding="utf-8")
            recovery_path.write_text(json.dumps(recovery), encoding="utf-8")
            result = self.module.finalize(SimpleNamespace(
                bundle_root=root,
                functional_review=functional_path,
                recovery_review=recovery_path,
            ))
            self.assertEqual(result["status"], "findings")
            self.assertEqual(result["finding_count"], 1)


if __name__ == "__main__":
    unittest.main()
