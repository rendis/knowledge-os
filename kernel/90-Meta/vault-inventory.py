#!/usr/bin/env python3
"""Inventory source repositories using instance.yaml prefixes and org."""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

from instance import load_instance
from vault_frontmatter import read_frontmatter

# workspace-config.py is the CLI filename; import the sibling module by path.
import importlib.util

_spec = importlib.util.spec_from_file_location(
    "workspace_config",
    Path(__file__).with_name("workspace-config.py"),
)
_workspace = importlib.util.module_from_spec(_spec)
assert _spec.loader is not None
_spec.loader.exec_module(_workspace)
load_config = _workspace.load_config
locate_repository = _workspace.locate_repository


def git_head(path: Path) -> str | None:
    result = subprocess.run(
        ["git", "-C", str(path), "rev-parse", "--short=12", "HEAD"],
        capture_output=True,
        text=True,
        check=False,
    )
    return result.stdout.strip() or None


def in_scope(name: str, prefixes: list[str], extras: list[str]) -> bool:
    if name in extras:
        return True
    if not prefixes:
        return True
    return any(name.startswith(prefix) or name == prefix.rstrip("-") for prefix in prefixes)


def inventory(root: Path) -> dict:
    instance = load_instance(root / "instance.yaml")
    prefixes = [str(item) for item in instance["sources"]["repo_prefixes"] if str(item).strip()]
    notes = []
    for path in (root / "20-Repos").rglob("*.md"):
        fields = read_frontmatter(path)
        notes.append(
            {
                "note": path.stem,
                "sistema": fields.get("sistema"),
                "commit-analizado": fields.get("commit-analizado"),
                "fecha-analisis": fields.get("fecha-analisis"),
                "rama-analizada": fields.get("rama-analizada"),
            }
        )
    return {
        "cell": instance["cell"]["name"],
        "github_org": instance["sources"]["github_org"],
        "repo_prefixes": prefixes,
        "notes": notes,
        "source_context": load_config(root)["source_context"],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault-root", type=Path, default=Path.cwd())
    parser.add_argument("--format", default="json")
    parser.add_argument("--locate", default="")
    args = parser.parse_args()
    root = args.vault_root.resolve()
    if args.locate:
        print(json.dumps(locate_repository(root, args.locate), indent=2))
        return 0
    data = inventory(root)
    if args.format == "markdown":
        print(f"# Inventory — {data['cell']}")
        print(f"org: {data['github_org'] or '(unset)'}")
        print(f"prefixes: {', '.join(data['repo_prefixes']) or '(any)'}")
        print(f"documented repos: {len(data['notes'])}")
        return 0
    print(json.dumps(data, indent=2, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
