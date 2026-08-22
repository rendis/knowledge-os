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


def is_knowledge_file(vault: Path, path: Path) -> bool:
    if not path.is_file():
        return False
    root = path.relative_to(vault).parts[0]
    match = re.match(r"^(\d{2})(?:-|$)", root)
    return match is not None and 10 <= int(match.group(1)) <= 70


def knowledge_snapshot(vault: Path) -> dict[str, bytes]:
    """Capture cell identity, orientation, and all knowledge notes byte for byte."""
    paths = [vault / "instance.yaml", vault / "00-Home.md"]
    paths.extend(
        path
        for path in vault.rglob("*")
        if is_knowledge_file(vault, path)
    )
    return {
        path.relative_to(vault).as_posix(): path.read_bytes()
        for path in sorted(set(paths))
        if path.is_file()
    }


COMMAND_TARGET = re.compile(
    r"(?:python3|python|<python>)\s+(?:-B\s+)?"
    r"(?P<path>(?:90-Meta|\.agents)/[A-Za-z0-9_./-]+\.py)"
)


def installed_command_targets(vault: Path) -> set[str]:
    documents = [vault / "AGENTS.md"]
    documents.extend((vault / "90-Meta").rglob("*.md"))
    documents.extend((vault / ".agents" / "skills").rglob("*.md"))
    targets: set[str] = set()
    for document in documents:
        if not document.is_file():
            continue
        text = document.read_text(encoding="utf-8")
        targets.update(match.group("path") for match in COMMAND_TARGET.finditer(text))
    return targets


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
            lock = (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8")
            self.assertIn('version: "3"', lock)
            self.assertIn('distribution_revision: "', lock)
            self.assertIn("distribution_dirty:", lock)
            self.assertIn('"AGENTS.md":', lock)
            self.assertIn('"90-Meta/audit-vault.py":', lock)
            self.assertNotIn('"Arquitectura.base":', lock)
            gitignore = (dest / ".gitignore").read_text(encoding="utf-8")
            self.assertNotIn(".knowledge-os.lock.yaml", gitignore)
            self.assertIn("/.agents/state/map-ecosystem/sync/", gitignore)
            self.assertNotIn(".agents/state/map-ecosystem/sync", lock)
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            info = json.loads(doctor.stdout)
            self.assertTrue(info["orientation"]["ready"])
            self.assertEqual(info["start_here"][0], "00-Home.md")
            self.assertTrue(info["portable_lock"])
            self.assertTrue(info["managed_matches_dist"])
            self.assertEqual(
                info["distribution_revision_installed"],
                info["distribution_revision_dist"],
            )

    def test_fresh_cell_has_every_documented_local_command(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "cell"
            initialized = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "Operations",
                    "--purpose",
                    "Shared operational knowledge",
                    "--system",
                    "operations:Operations",
                    "--adapter",
                    "gcp",
                    "--adapter",
                    "postgres",
                    "--adapter",
                    "reports",
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            targets = sorted(installed_command_targets(dest))
            self.assertTrue(targets)
            missing = [target for target in targets if not (dest / target).is_file()]
            self.assertEqual(missing, [], f"documented command targets missing: {missing}")
            self.assertTrue((dest / "90-Meta/git-change-manifest.py").is_file())
            self.assertTrue((dest / "90-Meta/sync-run.py").is_file())
            self.assertTrue((dest / "90-Meta/static-evidence-scan.py").is_file())
            for command in (
                [sys.executable, "-B", "90-Meta/audit-vault.py"],
                [sys.executable, "-B", "90-Meta/verify-links.py"],
                [sys.executable, "-B", "90-Meta/validate-bases.py"],
            ):
                checked = run(command, cwd=dest)
                self.assertEqual(
                    checked.returncode,
                    0,
                    checked.stdout + checked.stderr,
                )

    def test_kernel_has_no_product_leak(self) -> None:
        leaks = []
        skip = {".git", "evals", "plan", "__pycache__"}
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

    def test_adopt_preserves_knowledge_and_extras(self) -> None:
        sys.path.insert(0, str(DIST / "kernel" / "90-Meta"))
        from instance import dump_instance, validate_instance

        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "existing"
            dest.mkdir()
            home_text = "# Keep Home\n"
            (dest / "00-Home.md").write_text(home_text, encoding="utf-8")
            (dest / "AGENTS.md").write_text("# cell-agents-keep\n", encoding="utf-8")
            (dest / "10-Sistemas").mkdir()
            (dest / "10-Sistemas" / "Payments.md").write_text("# Payments\n", encoding="utf-8")
            for dirname in (
                "15-Arquitectura",
                "20-Flujos",
                "25-Topics",
                "30-Datos",
                "40-Integraciones",
                "50-Operaciones",
                "60-Decisiones",
                "70-Riesgos",
            ):
                directory = dest / dirname
                directory.mkdir()
                (directory / "Cell knowledge.md").write_text(
                    f"# {dirname}\nKeep this knowledge byte for byte.\n",
                    encoding="utf-8",
                )
            (dest / "90-Meta").mkdir()
            (dest / "90-Meta" / "Alcance.md").write_text("# Cell scope\nKeep extra.\n", encoding="utf-8")
            (dest / "90-Meta" / "audit-vault.py").write_text("# cell-audit-keep\n", encoding="utf-8")
            extra = dest / ".agents" / "skills" / "cell-local-tool"
            extra.mkdir(parents=True)
            (extra / "SKILL.md").write_text("# cell-local-tool\n", encoding="utf-8")
            (dest / ".gitignore").write_text(
                ".DS_Store\n/custom-ignore\n.knowledge-os.lock.yaml\n",
                encoding="utf-8",
            )
            refused = run(
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
                    "--yes",
                ]
            )
            self.assertNotEqual(refused.returncode, 0)
            missing = run(["sh", str(INSTALL), "adopt", "--dest", str(dest)])
            self.assertNotEqual(missing.returncode, 0)
            (dest / "instance.yaml").write_text(
                dump_instance(
                    validate_instance(
                        {
                            "cell": {"name": "Payments", "purpose": "Card-present checkout"},
                            "systems": [{"id": "payments", "name": "Payments", "aliases": ["pay"]}],
                            "evidence": {"profile": "production-gate"},
                            "locale": {"notes": "en"},
                            "adapters": [],
                        }
                    )
                ),
                encoding="utf-8",
            )
            knowledge_before = knowledge_snapshot(dest)
            self.assertIn("15-Arquitectura/Cell knowledge.md", knowledge_before)
            self.assertIn("25-Topics/Cell knowledge.md", knowledge_before)
            conflict = run(["sh", str(INSTALL), "adopt", "--dest", str(dest)])
            self.assertEqual(conflict.returncode, 3, conflict.stdout + conflict.stderr)
            self.assertEqual(json.loads(conflict.stdout)["status"], "ownership-conflict")
            adopted = run(["sh", str(INSTALL), "adopt", "--dest", str(dest), "--force"])
            self.assertEqual(adopted.returncode, 0, adopted.stderr)
            payload = json.loads(adopted.stdout)
            self.assertEqual(payload["status"], "adopted")
            self.assertEqual((dest / "00-Home.md").read_text(encoding="utf-8"), home_text)
            self.assertEqual(
                (dest / "AGENTS.md").read_bytes(),
                (DIST / "kernel" / "AGENTS.md").read_bytes(),
            )
            self.assertEqual((dest / "90-Meta" / "Alcance.md").read_text(encoding="utf-8"), "# Cell scope\nKeep extra.\n")
            self.assertEqual(
                (dest / "90-Meta" / "audit-vault.py").read_bytes(),
                (DIST / "kernel" / "90-Meta" / "audit-vault.py").read_bytes(),
            )
            self.assertTrue((dest / ".agents" / "skills" / "cell-local-tool" / "SKILL.md").is_file())
            self.assertTrue((dest / "90-Meta" / "graph-query.py").is_file())
            self.assertTrue((dest / ".knowledge-os.lock.yaml").is_file())
            gitignore = (dest / ".gitignore").read_text(encoding="utf-8")
            self.assertIn("/custom-ignore", gitignore)
            self.assertNotIn(".knowledge-os.lock.yaml", gitignore)
            lock = (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8")
            self.assertIn('version: "3"', lock)
            self.assertIn('"AGENTS.md":', lock)
            self.assertNotIn("cell-local-tool", lock)
            self.assertIn('"90-Meta/audit-vault.py":', lock)
            self.assertNotIn('"90-Meta/Alcance.md":', lock)
            self.assertEqual(knowledge_snapshot(dest), knowledge_before)
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            info = json.loads(doctor.stdout)
            self.assertEqual(info["state"], "installed")
            self.assertTrue(info["orientation"]["ready"])
            sync_state = dest / ".agents" / "state" / "map-ecosystem" / "sync" / "active" / "run-eval" / "run.json"
            sync_state.parent.mkdir(parents=True)
            sync_state.write_text('{"local":"keep"}\n', encoding="utf-8")
            updated = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(updated.returncode, 0, updated.stderr)
            self.assertEqual(sync_state.read_text(encoding="utf-8"), '{"local":"keep"}\n')
            self.assertNotIn(
                ".agents/state/map-ecosystem/sync",
                (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8"),
            )
            self.assertTrue((dest / ".agents" / "skills" / "cell-local-tool" / "SKILL.md").is_file())
            self.assertTrue((dest / "90-Meta" / "Alcance.md").is_file())
            self.assertEqual(
                (dest / "90-Meta" / "audit-vault.py").read_bytes(),
                (DIST / "kernel" / "90-Meta" / "audit-vault.py").read_bytes(),
            )
            self.assertEqual(
                (dest / "AGENTS.md").read_bytes(),
                (DIST / "kernel" / "AGENTS.md").read_bytes(),
            )
            self.assertEqual((dest / "00-Home.md").read_text(encoding="utf-8"), home_text)
            self.assertEqual(knowledge_snapshot(dest), knowledge_before)

    def test_clean_clone_can_update_without_touching_knowledge(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "source"
            clone = root / "clone"
            initialized = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(source),
                    "--cell-name",
                    "Payments",
                    "--purpose",
                    "Card-present checkout",
                    "--system",
                    "payments:Payments",
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            self.assertEqual(run(["git", "init", "-b", "main"], cwd=source).returncode, 0)
            self.assertEqual(run(["git", "add", "."], cwd=source).returncode, 0)
            committed = run(
                [
                    "git",
                    "-c",
                    "user.name=Bootstrap Eval",
                    "-c",
                    "user.email=bootstrap@example.invalid",
                    "commit",
                    "-m",
                    "init cell",
                ],
                cwd=source,
            )
            self.assertEqual(committed.returncode, 0, committed.stderr)
            cloned = run(["git", "clone", "--no-hardlinks", str(source), str(clone)], cwd=root)
            self.assertEqual(cloned.returncode, 0, cloned.stderr)
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(clone)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            info = json.loads(doctor.stdout)
            self.assertEqual(info["state"], "installed")
            self.assertTrue(info["portable_lock"])
            self.assertTrue(info["managed_matches_dist"])
            knowledge_before = knowledge_snapshot(clone)
            updated = run(["sh", str(INSTALL), "update", "--dest", str(clone)])
            self.assertEqual(updated.returncode, 0, updated.stderr)
            self.assertEqual(knowledge_snapshot(clone), knowledge_before)
            after_update = run(["sh", str(INSTALL), "doctor", "--dest", str(clone)])
            self.assertEqual(after_update.returncode, 0, after_update.stderr)
            updated_info = json.loads(after_update.stdout)
            self.assertTrue(updated_info["portable_lock"])
            self.assertTrue(updated_info["managed_matches_dist"])

    def test_update_migrates_version2_lock_without_silent_runtime_overwrite(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "cell"
            initialized = run(
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
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            home = (dest / "00-Home.md").read_bytes()
            agents = dest / "AGENTS.md"
            agents.write_text("# Cell-owned router\n", encoding="utf-8")
            lock_path = dest / ".knowledge-os.lock.yaml"
            current_lock = lock_path.read_text(encoding="utf-8")
            legacy_lock = current_lock.replace('version: "3"', 'version: "2"', 1)
            legacy_lock = "\n".join(
                line
                for line in legacy_lock.splitlines()
                if not line.startswith((
                    "distribution_revision:",
                    "distribution_dirty:",
                    '  "AGENTS.md":',
                ))
            ) + "\n"
            lock_path.write_text(legacy_lock, encoding="utf-8")
            gitignore = dest / ".gitignore"
            gitignore.write_text(
                gitignore.read_text(encoding="utf-8") + ".knowledge-os.lock.yaml\n",
                encoding="utf-8",
            )
            blocked = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(blocked.returncode, 3, blocked.stdout + blocked.stderr)
            self.assertIn("AGENTS.md", json.loads(blocked.stdout)["files"])
            updated = run([
                "sh",
                str(INSTALL),
                "update",
                "--dest",
                str(dest),
                "--force",
            ])
            self.assertEqual(updated.returncode, 0, updated.stderr)
            self.assertEqual((dest / "00-Home.md").read_bytes(), home)
            self.assertEqual(agents.read_bytes(), (DIST / "kernel" / "AGENTS.md").read_bytes())
            portable_lock = lock_path.read_text(encoding="utf-8")
            self.assertIn('version: "3"', portable_lock)
            self.assertIn('"AGENTS.md":', portable_lock)
            self.assertNotIn(
                ".knowledge-os.lock.yaml",
                gitignore.read_text(encoding="utf-8"),
            )


if __name__ == "__main__":
    unittest.main()
