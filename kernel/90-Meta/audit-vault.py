#!/usr/bin/env python3
"""Validate the structural contract of an installed cell vault."""
from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import date
from pathlib import Path
from typing import Any

from instance import load_instance
from vault_frontmatter import split_frontmatter

FORBIDDEN_RELATION_FILES = {
    "relations.jsonl",
    "relations.csv",
    "relations.sqlite",
    "relations.db",
}
SYNC_PROCESS_LEAKAGE = re.compile(
    r"\b(?:la sync|la review|claims? (?:aceptad|rechazad)|por el gate|"
    r"conocimiento durable adicional publicado)\b",
    re.IGNORECASE,
)

REPO_REQUIRED = {
    "aliases",
    "sistema",
    "tipo",
    "lenguaje",
    "gatillado-por",
    "publica-en",
    "consume-de",
    "lee-de",
    "escribe-en",
    "usa-infra",
    "participa-en",
    "cobertura-entradas",
    "cobertura-salidas",
    "cobertura-datos",
    "cobertura-infra",
    "cobertura-flujos",
    "ultima-auditoria",
    "commit-analizado",
    "fecha-analisis",
    "rama-analizada",
    "tags",
}
REPO_LIST_FIELDS = {
    "aliases",
    "gatillado-por",
    "publica-en",
    "consume-de",
    "lee-de",
    "escribe-en",
    "usa-infra",
    "participa-en",
    "tags",
}
REPO_SECTIONS = {
    "Propósito",
    "Gatillo",
    "Qué hace",
    "Entradas y salidas",
    "Persistencia y datos",
    "Infraestructura y scheduling",
    "Relaciones",
    "Limitaciones y desconocimientos",
}
REPO_TYPES = {
    "api",
    "bff",
    "adapter",
    "http-adapter",
    "suscriptor",
    "publicador",
    "job",
    "function",
    "frontend",
    "libreria",
    "scaffold",
}
COVERAGE_VALUES = {"completo", "parcial", "no-aplica", "por-confirmar"}
COVERAGE_FIELDS = {
    "cobertura-entradas",
    "cobertura-salidas",
    "cobertura-datos",
    "cobertura-infra",
    "cobertura-flujos",
}

ARCH_COMMON = {"tipo", "sistema", "participa-en", "ultima-auditoria", "tags"}
ARCH_SERVICE = {"compuesto-por"}
ARCH_COMPONENT = {"implementado-por"}
ARCH_RUNTIME_REQUIRED = {
    "nombre-raw",
    "plataforma",
    "proyecto",
    "ambiente",
    "ultima-verificacion",
}
ARCH_RUNTIME_OPTIONAL = {"region", "ejecuta", "gatilla-a", "usa-infra"}
ARCH_ALLOWED = (
    ARCH_COMMON
    | ARCH_SERVICE
    | ARCH_COMPONENT
    | ARCH_RUNTIME_REQUIRED
    | ARCH_RUNTIME_OPTIONAL
)
ARCH_SECTIONS = {
    "servicio": {
        "Qué representa",
        "Responsabilidad y límites",
        "Unidades que lo componen",
        "Interfaces y datos",
        "Relaciones",
        "Limitaciones y desconocimientos",
    },
    "componente": {
        "Propósito",
        "Implementación",
        "Interfaces",
        "Runtime e infraestructura",
        "Relaciones",
        "Limitaciones y desconocimientos",
    },
    "recurso-runtime": {
        "Qué representa",
        "Proyecto y ambiente",
        "Ejecución o disparo",
        "Evidencia",
        "Relaciones",
        "Limitaciones y desconocimientos",
    },
}

TOPIC_FIELDS = {"tipo", "nombre-raw", "sistema", "tags"}
TOPIC_SECTIONS = {
    "Qué representa",
    "Contrato",
    "Infraestructura verificada",
    "Limitaciones y desconocimientos",
}
TOPIC_FORBIDDEN_HEADINGS = {
    "Productores",
    "Consumidores",
    "Productores y consumidores",
}
INTEGRATION_FIELDS = {"tipo", "aliases", "tags"}
INTEGRATION_SECTIONS = {
    "Qué es",
    "Cómo se usa",
    "Contratos relevantes",
    "Infraestructura o ownership",
    "Limitaciones y desconocimientos",
}
FLOW_FIELDS = {"tipo", "sistema", "tags"}
FLOW_SECTIONS = {
    "Qué resuelve",
    "Disparador",
    "Paso a paso",
    "Diagrama de flujo",
    "Diagrama de componentes",
    "Participantes",
    "Pendientes",
}
SYSTEMS: set[str] = set()
SYSTEM_FIELDS = {"tipo", "aliases", "tags"}
SYSTEM_CONTRACTS: dict[str, tuple[str | None, str]] = {}
SYSTEM_TAGS: set[str] = set()
INDEX_FIELDS = {"tipo", "tags"}
GLOSSARY_REQUIRED = {"tipo", "tags"}
GLOSSARY_ALLOWED = GLOSSARY_REQUIRED | {"aliases"}
OPERATIONAL_COMMON_REQUIRED = {
    "tipo",
    "clase",
    "estado",
    "owner",
    "ultima-verificacion",
    "area",
    "canales",
    "tags",
}
OPERATIONAL_OPTIONAL = {"relacionado-con"}
OPERATIONAL_ALLOWED = OPERATIONAL_COMMON_REQUIRED | OPERATIONAL_OPTIONAL | {"report-id"}
OPERATIONAL_STATES = {"borrador", "vigente", "retirado"}
OPERATIONAL_AREAS: dict[str, str] = {}
OPERATIONAL_CLASSES = {"guia", "catalogo", "estandar", "procedimiento", "reporte"}
REPORT_ID = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
OPERATIONAL_SECTIONS = {
    "guia": {
        "Objetivo",
        "Prerrequisitos",
        "Procedimiento",
        "Validación",
        "Problemas y escalamiento",
    },
    "catalogo": {
        "Propósito",
        "Catálogo",
        "Criterios de uso",
        "Mantenimiento",
        "Limitaciones",
    },
    "estandar": {
        "Objetivo",
        "Alcance",
        "Reglas",
        "Plantillas",
        "Validación",
        "Excepciones",
    },
    "procedimiento": {
        "Objetivo",
        "Disparador",
        "Entradas requeridas",
        "Decisiones",
        "Pasos",
        "Evidencia de finalización",
        "Fallos y recuperación",
        "Limitaciones",
    },
    "reporte": {
        "Propósito",
        "Audiencia y decisiones",
        "Definiciones y grano",
        "Fuente y alcance",
        "Período, filtros y exclusiones",
        "Salida",
        "Validación",
        "Mantenimiento",
        "Limitaciones",
    },
}
OPERATIONAL_STEP_HEADER = (
    "| ID | Acción | Capacidad | Efecto externo | Entrada | Salida | Continuar si |"
)
LEARNING_FIELDS = {
    "tipo",
    "estado",
    "resultado",
    "aplica-a",
    "dimensiones",
    "investigaciones-origen",
    "fecha-conclusion",
    "ultima-validacion",
    "supersede-a",
    "tags",
}
LEARNING_STATES = {"vigente", "cuestionado", "superado"}
LEARNING_RESULTS = {
    "cambio-adoptado",
    "baseline-conservado",
    "alternativa-descartada",
    "hallazgo-metodologico",
}
LEARNING_SECTION_ORDER = (
    "Resumen",
    "Pregunta y contexto",
    "Baseline y alternativas",
    "Método",
    "Evidencia acumulada",
    "Decisión y justificación",
    "Enseñanza reutilizable",
    "Cuándo aplica",
    "Cuándo no aplica",
    "Implementación y despliegue",
    "Limitaciones y revalidación",
    "Trazabilidad",
)
LEARNING_SECTIONS = set(LEARNING_SECTION_ORDER)
LEARNING_LIST_FIELDS = {
    "aplica-a",
    "dimensiones",
    "investigaciones-origen",
    "supersede-a",
    "tags",
}
WIKILINK_VALUE = re.compile(r"^\[\[[^\]|#]+\]\]$")
LEARNING_DIMENSION = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
INVESTIGATION_ID = re.compile(
    r"^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$"
)
LOCAL_IGNORED_REFERENCE = re.compile(
    r"(?:\.investigations|\.operations|\.knowledge-os-handoffs)[/\\]"
    r"|(?<![A-Za-z0-9_-])\.plan[/\\]"
)
LEARNING_EVIDENCE_FIELDS = (
    "Investigación",
    "Fuentes durables",
    "Contexto y exclusiones",
    "Método y medidas",
    "Resultado",
    "Aporte a la conclusión",
)


def configure_cell_contract(root: Path) -> None:
    """Load cell identity and operational areas from versioned cell data."""
    instance = load_instance(root / "instance.yaml")
    systems = instance["systems"]
    global SYSTEMS, SYSTEM_CONTRACTS, SYSTEM_TAGS, OPERATIONAL_AREAS
    SYSTEMS = {f"[[{item['name']}]]" for item in systems}
    SYSTEM_CONTRACTS = {
        item["name"]: (
            item["aliases"][0] if item["aliases"] else None,
            f"sistema/{item['id']}",
        )
        for item in systems
    }
    SYSTEM_TAGS = {contract[1] for contract in SYSTEM_CONTRACTS.values()}

    areas: dict[str, str] = {}
    for directory in sorted((root / "60-Operacion").iterdir()):
        if not directory.is_dir():
            continue
        area_tags: set[str] = set()
        for note in directory.glob("*.md"):
            fields, _ = split_frontmatter(note.read_text(encoding="utf-8"))
            tags = fields.get("tags")
            tag_values = tags if isinstance(tags, list) else []
            area_tags.update(
                tag.removeprefix("operacion/area/")
                for tag in tag_values
                if isinstance(tag, str) and tag.startswith("operacion/area/")
            )
        if len(area_tags) == 1:
            areas[directory.name] = next(iter(area_tags))
    OPERATIONAL_AREAS = areas


configure_cell_contract(Path(__file__).resolve().parents[1])


def headings(body: str) -> set[str]:
    return set(re.findall(r"^##\s+(.+?)\s*$", body, re.MULTILINE))


def section_body(body: str, heading: str) -> str:
    match = re.search(
        rf"^##\s+{re.escape(heading)}\s*$\n(.*?)(?=^##\s+|\Z)",
        body,
        re.MULTILINE | re.DOTALL,
    )
    return match.group(1) if match else ""


def valid_date(value: Any) -> bool:
    if not isinstance(value, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value):
        return False
    try:
        date.fromisoformat(value)
    except ValueError:
        return False
    return True


def rel(path: Path, root: Path) -> str:
    return str(path.relative_to(root))


def check_required(
    path: Path,
    root: Path,
    fields: dict[str, Any],
    required: set[str],
    allowed: set[str],
) -> list[str]:
    issues = [f"{rel(path, root)}: missing frontmatter field {key}" for key in sorted(required - fields.keys())]
    issues.extend(
        f"{rel(path, root)}: unexpected frontmatter field {key}"
        for key in sorted(fields.keys() - allowed)
    )
    return issues


def check_sections(path: Path, root: Path, body: str, required: set[str]) -> list[str]:
    present = headings(body)
    return [f"{rel(path, root)}: missing section {name}" for name in sorted(required - present)]


def audit_repo(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(path, root, fields, REPO_REQUIRED, REPO_REQUIRED)
    issues.extend(check_sections(path, root, body, REPO_SECTIONS))
    prefix = rel(path, root)

    for key in sorted(REPO_LIST_FIELDS):
        if key in fields and not isinstance(fields[key], list):
            issues.append(f"{prefix}: {key} must be a list")
    if fields.get("sistema") not in SYSTEMS:
        issues.append(f"{prefix}: sistema must be one of {sorted(SYSTEMS)}")
    if fields.get("tipo") not in REPO_TYPES:
        issues.append(f"{prefix}: invalid repo tipo {fields.get('tipo')}")
    for key in COVERAGE_FIELDS:
        if fields.get(key) not in COVERAGE_VALUES:
            issues.append(f"{prefix}: invalid {key} value {fields.get(key)}")
    commit = fields.get("commit-analizado")
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{12}", commit):
        issues.append(f"{prefix}: commit-analizado is not a 12-char lowercase sha")
    if fields.get("rama-analizada") not in {"main", "master"}:
        issues.append(f"{prefix}: rama-analizada must be main or master")
    for key in ("fecha-analisis", "ultima-auditoria"):
        if not valid_date(fields.get(key)):
            issues.append(f"{prefix}: {key} must be a valid YYYY-MM-DD date")
    return issues


def audit_architecture(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    kind = fields.get("tipo")
    issues = check_required(path, root, fields, ARCH_COMMON, ARCH_ALLOWED)
    prefix = rel(path, root)
    if kind not in ARCH_SECTIONS:
        issues.append(f"{prefix}: invalid architecture tipo {kind}")
        return issues
    issues.extend(check_sections(path, root, body, ARCH_SECTIONS[kind]))
    if fields.get("sistema") not in SYSTEMS:
        issues.append(f"{prefix}: sistema must be one of {sorted(SYSTEMS)}")
    if not valid_date(fields.get("ultima-auditoria")):
        issues.append(f"{prefix}: ultima-auditoria must be a valid YYYY-MM-DD date")
    for key in ("participa-en", "tags"):
        if not isinstance(fields.get(key), list):
            issues.append(f"{prefix}: {key} must be a list")

    if kind == "servicio":
        composed = fields.get("compuesto-por")
        if not isinstance(composed, list) or len(composed) < 2:
            issues.append(f"{prefix}: servicio compuesto-por must contain at least two nodes")
        forbidden = (ARCH_COMPONENT | ARCH_RUNTIME_REQUIRED | ARCH_RUNTIME_OPTIONAL) & fields.keys()
    elif kind == "componente":
        implemented = fields.get("implementado-por")
        if not isinstance(implemented, list) or len(implemented) != 1:
            issues.append(f"{prefix}: componente implementado-por must contain exactly one repo")
        forbidden = (ARCH_SERVICE | ARCH_RUNTIME_REQUIRED | ARCH_RUNTIME_OPTIONAL) & fields.keys()
    else:
        issues.extend(
            f"{prefix}: missing runtime field {key}"
            for key in sorted(ARCH_RUNTIME_REQUIRED - fields.keys())
        )
        if "ultima-verificacion" in fields and not valid_date(fields["ultima-verificacion"]):
            issues.append(f"{prefix}: ultima-verificacion must be a valid YYYY-MM-DD date")
        for key in ("ejecuta", "gatilla-a", "usa-infra"):
            if key in fields and not isinstance(fields[key], list):
                issues.append(f"{prefix}: {key} must be a list")
        forbidden = (ARCH_SERVICE | ARCH_COMPONENT) & fields.keys()
    for key in sorted(forbidden):
        issues.append(f"{prefix}: {key} is not allowed for tipo {kind}")
    return issues


def audit_topic(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(path, root, fields, TOPIC_FIELDS, TOPIC_FIELDS)
    issues.extend(check_sections(path, root, body, TOPIC_SECTIONS))
    prefix = rel(path, root)
    if fields.get("tipo") != "topic":
        issues.append(f"{prefix}: tipo must be topic")
    forbidden = headings(body) & TOPIC_FORBIDDEN_HEADINGS
    for heading in sorted(forbidden):
        issues.append(f"{prefix}: forbidden manual relationship section {heading}")
    return issues


def audit_integration(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    required = {"tipo", "tags"}
    issues = check_required(path, root, fields, required, INTEGRATION_FIELDS)
    issues.extend(check_sections(path, root, body, INTEGRATION_SECTIONS))
    if fields.get("tipo") != "integracion-externa":
        issues.append(f"{rel(path, root)}: tipo must be integracion-externa")
    return issues


def audit_flow(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(path, root, fields, FLOW_FIELDS, FLOW_FIELDS)
    issues.extend(check_sections(path, root, body, FLOW_SECTIONS))
    prefix = rel(path, root)
    if fields.get("tipo") != "flujo":
        issues.append(f"{prefix}: tipo must be flujo")
    if len(re.findall(r"^```mermaid\s*$", body, re.MULTILINE)) != 2:
        issues.append(f"{prefix}: flow must contain exactly two Mermaid diagrams")
    component_diagram = section_body(body, "Diagrama de componentes")
    if "```mermaid" not in component_diagram or not re.search(r"^\s*subgraph\b", component_diagram, re.MULTILINE):
        issues.append(f"{prefix}: component diagram must be Mermaid and contain subgraph")
    return issues


def audit_system(path: Path, root: Path) -> list[str]:
    fields, _ = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(path, root, fields, SYSTEM_FIELDS, SYSTEM_FIELDS)
    prefix = rel(path, root)
    if fields.get("tipo") != "sistema":
        issues.append(f"{prefix}: tipo must be sistema")
    for key in ("aliases", "tags"):
        if not isinstance(fields.get(key), list):
            issues.append(f"{prefix}: {key} must be a list")

    contract = SYSTEM_CONTRACTS.get(path.stem)
    if contract is None:
        issues.append(f"{prefix}: unsupported system note {path.stem}")
        return issues
    app_code, system_tag = contract
    aliases = fields.get("aliases")
    tags = fields.get("tags")
    if app_code and isinstance(aliases, list) and app_code not in aliases:
        issues.append(f"{prefix}: aliases must contain {app_code}")
    if isinstance(tags, list):
        if system_tag not in tags:
            issues.append(f"{prefix}: tags must contain {system_tag}")
        if "moc" not in tags:
            issues.append(f"{prefix}: tags must contain moc")
    return issues


def audit_index(path: Path, root: Path) -> list[str]:
    fields, _ = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(path, root, fields, INDEX_FIELDS, INDEX_FIELDS)
    prefix = rel(path, root)
    if fields.get("tipo") != "indice":
        issues.append(f"{prefix}: tipo must be indice")
    tags = fields.get("tags")
    if not isinstance(tags, list):
        issues.append(f"{prefix}: tags must be a list")
    elif "moc" not in tags:
        issues.append(f"{prefix}: index tags must contain moc")
    if (
        path.relative_to(root).as_posix()
        == "70-Aprendizajes/Aprendizajes.md"
        and isinstance(tags, list)
        and "aprendizaje" not in tags
    ):
        issues.append(f"{prefix}: learning index tags must contain aprendizaje")
    return issues


def audit_glossary(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(path, root, fields, GLOSSARY_REQUIRED, GLOSSARY_ALLOWED)
    prefix = rel(path, root)
    if fields.get("tipo") != "glosario":
        issues.append(f"{prefix}: tipo must be glosario")
    tags = fields.get("tags")
    if not isinstance(tags, list):
        issues.append(f"{prefix}: tags must be a list")
    elif "glosario" not in tags:
        issues.append(f"{prefix}: tags must contain glosario")
    if "aliases" in fields and not isinstance(fields["aliases"], list):
        issues.append(f"{prefix}: aliases must be a list")
    if not re.search(r"\[\[[^\]]+\]\]", body):
        issues.append(f"{prefix}: glossary must contain at least one wikilink")
    return issues


def audit_operational(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(
        path,
        root,
        fields,
        OPERATIONAL_COMMON_REQUIRED,
        OPERATIONAL_ALLOWED,
    )
    prefix = rel(path, root)
    kind = fields.get("clase")
    if fields.get("tipo") != "operacional":
        issues.append(f"{prefix}: tipo must be operacional")
    if kind not in OPERATIONAL_SECTIONS:
        issues.append(f"{prefix}: invalid operational clase {kind}")
        return issues
    issues.extend(check_sections(path, root, body, OPERATIONAL_SECTIONS[kind]))
    if fields.get("estado") not in OPERATIONAL_STATES:
        issues.append(f"{prefix}: invalid operational estado {fields.get('estado')}")
    owner = fields.get("owner")
    if not isinstance(owner, str) or not owner.strip():
        issues.append(f"{prefix}: owner must be a non-empty string")
    elif fields.get("estado") == "vigente" and owner == "por-definir":
        issues.append(f"{prefix}: vigente operational note requires a real owner")
    if not valid_date(fields.get("ultima-verificacion")):
        issues.append(f"{prefix}: ultima-verificacion must be a valid YYYY-MM-DD date")
    for key in ("canales", "tags"):
        if not isinstance(fields.get(key), list):
            issues.append(f"{prefix}: {key} must be a list")
    tags = fields.get("tags")
    if isinstance(tags, list):
        if "operacion" not in tags:
            issues.append(f"{prefix}: tags must contain operacion")
        expected = f"operacion/{kind}"
        if expected not in tags:
            issues.append(f"{prefix}: tags must contain {expected}")
        relative = path.relative_to(root)
        area_name = relative.parts[1] if len(relative.parts) >= 3 else None
        area_slug = OPERATIONAL_AREAS.get(area_name or "")
        if area_slug:
            area_tag = f"operacion/area/{area_slug}"
            if area_tag not in tags:
                issues.append(f"{prefix}: tags must contain {area_tag}")
        allowed_tags = {"operacion", expected}
        if area_slug:
            allowed_tags.add(f"operacion/area/{area_slug}")
        allowed_tags.update(SYSTEM_TAGS)
        unexpected_tags = sorted(set(tags) - allowed_tags)
        if unexpected_tags:
            issues.append(f"{prefix}: unsupported operational tags {unexpected_tags}")
    relative = path.relative_to(root)
    if len(relative.parts) != 3 or relative.parts[0] != "60-Operacion":
        issues.append(f"{prefix}: operational note must live at 60-Operacion/<Area>/<note>.md")
    else:
        area_name = relative.parts[1]
        if area_name not in OPERATIONAL_AREAS:
            issues.append(f"{prefix}: unknown operational area {area_name}")
        expected_area = f"[[{area_name}]]"
        if fields.get("area") != expected_area:
            issues.append(f"{prefix}: area must be {expected_area}")
    related = fields.get("relacionado-con", [])
    if not isinstance(related, list):
        issues.append(f"{prefix}: relacionado-con must be a list")
    else:
        valid_area_links = {f"[[{name}]]" for name in OPERATIONAL_AREAS}
        invalid = sorted(set(related) - valid_area_links)
        if invalid:
            issues.append(f"{prefix}: relacionado-con must target operational area MOCs: {invalid}")
    report_id = fields.get("report-id")
    if kind == "reporte":
        if not isinstance(report_id, str) or not REPORT_ID.fullmatch(report_id):
            issues.append(f"{prefix}: reporte requires lowercase kebab-case report-id")
    elif "report-id" in fields:
        issues.append(f"{prefix}: report-id is only allowed for clase reporte")
    if kind == "procedimiento" and OPERATIONAL_STEP_HEADER not in section_body(
        body,
        "Pasos",
    ):
        issues.append(f"{prefix}: procedure steps must use the closed table header")
    return issues


def audit_operational_topology(root: Path, paths: list[Path]) -> list[str]:
    issues: list[str] = []
    operation_root = root / "60-Operacion"
    root_notes = sorted(path.name for path in operation_root.glob("*.md"))
    if root_notes != ["Operacion.md"]:
        issues.append(
            "60-Operacion: root must contain only Operacion.md; "
            f"found {root_notes}"
        )
    for area in OPERATIONAL_AREAS:
        directory = operation_root / area
        moc = directory / f"{area}.md"
        if not directory.is_dir():
            issues.append(f"60-Operacion/{area}: missing operational area directory")
        elif not moc.is_file():
            issues.append(f"60-Operacion/{area}: missing area MOC {area}.md")
    report_ids: dict[str, list[str]] = {}
    for path in paths:
        fields, _ = split_frontmatter(path.read_text(encoding="utf-8"))
        report_id = fields.get("report-id")
        if isinstance(report_id, str):
            report_ids.setdefault(report_id, []).append(rel(path, root))
    for report_id, owners in sorted(report_ids.items()):
        if len(owners) > 1:
            issues.append(f"60-Operacion: duplicate report-id {report_id}: {owners}")
    recipes_root = root / ".agents/skills/generate-reports/scripts/reports"
    if recipes_root.is_dir():
        recipe_paths = [
            path
            for path in recipes_root.glob("*/report.json")
            if not path.parent.name.startswith("sample-")
        ]
        recipe_ids = {path.parent.name for path in recipe_paths}
        contract_ids = set(report_ids)
        if recipe_ids != contract_ids:
            issues.append(
                "60-Operacion: report contracts and recipes differ: "
                f"contracts={sorted(contract_ids)} recipes={sorted(recipe_ids)}"
            )
        for path in sorted(recipe_paths):
            try:
                recipe = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError) as error:
                issues.append(f"{rel(path, root)}: invalid recipe JSON: {error}")
                continue
            if recipe.get("report_id") != path.parent.name:
                issues.append(f"{rel(path, root)}: report_id must match recipe directory")
            owners = report_ids.get(path.parent.name, [])
            if len(owners) == 1 and Path(owners[0]).stem != recipe.get("contract_basename"):
                issues.append(f"{rel(path, root)}: contract_basename does not match report note")
    return issues


def audit_learning(path: Path, root: Path) -> list[str]:
    fields, body = split_frontmatter(path.read_text(encoding="utf-8"))
    issues = check_required(
        path,
        root,
        fields,
        LEARNING_FIELDS,
        LEARNING_FIELDS,
    )
    issues.extend(check_sections(path, root, body, LEARNING_SECTIONS))
    prefix = rel(path, root)

    if fields.get("tipo") != "aprendizaje":
        issues.append(f"{prefix}: tipo must be aprendizaje")
    if fields.get("estado") not in LEARNING_STATES:
        issues.append(f"{prefix}: invalid learning estado {fields.get('estado')}")
    if fields.get("resultado") not in LEARNING_RESULTS:
        issues.append(
            f"{prefix}: invalid learning resultado {fields.get('resultado')}"
        )
    if not path.stem.startswith("Aprendizaje - "):
        issues.append(f"{prefix}: learning filename must start with Aprendizaje - ")

    for key in sorted(LEARNING_LIST_FIELDS):
        if not isinstance(fields.get(key), list):
            issues.append(f"{prefix}: {key} must be a list")

    for key in ("aplica-a", "dimensiones", "investigaciones-origen"):
        value = fields.get(key)
        if isinstance(value, list) and not value:
            issues.append(f"{prefix}: {key} must not be empty")

    for key in ("aplica-a", "supersede-a"):
        value = fields.get(key)
        if isinstance(value, list):
            for item in value:
                if not isinstance(item, str) or WIKILINK_VALUE.fullmatch(item) is None:
                    issues.append(f"{prefix}: {key} values must be canonical wikilinks")

    dimensions = fields.get("dimensiones")
    if isinstance(dimensions, list):
        for dimension in dimensions:
            if (
                not isinstance(dimension, str)
                or LEARNING_DIMENSION.fullmatch(dimension) is None
            ):
                issues.append(
                    f"{prefix}: dimensiones values must be lowercase kebab-case"
                )

    investigations = fields.get("investigaciones-origen")
    if isinstance(investigations, list):
        for investigation in investigations:
            if (
                not isinstance(investigation, str)
                or INVESTIGATION_ID.fullmatch(investigation) is None
            ):
                issues.append(
                    f"{prefix}: investigaciones-origen values must be investigation IDs"
                )

    for key in ("fecha-conclusion", "ultima-validacion"):
        if not valid_date(fields.get(key)):
            issues.append(f"{prefix}: {key} must be a valid YYYY-MM-DD date")
    if valid_date(fields.get("fecha-conclusion")) and valid_date(
        fields.get("ultima-validacion")
    ):
        if fields["ultima-validacion"] < fields["fecha-conclusion"]:
            issues.append(
                f"{prefix}: ultima-validacion must not precede fecha-conclusion"
            )

    tags = fields.get("tags")
    if isinstance(tags, list):
        if "aprendizaje" not in tags:
            issues.append(f"{prefix}: tags must contain aprendizaje")
        if isinstance(dimensions, list):
            for dimension in dimensions:
                if isinstance(dimension, str):
                    expected = f"aprendizaje/{dimension}"
                    if expected not in tags:
                        issues.append(f"{prefix}: tags must contain {expected}")

    evidence = section_body(body, "Evidencia acumulada")
    evidence_matches = list(
        re.finditer(r"^###\s+EV-(\d{3})\b.*$", evidence, re.MULTILINE)
    )
    evidence_origins: set[str] = set()
    if not evidence_matches:
        issues.append(
            f"{prefix}: Evidencia acumulada must contain at least one EV-### entry"
        )
    else:
        evidence_ids = [match.group(1) for match in evidence_matches]
        expected_ids = [
            f"{index:03d}" for index in range(1, len(evidence_matches) + 1)
        ]
        if evidence_ids != expected_ids:
            issues.append(
                f"{prefix}: evidence IDs must be unique and sequential from EV-001"
            )
        for index, match in enumerate(evidence_matches):
            end = (
                evidence_matches[index + 1].start()
                if index + 1 < len(evidence_matches)
                else len(evidence)
            )
            entry = evidence[match.end() : end]
            for field in LEARNING_EVIDENCE_FIELDS:
                if re.search(
                    rf"^-\s+{re.escape(field)}:\s+\S",
                    entry,
                    re.MULTILINE,
                ) is None:
                    issues.append(
                        f"{prefix}: EV-{match.group(1)} missing evidence field {field}"
                    )
            origin_match = re.search(
                r"^-\s+Investigación:\s+(.+?)\s*$",
                entry,
                re.MULTILINE,
            )
            if origin_match is not None:
                origin = origin_match.group(1).strip()
                if origin.startswith("`") and origin.endswith("`"):
                    origin = origin[1:-1]
                if INVESTIGATION_ID.fullmatch(origin) is None:
                    issues.append(
                        f"{prefix}: EV-{match.group(1)} Investigación must be "
                        "a valid investigation ID"
                    )
                elif (
                    isinstance(investigations, list)
                    and origin not in investigations
                ):
                    issues.append(
                        f"{prefix}: EV-{match.group(1)} Investigación must "
                        "appear in investigaciones-origen"
                    )
                else:
                    evidence_origins.add(origin)
            if LOCAL_IGNORED_REFERENCE.search(entry) is not None:
                issues.append(
                    f"{prefix}: EV-{match.group(1)} Fuentes durables must not "
                    "reference local ignored workspaces"
                )
    if isinstance(investigations, list):
        for investigation in investigations:
            if (
                isinstance(investigation, str)
                and INVESTIGATION_ID.fullmatch(investigation) is not None
                and investigation not in evidence_origins
            ):
                issues.append(
                    f"{prefix}: investigaciones-origen value {investigation} "
                    "must be referenced by at least one EV entry"
                )
    present_order = re.findall(r"^##\s+(.+?)\s*$", body, re.MULTILINE)
    if present_order != list(LEARNING_SECTION_ORDER):
        issues.append(
            f"{prefix}: learning sections must exactly match the closed order"
        )
    return issues


def audit_learning_relationships(paths: list[Path], root: Path) -> list[str]:
    issues: list[str] = []
    fields_by_name = {
        path.stem: split_frontmatter(path.read_text(encoding="utf-8"))[0]
        for path in paths
    }
    graph: dict[str, set[str]] = {name: set() for name in fields_by_name}
    inbound: dict[str, set[str]] = {name: set() for name in fields_by_name}

    for source, fields in fields_by_name.items():
        source_path = root / "70-Aprendizajes" / f"{source}.md"
        prefix = rel(source_path, root)
        targets = fields.get("supersede-a")
        if not isinstance(targets, list):
            continue
        for raw_target in targets:
            if (
                not isinstance(raw_target, str)
                or WIKILINK_VALUE.fullmatch(raw_target) is None
            ):
                continue
            target = raw_target[2:-2]
            if target == source:
                issues.append(
                    f"{prefix}: supersede-a must not reference the same learning"
                )
                continue
            target_fields = fields_by_name.get(target)
            if target_fields is None or target_fields.get("tipo") != "aprendizaje":
                issues.append(
                    f"{prefix}: supersede-a target must be a learning note"
                )
                continue
            graph[source].add(target)
            inbound[target].add(source)
            if target_fields.get("estado") != "superado":
                issues.append(
                    f"{prefix}: supersede-a target must have estado superado"
                )

    for name, fields in fields_by_name.items():
        if fields.get("estado") == "superado" and not inbound[name]:
            path = root / "70-Aprendizajes" / f"{name}.md"
            issues.append(
                f"{rel(path, root)}: superado learning must be referenced "
                "by a replacement"
            )

    visited: set[str] = set()
    active: set[str] = set()

    def visit(name: str) -> bool:
        if name in active:
            return True
        if name in visited:
            return False
        active.add(name)
        found_cycle = any(visit(target) for target in graph[name])
        active.remove(name)
        visited.add(name)
        return found_cycle

    if any(visit(name) for name in graph):
        issues.append(
            "70-Aprendizajes: learning supersession graph must not contain cycles"
        )
    return issues


def audit_forbidden_files(root: Path) -> list[str]:
    issues: list[str] = []
    for path in root.rglob("*"):
        relative_path = path.relative_to(root)
        if (
            path.is_dir()
            or any(part.startswith(".") for part in relative_path.parts)
        ):
            continue
        if path.name in FORBIDDEN_RELATION_FILES:
            issues.append(f"{rel(path, root)}: external relation ledger is not allowed")
    return issues


def audit_sync_process_leakage(path: Path, root: Path) -> list[str]:
    issues = []
    for line_number, line in enumerate(
        path.read_text(encoding="utf-8").splitlines(),
        start=1,
    ):
        if SYNC_PROCESS_LEAKAGE.search(line):
            issues.append(
                f"{rel(path, root)}:{line_number}: synchronization process "
                "state must not appear in a durable graph note"
            )
    return issues


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = args.root.resolve()
    configure_cell_contract(root)
    operational_area_indices = sorted(
        path
        for path in (root / "60-Operacion").glob("*/*.md")
        if path.parent.name in OPERATIONAL_AREAS and path.stem == path.parent.name
    )
    index_paths = [
        path
        for path in (
            root / "00-Home.md",
            root / "30-Flujos/Flujos.md",
            root / "60-Operacion/Operacion.md",
            root / "70-Aprendizajes/Aprendizajes.md",
        )
        if path.exists()
    ] + operational_area_indices
    learning_paths = sorted(
        path
        for path in (root / "70-Aprendizajes").glob("*.md")
        if path.name != "Aprendizajes.md"
    )
    operational_paths = sorted(
        path
        for path in (root / "60-Operacion").rglob("*.md")
        if path.name != "Operacion.md"
        and not (path.parent.name in OPERATIONAL_AREAS and path.stem == path.parent.name)
    )
    groups = [
        ("INDEX_NOTES", index_paths, audit_index),
        ("SYSTEM_NOTES", sorted((root / "10-Sistemas").glob("*.md")), audit_system),
        ("REPO_NOTES", sorted((root / "20-Repos").glob("*/*.md")), audit_repo),
        ("ARCHITECTURE_NOTES", sorted((root / "15-Arquitectura").glob("*.md")), audit_architecture),
        ("TOPIC_NOTES", sorted((root / "25-Topics").glob("*.md")), audit_topic),
        (
            "FLOW_NOTES",
            sorted(path for path in (root / "30-Flujos").glob("*.md") if path.name != "Flujos.md"),
            audit_flow,
        ),
        ("INTEGRATION_NOTES", sorted((root / "40-Integraciones").glob("*.md")), audit_integration),
        ("GLOSSARY_NOTES", sorted((root / "50-Glosario").glob("*.md")), audit_glossary),
        (
            "OPERATIONAL_NOTES",
            operational_paths,
            audit_operational,
        ),
        (
            "LEARNING_NOTES",
            learning_paths,
            audit_learning,
        ),
    ]
    issues = audit_forbidden_files(root)
    for _, paths, audit in groups:
        for path in paths:
            issues.extend(audit(path, root))
            if not path.is_relative_to(root / "60-Operacion"):
                issues.extend(audit_sync_process_leakage(path, root))
    issues.extend(audit_learning_relationships(learning_paths, root))
    issues.extend(audit_operational_topology(root, operational_paths))

    for label, paths, _ in groups:
        print(f"{label}: {len(paths)}")
    print(f"ISSUES: {len(issues)}")
    for issue in issues:
        print(issue)
    return 1 if issues else 0


if __name__ == "__main__":
    sys.exit(main())
