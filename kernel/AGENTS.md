# Knowledge OS — router

This vault is an evidence-backed map of one cell's systems. Load skills by name; do not restate them here.

`tracker` means one external work-tracking instance declared in `instance.yaml`. A `work item` is one externally tracked unit identified by its exact tracker and provider-native reference.

Classify the request, then select one primary skill. Use `map-ecosystem` for knowledge and dependency questions; use `manage-operational-workflow` to audit a known flow or entity for a period or to follow an operational procedure. Operational audit procedures belong to the registered audit area; use related-area links for the technologies they inspect. Operational audits keep continuity in `.operations/`; use `manage-investigation` when the user requests a case or the work needs open-ended hypothesis tracking beyond a procedure. For orientation or interrogation, first hop is `90-Meta/graph-query.py` (JSON stems and edges, no note bodies). Open `instance.yaml` and `00-Home.md` for cell identity. Open `90-Meta/Alcance.md` only for cell-specific scope, allowlists, or exceptions. Open a section of `90-Meta/Convenciones.md` or `90-Meta/Auditoria - Framework.md` only when schema, node type, or an evidence gate is required. Do not load those Meta files before classifying.

## Skills

- `map-ecosystem` — query, document, synchronize, confirm tooling readiness, or answer where a fact belongs. First step is **orientation** when bootstrap is incomplete.
- `configure-workspace` — sole writer of local `.knowledge-os-config.yaml`. Onboarding is demand-triggered when a skill cannot resolve repositories or the user asks whether the workspace is initialized or configured.
- `manage-investigation` — versionable cases under `investigations/` with optional private overlays.
- `manage-investigation-derived-learning` — assess and publish `70-Aprendizajes/` notes.
- `manage-development-handoff` — persistent work-item worktrees and handoff files.
- `reconcile-development-handoff` — pull one selected handoff's implementation evidence into its source case while preserving its worktree-local lifecycle state.
- `manage-git-workflow` — analyze or maintain the cell's Git/GitHub policy, or apply it to source repositories; routine local versioning of this vault follows repository instructions.
- `manage-operational-workflow` — audit flow executions or an entity by date through read-only evidence; draft or execute operational procedures; resume runs in `.operations/`. Uses inspection adapters for access and queries; business actions and publication require matching authorization.
- `grilling` — one decision at a time.
- `obsidian-cli`, `obsidian-markdown`, `obsidian-bases` — user-invoked Obsidian helpers.

For mapping closure or pending-work status, follow the skill's [mapping completion contract](.agents/skills/map-ecosystem/references/mapping-completion.md).

Adapters (only if listed in `instance.yaml` `adapters`):

- `inspect-gcp-runtime` with `gcloud` leaf syntax.
- `inspect-database` for a database target and its available evidence sources; schema repositories are optional.
- `generate-reports` for registered operational report recipes.

## Layout

`00-Home.md` · `10-Sistemas/` · `15-Arquitectura/` · `20-Repos/` · `25-Topics/` · `30-Flujos/` · `40-Integraciones/` · `50-Glosario/` · `60-Operacion/` · `70-Aprendizajes/` · `90-Meta/`

Instance files (`instance.yaml`, `00-Home.md`, notes under `10/`–`70/`) are never updated by the kernel installer.

## Guardrails

- Apply `90-Meta/evidence-policy.md` when assessing a technical write. Cell identity and capability bindings come from onboarding, never inferred company conventions.
- Evidence before inference. Separate facts, inferences, and limitations.
- Never persist secrets.
- Canonical wikilinks target the note basename.
- Persist one durable relationship direction; use backlinks for the inverse.
- Messaging goes producer → topic → consumer when `topic` is enabled.
- Business flows live in `30-Flujos/`; human procedures live in `60-Operacion/`.
- Author skills and this file with `writing-for-agents`. Keep harness-portable: no provider-specific metadata.

## Personal instructions

- Load @AGENTS.personal.md from the vault root before classifying requests, if present. Absence is valid.
- Within this router's guardrails, apply current user direction first, personal instructions second, and generic skill guidance and workflow defaults third.
- Persist recurring personal directions (including “whenever you do X, do Y”) in `AGENTS.personal.md`, creating it when needed and keeping it ignored and untracked. It owns local tools, agents, access procedures, writing preferences, and delegation, monitoring, resumption and verification workflows. Record access references, not credentials. Preserve this managed router and versioned skills; write personal specializations only in the personal file. One-time requests do not create persistent rules.

## Investigation and local stores

`investigations/` is versionable but excluded from the knowledge graph. Local ignored stores are `.investigations-private/` · legacy `.investigations/` · `.operations/` · `.knowledge-os-handoffs/` · `.plan/` · `.knowledge-os-config.yaml`.

Local implementation plans belong only in `.plan/`. Never create a visible
`plan/` directory inside a cell vault: Obsidian indexes it as graph content.
