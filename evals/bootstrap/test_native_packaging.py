#!/usr/bin/env python3
"""Native release packaging contracts; fake executable bytes are never executed."""
from __future__ import annotations

import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

DIST = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(DIST / "scripts"))
import native_runtime as runtime
spec = importlib.util.spec_from_file_location("native_packaging_installer", DIST / "scripts/knowledge_os.py")
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class NativePackagingTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dist = Path(self.tmp.name) / "distribution"
        self.dest = Path(self.tmp.name) / "vault"
        (self.dist / "dist").mkdir(parents=True)
        (self.dist / "VERSION").write_text("1.2.3\n")
        for directory in ("cmd", "internal"):
            (self.dist / directory).mkdir()
        (self.dist / "go.mod").write_text("module fixture\n")
        (self.dist / "go.sum").write_text("")
        (self.dist / "cmd/main.go").write_text("package main\n")
        self.manifest = {"schema": 1, "version": "1.2.3", "source_revision": "test-revision", "source_dirty": True, "artifacts": {}, "notices_sha256": "", "source_fingerprint": runtime.source_fingerprint(self.dist)}
        for target in runtime.TARGETS:
            name = runtime.filename(target)
            p = self.dist / "dist" / name
            p.write_bytes(("test-only artifact " + target).encode())
            self.manifest["artifacts"][target] = {"file": name, "sha256": runtime.sha256(p)}
        notices = self.dist / "dist/THIRD_PARTY_NOTICES.txt"
        notices.write_text("test-only redistribution notice")
        self.manifest["notices_sha256"] = runtime.sha256(notices)
        self.write_manifest()

    def write_manifest(self):
        (self.dist / "dist/runtime-manifest.json").write_text(json.dumps(self.manifest))

    def test_platform_selection_and_portable_lock(self):
        current = runtime.release(self.dist)
        locks = []
        for selected in sorted(runtime.TARGETS):
            vault = self.dest / selected.replace("/", "-")
            with mock.patch.object(runtime, "target", return_value=selected):
                runtime.install(self.dist, vault, current)
                status = runtime.status(vault, current, current)
                self.assertEqual(status["status"], "ready")
                self.assertTrue(status["release_matches_dist"])
                self.assertEqual(set(p.name for p in (vault / ".agents/bin").iterdir()), {runtime.filename(t) for t in runtime.TARGETS} | {"runtime-manifest.json", "THIRD_PARTY_NOTICES.txt"})
                self.assertEqual((vault / status["path"]).read_bytes(), (self.dist / "dist" / runtime.filename(selected)).read_bytes())
            payload = {"version": "4", "kernel_version": "1.2.3", "distribution_revision": "test", "distribution_dirty": True, "adapters": [], "managed_hashes": {"AGENTS.md": "ab"}, "runtime_release": current}
            serialized = installer.dump_lock(payload)
            path = vault / ".knowledge-os.lock.yaml"
            path.write_text(serialized)
            self.assertEqual(installer.load_lock(path)["runtime_release"], current)
            self.assertNotIn(".bin/", serialized)
            locks.append(serialized)
        self.assertEqual(len(set(locks)), 1)

    def test_reject_missing_target_and_tampered_release(self):
        del self.manifest["artifacts"]["windows/amd64"]
        self.write_manifest()
        with self.assertRaisesRegex(RuntimeError, "six supported"):
            runtime.release(self.dist)
        self.assertFalse(self.dest.exists())

    def test_tampered_source_and_installed_artifact(self):
        current = runtime.release(self.dist)
        runtime.install(self.dist, self.dest, current)
        local = self.dest / runtime.local_paths(runtime.target())[0]
        local.write_bytes(b"modified user binary")
        self.assertEqual(runtime.status(self.dest, current, current)["status"], "drift")
        self.assertIn(local.relative_to(self.dest).as_posix(), runtime.conflicts(self.dest, current, current))
        (self.dist / "dist" / runtime.filename("linux/amd64")).write_bytes(b"modified release")
        with self.assertRaisesRegex(RuntimeError, "missing or changed"):
            runtime.release(self.dist)

    def test_symlink_safety(self):
        current = runtime.release(self.dist)
        self.dest.mkdir()
        outside = self.dest.parent / "outside"
        outside.mkdir()
        (self.dest / ".agents").symlink_to(outside, target_is_directory=True)
        with self.assertRaisesRegex(RuntimeError, "unsafe"):
            runtime.install(self.dist, self.dest, current)
        self.assertEqual(list(outside.iterdir()), [])

    def test_same_version_modified_source_invalidates_release(self):
        runtime.release(self.dist)
        (self.dist / "cmd/main.go").write_text("package main\n// changed build input\n")
        with self.assertRaisesRegex(RuntimeError, "stale for current source"):
            runtime.release(self.dist)

    def test_non_host_drift(self):
        current = runtime.release(self.dist)
        self.assertEqual(runtime.conflicts(self.dest, current, current), [])
        runtime.install(self.dist, self.dest, current)
        other = self.dest / ".agents/bin/vaultctl-windows-amd64.exe"
        other.write_bytes(b"corrupt non-host artifact")
        self.assertIn(other.relative_to(self.dest).as_posix(), runtime.conflicts(self.dest, current, current))
        self.assertEqual(runtime.status(self.dest, current, current)["status"], "drift")

    def test_bundle_survives_git_clone(self):
        import subprocess
        current = runtime.release(self.dist)
        runtime.install(self.dist, self.dest, current)
        (self.dest / ".gitignore").write_text("*.exe\n")
        with mock.patch.object(installer, "DIST", self.dist):
            installer.ensure_gitignore_lines(self.dest)
        def git(*args):
            subprocess.run(["git", "-C", str(self.dest), *args], check=True, capture_output=True)
        git("init")
        git("add", ".")
        git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "test: bundle")
        clone = self.dest.parent / "clone"
        subprocess.run(["git", "clone", str(self.dest), str(clone)], check=True, capture_output=True)
        for selected in runtime.TARGETS:
            with mock.patch.object(runtime, "target", return_value=selected):
                self.assertEqual(runtime.status(clone, current, current)["status"], "ready")

    def test_ignore_and_version_mismatch(self):
        self.dest.mkdir()
        with mock.patch.object(installer, "DIST", self.dist):
            installer.ensure_gitignore_lines(self.dest)
        self.assertIn("!/.agents/bin/vaultctl-windows-amd64.exe", (self.dest / ".gitignore").read_text())
        self.manifest["version"] = "0.0.0"
        self.write_manifest()
        with self.assertRaisesRegex(RuntimeError, "version differs"):
            runtime.release(self.dist)


if __name__ == "__main__":
    unittest.main()
