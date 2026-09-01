#!/usr/bin/env python3
"""Unit tests for instance.yaml dump/load and orientation."""
from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "90-Meta"))

from instance import (  # noqa: E402
    InstanceError,
    dump_instance,
    load_instance,
    orientation_status,
    validate_instance,
)


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
        self.assertEqual(loaded["trackers"], [])

    def test_malformed_yaml_is_an_instance_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "instance.yaml"
            path.write_text("cell: [\n", encoding="utf-8")
            with self.assertRaisesRegex(
                InstanceError, "invalid instance.yaml syntax"
            ):
                load_instance(path)

    def test_tracker_roundtrip_supports_multiple_providers(self) -> None:
        data = self.sample()
        data["trackers"] = [
            {
                "id": "jira-core",
                "provider": "jira",
                "url": "https://core.atlassian.net/",
            },
            {
                "id": "clickup-product",
                "provider": "clickup",
                "url": "https://app.clickup.com/123456",
            },
        ]
        normalized = validate_instance(data)
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "instance.yaml"
            path.write_text(dump_instance(normalized), encoding="utf-8")
            loaded = load_instance(path)

        self.assertEqual(
            loaded["trackers"],
            [
                {
                    "id": "jira-core",
                    "provider": "jira",
                    "url": "https://core.atlassian.net",
                },
                {
                    "id": "clickup-product",
                    "provider": "clickup",
                    "url": "https://app.clickup.com/123456",
                },
            ],
        )

    def test_rejects_noncanonical_kebab_case_ids(self) -> None:
        base = self.sample()
        for field, value, message in (
            ("id", "core-", "tracker id must be kebab-case: core-"),
            (
                "provider",
                "jira--cloud",
                "tracker provider must be kebab-case: jira--cloud",
            ),
        ):
            with self.subTest(field=field, value=value):
                candidate = dict(base)
                tracker = {
                    "id": "core",
                    "provider": "jira",
                    "url": "https://core.invalid",
                }
                tracker[field] = value
                candidate["trackers"] = [tracker]
                with self.assertRaisesRegex(InstanceError, message):
                    validate_instance(candidate)

    def test_rejects_ambiguous_or_unsafe_trackers(self) -> None:
        base = self.sample()
        invalid_sets = (
            (
                [
                    {"id": "same", "provider": "jira", "url": "https://one.invalid"},
                    {"id": "same", "provider": "clickup", "url": "https://two.invalid"},
                ],
                "duplicate tracker id: same",
            ),
            (
                [
                    {"id": "one", "provider": "jira", "url": "https://same.invalid"},
                    {"id": "two", "provider": "jira", "url": "https://same.invalid/"},
                ],
                "duplicate tracker target: jira https://same.invalid",
            ),
            (
                [
                    {
                        "id": "credentialed",
                        "provider": "jira",
                        "url": "https://user:secret@example.invalid",
                    }
                ],
                "tracker URL must be a credential-free HTTPS URL",
            ),
            (
                [
                    {
                        "id": "bad id",
                        "provider": "jira",
                        "url": "https://one.invalid",
                    }
                ],
                "tracker id must be kebab-case: bad id",
            ),
        )
        for trackers, message in invalid_sets:
            with self.subTest(trackers=trackers):
                candidate = dict(base)
                candidate["trackers"] = trackers
                with self.assertRaisesRegex(InstanceError, message):
                    validate_instance(candidate)

    def test_rejects_empty_systems(self) -> None:
        with self.assertRaisesRegex(InstanceError, "systems must be a non-empty list"):
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
