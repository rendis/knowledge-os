#!/usr/bin/env python3
"""Bootstrap evals: init, domain-leak scan, update safety, orientation."""
from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

DIST = Path(__file__).resolve().parents[2]
INSTALL = DIST / "install.sh"
NATIVE = (DIST / "NATIVE_RUNTIME.json").is_file()

def native_cli(vault: Path) -> str:
    arch = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine().lower(), platform.machine().lower())
    return str(vault / ".agents/bin" / (f"vaultctl-{platform.system().lower()}-{arch}" + (".exe" if os.name == "nt" else "")))

def resolve_command(vault: Path) -> list[str]:
    if NATIVE:
        return [native_cli(vault), "config", "resolve", "--vault", str(vault)]
    return [sys.executable, "-B", str(vault / "90-Meta/resolve-vault.py"), "--path", str(vault)]

FORBIDDEN = re.compile(
    r"iot|acme|APP90001|APP90002|cell-dbs|tagger|\bsateo\b|vendorx|"
    r"cell-monthly|proj-a|\bSOS\b",
    re.I,
)
SCAN_SUFFIXES = {".md", ".py", ".yaml", ".yml", ".sh", ".txt", ".json", ".sql", ".tmpl", ".toml"}


def run(
    args: list[str],
    cwd: Path | None = None,
    env: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        args,
        cwd=str(cwd or DIST),
        env=env,
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
    def test_init_note_locale_and_inventory_readiness(self) -> None:
        for locale, heading, purpose in (("es", "Sistemas", "Propósito"), ("en", "Systems", "Purpose")):
            for discovered in (False, True):
                with self.subTest(locale=locale, discovered=discovered), tempfile.TemporaryDirectory() as tmp:
                    root = Path(tmp)
                    dest = root / "cell"
                    args = ["sh", str(INSTALL), "init", "--dest", str(dest),
                            "--cell-name", "Example", "--purpose", "Example purpose",
                            "--system", "example:Example", "--locale", locale, "--yes"]
                    if discovered:
                        source = root / "source"
                        source.mkdir()
                        self.assertEqual(run(["git", "init", str(source)]).returncode, 0)
                        self.assertEqual(run(["git", "-C", str(source), "remote", "add", "origin",
                                              "https://example.com/org/source.git"]).returncode, 0)
                        args += ["--discovery-root", str(source)]
                    initialized = run(args)
                    self.assertEqual(initialized.returncode, 0, initialized.stdout + initialized.stderr)
                    home = (dest / "00-Home.md").read_text()
                    system = (dest / "10-Sistemas/Example.md").read_text()
                    self.assertIn("## " + heading, home)
                    self.assertIn("## " + purpose, system)
                    self.assertNotIn("No discovery roots were given", home)
                    runtime_guidance = (
                        "ejecutable de tu plataforma en `.agents/bin/`"
                        if locale == "es" else "platform executable in `.agents/bin/`"
                    )
                    self.assertIn(runtime_guidance if NATIVE else "workspace-config.py --vault-root . status --format json", home)
                    self.assertNotIn("{{", home + system)
                    resolved = run(resolve_command(dest))
                    self.assertEqual(resolved.returncode, 0, resolved.stderr)
                    orientation = json.loads(resolved.stdout)["orientation"]
                    self.assertTrue(orientation["ready"])
                    self.assertEqual(orientation["pending_inventory"], discovered)
                    (dest / "00-Home.md").write_text(home + "\nCell-owned addition.\n")
                    before = knowledge_snapshot(dest)
                    updated = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
                    self.assertEqual(updated.returncode, 0, updated.stdout + updated.stderr)
                    self.assertEqual(before, knowledge_snapshot(dest))

    def test_operational_catalog_derives_cell_area_mocs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for area in ("Git", "Runtime"):
                moc = root / "60-Operacion" / area / f"{area}.md"
                moc.parent.mkdir(parents=True)
                moc.write_text(
                    "---\ntipo: indice\ntags: [moc, operacion]\n---\n",
                    encoding="utf-8",
                )

            result = run(
                [
                    sys.executable,
                    "-B",
                    str(DIST / "kernel/90-Meta/operational-catalog.py"),
                    "--root",
                    str(root),
                    "list-areas",
                ]
            )

            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(
                json.loads(result.stdout),
                [
                    {"area": "Git", "path": "60-Operacion/Git/Git.md"},
                    {
                        "area": "Runtime",
                        "path": "60-Operacion/Runtime/Runtime.md",
                    },
                ],
            )

            broken = root / "60-Operacion" / "Broken"
            broken.mkdir()
            (broken / "Rule.md").write_text("# Missing area MOC\n", encoding="utf-8")
            invalid = run(
                [
                    sys.executable,
                    "-B",
                    str(DIST / "kernel/90-Meta/operational-catalog.py"),
                    "--root",
                    str(root),
                    "list-areas",
                ]
            )
            self.assertEqual(invalid.returncode, 2, invalid.stdout + invalid.stderr)
            invalid_payload = json.loads(invalid.stdout)
            self.assertEqual(invalid_payload["error"], "contract-invalid")
            self.assertIn("missing area MOC", invalid_payload["message"])

    def test_specialists_follow_the_router_contract(self) -> None:
        result = subprocess.run([sys.executable, "-B", str(DIST / "scripts/render_specialists.py"), "--check"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_shared_vault_interfaces_are_kernel_owned(self) -> None:
        for relative in (
            "90-Meta/resolve-vault.py",
            "90-Meta/vault-resolution.md",
            "90-Meta/node-selection.md",
            "90-Meta/work-item-evidence.md",
        ):
            self.assertTrue((DIST / "kernel" / relative).is_file(), relative)
        for relative in (
            ".agents/skills/map-ecosystem/scripts/resolve-vault.py",
            ".agents/skills/map-ecosystem/references/vault-resolution.md",
            ".agents/skills/map-ecosystem/references/node-selection.md",
        ):
            self.assertFalse((DIST / "kernel" / relative).exists(), relative)

    def test_shared_vault_resolver_requires_a_valid_instance(self) -> None:
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
                    "Resolver review",
                    "--purpose",
                    "Validate canonical vault identity",
                    "--system",
                    "resolver:Resolver",
                    "--yes",
                ]
            )
            self.assertEqual(
                initialized.returncode,
                0,
                initialized.stdout + initialized.stderr,
            )
            legacy_config = dest / ".old-workspace-config.yaml"
            legacy_config.write_text(
                "workspace:\n"
                "  repository_roots:\n"
                "    - /tmp/legacy-repositories\n",
                encoding="utf-8",
            )
            canonical_config = dest / ".knowledge-os-config.yaml"
            self.assertFalse(canonical_config.exists())
            resolver = dest / "90-Meta" / "resolve-vault.py"
            framework = (dest / "90-Meta" / "Auditoria - Framework.md").read_text(
                encoding="utf-8"
            )
            self.assertIn(
                '`<cli> config resolve --vault "<vault_root>"`',
                framework,
            )
            resolver_cache = Path(tmp) / "resolver-pycache"
            resolver_env = os.environ.copy()
            resolver_env["PYTHONPYCACHEPREFIX"] = str(resolver_cache)
            resolved = run(
                resolve_command(dest),
                env=resolver_env,
            )
            self.assertEqual(resolved.returncode, 0, resolved.stdout + resolved.stderr)
            self.assertEqual(json.loads(resolved.stdout)["status"], "resolved")
            self.assertFalse(canonical_config.exists())
            self.assertEqual(list(resolver_cache.rglob("*.pyc")), [])


            instance_path = dest / "instance.yaml"
            valid_instance = instance_path.read_text(encoding="utf-8")
            for invalid_remote in ("::::", "https://[bad"):
                instance_path.write_text(
                    valid_instance.replace(
                        '  remote: ""',
                        f'  remote: "{invalid_remote}"',
                        1,
                    ),
                    encoding="utf-8",
                )
                invalid_identity = run(
                    resolve_command(dest)
                )
                self.assertNotEqual(invalid_identity.returncode, 0, invalid_identity.stdout + invalid_identity.stderr)
                if not NATIVE:
                    self.assertEqual(json.loads(invalid_identity.stdout)["status"], "invalid")

            instance_path.write_text("", encoding="utf-8")
            invalid = run(
                resolve_command(dest)
            )
            self.assertNotEqual(invalid.returncode, 0, invalid.stdout + invalid.stderr)
            if not NATIVE:
                self.assertEqual(json.loads(invalid.stdout)["status"], "invalid")

    def test_update_removes_retired_managed_files(self) -> None:
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
                    "Review",
                    "--purpose",
                    "Review retired managed paths",
                    "--system",
                    "review:Review",
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)

            retired_relative = ".agents/skills/map-ecosystem/scripts/retired-resolver.py"
            retired = dest / retired_relative
            retired.parent.mkdir(parents=True, exist_ok=True)
            retired_bytes = b"# retired distribution file\n"
            retired.write_bytes(retired_bytes)
            cell_owned = retired.parent / "cell-owned-helper.py"
            cell_owned.write_text("# keep\n", encoding="utf-8")
            skill_relative = ".agents/skills/retired-skill/references/guide.md"
            retired_skill = dest / skill_relative
            retired_skill.parent.mkdir(parents=True, exist_ok=True)
            retired_skill.write_bytes(retired_bytes)

            lock_path = dest / ".knowledge-os.lock.yaml"
            lock = lock_path.read_text(encoding="utf-8")
            digest = hashlib.sha256(retired_bytes).hexdigest()
            lock_path.write_text(
                lock.replace(
                    "managed_hashes:\n",
                    f'managed_hashes:\n  "{retired_relative}": {digest}\n  "{skill_relative}": {digest}\n',
                    1,
                ),
                encoding="utf-8",
            )

            updated = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(updated.returncode, 0, updated.stdout + updated.stderr)
            self.assertFalse(retired.exists())
            self.assertTrue(cell_owned.is_file())
            self.assertFalse((dest / ".agents/skills/retired-skill").exists())
            self.assertNotIn(
                retired_relative,
                lock_path.read_text(encoding="utf-8"),
            )

            retired.write_text("# local change\n", encoding="utf-8")
            refreshed_lock = lock_path.read_text(encoding="utf-8")
            lock_path.write_text(
                refreshed_lock.replace(
                    "managed_hashes:\n",
                    f'managed_hashes:\n  "{retired_relative}": {digest}\n',
                    1,
                ),
                encoding="utf-8",
            )
            blocked = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(blocked.returncode, 3, blocked.stdout + blocked.stderr)
            self.assertTrue(retired.is_file())

            forced = run(
                ["sh", str(INSTALL), "update", "--dest", str(dest), "--force"]
            )
            self.assertEqual(forced.returncode, 0, forced.stdout + forced.stderr)
            self.assertFalse(retired.exists())
            self.assertTrue(cell_owned.is_file())

            first_relative = ".agents/skills/map-ecosystem/scripts/a-retired.py"
            first = dest / first_relative
            first.write_bytes(retired_bytes)
            invalid_relative = ".agents/skills/map-ecosystem/scripts/z-retired"
            invalid_target = dest / invalid_relative
            invalid_target.mkdir()
            lock_path.write_text(
                lock_path.read_text(encoding="utf-8").replace(
                    "managed_hashes:\n",
                    "managed_hashes:\n"
                    f'  "{first_relative}": {digest}\n'
                    f'  "{invalid_relative}": {digest}\n',
                    1,
                ),
                encoding="utf-8",
            )
            invalid_batch = run(
                ["sh", str(INSTALL), "update", "--dest", str(dest), "--force"]
            )
            self.assertEqual(
                invalid_batch.returncode,
                3,
                invalid_batch.stdout + invalid_batch.stderr,
            )
            self.assertTrue(first.is_file())
            self.assertTrue(invalid_target.is_dir())

    def test_update_refuses_retired_managed_file_through_symlinked_parent(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            dest = root / "cell"
            initialized = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "Review",
                    "--purpose",
                    "Reject unsafe retired paths",
                    "--system",
                    "review:Review",
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)

            outside = root / "outside"
            outside.mkdir()
            outside_file = outside / "retired.py"
            retired_bytes = b"# must remain outside the cell\n"
            outside_file.write_bytes(retired_bytes)
            linked_parent = dest / ".agents" / "skills" / "retired-skill"
            linked_parent.symlink_to(outside, target_is_directory=True)
            retired_relative = ".agents/skills/retired-skill/retired.py"
            lock_path = dest / ".knowledge-os.lock.yaml"
            lock_path.write_text(
                lock_path.read_text(encoding="utf-8").replace(
                    "managed_hashes:\n",
                    "managed_hashes:\n"
                    f'  "{retired_relative}": '
                    f"{hashlib.sha256(retired_bytes).hexdigest()}\n",
                    1,
                ),
                encoding="utf-8",
            )

            blocked = run(
                ["sh", str(INSTALL), "update", "--dest", str(dest), "--force"]
            )
            self.assertEqual(blocked.returncode, 3, blocked.stdout + blocked.stderr)
            self.assertEqual(json.loads(blocked.stdout)["status"], "invalid-lock")
            self.assertEqual(outside_file.read_bytes(), retired_bytes)

    def test_changed_workflows_have_no_legacy_resolution_routes(self) -> None:
        documents = [
            DIST / "kernel/.agents/skills/manage-development-handoff/SKILL.md",
            DIST / "kernel/.agents/skills/manage-operational-workflow/SKILL.md",
        ]
        combined = "\n".join(path.read_text(encoding="utf-8") for path in documents)
        self.assertNotIn("Legacy maintenance", combined)
        self.assertNotIn("path-less compatibility", combined)
        self.assertNotIn("when that resolver is unavailable", combined)

    def test_development_handoff_is_an_atomic_task_package(self) -> None:
        skill = (DIST / "kernel/.agents/skills/manage-development-handoff/SKILL.md").read_text(encoding="utf-8")
        router = (DIST / "kernel/AGENTS.md").read_text(encoding="utf-8")
        for required in ("handoffs/DH-NNN.md", "handoff start", "handoff status", "handoff refresh", "--apply", ".handoff/", "deltas.md", "Managed segment", "Reconcile into the investigation", "no vault"):
            self.assertIn(required, skill)
        self.assertNotIn("reconcile-development-handoff", router)
        self.assertFalse((DIST / "kernel/.agents/skills/reconcile-development-handoff").exists())
        for retired in ("plan_token", "ACTIVE.yaml is", "closure fingerprint", "set-state"):
            self.assertNotIn(retired, skill)

    def test_distribution_and_installed_kernel_versions_match(self) -> None:
        distribution_version = (DIST / "VERSION").read_text(encoding="utf-8")
        installed_version = (DIST / "kernel" / "VERSION").read_text(encoding="utf-8")
        self.assertEqual(installed_version, distribution_version)

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
            self.assertFalse((dest / "90-Meta/SOUL.md").exists())
            self.assertFalse((dest / "90-Meta/response-quality.md").exists())
            self.assertFalse((dest / "AGENTS.personal.md").exists())
            self.assertTrue((dest / ".agents" / "skills" / "map-ecosystem" / "SKILL.md").is_file())
            self.assertFalse((dest / ".agents" / "skills" / "scheduled-vault-refresh").exists())
            self.assertFalse((dest / ".agents" / "skills" / "obsidian-cli").exists())
            git_skill = dest / ".agents" / "skills" / "manage-git-workflow"
            self.assertTrue((git_skill / "SKILL.md").is_file())
            self.assertTrue((git_skill / "references" / "defaults.md").is_file())
            self.assertFalse((dest / ".agents" / "skills" / "inspect-gcp-runtime").exists())
            for shared in (
                *( () if NATIVE else ("resolve-vault.py",) ),
                "vault-resolution.md",
                "node-selection.md",
                "work-item-evidence.md",
            ):
                self.assertTrue((dest / "90-Meta" / shared).is_file(), shared)
            self.assertFalse(
                (
                    dest
                    / ".agents/skills/map-ecosystem/scripts/resolve-vault.py"
                ).exists()
            )
            lock = (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8")
            self.assertIn('version: "4"' if NATIVE else 'version: "3"', lock)
            self.assertIn('distribution_revision: "', lock)
            self.assertIn("distribution_dirty:", lock)
            self.assertIn('"AGENTS.md":', lock)
            self.assertNotIn('"90-Meta/SOUL.md":', lock)
            (self.assertNotIn if NATIVE else self.assertIn)('"90-Meta/audit-vault.py":', lock)
            self.assertNotIn('"Arquitectura.base":', lock)
            gitignore = (dest / ".gitignore").read_text(encoding="utf-8")
            self.assertNotIn(".knowledge-os.lock.yaml", gitignore)
            self.assertIn("/.agents/state/map-ecosystem/", gitignore)
            self.assertIn("/AGENTS.personal.md", gitignore)
            self.assertIn("/.plan/", gitignore)
            self.assertIn("/.scratch/", gitignore)
            self.assertIn("/.venv/", gitignore)
            self.assertIn("/.investigations/", gitignore)
            self.assertIn("/.investigations-private/", gitignore)
            self.assertNotIn("/investigations/", gitignore)
            self.assertNotIn("yaak", gitignore)
            self.assertNotIn("artifacts/", gitignore)
            self.assertNotIn("/plan/", gitignore)
            obsidian_app = json.loads(
                (dest / ".obsidian" / "app.json").read_text(encoding="utf-8")
            )
            self.assertEqual(obsidian_app["userIgnoreFilters"], [".plan/", ".scratch/", ".investigations/", "AGENTS.personal.md"])
            self.assertNotIn(".agents/state/map-ecosystem/sync", lock)
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            info = json.loads(doctor.stdout)
            self.assertTrue(info["orientation"]["ready"])
            self.assertEqual(info["start_here"][0], "00-Home.md")
            self.assertTrue(info["portable_lock"])
            self.assertTrue(info["managed_matches_dist"])
            self.assertEqual(
                info["personal_instructions"],
                {
                    "path": "AGENTS.personal.md",
                    "exists": False,
                    "ignored": True,
                    "tracked": False,
                },
            )
            self.assertEqual(
                info["distribution_revision_installed"],
                info["distribution_revision_dist"],
            )

    def test_init_roundtrips_trackers_and_rejects_invalid_updates(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            invalid_dest = Path(tmp) / "invalid-cell"
            invalid_init = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(invalid_dest),
                    "--tracker",
                    "broken",
                    "--system",
                    "test:Test",
                    "--yes",
                ]
            )
            self.assertEqual(invalid_init.returncode, 2, invalid_init.stdout)
            self.assertEqual(
                json.loads(invalid_init.stdout)["status"], "invalid-instance"
            )
            self.assertFalse(invalid_dest.exists())

            dest = Path(tmp) / "cell"
            initialized = run(
                [
                    "sh",
                    str(INSTALL),
                    "init",
                    "--dest",
                    str(dest),
                    "--cell-name",
                    "Multi tracker",
                    "--purpose",
                    "Validate work-item routing",
                    "--system",
                    "platform:Platform",
                    "--tracker",
                    "jira-core:jira:https://core.atlassian.net/",
                    "--tracker",
                    "clickup-product:clickup:https://app.clickup.com/123456",
                    "--yes",
                ]
            )
            self.assertEqual(
                initialized.returncode,
                0,
                initialized.stdout + initialized.stderr,
            )
            instance_path = dest / "instance.yaml"
            instance = instance_path.read_text(encoding="utf-8")
            self.assertIn('id: "jira-core"', instance)
            self.assertIn('provider: "clickup"', instance)
            self.assertIn('url: "https://core.atlassian.net"', instance)

            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stdout + doctor.stderr)
            self.assertEqual(json.loads(doctor.stdout)["instance"]["status"], "valid")

            instance_path.write_text(
                instance.replace(
                    'url: "https://core.atlassian.net"',
                    'url: "https://user:secret@core.atlassian.net"',
                    1,
                ),
                encoding="utf-8",
            )
            invalid_doctor = run(
                ["sh", str(INSTALL), "doctor", "--dest", str(dest)]
            )
            self.assertEqual(invalid_doctor.returncode, 2, invalid_doctor.stdout)
            self.assertEqual(
                json.loads(invalid_doctor.stdout)["instance"]["status"],
                "invalid",
            )
            invalid_update = run(
                ["sh", str(INSTALL), "update", "--dest", str(dest)]
            )
            self.assertEqual(invalid_update.returncode, 2, invalid_update.stdout)
            self.assertEqual(
                json.loads(invalid_update.stdout)["status"],
                "invalid-instance",
            )

            instance_path.write_text("cell: [\n", encoding="utf-8")
            malformed_doctor = run(
                ["sh", str(INSTALL), "doctor", "--dest", str(dest)]
            )
            self.assertEqual(
                malformed_doctor.returncode,
                2,
                malformed_doctor.stdout + malformed_doctor.stderr,
            )
            malformed_payload = json.loads(malformed_doctor.stdout)
            self.assertEqual(malformed_payload["instance"]["status"], "invalid")
            self.assertEqual(
                malformed_payload["instance"]["error"],
                "invalid instance.yaml syntax",
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
                    "reports",
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            targets = sorted(installed_command_targets(dest))
            if NATIVE:
                self.assertEqual(targets, [], "consumer docs still invoke retired Python runtime")
                self.assertEqual(list(dest.rglob("*.py")), [])
                self.assertTrue(Path(native_cli(dest)).is_file())
                commands = [[native_cli(dest), "audit", "--vault", str(dest)],
                            [native_cli(dest), "check", "links", "--vault", str(dest)],
                            [native_cli(dest), "check", "bases", "--vault", str(dest)]]
            else:
                # During migration the docs may already use native commands;
                # legacy scripts remain exercised as distribution regressions.
                missing = [target for target in targets if not (dest / target).is_file()]
                self.assertEqual(missing, [], f"documented command targets missing: {missing}")
                commands = [[sys.executable, "-B", "90-Meta/" + script] for script in
                            ("audit-vault.py", "verify-links.py", "validate-bases.py")]
            for command in commands:
                checked = run(command, cwd=dest)
                self.assertEqual(
                    checked.returncode,
                    0,
                    checked.stdout + checked.stderr,
                )

    def test_quality_gate_normalizes_signal_return_codes(self) -> None:
        helper = DIST / "kernel/90-Meta/check-code-quality.py"
        spec = importlib.util.spec_from_file_location("code_quality", helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)

        cases = (
            ((0, 0, 0), 0),
            ((1, 0, 0), 1),
            ((2, 1, 0), 2),
            ((-9, 0, 0), 137),
            ((-2, 1, 0), 130),
        )
        for return_codes, expected in cases:
            with (
                self.subTest(return_codes=return_codes),
                mock.patch.object(module.importlib.util, "find_spec", return_value=object()),
                mock.patch.object(module, "run", return_value=return_codes[0]) as run_mock,
                mock.patch.object(module, "run_bandit", side_effect=return_codes[1:]) as bandit_mock,
                mock.patch.object(sys, "argv", [str(helper), "--root", str(DIST)]),
            ):
                self.assertEqual(module.main(), expected)
                self.assertEqual(run_mock.call_count, 1)
                self.assertEqual(bandit_mock.call_count, 2)

    def test_kernel_has_no_product_leak(self) -> None:
        leaks = []
        skip = {".git", ".venv", "evals", "plan", "__pycache__"}
        for path in DIST.rglob("*"):
            if not path.is_file():
                continue
            if any(part in skip for part in path.parts):
                continue
            # Research reports are evaluation evidence, not installed cell policy.
            # Keep public runtime, installer, agent definitions and ordinary docs checked.
            if path.relative_to(DIST).parts[:2] == ("docs", "research"):
                continue
            if path.suffix not in SCAN_SUFFIXES:
                continue
            text = path.read_text(encoding="utf-8", errors="ignore")
            if FORBIDDEN.search(text):
                leaks.append(str(path.relative_to(DIST)))
        self.assertEqual(leaks, [], f"product leaks: {leaks}")

    def test_product_scan_keeps_runtime_protected_but_allows_research(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report = root / "docs/research/example.md"
            report.parent.mkdir(parents=True)
            report.write_text("IoT evaluation evidence", encoding="utf-8")
            with mock.patch.dict(globals(), {"DIST": root}):
                self.test_kernel_has_no_product_leak()
                for relative in ("kernel/AGENTS.md", "kernel/.codex/agents/example.toml", "adapters/example/SKILL.md", "README.md"):
                    target = root / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text("IoT consumer-specific policy", encoding="utf-8")
                    with self.assertRaises(AssertionError):
                        self.test_kernel_has_no_product_leak()
                    target.unlink()

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

    def test_update_accepts_gitleaks_annotated_managed_digest(self) -> None:
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
                    "Annotated lock",
                    "--purpose",
                    "Validate managed digest annotations",
                    "--system",
                    "lock:Lock",
                    "--yes",
                ]
            )
            self.assertEqual(
                initialized.returncode,
                0,
                initialized.stdout + initialized.stderr,
            )
            lock_path = dest / ".knowledge-os.lock.yaml"
            digest = hashlib.sha256((dest / "AGENTS.md").read_bytes()).hexdigest()
            plain_entry = f'  "AGENTS.md": {digest}'
            annotated_entry = (
                f"{plain_entry} # gitleaks:allow -- managed SHA-256 digest"
            )
            lock = lock_path.read_text(encoding="utf-8")
            if annotated_entry not in lock:
                self.assertIn(plain_entry, lock)
                lock_path.write_text(
                    lock.replace(plain_entry, annotated_entry, 1),
                    encoding="utf-8",
                )

            updated = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(
                updated.returncode,
                0,
                updated.stdout + updated.stderr,
            )
            self.assertIn(
                annotated_entry,
                lock_path.read_text(encoding="utf-8"),
            )

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
                    "reports",
                    "--yes",
                ]
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((dest / ".agents/skills/generate-reports/SKILL.md").is_file())
            self.assertTrue((dest / ".agents/skills/inspect-database/SKILL.md").is_file())
            for name in ("inspect-gcp-runtime", "gcloud", "cloud-logging-query-generation",
                         "cloud-monitoring-metric-selection"):
                self.assertFalse((dest / ".agents/skills" / name).exists())

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
            git_policy = dest / "60-Operacion" / "Git" / "Git.md"
            git_policy.parent.mkdir(parents=True)
            git_policy_text = "# Git\nKeep cell Git policy byte for byte.\n"
            git_policy.write_text(git_policy_text, encoding="utf-8")
            (dest / "90-Meta").mkdir()
            (dest / "90-Meta" / "Alcance.md").write_text("# Cell scope\nKeep extra.\n", encoding="utf-8")
            (dest / "90-Meta" / "audit-vault.py").write_text("# cell-audit-keep\n", encoding="utf-8")
            extra = dest / ".agents" / "skills" / "cell-local-tool"
            extra.mkdir(parents=True)
            (extra / "SKILL.md").write_text("# cell-local-tool\n", encoding="utf-8")
            (dest / ".gitignore").write_text(
                ".DS_Store\n/custom-ignore\n.knowledge-os.lock.yaml\n/plan/\n",
                encoding="utf-8",
            )
            (dest / ".obsidian").mkdir()
            (dest / ".obsidian" / "app.json").write_text(
                '{"livePreview": true, "userIgnoreFilters": ["archive/", "plan/", "investigations/"]}\n',
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
                b"# cell-audit-keep\n" if NATIVE else (DIST / "kernel" / "90-Meta" / "audit-vault.py").read_bytes(),
            )
            self.assertTrue((dest / ".agents" / "skills" / "cell-local-tool" / "SKILL.md").is_file())
            self.assertEqual((dest / "90-Meta" / "graph-query.py").is_file(), not NATIVE)
            self.assertTrue((dest / ".knowledge-os.lock.yaml").is_file())
            gitignore = (dest / ".gitignore").read_text(encoding="utf-8")
            self.assertIn("/custom-ignore", gitignore)
            self.assertNotIn(".knowledge-os.lock.yaml", gitignore)
            self.assertIn("/.plan/", gitignore)
            self.assertIn("/.scratch/", gitignore)
            self.assertIn("/.venv/", gitignore)
            self.assertIn("/AGENTS.personal.md", gitignore)
            self.assertNotIn("/plan/", gitignore)
            obsidian_app = json.loads(
                (dest / ".obsidian" / "app.json").read_text(encoding="utf-8")
            )
            self.assertTrue(obsidian_app["livePreview"])
            self.assertEqual(
                obsidian_app["userIgnoreFilters"],
                ["archive/", ".plan/", ".scratch/", ".investigations/", "AGENTS.personal.md"],
            )
            lock = (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8")
            self.assertIn('version: "4"' if NATIVE else 'version: "3"', lock)
            self.assertIn('"AGENTS.md":', lock)
            self.assertNotIn("cell-local-tool", lock)
            (self.assertNotIn if NATIVE else self.assertIn)('"90-Meta/audit-vault.py":', lock)
            self.assertNotIn('"90-Meta/Alcance.md":', lock)
            self.assertEqual(knowledge_snapshot(dest), knowledge_before)
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            info = json.loads(doctor.stdout)
            self.assertEqual(info["state"], "installed")
            personal = dest / "AGENTS.personal.md"
            personal_text = "# Personal instructions\n\nUse the local mail profile.\n"
            personal.write_text(personal_text, encoding="utf-8")
            self.assertTrue(info["orientation"]["ready"])
            sync_state = dest / ".agents" / "state" / "map-ecosystem" / "sync" / "active" / "run-eval" / "run.json"
            sync_state.parent.mkdir(parents=True)
            sync_state.write_text('{"local":"keep"}\n', encoding="utf-8")
            updated = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(updated.returncode, 0, updated.stderr)
            self.assertEqual(sync_state.read_text(encoding="utf-8"), '{"local":"keep"}\n')
            self.assertEqual(personal.read_text(encoding="utf-8"), personal_text)
            self.assertNotIn(
                ".agents/state/map-ecosystem/sync",
                (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8"),
            )
            self.assertTrue((dest / ".agents" / "skills" / "cell-local-tool" / "SKILL.md").is_file())
            self.assertEqual(git_policy.read_text(encoding="utf-8"), git_policy_text)
            installed_git_skill = dest / ".agents/skills/manage-git-workflow/SKILL.md"
            distributed_git_skill = DIST / "kernel/.agents/skills/manage-git-workflow/SKILL.md"
            self.assertEqual(
                installed_git_skill.read_bytes(),
                distributed_git_skill.read_bytes(),
            )
            self.assertTrue((dest / "90-Meta" / "Alcance.md").is_file())
            self.assertEqual(
                (dest / "90-Meta" / "audit-vault.py").read_bytes(),
                b"# cell-audit-keep\n" if NATIVE else (DIST / "kernel" / "90-Meta" / "audit-vault.py").read_bytes(),
            )
            self.assertEqual(
                (dest / "AGENTS.md").read_bytes(),
                (DIST / "kernel" / "AGENTS.md").read_bytes(),
            )
            self.assertFalse((dest / "90-Meta/SOUL.md").exists())
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
            self.assertIn('version: "4"' if NATIVE else 'version: "3"', portable_lock)
            self.assertIn('"AGENTS.md":', portable_lock)
            self.assertNotIn(
                ".knowledge-os.lock.yaml",
                gitignore.read_text(encoding="utf-8"),
            )

    def test_personal_agents_contract_and_doctor_rejects_tracked_copy(self) -> None:
        router = (DIST / "kernel" / "AGENTS.md").read_text(encoding="utf-8")
        self.assertIn("## Personal instructions", router)
        self.assertIn("@AGENTS.personal.md", router)
        self.assertIn("## Evidence contract", router)
        self.assertLess(
            router.index("## Personal instructions"),
            router.index("## Route the request"),
        )
        self.assertLess(
            router.index("## Personal instructions"),
            router.index("## Guardrails"),
        )

        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "cell"
            initialized = run([
                "sh", str(INSTALL), "init", "--dest", str(dest),
                "--cell-name", "Payments", "--purpose", "Card-present checkout",
                "--system", "payments:Payments", "--yes",
            ])
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            self.assertEqual(run(["git", "init", str(dest)]).returncode, 0)
            spec = importlib.util.spec_from_file_location("personal_installer", DIST / "scripts/knowledge_os.py")
            installer = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(installer)
            lock = installer.load_lock(dest / installer.LOCK_NAME)
            # Isolate provenance only; exercise real hashes, Git and strict checks.
            lock["distribution_revision"] = "test-revision"
            lock["distribution_dirty"] = False

            def strict_status():
                with mock.patch.object(installer, "distribution_provenance", return_value=("test-revision", False)), mock.patch.object(installer, "load_lock", return_value=lock), mock.patch("builtins.print"):
                    return installer.cmd_doctor(SimpleNamespace(dest=str(dest), strict=True))

            self.assertEqual(strict_status(), 0)
            personal = dest / "AGENTS.personal.md"
            personal.write_text("# Personal instructions\n", encoding="utf-8")
            self.assertEqual(
                run(["git", "-C", str(dest), "check-ignore", "AGENTS.personal.md"]).returncode,
                0,
            )
            self.assertNotEqual(
                run(["git", "-C", str(dest), "add", "AGENTS.personal.md"]).returncode,
                0,
            )
            self.assertEqual(strict_status(), 0)
            self.assertEqual(
                run(["git", "-C", str(dest), "add", "-f", "AGENTS.personal.md"]).returncode,
                0,
            )
            doctor = run(["sh", str(INSTALL), "doctor", "--dest", str(dest)])
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            self.assertTrue(json.loads(doctor.stdout)["personal_instructions"]["tracked"])
            self.assertEqual(strict_status(), 2)

    def test_router_defers_rare_workflows_and_ships_sync_skill(self) -> None:
        router = (DIST / "kernel" / "AGENTS.md").read_text(encoding="utf-8")
        self.assertNotIn("vault-catalog.md", router)
        self.assertNotIn("cross-vault-consultation.md", router)
        self.assertIn("[synchronize-ecosystem](.agents/skills/synchronize-ecosystem/SKILL.md)", router)
        self.assertIn("only when dispatching or reconsidering a subagent", router)
        map_skill = (
            DIST / "kernel/.agents/skills/map-ecosystem/SKILL.md"
        ).read_text(encoding="utf-8")
        self.assertIn("synchronize-ecosystem", map_skill)
        self.assertNotIn(
            "Load `references/synchronization-package-worker.md`",
            map_skill,
        )
        self.assertTrue(
            (DIST / "kernel/.agents/skills/synchronize-ecosystem/SKILL.md").is_file()
        )
        investigation = (
            DIST / "kernel/.agents/skills/manage-investigation/SKILL.md"
        ).read_text(encoding="utf-8")
        for required in ("**understanding**", "**development**", "investigation new", "investigation check", "sync start --vault", "Retired-Case:", "never copied", "investigation add", "--for-vault", "handoff reconcile", "neutral, practical language", "write it only through the CLI"):
            self.assertIn(required, investigation)
        for retired in ("record-contract", "--expected-public-sha256", "transition --to"):
            self.assertNotIn(retired, investigation)


if __name__ == "__main__":
    unittest.main()
