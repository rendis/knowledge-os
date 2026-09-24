# Knowledge OS — router

This vault is an evidence-backed map of one cell's systems.

## Evidence and completion

Apply this contract to every material answer, diagram and retained conclusion; loading a skill is not a prerequisite.

1. Establish the user's question, intended outcome and authorized scope. Bind the actual vault, repository, entity, environment and revision/time before relying on their evidence; a matching folder name is not identity.
2. Inspect sources that support the exact claim. Search results, titles, case narratives and prior knowledge locate evidence; they do not replace it. Distinguish documented intent, implementation, configuration, deployment, execution and persisted or business outcomes. Tests or a successful tool exit alone do not prove production behavior.
3. Resolve accessible, authorized checks that can change the answer. Trace relevant callers, conditions, consumers and outcomes; investigate contradictions and evidence against the leading explanation. Establish a hypothesis's connection to this case before using it as a cause. Resolve locally answerable parts before reporting unavailable external evidence. A missing source blocks only dependent claims.
4. Check support, correspondence, sufficiency and completeness. Each material assertion needs inspected evidence at the claimed level. A flag's value does not establish its operational effect without consumer use. Samples, truncated results and missing search hits are bounded observations; a type inventory requires an exhaustive query using declared types. Include known material branches in process explanations and diagrams; show unknown segments or label the view partial. Do not connect verified steps across an unverified gap.
5. Preserve established facts and valid checks; retract conclusions contradicted by new evidence. Separate observations, grounded inferences, proposals and unresolved questions. Complete the scoped work when the sources support the answer and remaining limitations are explicit; continue necessary checks within authority, and stop expansion when another read cannot change the scoped answer.
6. Lead with the answer in clear, concise language. Include decisive references and limits, even for a one-sentence conclusion. Explain unfamiliar terms and preserve conditions when simplifying into prose, tables or diagrams. Deliver the result, not investigation narration.

### Review before delivery

- For a new bounded, low-impact conclusion, perform a distinct author verification pass against the contract above. A short or read-only answer is not automatically low impact.
- Obtain independent acceptance **before delivery** for new operational status, eligibility, enablement or cause classifications; recommendations that could justify intervention or closure; conflicting sources; and uncertain cross-component relationships material to correctness. Operational-flow explanations or diagrams that could guide a procedure also qualify. The author, including a delegated author, requests review and waits; the reviewer does not delegate another review.
- Give the reviewer the question, candidate answer, scope and actual source references, not only the author's summary. It must inspect decisive evidence and check both unsupported assertions and omitted conclusions the evidence can establish. Correct specific defects and recheck affected claims and dependent conclusions.
- Reuse prior acceptance only when its review evidence, claims, entity, environment, revision/period and intended use still match, without a new inference or contradiction. User agreement or a previous answer alone is not review. Recheck the resulting meaning after transformations; additions need review. Preserve enough review context in the existing workflow for reuse; no new case or file is required.
- If independent review is unavailable or prohibited, perform an author check, briefly disclose the missing review and deliver supported observations and precise limits. Withhold the classification or recommendation requiring independent acceptance. Stricter publication and executor gates remain mandatory.
- Apply the bounded repair policy in execution profiles to delegated review. For author-only review, stop the affected conclusion if the same defect survives one focused correction; report supported findings and the remaining limitation. Review acceptance is not a guarantee of truth.

### Initiative within authority

Complete and verify authorized work through its owner. Preserve valid progress across follow-ups and compaction. When a concrete gap remains, explain its consequence and smallest useful next action; do not turn unchanged work into recurring suggestions. Analysis alone authorizes no mutation, schedule, deployment or publication. When an investigation closes, a production deployment is verified, a durable documentation gap appears, or map/sync begins, check relevant knowledge deltas through [associated investigation evidence](.agents/skills/map-ecosystem/references/investigation-context.md); follow the responsible workflow and its write authority.

## Personal instructions

- Load @AGENTS.personal.md from the vault root before classifying requests, if present. Absence is valid.
- Within this router's guardrails, apply current user direction first, personal instructions second, and generic skill guidance and workflow defaults third.
- When the user establishes or changes how future work should be done for them or in their local environment, create or update `AGENTS.personal.md` with that reusable rule, preserving unrelated preferences. Recurring directions imply persistence even without an explicit request to save; one-time task instructions do not.
- Write personal customizations only in `AGENTS.personal.md`; never edit the vault's root `AGENTS.md` or versioned skills to persist them. Keep the personal file ignored and untracked, and record access references instead of credentials.

## Kernel CLI

Use the installed `vaultctl` for kernel configuration, indexed retrieval, structural checks and workflow state operations it supports. Before the first CLI operation, read [use-vault-cli](.agents/skills/use-vault-cli/SKILL.md); reuse it while its instructions remain in context. This is a required operational reference, independent of primary-skill selection.

Before reading domain notes or source code, run `<CLI> config resolve --vault "<candidate-path>"` with the executable selected by `use-vault-cli` and bind its successful result. Load the router, personal instructions and relevant skill/access references as bootstrap context; domain-note reads start only after resolution. Reuse that binding until its inputs change.

Use search when the starting source is unknown, links for relationship discovery, and direct reads for an identified source. Retrieval results locate evidence; verify the source under the evidence contract above. Empty results do not establish absence. Keep configuration and lifecycle mutations with their owning skills and existing authorization gates.

## Routing

`tracker` means one external work-tracking instance declared in `instance.yaml`. A `work item` is one externally tracked unit identified by its exact tracker and provider-native reference.

1. Classify the request and select one primary skill from the catalog below. Loading an auxiliary skill supplies a method, not a new owner: keep the selected workflow through evidence gathering and the answer.
   When the user requests repository implementation, select work-in-repository for that scope; auxiliary context loading alone retains the current owner.
   Reclassify each follow-up for newly requested methods or presentation. Keep the existing owner and applicable evidence; an explicit diagram or visual explanation invokes `explain-visually` as an auxiliary.
   Before working in a source repository, use work-in-repository to load its applicable instructions and relevant skills; reuse the binding while its target and inputs remain current.
2. For initial orientation, use `config status --vault "<root>"`. Follow `use-vault-cli` for retrieval and result handling. For repository identity and failed-binding recovery, load [vault resolution](90-Meta/vault-resolution.md) before locating or reading a checkout.
3. After classification, load only the context required by these triggers:
   - Cell identity: `instance.yaml` and `00-Home.md`.
   - Cell-specific scope, allowlists, or exceptions: `90-Meta/Alcance.md`.
   - Schema, node type, or evidence gate: the relevant section of `90-Meta/Convenciones.md` or `90-Meta/Auditoria - Framework.md`.

Operational audit procedures belong to the registered audit area; link the technologies they inspect through related areas.

Answer questions without a persistent record by default. Recommend an investigation when preserving evidence, decisions or pending questions would make continuation or sharing easier; open it through `manage-investigation` only when the user requests it or accepts the recommendation. A declined recommendation leaves the conversation unrecorded. An explicitly requested operational audit retains its `.operations/` workflow, not a second investigation.

For vault-local helper scripts or code without a selected investigation or a workflow-defined working location, including when case association is uncertain, use a unique `VAULT_ROOT/.scratch/<task>/` directory. This directory is local and Git-ignored. If the work later belongs to an investigation, move the relevant files into that case through `manage-investigation`: local working material goes in its private `local/` store, while reviewed methods or evidence that merit retention go in its `artifacts/` register. Remove only the files this task created in `.scratch/` after verifying the move. Do not open an investigation solely to hold temporary code.

Before using an existing case as context, use the read-only case load in `manage-investigation` to discover its unpublished or published path and private overlay by ID, even for an informational query. Search results locate cases; they are not a substitute for loading their context. Preserve the returned private and local-working provenance and disclosure restrictions.

## Delegation and execution

Stay with the current agent unless a subtask is bounded, independently checkable, and cheaper than coordinating another executor. For that subtask, use the cheapest capable executor the harness exposes. Run deterministic checks as tools. Keep operational status, eligibility, enablement, and failure-cause classification with the coordinating agent.

Project specialists are `evidence-investigator` for bounded independent evidence questions and `evidence-reviewer` for the independent acceptance required above. Give them the question, resolved scope, available sources, restrictions and expected result; give the reviewer the candidate answer too. Use the investigator only when delegation earns its coordination cost, not on every query. A specialist's response is evidence for the coordinator, not permission to publish or operate. If a native role is unavailable, use an allowed generic executor with the same role instructions from `90-Meta/specialists/`, or report required review unavailable. See [specialist availability](90-Meta/specialists.md) when configuring or diagnosing discovery.

Personal executor preferences override these defaults; evidence, authorization, and workflow ownership still apply. Load [execution profiles](90-Meta/execution-profiles.md) only when dispatching or reconsidering a subagent. Preserve the primary workflow, its evidence gates, authorized scope, and exclusive ownership of shared-state writes across delegation.

## Skills

- [evidence-driven-analysis](.agents/skills/evidence-driven-analysis/SKILL.md) — analysis, diagnosis or audit without a specialized workflow; with or without a case.
- [map-ecosystem](.agents/skills/map-ecosystem/SKILL.md) — vault navigation, dependency questions, knowledge publication, related-vault catalog, and placement of facts. First step is **orientation** when bootstrap is incomplete.
- [synchronize-ecosystem](.agents/skills/synchronize-ecosystem/SKILL.md) — inventory, knowledge-map synchronization, resume of a recorded sync run, or a `SYNC_PACKAGE_WORKER_V1` card.
- [scheduled-vault-refresh](.agents/skills/scheduled-vault-refresh/SKILL.md) — guide an authorized manual or scheduled mapping cycle that reconciles source and investigation deltas, then follows the team's review or Git publication path.
- [configure-workspace](.agents/skills/configure-workspace/SKILL.md) — sole writer of local `.knowledge-os-config.yaml`. Onboarding is demand-triggered when a skill cannot resolve repositories or the user asks whether the workspace is initialized or configured.
- [manage-investigation](.agents/skills/manage-investigation/SKILL.md) — unpublished cases under `.investigations/` by default, published cases under `investigations/`, optional private overlays and local working stores.
- [manage-investigation-derived-learning](.agents/skills/manage-investigation-derived-learning/SKILL.md) — assess and publish `70-Aprendizajes/` notes.
- [work-in-repository](.agents/skills/work-in-repository/SKILL.md) — repository implementation from this session, or auxiliary loading of repository instructions and skills.
- [manage-development-handoff](.agents/skills/manage-development-handoff/SKILL.md) — persistent work-item worktrees and handoff files.
- [reconcile-development-handoff](.agents/skills/reconcile-development-handoff/SKILL.md) — pull one selected handoff's implementation evidence into its source case while preserving its worktree-local lifecycle state.
- [manage-git-workflow](.agents/skills/manage-git-workflow/SKILL.md) — own analysis and publication of the cell's specific Git/GitHub policy notes and their area-index links, or apply those notes to source repositories and standalone Git/GitHub operations; routine local versioning of this vault follows repository instructions.
- [manage-operational-workflow](.agents/skills/manage-operational-workflow/SKILL.md) — requested audits of a known flow or entity for a period through read-only evidence; draft or execute operational procedures, including applicable release procedures with Git/GitHub steps; resume runs in `.operations/`. Uses inspection adapters for access and queries; business actions and publication require matching authorization.
- [inspect-database](.agents/skills/inspect-database/SKILL.md) — engine-neutral database evidence through the target’s configured runbook.
- [explain-visually](.agents/skills/explain-visually/SKILL.md) — auxiliary static or interactive explanations when requested or materially useful; temporary aids or source-linked retained artifacts.
- [obsidian-markdown](.agents/skills/obsidian-markdown/SKILL.md) — author Obsidian-specific note syntax, including wikilinks, embeds, callouts and properties.
- [obsidian-bases](.agents/skills/obsidian-bases/SKILL.md) — create or edit `.base` views, filters and formulas.
- [obsidian-cli](.agents/skills/obsidian-cli/SKILL.md) — interact with the Obsidian application, query Base views or verify app-specific behavior; use `vaultctl` for ordinary kernel retrieval.

For mapping closure or pending-work status, use `map-ecosystem`.

Adapters (only if listed in `instance.yaml` `adapters`):

- [generate-reports](.agents/skills/generate-reports/SKILL.md) for registered operational report recipes.

Provider-specific skills are selected and installed by the developer in the destination vault. Resolve their access through configured procedures; the kernel does not install provider tooling.

## Layout

`00-Home.md` · `10-Sistemas/` · `15-Arquitectura/` · `20-Repos/` · `25-Topics/` · `30-Flujos/` · `40-Integraciones/` · `50-Glosario/` · `60-Operacion/` · `70-Aprendizajes/` · `90-Meta/`

Instance files (`instance.yaml`, `00-Home.md`, notes under `10/`–`70/`) are never updated by the kernel installer.

## Guardrails

- Apply `90-Meta/evidence-policy.md` when assessing a technical write. Cell identity and capability bindings come from onboarding, never inferred company conventions.
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

`investigations/` is versionable and searchable in Obsidian, but excluded from the kernel's technical graph traversal and audit gates; the graph helper can still look up investigations explicitly. Local ignored stores are `.investigations/` · `.investigations-private/` · `.operations/` · `.knowledge-os-handoffs/` · `.plan/` · `.scratch/` · `.knowledge-os-config.yaml`.

Local implementation plans belong only in `.plan/`. Never create a visible
`plan/` directory inside a cell vault: Obsidian indexes it as graph content.
