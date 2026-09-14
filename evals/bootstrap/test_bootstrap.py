#!/usr/bin/env python3
"""Bootstrap evals: init, domain-leak scan, update safety, orientation."""
from __future__ import annotations

import hashlib
import importlib.util
import json
import os
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
FORBIDDEN = re.compile(
    r"iot|acme|APP90001|APP90002|cell-dbs|tagger|\bsateo\b|vendorx|"
    r"cell-monthly|proj-a|\bSOS\b",
    re.I,
)
SCAN_SUFFIXES = {".md", ".py", ".yaml", ".yml", ".sh", ".txt", ".json", ".sql", ".tmpl"}


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
                    self.assertIn("workspace-config.py --vault-root . status --format json", home)
                    self.assertNotIn("{{", home + system)
                    resolved = run([sys.executable, "-B", str(dest / "90-Meta/resolve-vault.py"),
                                    "--path", str(dest)])
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

    def test_development_handoff_responsibilities_keep_state_in_the_worktree(self) -> None:
        managed_block = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/assets/agents-managed-block.md"
        ).read_text(encoding="utf-8")
        start = (
            DIST / "kernel/.agents/skills/manage-development-handoff/assets/start.md"
        ).read_text(encoding="utf-8")
        reconcile = (
            DIST / "kernel/.agents/skills/reconcile-development-handoff/SKILL.md"
        ).read_text(encoding="utf-8")
        manage = (
            DIST / "kernel/.agents/skills/manage-development-handoff/SKILL.md"
        ).read_text(encoding="utf-8")
        repository_state = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/references/repository-state.md"
        ).read_text(encoding="utf-8")
        router = (DIST / "kernel/AGENTS.md").read_text(encoding="utf-8")
        operational = (
            DIST / "kernel/.agents/skills/manage-operational-workflow/SKILL.md"
        ).read_text(encoding="utf-8")

        self.assertNotIn("reconcile-development-handoff", managed_block)
        self.assertNotIn("reconcile-development-handoff", start)
        self.assertIn("worktree-local lifecycle source of truth", managed_block)
        self.assertIn("current worktree and its anchor branch", managed_block)
        self.assertIn(
            "agent working in this worktree owns handoff lifecycle transitions",
            managed_block,
        )
        self.assertNotIn("source cell", managed_block)
        self.assertIn("A state-only request", managed_block)
        self.assertIn("counts as explicit lifecycle direction", managed_block)
        self.assertIn("without another confirmation", managed_block)
        self.assertIn("Ask once only when", managed_block)
        self.assertIn("does not authorize implementation", managed_block)
        self.assertIn("**Set state**", managed_block)
        self.assertIn("never edited freehand", managed_block)
        self.assertNotIn("The vault independently resolves this worktree", managed_block)
        self.assertNotIn("owns authorized state changes", managed_block)
        self.assertNotIn("Leave `.knowledge-os-handoffs/ACTIVE.yaml` intact", managed_block)
        self.assertIn("For each selected entry, read its `handoff.yaml`", managed_block)
        self.assertNotIn("every listed `handoff.yaml`", managed_block)
        self.assertIn("`ready-for-production` or `production`", managed_block)
        self.assertIn("files they affect", managed_block)
        self.assertIn("worktree-local lifecycle source of truth", start)
        self.assertIn("**Set state**", start)
        self.assertNotIn("source cell", start)
        self.assertIn("without changing its lifecycle state", start)
        self.assertNotIn("updates the selected story state", start)
        self.assertNotIn("manage-operational-workflow", reconcile)
        self.assertIn("90-Meta/work-item-evidence.md", reconcile)
        self.assertIn("reconciliation reads it but does not change it", reconcile)
        self.assertNotIn("invoke `manage-development-handoff` **Set state**", reconcile)
        self.assertNotIn("state changes belong only", reconcile)
        self.assertIn("worktree agent owns lifecycle transitions", manage)
        self.assertNotIn("source cell", manage)
        self.assertIn("without another confirmation", manage)
        self.assertIn("without another confirmation", repository_state)
        self.assertIn("preserving its worktree-local lifecycle state", router)
        self.assertNotIn("optionally align its worktree-local lifecycle state", router)
        self.assertIn("90-Meta/work-item-evidence.md", operational)
        self.assertIn("90-Meta/jira-evidence.md", operational)

    def test_shared_vault_interfaces_are_kernel_owned(self) -> None:
        for relative in (
            "90-Meta/resolve-vault.py",
            "90-Meta/vault-resolution.md",
            "90-Meta/node-selection.md",
            "90-Meta/work-item-evidence.md",
            "90-Meta/jira-evidence.md",
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
                "`python3 -B 90-Meta/resolve-vault.py [--path <vault>]`",
                framework,
            )
            resolver_cache = Path(tmp) / "resolver-pycache"
            resolver_env = os.environ.copy()
            resolver_env["PYTHONPYCACHEPREFIX"] = str(resolver_cache)
            resolved = run(
                [
                    sys.executable,
                    "-B",
                    str(resolver),
                    "--path",
                    str(dest),
                ],
                env=resolver_env,
            )
            self.assertEqual(resolved.returncode, 0, resolved.stdout + resolved.stderr)
            self.assertEqual(json.loads(resolved.stdout)["status"], "resolved")
            self.assertFalse(canonical_config.exists())
            self.assertEqual(list(resolver_cache.rglob("*.pyc")), [])

            scanner_cache = Path(tmp) / "scanner-pycache"
            scanner_env = os.environ.copy()
            scanner_env["PYTHONPYCACHEPREFIX"] = str(scanner_cache)
            scanner = dest / "90-Meta" / "static-evidence-scan.py"
            scanned = run(
                [sys.executable, "-B", str(scanner)],
                cwd=dest,
                env=scanner_env,
            )
            self.assertEqual(scanned.returncode, 2, scanned.stdout + scanned.stderr)
            self.assertEqual(list(scanner_cache.rglob("*.pyc")), [])

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
                    [
                        sys.executable,
                        "-B",
                        str(resolver),
                        "--path",
                        str(dest),
                    ]
                )
                self.assertEqual(
                    invalid_identity.returncode,
                    2,
                    invalid_identity.stdout + invalid_identity.stderr,
                )
                self.assertEqual(json.loads(invalid_identity.stdout)["status"], "invalid")

            instance_path.write_text("", encoding="utf-8")
            invalid = run(
                [
                    sys.executable,
                    "-B",
                    str(resolver),
                    "--path",
                    str(dest),
                ]
            )
            self.assertEqual(invalid.returncode, 2, invalid.stdout + invalid.stderr)
            self.assertEqual(json.loads(invalid.stdout)["status"], "invalid")

    def test_investigations_register_exact_development_handoffs(self) -> None:
        template = (
            DIST
            / "kernel/.agents/skills/manage-investigation/assets/investigation-template.md"
        ).read_text(encoding="utf-8")
        contract = (
            DIST
            / "kernel/.agents/skills/manage-investigation/references/record-contract.md"
        ).read_text(encoding="utf-8")
        consumer = (
            DIST / "kernel/.agents/skills/manage-development-handoff/SKILL.md"
        ).read_text(encoding="utf-8")
        owner = (
            DIST / "kernel/.agents/skills/manage-investigation/SKILL.md"
        ).read_text(encoding="utf-8")
        export_contract = (
            DIST
            / "kernel/.agents/skills/manage-investigation/references/export-contract.md"
        ).read_text(encoding="utf-8")
        repository_state = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/references/repository-state.md"
        ).read_text(encoding="utf-8")
        self.assertIn("## Development handoffs", template)
        self.assertIn("`DH-001`", contract)
        self.assertIn("exact Git branch", contract)
        self.assertIn("**Bind development handoff**", consumer)
        self.assertIn("## Bind development handoff", owner)
        self.assertIn("exact-retry no-op", owner)
        self.assertIn("byte-level case no-op", contract)
        self.assertIn(
            "No materialization or activation is complete until",
            consumer,
        )
        self.assertIn("component or implementation scope", owner)
        self.assertIn("existing `DH-NNN`", owner)
        self.assertIn("separate worktree", export_contract)
        self.assertIn("shared group", consumer)
        self.assertIn("collision-safe work-item token", repository_state)
        self.assertIn("selected entry is not `active`", repository_state)
        self.assertNotIn("while another family is active", repository_state)

    def test_investigation_lifecycle_is_three_state_and_output_scoped(self) -> None:
        helper_path = (
            DIST
            / "kernel/.agents/skills/manage-investigation/scripts/investigation-case.py"
        )
        module_name = "investigation_case_lifecycle_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper_path)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)
        self.assertEqual(module.STATUSES, {"investigating", "blocked", "closed"})
        self.assertEqual(module.CLOSURE_OUTCOMES, {"completed", "abandoned"})

        lifecycle = (
            DIST
            / "kernel/.agents/skills/manage-investigation/references/readiness-and-lifecycle.md"
        ).read_text(encoding="utf-8")
        export_contract = (
            DIST
            / "kernel/.agents/skills/manage-investigation/references/export-contract.md"
        ).read_text(encoding="utf-8")
        input_bundle = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/references/input-bundle.md"
        ).read_text(encoding="utf-8")
        reconciler = (
            DIST / "kernel/.agents/skills/reconcile-development-handoff/SKILL.md"
        ).read_text(encoding="utf-8")

        for status in ("`investigating`", "`blocked`", "`closed`"):
            self.assertIn(status, lifecycle)
        self.assertIn("Do not store `resume-to`", lifecycle)
        self.assertIn("objective", lifecycle)
        self.assertIn("unrelated pending story", lifecycle)
        self.assertIn("selected story-and-repository output", input_bundle)
        self.assertIn("Another story or repository may remain pending", input_bundle)
        self.assertIn("None of these events closes the investigation automatically", export_contract)
        self.assertIn("Do not close, reopen, block, or unblock the investigation automatically", reconciler)

    def test_investigation_reconciliation_requires_semantic_review_after_clean_merge(self) -> None:
        runbook = (
            DIST
            / "kernel/.agents/skills/manage-investigation/references/investigation-reconciliation.md"
        ).read_text(encoding="utf-8")
        self.assertIn("even when Git reports a clean textual merge", runbook)
        self.assertIn("common base", runbook)
        self.assertIn("contradictory", runbook)
        self.assertIn("explicit resolutions", runbook)
        self.assertIn("SHA-256 of canonical current", runbook)

        audit_text = (DIST / "kernel/90-Meta/audit-vault.py").read_text(encoding="utf-8")
        self.assertIn('investigations[/\\\\]', audit_text)

    def test_investigation_allows_shared_worktree_only_for_same_repository(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-investigation/scripts/investigation-case.py"
        )
        module_name = "investigation_case_shared_worktree_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        worktree = "issue/abc-123-change"
        tracker_url = "https://tracker.example.com"
        tracker_id = "delivery"
        provider = "example"

        def entry(number: int, story: str, reference: str, remote: str) -> tuple[str, dict[str, str]]:
            repository = remote.rsplit("/", 1)[-1]
            handoff_id = hashlib.sha256(
                "|".join((tracker_id, reference, remote)).encode("utf-8")
            ).hexdigest()
            token = f"{tracker_id}-{reference.casefold()}-{handoff_id[:10]}"
            fields = {
                "dh": f"DH-{number:03d}",
                "story-id": story,
                "tracker-id": tracker_id,
                "provider": provider,
                "tracker-url": tracker_url,
                "work-item-reference": reference,
                "repository-remote": remote,
                "branch": worktree,
                "handoff-id": handoff_id,
                "family": f"{token}--{repository}",
                "revision": "v0001",
                "materialized-at": f"2026-08-31T12:0{number}:00+00:00",
            }
            register = (
                f"### {fields['dh']} — {tracker_id}:{reference} / {repository}\n\n"
                f"- Story ID: {story}\n"
                f"- Tracker ID: {tracker_id}\n"
                f"- Provider: {provider}\n"
                f"- Tracker URL: {tracker_url}\n"
                f"- Work item reference: {reference}\n"
                f"- Repository remote: {remote}\n"
                f"- Branch: {worktree}\n"
                f"- Handoff ID: {handoff_id}\n"
                f"- Family: {fields['family']}\n"
                "- Revision: v0001\n"
                f"- Materialized at: {fields['materialized-at']}\n"
            )
            return register, fields

        def history(fields: dict[str, str]) -> str:
            marker = json.dumps(fields, sort_keys=True, separators=(",", ":"))
            return (
                f"- {fields['materialized-at']} — bound development handoff "
                f"`{fields['dh']}`; story `{fields['story-id']}`; work item "
                f"`{fields['tracker-id']}:{fields['work-item-reference']}`; repository `{fields['repository-remote']}`; "
                f"branch `{fields['branch']}`; handoff "
                f"`{fields['handoff-id']}`; revision `v0001`.\n"
                f"  <!-- knowledge-os:development-handoff-binding {marker} -->\n"
            )

        first_register, first = entry(
            1, "S-001", "ABC-123", "example.invalid/team/repository"
        )
        second_register, second = entry(
            2, "S-002", "ABC-124", "example.invalid/team/repository"
        )
        same_repository = (
            "## Development handoffs\n\n"
            f"{first_register}\n{second_register}\n"
            "## History\n\n"
            f"{history(first)}{history(second)}"
        )
        self.assertEqual(module.validate_development_handoffs(same_repository), [])

        other_register, other = entry(
            2, "S-002", "ABC-124", "example.invalid/team/other-repository"
        )
        different_repository = (
            "## Development handoffs\n\n"
            f"{first_register}\n{other_register}\n"
            "## History\n\n"
            f"{history(first)}{history(other)}"
        )
        errors = module.validate_development_handoffs(different_repository)
        self.assertIn(
            "DH-002 reuses a Branch for a different Repository remote",
            errors,
        )

    def test_investigation_validator_requires_the_handoff_register(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-investigation/scripts/investigation-case.py"
        )
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "investigations"
            root.mkdir()
            self.assertEqual(run(["git", "init", "-q", tmp]).returncode, 0)
            self.assertEqual(run(["git", "-C", tmp, "config", "user.name", "Test Recorder"]).returncode, 0)
            self.assertEqual(run(["git", "-C", tmp, "config", "user.email", "recorder@example.invalid"]).returncode, 0)
            opened = run(
                [
                    sys.executable,
                    "-B",
                    str(helper),
                    "--root",
                    str(root),
                    "open",
                    "--id",
                    "20260826-120000-responsibility-boundary",
                    "--title",
                    "Responsibility boundary",
                    "--objective",
                    "Keep reconciliation in the vault",
                    "--request-summary",
                    "Preserve the vault-side reconciliation boundary.",
                    "--dedupe-key",
                    "responsibility-boundary",
                    "--source-ref",
                    "test",
                    "--purpose",
                    "development",
                    "--vault-outcome",
                    "deferred-until-production",
                    "--learning-outcome",
                    "not-evaluated",
                ]
            )
            self.assertEqual(opened.returncode, 0, opened.stdout + opened.stderr)
            valid = run([sys.executable, "-B", str(helper), "--root", str(root), "validate"])
            self.assertEqual(valid.returncode, 0, valid.stdout + valid.stderr)

            case = (
                root
                / "20260826-120000-responsibility-boundary"
                / "investigation.md"
            )
            original = case.read_text(encoding="utf-8")
            worktree = "issue/abc-123-change"
            tracker_id = "delivery"
            provider = "example"
            tracker_url = "https://tracker.example.com"
            reference = "ABC-123"
            handoff_id = hashlib.sha256(
                b"delivery|ABC-123|example.invalid/team/repository"
            ).hexdigest()
            family = f"delivery-abc-123-{handoff_id[:10]}--repository"
            valid_register = f"""## Development handoffs

### DH-001 — delivery:ABC-123 / repository

- Story ID: S-001
- Tracker ID: delivery
- Provider: example
- Tracker URL: https://tracker.example.com
- Work item reference: ABC-123
- Repository remote: example.invalid/team/repository
- Branch: {worktree}
- Handoff ID: {handoff_id}
- Family: {family}
- Revision: v0002
- Materialized at: 2026-08-26T12:00:00+00:00
"""
            register_without_history = original.replace(
                "## Development handoffs\n",
                valid_register,
                1,
            )
            case.write_text(
                register_without_history,
                encoding="utf-8",
            )
            missing_history = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                missing_history.returncode,
                2,
                missing_history.stdout + missing_history.stderr,
            )
            self.assertIn(
                "History must contain exactly one current binding for DH-001",
                missing_history.stderr,
            )

            def history_marker(revision: str, materialized_at: str) -> str:
                binding = {
                    "dh": "DH-001",
                    "story-id": "S-001",
                    "tracker-id": tracker_id,
                    "provider": provider,
                    "tracker-url": tracker_url,
                    "work-item-reference": reference,
                    "repository-remote": "example.invalid/team/repository",
                    "branch": str(worktree),
                    "handoff-id": handoff_id,
                    "family": family,
                    "revision": revision,
                    "materialized-at": materialized_at,
                }
                return (
                    "  <!-- knowledge-os:development-handoff-binding "
                    f"{json.dumps(binding, sort_keys=True, separators=(',', ':'))} -->"
                )

            def history_event(
                action: str,
                revision: str,
                materialized_at: str,
                marker: str | None = None,
            ) -> str:
                return (
                    f"- {materialized_at} — {action} development handoff `DH-001`; "
                    "story `S-001`; work item `delivery:ABC-123`; repository "
                    "`example.invalid/team/repository`; branch "
                    f"`{worktree}`; handoff `{handoff_id}`; revision `{revision}`.\n"
                    f"{marker or history_marker(revision, materialized_at)}\n"
                )

            current_only_history = (
                "## History\n\n"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    current_only_history,
                    1,
                ),
                encoding="utf-8",
            )
            missing_revision = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                missing_revision.returncode,
                2,
                missing_revision.stdout + missing_revision.stderr,
            )
            self.assertIn(
                "History bindings for DH-001 must cover each Revision from v0001",
                missing_revision.stderr,
            )

            history = (
                "## History\n\n"
                f"{history_event('bound', 'v0001', '2026-08-26T11:00:00+00:00')}"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
            )
            valid_case = register_without_history.replace("## History\n", history, 1)
            case.write_text(valid_case, encoding="utf-8")
            valid_register_result = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                valid_register_result.returncode,
                0,
                valid_register_result.stdout + valid_register_result.stderr,
            )

            marker_outside_history = valid_case.replace(
                "## Current state\n",
                "## Current state\n\n"
                f"{history_marker('v0002', '2026-08-26T12:00:00+00:00')}\n",
                1,
            )
            case.write_text(marker_outside_history, encoding="utf-8")
            outside_history = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                outside_history.returncode,
                2,
                outside_history.stdout + outside_history.stderr,
            )
            self.assertIn(
                "Development handoff binding markers must appear only in History",
                outside_history.stderr,
            )

            noncompact_marker = history_marker(
                "v0001",
                "2026-08-26T11:00:00+00:00",
            ).replace('"dh":', '"dh": ', 1)
            noncompact_history = (
                "## History\n\n"
                + history_event(
                    "bound",
                    "v0001",
                    "2026-08-26T11:00:00+00:00",
                    noncompact_marker,
                )
                + history_event(
                    "advanced",
                    "v0002",
                    "2026-08-26T12:00:00+00:00",
                )
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    noncompact_history,
                    1,
                ),
                encoding="utf-8",
            )
            noncompact = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                noncompact.returncode,
                2,
                noncompact.stdout + noncompact.stderr,
            )
            self.assertIn(
                "must use canonical compact JSON",
                noncompact.stderr,
            )

            missing_action_history = (
                "## History\n\n"
                f"{history_event('', 'v0001', '2026-08-26T11:00:00+00:00')}"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    missing_action_history,
                    1,
                ),
                encoding="utf-8",
            )
            missing_action = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                missing_action.returncode,
                2,
                missing_action.stdout + missing_action.stderr,
            )
            self.assertIn(
                "must name an action before its development handoff target",
                missing_action.stderr,
            )

            reverse_chronology_history = (
                "## History\n\n"
                f"{history_event('bound', 'v0001', '2026-08-26T13:00:00+00:00')}"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    reverse_chronology_history,
                    1,
                ),
                encoding="utf-8",
            )
            reverse_chronology = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                reverse_chronology.returncode,
                2,
                reverse_chronology.stdout + reverse_chronology.stderr,
            )
            self.assertIn(
                "Materialized at timestamps must preserve Revision chronology",
                reverse_chronology.stderr,
            )

            standalone_marker_history = (
                "## History\n\n"
                f"{history_marker('v0001', '2026-08-26T11:00:00+00:00')}\n"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    standalone_marker_history,
                    1,
                ),
                encoding="utf-8",
            )
            standalone_marker = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                standalone_marker.returncode,
                2,
                standalone_marker.stdout + standalone_marker.stderr,
            )
            self.assertIn(
                "must belong to one human-readable History event",
                standalone_marker.stderr,
            )

            incomplete_event_history = (
                "## History\n\n"
                "- 2026-08-26T11:00:00+00:00 — Development handoff `DH-001` "
                "bound at `v0001`.\n"
                f"{history_marker('v0001', '2026-08-26T11:00:00+00:00')}\n"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    incomplete_event_history,
                    1,
                ),
                encoding="utf-8",
            )
            incomplete_event = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                incomplete_event.returncode,
                2,
                incomplete_event.stdout + incomplete_event.stderr,
            )
            self.assertIn(
                "does not name its exact binding target",
                incomplete_event.stderr,
            )

            role_collision_history = (
                "## History\n\n"
                + history_event(
                    "bound",
                    "v0001",
                    "2026-08-26T11:00:00+00:00",
                ).replace("work item `delivery:ABC-123`; ", "", 1)
                + history_event(
                    "advanced",
                    "v0002",
                    "2026-08-26T12:00:00+00:00",
                )
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    role_collision_history,
                    1,
                ),
                encoding="utf-8",
            )
            role_collision = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                role_collision.returncode,
                2,
                role_collision.stdout + role_collision.stderr,
            )
            self.assertIn(
                "does not name its exact binding target",
                role_collision.stderr,
            )

            duplicate_key_marker = history_marker(
                "v0001",
                "2026-08-26T11:00:00+00:00",
            ).replace(
                '"dh":"DH-001"',
                '"dh":"DH-999","dh":"DH-001"',
                1,
            )
            duplicate_key_history = (
                "## History\n\n"
                + history_event(
                    "bound",
                    "v0001",
                    "2026-08-26T11:00:00+00:00",
                    duplicate_key_marker,
                )
                + history_event(
                    "advanced",
                    "v0002",
                    "2026-08-26T12:00:00+00:00",
                )
            )
            case.write_text(
                register_without_history.replace(
                    "## History\n",
                    duplicate_key_history,
                    1,
                ),
                encoding="utf-8",
            )
            duplicate_key = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                duplicate_key.returncode,
                2,
                duplicate_key.stdout + duplicate_key.stderr,
            )
            self.assertIn(
                "duplicate keys",
                duplicate_key.stderr,
            )

            reversed_history = (
                "## History\n\n"
                f"{history_event('advanced', 'v0002', '2026-08-26T12:00:00+00:00')}"
                f"{history_event('bound', 'v0001', '2026-08-26T11:00:00+00:00')}"
            )
            case.write_text(
                register_without_history.replace("## History\n", reversed_history, 1),
                encoding="utf-8",
            )
            reversed_result = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                reversed_result.returncode,
                2,
                reversed_result.stdout + reversed_result.stderr,
            )
            self.assertIn(
                "must appear in Revision order from v0001",
                reversed_result.stderr,
            )

            case.write_text(
                original.replace(
                    "## Development handoffs\n",
                    "## Development handoffs\n\n"
                    "### DH-001 — broken\n\n"
                    "- Story ID: S-001\n",
                    1,
                ),
                encoding="utf-8",
            )
            incomplete = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(incomplete.returncode, 2, incomplete.stdout + incomplete.stderr)
            self.assertIn("DH-001 is missing fields", incomplete.stderr)

            case.write_text(
                original.replace(
                    "## Development handoffs\n",
                    valid_register.replace(
                        "https://tracker.example.com",
                        "https://tracker.example.com:not-a-port",
                    ),
                    1,
                ),
                encoding="utf-8",
            )
            invalid_site = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                invalid_site.returncode,
                2,
                invalid_site.stdout + invalid_site.stderr,
            )
            self.assertIn("invalid canonical Tracker URL", invalid_site.stderr)

            case.write_text(
                register_without_history.replace(
                    "- Revision: v0002",
                    "- Revision: vabc",
                    1,
                ),
                encoding="utf-8",
            )
            invalid_revision = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(
                invalid_revision.returncode,
                2,
                invalid_revision.stdout + invalid_revision.stderr,
            )
            self.assertIn("invalid Revision", invalid_revision.stderr)

            case.write_text(
                original.replace(
                    "## Development handoffs\n\n",
                    "",
                    1,
                ),
                encoding="utf-8",
            )
            invalid = run(
                [sys.executable, "-B", str(helper), "--root", str(root), "validate"]
            )
            self.assertEqual(invalid.returncode, 2, invalid.stdout + invalid.stderr)
            self.assertIn("missing sections Development handoffs", invalid.stderr)

    def test_set_state_requires_exact_target_handoff_and_state(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        result = run(
            [
                sys.executable,
                "-B",
                str(helper),
                "--vault-root",
                str(DIST),
                "set-state",
                "--repository-remote",
                "https://example.invalid/example/repository.git",
            ]
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        error = json.loads(result.stderr)["error"]
        self.assertEqual(error["code"], "usage_error")
        self.assertIn("--worktree-path", error["message"])
        self.assertIn("--handoff-id", error["message"])
        self.assertIn("--state", error["message"])

    def test_state_updates_bind_the_selected_handoff_and_snapshot(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        module_name = "development_handoff_bootstrap_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        with self.assertRaises(module.HandoffError) as obsolete:
            module.managed_section_bounds(
                "<!-- BEGIN MANAGED: System A-System B DEVELOPMENT HANDOFF -->\n"
                "<!-- END MANAGED: System A-System B DEVELOPMENT HANDOFF -->",
                label="AGENTS.md",
            )
        self.assertEqual(obsolete.exception.code, "unsupported_managed_block_version")

        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "worktree"
            store = target / ".knowledge-os-handoffs"
            family_path = store / "abc-123--repository"
            family_path.mkdir(parents=True)
            entry = {
                "handoff-id": "a" * 64,
                "family": family_path.name,
                "revision": "v0002",
                "manifest": f"{family_path.name}/handoff.yaml",
                "activated-at": "2026-08-31T12:00:00Z",
                "state": "active",
            }
            active = {
                "schema-version": 2,
                "investigation-id": "20260831-shared-worktree",
                "handoffs": [entry],
            }
            anchor_id = hashlib.sha256(
                b"delivery|ABC-123|example.invalid/team/repository"
            ).hexdigest()
            replacements = {
                "resolve_handoff_target": mock.Mock(
                    return_value=(
                        target,
                        "example.invalid/team/repository",
                        f"issue/delivery-abc-123-{anchor_id[:10]}-change",
                    )
                ),
                "prepare_instructions": mock.Mock(return_value=({}, {})),
                "read_active": mock.Mock(return_value=(active, b"active")),
                "read_existing_handoff": mock.Mock(
                    return_value=SimpleNamespace(
                        revision="v0002",
                        family_path=family_path,
                        manifest={
                            "source": {"investigation-id": active["investigation-id"]},
                            "work-item": {
                                "tracker-id": "delivery",
                                "reference": "ABC-123",
                            },
                        },
                    )
                ),
                "validate_implementation_updates": mock.Mock(return_value={}),
                "closure_fingerprint": mock.Mock(return_value="b" * 64),
                "read_asset": mock.Mock(return_value=b"managed-policy"),
            }
            with mock.patch.multiple(module, **replacements):
                with self.assertRaises(module.HandoffError) as raised:
                    module.build_set_state_plan(
                        DIST,
                        "https://example.invalid/team/repository.git",
                        str(target),
                        entry["handoff-id"],
                        "ready-for-production",
                    )
                self.assertEqual(
                    raised.exception.code, "reconciliation_fingerprint_required"
                )

                ready = module.build_set_state_plan(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    str(target),
                    entry["handoff-id"],
                    "ready-for-production",
                    "b" * 64,
                )
                reopened = module.build_set_state_plan(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    str(target),
                    entry["handoff-id"],
                    "active",
                )
                self.assertEqual(ready["action"], "set-state")
                self.assertEqual(ready["state"], "ready-for-production")
                self.assertEqual(reopened["action"], "noop")
                self.assertNotEqual(ready["plan_token"], reopened["plan_token"])

                with self.assertRaises(module.HandoffError) as skipped:
                    module.build_set_state_plan(
                        DIST,
                        "https://example.invalid/team/repository.git",
                        str(target),
                        entry["handoff-id"],
                        "production",
                    )
                self.assertEqual(skipped.exception.code, "invalid_state_transition")

                replacements["closure_fingerprint"].return_value = "c" * 64
                with self.assertRaises(module.HandoffError) as changed:
                    module.build_set_state_plan(
                        DIST,
                        "https://example.invalid/team/repository.git",
                        str(target),
                        entry["handoff-id"],
                        "ready-for-production",
                        "b" * 64,
                    )
                self.assertEqual(changed.exception.code, "reconciliation_snapshot_mismatch")

    def test_active_registry_accepts_multiple_handoffs_and_rejects_bad_state(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        module_name = "development_handoff_active_registry_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        with tempfile.TemporaryDirectory() as tmp:
            store = Path(tmp) / ".knowledge-os-handoffs"
            store.mkdir()
            investigation = "20260831-shared-worktree"
            first = SimpleNamespace(
                active=None,
                handoff_id="a" * 64,
                family="abc-101--repository",
                bundle=SimpleNamespace(source={"investigation-id": investigation}),
            )
            first_raw = module.active_bytes(
                plan=first,
                revision="v0001",
                activated_at="2026-08-31T12:00:00Z",
            )
            (store / "ACTIVE.yaml").write_bytes(first_raw)
            active, _ = module.read_active(store)
            self.assertEqual(active["schema-version"], 2)
            self.assertEqual(active["handoffs"][0]["state"], "active")

            first.active = active
            reactivated_raw = module.active_bytes(
                plan=first,
                revision="v0001",
                activated_at="2026-08-31T12:02:00Z",
            )
            (store / "ACTIVE.yaml").write_bytes(reactivated_raw)
            active, _ = module.read_active(store)
            self.assertEqual(
                active["handoffs"][0]["activated-at"],
                "2026-08-31T12:02:00Z",
            )

            second = SimpleNamespace(
                active=active,
                handoff_id="b" * 64,
                family="abc-102--repository",
                bundle=SimpleNamespace(source={"investigation-id": investigation}),
            )
            second_raw = module.active_bytes(
                plan=second,
                revision="v0001",
                activated_at="2026-08-31T12:01:00Z",
            )
            (store / "ACTIVE.yaml").write_bytes(second_raw)
            shared, _ = module.read_active(store)
            self.assertEqual(len(shared["handoffs"]), 2)
            self.assertEqual(
                [item["family"] for item in shared["handoffs"]],
                ["abc-101--repository", "abc-102--repository"],
            )

            shared["handoffs"][1]["state"] = "unknown"
            (store / "ACTIVE.yaml").write_bytes(module.yaml_bytes(shared))
            with self.assertRaises(module.HandoffError) as invalid:
                module.read_active(store)
            self.assertEqual(invalid.exception.code, "invalid_handoff_state")

    def test_existing_worktree_accepts_only_same_investigation_packages(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        module_name = "development_handoff_shared_worktree_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        investigation = "20260831-shared-worktree"
        active = {
            "schema-version": 2,
            "investigation-id": investigation,
            "handoffs": [
                {
                    "handoff-id": "a" * 64,
                    "family": "abc-101--repository",
                    "revision": "v0001",
                    "manifest": "abc-101--repository/handoff.yaml",
                    "activated-at": "2026-08-31T12:00:00Z",
                    "state": "active",
                }
            ],
        }
        bundle = SimpleNamespace(
            source={"investigation-id": investigation},
            work_item={"tracker-id": "delivery", "reference": "ABC-102"},
            documents={},
            fingerprint="bundle",
        )
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "worktree"
            store = target / ".knowledge-os-handoffs"
            store.mkdir(parents=True)
            (store / "ACTIVE.yaml").write_bytes(module.yaml_bytes(active))

            def existing(family_path: Path, **_: object) -> SimpleNamespace | None:
                if family_path.name != "abc-101--repository":
                    return None
                return SimpleNamespace(
                    revision="v0001",
                    family_path=family_path,
                    manifest={
                        "source": {"investigation-id": investigation},
                        "work-item": {
                            "tracker-id": "delivery",
                            "reference": "ABC-101",
                        },
                    },
                )

            replacements = {
                "read_active": mock.Mock(return_value=(active, b"active")),
                "read_existing_handoff": mock.Mock(side_effect=existing),
                "prepare_instructions": mock.Mock(return_value=({}, {})),
                "prepare_gitignore": mock.Mock(return_value=(b"", "none")),
                "prepare_implementation_updates": mock.Mock(
                    return_value=("create", {"entries": 0})
                ),
                "validate_implementation_updates": mock.Mock(return_value={}),
                "verify_ignored": mock.Mock(),
                "rev_parse_optional": mock.Mock(return_value="c" * 40),
                "read_asset": mock.Mock(return_value=b"asset"),
                "handoff_identity": mock.Mock(
                    return_value=(
                        "abc-102--repository",
                        "b" * 64,
                        "delivery-abc-102-token",
                    )
                ),
            }
            anchor_id = hashlib.sha256(
                b"delivery|ABC-101|example.invalid/team/repository"
            ).hexdigest()
            anchor_branch = f"issue/delivery-abc-101-{anchor_id[:10]}-first-story"
            second_id = hashlib.sha256(
                b"delivery|ABC-102|example.invalid/team/repository"
            ).hexdigest()
            second_branch = f"issue/delivery-abc-102-{second_id[:10]}-second-story"
            with mock.patch.multiple(module, **replacements):
                plan = module.build_apply_plan_from_state(
                    bundle,
                    state_root=target,
                    target=target,
                    normalized_remote="example.invalid/team/repository",
                    branch=anchor_branch,
                    explicit_worktree=True,
                )
                self.assertEqual(plan.output["action"], "create")
                self.assertEqual(plan.active["investigation-id"], investigation)

                with self.assertRaises(module.HandoffError) as wrong_branch:
                    module.build_apply_plan_from_state(
                        bundle,
                        state_root=target,
                        target=target,
                        normalized_remote="example.invalid/team/repository",
                        branch="issue/unrelated",
                        explicit_worktree=True,
                    )
                self.assertEqual(
                    wrong_branch.exception.code, "worktree_branch_mismatch"
                )

                third_bundle = SimpleNamespace(
                    source={"investigation-id": investigation},
                    work_item={"tracker-id": "delivery", "reference": "ABC-103"},
                    documents={},
                    fingerprint="third-bundle",
                )
                replacements["handoff_identity"].return_value = (
                    "abc-103--repository",
                    "c" * 64,
                    "delivery-abc-103-token",
                )
                third_before_attach = module.build_apply_plan_from_state(
                    third_bundle,
                    state_root=target,
                    target=target,
                    normalized_remote="example.invalid/team/repository",
                    branch=anchor_branch,
                    explicit_worktree=True,
                )
                active_after_attach = {
                    **active,
                    "handoffs": [
                        *active["handoffs"],
                        {
                            "handoff-id": "b" * 64,
                            "family": "abc-102--repository",
                            "revision": "v0001",
                            "manifest": "abc-102--repository/handoff.yaml",
                            "activated-at": "2026-08-31T12:05:00Z",
                            "state": "active",
                        },
                    ],
                }
                (store / "ACTIVE.yaml").write_bytes(
                    module.yaml_bytes(active_after_attach)
                )
                replacements["read_active"].return_value = (
                    active_after_attach,
                    b"active-after-attach",
                )

                def existing_after_attach(
                    family_path: Path, **_: object
                ) -> SimpleNamespace | None:
                    if family_path.name == "abc-103--repository":
                        return None
                    return SimpleNamespace(
                        revision="v0001",
                        family_path=family_path,
                        manifest={
                            "source": {"investigation-id": investigation},
                            "work-item": {
                                "tracker-id": "delivery",
                                "reference": "ABC-101",
                            },
                        },
                    )

                replacements["read_existing_handoff"].side_effect = (
                    existing_after_attach
                )
                third_after_attach = module.build_apply_plan_from_state(
                    third_bundle,
                    state_root=target,
                    target=target,
                    normalized_remote="example.invalid/team/repository",
                    branch=anchor_branch,
                    explicit_worktree=True,
                )
                self.assertNotEqual(
                    third_before_attach.output["plan_token"],
                    third_after_attach.output["plan_token"],
                )

                replacements["read_existing_handoff"].side_effect = None
                replacements["read_existing_handoff"].return_value = None
                with self.assertRaises(module.HandoffError) as invalid_registry:
                    module.build_apply_plan_from_state(
                        bundle,
                        state_root=target,
                        target=target,
                        normalized_remote="example.invalid/team/repository",
                        branch=anchor_branch,
                        explicit_worktree=True,
                    )
                self.assertEqual(
                    invalid_registry.exception.code, "invalid_active_pointer"
                )

                replacements["read_existing_handoff"].side_effect = existing

                bundle.source = {"investigation-id": "20260831-other"}
                with self.assertRaises(module.HandoffError) as mismatch:
                    module.build_apply_plan_from_state(
                        bundle,
                        state_root=target,
                        target=target,
                        normalized_remote="example.invalid/team/repository",
                        branch=anchor_branch,
                        explicit_worktree=True,
                    )
                self.assertEqual(
                    mismatch.exception.code, "worktree_investigation_mismatch"
                )

                bundle.source = {"investigation-id": investigation}
                replacements["read_active"].return_value = (None, None)
                replacements["handoff_identity"].return_value = (
                    "abc-102--repository",
                    "b" * 64,
                    f"delivery-abc-102-{second_id[:10]}",
                )
                replacements["read_existing_handoff"].side_effect = lambda path, **_: (
                    SimpleNamespace(
                        revision="v0001",
                        family_path=path,
                        manifest={
                            "source": {"investigation-id": "20260831-previous"},
                            "work-item": {
                                "tracker-id": "delivery",
                                "reference": "ABC-102",
                            },
                        },
                    )
                )
                with self.assertRaises(module.HandoffError) as orphan:
                    module.build_apply_plan_from_state(
                        bundle,
                        state_root=target,
                        target=target,
                        normalized_remote="example.invalid/team/repository",
                        branch=second_branch,
                        explicit_worktree=True,
                    )
                self.assertEqual(
                    orphan.exception.code, "worktree_investigation_mismatch"
                )

    def test_shared_handoff_apply_adopts_v1_and_serializes_store_writes(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        module_name = "development_handoff_real_shared_flow_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            target = root / "worktree"
            target.mkdir()
            self.assertEqual(
                run(["git", "init", "-q", "-b", "issue/ABC-101-first"], cwd=target).returncode,
                0,
            )
            (target / ".gitignore").write_text(
                "/.knowledge-os-handoffs/\n", encoding="utf-8"
            )
            (target / "README.md").write_text("# Repository\n", encoding="utf-8")
            self.assertEqual(run(["git", "add", "."], cwd=target).returncode, 0)
            committed = run(
                [
                    "git",
                    "-c",
                    "user.name=Bootstrap Eval",
                    "-c",
                    "user.email=bootstrap@example.invalid",
                    "commit",
                    "-q",
                    "-m",
                    "baseline",
                ],
                cwd=target,
            )
            self.assertEqual(committed.returncode, 0, committed.stderr)

            remote = "https://example.invalid/team/repository.git"
            normalized_remote = "example.invalid/team/repository"
            investigation = "20260831-shared-worktree"

            def write_bundle(issue_key: str, story_id: str) -> Path:
                bundle = root / issue_key.casefold()
                bundle.mkdir()
                (bundle / "bundle.yaml").write_text(
                    "\n".join(
                        (
                            "schema-version: 2",
                            "source:",
                            f'  investigation-id: "{investigation}"',
                            '  investigation-updated-at: "2026-08-31T12:00:00Z"',
                            f'  story-id: "{story_id}"',
                            "work-item:",
                            '  tracker-id: "delivery"',
                            '  provider: "example"',
                            '  tracker-url: "https://tracker.example.com"',
                            f'  reference: "{issue_key}"',
                            f'  url: "https://tracker.example.com/items/{issue_key}"',
                            '  updated-at: "2026-08-31T11:55:00Z"',
                            '  captured-at: "2026-08-31T12:01:00Z"',
                            '  freshness: "current"',
                            '  snapshot-source: "connected-readback"',
                            "repository:",
                            f'  remote: "{remote}"',
                            "change:",
                            f'  summary: "Implement {issue_key}."',
                            "",
                        )
                    ),
                    encoding="utf-8",
                )
                (bundle / "work-item.md").write_text(
                    f"# Work item\n\n{issue_key}\n", encoding="utf-8"
                )
                (bundle / "context.md").write_text(
                    f"# Context\n\nContext for {issue_key}.\n", encoding="utf-8"
                )
                (bundle / "scope.md").write_text(
                    f"# Scope\n\nScope for {issue_key}.\n", encoding="utf-8"
                )
                return bundle

            first_bundle = write_bundle("ABC-101", "S-001")
            second_bundle = write_bundle("ABC-102", "S-002")
            first_id = hashlib.sha256(
                f"delivery|ABC-101|{normalized_remote}".encode("utf-8")
            ).hexdigest()
            second_id = hashlib.sha256(
                f"delivery|ABC-102|{normalized_remote}".encode("utf-8")
            ).hexdigest()
            first_family_name = f"delivery-abc-101-{first_id[:10]}--repository"
            second_family_name = f"delivery-abc-102-{second_id[:10]}--repository"
            resolved = mock.Mock(
                return_value=(
                    target,
                    normalized_remote,
                    f"issue/delivery-abc-101-{first_id[:10]}-first",
                )
            )
            with (
                mock.patch.object(module, "resolve_handoff_target", resolved),
                mock.patch.object(module, "validate_tracker_binding"),
            ):
                first_plan = module.build_apply_plan(
                    DIST, str(first_bundle), str(target)
                )
                original_atomic_write = module.atomic_write
                first_interrupted = False

                def interrupt_first(path: Path, raw: bytes) -> None:
                    nonlocal first_interrupted
                    if (
                        not first_interrupted
                        and path == target / ".knowledge-os-handoffs/ACTIVE.yaml"
                    ):
                        first_interrupted = True
                        raise KeyboardInterrupt()
                    original_atomic_write(path, raw)

                with mock.patch.object(
                    module, "atomic_write", side_effect=interrupt_first
                ):
                    with self.assertRaises(KeyboardInterrupt):
                        module.apply_plan(
                            DIST,
                            str(first_bundle),
                            first_plan.output["plan_token"],
                            str(target),
                        )
                self.assertTrue((target / "AGENTS.md").is_file())
                interrupted_agents = (target / "AGENTS.md").read_bytes()
                (target / "AGENTS.md").write_bytes(
                    interrupted_agents + b"\nHuman edit after interruption.\n"
                )
                with self.assertRaises(module.HandoffError) as root_conflict:
                    module.apply_plan(
                        DIST,
                        str(first_bundle),
                        first_plan.output["plan_token"],
                        str(target),
                    )
                self.assertEqual(
                    root_conflict.exception.code, "handoff_recovery_conflict"
                )
                self.assertTrue(
                    (target / "AGENTS.md")
                    .read_bytes()
                    .endswith(b"Human edit after interruption.\n")
                )
                self.assertTrue(
                    (
                        target
                        / ".knowledge-os-handoffs"
                        / module.TRANSACTION_NAME
                    ).is_dir()
                )
                (target / "AGENTS.md").write_bytes(interrupted_agents)
                first_result = module.apply_plan(
                    DIST,
                    str(first_bundle),
                    first_plan.output["plan_token"],
                    str(target),
                )
                self.assertEqual(first_result["status"], "applied")

                store = target / ".knowledge-os-handoffs"
                first_active, _ = module.read_active(store)
                first_entry = first_active["handoffs"][0]
                first_manifest_path = store / first_entry["manifest"]
                first_manifest_raw = first_manifest_path.read_bytes()
                malformed_manifest, _ = module.load_yaml_mapping(
                    first_manifest_path, label="handoff.yaml"
                )
                malformed_manifest["source"] = "invalid"
                first_manifest_path.write_bytes(module.yaml_bytes(malformed_manifest))
                with self.assertRaises(module.HandoffError) as malformed:
                    module.validate_repository(DIST, remote, worktree_path=str(target))
                self.assertEqual(malformed.exception.code, "invalid_type")
                first_manifest_path.write_bytes(first_manifest_raw)

                legacy_active = {
                    "schema-version": 1,
                    "handoff-id": first_entry["handoff-id"],
                    "family": first_entry["family"],
                    "revision": first_entry["revision"],
                    "manifest": first_entry["manifest"],
                    "activated-at": first_entry["activated-at"],
                }
                (store / "ACTIVE.yaml").write_bytes(module.yaml_bytes(legacy_active))

                legacy_validation = module.validate_repository(
                    DIST, remote, worktree_path=str(target)
                )
                self.assertEqual(legacy_validation["status"], "valid")
                self.assertEqual(len(legacy_validation["handoffs"]), 1)

                second_plan = module.build_apply_plan(
                    DIST, str(second_bundle), str(target)
                )
                second_result = module.apply_plan(
                    DIST,
                    str(second_bundle),
                    second_plan.output["plan_token"],
                    str(target),
                )
                self.assertEqual(second_result["status"], "applied")
                shared, _ = module.read_active(store)
                self.assertEqual(shared["schema-version"], 2)
                self.assertEqual(shared["investigation-id"], investigation)
                self.assertEqual(
                    [entry["family"] for entry in shared["handoffs"]],
                    [first_family_name, second_family_name],
                )
                self.assertTrue((store / first_family_name / "handoff.yaml").is_file())
                self.assertTrue((store / second_family_name / "handoff.yaml").is_file())

                second_family = store / second_family_name
                second_scope = second_bundle / "scope.md"

                def assert_failed_apply_restores(
                    content: str,
                    fail_path: Path,
                ) -> None:
                    second_scope.write_text(content, encoding="utf-8")
                    update_plan = module.build_apply_plan(
                        DIST, str(second_bundle), str(target)
                    )
                    family_before = module.path_fingerprint(second_family)
                    active_before = (store / "ACTIVE.yaml").read_bytes()
                    failed = False

                    def fail_once(path: Path, raw: bytes) -> None:
                        nonlocal failed
                        if not failed and path == fail_path:
                            failed = True
                            raise OSError("forced write failure")
                        original_atomic_write(path, raw)

                    with mock.patch.object(
                        module, "atomic_write", side_effect=fail_once
                    ):
                        with self.assertRaises(OSError):
                            module.apply_plan(
                                DIST,
                                str(second_bundle),
                                update_plan.output["plan_token"],
                                str(target),
                            )
                    self.assertTrue(failed)
                    self.assertFalse((store / module.TRANSACTION_NAME).exists())
                    self.assertEqual(
                        module.path_fingerprint(second_family), family_before
                    )
                    self.assertEqual(
                        (store / "ACTIVE.yaml").read_bytes(), active_before
                    )
                    retry = module.build_apply_plan(
                        DIST, str(second_bundle), str(target)
                    )
                    self.assertEqual(
                        retry.output["plan_token"],
                        update_plan.output["plan_token"],
                    )
                    applied = module.apply_plan(
                        DIST,
                        str(second_bundle),
                        retry.output["plan_token"],
                        str(target),
                    )
                    self.assertEqual(applied["status"], "applied")

                assert_failed_apply_restores(
                    "# Scope\n\nScope v2 for ABC-102.\n",
                    second_family / "handoff.yaml",
                )
                assert_failed_apply_restores(
                    "# Scope\n\nScope v3 for ABC-102.\n",
                    store / "ACTIVE.yaml",
                )

                second_scope.write_text(
                    "# Scope\n\nScope v4 for ABC-102.\n", encoding="utf-8"
                )
                interrupted_plan = module.build_apply_plan(
                    DIST, str(second_bundle), str(target)
                )
                interrupted = False

                def interrupt_once(path: Path, raw: bytes) -> None:
                    nonlocal interrupted
                    if not interrupted and path == store / "ACTIVE.yaml":
                        interrupted = True
                        raise KeyboardInterrupt()
                    original_atomic_write(path, raw)

                with mock.patch.object(
                    module, "atomic_write", side_effect=interrupt_once
                ):
                    with self.assertRaises(KeyboardInterrupt):
                        module.apply_plan(
                            DIST,
                            str(second_bundle),
                            interrupted_plan.output["plan_token"],
                            str(target),
                        )
                self.assertTrue((store / module.TRANSACTION_NAME).is_dir())
                with self.assertRaises(module.HandoffError) as pending:
                    module.build_apply_plan(DIST, str(second_bundle), str(target))
                self.assertEqual(
                    pending.exception.code, "handoff_recovery_required"
                )
                interrupted_family = module.path_fingerprint(second_family)
                interrupted_active = (store / "ACTIVE.yaml").read_bytes()
                with self.assertRaises(module.HandoffError) as wrong_token:
                    module.apply_plan(
                        DIST,
                        str(second_bundle),
                        "0" * 64,
                        str(target),
                    )
                self.assertEqual(wrong_token.exception.code, "plan_stale")
                self.assertTrue((store / module.TRANSACTION_NAME).is_dir())
                self.assertEqual(
                    module.path_fingerprint(second_family), interrupted_family
                )
                self.assertEqual(
                    (store / "ACTIVE.yaml").read_bytes(), interrupted_active
                )
                with self.assertRaises(module.HandoffError) as wrong_bundle:
                    module.apply_plan(
                        DIST,
                        str(first_bundle),
                        interrupted_plan.output["plan_token"],
                        str(target),
                    )
                self.assertEqual(wrong_bundle.exception.code, "plan_stale")
                self.assertTrue((store / module.TRANSACTION_NAME).is_dir())
                self.assertEqual(
                    module.path_fingerprint(second_family), interrupted_family
                )
                self.assertEqual(
                    (store / "ACTIVE.yaml").read_bytes(), interrupted_active
                )
                history = second_family / "history"
                saved_history = root / "saved-history"
                external_history = root / "external-history"
                history.rename(saved_history)
                external_history.mkdir()
                external_event = (
                    external_history
                    / f"{interrupted_plan.output['handoff']['revision']}.md"
                )
                external_event.write_text("external\n", encoding="utf-8")
                history.symlink_to(external_history, target_is_directory=True)
                with self.assertRaises(module.HandoffError) as unsafe_target:
                    module.apply_plan(
                        DIST,
                        str(second_bundle),
                        interrupted_plan.output["plan_token"],
                        str(target),
                    )
                self.assertEqual(
                    unsafe_target.exception.code, "unsafe_transaction_target"
                )
                self.assertEqual(
                    external_event.read_text(encoding="utf-8"), "external\n"
                )
                self.assertTrue((store / module.TRANSACTION_NAME).is_dir())
                history.unlink()
                saved_history.rename(history)
                recovered = module.apply_plan(
                    DIST,
                    str(second_bundle),
                    interrupted_plan.output["plan_token"],
                    str(target),
                )
                self.assertEqual(recovered["status"], "applied")
                self.assertFalse((store / module.TRANSACTION_NAME).exists())

                third_bundle = write_bundle("ABC-103", "S-003")
                third_id = hashlib.sha256(
                    f"delivery|ABC-103|{normalized_remote}".encode("utf-8")
                ).hexdigest()
                third_plan = module.build_apply_plan(
                    DIST, str(third_bundle), str(target)
                )
                third_interrupted = False

                def interrupt_third(path: Path, raw: bytes) -> None:
                    nonlocal third_interrupted
                    if not third_interrupted and path == store / "ACTIVE.yaml":
                        third_interrupted = True
                        raise KeyboardInterrupt()
                    original_atomic_write(path, raw)

                with mock.patch.object(
                    module, "atomic_write", side_effect=interrupt_third
                ):
                    with self.assertRaises(KeyboardInterrupt):
                        module.apply_plan(
                            DIST,
                            str(third_bundle),
                            third_plan.output["plan_token"],
                            str(target),
                        )
                third_family = (
                    store
                    / f"delivery-abc-103-{third_id[:10]}--repository"
                )
                diagnostic = third_family / "diagnostic.txt"
                diagnostic.write_text("preserve\n", encoding="utf-8")
                with self.assertRaises(module.HandoffError) as conflict:
                    module.apply_plan(
                        DIST,
                        str(third_bundle),
                        third_plan.output["plan_token"],
                        str(target),
                    )
                self.assertEqual(
                    conflict.exception.code, "handoff_recovery_conflict"
                )
                self.assertEqual(diagnostic.read_text(encoding="utf-8"), "preserve\n")
                self.assertTrue((store / module.TRANSACTION_NAME).is_dir())
                diagnostic.unlink()
                third_recovered = module.apply_plan(
                    DIST,
                    str(third_bundle),
                    third_plan.output["plan_token"],
                    str(target),
                )
                self.assertEqual(third_recovered["status"], "applied")
                self.assertTrue((third_family / "handoff.yaml").is_file())

            entered = threading.Event()

            def enter_same_store() -> None:
                with module.handoff_store_lock(target):
                    entered.set()

            with module.handoff_store_lock(target):
                waiter = threading.Thread(target=enter_same_store)
                waiter.start()
                self.assertFalse(entered.wait(0.1))
            self.assertTrue(entered.wait(1))
            waiter.join(timeout=1)
            self.assertFalse(waiter.is_alive())

            windows_lock = SimpleNamespace(
                LK_LOCK=1,
                LK_UNLCK=2,
                locking=mock.Mock(),
            )
            with (
                mock.patch.object(module.os, "name", "nt"),
                mock.patch.object(module, "msvcrt", windows_lock, create=True),
            ):
                with module.handoff_store_lock(target):
                    pass
            self.assertEqual(
                [call.args[1] for call in windows_lock.locking.call_args_list],
                [windows_lock.LK_LOCK, windows_lock.LK_UNLCK],
            )

            failed_windows_lock = SimpleNamespace(
                LK_LOCK=1,
                LK_UNLCK=2,
                locking=mock.Mock(side_effect=OSError("busy")),
            )
            real_close = module.os.close
            with (
                mock.patch.object(module.os, "name", "nt"),
                mock.patch.object(
                    module, "msvcrt", failed_windows_lock, create=True
                ),
                mock.patch.object(module.os, "close", wraps=real_close) as close,
            ):
                with self.assertRaises(OSError):
                    with module.handoff_store_lock(target):
                        self.fail("The body must not run without the lock")
            self.assertEqual(failed_windows_lock.locking.call_count, 1)
            self.assertEqual(
                failed_windows_lock.locking.call_args.args[1],
                failed_windows_lock.LK_LOCK,
            )
            close.assert_called_once()

    def test_multi_provider_handoffs_in_initialized_temp_cell(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cell = root / "cell"
            initialized = run(
                [
                    str(DIST / "install.sh"),
                    "init",
                    "--dest",
                    str(cell),
                    "--cell-name",
                    "multi-provider-eval",
                    "--purpose",
                    "integration test",
                    "--system",
                    "test-system",
                    "--tracker",
                    "clickup-main:clickup:https://clickup.example.com/team",
                    "--tracker",
                    "notion-main:notion:https://notion.example.com",
                    "--yes",
                ]
            )
            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            instance_path = cell / "instance.yaml"
            instance_path.write_text(
                instance_path.read_text(encoding="utf-8").replace(
                    'provider: "clickup"', 'provider: "ClickUp"', 1
                ),
                encoding="utf-8",
            )

            helper = (
                cell
                / ".agents/skills/manage-development-handoff/scripts/development-handoff.py"
            )
            module_name = "development_handoff_multi_provider_temp_eval"
            spec = importlib.util.spec_from_file_location(module_name, helper)
            self.assertIsNotNone(spec)
            self.assertIsNotNone(spec.loader)
            module = importlib.util.module_from_spec(spec)
            sys.modules[module_name] = module
            self.addCleanup(sys.modules.pop, module_name, None)
            spec.loader.exec_module(module)

            target = root / "repository"
            target.mkdir()
            normalized_remote = "example.invalid/team/repository"
            remote = f"https://{normalized_remote}.git"
            first_id = hashlib.sha256(
                f"clickup-main|TASK-7|{normalized_remote}".encode("utf-8")
            ).hexdigest()
            branch = f"issue/clickup-main-task-7-{first_id[:10]}-integration"
            self.assertEqual(
                run(["git", "init", "-q", "-b", branch], cwd=target).returncode,
                0,
            )
            (target / ".gitignore").write_text(
                "/.knowledge-os-handoffs/\n", encoding="utf-8"
            )
            (target / "README.md").write_text("# Temporary repository\n", encoding="utf-8")
            self.assertEqual(run(["git", "add", "."], cwd=target).returncode, 0)
            committed = run(
                [
                    "git",
                    "-c",
                    "user.name=Bootstrap Eval",
                    "-c",
                    "user.email=bootstrap@example.invalid",
                    "commit",
                    "-q",
                    "-m",
                    "baseline",
                ],
                cwd=target,
            )
            self.assertEqual(committed.returncode, 0, committed.stderr)

            def package(
                name: str,
                story: str,
                tracker_id: str,
                provider: str,
                tracker_url: str,
                item_url: str | None = None,
            ) -> Path:
                bundle = root / name
                bundle.mkdir()
                reference = "TASK-7"
                item_url = item_url or f"{tracker_url}/items/{reference}"
                (bundle / "bundle.yaml").write_text(
                    f'''schema-version: 2
source:
  investigation-id: "20260831-multi-provider"
  investigation-updated-at: "2026-08-31T12:00:00Z"
  story-id: "{story}"
work-item:
  tracker-id: "{tracker_id}"
  provider: "{provider}"
  tracker-url: "{tracker_url}"
  reference: "{reference}"
  url: "{item_url}"
  updated-at: "2026-08-31T11:55:00Z"
  captured-at: "2026-08-31T12:01:00Z"
  freshness: "current"
  snapshot-source: "connected-readback"
repository:
  remote: "{remote}"
change:
  summary: "Materialize {tracker_id}:{reference}."
''',
                    encoding="utf-8",
                )
                (bundle / "work-item.md").write_text(
                    f"# Work item\n\n{reference}\n", encoding="utf-8"
                )
                (bundle / "context.md").write_text(
                    f"# Context\n\n{tracker_id}\n", encoding="utf-8"
                )
                (bundle / "scope.md").write_text(
                    f"# Scope\n\n{provider}\n", encoding="utf-8"
                )
                return bundle

            bundles = (
                package(
                    "clickup-package",
                    "S-001",
                    "clickup-main",
                    "clickup",
                    "https://clickup.example.com/team",
                    "https://clickup.example.com/t/TASK-7",
                ),
                package(
                    "notion-package",
                    "S-002",
                    "notion-main",
                    "notion",
                    "https://notion.example.com",
                ),
            )
            resolved = mock.Mock(
                return_value=(target, normalized_remote, branch)
            )
            with mock.patch.object(module, "resolve_handoff_target", resolved):
                for bundle in bundles:
                    plan = module.build_apply_plan(cell, str(bundle), str(target))
                    applied = module.apply_plan(
                        cell,
                        str(bundle),
                        plan.output["plan_token"],
                        str(target),
                    )
                    self.assertEqual(applied["status"], "applied")
                validated = module.validate_repository(
                    cell, remote, worktree_path=str(target)
                )

            self.assertEqual(validated["status"], "valid")
            self.assertEqual(len(validated["handoffs"]), 2)
            store = target / ".knowledge-os-handoffs"
            active, _ = module.read_active(store)
            families = [item["family"] for item in active["handoffs"]]
            self.assertEqual(len(set(families)), 2)
            manifests = [
                module.load_yaml_mapping(store / item["manifest"], label="handoff.yaml")[0]
                for item in active["handoffs"]
            ]
            self.assertEqual(
                {manifest["work-item"]["provider"] for manifest in manifests},
                {"clickup", "notion"},
            )
            self.assertTrue(
                all((store / family / "work-item.md").is_file() for family in families)
            )
            instance_path.write_text(
                instance_path.read_text(encoding="utf-8")
                .replace('provider: "notion"', 'provider: "replacement"', 1)
                .replace(
                    'url: "https://notion.example.com"',
                    'url: "https://replacement.example.com"',
                    1,
                ),
                encoding="utf-8",
            )
            notion_metadata = bundles[1] / "bundle.yaml"
            notion_metadata.write_text(
                notion_metadata.read_text(encoding="utf-8")
                .replace('provider: "notion"', 'provider: "replacement"', 1)
                .replace(
                    "https://notion.example.com",
                    "https://replacement.example.com",
                ),
                encoding="utf-8",
            )
            with mock.patch.object(module, "resolve_handoff_target", resolved):
                with self.assertRaises(module.HandoffError) as rebound:
                    module.build_apply_plan(cell, str(bundles[1]), str(target))
            self.assertEqual(rebound.exception.code, "tracker_binding_mismatch")
            instance_path.rename(cell / "instance.yaml.saved")
            with mock.patch.object(module, "resolve_handoff_target", resolved):
                with self.assertRaises(module.HandoffError) as missing_instance:
                    module.build_apply_plan(cell, str(bundles[0]), str(target))
            self.assertEqual(
                missing_instance.exception.code,
                "invalid_tracker_configuration",
            )
            instance_path.write_text("cell: [\n", encoding="utf-8")
            with mock.patch.object(module, "resolve_handoff_target", resolved):
                with self.assertRaises(module.HandoffError) as malformed_instance:
                    module.build_apply_plan(cell, str(bundles[0]), str(target))
            self.assertEqual(
                malformed_instance.exception.code,
                "invalid_tracker_configuration",
            )
            self.assertEqual(
                malformed_instance.exception.details["reason"],
                "invalid instance.yaml syntax",
            )

    def test_state_flow_preserves_other_handoffs_and_reopens(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        module_name = "development_handoff_state_flow_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "worktree"
            store = target / ".knowledge-os-handoffs"
            first_family = store / "abc-101--repository"
            second_family = store / "abc-102--repository"
            first_family.mkdir(parents=True)
            second_family.mkdir()
            active = {
                "schema-version": 2,
                "investigation-id": "20260831-shared-worktree",
                "handoffs": [
                    {
                        "handoff-id": "a" * 64,
                        "family": first_family.name,
                        "revision": "v0001",
                        "manifest": f"{first_family.name}/handoff.yaml",
                        "activated-at": "2026-08-31T12:00:00Z",
                        "state": "active",
                    },
                    {
                        "handoff-id": "b" * 64,
                        "family": second_family.name,
                        "revision": "v0001",
                        "manifest": f"{second_family.name}/handoff.yaml",
                        "activated-at": "2026-08-31T12:01:00Z",
                        "state": "active",
                    },
                ],
            }
            (store / "ACTIVE.yaml").write_bytes(module.yaml_bytes(active))

            def existing(path: Path, **_: object) -> SimpleNamespace:
                return SimpleNamespace(
                    revision="v0001",
                    family_path=path,
                    manifest={
                        "source": {
                            "investigation-id": active["investigation-id"]
                        },
                        "work-item": {
                            "tracker-id": "delivery",
                            "reference": "ABC-101",
                        },
                    },
                )

            anchor_id = hashlib.sha256(
                b"delivery|ABC-101|example.invalid/team/repository"
            ).hexdigest()
            replacements = {
                "resolve_handoff_target": mock.Mock(
                    return_value=(
                        target,
                        "example.invalid/team/repository",
                        f"issue/delivery-abc-101-{anchor_id[:10]}-first-story",
                    )
                ),
                "prepare_instructions": mock.Mock(return_value=({}, {})),
                "read_existing_handoff": mock.Mock(side_effect=existing),
                "validate_implementation_updates": mock.Mock(return_value={}),
                "closure_fingerprint": mock.Mock(return_value="c" * 64),
                "read_asset": mock.Mock(return_value=b"managed-policy"),
                "now_utc": mock.Mock(return_value="2026-08-31T13:00:00Z"),
            }
            with mock.patch.multiple(module, **replacements):
                ready = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    None,
                    str(target),
                    "a" * 64,
                    "ready-for-production",
                    "c" * 64,
                )
                applied_ready = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    ready["plan_token"],
                    str(target),
                    "a" * 64,
                    "ready-for-production",
                    "c" * 64,
                )
                self.assertEqual(applied_ready["status"], "state-updated")
                observed, _ = module.read_active(store)
                self.assertEqual(observed["handoffs"][0]["state"], "ready-for-production")
                self.assertEqual(observed["handoffs"][1]["state"], "active")
                ready_noop = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    None,
                    str(target),
                    "a" * 64,
                    "ready-for-production",
                )
                self.assertEqual(ready_noop["action"], "noop")

                production = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    None,
                    str(target),
                    "a" * 64,
                    "production",
                )
                module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    production["plan_token"],
                    str(target),
                    "a" * 64,
                    "production",
                )
                production_noop = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    None,
                    str(target),
                    "a" * 64,
                    "production",
                )
                self.assertEqual(production_noop["action"], "noop")
                with self.assertRaises(module.HandoffError) as invalid_downgrade:
                    module.build_set_state_plan(
                        DIST,
                        "https://example.invalid/team/repository.git",
                        str(target),
                        "a" * 64,
                        "ready-for-production",
                        "c" * 64,
                    )
                self.assertEqual(
                    invalid_downgrade.exception.code,
                    "invalid_state_transition",
                )
                reopen = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    None,
                    str(target),
                    "a" * 64,
                    "active",
                )
                module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    reopen["plan_token"],
                    str(target),
                    "a" * 64,
                    "active",
                )
                reopened, _ = module.read_active(store)
                self.assertEqual(reopened["handoffs"][0]["state"], "active")
                self.assertEqual(
                    reopened["handoffs"][0]["activated-at"],
                    "2026-08-31T13:00:00Z",
                )
                self.assertEqual(reopened["handoffs"][1]["state"], "active")

                stale = module.set_state(
                    DIST,
                    "https://example.invalid/team/repository.git",
                    None,
                    str(target),
                    "a" * 64,
                    "ready-for-production",
                    "c" * 64,
                )
                reopened["handoffs"][1]["state"] = "ready-for-production"
                (store / "ACTIVE.yaml").write_bytes(module.yaml_bytes(reopened))
                with self.assertRaises(module.HandoffError) as changed:
                    module.set_state(
                        DIST,
                        "https://example.invalid/team/repository.git",
                        stale["plan_token"],
                        str(target),
                        "a" * 64,
                        "ready-for-production",
                        "c" * 64,
                    )
                self.assertEqual(changed.exception.code, "plan_stale")

    def test_closure_fingerprint_binds_handoff_and_repository_evidence(self) -> None:
        helper = (
            DIST
            / "kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py"
        )
        module_name = "development_handoff_closure_eval"
        spec = importlib.util.spec_from_file_location(module_name, helper)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        sys.modules[module_name] = module
        self.addCleanup(sys.modules.pop, module_name, None)
        spec.loader.exec_module(module)

        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "worktree"
            target.mkdir()
            self.assertEqual(run(["git", "init", "-q"], cwd=target).returncode, 0)
            self.assertEqual(
                run(["git", "config", "user.email", "review@example.invalid"], cwd=target).returncode,
                0,
            )
            self.assertEqual(
                run(["git", "config", "user.name", "Review"], cwd=target).returncode,
                0,
            )
            (target / ".gitignore").write_text("/.knowledge-os-handoffs/\n", encoding="utf-8")
            tracked = target / "implementation.txt"
            tracked.write_text("baseline\n", encoding="utf-8")
            self.assertEqual(run(["git", "add", "."], cwd=target).returncode, 0)
            self.assertEqual(
                run(["git", "commit", "-q", "-m", "baseline"], cwd=target).returncode,
                0,
            )

            family = target / ".knowledge-os-handoffs" / "abc-123--repository"
            family.mkdir(parents=True)
            updates = family / "implementation-updates.md"
            updates.write_text("# Implementation updates\n", encoding="utf-8")
            active = target / ".knowledge-os-handoffs" / "ACTIVE.yaml"
            active.write_text("revision: v0001\n", encoding="utf-8")

            baseline = module.closure_fingerprint(target, family, active)
            active.write_text("revision: v0002\n", encoding="utf-8")
            changed_active = module.closure_fingerprint(target, family, active)
            self.assertNotEqual(baseline, changed_active)
            active.write_text("revision: v0001\n", encoding="utf-8")

            updates.write_text("# Implementation updates\n\n## UPD-001 — Delta\n", encoding="utf-8")
            changed_handoff = module.closure_fingerprint(target, family, active)
            self.assertNotEqual(baseline, changed_handoff)

            tracked.write_text("changed\n", encoding="utf-8")
            changed_tracked = module.closure_fingerprint(target, family, active)
            self.assertNotEqual(changed_handoff, changed_tracked)

            untracked = target / "local-evidence.txt"
            untracked.write_text("one\n", encoding="utf-8")
            first_untracked = module.closure_fingerprint(target, family, active)
            untracked.write_text("two\n", encoding="utf-8")
            second_untracked = module.closure_fingerprint(target, family, active)
            self.assertNotEqual(first_untracked, second_untracked)

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

            lock_path = dest / ".knowledge-os.lock.yaml"
            lock = lock_path.read_text(encoding="utf-8")
            digest = hashlib.sha256(retired_bytes).hexdigest()
            lock_path.write_text(
                lock.replace(
                    "managed_hashes:\n",
                    f'managed_hashes:\n  "{retired_relative}": {digest}\n',
                    1,
                ),
                encoding="utf-8",
            )

            updated = run(["sh", str(INSTALL), "update", "--dest", str(dest)])
            self.assertEqual(updated.returncode, 0, updated.stdout + updated.stderr)
            self.assertFalse(retired.exists())
            self.assertTrue(cell_owned.is_file())
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
            DIST
            / "kernel/.agents/skills/manage-development-handoff/references/worktree-lifecycle.md",
            DIST
            / "kernel/.agents/skills/manage-development-handoff/references/repository-state.md",
            DIST
            / "kernel/.agents/skills/manage-investigation/references/record-contract.md",
            DIST / "kernel/.agents/skills/manage-operational-workflow/SKILL.md",
        ]
        combined = "\n".join(path.read_text(encoding="utf-8") for path in documents)
        self.assertNotIn("Legacy maintenance", combined)
        self.assertNotIn("path-less compatibility", combined)
        self.assertNotIn("when that resolver is unavailable", combined)
        self.assertNotIn("legacy", combined.casefold())

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
            self.assertFalse((dest / "AGENTS.personal.md").exists())
            self.assertTrue((dest / ".agents" / "skills" / "map-ecosystem" / "SKILL.md").is_file())
            git_skill = dest / ".agents" / "skills" / "manage-git-workflow"
            self.assertTrue((git_skill / "SKILL.md").is_file())
            self.assertTrue((git_skill / "references" / "defaults.md").is_file())
            self.assertFalse((dest / ".agents" / "skills" / "inspect-gcp-runtime").exists())
            for shared in (
                "resolve-vault.py",
                "vault-resolution.md",
                "node-selection.md",
                "work-item-evidence.md",
                "jira-evidence.md",
            ):
                self.assertTrue((dest / "90-Meta" / shared).is_file(), shared)
            self.assertFalse(
                (
                    dest
                    / ".agents/skills/map-ecosystem/scripts/resolve-vault.py"
                ).exists()
            )
            lock = (dest / ".knowledge-os.lock.yaml").read_text(encoding="utf-8")
            self.assertIn('version: "3"', lock)
            self.assertIn('distribution_revision: "', lock)
            self.assertIn("distribution_dirty:", lock)
            self.assertIn('"AGENTS.md":', lock)
            self.assertIn('"90-Meta/audit-vault.py":', lock)
            self.assertNotIn('"Arquitectura.base":', lock)
            gitignore = (dest / ".gitignore").read_text(encoding="utf-8")
            self.assertNotIn(".knowledge-os.lock.yaml", gitignore)
            self.assertIn("/.agents/state/map-ecosystem/", gitignore)
            self.assertIn("/AGENTS.personal.md", gitignore)
            self.assertIn("/.plan/", gitignore)
            self.assertIn("/.investigations/", gitignore)
            self.assertIn("/.investigations-private/", gitignore)
            self.assertNotIn("/investigations/", gitignore)
            self.assertNotIn("/plan/", gitignore)
            obsidian_app = json.loads(
                (dest / ".obsidian" / "app.json").read_text(encoding="utf-8")
            )
            self.assertEqual(obsidian_app["userIgnoreFilters"], [".plan/", "investigations/", "AGENTS.personal.md"])
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
                    "gcp",
                    "--yes",
                ]
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((dest / ".agents" / "skills" / "inspect-gcp-runtime" / "SKILL.md").is_file())
            self.assertTrue((dest / ".agents" / "skills" / "gcloud" / "SKILL.md").is_file())
            self.assertFalse((dest / ".agents" / "skills" / "inspect-database").exists())
            gcp_skill = (
                dest / ".agents" / "skills" / "inspect-gcp-runtime" / "SKILL.md"
            ).read_text(encoding="utf-8")
            self.assertIn("database-evidence-adapter-unavailable", gcp_skill)

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
                '{"livePreview": true, "userIgnoreFilters": ["archive/", "plan/"]}\n',
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
            self.assertIn("/.plan/", gitignore)
            self.assertIn("/AGENTS.personal.md", gitignore)
            self.assertNotIn("/plan/", gitignore)
            obsidian_app = json.loads(
                (dest / ".obsidian" / "app.json").read_text(encoding="utf-8")
            )
            self.assertTrue(obsidian_app["livePreview"])
            self.assertEqual(
                obsidian_app["userIgnoreFilters"],
                ["archive/", ".plan/", "investigations/", "AGENTS.personal.md"],
            )
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

    def test_personal_agents_contract_and_doctor_rejects_tracked_copy(self) -> None:
        router = (DIST / "kernel" / "AGENTS.md").read_text(encoding="utf-8")
        self.assertIn("## Personal instructions", router)
        self.assertIn("load @AGENTS.personal.md when it exists", router)
        self.assertIn("local tool and agent selection", router)
        self.assertIn("delegation, monitoring, resumption and verification workflows", router)
        self.assertIn("personal instructions take precedence over generic skill guidance", router)
        self.assertIn("preserve that skill", router)
        self.assertIn("record the specialization only in the personal file", router)
        self.assertGreater(
            router.index("## Personal instructions"),
            router.index("## Guardrails"),
        )
        self.assertLess(
            router.index("## Personal instructions"),
            router.index("## Investigation and local stores"),
        )
        self.assertIn("create or update the personal file", router)
        self.assertIn("its absence is valid", router)
        self.assertIn("Keep credentials in their proper secret store", router)

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


if __name__ == "__main__":
    unittest.main()
