# Knowledge OS — router

This vault is an evidence-backed map of one cell's systems. Load skills by name; do not restate them here.

`tracker` means one external work-tracking instance declared in `instance.yaml`. A `work item` is one externally tracked unit identified by its exact tracker and provider-native reference.

Classify the request, then select one primary skill. Use `evidence-driven-analysis` for analysis, diagnosis or audit without a specialized workflow; `map-ecosystem` owns vault navigation, dependency questions and knowledge publication; `manage-operational-workflow` owns requested audits of a known flow or entity for a period and operational procedures. Operational audit procedures belong to the registered audit area; use related-area links for the technologies they inspect. An auxiliary method or access capability returns to the current owner without restarting routing. For orientation or interrogation, first hop is `90-Meta/graph-query.py` (JSON stems and edges, no note bodies). Open `instance.yaml` and `00-Home.md` for cell identity. Open `90-Meta/Alcance.md` only for cell-specific scope, allowlists, or exceptions. Open a section of `90-Meta/Convenciones.md` or `90-Meta/Auditoria - Framework.md` only when schema, node type, or an evidence gate is required. Do not load those Meta files before classifying.

Answer questions without a persistent record by default. Recommend an investigation when preserving evidence, decisions or pending questions would make continuation or sharing easier; open it through `manage-investigation` only when the user requests it or accepts the recommendation. A declined recommendation leaves the conversation unrecorded. An explicitly requested operational audit retains its `.operations/` workflow, not a second investigation.

Before using an existing case as context, use the read-only case load in `manage-investigation` to discover its private overlay by ID, even for an informational query. Public search results locate cases; they are not a substitute for loading their context. Preserve the returned private provenance and disclosure restrictions.

## Skills

- `evidence-driven-analysis` — proportional search, diagnosis and audit; conclusions with evidence and limits, with or without a case.
- `map-ecosystem` — query, document, synchronize, check prerequisites for a specified map operation, or answer where a fact belongs. First step is **orientation** when bootstrap is incomplete.
- `configure-workspace` — sole writer of local `.knowledge-os-config.yaml`. Onboarding is demand-triggered when a skill cannot resolve repositories or the user asks whether the workspace is initialized or configured.
- `manage-investigation` — versionable cases under `investigations/` with optional private overlays.
- `manage-investigation-derived-learning` — assess and publish `70-Aprendizajes/` notes.
- `manage-development-handoff` — persistent work-item worktrees and handoff files.
- `reconcile-development-handoff` — pull one selected handoff's implementation evidence into its source case while preserving its worktree-local lifecycle state.
- `manage-git-workflow` — own analysis and publication of the cell's Git/GitHub policy, or apply it to source repositories; routine local versioning of this vault follows repository instructions.
- `manage-operational-workflow` — audit flow executions or an entity by date through read-only evidence; draft or execute operational procedures; resume runs in `.operations/`. Uses inspection adapters for access and queries; business actions and publication require matching authorization.
- `inspect-database` — engine-neutral database evidence through the target’s configured runbook.
- `explain-visually` — auxiliary static or interactive explanations when requested or materially useful; temporary aids or source-linked retained artifacts.
- `grilling` — one decision at a time.
- `obsidian-cli`, `obsidian-markdown`, `obsidian-bases` — user-invoked Obsidian helpers.

For mapping closure or pending-work status, follow the skill's [mapping completion contract](.agents/skills/map-ecosystem/references/mapping-completion.md).

Adapters (only if listed in `instance.yaml` `adapters`):

- `generate-reports` for registered operational report recipes.

Provider-specific skills are selected and installed by the developer in the destination vault. Resolve their access through configured procedures; the kernel does not install provider tooling.

## Layout

`00-Home.md` · `10-Sistemas/` · `15-Arquitectura/` · `20-Repos/` · `25-Topics/` · `30-Flujos/` · `40-Integraciones/` · `50-Glosario/` · `60-Operacion/` · `70-Aprendizajes/` · `90-Meta/`

Instance files (`instance.yaml`, `00-Home.md`, notes under `10/`–`70/`) are never updated by the kernel installer.

## Guardrails

- Apply `90-Meta/evidence-policy.md` when assessing a technical write. Cell identity and capability bindings come from onboarding, never inferred company conventions.
- Before a material conclusion, inspect relevant available sources and verify that they support the precise claim. Distinguish observations, inferences, and proposals. Check alternative explanations when they could change the conclusion. When evidence is insufficient, limit the claim and name the missing verification. Scale inquiry to uncertainty and impact; reuse valid evidence.
- Never persist secrets.
- Canonical wikilinks target the note basename.
- Persist one durable relationship direction; use backlinks for the inverse.
- Messaging goes producer → topic → consumer when `topic` is enabled.
- Business flows live in `30-Flujos/`; human procedures live in `60-Operacion/`.
- Author skills and this file with `writing-for-agents`. Keep harness-portable: no provider-specific metadata.

## Distribution changes

Treat the distributed kernel and skills as managed dependencies. When a vault task requires changing them, diagnose the problem and present the proposed change and its impact for user approval before implementation. Completing the task or fixing its checks does not authorize distribution changes, even when a local distribution checkout is available. With explicit authorization already covering the change, proceed without asking again: prepare a PR to the distribution repository and keep integration and propagation within the authorized scope. Continue independent vault work while approval is pending.

## Personal instructions

- Load @AGENTS.personal.md from the vault root before classifying requests, if present. Absence is valid.
- Within this router's guardrails, apply current user direction first, personal instructions second, and generic skill guidance and workflow defaults third.
- When the user establishes or changes how future work should be done for them or in their local environment, create or update `AGENTS.personal.md` with that reusable rule, preserving unrelated preferences. Recurring directions imply persistence even without an explicit request to save; one-time task instructions do not.
- Write personal customizations only in `AGENTS.personal.md`; never edit the vault's root `AGENTS.md` or versioned skills to persist them. Keep the personal file ignored and untracked, and record access references instead of credentials.

## Investigation and local stores

`investigations/` is versionable but excluded from the knowledge graph. Local ignored stores are `.investigations-private/` · legacy `.investigations/` · `.operations/` · `.knowledge-os-handoffs/` · `.plan/` · `.knowledge-os-config.yaml`.

Local implementation plans belong only in `.plan/`. Never create a visible
`plan/` directory inside a cell vault: Obsidian indexes it as graph content.
