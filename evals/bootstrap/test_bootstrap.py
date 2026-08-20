#!/usr/bin/env python3
"""Bootstrap evals: init, domain-leak scan, update safety, orientation."""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]
INSTALL = DIST / "install.sh"
FORBIDDEN = re.compile(
    r"iot|acme|APP90001|APP90002|cell-dbs|tagger|\bsateo\b|vendorx|"
    r"cell-monthly|proj-a|\bSOS\b",
    re.I,
)
SCAN_SUFFIXES = {".md", ".py", ".yaml", ".yml", ".sh", ".txt", ".json", ".sql", ".tmpl"}


def run(args: list[str], cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        args,
        cwd=str(cwd or DIST),
        text=True,
        capture_output=True,
        check=False,
    )


class BootstrapEval(unittest.TestCase):
    def test_init_creates_orientation(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "cell"
            result = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "Payments",
                    "--purpose",
                    "Card-present checkout",
                    "--system",
                    "payments:Payments",
                    "--system",
                    "ledger:Ledger",
                    "--yes",
                ]
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["status"], "initialized")
            home = (dest / "00-Home.md").read_text(encoding="utf-8")
            self.assertIn("Payments", home)
            self.assertIn("[[Ledger]]", home)
            self.assertTrue((dest / "10-Sistemas" / "Payments.md").is_file())
            self.assertTrue((dest / "instance.yaml").is_file())
            self.assertTrue((dest / "AGENTS.md").is_file())
            self.assertTrue((dest / ".agents" / "skills" / "map-ecosystem" / "SKILL.md").is_file())
            self.assertFalse((dest / ".agents" / "skills" / "inspect-gcp-runtime").exists())
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            info = json.loads(doctor.stdout)
            self.assertTrue(info["orientation"]["ready"])
            self.assertEqual(info["start_here"][0], "00-Home.md")

    def test_kernel_has_no_product_leak(self) -> None:
        leaks = []
        skip = {".git", "evals", "__pycache__"}
        for path in DIST.rglob("*"):
            if not path.is_file():
                continue
            if any(part in skip for part in path.parts):
                continue
            if path.suffix not in SCAN_SUFFIXES:
                continue
            text = path.read_text(encoding="utf-8", errors="ignore")
            if FORBIDDEN.search(text):
                leaks.append(str(path.relative_to(DIST)))
        self.assertEqual(leaks, [], f"product leaks: {leaks}")

    def test_update_does_not_overwrite_home(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "cell"
            run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "Platform",
                    "--purpose",
                    "Shared libraries",
                    "--system",
                    "platform:Platform",
                    "--disable-topics",
                    "--yes",
                ],
            )
            home = dest / "00-Home.md"
            original = home.read_text(encoding="utf-8")
            home.write_text(original + "\n\n## Cell note\nKeep me.\n", encoding="utf-8")
            instance = dest / "instance.yaml"
            instance_text = instance.read_text(encoding="utf-8")
            update = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(update.returncode, 0, update.stderr)
            self.assertIn("Keep me.", home.read_text(encoding="utf-8"))
            self.assertEqual(instance.read_text(encoding="utf-8"), instance_text)
            self.assertFalse((dest / "25-Topics").exists())

    def test_knowledge_without_lock_is_refused(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "existing"
            dest.mkdir()
            (dest / "00-Home.md").write_text("# existing\n", encoding="utf-8")
            result = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "X",
                    "--purpose",
                    "Y",
                    "--system",
                    "x:X",
                    "--yes",
                ]
            )
            self.assertNotEqual(result.returncode, 0)

    def test_adapter_opt_in(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "cell"
            result = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "Ops",
                    "--purpose",
                    "Runtime inspection",
                    "--system",
                    "runtime:Runtime",
                    "--adapter",
                    "gcp",
                    "--yes",
                ]
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((dest / ".agents" / "skills" / "inspect-gcp-runtime" / "SKILL.md").is_file())
            self.assertTrue((dest / ".agents" / "skills" / "gcloud" / "SKILL.md").is_file())


if __name__ == "__main__":
    unittest.main()
