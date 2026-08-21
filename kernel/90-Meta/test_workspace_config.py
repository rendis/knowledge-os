#!/usr/bin/env python3
"""Tests for workspace-config migrate, update merge, and proxy ports."""
from __future__ import annotations

import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent


def load_module():
    path = HERE / "workspace-config.py"
    spec = importlib.util.spec_from_file_location("workspace_config", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class WorkspaceConfigTests(unittest.TestCase):
    def test_locate_repository_uses_configured_remote_before_url_rewrite(self) -> None:
        wc = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repositories = root / "repositories"
            repository = repositories / "example"
            repository.mkdir(parents=True)
            subprocess.run(["git", "init", "-q", str(repository)], check=True)
            remote = "https://example.test/acme/APP00001-example.git"
            subprocess.run(
                ["git", "-C", str(repository), "remote", "add", "origin", remote],
                check=True,
            )
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(repository),
                    "config",
                    "url.file:///tmp/rewrite.git.insteadOf",
                    remote,
                ],
                check=True,
            )
            wc.apply_config(root, roots=[str(repositories)], replace=True)

            located = wc.locate_repository(root, remote)

            self.assertEqual(located["status"], "ok")
            self.assertEqual(located["path"], str(repository.resolve()))
            self.assertEqual(located["remote"], "example.test/acme/app00001-example")

    def test_update_keeps_ports(self) -> None:
        wc = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            wc.apply_config(
                root,
                roots=["/tmp/repos"],
                managed_root="/tmp/repos",
                worktree_root="/tmp/trees",
                proxy_ports={"uat": "5434"},
                replace=True,
            )
            wc.apply_config(root, proxy_ports={"prod": "5436"})
            status = wc.load_config(root)
            self.assertEqual(status["proxy_ports"]["uat"], "5434")
            self.assertEqual(status["proxy_ports"]["prod"], "5436")
            self.assertEqual(status["worktree_root"], "/tmp/trees")

    def test_migrate_unquoted_legacy_paths(self) -> None:
        wc = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".old-workspace-config.yaml").write_text(
                "\n".join(
                    [
                        "version: 1",
                        "workspace:",
                        "  managed_clone:",
                        "    enabled: true",
                        "    root: /opt/src",
                        "  repository_roots:",
                        "    - /opt/src",
                        "skills:",
                        "  inspect-database:",
                        "    environments:",
                        "      uat:",
                        "        proxy_port: 5434",
                        "  manage-development-handoff:",
                        "    worktree_root: /opt/trees",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            self.assertTrue(wc.migrate_legacy_config(root))
            status = wc.load_config(root)
            self.assertEqual(status["source_context"]["roots"][0]["path"], "/opt/src")
            self.assertEqual(status["proxy_ports"]["uat"], "5434")
            self.assertTrue((root / ".knowledge-os-config.yaml").is_file())

    def test_migrate_quoted_legacy_file(self) -> None:
        wc = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".old-workspace-config.yaml").write_text(
                "\n".join(
                    [
                        "workspace:",
                        "  repository_roots:",
                        '    - "/opt/src"',
                        "  managed_clone:",
                        "    enabled: true",
                        '    root: "/opt/src"',
                        "skills:",
                        "  manage-development-handoff:",
                        '    worktree_root: "/opt/trees"',
                        "  inspect-database:",
                        "    environments:",
                        "      dev:",
                        "        proxy_port: 5433",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            self.assertTrue(wc.migrate_legacy_config(root))
            status = wc.load_config(root)
            self.assertEqual(status["source_context"]["roots"][0]["path"], "/opt/src")
            self.assertEqual(status["proxy_ports"]["dev"], "5433")
            self.assertEqual(status["worktree_root"], "/opt/trees")


if __name__ == "__main__":
    unittest.main()
