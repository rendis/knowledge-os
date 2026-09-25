#!/usr/bin/env python3
"""Install, update, and diagnose cell vaults from this knowledge-os distribution."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

DIST = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(DIST / "scripts"))
import native_runtime  # noqa: E402
from instance import (  # noqa: E402
    DEFAULT_TYPES,
    dump_instance,
    load_instance,
    orientation_status,
    validate_instance,
    InstanceError,
)

from vault_catalog import CATALOG_PATH, CatalogError, load_catalog  # noqa: E402

LOCK_NAME = ".knowledge-os.lock.yaml"
PERSONAL_AGENTS = "AGENTS.personal.md"
MANAGED_HASH_COMMENT = "# gitleaks:allow -- managed SHA-256 digest"
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
        ["git", "-C", str(DIST), "status", "--porcelain", "--untracked-files=all"],
        text=True,
        capture_output=True,
        check=False,
    )
    # Ignored files are still shipped by managed_sources, so status alone is
    # insufficient evidence that the committed revision reproduces the payload.
    adapters = [path.name for path in (DIST / "adapters").iterdir() if path.is_dir()]
    shipped = set(managed_sources([]).values())
    for adapter in adapters:
        shipped.update(managed_sources([adapter]).values())
    committed = subprocess.run(
        ["git", "-C", str(DIST), "ls-tree", "-rz", "HEAD"],
        capture_output=True, check=False,
    )
    entries = {}
    for entry in committed.stdout.split(b"\0"):
        if entry:
            metadata, path = entry.split(b"\t", 1)
            mode, kind, oid = metadata.decode("ascii").split()
            entries[os.fsdecode(path)] = (mode, kind, oid)
    payload_dirty = committed.returncode != 0
    for source in shipped:
        entry = entries.get(source.relative_to(DIST).as_posix())
        content = source.read_bytes()
        if entry is None:
            payload_dirty = True
            continue
        mode, kind, oid = entry
        algorithm = "sha256" if len(oid) == 64 else "sha1"
        blob = hashlib.new(algorithm, b"blob " + str(len(content)).encode("ascii") + b"\0" + content).hexdigest()
        expected_mode = "100755" if source.stat().st_mode & 0o111 else "100644"
        if kind != "blob" or blob != oid or mode != expected_mode or source.is_symlink():
            payload_dirty = True
    return revision.stdout.strip(), (
        dirty.returncode != 0 or bool(dirty.stdout.strip()) or payload_dirty
    )


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
    # Consumer catalogs are never distribution payload, even if accidentally added.
    sources.pop(CATALOG_PATH.as_posix(), None)
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
    if data.get("runtime_release") is not None:
        lines.append("runtime_release: " + json.dumps(data["runtime_release"], sort_keys=True, separators=(",", ":")))
    lines.append("managed_hashes:")
    for rel, digest in sorted((data.get("managed_hashes") or {}).items()):
        lines.append(f'  "{rel}": {digest} {MANAGED_HASH_COMMENT}')
    lines.append("")
    return "\n".join(lines)


def load_lock(path: Path) -> dict[str, Any]:
    version = ""
    kernel_version = ""
    distribution_revision = "unknown"
    distribution_dirty = True
    adapters: list[str] = []
    hashes: dict[str, str] = {}
    runtime_release = None
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
        elif line.startswith("runtime_release:"):
            runtime_release = native_runtime.descriptor(json.loads(line.split(":", 1)[1].strip()))
            section = ""
        elif line.startswith("adapters:"):
            section = "adapters"
        elif line.startswith("managed_hashes:"):
            section = "hashes"
        elif section == "adapters" and line.startswith("- "):
            adapters.append(line[2:].strip())
        elif section == "hashes" and ":" in line:
            key, _, digest = line.partition(":")
            digest = digest.strip()
            comment_suffix = f" {MANAGED_HASH_COMMENT}"
            if digest.endswith(comment_suffix):
                digest = digest[: -len(comment_suffix)]
            hashes[key.strip().strip('"')] = digest
    return {
        "version": version,
        "kernel_version": kernel_version,
        "distribution_revision": distribution_revision,
        "distribution_dirty": distribution_dirty,
        "adapters": adapters,
        "managed_hashes": hashes,
        "runtime_release": runtime_release,
    }


def preflight_kernel(dest: Path, adapters: list[str], extra_paths: list[str] | None = None) -> None:
    native_runtime.preflight(dest, native_runtime.release(DIST))
    sources = managed_sources(adapters)
    extra = [".claude/skills", "Arquitectura.base", "Auditoria.base",
             "Operacion.base", "Repos.base", ".gitignore", ".obsidian/app.json", LOCK_NAME]
    for rel in [*sources, *extra, *(extra_paths or [])]:
        relative = Path(rel)
        if relative.is_absolute() or ".." in relative.parts:
            raise RuntimeError(f"unsafe managed path: {rel}")
        parent = dest
        for part in relative.parts[:-1]:
            parent /= part
            if parent.is_symlink() or (parent.exists() and not parent.is_dir()):
                raise RuntimeError(f"managed path has an unsafe parent: {rel}")
        target = dest / relative
        if target.exists() and not target.is_symlink() and not target.is_file():
            raise RuntimeError(f"managed target collision; move it explicitly before retrying: {rel}")
        if target.is_symlink() and rel not in sources and rel != ".claude/skills":
            raise RuntimeError(f"local write target is a symlink: {rel}")
        if rel in sources:
            source = sources[rel]
            if source.is_symlink() or any(parent.is_symlink() for parent in source.parents if parent != DIST and DIST in parent.parents):
                raise RuntimeError(f"managed source must be a regular file: {rel}")
    skills = dest / ".claude/skills"
    if skills.exists() and not skills.is_symlink():
        raise RuntimeError(".claude/skills must be moved explicitly before installation")


def copy_kernel(dest: Path, adapters: list[str]) -> None:
    preflight_kernel(dest, adapters)
    kernel = DIST / "kernel"
    for rel, source in managed_sources(adapters).items():
        target = dest / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        if target.is_symlink():
            target.unlink()
        elif target.exists() and not target.is_file():
            raise RuntimeError(f"managed target is not a file: {rel}")
        shutil.copy2(source, target)
    native_runtime.install(DIST, dest, native_runtime.release(DIST))
    for name in ("Arquitectura.base", "Auditoria.base", "Operacion.base", "Repos.base"):
        target = dest / name
        if not target.is_file():
            shutil.copy2(kernel / name, target)
    claude_skills = dest / ".claude" / "skills"
    claude_skills.parent.mkdir(parents=True, exist_ok=True)
    if claude_skills.exists() or claude_skills.is_symlink():
        claude_skills.unlink()
    os.symlink(os.path.join("..", ".agents", "skills"), claude_skills)


def _copytree(src: Path, dst: Path) -> None:
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))


def ensure_gitignore_lines(dest: Path) -> None:
    required = (
        f"/{PERSONAL_AGENTS}",
        "/.investigations/",
        "/.investigations-private/",
        "/.knowledge-os-config.yaml",
        "/.knowledge-os-config.*.tmp",
        "/.agents/state/discovery/",
        "/.plan/",
        "/.scratch/",
        "/.venv/",
    )
    path = dest / ".gitignore"
    lines = path.read_text(encoding="utf-8").splitlines() if path.is_file() else []
    portable_lines = [
        line
        for line in lines
        if line.strip()
        not in {
            ".knowledge-os.lock.yaml",
            "/.knowledge-os.lock.yaml",
            "plan/",
            "/plan/",
        }
    ]
    changed = not path.is_file() or portable_lines != lines
    lines = portable_lines
    for item in required:
        if item not in lines:
            lines.append(item)
            changed = True
    portable = ["!/.agents/bin/"] + ["!/" + name for name in sorted(native_runtime.bundle_hashes(native_runtime.release(DIST)))]
    # Place exceptions last so pre-existing *.exe or bin rules cannot hide the bundle.
    updated = [line for line in lines if line not in portable] + portable
    changed = changed or updated != lines
    lines = updated
    if changed:
        path.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")


def personal_agents_ignored(dest: Path) -> bool:
    """Return whether Git or the portable root rule ignores the personal router."""
    checked = subprocess.run(
        ["git", "-C", str(dest), "check-ignore", "--no-index", "--quiet", "--", PERSONAL_AGENTS],
        capture_output=True,
        check=False,
    )
    if checked.returncode in {0, 1}:
        return checked.returncode == 0
    gitignore = dest / ".gitignore"
    if not gitignore.is_file():
        return False
    rules = {line.strip() for line in gitignore.read_text(encoding="utf-8").splitlines()}
    return f"/{PERSONAL_AGENTS}" in rules or PERSONAL_AGENTS in rules


def ensure_obsidian_ignore_filters(dest: Path) -> None:
    path = dest / ".obsidian" / "app.json"
    payload: dict[str, object] = {}
    if path.is_file():
        loaded = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(loaded, dict):
            raise ValueError(f"{path} must contain a JSON object")
        payload = loaded
    raw_filters = payload.get("userIgnoreFilters", [])
    if not isinstance(raw_filters, list) or not all(
        isinstance(item, str) for item in raw_filters
    ):
        raise ValueError(f"{path} userIgnoreFilters must be a list of strings")
    filters = [
        item
        for item in raw_filters
        if item not in {"plan/", "/plan/", "investigations/", "/investigations/"}
    ]
    if ".plan/" not in filters:
        filters.append(".plan/")
    if ".scratch/" not in filters:
        filters.append(".scratch/")
    if ".investigations/" not in filters:
        filters.append(".investigations/")
    if PERSONAL_AGENTS not in filters:
        filters.append(PERSONAL_AGENTS)
    payload["userIgnoreFilters"] = filters
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )


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


def render_template(name: str, mapping: dict[str, str], locale: str = "en") -> str:
    templates = DIST / "kernel" / "templates"
    if locale == "es":
        templates = templates / "es"
    text = (templates / name).read_text(encoding="utf-8")
    for key, value in mapping.items():
        text = text.replace("{{" + key + "}}", value)
    return text


def pending_inventory(instance: dict[str, Any]) -> str:
    roots = [Path(item) for item in instance["sources"]["discovery_roots"] if str(item).strip()]
    prefixes = [str(item) for item in instance["sources"]["repo_prefixes"] if str(item).strip()]
    spanish = instance["locale"]["notes"] == "es"
    guidance = (
        "Consulta la configuración local vigente desde la raíz del vault con "
        if spanish else "Check the current local configuration from the vault root with "
    )
    guidance += (
        "el ejecutable de tu plataforma en `.agents/bin/` y `config status --vault .`; consulta `.agents/skills/use-vault-cli/SKILL.md`.\n"
        if spanish else
        "the platform executable in `.agents/bin/` with `config status --vault .`; see `.agents/skills/use-vault-cli/SKILL.md`.\n"
    )
    if not roots:
        return guidance
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
        return guidance
    heading = "Inventario pendiente" if spanish else "Pending inventory"
    description = (
        "Remotos detectados al inicializar, pendientes de documentación respaldada por evidencia."
        if spanish else
        "Remotes detected at initialization, awaiting evidence-backed documentation."
    )
    return guidance + f"\n## {heading}\n\n{description}\n\n" + "\n".join(found) + "\n"


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
        locale=instance["locale"]["notes"],
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
            locale=instance["locale"]["notes"],
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
                f"/{PERSONAL_AGENTS}",
                "/.investigations/",
                "/.investigations-private/",
                "/.operations/",
                "/.knowledge-os-config.yaml",
                "/.knowledge-os-config.*.tmp",
                "/.plan/",
                "/.scratch/",
                "/output/",
                "__pycache__/",
                "",
            ]
        ),
        encoding="utf-8",
    )


def write_lock(dest: Path, adapters: list[str]) -> None:
    revision, dirty = distribution_provenance()
    runtime_release = native_runtime.release(DIST)
    dirty = dirty or runtime_release["source_dirty"] or runtime_release["source_revision"] != revision
    payload = {
        "version": "4",
        "kernel_version": dist_version(),
        "distribution_revision": revision,
        "distribution_dirty": dirty,
        "adapters": adapters,
        "managed_hashes": tree_hashes(dest, adapters),
        "runtime_release": runtime_release,
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
    except (EOFError, KeyboardInterrupt):
        raise SystemExit("Initialization cancelled: input interrupted.") from None
    return value or default


TRACKER_HOSTS = (("atlassian.net", "jira"), ("jira", "jira"), ("github.com", "github"), ("gitlab", "gitlab"),
                 ("dev.azure.com", "azure-devops"), ("visualstudio.com", "azure-devops"), ("linear.app", "linear"))


def parse_tracker(value: str) -> dict[str, str]:
    """A tracker URL (its provider is read from the host) or id:provider:https://url."""
    value = value.strip()
    if value.startswith("https://"):
        host = urlparse(value).hostname or ""
        provider = next((name for marker, name in TRACKER_HOSTS if marker in host), host.split(".")[0] or "tracker")
        return {"id": provider, "provider": provider, "url": value}
    tracker_id, separator, remainder = value.partition(":")
    provider, second_separator, url = remainder.partition(":")
    if not separator or not second_separator or not tracker_id or not provider or not url:
        raise InstanceError("a tracker is an https:// URL, or id:provider:https://url")
    return {"id": tracker_id.strip(), "provider": provider.strip(), "url": url.strip()}


def parse_system(value: str) -> dict[str, Any] | None:
    """A system name (its id is derived) or id:Name."""
    ident, colon, name = value.strip().partition(":")
    if not colon:
        name = ident
        ident = re.sub(r"[^a-z0-9]+", "-", ident.lower()).strip("-")
    ident, name = ident.strip(), (name or ident).strip()
    return {"id": ident, "name": name, "aliases": [ident]} if ident else None


def listed(raw: str) -> list[str]:
    return [part.strip() for part in raw.split(",") if part.strip()]


def build_instance_from_args(args: argparse.Namespace) -> dict[str, Any]:
    """Project onboarding: who the cell is, where its code is, where it runs. Flags skip the questions;
    the evidence profile and adapters keep their defaults unless passed."""
    interactive = not args.yes
    cell_name = args.cell_name or prompt("Cell name", "Cell", args.yes)
    purpose = args.purpose or prompt("What the cell owns, in 1-3 sentences", "Describe this cell.", args.yes)
    systems = [s for s in (parse_system(item) for item in args.system or []) if s]
    if not systems:
        if args.yes:
            raise SystemExit("--system Name (or id:Name) is required with --yes")
        systems = [s for s in (parse_system(part) for part in listed(prompt("Systems it owns, comma-separated (e.g. Orders, Payments)", "Platform", False))) if s]
    github_org = args.github_org or (prompt("GitHub organization of its repositories (empty to skip)", "", False) if interactive else "")
    prefixes = list(args.repo_prefix or [])
    if interactive and not prefixes:
        prefixes = listed(prompt("Repository name prefixes, comma-separated (e.g. APP01234-; empty to list them later)", "", False))
    branch_order = args.reference_branch or listed(prompt(
        "Reference branches, in order: the first that exists in each repository is read (main, master, develop…)",
        "main,master",
        args.yes,
    ))
    platforms = list(args.platform or [])
    if interactive and not platforms:
        platforms = listed(prompt("Clouds the systems run on (gcp, aws, azure; empty for none)", "", False))
    trackers = [parse_tracker(item) for item in (args.tracker or [])]
    if interactive and not trackers:
        trackers = [parse_tracker(part) for part in listed(prompt("Issue tracker URLs, comma-separated (empty for none)", "", False))]
    locale = args.locale or prompt("Language of the notes (es|en)", "es", args.yes)
    types = list(DEFAULT_TYPES)
    if args.disable_topics:
        types = [item for item in types if item != "topic"]
    data = {
        "version": 1,
        "cell": {"name": cell_name, "purpose": purpose},
        "systems": systems,
        "trackers": trackers,
        "vault": {"remote": args.vault_remote or ""},
        "sources": {
            "github_org": github_org,
            "repo_prefixes": prefixes,
            "discovery_roots": args.discovery_root or [],
            "reference_branch_order": branch_order,
            "schema_repository": {"remote": "", "note": ""},
        },
        "graph": {"enabled_types": types},
        "evidence": {"profile": args.evidence_profile or "production-gate"},
        "platform": {"providers": platforms},
        "adapters": list(args.adapter or []),
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
    try:
        instance = build_instance_from_args(args)
    except InstanceError as error:
        print(
            json.dumps(
                {"status": "invalid-instance", "error": str(error)},
                indent=2,
            )
        )
        return 2
    bootstrap_paths = ["instance.yaml", "00-Home.md", "30-Flujos/Flujos.md",
                       "60-Operacion/Operacion.md", "70-Aprendizajes/Aprendizajes.md"]
    bootstrap_paths.extend(f"{child.name}/.gitkeep" for child in (DIST / "kernel").iterdir() if child.is_dir() and child.name[:2].isdigit())
    for item in instance["systems"]:
        bootstrap_paths.extend([f"10-Sistemas/{item['name']}.md", f"20-Repos/{item['id']}/.gitkeep"])
    runtime_conflicts = native_runtime.conflicts(dest, native_runtime.release(DIST), None)
    if runtime_conflicts and not args.force:
        print(json.dumps({"status": "ownership-conflict", "files": runtime_conflicts}))
        return 3
    preflight_kernel(dest, instance["adapters"], bootstrap_paths)
    dest.mkdir(parents=True, exist_ok=True)
    seed_skeleton(dest, instance["graph"]["enabled_types"])
    copy_kernel(dest, instance["adapters"])
    write_bootstrap(dest, instance)
    ensure_gitignore_lines(dest)
    ensure_obsidian_ignore_filters(dest)
    write_lock(dest, instance["adapters"])
    print(json.dumps({"status": "initialized", "dest": str(dest), "cell": instance["cell"],
                      "next": "Open the vault with your agent and ask it to finish onboarding (onboard-cell): it lists the "
                              "repositories, runs the first discovery and reports what the vault can answer. Each teammate "
                              "is onboarded the first time they open it (onboard-developer)."}, indent=2))
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
    conflicts.extend(native_runtime.conflicts(dest, native_runtime.release(DIST), None))
    if conflicts and not args.force:
        print(json.dumps({"status": "ownership-conflict", "files": sorted(set(conflicts))}, indent=2))
        print("distribution-owned files differ; move cell logic to cell-owned files or pass --force", file=sys.stderr)
        return 3
    copy_kernel(dest, instance["adapters"])
    ensure_gitignore_lines(dest)
    ensure_obsidian_ignore_filters(dest)
    write_lock(dest, instance["adapters"])
    print(json.dumps({"status": "adopted", "dest": str(dest), "cell": instance["cell"]}, indent=2))
    return 0


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


def managed_lock_target(dest: Path, rel: str) -> Path:
    relative = Path(rel)
    allowed = any(
        rel == entry
        if not entry.endswith("/")
        else rel.startswith(entry) and rel != entry
        for entry in managed_paths()
    )
    if (
        not rel
        or "\\" in rel
        or relative.is_absolute()
        or any(part in {".", ".."} for part in relative.parts)
        or relative.as_posix() != rel
        or not allowed
    ):
        raise RuntimeError(f"lock contains an unsafe managed path: {rel}")
    parent = dest
    for part in relative.parts[:-1]:
        parent /= part
        if parent.is_symlink():
            raise RuntimeError(f"managed path has a symlinked parent: {rel}")
        if parent.exists() and not parent.is_dir():
            raise RuntimeError(f"managed path has a non-directory parent: {rel}")
    return dest / relative


def retired_managed_conflicts(
    dest: Path,
    retired: list[str],
    previous_hashes: dict[str, str],
) -> list[str]:
    conflicts: list[str] = []
    for rel in retired:
        target = managed_lock_target(dest, rel)
        if not target.exists() and not target.is_symlink():
            continue
        if (
            target.is_symlink()
            or not target.is_file()
            or sha256_file(target) != previous_hashes[rel]
        ):
            conflicts.append(rel)
    return conflicts


def remove_retired_managed_files(dest: Path, retired: list[str]) -> list[str]:
    targets = [(rel, managed_lock_target(dest, rel)) for rel in retired]
    for rel, target in targets:
        if not target.exists() and not target.is_symlink():
            continue
        if not target.is_symlink() and not target.is_file():
            raise RuntimeError(f"retired managed target is not a file: {rel}")
    removed: list[str] = []
    for rel, target in targets:
        if not target.exists() and not target.is_symlink():
            continue
        target.unlink()
        removed.append(rel)
        # Remove directories the retirement left empty (a retired skill leaves no empty folder).
        parent = target.parent
        while parent != dest and dest in parent.parents and parent.is_dir() and not any(parent.iterdir()):
            parent.rmdir()
            parent = parent.parent
    return removed


def cmd_update(args: argparse.Namespace) -> int:
    dest = Path(args.dest).expanduser().resolve()
    lock_path = dest / LOCK_NAME
    if not lock_path.is_file():
        print("no lock file; run init or doctor", file=sys.stderr)
        return 2
    lock = load_lock(lock_path)
    try:
        instance = load_instance(dest / "instance.yaml")
    except InstanceError as error:
        print(
            json.dumps(
                {"status": "invalid-instance", "error": str(error)},
                indent=2,
            )
        )
        return 2
    target_adapters = instance["adapters"]
    target_sources = managed_sources(target_adapters)
    previous_hashes = lock["managed_hashes"]
    retired = sorted(set(previous_hashes) - set(target_sources))
    current = tree_hashes(dest, target_adapters)
    owned = set(target_sources)
    drifted = [
        rel
        for rel, digest in previous_hashes.items()
        if rel in owned and current.get(rel) != digest
    ]
    drifted.extend(
        ownership_conflicts(dest, target_adapters, set(previous_hashes))
    )
    try:
        drifted.extend(retired_managed_conflicts(dest, retired, previous_hashes))
    except RuntimeError as error:
        print(
            json.dumps(
                {"status": "invalid-lock", "error": str(error)},
                indent=2,
            )
        )
        return 3
    drifted.extend(native_runtime.conflicts(dest, native_runtime.release(DIST), lock.get("runtime_release")))
    drifted = sorted(set(drifted))
    if drifted and not args.force:
        # The lock records what the distribution installed; Git shows what changed locally.
        print(json.dumps({"status": "drift", "files": drifted}, indent=2))
        print("kernel files changed locally (see git diff); keep cell logic in cell-owned files, then pass --force to restore the distribution version", file=sys.stderr)
        return 3
    try:
        preflight_kernel(dest, target_adapters)
        removed = remove_retired_managed_files(dest, retired)
    except RuntimeError as error:
        print(
            json.dumps(
                {"status": "invalid-lock", "error": str(error)},
                indent=2,
            )
        )
        return 3
    copy_kernel(dest, target_adapters)
    ensure_gitignore_lines(dest)
    ensure_obsidian_ignore_filters(dest)
    write_lock(dest, target_adapters)
    print(
        json.dumps(
            {
                "status": "updated",
                "dest": str(dest),
                "kernel_version": dist_version(),
                "retired_managed_files": removed,
            },
            indent=2,
        )
    )
    return 0


def cmd_doctor(args: argparse.Namespace) -> int:
    dest = Path(args.dest).expanduser().resolve()
    state = dest_state(dest)
    payload: dict[str, Any] = {"dest": str(dest), "state": state}
    tracked = subprocess.run(
        ["git", "-C", str(dest), "ls-files", "--error-unmatch", "--", PERSONAL_AGENTS],
        capture_output=True,
        check=False,
    ).returncode == 0
    payload["personal_instructions"] = {
        "path": PERSONAL_AGENTS,
        "exists": (dest / PERSONAL_AGENTS).is_file(),
        "ignored": personal_agents_ignored(dest),
        "tracked": tracked,
    }
    invalid_catalog = False
    try:
        catalog = load_catalog(dest)
        payload["vault_catalog"] = {
            "status": "valid" if (dest / CATALOG_PATH).exists() else "absent",
            "count": len(catalog["vaults"]),
        }
    except CatalogError as error:
        payload["vault_catalog"] = {"status": "invalid", "error": str(error)}
        invalid_catalog = True
    invalid_instance = False
    instance = None
    if state == "installed":
        try:
            instance = load_instance(dest / "instance.yaml")
            payload["instance"] = {"status": "valid"}
        except InstanceError as error:
            payload["instance"] = {
                "status": "invalid",
                "error": str(error),
            }
            invalid_instance = True
        lock = load_lock(dest / LOCK_NAME)
        adapters = lock.get("adapters") or []
        payload["adapters_installed"] = adapters
        payload["adapters_configured"] = instance["adapters"] if instance is not None else None
        payload["adapter_configuration_drift"] = (
            sorted(set(adapters) ^ set(instance["adapters"])) if instance is not None else []
        )
        available_adapters = [name for name in adapters if (DIST / "adapters" / name).is_dir()]
        current = tree_hashes(dest, available_adapters)
        expected = distribution_hashes(available_adapters)
        payload["kernel_version_installed"] = lock.get("kernel_version")
        payload["kernel_version_dist"] = dist_version()
        payload["lock_version"] = lock.get("version")
        payload["portable_lock"] = lock.get("version") == "4"
        payload["native_runtime"] = native_runtime.status(dest, native_runtime.release(DIST), lock.get("runtime_release"))
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
        payload["topology_drift"] = [
            relative for relative, expected_target in (
                (".claude/skills", "../.agents/skills"),
            )
            if not (dest / relative).is_symlink()
            or os.readlink(dest / relative) != expected_target
        ]
        try:
            payload["orientation"] = orientation_status(dest)
        except InstanceError as error:
            payload["orientation"] = {"ready": False, "issues": [str(error)]}
        payload["start_here"] = payload.get("orientation", {}).get(
            "start_here", ["00-Home.md", "instance.yaml"]
        )
    print(json.dumps(payload, indent=2, ensure_ascii=False))
    if invalid_instance or invalid_catalog:
        return 2
    if getattr(args, "strict", False) and (
        state != "installed" or not payload.get("portable_lock")
        or not payload.get("reproducible_distribution")
        or payload.get("distribution_dirty_dist")
        or payload.get("drift") or payload.get("topology_drift")
        or payload.get("adapter_configuration_drift")
        or not payload.get("managed_matches_dist")
        or payload.get("native_runtime", {}).get("status") != "ready"
        or not payload["native_runtime"].get("release_matches_dist")
        or not payload["personal_instructions"]["ignored"]
        or payload["personal_instructions"]["tracked"]
    ):
        return 2
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
    parser.add_argument("--tracker", action="append", default=[])
    parser.add_argument("--adapter", action="append", default=[])
    parser.add_argument("--platform", action="append", default=[], help="cloud the systems run on (gcp, aws, azure); repeat for several")
    parser.add_argument("--evidence-profile", default="")
    parser.add_argument("--locale", default="")
    parser.add_argument("--vault-remote", default="")
    parser.add_argument("--github-org", default="")
    parser.add_argument("--repo-prefix", action="append", default=[])
    parser.add_argument("--discovery-root", action="append", default=[])
    parser.add_argument("--reference-branch", action="append", default=[], help="reference branch in preference order; repeat (default: main, then master)")
    parser.add_argument("--disable-topics", action="store_true")
    parser.add_argument("--yes", action="store_true")
    parser.add_argument("--force", action="store_true")
    parser.add_argument("--strict", action="store_true", help="doctor fails for invalid, unreproducible, or drifted installations")
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
