# Operational procedure contract

Use `60-Operacion/<Area>/` as the versioned source for operational knowledge. `Operacion.md` is the only Markdown at the domain root; each registered area has one homonymous MOC. The closed frontmatter for every non-index note is:

```yaml
---
tipo: operacional
clase: procedimiento # guia | catalogo | estandar | procedimiento | reporte
estado: borrador      # borrador | vigente | retirado
owner: por-definir
ultima-verificacion: 2026-07-23
area: "[[DevOps]]"
relacionado-con: ["[[Jira]]"] # optional
canales: [jira, correo, mensajeria-equipo]
tags: [operacion, operacion/procedimiento, operacion/area/devops]
---
```

`owner` identifies the accountable role or team. `por-definir` is valid only while `estado: borrador`. `canales` names neutral capabilities; connector selection happens during execution.

Resolve registered areas through `90-Meta/operational-catalog.py list-areas` and choose one owning area by the primary outcome and completion evidence. For a cross-system audit, prefer a registered audit area over an execution technology. The directory, `area`, and area tag must agree. Use `relacionado-con` for secondary area MOCs and keep one canonical note.

## Classes

### Guide

Use `clase: guia` for access, navigation, or troubleshooting knowledge. Require: Objetivo, Prerrequisitos, Procedimiento, Validación, and Problemas y escalamiento.

### Catalog

Use `clase: catalogo` for projects, dashboards, channels, owners, or other selectable operational destinations. Require: Propósito, Catálogo, Criterios de uso, Mantenimiento, and Limitaciones.

### Standard

Use `clase: estandar` for normative classification, naming, structure, or quality rules. Require: Objetivo, Alcance, Reglas, Plantillas, Validación, and Excepciones.

### Procedure

Use `clase: procedimiento` for an ordered workflow that produces an operational outcome. Require: Objetivo, Disparador, Entradas requeridas, Decisiones, Pasos, Evidencia de finalización, Fallos y recuperación, and Limitaciones.

The `Pasos` section must contain this table:

```markdown
| ID | Acción | Capacidad | Efecto externo | Entrada | Salida | Continuar si |
|---|---|---|---|---|---|---|
```

Use stable IDs such as `DEVOPS-01`. Write `sí` only when the step creates, modifies, sends, publishes, deletes, or otherwise changes an external system. Drafting content in memory is not an external effect; saving a draft in an external system is.

For an audit procedure, use the same sections and step table with read-only remote steps. Specify entity/date inputs, timezone, exact target resolution, source adapters, bounded queries or versioned scripts, correlation keys, outcome definitions, and completion evidence for each question. Define permitted terminal unknowns, such as expired retention, separately from recoverable access blockers. Distinguish current-state evidence from historical evidence and provider acceptance from verified correction. Declare local output and retention rules; domain-specific values belong here rather than in the generic orchestration skill.

### Report

Use `clase: reporte` for one implemented report contract. Add one globally unique lowercase kebab-case `report-id` and require: Propósito, Audiencia y decisiones, Definiciones y grano, Fuente y alcance, Período, filtros y exclusiones, Salida, Validación, Mantenimiento, and Limitaciones. Keep execution mechanics in the shared report skill and its matching recipe.

## Lifecycle

- Keep unverified organization-specific content in `borrador`.
- Promote to `vigente` only with a real owner, verified destinations, and a current verification date.
- Mark obsolete guidance `retirado`; link its replacement when one exists.
- Update `ultima-verificacion` only after checking the relevant human or live-system authority.
- Use verified project keys, dashboards, recipients, channels, permissions, and approval chains. Keep unknown organization-specific values explicit in a `borrador`.

## Relationship to execution

Procedure notes define expected behavior. `.operations/` records one run. Connected systems remain authoritative for current fields, workflows, permissions, and artifact state.
