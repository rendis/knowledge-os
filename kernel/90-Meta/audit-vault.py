#!/usr/bin/env python3
"""Validate closed note contracts for an installed cell vault."""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

from instance import load_instance, InstanceError
from vault_frontmatter import read_frontmatter

ALLOWED_TIPOS = {
    "indice",
    "sistema",
    "servicio",
    "componente",
    "recurso-runtime",
    "repositorio",
    "topic",
    "flujo",
    "integracion-externa",
    "glosario",
    "operacional",
    "aprendizaje",
}
FOLDER_TIPO = {
    "10-Sistemas": {"sistema", "indice"},
    "15-Arquitectura": {"servicio", "componente", "recurso-runtime"},
    "20-Repos": {"repositorio"},
    "25-Topics": {"topic"},
    "30-Flujos": {"flujo", "indice"},
    "40-Integraciones": {"integracion-externa"},
    "50-Glosario": {"glosario"},
    "60-Operacion": {"operacional", "indice"},
    "70-Aprendizajes": {"aprendizaje", "indice"},
    "90-Meta": {"indice"},
}


def iter_notes(root: Path) -> list[Path]:
    skip = {".git", ".agents", ".obsidian", ".investigations", ".operations", "plan"}
    notes: list[Path] = []
    for path in root.rglob("*.md"):
        if any(part in skip or part.startswith(".") for part in path.relative_to(root).parts[:-1]):
            continue
        notes.append(path)
    return notes


def audit(root: Path) -> list[str]:
    issues: list[str] = []
    try:
        instance = load_instance(root / "instance.yaml")
    except InstanceError as error:
        return [f"instance.yaml: {error}"]
    enabled = set(instance["graph"]["enabled_types"])
    system_names = {item["name"] for item in instance["systems"]}
    home = root / "00-Home.md"
    if not home.is_file():
        issues.append("missing 00-Home.md")
    else:
        fields = read_frontmatter(home)
        if fields.get("tipo") not in {None, "indice", []}:
            if fields.get("tipo") != "indice":
                issues.append("00-Home.md must have tipo: indice")
    for system in instance["systems"]:
        note = root / "10-Sistemas" / f"{system['name']}.md"
        if not note.is_file():
            issues.append(f"missing system note for {system['name']}")
    for note in iter_notes(root):
        rel = note.relative_to(root).as_posix()
        fields = read_frontmatter(note)
        tipo = fields.get("tipo")
        if isinstance(tipo, list):
            tipo = tipo[0] if tipo else None
        if not tipo:
            if rel.startswith("90-Meta/"):
                continue
            issues.append(f"{rel}: missing tipo")
            continue
        if tipo not in ALLOWED_TIPOS:
            issues.append(f"{rel}: unknown tipo {tipo}")
            continue
        if tipo not in enabled and tipo not in {"indice", "sistema"}:
            issues.append(f"{rel}: tipo {tipo} disabled in instance.yaml")
        top = rel.split("/", 1)[0]
        if top in FOLDER_TIPO and tipo not in FOLDER_TIPO[top] and not rel.endswith("00-Home.md"):
            issues.append(f"{rel}: tipo {tipo} does not belong in {top}")
        if tipo == "sistema" and fields.get("tipo") == "sistema":
            name = note.stem
            if name not in system_names and rel.startswith("10-Sistemas/"):
                issues.append(f"{rel}: system {name} is not declared in instance.yaml")
    return issues


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    args = parser.parse_args()
    root = args.root.resolve()
    issues = audit(root)
    if issues:
        print(f"FAIL {len(issues)} issue(s)")
        for item in issues:
            print(f"- {item}")
        return 1
    print("PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
