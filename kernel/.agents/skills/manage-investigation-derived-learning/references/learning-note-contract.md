# Learning-note contract

Load this reference before planning or writing a note under `70-Aprendizajes/`.

## Identity and filename

Name a note `Aprendizaje - <problema o decisión>.md`. The basename describes the stable question and material context, never the investigation ID, Jira key, implementation branch, person, or date.

Search existing basenames, `aplica-a`, dimensions, body text, wikilinks, and backlinks before choosing a target. Prefer one cumulative note for equivalent contexts.

## Closed frontmatter

```yaml
---
tipo: aprendizaje
estado: vigente
resultado: cambio-adoptado
aplica-a: ["[[nodo-canónico]]"]
dimensiones: [rendimiento]
investigaciones-origen: [20260729-174159-rendimiento-reporteria-tags]
fecha-conclusion: 2026-07-29
ultima-validacion: 2026-07-30
supersede-a: []
tags: [aprendizaje, aprendizaje/rendimiento]
---
```

Required fields are exactly those shown.

- `estado`: `vigente`, `cuestionado`, or `superado`; a `superado` note must be referenced by at least one replacement.
- `resultado`: `cambio-adoptado`, `baseline-conservado`, `alternativa-descartada`, or `hallazgo-metodologico`.
- `aplica-a`: one or more canonical wikilinks to the technical, business, operational, or learning nodes whose context the teaching helps evaluate.
- `dimensiones`: one or more lowercase kebab-case dimensions such as `rendimiento`, `confiabilidad`, `datos`, `testing`, or `operabilidad`.
- `investigaciones-origen`: immutable local investigation IDs that contributed evidence. Every listed ID appears in at least one `EV-###` entry; they are provenance strings, not wikilinks or evidence by themselves.
- `fecha-conclusion`: date when the current conclusion first passed the durable-learning gate.
- `ultima-validacion`: most recent date on which qualifying evidence revalidated or changed the conclusion.
- `supersede-a`: canonical learning-note wikilinks replaced by this note; normally empty because accumulation is preferred. Targets must have `estado: superado`; self-links, non-learning targets, and cycles are invalid.
- `tags`: include `aprendizaje` and `aprendizaje/<dimension>` for every listed dimension.

## Required sections

Keep these sections in this order:

1. Resumen
2. Pregunta y contexto
3. Baseline y alternativas
4. Método
5. Evidencia acumulada
6. Decisión y justificación
7. Enseñanza reutilizable
8. Cuándo aplica
9. Cuándo no aplica
10. Implementación y despliegue
11. Limitaciones y revalidación
12. Trazabilidad

`Resumen` states the current teaching without removing its boundary. `Enseñanza reutilizable` tells a future agent what to evaluate, not what to copy blindly.

## Cumulative evidence

Under **Evidencia acumulada**, assign immutable IDs `EV-001`, `EV-002`, and so on. Each entry records:

- contributing investigation ID;
- directly inspected durable sources;
- material context and exclusions;
- method, sample/window, controls, and metrics or observable criteria;
- result, including neutral or negative observations;
- effect on the current conclusion.

Append the next ID. Never renumber, recycle, delete, or rewrite an older entry to match a newer conclusion. Correct an error with a new entry that identifies what it corrects.

The note summarizes evidence and links its durable locations; it does not copy raw logs, production rows, sensitive attachments, or secret values. `Fuentes durables` never points into ignored local workspaces such as `.investigations/`, `.operations/`, `.knowledge-os-handoffs/`, or `plan/`, nor names one of their local manifests, runners, hashes, results, run records, handoffs, or attachments as proof. A reproducible method qualifies only when the procedure, inputs, and criteria are themselves preserved durably or can be rerun from versioned tooling. If a local-only artifact is the sole support, the assessment is `insufficient-evidence`.

## Lifecycle

- `create`: start with `EV-001`.
- `enrich`: append evidence and change the teaching or boundaries only where supported.
- `challenge`: append the conflicting evidence, preserve both positions, set `estado: cuestionado`, and state the exact revalidation needed.
- `supersede`: create or select the replacement, set the old note to `superado`, add the old canonical wikilink to the replacement's `supersede-a`, and preserve both histories.
- Revalidation with fresh qualifying evidence is `enrich` even when the teaching remains stable; append that evidence and update `ultima-validacion`. A check that adds no new source, context coverage, confidence, or revalidation value is `already-covered` and makes no write.

For `cambio-adoptado`, **Implementación y despliegue** must identify exact implementation and productive applicability that independently passed the production-evidence gate. For `baseline-conservado`, `alternativa-descartada`, or `hallazgo-metodologico`, state why a new deployment is not applicable and trace the authoritative baseline and comparison evidence.

## Completion criterion

A note conforms when its stable identity and applicability are clear, the current teaching follows from cumulative evidence, negative and neutral results remain visible, another authorized agent can revisit the sources and method, implementation/deployment claims meet their separate gate, prior notes are deduplicated or linked through supersession, and the closed schema plus vault gates pass.
