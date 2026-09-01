# Knowledge OS — router

This vault is an evidence-backed map of one cell's systems. Load skills by name; do not restate them here.

`tracker` means one external work-tracking instance declared in `instance.yaml`. A `work item` is one externally tracked unit identified by its exact tracker and provider-native reference.

Classify the request, then select one skill. For orientation or interrogation, first hop is `90-Meta/graph-query.py` (JSON stems and edges, no note bodies). Open `instance.yaml` and `00-Home.md` for cell identity. Open `90-Meta/Alcance.md` only for cell-specific scope, allowlists, or exceptions. Open a section of `90-Meta/Convenciones.md` or `90-Meta/Auditoria - Framework.md` only when schema, node type, or an evidence gate is required. Do not load those Meta files before classifying.

## Skills

- `map-ecosystem` — query, document, synchronize, confirm tooling readiness, or answer where a fact belongs. First step is **orientation** when bootstrap is incomplete.
- `configure-workspace` — sole writer of local `.knowledge-os-config.yaml`. Onboarding is demand-triggered when a skill cannot resolve repositories or the user asks whether the workspace is initialized or configured.
- `manage-investigation` — local case files under `.investigations/`.
- `manage-investigation-derived-learning` — assess and publish `70-Aprendizajes/` notes.
- `manage-development-handoff` — persistent work-item worktrees and handoff files.
- `reconcile-development-handoff` — pull implementation evidence from a registered worktree into its source case and coordinate completed closure.
- `manage-git-workflow` — analyze or maintain the cell's Git/GitHub policy, or apply it to source repositories; routine local versioning of this vault follows repository instructions.
- `manage-operational-workflow` — runbooks with external effects after authorization.
- `grilling` — one decision at a time.
- `obsidian-cli`, `obsidian-markdown`, `obsidian-bases` — user-invoked Obsidian helpers.

Adapters (only if listed in `instance.yaml` `adapters`):

- `inspect-gcp-runtime` with `gcloud` leaf syntax.
- `inspect-database` for a declared schema repository.
- `generate-reports` for registered operational report recipes.

## Layout

`00-Home.md` · `10-Sistemas/` · `15-Arquitectura/` · `20-Repos/` · `25-Topics/` · `30-Flujos/` · `40-Integraciones/` · `50-Glosario/` · `60-Operacion/` · `70-Aprendizajes/` · `90-Meta/`

Instance files (`instance.yaml`, `00-Home.md`, notes under `10/`–`70/`) are never updated by the kernel installer.

## Guardrails

- Production-only durable technical truth when `evidence.profile` is `production-gate` or the technical half of `mixed`. `documented-source` accepts versioned source without a deploy chain.
- Evidence before inference. Separate facts, inferences, and limitations.
- Never persist secrets.
- Canonical wikilinks target the note basename.
- Persist one durable relationship direction; use backlinks for the inverse.
- Messaging goes producer → topic → consumer when `topic` is enabled.
- Business flows live in `30-Flujos/`; human procedures live in `60-Operacion/`.
- Author skills and this file with `writing-for-agents`. Keep harness-portable: no provider-specific metadata.

## Local stores (Git-ignored)

`.investigations/` · `.operations/` · `.knowledge-os-handoffs/` · `.plan/` · `.knowledge-os-config.yaml`

Local implementation plans belong only in `.plan/`. Never create a visible
`plan/` directory inside a cell vault: Obsidian indexes it as graph content.
