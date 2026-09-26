#!/usr/bin/env python3
"""Execute the installed native runtime against disposable vaults.

Requires `make release`; uses real release artifacts, never mocked executables.
"""
from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(DIST / "scripts"))
import native_runtime


class NativeInstallationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.tmp.cleanup)
        cls.dist = Path(cls.tmp.name) / "distribution"
        cls.dist.mkdir()
        for name in ("scripts", "kernel", "adapters", "cmd", "internal"):
            shutil.copytree(DIST / name, cls.dist / name, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
        for name in ("install.sh", "VERSION", "MANAGED_PATHS", "instance.schema.yaml", "go.mod", "go.sum"):
            shutil.copy2(DIST / name, cls.dist / name)
        # All artifacts are checked by installer preflight, including non-host targets.
        shutil.copytree(DIST / "dist", cls.dist / "dist", ignore=shutil.ignore_patterns("tests", "kos", "*.test"))
        native_runtime.release(cls.dist)  # Fail with actionable build error, never skip coverage.

    def setUp(self):
        self.work = tempfile.TemporaryDirectory()
        self.addCleanup(self.work.cleanup)
        self.vault = Path(self.work.name) / "vault"
        self.install("init", "--cell-name", "Native test", "--purpose", "Disposable native installation", "--system", "sample:Sample", "--yes")

    def install(self, verb, *args, ok=True):
        p = subprocess.run([sys.executable, "-B", str(self.dist / "scripts/knowledge_os.py"), verb, "--dest", str(self.vault), *args], capture_output=True, text=True)
        if ok:
            self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        return p

    def cli(self, *args, ok=True):
        p = subprocess.run([str(self.vault / native_runtime.local_paths(native_runtime.target())[0]), *args], capture_output=True, text=True)
        if ok:
            self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        return p

    def test_installed_native_payload_and_config(self):
        self.assertEqual(list(self.vault.rglob("*.py")), [])
        self.assertEqual(list(self.vault.rglob("*.sh")), [])
        self.assertTrue((self.vault / ".agents/skills/explain-visually/scripts/check_text_fit.cjs").is_file())
        result = json.loads(self.cli("config", "resolve", "--vault", str(self.vault)).stdout)
        self.assertEqual(result["status"], "resolved")
        self.assertEqual(result["vault_root"], str(self.vault.resolve()))
        for args in (("audit",), ("check", "links"), ("check", "bases")):
            self.cli(*args, "--vault", str(self.vault))
        doctor = json.loads(self.install("doctor").stdout)
        self.assertEqual(doctor["native_runtime"]["status"], "ready")
        self.assertEqual(doctor["lock_version"], "4")
        self.assertNotIn('".bin/', (self.vault / ".knowledge-os.lock.yaml").read_text())

    def test_search_refresh_new_edit_rename_delete(self):
        note = self.vault / "50-Glosario" / "Lookup.md"
        note.write_text("# Lookup\n\n## Example\n\nUniqueindexexample alpha.\n")
        def cards():
            return json.loads(self.cli("search", "--vault", str(self.vault), "--query", "Uniqueindexexample").stdout)["cards"]
        found = cards()
        self.assertTrue(any(c["path"] == "50-Glosario/Lookup.md" for c in found))
        self.assertTrue(all("origin" in c and "section" in c and "excerpt" in c for c in found))
        note.write_text("# Lookup\n\n## New section\n\nUniqueindexexample beta.\n")
        self.assertTrue(any("beta" in c["excerpt"] for c in cards()))
        renamed = note.with_name("Renamed.md")
        note.rename(renamed)
        self.assertTrue(any(c["path"] == "50-Glosario/Renamed.md" for c in cards()))
        self.assertFalse(any(c["path"] == "50-Glosario/Lookup.md" for c in cards()))
        renamed.unlink()
        self.assertEqual(cards(), [])

    def test_binary_drift_blocks_update_and_missing_binary_recovers(self):
        binary = self.vault / native_runtime.local_paths(native_runtime.target())[0]
        binary.write_bytes(b"user-modified")
        before = binary.read_bytes()
        result = self.install("update", ok=False)
        self.assertEqual(result.returncode, 3, result.stdout + result.stderr)
        self.assertEqual(binary.read_bytes(), before)
        doctor = json.loads(self.install("doctor").stdout)
        self.assertEqual(doctor["native_runtime"]["status"], "drift")
        binary.unlink()
        self.install("update")
        self.cli("version")

    def test_retirement_preserves_local_changes_then_removes_unchanged(self):
        # Recreate a previous portable lock's tracked runtime entry.
        sys.path.insert(0, str(DIST / "scripts"))
        import knowledge_os
        old = self.vault / "90-Meta/removed-helper.py"  # a file an earlier release managed
        retired = b"# helper no longer shipped\n"
        old.write_bytes(retired)
        lockpath = self.vault / ".knowledge-os.lock.yaml"
        lock = knowledge_os.load_lock(lockpath)
        lock["managed_hashes"]["90-Meta/removed-helper.py"] = native_runtime.sha256(old)
        lockpath.write_text(knowledge_os.dump_lock(lock))
        old.write_text("# local change\n")
        result = self.install("update", ok=False)
        self.assertEqual(result.returncode, 3, result.stdout + result.stderr)
        self.assertEqual(old.read_text(), "# local change\n")
        old.write_bytes(retired)
        local = self.vault / "90-Meta/cell-owned.py"
        local.write_text("# keep consumer tool\n")
        self.install("update")
        self.assertFalse(old.exists())
        self.assertEqual(local.read_text(), "# keep consumer tool\n")


if __name__ == "__main__":
    unittest.main()
