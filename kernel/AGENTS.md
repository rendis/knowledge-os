# Knowledge OS — router

This vault is an evidence-backed map of one cell's systems. Load skills by name; do not restate them here.

## Personal instructions

- Load @AGENTS.personal.md from the vault root before classifying requests, if present. Absence is valid.
- Within this router's guardrails, apply current user direction first, personal instructions second, and generic skill guidance and workflow defaults third.
- When the user establishes or changes how future work should be done for them or in their local environment, create or update `AGENTS.personal.md` with that reusable rule, preserving unrelated preferences. Recurring directions imply persistence even without an explicit request to save; one-time task instructions do not.
- Write personal customizations only in `AGENTS.personal.md`; never edit the vault's root `AGENTS.md` or versioned skills to persist them. Keep the personal file ignored and untracked, and record access references instead of credentials.

## Routing

`tracker` means one external work-tracking instance declared in `instance.yaml`. A `work item` is one externally tracked unit identified by its exact tracker and provider-native reference.

Before reading domain notes or source code, run `90-Meta/resolve-vault.py` (with `--path` when supplied) and bind its successful result. Load the router, personal instructions and relevant skill/access references as bootstrap context; domain-note reads start only after resolution. Reuse that binding until its inputs change.

1. Classify the request and select one primary skill from the catalog below. Loading an auxiliary skill supplies a method, not a new owner: keep the selected workflow through evidence gathering and the answer.
2. For orientation or relationship discovery, start with `90-Meta/graph-query.py` (JSON stems and edges, no note bodies). Read an already identified source directly for a bounded question that needs no graph discovery; For repository identity and failed-binding recovery, load [vault resolution](90-Meta/vault-resolution.md) before locating or reading a checkout.
3. After classification, load only the context required by these triggers:
   - Cell identity: `instance.yaml` and `00-Home.md`.
   - Cell-specific scope, allowlists, or exceptions: `90-Meta/Alcance.md`.
   - Schema, node type, or evidence gate: the relevant section of `90-Meta/Convenciones.md` or `90-Meta/Auditoria - Framework.md`.

Operational audit procedures belong to the registered audit area; link the technologies they inspect through related areas.

Answer questions without a persistent record by default. Recommend an investigation when preserving evidence, decisions or pending questions would make continuation or sharing easier; open it through `manage-investigation` only when the user requests it or accepts the recommendation. A declined recommendation leaves the conversation unrecorded. An explicitly requested operational audit retains its `.operations/` workflow, not a second investigation.

Before using an existing case as context, use the read-only case load in `manage-investigation` to discover its private overlay by ID, even for an informational query. Public search results locate cases; they are not a substitute for loading their context. Preserve the returned private provenance and disclosure restrictions.

## Delegation and execution

Stay with the current agent unless a subtask is bounded, independently checkable, and cheaper than coordinating another executor. For that subtask, use the cheapest capable executor the harness exposes. Run deterministic checks as tools. Keep operational status, eligibility, enablement, and failure-cause classification with the coordinating agent.

Personal executor preferences override these defaults; evidence, authorization, and workflow ownership still apply. Load [execution profiles](90-Meta/execution-profiles.md) only when dispatching or reconsidering a subagent. Preserve the primary workflow, its evidence gates, authorized scope, and exclusive ownership of shared-state writes across delegation.

## Skills

- `evidence-driven-analysis` — analysis, diagnosis or audit without a specialized workflow; with or without a case.
- `map-ecosystem` — vault navigation, dependency questions, knowledge publication, related-vault catalog, and placement of facts. First step is **orientation** when bootstrap is incomplete.
- `synchronize-ecosystem` — inventory, knowledge-map synchronization, resume of a recorded sync run, or a `SYNC_PACKAGE_WORKER_V1` card.
- `configure-workspace` — sole writer of local `.knowledge-os-config.yaml`. Onboarding is demand-triggered when a skill cannot resolve repositories or the user asks whether the workspace is initialized or configured.
- `manage-investigation` — versionable cases under `investigations/` with optional private overlays.
- `manage-investigation-derived-learning` — assess and publish `70-Aprendizajes/` notes.
- `manage-development-handoff` — persistent work-item worktrees and handoff files.
- `reconcile-development-handoff` — pull one selected handoff's implementation evidence into its source case while preserving its worktree-local lifecycle state.
- `manage-git-workflow` — own analysis and publication of the cell's Git/GitHub policy, or apply it to source repositories; routine local versioning of this vault follows repository instructions.
- `manage-operational-workflow` — requested audits of a known flow or entity for a period through read-only evidence; draft or execute operational procedures; resume runs in `.operations/`. Uses inspection adapters for access and queries; business actions and publication require matching authorization.
- `inspect-database` — engine-neutral database evidence through the target’s configured runbook.
- `explain-visually` — auxiliary static or interactive explanations when requested or materially useful; temporary aids or source-linked retained artifacts.
- `obsidian-cli`, `obsidian-markdown`, `obsidian-bases` — user-invoked Obsidian helpers.

For mapping closure or pending-work status, use `map-ecosystem`.

Adapters (only if listed in `instance.yaml` `adapters`):

- `generate-reports` for registered operational report recipes.

Provider-specific skills are selected and installed by the developer in the destination vault. Resolve their access through configured procedures; the kernel does not install provider tooling.

## Layout

`00-Home.md` · `10-Sistemas/` · `15-Arquitectura/` · `20-Repos/` · `25-Topics/` · `30-Flujos/` · `40-Integraciones/` · `50-Glosario/` · `60-Operacion/` · `70-Aprendizajes/` · `90-Meta/`

Instance files (`instance.yaml`, `00-Home.md`, notes under `10/`–`70/`) are never updated by the kernel installer.

## Guardrails

- Apply `90-Meta/evidence-policy.md` when assessing a technical write. Cell identity and capability bindings come from onboarding, never inferred company conventions.
- Before material conclusions, inspect evidence supporting the exact claim and apply [response quality](90-Meta/response-quality.md), including its review and reuse conditions. This applies to conversational answers as well as retained documents. New operational status, eligibility and cause classifications require independent review before delivery, even for read-only questions; reuse prior independent acceptance only under the linked reuse conditions.
- Communicate in clear, concise language: answer first, explain unfamiliar terms, and retain evidence and limits that affect the decision. Match detail to the reader; omit repetition and the search diary.
- Execute only after required deterministic checks pass for the exact operation. Correct a rejected input and recheck, or report the blocker; a tool failure never grants permission to bypass its contract.
- Never persist secrets.
- Canonical wikilinks target the note basename.
- Persist one durable relationship direction; use backlinks for the inverse.
- Messaging goes producer → topic → consumer when `topic` is enabled.
- Business flows live in `30-Flujos/`; human procedures live in `60-Operacion/`.
- Before executing an operational procedure, apply the [execution eligibility contract](.agents/skills/manage-operational-workflow/references/procedure-contract.md#execution-eligibility), including procedures used by access capabilities.
- Keep instructions harness-portable: no provider-specific metadata. Resolve technology-specific execution through the cell's configured procedures.

## Distribution changes

Treat the distributed kernel and skills as managed dependencies. When a vault task requires changing them, diagnose the problem and present the proposed change and its impact for user approval before implementation. Completing the task or fixing its checks does not authorize distribution changes, even when a local distribution checkout is available. With explicit authorization already covering the change, proceed without asking again: prepare a PR to the distribution repository and keep integration and propagation within the authorized scope. Continue independent vault work while approval is pending.

## Investigation and local stores

`investigations/` is versionable but excluded from the knowledge graph. Local ignored stores are `.investigations-private/` · legacy `.investigations/` · `.operations/` · `.knowledge-os-handoffs/` · `.plan/` · `.knowledge-os-config.yaml`.

Local implementation plans belong only in `.plan/`. Never create a visible
`plan/` directory inside a cell vault: Obsidian indexes it as graph content.
