# Operational procedure contract

Use `60-Operacion/<Area>/` as the versioned source for operational knowledge. `Operacion.md` is the only Markdown at the domain root; each registered area has one homonymous MOC. The MOC contains a brief scope description, an optional high-level synthesis, and links to the area's specific notes. Policies, standards, guides, catalogs, procedures, and report contracts live in separate non-index notes; the MOC never substitutes for one of those documents.

Use this frontmatter for an area MOC:

```yaml
---
tipo: indice
tags: [moc, operacion, operacion/area/devops]
---
```

When creating or updating a specific operational note, add or update its MOC link without copying the note's rules or steps into the MOC. A misplaced contract in an existing MOC is a structural issue: propose an explicit move to a specific note and preserve the current bytes until that move is authorized.

The closed frontmatter for every non-index note is:

```yaml
---
tipo: operacional
clase: procedimiento # politica | guia | catalogo | estandar | procedimiento | reporte
estado: borrador      # borrador | vigente | retirado
owner: por-definir
ultima-verificacion: 2026-07-23
area: "[[DevOps]]"
relacionado-con: ["[[Delivery]]"] # optional
canales: [tracker, correo, mensajeria-equipo]
tags: [operacion, operacion/procedimiento, operacion/area/devops]
---
```

`owner` identifies the accountable role or team. `por-definir` is valid only while `estado: borrador`. `canales` names neutral capabilities; connector selection happens during execution.

Resolve registered areas through `<VAULTCTL> config areas --vault "<VAULT_ROOT>"` and choose one owning area by the primary outcome and completion evidence. For a cross-system audit, prefer a registered audit area over an execution technology. The directory, `area`, and area tag must agree. Use `relacionado-con` for secondary area MOCs and keep one canonical note.

## Classes

### Policy

Use `clase: politica` for durable governance rules that define required, permitted, or prohibited behavior across one or more workflows. Require: Objetivo, Alcance, Reglas, Excepciones, Escalamiento, and Mantenimiento.

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

Use `clase: reporte` for one implemented report contract. Add one globally unique lowercase kebab-case `report-id` and require: Propósito, Audiencia y decisiones, Definiciones y grano, Fuente y alcance, Período, filtros y exclusiones, Salida, Validación, Mantenimiento, and Limitaciones. Link the execution procedure or implementation from the source or maintenance section.

The note and its linked procedure must establish input identity and required parameters, bounded scope and exclusions, the actual executor, output format and destination, and observable acceptance criteria. Specify access requirements, resource limits, reproducibility evidence and retention when applicable to that executor and data. A period may be inapplicable; state that explicitly rather than inventing a date range. No query language, renderer, file format, cost-preview mechanism or calendar grain is required by the shared contract.

Generation requires all applicable execution inputs and checks. Validation of existing artifacts requires only the inputs and checks needed to evaluate those artifacts; listing requires neither execution nor live access. A missing required element blocks only the dependent mode with an exact gap. Report identity and duplicate detection belong to the catalog; execution and output checks belong to the destination procedure. Generated source data and result artifacts stay outside the knowledge graph and follow the procedure's retention policy.

## Lifecycle

- Keep unverified organization-specific content in `borrador`.
- Promote to `vigente` only with a real owner, verified destinations, and a current verification date.
- Mark obsolete guidance `retirado`; link its replacement when one exists.
- Update `ultima-verificacion` only after checking the relevant human or live-system authority.
- Use verified project keys, dashboards, recipients, channels, permissions, and approval chains. Keep unknown organization-specific values explicit in a `borrador`.

## Relationship to execution

Procedure notes define expected behavior. `.operations/` records one run. Connected systems remain authoritative for current fields, workflows, permissions, and artifact state.

## Execution eligibility

Apply this contract when selecting or resuming any operational procedure, including one used by an inspection capability or a report executor. Catalog resolution identifies a note; it does not establish that the note is eligible for execution.

| State | Permitted use |
|---|---|
| `vigente` | Execute the requested steps after checking their current prerequisites, exact targets and existing authorization. The label alone does not prove current access or correctness. |
| `borrador` | Read, assess or prepare drafts. Before executing a step, verify its required inputs, target, instructions and completion criteria against the relevant authoritative source. Record the supporting evidence in the caller's existing context or run. Missing verification blocks that step; verified steps may proceed within existing authorization without automatically promoting the note. |
| `retirado` | Read as historical context. Resolve and assess its replacement before execution; if none is available, report the missing current procedure. Do not turn the retired procedure into an ad-hoc execution plan to bypass retirement. |

A missing or unknown state permits inspection only until its validity is resolved. For a procedure selected before an interruption, recheck its state and any material changes before pending steps. Preserve evidence of already completed steps. An authorized ad-hoc plan where no runbook exists follows the same step-verification requirements as a draft; it does not require creating or promoting a note. Eligibility never supplies external-write authorization or requires an additional approval when the existing authorization already covers the verified steps.
