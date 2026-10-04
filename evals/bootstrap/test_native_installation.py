#!/usr/bin/env python3
"""kos as a per-machine tool: the release, its installer, self-update and a vault without binaries.

Requires `make release`; uses the real release artifacts, served from a local mirror. Per ADR 0002 a
cell is created, adopted and updated by `kos` alone; there is no separate checkout installer any more.
"""
from __future__ import annotations

import functools
import hashlib
import http.server
import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _kos  # noqa: E402


def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


class Mirror:
    """A directory served over HTTP, standing in for a GitHub release."""

    def __init__(self, root: Path):
        handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(root))
        handler.log_message = lambda *args: None
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"

    def close(self):
        try:
            self.server.shutdown()
        finally:
            self.server.server_close()


class ReleaseTests(unittest.TestCase):
    def test_release_lists_every_platform_with_checksums(self):
        value = _kos.release(DIST)
        self.assertEqual(set(value["artifacts"]), _kos.TARGETS)
        sums = dict(line.split()[::-1] for line in (DIST / "dist/SHA256SUMS").read_text().splitlines())
        for item in value["artifacts"].values():
            self.assertEqual(sums[item["file"]], sha(DIST / "dist" / item["file"]))
        for name in ("install-kos.sh", "install-kos.ps1", "VERSION"):
            self.assertTrue((DIST / "dist" / name).is_file(), name)

    def test_stale_or_tampered_release_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            copy = Path(tmp) / "distribution"
            for name in ("kernel", "adapters", "cmd", "internal"):
                shutil.copytree(DIST / name, copy / name, ignore=shutil.ignore_patterns("__pycache__"))
            for name in ("go.mod", "go.sum", "payload.go", "MANAGED_PATHS"):
                shutil.copy2(DIST / name, copy / name)
            shutil.copytree(DIST / "dist", copy / "dist", ignore=shutil.ignore_patterns("tests", "windows-*"))
            _kos.binary(copy)
            (copy / "kernel/AGENTS.md").write_text("changed kernel\n")
            with self.assertRaisesRegex(RuntimeError, "stale"):
                _kos.binary(copy)
            shutil.copy2(DIST / "kernel/AGENTS.md", copy / "kernel/AGENTS.md")
            host = copy / "dist" / _kos.release(copy)["artifacts"][_kos.target()]["file"]
            host.write_bytes(b"tampered")
            with self.assertRaisesRegex(RuntimeError, "missing or changed"):
                _kos.binary(copy)


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.mirror_dir = self.root / "mirror"
        shutil.copytree(DIST / "dist", self.mirror_dir, ignore=shutil.ignore_patterns("tests", "windows-*"))
        self.mirror = Mirror(self.mirror_dir)
        self.addCleanup(self.mirror.close)
        self.bin = self.root / "bin"
        self.env = {**os.environ, "KOS_DOWNLOAD_URL": self.mirror.url, "KOS_INSTALL_DIR": str(self.bin),
                    "HOME": str(self.root), "XDG_CACHE_HOME": str(self.root / "cache"), "CI": ""}
        if os.name == "nt":
            self.env.update(USERPROFILE=str(self.root), LOCALAPPDATA=str(self.root / "cache"),
                            APPDATA=str(self.root / "config"))
        self.installed_binary = self.bin / ("kos.exe" if os.name == "nt" else "kos")
        self.env.pop("KOS_NO_UPDATE_CHECK", None)

    def install(self) -> subprocess.CompletedProcess:
        if os.name == "nt":
            command = ["pwsh", "-NoProfile", "-File", str(DIST / "scripts/install-kos.ps1")]
        else:
            command = ["sh", str(DIST / "scripts/install-kos.sh")]
        return subprocess.run(command, env=self.env, capture_output=True, text=True, timeout=60)

    def kos(self, *args: str, cwd: Path | None = None) -> subprocess.CompletedProcess:
        return subprocess.run([str(self.installed_binary), *args], env=self.env, capture_output=True, text=True, cwd=cwd, timeout=60)

    def test_install_verify_and_update_in_place(self):
        installed = self.install()
        self.assertEqual(installed.returncode, 0, installed.stderr)
        version = json.loads(self.kos("version").stdout)
        self.assertEqual(version["kos"], (DIST / "kernel/VERSION").read_text().strip())
        self.assertNotIn("latest", version)

        (self.mirror_dir / "VERSION").write_text("99.0.0\n")
        for cache in (self.root / "cache", self.root / "Library" / "Caches"):  # the daily check is cached
            shutil.rmtree(cache, ignore_errors=True)
        noticed = self.kos("config", "--help")
        self.assertIn("kos 99.0.0 is available", noticed.stderr)
        updated = self.kos("update")
        self.assertEqual(updated.returncode, 0, updated.stderr)
        self.assertEqual(json.loads(updated.stdout)["to"], "99.0.0")

        sums = self.mirror_dir / "SHA256SUMS"
        sums.write_text(sums.read_text().replace(sha(self.mirror_dir / _kos.filename(_kos.target())), "0" * 64))
        refused = self.install()
        self.assertNotEqual(refused.returncode, 0)
        self.assertIn("checksum mismatch", refused.stderr)

    def test_vault_without_binaries_is_operated_from_its_directory(self):
        self.assertEqual(self.install().returncode, 0)
        vault = self.root / "vault"
        init = subprocess.run([str(self.installed_binary), "init", "--vault", str(vault), "--yes", "--cell-name", "C",
                               "--purpose", "P", "--system", "Orders"], capture_output=True, text=True, timeout=120,
                              env={**self.env, "KOS_NO_UPDATE_CHECK": "1"})
        self.assertEqual(init.returncode, 0, init.stderr)
        self.assertFalse((vault / ".agents" / "bin").exists())
        self.assertEqual(list(vault.rglob("*.py")), [])
        env = {**self.env, "KOS_NO_UPDATE_CHECK": "1"}
        status = subprocess.run([str(self.installed_binary), "kernel", "status"], env=env, cwd=vault / "10-Sistemas",
                                capture_output=True, text=True, timeout=60)
        self.assertEqual(status.returncode, 0, status.stderr)
        self.assertTrue(json.loads(status.stdout)["current"])
        audit = subprocess.run([str(self.installed_binary), "audit"], env=env, cwd=vault, capture_output=True, text=True, timeout=60)
        self.assertEqual(audit.returncode, 0, audit.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
