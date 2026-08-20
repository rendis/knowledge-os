#!/usr/bin/env python3
"""Unit tests for instance.yaml dump/load and orientation."""
from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "90-Meta"))

from instance import dump_instance, load_instance, orientation_status, validate_instance  # noqa: E402


class InstanceTests(unittest.TestCase):
    def sample(self) -> dict:
        return validate_instance(
            {
                "cell": {"name": "Payments", "purpose": "Checkout and settlement"},
                "systems": [{"id": "payments", "name": "Payments", "aliases": ["pay"]}],
                "evidence": {"profile": "production-gate"},
                "locale": {"notes": "en"},
                "adapters": [],
            }
        )

    def test_roundtrip(self) -> None:
        data = self.sample()
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "instance.yaml"
            path.write_text(dump_instance(data), encoding="utf-8")
            loaded = load_instance(path)
        self.assertEqual(loaded["cell"]["name"], "Payments")
        self.assertEqual(loaded["systems"][0]["id"], "payments")
        self.assertEqual(loaded["evidence"]["profile"], "production-gate")

    def test_rejects_empty_systems(self) -> None:
        with self.assertRaises(Exception):
            validate_instance({"cell": {"name": "X", "purpose": "Y"}, "systems": []})

    def test_orientation_incomplete(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            status = orientation_status(root)
        self.assertFalse(status["ready"])
        self.assertEqual(status["reason"], "complete-bootstrap")

    def test_pending_inventory_requires_heading_and_items(self) -> None:
        from instance import pending_inventory_listed

        home = "# Cell\n\nIf **Pending inventory** is listed below, wait.\n"
        self.assertFalse(pending_inventory_listed(home))
        heading_only = home + "\n## Pending inventory\n\n_No discovery roots were given._\n"
        self.assertFalse(pending_inventory_listed(heading_only))
        listed = home + "\n## Pending inventory\n\n- git@example.com:org/repo.git\n\n## Indexes\n"
        self.assertTrue(pending_inventory_listed(listed))


if __name__ == "__main__":
    unittest.main()
