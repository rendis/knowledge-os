#!/usr/bin/env python3
"""Unit tests for instance.yaml dump/load and orientation."""
from __future__ import annotations

import sys
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from instance import (  # noqa: E402
    InstanceError,
    dump_instance,
    load_instance,
    orientation_status,
    validate_instance,
    reference_branches,
    valid_branch_name,
    _parse_minimal_yaml,
    _parse_simple_yaml,
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

    def test_special_strings_roundtrip_without_optional_yaml(self) -> None:
        data = self.sample()
        data["cell"]["purpose"] = 'Quotes "inside", hash # and slash \\; Unicode á'
        data["systems"][0]["aliases"] = ["one,two", 'a"b', "it's", "#tag", "[bracket]", ""]
        data["sources"]["repo_prefixes"] = ["prefix,with,commas"]
        data["capabilities"] = {"runtime-inspection": ['Procedure, "quoted"']}
        self.assertEqual(
            _parse_simple_yaml(dump_instance(data))["systems"][0]["aliases"],
            data["systems"][0]["aliases"],
        )
        data["sources"]["schema_repository"] = {"remote": "", "note": ""}
        data = validate_instance(data)
        text = dump_instance(data)
        with patch.dict(sys.modules, {"yaml": None}):
            self.assertEqual(validate_instance(_parse_simple_yaml(text)), data)
        # An installed YAML module must never be imported or consulted.
        with patch.dict(sys.modules, {"yaml": object()}):
            self.assertEqual(validate_instance(_parse_simple_yaml(text)), data)

    def test_comments_and_single_quoted_flow_lists(self) -> None:
        parsed = _parse_simple_yaml(
            "# comment with an unmatched quote ' ignored\n"
            "name: 'It''s # literal' # actual comment\n"
            'aliases: ["a,b", \'it\'\'s\', plain,] # comment\n'
            "url: https://example.test/a#fragment\n"
            "empty: {}\n"
            "enabled: true\n"
        )
        self.assertEqual(parsed, {
            "name": "It's # literal", "aliases": ["a,b", "it's", "plain"],
            "url": "https://example.test/a#fragment", "empty": {}, "enabled": True,
        })

    def test_plain_scalar_punctuation_and_comment_context(self) -> None:
        cases = {
            "Notes [2026]": "Notes [2026]",
            "Notes {draft}, [] and {}": "Notes {draft}, [] and {}",
            "customer 's workflows": "customer 's workflows",
            "customer's workflows": "customer's workflows",
            'Notes "quoted" and "unfinished': 'Notes "quoted" and "unfinished',
            'Notes, "quoted" # comment with unmatched quote "': 'Notes, "quoted"',
            'Notes ["unfinished # actual comment': 'Notes ["unfinished',
            "customer 's # actual comment": "customer 's",
            'Notes " # actual comment': 'Notes "',
            'Notes "#literal': 'Notes "#literal',
        }
        for raw, expected in cases.items():
            with self.subTest(raw=raw):
                self.assertEqual(_parse_simple_yaml(f"purpose: {raw}\n"), {"purpose": expected})
        self.assertEqual(
            _parse_simple_yaml('aliases: [customer\'s, customer \'s, "a,b", \'#literal\'] # comment\n'),
            {"aliases": ["customer's", "customer 's", "a,b", "#literal"]},
        )

    def test_rejects_malformed_or_unsupported_yaml(self) -> None:
        cases = (
            "cell: [", 'name: "unterminated', "name: 'unterminated",
            'name: "bad\\q"', 'name: "ok" junk', "name: 'ok' junk",
            "name missing colon", " name: root-indented", "name: value\n  extra: child",
            "name: one\nname: two", "cell:\n  name: one\n  name: two",
            "items:\n  - id: one\n    id: two", "cell:\n\tname: tab",
            "items: [one,,two]", "items: [one, [two]]", "cell: {name: X}",
            "name: &anchor value", "name: *anchor", "name: !!str value",
            "purpose: |\n  multiline", "purpose: >\n  folded", "---\nname: X",
            "items:\n  - one\n  name: mixed", "items:\n  - one\n    nested: invalid",
        )
        for text in cases:
            with self.subTest(text=text):
                with self.assertRaisesRegex(InstanceError, "invalid instance.yaml syntax"):
                    _parse_simple_yaml(text)

    def test_reference_branch_policy_roundtrip_and_legacy_defaults(self) -> None:
        data = self.sample()
        data["sources"]["reference_branches"] = {"payments-api": "release/stable", "123": "trunk", "quoted": 'release/"stable"'}
        for loaded in (validate_instance(_parse_minimal_yaml(dump_instance(data))),):
            self.assertEqual(loaded["sources"]["reference_branches"], data["sources"]["reference_branches"])
            self.assertEqual(reference_branches(loaded, "payments-api"), ("release/stable",))
            self.assertEqual(reference_branches(loaded, "123"), ("trunk",))
            self.assertEqual(reference_branches(loaded, "unlisted"), ("main", "master"))
        data["sources"]["reference_branch_order"] = ["develop", "main", "master"]
        loaded = validate_instance(_parse_minimal_yaml(dump_instance(data)))
        self.assertEqual(reference_branches(loaded, "unlisted"), ("develop", "main", "master"))
        self.assertEqual(reference_branches(loaded, "payments-api"), ("release/stable",))
        for bad in ([], ["main", "main"], ["refs/heads/main"]):
            data["sources"]["reference_branch_order"] = bad
            with self.assertRaises(InstanceError):
                validate_instance(data)
        data["sources"]["reference_branch_order"] = ["main", "master"]
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "instance.yaml"
            path.write_text(dump_instance(data))
            self.assertEqual(load_instance(path)["sources"]["reference_branches"], data["sources"]["reference_branches"])

    def test_reference_branch_policy_rejects_ref_expressions(self) -> None:
        for branch in ("", "HEAD", "refs/heads/trunk", "../main", "-option", "a..b", "a.lock", "a/.hidden", "a//b", "HEAD~1", "a b", "a\\b"):
            with self.subTest(branch=branch):
                self.assertFalse(valid_branch_name(branch))
                data = self.sample()
                data["sources"]["reference_branches"] = {"payments-api": branch}
                with self.assertRaises(InstanceError):
                    validate_instance(data)

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
        self.assertTrue(pending_inventory_listed(listed.replace("Pending inventory", "Inventario pendiente")))
        self.assertFalse(pending_inventory_listed(heading_only.replace("Pending inventory", "Inventario pendiente")))
        self.assertFalse(pending_inventory_listed(home.replace("Pending inventory", "Inventario pendiente")))


if __name__ == "__main__":
    unittest.main()
