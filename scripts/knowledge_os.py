#!/usr/bin/env python3
"""Install, update, and diagnose cell vaults from this knowledge-os distribution."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any

DIST = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(DIST / "kernel" / "90-Meta"))
from instance import (  # noqa: E402
    DEFAULT_TYPES,
    dump_instance,
    load_instance,
    orientation_status,
    validate_instance,
    InstanceError,
)

LOCK_NAME = ".knowledge-os.lock.yaml"
INSTANCE_OWNED = {
    "instance.yaml",
    "00-Home.md",
    "10-Sistemas",
    "15-Arquitectura",
    "20-Repos",
    "25-Topics",
    "30-Flujos",
    "40-Integraciones",
    "50-Glosario",
    "60-Operacion",
    "70-Aprendizajes",
}
KNOWLEDGE_HINTS = ("10-Sistemas", "00-Home.md", "30-Flujos")


def dist_version() -> str:
    return (DIST / "VERSION").read_text(encoding="utf-8").strip()


def distribution_provenance() -> tuple[str, bool]:
    revision = subprocess.run(
        ["git", "-C", str(DIST), "rev-parse", "HEAD"],
        text=True,
        capture_output=True,
        check=False,
    )
    if revision.returncode != 0:
        return "unversioned", True
    dirty = subprocess.run(
        ["git", "-C", str(DIST), "status", "--porcelain", "--untracked-files=no"],
        text=True,
        capture_output=True,
        check=False,
    )
    return revision.stdout.strip(), dirty.returncode != 0 or bool(dirty.stdout.strip())


def managed_paths() -> list[str]:
    rows: list[str] = []
    for line in (DIST / "MANAGED_PATHS").read_text(encoding="utf-8").splitlines():
        item = line.strip()
        if item and not item.startswith("#"):
            rows.append(item)
    return rows


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    digest.update(path.read_bytes())
    return digest.hexdigest()


def _source_files(root: Path, target_root: str) -> dict[str, Path]:
    files: dict[str, Path] = {}
    for path in root.rglob("*"):
        if not path.is_file():
            continue
        rel = path.relative_to(root)
        if any(part == "__pycache__" or part.endswith(".pyc") for part in rel.parts):
            continue
        files[(Path(target_root) / rel).as_posix()] = path
    return files


def managed_sources(adapters: list[str]) -> dict[str, Path]:
    """Return the exact distribution-owned files for an installed cell."""
    sources: dict[str, Path] = {}
    for entry in managed_paths():
        source = DIST / "kernel" / entry
        if not source.exists():
            source = DIST / entry
        if entry.endswith("/"):
            if not source.is_dir():
                raise RuntimeError(f"managed source directory is missing: {entry}")
            sources.update(_source_files(source, entry))
        else:
            if not source.is_file():
                raise RuntimeError(f"managed source is missing: {entry}")
            sources[entry] = source
        if entry == ".agents/skills/":
            for name in adapters:
                adapter = DIST / "adapters" / name
                if not adapter.is_dir():
                    raise SystemExit(f"unknown adapter: {name}")
                for skill_dir in adapter.iterdir():
                    if skill_dir.is_dir():
                        sources.update(
                            _source_files(skill_dir, f".agents/skills/{skill_dir.name}")
                        )
    return dict(sorted(sources.items()))


def tree_hashes(vault: Path, adapters: list[str]) -> dict[str, str]:
    hashes: dict[str, str] = {}
    for rel in managed_sources(adapters):
        path = vault / rel
        if path.is_file() and not path.is_symlink():
            hashes[rel] = sha256_file(path)
    return hashes


def distribution_hashes(adapters: list[str]) -> dict[str, str]:
    return {rel: sha256_file(path) for rel, path in managed_sources(adapters).items()}


def dump_lock(data: dict[str, Any]) -> str:
    lines = [
        f'version: "{data["version"]}"',
        f'kernel_version: "{data["kernel_version"]}"',
        f'distribution_revision: "{data["distribution_revision"]}"',
        f'distribution_dirty: {str(bool(data["distribution_dirty"])).lower()}',
        "adapters:",
    ]
    adapters = data.get("adapters") or []
    if adapters:
        for item in adapters:
            lines.append(f"  - {item}")
    else:
        lines.append("  []")
    lines.append("managed_hashes:")
    for rel, digest in sorted((data.get("managed_hashes") or {}).items()):
        lines.append(f'  "{rel}": {digest}')
    lines.append("")
    return "\n".join(lines)


def load_lock(path: Path) -> dict[str, Any]:
    version = ""
    kernel_version = ""
    distribution_revision = "unknown"
    distribution_dirty = True
    adapters: list[str] = []
    hashes: dict[str, str] = {}
    section = ""
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if line.startswith("version:"):
            version = line.split(":", 1)[1].strip().strip('"')
        elif line.startswith("kernel_version:"):
            kernel_version = line.split(":", 1)[1].strip().strip('"')
        elif line.startswith("distribution_revision:"):
            distribution_revision = line.split(":", 1)[1].strip().strip('"')
        elif line.startswith("distribution_dirty:"):
            distribution_dirty = line.split(":", 1)[1].strip().casefold() == "true"
        elif line.startswith("adapters:"):
            section = "adapters"
        elif line.startswith("managed_hashes:"):
            section = "hashes"
        elif section == "adapters" and line.startswith("- "):
            adapters.append(line[2:].strip())
        elif section == "hashes" and ":" in line:
            key, _, digest = line.partition(":")
            hashes[key.strip().strip('"')] = digest.strip()
    return {
        "version": version,
        "kernel_version": kernel_version,
        "distribution_revision": distribution_revision,
        "distribution_dirty": distribution_dirty,
        "adapters": adapters,
        "managed_hashes": hashes,
    }


def copy_kernel(dest: Path, adapters: list[str]) -> None:
    kernel = DIST / "kernel"
    for rel, source in managed_sources(adapters).items():
        target = dest / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        if target.is_symlink():
            target.unlink()
        elif target.exists() and not target.is_file():
            raise RuntimeError(f"managed target is not a file: {rel}")
        shutil.copy2(source, target)
    target_claude = dest / "CLAUDE.md"
    if target_claude.exists() or target_claude.is_symlink():
        target_claude.unlink()
    os.symlink("AGENTS.md", target_claude)
    for name in ("Arquitectura.base", "Auditoria.base", "Operacion.base", "Repos.base"):
        target = dest / name
        if not target.is_file():
            shutil.copy2(kernel / name, target)
    claude_skills = dest / ".claude" / "skills"
    claude_skills.parent.mkdir(parents=True, exist_ok=True)
    if claude_skills.exists() or claude_skills.is_symlink():
        if claude_skills.is_dir() and not claude_skills.is_symlink():
            shutil.rmtree(claude_skills)
        else:
            claude_skills.unlink()
    os.symlink(os.path.join("..", ".agents", "skills"), claude_skills)


def _copytree(src: Path, dst: Path) -> None:
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))


def ensure_gitignore_lines(dest: Path) -> None:
    required = (
        "/.knowledge-os-config.yaml",
        "/.knowledge-os-config.*.tmp",
        "/.agents/state/map-ecosystem/sync/",
    )
    path = dest / ".gitignore"
    lines = path.read_text(encoding="utf-8").splitlines() if path.is_file() else []
    portable_lines = [
        line
        for line in lines
        if line.strip() not in {".knowledge-os.lock.yaml", "/.knowledge-os.lock.yaml"}
    ]
    changed = not path.is_file() or portable_lines != lines
    lines = portable_lines
    for item in required:
        if item not in lines:
            lines.append(item)
            changed = True
    if changed:
        path.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")


def seed_skeleton(dest: Path, enabled_types: list[str] | None = None) -> None:
    types = set(enabled_types or DEFAULT_TYPES)
    skeleton = DIST / "kernel" / "skeleton"
    for child in skeleton.iterdir():
        if child.name == "25-Topics" and "topic" not in types:
            continue
        target = dest / child.name
        target.mkdir(parents=True, exist_ok=True)
        gitkeep = child / ".gitkeep"
        if gitkeep.is_file():
            shutil.copy2(gitkeep, target / ".gitkeep")


def render_template(name: str, mapping: dict[str, str]) -> str:
    text = (DIST / "kernel" / "templates" / name).read_text(encoding="utf-8")
    for key, value in mapping.items():
        text = text.replace("{{" + key + "}}", value)
    return text


def pending_inventory(instance: dict[str, Any]) -> str:
    roots = [Path(item) for item in instance["sources"]["discovery_roots"] if str(item).strip()]
    prefixes = [str(item) for item in instance["sources"]["repo_prefixes"] if str(item).strip()]
    if not roots:
        return "_No discovery roots were given. Configure them later with `configure-workspace`._\n"
    found: list[str] = []
    for root in roots:
        if not root.is_dir():
            continue
        candidates = [root] if (root / ".git").exists() else [path for path in root.iterdir() if path.is_dir()]
        for candidate in candidates:
            result = subprocess.run(
                ["git", "-C", str(candidate), "remote", "get-url", "origin"],
                capture_output=True,
                text=True,
                check=False,
            )
            remote = result.stdout.strip()
            if not remote:
                continue
            name = candidate.name
            if prefixes and not any(name.startswith(prefix) or prefix in remote for prefix in prefixes):
                continue
            found.append(f"- `{name}` — `{remote}`")
    if not found:
        return "_Discovery roots were set, but no matching remotes were visible yet._\n"
    return "## Pending inventory\n\nThese remotes look in-scope and are not documented yet. Do not invent repository notes without evidence.\n\n" + "\n".join(found) + "\n"


def write_bootstrap(dest: Path, instance: dict[str, Any]) -> None:
    dest.mkdir(parents=True, exist_ok=True)
    (dest / "instance.yaml").write_text(dump_instance(instance), encoding="utf-8")
    systems_list = "\n".join(
        f"- [[{item['name']}]] (`{item['id']}`)" for item in instance["systems"]
    )
    home = render_template(
        "00-Home.md.tmpl",
        {
            "cell_name": instance["cell"]["name"],
            "cell_purpose": instance["cell"]["purpose"],
            "systems_list": systems_list,
            "pending_inventory": pending_inventory(instance),
        },
    )
    (dest / "00-Home.md").write_text(home, encoding="utf-8")
    systems_dir = dest / "10-Sistemas"
    systems_dir.mkdir(parents=True, exist_ok=True)
    for item in instance["systems"]:
        aliases = item.get("aliases") or []
        note = render_template(
            "sistema.md.tmpl",
            {
                "system_id": item["id"],
                "system_name": item["name"],
                "cell_purpose": instance["cell"]["purpose"],
                "aliases_yaml": "[" + ", ".join(aliases) + "]" if aliases else "[]",
            },
        )
        (systems_dir / f"{item['name']}.md").write_text(note, encoding="utf-8")
        (dest / "20-Repos" / item["id"]).mkdir(parents=True, exist_ok=True)
        (dest / "20-Repos" / item["id"] / ".gitkeep").write_text("", encoding="utf-8")
    for name, folder in (
        ("Flujos.md.tmpl", dest / "30-Flujos" / "Flujos.md"),
        ("Operacion.md.tmpl", dest / "60-Operacion" / "Operacion.md"),
        ("Aprendizajes.md.tmpl", dest / "70-Aprendizajes" / "Aprendizajes.md"),
    ):
        folder.parent.mkdir(parents=True, exist_ok=True)
        folder.write_text(render_template(name, {}), encoding="utf-8")
    gitignore = dest / ".gitignore"
    gitignore.write_text(
        "\n".join(
            [
                ".DS_Store",
                ".obsidian/workspace.json",
                ".obsidian/workspace-mobile.json",
                "/.investigations/",
                "/.operations/",
                "/.knowledge-os-config.yaml",
                "/.knowledge-os-config.*.tmp",
                "/plan/",
                "/output/",
                "__pycache__/",
                "",
            ]
        ),
        encoding="utf-8",
    )


def write_lock(dest: Path, adapters: list[str]) -> None:
    revision, dirty = distribution_provenance()
    payload = {
        "version": "3",
        "kernel_version": dist_version(),
        "distribution_revision": revision,
        "distribution_dirty": dirty,
        "adapters": adapters,
        "managed_hashes": tree_hashes(dest, adapters),
    }
    path = dest / LOCK_NAME
    path.write_text(dump_lock(payload), encoding="utf-8")
    path.chmod(0o644)


def dest_state(dest: Path) -> str:
    if not dest.exists():
        return "missing"
    if (dest / LOCK_NAME).is_file():
        return "installed"
    entries = [path for path in dest.iterdir() if path.name not in {".git", ".DS_Store"}]
    if not entries:
        return "empty"
    if any((dest / hint).exists() for hint in KNOWLEDGE_HINTS) or (dest / "instance.yaml").is_file():
        return "knowledge-without-lock"
    return "empty" if not any(entries) else "nonempty"


def prompt(label: str, default: str, yes: bool) -> str:
    if yes and default:
        return default
    if yes:
        return default
    suffix = f" [{default}]" if default else ""
    try:
        value = input(f"{label}{suffix}: ").strip()
    except EOFError:
        return default
    return value or default


def build_instance_from_args(args: argparse.Namespace) -> dict[str, Any]:
    systems: list[dict[str, Any]] = []
    for item in args.system or []:
        ident, _, name = item.partition(":")
        ident = ident.strip()
        name = (name or ident).strip()
        systems.append({"id": ident, "name": name, "aliases": [ident]})
    if not systems:
        if args.yes:
            raise SystemExit("--system id:Name is required with --yes")
        raw = prompt("Systems as id:Name, comma-separated", "platform:Platform", False)
        for part in raw.split(","):
            ident, _, name = part.partition(":")
            ident = ident.strip()
            name = (name or ident).strip()
            if ident:
                systems.append({"id": ident, "name": name, "aliases": [ident]})
    cell_name = args.cell_name or prompt("Cell name", "Cell", args.yes)
    purpose = args.purpose or prompt("Cell purpose (1-3 sentences)", "Describe this cell.", args.yes)
    profile = args.evidence_profile or prompt(
        "Evidence profile (production-gate|documented-source|mixed)",
        "production-gate",
        args.yes,
    )
    locale = args.locale or prompt("Note locale (es|en)", "es", args.yes)
    adapters = list(args.adapter or [])
    if not args.yes and not adapters:
        raw = prompt("Adapters (gcp,postgres,reports — empty for none)", "", False)
        adapters = [part.strip() for part in raw.split(",") if part.strip()]
    types = list(DEFAULT_TYPES)
    if args.disable_topics:
        types = [item for item in types if item != "topic"]
    data = {
        "version": 1,
        "cell": {"name": cell_name, "purpose": purpose},
        "systems": systems,
        "vault": {"remote": args.vault_remote or ""},
        "sources": {
            "github_org": args.github_org or "",
            "repo_prefixes": args.repo_prefix or [],
            "discovery_roots": args.discovery_root or [],
            "schema_repository": {"remote": "", "note": ""},
        },
        "graph": {"enabled_types": types},
        "evidence": {"profile": profile},
        "adapters": adapters,
        "locale": {"notes": locale},
    }
    return validate_instance(data)


def cmd_init(args: argparse.Namespace) -> int:
    dest = Path(args.dest).expanduser().resolve()
    state = dest_state(dest)
    if state == "installed" and not args.force:
        print(f"already installed: {dest} (use update, or --force to re-init)", file=sys.stderr)
        return 2
    if state == "knowledge-without-lock" and not args.force:
        print(
            f"{dest} has knowledge files but no lock. Run doctor or pass --force.",
            file=sys.stderr,
        )
        return 2
    dest.mkdir(parents=True, exist_ok=True)
    instance = build_instance_from_args(args)
    seed_skeleton(dest, instance["graph"]["enabled_types"])
    copy_kernel(dest, instance["adapters"])
    write_bootstrap(dest, instance)
    ensure_gitignore_lines(dest)
    write_lock(dest, instance["adapters"])
    print(json.dumps({"status": "initialized", "dest": str(dest), "cell": instance["cell"]}, indent=2))
    return 0


def cmd_adopt(args: argparse.Namespace) -> int:
    dest = Path(args.dest).expanduser().resolve()
    state = dest_state(dest)
    if state == "installed":
        print(f"already installed: {dest} (use update)", file=sys.stderr)
        return 2
    if state != "knowledge-without-lock":
        print(
            f"adopt requires an existing knowledge vault without a lock (state={state})",
            file=sys.stderr,
        )
        return 2
    if not (dest / "00-Home.md").is_file():
        print("00-Home.md is required for adopt", file=sys.stderr)
        return 2
    systems_dir = dest / "10-Sistemas"
    if not systems_dir.is_dir() or not any(systems_dir.glob("*.md")):
        print("10-Sistemas/*.md is required for adopt", file=sys.stderr)
        return 2
    instance_path = dest / "instance.yaml"
    if not instance_path.is_file():
        print("write instance.yaml (cell identity) before adopt; Home is never rewritten", file=sys.stderr)
        return 2
    try:
        instance = load_instance(instance_path)
    except InstanceError as error:
        print(str(error), file=sys.stderr)
        return 2
    conflicts = ownership_conflicts(dest, instance["adapters"], set())
    overlays = dest / ".agents" / "overlays"
    uncovered = [rel for rel in conflicts if not overlay_covers(overlays, dest, rel)]
    if uncovered and not args.force:
        print(json.dumps({"status": "ownership-conflict", "files": uncovered}, indent=2))
        print("distribution-owned files differ; move cell logic to extensions or pass --force", file=sys.stderr)
        return 3
    copy_kernel(dest, instance["adapters"])
    ensure_gitignore_lines(dest)
    write_lock(dest, instance["adapters"])
    print(json.dumps({"status": "adopted", "dest": str(dest), "cell": instance["cell"]}, indent=2))
    return 0


def overlay_covers(overlays: Path, vault: Path, rel: str) -> bool:
    """True when a local overlay exists for this managed relative path."""
    parts = Path(rel).parts
    if len(parts) >= 2:
        if (overlays / parts[-2] / Path(rel).name).exists():
            return True
    elif (overlays / Path(rel).name).exists():
        return True
    target = vault / rel
    if target.is_symlink():
        linked = Path(os.readlink(target))
        if not linked.is_absolute():
            return overlay_covers(overlays, vault, linked.as_posix())
    return False


def ownership_conflicts(
    dest: Path,
    adapters: list[str],
    previously_owned: set[str],
) -> list[str]:
    conflicts: list[str] = []
    for rel, source in managed_sources(adapters).items():
        if rel in previously_owned:
            continue
        target = dest / rel
        if target.is_symlink() or (
            target.exists()
            and (
                not target.is_file()
                or sha256_file(target) != sha256_file(source)
            )
        ):
            conflicts.append(rel)
    return conflicts


def cmd_update(args: argparse.Namespace) -> int:
    dest = Path(args.dest).expanduser().resolve()
    lock_path = dest / LOCK_NAME
    if not lock_path.is_file():
        print("no lock file; run init or doctor", file=sys.stderr)
        return 2
    lock = load_lock(lock_path)
    instance = load_instance(dest / "instance.yaml")
    target_adapters = instance["adapters"]
    current = tree_hashes(dest, lock["adapters"])
    owned = set(managed_sources(lock["adapters"]))
    drifted = [
        rel
        for rel, digest in lock["managed_hashes"].items()
        if rel in owned and current.get(rel) != digest
    ]
    drifted.extend(
        ownership_conflicts(dest, target_adapters, set(lock["managed_hashes"]))
    )
    drifted = sorted(set(drifted))
    overlays = dest / ".agents" / "overlays"
    if drifted and not args.force:
        uncovered = [rel for rel in drifted if not overlay_covers(overlays, dest, rel)]
        if uncovered:
            print(json.dumps({"status": "drift", "files": uncovered}, indent=2))
            print("kernel files changed locally; add overlays or pass --force", file=sys.stderr)
            return 3
    copy_kernel(dest, target_adapters)
    ensure_gitignore_lines(dest)
    write_lock(dest, target_adapters)
    print(json.dumps({"status": "updated", "dest": str(dest), "kernel_version": dist_version()}, indent=2))
    return 0


def cmd_doctor(args: argparse.Namespace) -> int:
    dest = Path(args.dest).expanduser().resolve()
    state = dest_state(dest)
    payload: dict[str, Any] = {"dest": str(dest), "state": state}
    if state == "installed":
        lock = load_lock(dest / LOCK_NAME)
        adapters = lock.get("adapters") or []
        current = tree_hashes(dest, adapters)
        expected = distribution_hashes(adapters)
        payload["kernel_version_installed"] = lock.get("kernel_version")
        payload["kernel_version_dist"] = dist_version()
        payload["lock_version"] = lock.get("version")
        payload["portable_lock"] = lock.get("version") in {"2", "3"}
        revision, dirty = distribution_provenance()
        payload["distribution_revision_installed"] = lock.get("distribution_revision")
        payload["distribution_dirty_installed"] = lock.get("distribution_dirty")
        payload["distribution_revision_dist"] = revision
        payload["distribution_dirty_dist"] = dirty
        payload["reproducible_distribution"] = (
            lock.get("distribution_revision") not in {None, "", "unknown", "unversioned"}
            and not lock.get("distribution_dirty", True)
        )
        payload["drift"] = [
            rel
            for rel, digest in lock.get("managed_hashes", {}).items()
            if rel in expected and current.get(rel) != digest
        ]
        payload["distribution_drift"] = [
            rel for rel, digest in expected.items() if current.get(rel) != digest
        ]
        payload["managed_matches_dist"] = not payload["distribution_drift"]
        try:
            payload["orientation"] = orientation_status(dest)
        except InstanceError as error:
            payload["orientation"] = {"ready": False, "issues": [str(error)]}
        payload["start_here"] = payload.get("orientation", {}).get(
            "start_here", ["00-Home.md", "instance.yaml"]
        )
    print(json.dumps(payload, indent=2, ensure_ascii=False))
    return 0 if state in {"installed", "empty", "missing"} else 1


def detect_command(args: argparse.Namespace) -> str:
    if args.command:
        return args.command
    dest = Path(args.dest or Path.cwd()).expanduser().resolve()
    state = dest_state(dest)
    if state == "installed":
        return "update"
    if state in {"missing", "empty"}:
        return "init"
    return "doctor"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", nargs="?", choices=["init", "update", "doctor", "adopt"])
    parser.add_argument("--dest", default="")
    parser.add_argument("--cell-name", default="")
    parser.add_argument("--purpose", default="")
    parser.add_argument("--system", action="append", default=[])
    parser.add_argument("--adapter", action="append", default=[])
    parser.add_argument("--evidence-profile", default="")
    parser.add_argument("--locale", default="")
    parser.add_argument("--vault-remote", default="")
    parser.add_argument("--github-org", default="")
    parser.add_argument("--repo-prefix", action="append", default=[])
    parser.add_argument("--discovery-root", action="append", default=[])
    parser.add_argument("--disable-topics", action="store_true")
    parser.add_argument("--yes", action="store_true")
    parser.add_argument("--force", action="store_true")
    args = parser.parse_args()
    if not args.dest:
        args.dest = str(Path.cwd())
    command = detect_command(args)
    if command == "init":
        return cmd_init(args)
    if command == "adopt":
        return cmd_adopt(args)
    if command == "update":
        return cmd_update(args)
    return cmd_doctor(args)


if __name__ == "__main__":
    sys.exit(main())
