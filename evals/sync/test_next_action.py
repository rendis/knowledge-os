#!/usr/bin/env python3
"""Next-action hints preserve CLI requirements and do not mutate run state."""
from __future__ import annotations

import copy
import importlib.util
import tempfile
import unittest
from pathlib import Path

PATH = Path(__file__).resolve().parents[2] / "kernel/90-Meta/sync-run.py"
SPEC = importlib.util.spec_from_file_location("sync_next_action", PATH)
assert SPEC and SPEC.loader
sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(sync)


class NextActionTests(unittest.TestCase):
    def setUp(self):
        self.run = {
            "run_id": "example", "packages": [], "gate_digest": "sealed",
            "gate_stale": False, "vault_locator": "", "units": [],
        }

    def unit(self, status, reviewed=False, kind="write-group"):
        return {
            "unit_id": "unit-1", "unit_type": kind, "status": status,
            "note_review_digest": "reviewed" if reviewed else "",
            "stale_reason": "", "receipt_digest": "",
        }

    def assert_hint(self, command, missing):
        before = copy.deepcopy(self.run)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / "absent-state"
            action = sync.next_action(root, self.run)
            self.assertFalse(root.exists())
            self.assertEqual(action["command"], command)
            self.assertEqual(set(action["missing_inputs"]), set(missing))
            self.assertEqual(sync.next_command(self.run), command)
            # Completing only the declared missing flags must satisfy the actual CLI.
            argv = action["argv"][:]
            for flag in missing:
                argv.extend([flag, "supplied-input"])
            parsed = sync.parser().parse_args(argv)
            self.assertEqual(parsed.command, command)
        self.assertEqual(before, self.run)
        return action

    def test_selects_pending_package_after_checkpointed(self):
        self.run["packages"] = [
            {"repository": "done", "oid": "a", "status": "checkpointed"},
            {"repository": "todo", "oid": "b", "status": "stale"},
        ]
        action = self.assert_hint("checkpoint-package", ["--artifact"])
        self.assertEqual((action["repository"], action["oid"]), ("todo", "b"))
        self.assertIn("todo", action["argv"])

    def test_gate_requires_input_even_with_old_gate(self):
        self.run["gate_stale"] = True
        self.assert_hint("seal-gate", ["--gate"])

    def test_projection_never_reuses_stale_artifacts(self):
        for status in ("pending", "stale", "projection-invalid"):
            self.run["units"] = [self.unit(status)]
            action = self.assert_hint("validate-unit", ["--projection", "--patch"])
            self.assertEqual(action["unit_id"], "unit-1")

    def test_review_selects_unreviewed_write_group(self):
        ack = self.unit("validated", kind="acknowledgement")
        ack["unit_id"] = "ack"
        self.run["units"] = [ack, self.unit("validated")]
        action = self.assert_hint("review-unit", ["--candidate", "--evidence-root", "--manifest", "--review", "--vault"])
        self.assertEqual(action["unit_id"], "unit-1")

    def test_apply_uses_known_vault(self):
        self.run["vault_locator"] = "../../vault"
        self.run["units"] = [self.unit("apply-failed", reviewed=True)]
        action = self.assert_hint("apply-unit", [])
        self.assertIn("--vault", action["argv"])

    def test_source_retraction_takes_priority_over_package(self):
        unit = self.unit("stale")
        unit.update(stale_reason="source", receipt_digest="receipt")
        self.run["units"] = [unit]
        self.run["packages"] = [{"repository": "todo", "oid": "b", "status": "stale"}]
        self.assert_hint("resume", [])

    def test_close(self):
        self.run["units"] = [self.unit("applied", reviewed=True)]
        self.assert_hint("close", [])


if __name__ == "__main__":
    unittest.main()
