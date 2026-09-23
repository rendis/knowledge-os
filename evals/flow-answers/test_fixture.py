"""Verify synthetic flow semantics and blind packaging, not agent wording."""

import importlib.util
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("flow_answer_prepare", HERE / "prepare.py")
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


class FixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.root = Path(cls.temp.name)
        cls.worker, cls.evaluator = cls.root / "worker", cls.root / "evaluator"
        cls.evaluation = fixture.prepare(HERE.parents[1], cls.worker, cls.evaluator)
        namespace = {}
        exec((cls.worker / "repos/exit-adapter/flow.py").read_text(), namespace)
        cls.flow = namespace

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_local_hit_skips_catalog_and_burn_records_only_trace(self):
        calls, traces = [], []

        def lookup(ean):
            calls.append(ean)
            return {"high_value": False}

        result = self.flow["validate_ean"]("111", True, {"111": {"high_value": True}}, lookup, traces)
        self.assertEqual(result, {"status": 201, "qr": "Q-111"})
        self.assertEqual(calls, [])
        self.assertEqual(traces, [("Q-111", "generated")])
        self.assertEqual(self.flow["burn"]("Q-111", traces), {"status": 200, "qr": "Q-111"})
        self.assertEqual(traces[-1], ("Q-111", "burned"))
        self.assertEqual(calls, [])

    def test_miss_rejection_and_missing_order_enforcement(self):
        calls, traces = [], []

        def lookup(ean):
            calls.append(ean)
            return {"high_value": False}

        self.assertEqual(
            self.flow["validate_ean"]("222", True, {}, lookup, traces),
            {"status": 422, "reason": "not_high_value"},
        )
        self.assertEqual(calls, ["222"])
        self.assertEqual(traces, [])
        self.assertEqual(
            self.flow["validate_ean"]("333", False, {}, lookup, traces),
            {"status": 422, "reason": "invalid_store"},
        )
        self.assertEqual(calls, ["222"])
        self.assertEqual(self.flow["burn"]("Q-unknown", traces)["status"], 200)
        self.assertEqual(traces, [("Q-unknown", "burned")])

    def test_evaluator_is_separate_and_prompts_are_staged(self):
        self.assertTrue((self.worker / "vault/AGENTS.md").is_file())
        for name in (
            "AGENTS.md",
            "90-Meta/response-quality.md",
            ".agents/skills/explain-visually/SKILL.md",
            ".agents/skills/map-ecosystem/references/interrogation.md",
        ):
            self.assertEqual(
                (self.worker / "vault" / name).read_bytes(),
                (HERE.parents[1] / "kernel" / name).read_bytes(),
            )
        self.assertEqual({p.name for p in self.worker.iterdir()}, {"vault", "repos", "sources", "task.txt"})
        task = (self.worker / "task.txt").read_text()
        self.assertIn(self.evaluation["turns"][0]["prompt"], task)
        for turn in self.evaluation["turns"][1:]:
            self.assertNotIn(turn["prompt"], task)
        self.assertFalse(list(self.worker.rglob("evaluation.json")))
        self.assertEqual(json.loads((self.evaluator / "evaluation.json").read_text()), self.evaluation)
        self.assertEqual(len(self.evaluation["repository_commit"]), 40)
        for name, digest in self.evaluation["source_sha256"].items():
            self.assertEqual(hashlib.sha256((self.worker / name).read_bytes()).hexdigest(), digest)

    def test_worker_material_does_not_contain_evaluator_cautions(self):
        prompts = " ".join(turn["prompt"] for turn in self.evaluation["turns"]).lower()
        for clue in ("condicional", "incertidumbre", "salida física", "422", "catalog"):
            self.assertNotIn(clue, prompts)
        note = (self.worker / "vault/30-Flujos/Exit pass.md").read_text().lower()
        for answer in ("only when the ean is absent", "not a physical exit", "does not mean every request"):
            self.assertNotIn(answer, note)

    def test_vault_graph_and_repository_binding(self):
        vault = self.worker / "vault"
        located = subprocess.run([
            "python3", "-B", str(vault / "90-Meta/workspace-config.py"),
            "--vault-root", str(vault), "locate-repository",
            "https://example.invalid/boreal/exit-adapter.git",
        ], check=True, capture_output=True, text=True)
        payload = json.loads(located.stdout)
        self.assertEqual(payload["status"], "ok")
        self.assertEqual(Path(payload["path"]).resolve(), (self.worker / "repos/exit-adapter").resolve())
        graph = subprocess.run([
            "python3", "-B", str(vault / "90-Meta/graph-query.py"),
            "--root", str(vault), "neighbors", "--node", "Exit pass",
        ], check=True, capture_output=True, text=True)
        self.assertEqual(json.loads(graph.stdout)["path"], "30-Flujos/Exit pass.md")

    def test_reject_overlap_and_reuse(self):
        for worker, evaluator in (
            (self.root / "x", self.root / "x/y"),
            (self.root / "x/y", self.root / "x"),
            (self.root / "x", self.root / "x"),
            (self.worker, self.root / "new"),
        ):
            with self.assertRaises(ValueError):
                fixture.prepare(HERE.parents[1], worker, evaluator)


if __name__ == "__main__":
    unittest.main()
