#!/usr/bin/env python3
"""Report static evidence for repo notes without editing the vault.

The script scans the ordered configuration-backed source roots returned by the
vault resolver, an exact authorized source repository override, and the
Obsidian repo notes. It is a helper for resolving evidence from source code,
versioned config, GitHub Actions, deploy manifests, and cross-repository
references.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlparse
from typing import Any

from instance import load_instance
from cell_scope import approved_cross_scope_repositories
from vault_frontmatter import split_frontmatter

ROOT = Path(__file__).resolve().parents[1]
NOTES_ROOT = ROOT / "20-Repos"
VAULT_RESOLVER = ROOT / "90-Meta/resolve-vault.py"
VAULT_CONTAINER_REPO = ""
APP_PREFIXES: tuple[str, ...] = ()
APPROVED_CROSS_APP_REPOSITORIES: set[str] = set()
SYSTEM_PREFIX_BY_ID: dict[str, str] = {}

SKIP_DIRS = {
    ".git",
    ".superpowers",
    "node_modules",
    "vendor",
    "dist",
    "build",
    ".next",
    "coverage",
    "public",
    "target",
    "__pycache__",
}

STOP_TOKENS = {
    "README",
    "PENDING",
    "PROJECT_NAME",
    "APP_COUNTRY",
    "APP_PORT",
    "NODE_ENV",
    "PORT",
}

DEPLOY_MARKERS = (
    ".github/workflows",
    "k8s",
    "kubernetes",
    "kustomization",
    "cloudrun",
    "deploy",
    "helm",
    "cloudbuild.yaml",
    "cloudbuild.yml",
    "Dockerfile",
)


def configure_scope(root: Path) -> None:
    """Derive scan scope and system-to-prefix mapping from cell data."""
    instance = load_instance(root / "instance.yaml")
    prefixes = [str(value).rstrip("-") for value in instance["sources"]["repo_prefixes"]]
    global APP_PREFIXES, APPROVED_CROSS_APP_REPOSITORIES
    global SYSTEM_PREFIX_BY_ID, VAULT_CONTAINER_REPO, STOP_TOKENS
    APP_PREFIXES = tuple(f"{value.lower()}-" for value in prefixes if value)
    STOP_TOKENS = STOP_TOKENS | set(prefixes)

    system_prefixes: dict[str, str] = {}
    for system in instance["systems"]:
        aliases = [str(value) for value in system["aliases"]]
        match = next((value for value in aliases if value in prefixes), None)
        if match:
            system_prefixes[system["id"]] = f"{match}-"
    SYSTEM_PREFIX_BY_ID = system_prefixes

    remote = instance["vault"]["remote"].rstrip("/")
    VAULT_CONTAINER_REPO = remote.rsplit("/", 1)[-1].removesuffix(".git") if remote else ""
    APPROVED_CROSS_APP_REPOSITORIES = {
        name.lower()
        for name in approved_cross_scope_repositories(
            root,
            APP_PREFIXES,
            VAULT_CONTAINER_REPO,
        )
    }


if (ROOT / "instance.yaml").is_file():
    configure_scope(ROOT)


def is_in_scope_repository(name: str) -> bool:
    lowered = name.lower()
    return lowered.startswith(APP_PREFIXES) or lowered in APPROVED_CROSS_APP_REPOSITORIES


@dataclass
class Note:
    path: Path
    name: str
    frontmatter: dict[str, Any]
    body: str

    @property
    def aliases(self) -> list[str]:
        raw = self.frontmatter.get("aliases", [])
        values = raw if isinstance(raw, list) else [raw]
        return [str(value) for value in values if value]

    @property
    def source_repo_name(self) -> str:
        for alias in self.aliases:
            if alias.startswith("APP"):
                return alias
        prefix = SYSTEM_PREFIX_BY_ID.get(self.path.parent.name.lower(), "")
        return prefix + self.name


def run_git(repo: Path, *args: str, preserve_output: bool = False) -> str:
    try:
        output = subprocess.check_output(
            ["git", "-C", str(repo), *args],
            stderr=subprocess.DEVNULL,
            text=True,
        )
        return output if preserve_output else output.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return ""


def read_note(path: Path) -> Note:
    text = path.read_text(encoding="utf-8")
    frontmatter, body = split_frontmatter(text)
    return Note(path=path, name=path.stem, frontmatter=frontmatter, body=body)


def iter_note_paths() -> list[Path]:
    return sorted(NOTES_ROOT.glob("*/*.md"))


def should_skip(path: Path) -> bool:
    return any(part in SKIP_DIRS for part in path.parts)


def iter_source_files(repo: Path) -> list[Path]:
    """Select tracked regular files and read their current working-tree bytes.

    Untracked local artifacts (including ignored ones) and symlinks are excluded. A tracked file
    may be dirty; HEAD identifies the checkout baseline, not the scanned bytes.
    """
    raw = run_git(repo, "ls-files", "-z", "--cached", preserve_output=True)
    files: list[Path] = []
    for name in sorted(set(raw.split("\0"))):
        if not name:
            continue
        path = repo / name
        relative = Path(name)
        ancestors = [repo.joinpath(*relative.parts[:index]) for index in range(1, len(relative.parts) + 1)]
        if (
            should_skip(relative)
            or any(candidate.is_symlink() for candidate in ancestors)
            or not path.resolve().is_relative_to(repo.resolve())
            or not path.is_file()
        ):
            continue
        if path.stat().st_size > 1_500_000:
            continue
        files.append(path)
    return files


def read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return ""


def deploy_inventory(repo: Path) -> list[str]:
    found: set[str] = set()
    for path in iter_source_files(repo):
        relative = path.relative_to(repo).as_posix()
        for marker in DEPLOY_MARKERS:
            if relative == marker or relative.startswith(marker + "/"):
                found.add(marker)
        if relative.startswith(".github/workflows/"):
            found.add(relative)
    return sorted(found)


def extract_tokens(note: Note) -> list[str]:
    text = note.body + "\n" + "\n".join(str(value) for value in note.frontmatter.values())
    tokens = set(note.aliases + [note.name, note.source_repo_name])
    tokens.update(re.findall(r"\b[A-Z][A-Z0-9_]{3,}\b", text))
    tokens.update(re.findall(r"\bproj-[a-z0-9][a-z0-9-]{8,}\b", text))
    tokens.update(re.findall(r"\b[a-z0-9]+(?:-[a-z0-9]+){2,}\b", text))
    return sorted(t for t in tokens if len(t) >= 6 and t not in STOP_TOKENS)


def normalize_remote(remote: str) -> str | None:
    value = remote.strip()
    if not value:
        return None
    scp = re.fullmatch(r"(?:[^@]+@)?([^:]+):(.+)", value)
    if scp and "://" not in value:
        host, path = scp.groups()
    else:
        parsed = urlparse(value if "://" in value else f"https://{value}")
        host, path = parsed.hostname or "", parsed.path
    path = path.strip("/")
    if path.casefold().endswith(".git"):
        path = path[:-4]
    return f"{host.lower()}/{path.lower()}" if host and path else None


def source_repo_identity(repo: Path) -> str | None:
    remote = run_git(repo, "remote", "get-url", "origin")
    return normalize_remote(remote)


def source_repo_name(repo: Path) -> str:
    remote = run_git(repo, "remote", "get-url", "origin")
    raw = remote.rstrip("/").rsplit("/", 1)[-1] if remote else repo.name
    return raw.removesuffix(".git")


@dataclass(frozen=True)
class SourceRepo:
    path: Path
    identity: str
    name: str
    branch: str
    commit: str


class SourceAmbiguityError(ValueError):
    pass


def source_repo(path: Path) -> SourceRepo | None:
    identity = source_repo_identity(path)
    if identity is None:
        return None
    return SourceRepo(
        path=path.resolve(),
        identity=identity,
        name=source_repo_name(path),
        branch=run_git(path, "branch", "--show-current") or "detached",
        commit=run_git(path, "rev-parse", "HEAD") or "unknown",
    )


def source_repos(
    source_roots: list[Path],
    extra_source: Path | None = None,
) -> tuple[list[SourceRepo], list[str]]:
    paths: list[Path] = []
    seen_paths: set[Path] = set()

    def add_path(path: Path) -> None:
        resolved = path.resolve()
        if resolved not in seen_paths:
            seen_paths.add(resolved)
            paths.append(resolved)

    if extra_source:
        add_path(extra_source)
    for source_root in source_roots:
        for path in sorted(source_root.iterdir(), key=lambda item: item.name):
            if (path / ".git").exists():
                add_path(path)

    records: list[SourceRepo] = []
    for path in paths:
        record = source_repo(path)
        if record is None:
            continue
        lowered = record.name.lower()
        if lowered == VAULT_CONTAINER_REPO.lower() or not is_in_scope_repository(lowered):
            continue
        records.append(record)
    extra_identity = source_repo_identity(extra_source) if extra_source else None
    by_identity: dict[str, list[SourceRepo]] = {}
    for record in records:
        by_identity.setdefault(record.identity, []).append(record)

    selected: list[SourceRepo] = []
    warnings: list[str] = []
    for identity, candidates in by_identity.items():
        if extra_source and identity == extra_identity:
            selected.append(next(item for item in candidates if item.path == extra_source.resolve()))
            continue
        states = {(item.branch, item.commit) for item in candidates}
        if len(states) > 1:
            details = "; ".join(
                f"{item.path} ({item.branch}, {item.commit[:12]})"
                for item in candidates
            )
            raise SourceAmbiguityError(
                f"divergent clones for remote repository {candidates[0].name}: {details}"
            )
        canonical = [item for item in candidates if item.path.name == item.name]
        chosen = min(canonical or candidates, key=lambda item: str(item.path))
        selected.append(chosen)
        if len(candidates) > 1:
            ignored = ", ".join(str(item.path) for item in candidates if item != chosen)
            warnings.append(
                f"duplicate clones for {chosen.name}; using {chosen.path}; ignored {ignored}"
            )
    return selected, warnings


def resolved_source_context() -> dict[str, object]:
    result = subprocess.run(
        [sys.executable, "-B", str(VAULT_RESOLVER), "--path", str(ROOT)],
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode:
        raise ValueError(result.stdout.strip() or result.stderr.strip())
    data = json.loads(result.stdout)
    context = data["source_context"]
    if context["status"] != "resolved":
        raise ValueError(
            "no source roots resolved; initialize or update .knowledge-os-config.yaml "
            "through configure-workspace"
        )
    return context


def validate_extra_source(
    extra_source: Path,
    source_roots: list[Path],
    *,
    clone_root: Path | None,
    clone_authorized: bool,
) -> None:
    if extra_source.parent in source_roots:
        return
    if not clone_authorized or clone_root is None:
        raise ValueError("--source-repo outside configured roots requires managed-clone authority")
    if not (extra_source / ".git").is_file():
        raise ValueError("--source-repo outside configured roots must be a linked Git worktree")
    raw_common = run_git(extra_source, "rev-parse", "--git-common-dir")
    if not raw_common:
        raise ValueError("--source-repo has no resolvable Git common directory")
    common = Path(raw_common)
    if not common.is_absolute():
        common = (extra_source / common).resolve()
    else:
        common = common.resolve()
    if common.name != ".git" or common.parent.parent.resolve() != clone_root.resolve():
        raise ValueError(
            "--source-repo is not a linked worktree of an immediate child of the managed-clone root"
        )


def find_cross_refs(
    tokens: list[str],
    own_repo: Path | None,
    max_hits: int,
    repositories: list[SourceRepo],
) -> list[str]:
    hits: list[str] = []
    lowered = [(token, token.lower()) for token in tokens]
    for source in repositories:
        repo = source.path
        if own_repo and repo == own_repo.resolve():
            continue
        for path in iter_source_files(repo):
            text = read_text(path)
            if not text:
                continue
            lower = text.lower()
            matched = [token for token, token_lower in lowered if token_lower in lower]
            if not matched:
                continue
            rel = Path(repo.name) / path.relative_to(repo)
            hits.append(f"{rel}: {', '.join(matched[:5])}")
            if len(hits) >= max_hits:
                return hits
    return hits


def limitation_lines(note: Note) -> list[str]:
    match = re.search(
        r"^## Limitaciones y desconocimientos\s*\n(?P<body>.*?)(?=^##\s|\Z)",
        note.body,
        re.MULTILINE | re.DOTALL,
    )
    if not match:
        return []
    return [
        line.strip()
        for line in match.group("body").splitlines()
        if line.strip().startswith("-")
    ]


def selected_notes(args: argparse.Namespace) -> list[Note]:
    notes = [read_note(path) for path in iter_note_paths()]
    if args.por_confirmar:
        notes = [
            note
            for note in notes
            if "#por-confirmar" in note.body
            or any(value == "por-confirmar" for key, value in note.frontmatter.items() if key.startswith("cobertura-"))
        ]
    if args.repo:
        wanted = set(args.repo)
        notes = [
            note
            for note in notes
            if note.name in wanted or note.source_repo_name in wanted
        ]
    return notes


def report_note(
    note: Note,
    max_hits: int,
    repositories: list[SourceRepo],
) -> None:
    source = next(
        (item for item in repositories if item.name.lower() == note.source_repo_name.lower()),
        None,
    )
    print(f"## {note.name}")
    print(f"- Nota: `{note.path.relative_to(ROOT)}`")
    if source is None:
        print("- Repo fuente: no encontrado")
        print()
        return
    repo = source.path
    print(f"- Repo fuente: `{repo}`")

    branch = source.branch
    commit = source.commit[:12]
    files = [path for path in iter_source_files(repo) if ".git" not in path.parts]
    deploy = deploy_inventory(repo)
    print(f"- Rama/HEAD de referencia: `{branch}` / `{commit}` (no identifica los bytes escaneados)")
    print("- Evidencia: archivos rastreados del working tree local; incluye cambios sin commit")
    print(f"- Archivos rastreados locales escaneados: {len(files)}")
    print(f"- Deploy/config estático: {', '.join(f'`{d}`' for d in deploy) if deploy else 'no observado'}")

    limitations = limitation_lines(note)
    if limitations:
        print("- Limitaciones en nota:")
        for line in limitations:
            print(f"  {line}")

    tokens = extract_tokens(note)
    hits = find_cross_refs(tokens, repo, max_hits, repositories)
    print("- Referencias cross-repo:")
    if hits:
        for hit in hits:
            print(f"  - {hit}")
    else:
        print("  - no observadas en clones locales")
    print()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--por-confirmar",
        action="store_true",
        help="solo notas con cobertura por-confirmar o #por-confirmar inline",
    )
    parser.add_argument("--repo", action="append", help="filtrar por basename de nota o nombre completo de repo")
    parser.add_argument("--max-hits", type=int, default=12, help="máximo de referencias cross-repo por nota")
    parser.add_argument(
        "--source-repo",
        type=Path,
        help="checkout exacto ya autorizado por el flujo de worktree administrado",
    )
    args = parser.parse_args()
    configure_scope(ROOT)

    try:
        context = resolved_source_context()
        source_roots = [Path(item["path"]) for item in context["roots"]]
        resolver_warnings = list(context["warnings"])
    except (ValueError, json.JSONDecodeError, KeyError) as error:
        parser.error(str(error))
    extra_source = args.source_repo.expanduser().resolve() if args.source_repo else None
    if extra_source and not (extra_source / ".git").exists():
        parser.error(f"--source-repo is not a Git repository: {extra_source}")
    if extra_source and source_repo_identity(extra_source) is None:
        parser.error(f"--source-repo has no resolvable origin remote: {extra_source}")
    if extra_source:
        try:
            validate_extra_source(
                extra_source,
                source_roots,
                clone_root=Path(context["clone_root"]) if context["clone_root"] else None,
                clone_authorized=bool(context["clone_authorized"]),
            )
        except ValueError as error:
            parser.error(str(error))

    try:
        repositories, duplicate_warnings = source_repos(source_roots, extra_source)
    except SourceAmbiguityError as error:
        parser.error(str(error))
    notes = selected_notes(args)
    print("# Static evidence scan")
    print()
    print(f"- Notas seleccionadas: {len(notes)}")
    print(f"- Repos fuente disponibles: {len(repositories)}")
    print("- Modo: solo lectura; escanea archivos rastreados locales, incluidos cambios sin commit; excluye archivos no rastreados (incluidos ignorados) y enlaces simbólicos; no prueba contenido de HEAD ni runtime")
    for warning in [*resolver_warnings, *duplicate_warnings]:
        print(f"- Advertencia de fuentes: {warning}")
    print()
    for note in notes:
        report_note(note, args.max_hits, repositories)
    return 0


if __name__ == "__main__":
    sys.exit(main())
