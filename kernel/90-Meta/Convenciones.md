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
| `60-Operacion/` | Team policies, procedures, standards, catalogs, guides |
| `70-Aprendizajes/` | Evidence-bounded engineering learnings |
| `90-Meta/` | Schema, evidence framework, validators |

`.agents/` holds skills and stays outside the Obsidian graph. Versioned `investigations/` is searchable in Obsidian but excluded from the kernel's technical graph traversal and audit gates; the graph helper can still look up investigations explicitly. It is collaborative provenance, not canonical technical truth. Local ignored stores (`.investigations/`, `.investigations-private/`, `.operations/`, `.knowledge-os-handoffs/`, `.knowledge-os-config.yaml`, `.plan/`) are not graph sources. Local plans must live in `.plan/`; never create a visible `plan/` directory in a cell vault.

For native kernel checks and the separate distribution development gates,
follow [[code-quality]]. Installed vault checks use the CLI selected through
`use-vault-cli`.

## Node selection

Choose the type from the observed identity. Search basename, `aliases`, `nombre-raw`, and backlinks before creating a node.

| Destination | Create when | Do not use for |
|---|---|---|
| `00-Home.md` or `tipo: indice` | Brief scope, high-level synthesis, and curated links | Policies, standards, guides, procedures, or other domain contracts |
| `10-Sistemas/` `tipo: sistema` | A system listed in `instance.yaml` | A capability or brand treated as a new system |
| `15-Arquitectura/` `tipo: servicio` | A stable capability composed of two or more repos/components | A 1:1 repo duplicate or an end-to-end sequence |
| `15-Arquitectura/` `tipo: componente` | One repo with several deployables that have their own identity | An extra layer on the normal 1:1 repo case |
| `15-Arquitectura/` `tipo: recurso-runtime` | Productive scheduler/function/runtime without an identifiable repo | Names seen only in templates |
| `20-Repos/` functional `tipo` from the repository contract below | An in-scope source repository, including a library or scaffold with an observed role | This vault container or an empty placeholder without an observed contract |
| `25-Topics/` `tipo: topic` | An async contract with topology relevance (`topic` enabled) | A subscription, a test name, or HTTP |
| `25-Topics/` `tipo: evento` | An event type (message attribute such as `eventType`) carried by a topic and selected by subscription filters | A topic, a subscription, or an internal enum never published |
| `30-Flujos/` `tipo: flujo` | An end-to-end business outcome crossing two durable nodes or a system boundary | Internal methods or a service composition copy |
| `40-Integraciones/` `tipo: integracion-externa` | An external system with a stable business role | Internal repos or isolated endpoints |
| `50-Glosario/` `tipo: glosario` | A term used in two or more durable nodes, or needed to disambiguate a contract | Generic technical terms |
| `60-Operacion/` `tipo: operacional` | A stable human/team policy, standard, guide, procedure, catalog, or report contract | Business flows, a single run, or area navigation |
| `70-Aprendizajes/` `tipo: aprendizaje` | A reusable, evidence-bounded conclusion from `manage-investigation-derived-learning` | Session logs or undeployed designs |
| `90-Meta/` or a root Base | Schema, gates, helpers, derived views | Ecosystem facts |

Operational areas are discovered from `60-Operacion/<Area>/<Area>.md` and consistent `operacion/area/<slug>` tags. The homonymous note is the area index: it briefly describes scope and links the area's specific operational notes; it is not a policy, standard, guide, or procedure. Classify an operational note by its primary outcome; use `relacionado-con` for other registered areas. A cross-system audit belongs in the registered audit area; access guides and corrective procedures retain their own purpose-based area. The closed operational contract is maintained in `manage-operational-workflow` under `references/procedure-contract.md`.

Decision rules: update before duplicating; a service is not a flow; a business flow is not an operational procedure; a learning is not a technical production claim; runtime is not an integration; low-level tables/buckets stay as strings; ambiguity does not create a node; do not invent folder types locally.

Skip types listed as disabled in `instance.yaml` `graph.enabled_types`. Local ignored stores (unpublished cases, plans, scratch code, discovery state) are listed in [[local-stores]].

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

Closed contract. Do not add properties without updating this file, affected Bases, and the native `vaultctl audit` validator.

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

`tipo` for repos: `api | bff | adapter | http-adapter | suscriptor | publicador | job | function | frontend | libreria | scaffold | infraestructura`.

Use `infraestructura` for repositories whose owned behavior is provisioning or configuring infrastructure. `scaffold` is for reusable project starters; it is not a fallback type for infrastructure code.

Relational properties that point at durable notes use quoted wikilinks.

## Required note sections

Use the exact headings below. The current validator requires these Spanish headings even when `locale.notes` is `en`; prose follows the cell locale. Additional sections are allowed unless explicitly forbidden below. Required fields and allowed fields form closed frontmatter contracts.

### Repository

- Propósito
- Gatillo
- Qué hace
- Entradas y salidas
- Persistencia y datos
- Infraestructura y scheduling
- Relaciones
- Limitaciones y desconocimientos

Use the repository frontmatter above. `aliases`, relationship properties, and `tags` must be lists. Coverage values are `completo | parcial | no-aplica | por-confirmar`. `sistema` must link a declared system; `commit-analizado` is the observed 12-character lowercase SHA; `rama-analizada` is the observed Git branch name; new analyses select the reference through [[reference-branches]], while historical metadata remains readable. Both dates must be valid `YYYY-MM-DD` values. Replace the example SHA and dates with observed values.

Under `Limitaciones y desconocimientos`, use the optional subsection `### Verificaciones pendientes` when an accepted map has partial or unresolved `connection.*` claims. Each unchecked item uses this compact form:

```markdown
- [ ] `connection.<stable-key>` — <source, platform or database to inspect>: <exact unresolved question>. Close with: <required evidence>.
```

Apply `.agents/skills/map-ecosystem/references/evidence-sufficiency.md` before creating or retaining these items: record only material questions for the requested scope, and explicitly retire obsolete demands without claiming functional success. Unverified delivery, traffic or business rows are limitations unless the task requires that verification. Record a concrete check, not a generic request to investigate. Name the environment or resource/configuration key when known. Add `#por-confirmar` only when the missing evidence affects business behavior or the runtime/production baseline. When evidence closes the question, update the relevant flow, data, infrastructure or relationship section and remove the pending item; Git retains its history. These items track knowledge verification, not implementation work or delivery ownership.

Keep the connection identity beside its description in the owning repository note, including after a pending item closes:

```markdown
<!-- connection:connection.orders.publish -->
```

Use one anchor per connection per owning note. The key identifies the local flow/connector slot, so a destination change preserves it. Other notes link to the owning note instead of owning a second copy. Initial adoption in an older note adds anchors only for the inspected scope; it does not require remapping the vault. Final-note review accounts for every old/new anchor as create, preserve, update or retire. Retirement requires evidence of the removed connection; closing a verification alone preserves the anchor. The anchor stores identity, while prose and the pending item store knowledge; there is no parallel relationship ledger.

### Deployment matrix

For an explicitly requested deployment audit, place the per-environment matrix under `Infraestructura y scheduling`. Ordinary repository maps summarize only runtime, schedules and configuration needed to understand main-flow connectors; a full matrix is not their completion gate. Preserve valid existing matrices. For the deployment audit, record one row per observed environment and deployable, tracing:

| Environment / deployable | Event or manual input | Workflow / job / condition | Build artifact | Deploy action or command | Project | Platform / resource | Region or zone | Namespace / workload | Manifest, overlay, or values source |
|---|---|---|---|---|---|---|---|---|---|

Attach source evidence or the exact unresolved indirection to the relevant row. A versioned target establishes deployment intent at the analyzed commit; current runtime existence requires reconciled control-plane evidence. After following available versioned indirections, use `no observado en fuentes estáticas revisadas` for missing values and `#por-confirmar` only when the gap affects runtime or production-baseline understanding. Keep contradictions visible and current health/logs outside this stable topology matrix. See `.agents/skills/map-ecosystem/references/deployment-evidence.md` for the evidence sequence.

### Topic

All four fields are required; no other fields are allowed:

```yaml
---
tipo: topic
nombre-raw: "<observed name>"
sistema: "[[System]]"
tags: []
---
```

Required headings:

- Qué representa
- Contrato
- Infraestructura verificada
- Limitaciones y desconocimientos

The headings `Productores`, `Consumidores`, and `Productores y consumidores` are forbidden. Derive inverse relationships through backlinks.

### Event

An event is one message type on a shared topic, observed in subscription filters (platform or IaC) or in the publisher's code. All five fields are required; no other fields are allowed:

```yaml
---
tipo: evento
nombre-raw: "<attribute value, e.g. orderConfirmed>"
sistema: "[[System]]"
topico: "[[<carrier topic>]]"
tags: []
---
```

Required headings are the topic headings. `Contrato` names the attribute (`eventType` or another key) and value; `Infraestructura verificada` cites the subscription filters that select it.

### External integration

`tipo` and `tags` are required; `aliases` is optional. No other fields are allowed:

```yaml
---
tipo: integracion-externa
aliases: []
tags: []
---
```

Required headings:

- Qué es
- Cómo se usa
- Contratos relevantes
- Infraestructura o ownership
- Limitaciones y desconocimientos

### Flow

All three fields are required; no other fields are allowed:

```yaml
---
tipo: flujo
sistema: "[[System]]"
tags: []
---
```

Required headings:

- Qué resuelve
- Disparador
- Paso a paso
- Diagrama de flujo
- Diagrama de componentes
- Participantes
- Pendientes

Include exactly two Mermaid blocks. The block under `Diagrama de componentes` must contain `subgraph`. Populate both diagrams from observed participants and relationships.

## Relationship grammar

Persist one durable direction. Inverse lists are backlinks.

- Asynchronous messaging: producer `publica-en` → topic; consumer `gatillado-por` the topic. When a shared topic carries several event types and the consumer's subscription filters one, the consumer is `gatillado-por` the event note and the publisher that sets that attribute `publica-en` the event note; the event note links its carrier topic.
- HTTP: caller `consume-de` callee or integration.
- Data: `lee-de` / `escribe-en` as strings unless a runtime or integration node qualifies.
- Runtime: `usa-infra`.
- Service: `compuesto-por`.
- Exceptional component: `implementado-por`.
- Flow participation: `participa-en`.

## Agent execution

The shared evidence, initiative and interaction contract lives in [AGENTS.md](../AGENTS.md#evidence-and-completion). Personal preferences belong in the root `AGENTS.personal.md`.

For project evidence roles, consult [[specialists]]. For delegation and executor selection, consult [[execution-profiles]]. Personal execution preferences take precedence over its defaults.

For evidence-backed conversational and retained answers, apply [evidence and completion](../AGENTS.md#evidence-and-completion) before delivery. It defines review, reuse and clear-language criteria; publication gates still apply.
