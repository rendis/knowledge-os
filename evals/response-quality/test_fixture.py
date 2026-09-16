"""Test source truth and blind packaging, never score model wording."""
import importlib.util
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("response_quality_prepare", HERE / "prepare.py")
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


class FixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.root = Path(cls.temp.name)
        cls.worker, cls.evaluator = cls.root / "worker", cls.root / "evaluator"
        cls.evaluation = fixture.prepare(HERE.parents[1], cls.worker, cls.evaluator)

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_actual_dispatch_and_uncalled_helper(self):
        namespace = {}
        exec((self.worker / "sources/service.py").read_text(), namespace)
        db = sqlite3.connect(":memory:")
        self.addCleanup(db.close)
        db.execute("CREATE TABLE stores (id TEXT, provider TEXT, reader_flag INTEGER, stock_enabled INTEGER)")
        rows = json.loads((self.worker / "sources/stores.json").read_text())["rows"]
        db.executemany("INSERT INTO stores VALUES (:id, :provider, :reader_flag, :stock_enabled)", rows)
        def fail_if_called(connection):
            raise AssertionError("Unregistered helper reached")
        namespace["get_reader_stores"] = fail_if_called
        self.assertEqual(namespace["dispatch"]("send-A", db), [("S101",)])
        self.assertEqual(namespace["dispatch"]("send-B", db), [("S202",)])
        db.execute("UPDATE stores SET reader_flag=0")
        self.assertEqual(namespace["dispatch"]("send-A", db), [("S101",)])
        self.assertEqual(namespace["dispatch"]("send-B", db), [("S202",)])
        db.execute("UPDATE stores SET stock_enabled=0 WHERE provider='B'")
        self.assertEqual(namespace["dispatch"]("send-B", db), [])
        self.assertEqual(namespace["dispatch"]("send-A", db), [("S101",)])

    def test_observation_scope_and_review_binding(self):
        runtime = json.loads((self.worker / "sources/runtime.json").read_text())
        self.assertEqual(runtime["status"], "completed")
        self.assertEqual(runtime["environment"], "lab")
        self.assertFalse(runtime["store_ids_logged"])
        self.assertFalse(runtime["delivery_receipts_available"])
        review = self.evaluation["prior_review_injection"]
        self.assertEqual(review["scope"]["environment"], runtime["environment"])
        self.assertEqual(review["scope"]["revision"], runtime["revision"])
        for name, sha in review["evidence_sha256"].items():
            self.assertEqual(fixture.digest(self.worker / name), sha)

    def test_blind_material_and_staged_turns(self):
        self.assertTrue((self.worker / "vault/AGENTS.md").is_file())
        self.assertFalse((self.worker / "sources/prior-review.json").exists())
        task = (self.worker / "task.txt").read_text()
        self.assertIn(self.evaluation["turns"][0]["prompt"], task)
        for turn in self.evaluation["turns"][1:]:
            self.assertNotIn(turn["prompt"], task)
        self.assertEqual({p.name for p in self.worker.iterdir()}, {"vault", "sources", "task.txt"})
        self.assertFalse(list(self.worker.rglob("rubric.json")))
        self.assertFalse(list(self.worker.rglob("evaluation.json")))
        self.assertEqual(json.loads((self.evaluator / "evaluation.json").read_text()), self.evaluation)

    def test_reject_overlap_and_existing_outputs(self):
        for worker, evaluator in ((self.root / "x", self.root / "x/y"),
                                  (self.root / "x/y", self.root / "x"),
                                  (self.root / "x", self.root / "x"),
                                  (self.worker, self.root / "new")):
            with self.assertRaises(ValueError):
                fixture.prepare(HERE.parents[1], worker, evaluator)


if __name__ == "__main__":
    unittest.main()
