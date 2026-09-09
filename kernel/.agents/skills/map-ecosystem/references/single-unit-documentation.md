# Document or refresh one unit

## Input and authority

Accept one target repository or durable vault node and confirm that the user authorized a vault update. Load [node-selection.md](../../../../90-Meta/node-selection.md), classify the target type and lifecycle action, and state the evidence boundary. If the request is analysis or diagnosis only, return to the interrogation branch. If the target is `tipo: aprendizaje`, hand it to `manage-investigation-derived-learning`; this technical documentation recipe does not own that assessment or write.

## Baseline

1. Read the existing note, system MOC, flows, nearby topics/integrations/glossary terms, and backlinks. For a new target, search basenames, aliases, `nombre-raw`, and backlinks before concluding that no canonical note exists.
2. For repository targets, run `python3 90-Meta/vault-inventory.py --repo <note-or-alias> --format markdown` from `VAULT_ROOT` before cloning, fetching, or scanning. Treat this classification as the decision gate; it resolves the production branch by preferring `main` and using `master` only when `main` does not exist. The command also resolves GitHub identity without changing the global `gh` account. If it reports multiple accessible accounts, repeat it with the requested `--github-user <login>`; do not run `gh auth switch`.
3. Act on the inventory result:
   - `current`: stop before source scanning unless the user explicitly requested a forced audit of the unchanged commit. An explicit post-deployment production audit is such a request because productive applicability may have changed without a new source commit. Report the recorded and remote SHA when stopping; when continuing, reapply the production-evidence gate and leave source traceability unchanged unless the repository was actually re-analyzed.
   - `changed`: record the existing `commit-analizado`, production branch, and new 12-character remote SHA, then continue with the old-to-new comparison.
   - `new`: confirm ownership via instance.yaml prefixes or an explicit allowlist in the instance catalog, plus the production branch, then continue without an old SHA.
   - `acknowledged-no-change` or `acknowledged-no-node`: report the accepted decision at the inspected SHA and stop unless the user explicitly requests re-analysis. `acknowledged-review-rejected` or `acknowledged-inspection-limited`: report the outstanding limitation and retained note baseline; a closed inspection does not establish current documentation.
   - Any lifecycle, ambiguity, or invalid-note state: resolve it or report it as a blocker before scanning; do not silently treat it as `changed`.
4. For `changed`, `new`, or an explicitly forced audit, resolve the repository by remote identity under `SOURCE_ROOTS`. Reuse a non-managed checkout only when it already contains the required evidence; never fetch, checkout, reset, merge, or write there. When its root is also `managed: true`, switch explicitly to the `CLONE_ROOT` role before clone/fetch and follow [vault-resolution.md](../../../../90-Meta/vault-resolution.md); its existing working tree remains read-only. If no managed root is authorized, obtain explicit approval for an exact existing root and hand configuration to its owner skill before cloning.
5. For `changed`, ensure both the recorded commit and new HEAD are available. If a read-only source lacks either commit, switch to the managed `CLONE_ROOT` or request authorization; never deepen or fetch the read-only clone. In a managed clone, deepen the fetch or fetch the old commit explicitly when required. Inspect `git log --oneline <old>..<new>` and `git diff --stat <old>..<new>` before the full scan. Record the limitation if the remote no longer exposes the recorded commit.
6. For repository targets, run `<python> -B 90-Meta/static-evidence-scan.py --repo <note>` from `VAULT_ROOT` as an initial sweep. Add `--source-repo "<resolved-repository-path>"` only for an exact configured checkout or the detached temporary worktree produced after fetching a managed clone. Treat the report as a sweep, not a conclusion.

For a non-repository target, skip inventory and clone operations. Identify the repository notes, source clones, audited vault notes, or read-only GCP metadata that own the evidence; do not fabricate repository traceability for the target node.

## Evidence interrogation

For a repository target, inspect real entrypoints, routes/handlers, subscribers/publishers, HTTP clients, migrations/models, storage, flags, retries/idempotency, and versioned deployment files (`.github/workflows`, Cloud Build, Dockerfile, Kubernetes/Kustomize, Cloud Run, Functions, or Helm). Treat the README only as a clue. When deployment markers exist or the unit is expected to deploy, apply [deployment-evidence.md](deployment-evidence.md); a file list is not a resolved deployment target.

For any other node, inspect only the sources that establish its selected contract and lifecycle: message definitions and publishers/consumers for a topic, clients/configuration for an integration, participant evidence for a flow, observed usages for a glossary term, constituent nodes for a service/MOC/index, read-only platform or owner evidence for operational documentation, or control-plane metadata for runtime. Expand to a full repository sweep only when the node's conclusion depends on it.

Before modeling, use the claim-level production-evidence decisions required by the common contract. Re-inspect authoritative sources behind contextual inputs before relying on them.

For a repository target, complete this minimum sweep:

- **Inputs**: HTTP/OpenAPI, Pub/Sub/subscriptions, cron/CronJobs, Scheduler, Functions, Eventarc, Cloud Run, application, or user.
- **Outputs**: publications and payload/attributes, outgoing HTTP, data writes, integrations, fire-and-forget, retries, and DLQ.
- **Data**: engine, database/schema/dataset/bucket/collection, read/written tables, migrations/entities, and risky operations such as upsert, truncation, bulk deletion, or `synchronize: true`.
- **Business behavior**: validations, transformations, states, country/store/BU/brand/category/vendor, flags, deduplication, and idempotency.
- **Infrastructure**: runtime and project by environment, deployment, schedulers/subscriptions, config maps, service accounts, and secret references without exposing their values. For every deployable, trace the versioned event or input through its workflow/job and artifact or overlay to the project, platform, resource, location, namespace/workload, and environment recorded in the deployment matrix.

Search across repositories for the names, modules, endpoints, topics, subscriptions, environment variables, tables, schedulers, services, or terms relevant to the selected node. Query the GCP control plane with `list`/`describe` only when it resolves a relevant relationship or runtime.

For System B PostgreSQL, distinguish a confirmed writer from a reader or candidate owner. A backlink to `[[schema-repository]]` requires both an observed business-data mutation path in application code or a versioned job and versioned configuration that resolves that path to a database represented in `schema-repository`; names, unused entities, baselines/migrations alone, stale templates, read-only queries, or inferred ownership are insufficient.

## Modeling and writing

1. Confirm identity, placement, lifecycle action, naming, and contract using Convenciones and [node-selection.md](../../../../90-Meta/node-selection.md).
2. Extract the fields and sections required by the selected node contract. For repositories, include purpose, triggers, inputs/outputs, rules, data, infrastructure, countries, and relationships with supporting evidence. A deployable repository must include the per-environment deployment matrix required by Convenciones.
3. Edit the target note using the exact Convenciones contract.
4. When publishing a repository documentation update, update `commit-analizado`, `fecha-analisis`, `rama-analizada`, and `ultima-auditoria` together. If inspection finds no necessary documentation change, use the accepted acknowledgement path in [vault-synchronization.md](vault-synchronization.md), preserving the note baseline; a metadata-only request uses that same reviewed closure.
5. Propagate contract changes to every node class required by [node-selection.md](../../../../90-Meta/node-selection.md), including relevant glossary and navigation nodes. Keep asynchronous topology routed through the topic.
6. Express absence using the required Spanish wording “no observado en fuentes estáticas revisadas”; use `#por-confirmar` only when the limitation affects business understanding.

For an operational note, use the closed `tipo: operacional` contract, keep unverified organization-specific content in `borrador`, and update `ultima-verificacion` only after checking the relevant platform or owner. Never turn one `.operations/` run into the normative procedure.

When the target is `schema-repository`, synthesize every represented database at business level: list each logical table or explicit empty schema, its domain/purpose, and its principal verified or functional relationships. Keep physical columns, data types, constraints, indexes, DDL, partitions, and exhaustive ER diagrams in the source repository. Reconcile this map and the backlinks from confirmed writer notes in the same change.

When another System B repo is confirmed to write to a represented database, add `[[schema-repository]]` in **Persistencia y datos** and name the covered database/schema. Do not add it to `consume-de` and do not add the link for read-only or unconfirmed relationships.

## Verification

Run the Framework gates, inspect the complete diff, and confirm that unrelated work and traceability for repositories that were not re-analyzed remain unchanged.

## Completion criterion

For an unchanged repository, the unit is complete when the inventory proves that remote HEAD equals `commit-analizado`, the result is reported as `current`, and no file was modified. An accepted no-documentation-change inspection completes through the synchronization acknowledgement and receipt, with the note baseline retained and that outcome reported. For a scanned repository with a documentation update, completion requires evidence-backed identity and productive behavior, every written claim passing the production-evidence gate, a contract-compliant note, a resolved or explicitly limited deployment row for every observed environment and deployable, an explicit decision for every affected node, traceability updated to the scanned branch and commit, and all applicable gates passing. For any other node, completion requires an evidence-backed productive subject and behavior, every written claim passing the same gate, a valid type and lifecycle action, canonical placement and schema, no duplicate node, explicit propagation decisions, and all applicable gates passing.
