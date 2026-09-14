"""Portable investigation handoff resolution against isolated Git repositories."""
from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


DIST = Path(__file__).resolve().parents[2]
INSTALL = DIST / "install.sh"


def run(*args: str, cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, cwd=cwd, text=True, capture_output=True, check=False)


class PortableResolution(unittest.TestCase):
    def test_same_remote_and_branch_resolve_under_two_local_roots(self) -> None:
        with tempfile.TemporaryDirectory() as raw:
            base = Path(raw)
            cell = base / "cell"
            installed = run(
                "sh", str(INSTALL), "init", "--dest", str(cell),
                "--cell-name", "Portable test", "--purpose", "Isolated evaluation",
                "--system", "test:Test", "--yes",
            )
            self.assertEqual(installed.returncode, 0, installed.stderr)

            outputs: list[dict[str, str]] = []
            for suffix in ("a", "b"):
                repository = base / f"repos-{suffix}" / "synthetic"
                repository.mkdir(parents=True)
                self.assertEqual(run("git", "init", "-b", "main", str(repository)).returncode, 0)
                (repository / "README.md").write_text("synthetic\n", encoding="utf-8")
                self.assertEqual(run("git", "-C", str(repository), "add", "README.md").returncode, 0)
                committed = run(
                    "git", "-C", str(repository), "-c", "user.name=Fixture",
                    "-c", "user.email=fixture@example.invalid", "commit", "-m", "test: seed",
                )
                self.assertEqual(committed.returncode, 0, committed.stderr)
                self.assertEqual(
                    run("git", "-C", str(repository), "remote", "add", "origin", "https://example.invalid/team/synthetic.git").returncode,
                    0,
                )
                worktree_root = base / f"wt-{suffix}"
                target = worktree_root / "synthetic" / "portable-case"
                target.parent.mkdir(parents=True)
                added = run(
                    "git", "-C", str(repository), "worktree", "add", "-b",
                    "issue/portable-case", str(target), "main",
                )
                self.assertEqual(added.returncode, 0, added.stderr)

                config = cell / "90-Meta/workspace-config.py"
                action = "initialize" if suffix == "a" else "update"
                configured = run(
                    sys.executable, "-B", str(config), "--vault-root", str(cell), action,
                    "--repository-root", str(repository),
                    "--development-worktree-root", str(worktree_root),
                )
                self.assertEqual(configured.returncode, 0, configured.stderr)
                helper = cell / ".agents/skills/manage-development-handoff/scripts/development-handoff.py"
                resolved = run(
                    sys.executable, "-B", str(helper), "--vault-root", str(cell),
                    "resolve-branch", "--repository-remote", "example.invalid/team/synthetic",
                    "--branch", "issue/portable-case",
                )
                self.assertEqual(resolved.returncode, 0, resolved.stderr)
                outputs.append(json.loads(resolved.stdout))

            self.assertEqual([item["status"] for item in outputs], ["available", "available"])
            self.assertEqual([item["branch"] for item in outputs], ["issue/portable-case"] * 2)
            self.assertNotEqual(outputs[0]["worktree_path"], outputs[1]["worktree_path"])

            missing = run(
                sys.executable, "-B", str(cell / ".agents/skills/manage-development-handoff/scripts/development-handoff.py"),
                "--vault-root", str(cell), "resolve-branch",
                "--repository-remote", "example.invalid/team/synthetic",
                "--branch", "issue/not-local",
            )
            self.assertEqual(missing.returncode, 0, missing.stderr)
            payload = json.loads(missing.stdout)
            self.assertEqual(payload["status"], "unavailable")
            self.assertEqual(payload["availability"], "not-local")


if __name__ == "__main__":
    unittest.main()
