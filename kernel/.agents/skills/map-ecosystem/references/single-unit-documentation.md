# Document or refresh one unit

## Input and authority

Accept one target repository or durable vault node and confirm that the user authorized a vault update. Load [node-selection.md](../../../../90-Meta/node-selection.md), classify the target type and lifecycle action, and state the evidence boundary. If the request is analysis or diagnosis only, return to the interrogation branch. If the target is `tipo: aprendizaje`, hand it to `manage-investigation-derived-learning`; this technical documentation recipe does not own that assessment or write.

Apply [evidence-extraction.md](evidence-extraction.md) throughout extraction and verification.

For an external verification of an already mapped connection, reuse the repository baseline and inspect only the selected external authority. Use this documentation recipe outside the closed Git synchronization run: a provider observation is not a repository Git blob. Record the external evidence identity, environment, observation time and safe retrieval reference in the candidate note. Review and check the complete candidate through [final-note-review.md](final-note-review.md) before publishing it. Preserve source analysis metadata when the source repository was not re-analyzed. This branch may resolve a pending item even when the repository commit is unchanged; it does not alter a sync receipt or bypass a failed source review.

For Git/GitHub policy maintenance, return the selected policy change to `manage-git-workflow` before entering extraction or publication. Normative rules use that workflow’s policy evidence; they are not claims of inspected implementation.

## Baseline

For an investigation correction package backed by exact evidence of a defect at the mapped revision, perform step 1 and verify the supplied source identity/revision and affected assertions. Reuse the existing baseline and relevant evidence; skip steps 2–6 unless a concrete unresolved part of the defect needs them. In Evidence interrogation below, apply P1–P5 only to the affected meaning and directly impacted relationships, preserving the rest. Keep source analysis metadata unchanged for a correction at the same revision. A newer implementation follows the normal delta branch instead. Publication still requires independent review of complete resulting notes; a confirmed correction is not a `current` no-write outcome.

For external-only reconciliation, perform step 1 and reuse the accepted source baseline; skip source inventory, acquisition and scanning in steps 2–6. Follow connection-reconciliation.md for the selected external evidence. Any discovered source contradiction becomes a separate source update, not a silent refresh of source metadata.

1. Read the existing note, system MOC, flows, nearby topics/integrations/glossary terms, and backlinks. For a new target, search basenames, aliases, `nombre-raw`, and backlinks before concluding that no canonical note exists.
2. For repository targets, run `python3 90-Meta/vault-inventory.py --repo <note-or-alias> --format markdown` from `VAULT_ROOT` before cloning, fetching, or scanning. Treat this classification as the decision gate; it resolves the reference branch according to `90-Meta/reference-branches.md`. The command also resolves GitHub identity without changing the global `gh` account. If it reports multiple accessible accounts, repeat it with the requested `--github-user <login>`; do not run `gh auth switch`.
3. Act on the inventory result:
   - `current`: reuse the baseline unless the request explicitly calls for auditing an unchanged commit or supplied evidence changes a dependency, profile, or previously unresolved source. Record the exact same-commit audit reason and scope. An explicit post-deployment production audit is such a request because productive applicability may have changed without a new source commit. Report the recorded and remote SHA when stopping; when continuing, reapply the profile-specific evidence policy and leave source traceability unchanged unless the repository was actually re-analyzed.
   - `changed`: record the existing `commit-analizado`, production branch, and new 12-character remote SHA, then continue with the old-to-new comparison.
   - `new`: confirm ownership via instance.yaml prefixes or an explicit allowlist in the instance catalog, plus the production branch, then continue without an old SHA.
   - `acknowledged-no-change` or `acknowledged-no-node`: report the accepted decision at the inspected SHA and stop unless the user explicitly requests re-analysis. `acknowledged-review-rejected` or `acknowledged-inspection-limited`: report the outstanding limitation and retained note baseline; a closed inspection does not establish current documentation.
   - Any lifecycle, ambiguity, or invalid-note state: resolve it or report it as a blocker before scanning; do not silently treat it as `changed`.
4. For `changed`, `new`, or an explicitly forced audit, resolve the repository by remote identity under `SOURCE_ROOTS`. Reuse a non-managed checkout only when it already contains the required evidence; never fetch, checkout, reset, merge, or write there. When its root is also `managed: true`, switch explicitly to the `CLONE_ROOT` role before clone/fetch and follow [vault-resolution.md](../../../../90-Meta/vault-resolution.md); its existing working tree remains read-only. If no managed root is authorized, obtain explicit approval for an exact existing root and hand configuration to its owner skill before cloning.
5. For `changed`, ensure both the recorded commit and new HEAD are available. If a read-only source lacks either commit, switch to the managed `CLONE_ROOT` or request authorization; never deepen or fetch the read-only clone. In a managed clone, deepen the fetch or fetch the old commit explicitly when required. Inspect `git log --oneline <old>..<new>` and `git diff --stat <old>..<new>` before the bounded map update. Record the limitation if the remote no longer exposes the recorded commit.
6. For repository targets, run `<python> -B 90-Meta/static-evidence-scan.py --repo <note>` from `VAULT_ROOT` as an initial sweep. Add `--source-repo "<resolved-repository-path>"` only for an exact configured checkout or the detached temporary worktree produced after fetching a managed clone. Treat the report as a sweep, not a conclusion.

For a non-repository target, skip inventory and clone operations. Identify the repository notes, source clones, audited vault notes, or authorized read-only external metadata that own the evidence; do not fabricate repository traceability for the target node.

## Evidence interrogation

For a repository target, follow [repository-map.md](repository-map.md): purpose, stack, main triggers, flow paths, connectors and meaningful logic. Use README/docs when supported by implementation. Load deployment-evidence.md only for an explicit deployment audit or a concrete unresolved connector/trigger; unresolved cloud edges are reconciled centrally after local mapping.

For any other node, inspect only the sources that establish its selected contract and lifecycle: message definitions and publishers/consumers for a topic, clients/configuration for an integration, participant evidence for a flow, observed usages for a glossary term, constituent nodes for a service/MOC/index, read-only platform or owner evidence for operational documentation, or control-plane metadata for runtime. Expand to a full repository sweep only when the node's conclusion depends on it.

Before modeling, use the claim-level profile-specific evidence decisions required by the common contract. Re-inspect authoritative sources behind contextual inputs before relying on them.

Record actual input/output and read/write contracts, their destinations/configuration keys, important transformations/conditions and source references. Mark unavailable destinations and secondary details explicitly. Cross-repository/cloud matching follows local extraction under repository-map.md; naming similarity never proves a connection.

For a declared schema repository, distinguish a confirmed writer from a reader or candidate owner. A backlink to the canonical note named by `instance.yaml` `sources.schema_repository.note` requires both an observed business-data mutation path in application code or a versioned job and versioned configuration that resolves that path to a database represented in that schema repository; names, unused entities, baselines/migrations alone, stale templates, read-only queries, or inferred ownership are insufficient.

## Modeling and writing

1. Confirm identity, placement, lifecycle action, naming, and contract using Convenciones and [node-selection.md](../../../../90-Meta/node-selection.md).
2. Extract the fields and sections required by the selected node contract. For repositories, include purpose, triggers, inputs/outputs, rules, data, infrastructure, countries, and relationships with supporting evidence. Use compact runtime/schedule and environment information needed for its connectors. Under `Limitaciones y desconocimientos`, project each partial or unresolved `connection.*` claim into one `Verificaciones pendientes` item with its exact check and close condition. The full deployment matrix is conditional under Convenciones.
3. Prepare the complete candidate note outside the knowledge graph using the exact Convenciones contract. Preserve stable connection anchors for resolved as well as pending connections. Apply it only after the final-note review below accepts its exact bytes.
4. When publishing a repository documentation update based on source re-analysis (not external-only reconciliation), update `commit-analizado`, `fecha-analisis`, `rama-analizada`, and `ultima-auditoria` together. If inspection finds no necessary documentation change, use the accepted acknowledgement path in [vault-synchronization.md](vault-synchronization.md), preserving the note baseline; a metadata-only request uses that same reviewed closure.
5. Propagate contract changes to every node class required by [node-selection.md](../../../../90-Meta/node-selection.md), including relevant glossary and navigation nodes. Keep asynchronous topology routed through the topic.
6. Express absence using the required Spanish wording “no observado en fuentes estáticas revisadas”; use `#por-confirmar` only when the limitation affects business understanding.

For an operational note, use the closed `tipo: operacional` contract, keep unverified organization-specific content in `borrador`, and update `ultima-verificacion` only after checking the relevant platform or owner. Never turn one `.operations/` run into the normative procedure.

When the target is `schema-repository`, synthesize every represented database at business level: list each logical table or explicit empty schema, its domain/purpose, and its principal verified or functional relationships. Keep physical columns, data types, constraints, indexes, DDL, partitions, and exhaustive ER diagrams in the source repository. Reconcile this map and the backlinks from confirmed writer notes in the same change.

When another repository is confirmed to write to a represented database, link the configured schema-repository note in **Persistencia y datos** and name the covered database/schema. Do not add it to `consume-de` and do not add the link for read-only or unconfirmed relationships.

## Verification

Answer source-derived questions from the resulting note and verify preservation/removal decisions under [evidence-extraction.md](evidence-extraction.md). Run the Framework gates, inspect the complete diff, and confirm that unrelated work and traceability for repositories that were not re-analyzed remain unchanged.

Use [final-note-review.md](final-note-review.md) for independent review of the complete candidate, including external evidence and connection create/preserve/update/retire decisions. For an external-only update, report external verification separately from source freshness and keep the source commit and analysis dates unchanged. Apply [evidence-sufficiency.md](evidence-sufficiency.md): retain unmet in-scope questions and explicitly reclassify obsolete or out-of-scope conditions without inventing observed success.

## Completion criterion

For the external-only branch, completion is the accepted final-note review, exact reviewed images published, applicable vault gates passing and each selected pending item either answered by sufficient evidence, retained with its material in-scope question, or explicitly retired as obsolete/out of scope under evidence-sufficiency.md. Retiring a demand does not claim that its test passed; preserve connection anchors and supported evidence. The unchanged-source/no-write criterion below applies only to source-inventory work, not this branch.

For an unchanged repository, the unit is complete when the inventory proves that remote HEAD equals `commit-analizado`, the result is reported as `current`, and no file was modified. An accepted no-documentation-change inspection completes through the synchronization acknowledgement and receipt, with the note baseline retained and that outcome reported. For a scanned repository with a documentation update, completion requires evidence-backed identity and behavior at the declared evidence level, every written claim passing the profile-specific evidence policy, a contract-compliant note, resolved or explicitly limited main-flow connectors, an explicit decision for every affected node, traceability updated to the scanned branch and commit, and all applicable gates passing. For any other node, completion requires an evidence-backed subject and behavior at the declared evidence level, every written claim passing the same gate, a valid type and lifecycle action, canonical placement and schema, no duplicate node, explicit propagation decisions, and all applicable gates passing.
