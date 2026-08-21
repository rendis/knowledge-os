#!/usr/bin/env python3
"""Focused distribution checks for the installed synchronization helpers."""
from __future__ import annotations

import importlib.util
import subprocess
import sys
import unittest
from pathlib import Path


DIST = Path(__file__).resolve().parents[2]
META = DIST / "kernel" / "90-Meta"
MANIFEST = META / "git-change-manifest.py"
STATIC_SCAN = META / "static-evidence-scan.py"


def load_manifest_module():
    spec = importlib.util.spec_from_file_location(
        "distribution_git_change_manifest",
        MANIFEST,
    )
    if spec is None or spec.loader is None:
        raise RuntimeError("unable to load git-change-manifest.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class SyncToolingEval(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.module = load_manifest_module()

    def test_short_credential_literals_are_detected_and_redacted(self) -> None:
        for literal in ("x", "xy", "src"):
            with self.subTest(literal=literal):
                text = f"generated prose exposes {literal} as a credential"
                self.assertTrue(self.module.credential_literal_occurs(text, literal))
                self.assertFalse(
                    self.module.credential_literal_occurs(
                        self.module.redact_credential_literals(text, {literal}),
                        literal,
                    )
                )
                self.assertFalse(
                    self.module.credential_literal_occurs(
                        f"canonical{literal}identity",
                        literal,
                    )
                )

    def test_credential_surfaces_exclude_canonical_identity(self) -> None:
        analysis = {
            "repository": "credential-value",
            "old_oid": "a" * 40,
            "new_oid": "b" * 40,
            "paths": [{"path": "credential-value/file.py", "reason": "safe reason"}],
            "checklist": {},
            "claims": [],
            "nodes": [{"basename": "credential-value", "reason": "safe node reason"}],
            "blockers": [],
        }
        surface = self.module.analysis_credential_surface(analysis)
        self.assertNotIn("repository", surface)
        self.assertNotIn("basename", surface["nodes"][0])
        self.assertEqual(surface["path_reasons"], ["safe reason"])

        review = {
            "findings": [{
                "target": "credential-value",
                "category": "credential-value",
                "reason": "safe review reason",
                "nodes": ["credential-value"],
                "evidence": {"path": "credential-value/file.py", "anchor": "safe anchor"},
            }],
        }
        review_surface = self.module.review_credential_surface(review)
        finding = review_surface["findings"][0]
        self.assertEqual(set(finding), {"reason", "evidence_anchor"})

    def test_promoted_clis_expose_help(self) -> None:
        for script in (MANIFEST, STATIC_SCAN):
            with self.subTest(script=script.name):
                result = subprocess.run(
                    [sys.executable, "-B", str(script), "--help"],
                    cwd=str(DIST),
                    text=True,
                    capture_output=True,
                    check=False,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
