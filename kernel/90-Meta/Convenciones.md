---
tags: [meta]
---

# Vault conventions

Schema for a cell knowledge vault. Cell identity, systems, and source prefixes live in `instance.yaml`, not here. Notes follow `locale.notes` from that file.

## Structure

| Folder | Content |
|---|---|
| `00-Home.md` | Curated entry; links MOCs, indexes, and primary views |
| `*.base` at root | Derived radar views (including missing `commit-analizado` and cobertura `por-confirmar`); not a source of truth |
| `10-Sistemas/` | One MOC per system declared in `instance.yaml` |
| `15-Arquitectura/` | Logical services, exceptional components, runtime resources |
| `20-Repos/<system-id>/` | One note per source repository, filed under the system it implements |
| `25-Topics/` | One note per asynchronous contract when `topic` is enabled |
| `30-Flujos/` | End-to-end business flows with Mermaid |
| `40-Integraciones/` | External systems with a stable business role |
| `50-Glosario/` | Domain terms used in two or more durable nodes |
| `60-Operacion/` | Team procedures, standards, catalogs, guides |
| `70-Aprendizajes/` | Evidence-bounded engineering learnings |
| `90-Meta/` | Schema, evidence framework, validators |

`.agents/` holds skills and stays outside the Obsidian graph. Local ignored stores (`.investigations/`, `.operations/`, `.knowledge-os-handoffs/`, `.knowledge-os-config.yaml`, `.plan/`) are not graph sources. Local plans must live in `.plan/`; never create a visible `plan/` directory in a cell vault.

## Node selection

Choose the type from the observed identity. Search basename, `aliases`, `nombre-raw`, and backlinks before creating a node.

| Destination | Create when | Do not use for |
|---|---|---|
| `00-Home.md` or `tipo: indice` | Curated navigation | Domain contracts |
| `10-Sistemas/` `tipo: sistema` | A system listed in `instance.yaml` | A capability or brand treated as a new system |
| `15-Arquitectura/` `tipo: servicio` | A stable capability composed of two or more repos/components | A 1:1 repo duplicate or an end-to-end sequence |
| `15-Arquitectura/` `tipo: componente` | One repo with several deployables that have their own identity | An extra layer on the normal 1:1 repo case |
| `15-Arquitectura/` `tipo: recurso-runtime` | Productive scheduler/function/runtime without an identifiable repo | Names seen only in templates |
| `20-Repos/` `tipo: repositorio` | An in-scope source repository | This vault container, generic libraries, setup docs |
| `25-Topics/` `tipo: topic` | An async contract with topology relevance (`topic` enabled) | A subscription, a test name, or HTTP |
| `30-Flujos/` `tipo: flujo` | An end-to-end business outcome crossing two durable nodes or a system boundary | Internal methods or a service composition copy |
| `40-Integraciones/` `tipo: integracion-externa` | An external system with a stable business role | Internal repos or isolated endpoints |
| `50-Glosario/` `tipo: glosario` | A term used in two or more durable nodes, or needed to disambiguate a contract | Generic technical terms |
| `60-Operacion/` `tipo: operacional` | A stable human/team procedure, catalog, or report contract | Business flows or a single run |
| `70-Aprendizajes/` `tipo: aprendizaje` | A reusable, evidence-bounded conclusion from `manage-investigation-derived-learning` | Session logs or undeployed designs |
| `90-Meta/` or a root Base | Schema, gates, helpers, derived views | Ecosystem facts |

Decision rules: update before duplicating; a service is not a flow; a business flow is not an operational procedure; a learning is not a technical production claim; runtime is not an integration; low-level tables/buckets stay as strings; ambiguity does not create a node; do not invent folder types locally.

Skip types listed as disabled in `instance.yaml` `graph.enabled_types`.

## Lifecycle

- Technical create/update requires the evidence profile in `instance.yaml`.
- Learning create/update requires the durable-learning gate.
- Propagate: every affected node gets an explicit decision, including no-change.
- Rename: keep the old name as an alias; retarget wikilinks to the canonical basename.
- Retire only with evidence of archive, transfer, replacement, or loss of force.

## Names

- Systems: short canonical name; codes and expansions in `aliases`.
- Repos: repository name without a product prefix when `sources.repo_prefixes` lists that prefix; full name in `aliases`.
- Services: `<System> - <capability>`.
- Components: `<repo> - <deployable>`.
- Runtime: `<System> - <platform> - <raw-name>`.
- Topics: logical name without environment suffix.
- Flows: `Flujo - <outcome>` or `Flow - <outcome>` per locale.
- Integrations: proper name of the external system.
- Glossary: shortest canonical term.
- Operational: `<Area> - <subject>`.
- Learning: `Aprendizaje - <problem>` or `Learning - <problem>` per locale.

The graph navigates repo → topic → repo when topics are enabled. Never document producer→consumer as a direct repo link.

## Repository note frontmatter

Closed contract. Do not add properties without updating this file, affected Bases, and `90-Meta/audit-vault.py`.

```yaml
---
aliases: []
sistema: "[[System]]"
tipo: api
lenguaje: go
gatillado-por: []
publica-en: []
consume-de: []
lee-de: []
escribe-en: []
usa-infra: []
participa-en: []
cobertura-entradas: por-confirmar
cobertura-salidas: por-confirmar
cobertura-datos: por-confirmar
cobertura-infra: por-confirmar
cobertura-flujos: por-confirmar
ultima-auditoria: 1970-01-01
commit-analizado: "000000000000"
fecha-analisis: 1970-01-01
rama-analizada: main
tags: [tipo/api]
---
```

`tipo` for repos: `api | bff | adapter | http-adapter | suscriptor | publicador | job | function | frontend | libreria | scaffold`.

Relational properties that point at durable notes use quoted wikilinks.

## Repository note sections

1. Propósito / Purpose
2. Gatillo / Trigger
3. Contratos
4. Persistencia y datos
5. Infraestructura y scheduling
6. Relación con flujos
7. Limitaciones

## Relationship grammar

Persist one durable direction. Inverse lists are backlinks.

- Pub/Sub: producer `publica-en` → topic; consumer `gatillado-por` the topic.
- HTTP: caller `consume-de` callee or integration.
- Data: `lee-de` / `escribe-en` as strings unless a runtime or integration node qualifies.
- Runtime: `usa-infra`.
- Service: `compuesto-por`.
- Exceptional component: `implementado-por`.
- Flow participation: `participa-en`.
