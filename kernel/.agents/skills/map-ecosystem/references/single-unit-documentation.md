# Document or refresh one unit

## Input and authority

Accept one target repository or durable vault node and confirm vault-update authority. Load [node-selection.md](../../../../90-Meta/node-selection.md), classify the target type and lifecycle action, and state the evidence boundary. Analysis or diagnosis alone returns to the interrogation branch; `tipo: aprendizaje` belongs to `manage-investigation-derived-learning`; Git/GitHub policy changes belong to `manage-git-workflow`.

Apply [repository-map.md](repository-map.md) to extraction and verification and [investigation-context.md](investigation-context.md) to locate associated cases. Publication always goes through a `sync/` branch: `sync start`, commit, `discover check`, [final-note-review.md](final-note-review.md), `sync review`, `sync verify`, `sync finish`.

## Branches of work

- **Correction at the mapped revision** (an investigation supplies exact evidence of a defect): verify the supplied identity/revision and the affected assertions, change only the affected meaning, keep `commit-analizado` unchanged.
- **External verification of a mapped connection**: reuse the repository baseline, inspect only the selected external authority (platform snapshot, database, control plane), record its identity, environment, observation time and safe retrieval reference, keep source metadata unchanged. It may close a pending item without a new commit. See connection-reconciliation.md.
- **Source delta or first map**: the steps below.

## Baseline and source

1. Read the existing note, system MOC, flows, nearby topics/events/integrations/glossary terms and backlinks. For a new target, search basenames, aliases, `nombre-raw` and backlinks before concluding that no canonical note exists.
2. `<cli> inventory --vault "<vault>" --repo <note-or-alias> --format markdown` decides the case (reference branch per `90-Meta/reference-branches.md`; with several GitHub accounts repeat with `--github-user <login>`, never `gh auth switch`):
   - `current`: stop and report, unless an explicit audit of the unchanged commit or new evidence justifies it (record the reason).
   - `changed`: record the old `commit-analizado` and new SHA; continue with the delta.
   - `new`: confirm ownership (prefixes or allowlist) and production branch; continue without an old SHA.
   - `acknowledged-*`: report the accepted decision and stop unless re-analysis is requested.
   - lifecycle, ambiguity or invalid note: resolve or report before reading sources.
3. Resolve the checkout by remote identity under the configured roots. A non-managed checkout is read-only (never fetch, checkout, reset or write there); clone or fetch only in the managed `CLONE_ROOT` per [vault-resolution.md](../../../../90-Meta/vault-resolution.md), with approval when no managed root exists. For `changed`, ensure both commits exist and inspect `git log --oneline <old>..<new>` and `git diff --stat <old>..<new>`.
4. `<cli> discover run --vault "<vault>" --repo <name>` gives the connector facts at the production head; they replace manual connector sweeps.

A non-repository target skips inventory and clones: inspect only the sources that establish its contract (message definitions and publishers/consumers or platform subscriptions for a topic/event, clients/configuration for an integration, participant evidence for a flow, usages for a glossary term, constituent nodes for a service/MOC). Never fabricate repository traceability for it.

## Modeling and writing

1. Confirm identity, placement, lifecycle action, naming and contract with Convenciones and node-selection.md.
2. Write the complete note with the fields and sections its contract requires. For repositories: purpose, triggers, inputs/outputs, rules, data, infrastructure, countries and relationships with evidence, compact runtime/environment information for its connectors, and under `Limitaciones y desconocimientos` one `Verificaciones pendientes` item per partial or unresolved connection with its exact check and close condition.
3. For a source re-analysis update `commit-analizado`, `fecha-analisis`, `rama-analizada` and `ultima-auditoria` together. When the delta needs no durable change, record `sync acknowledge --decision no-documentation-change` instead.
4. Propagate contract changes to every affected node (topics, events, flows, glossary, navigation); asynchronous topology goes producer → topic/event → consumer.
5. Express absence as “no observado en fuentes estáticas revisadas”; use `#por-confirmar` only when the limit affects business understanding.

Operational notes use the closed `tipo: operacional` contract, stay `borrador` while organization-specific content is unverified, and update `ultima-verificacion` only after checking the platform or owner; one `.operations/` run never becomes the procedure.

A `schema-repository` note synthesizes every represented database at business level (logical tables or explicit empty schemas, purpose, principal relationships); columns, DDL and indexes stay in the source. A repository confirmed to write to a represented database (observed mutation path plus configuration resolving it to that database) links the schema-repository note under **Persistencia y datos**, not in `consume-de`; readers and unconfirmed writers do not.

## Verification and completion

Answer the P1–P5 questions from the resulting note, pass `discover check`, obtain the review and pass `sync verify`. Apply [evidence-sufficiency.md](evidence-sufficiency.md) to pending items: keep unmet in-scope questions, retire obsolete ones explicitly without claiming a test passed.

Complete when: an unchanged repository is reported `current` with no file modified; or an acknowledgement is published; or the published note is evidence-backed at the declared level, every claim passes the evidence policy, connectors are resolved or explicitly limited, every affected node has a decision, traceability matches the analyzed commit and `sync verify` passed. An external-only update completes with its reviewed publication and each selected pending item answered, retained or retired.
