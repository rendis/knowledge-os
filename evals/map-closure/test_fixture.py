"""Check scenario truth and isolation, not agent compliance."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("closure_prepare", HERE / "prepare.py")
prepare = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(prepare)


class ClosureFixture(unittest.TestCase):
    def test_source_relationship_and_stale_counts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = prepare.prepare(Path(temporary) / "case")
            publisher, consumer = {}, {}
            exec((root / "sources/dispatch/dispatch.py").read_text(), publisher)
            exec((root / "sources/ledger/ledger.py").read_text(), consumer)
            class Bus:
                def subscribe(self, topic, handler):
                    self.topic, self.handler = topic, handler
                def publish(self, topic, event):
                    if topic == self.topic:
                        self.handler(event)
            class Database:
                def insert(self, table, event):
                    self.written = (table, event)
            bus, database = Bus(), Database()
            consumer["register"](bus, database)
            publisher["publish_shipment"](bus, "S1")
            self.assertEqual(database.written, ("shipment_ledger", {"shipment_id": "S1"}))
            state = json.loads((root / "vault/checkpoint.json").read_text())
            self.assertEqual(state["accepted_local_maps"], 2)
            self.assertIn("1/2", (root / "vault/00-Home.md").read_text())
            self.assertIn("1/2", (root / "vault/90-Meta/Coverage.md").read_text())
            self.assertFalse(list(root.rglob("*oracle*")))
            for repo in (root / "sources").iterdir():
                result = subprocess.run(["git", "status", "--porcelain"], cwd=repo, check=True, capture_output=True, text=True)
                self.assertEqual(result.stdout, "")
            with self.assertRaises(ValueError):
                prepare.prepare(root)


if __name__ == "__main__":
    unittest.main()
