# Process one frozen synchronization package

## Activation

Use this mode only when the parent supplies one complete JSON card with this shape:

```json
{
  "marker": "SYNC_PACKAGE_WORKER_V1",
  "role": "extractor | reviewer",
  "evidence_profile": "production-gate | documented-source | mixed",
  "run_root": "<absolute package work path>",
  "repository": "<full repository name>",
  "source_checkout": "<absolute read-only checkout>",
  "production_ref": "<explicit local or remote ref>",
  "old_oid": "<full commit oid>",
  "new_oid": "<full commit oid>",
  "manifest": "<absolute path>",
  "scaffold": "<absolute path>",
  "analysis": "<absolute path>",
  "review": "<absolute path or null>",
  "check_result": "<absolute path or null>",
  "vault_context": ["<absolute note path>"],
  "search_roots": ["<absolute read-only root>"],
  "allowed_write": "<absolute path>"
}
```

A correction dispatch may additionally supply `correction` with exact `receipt`, `initial_analysis`, and `initial_review` paths below `run_root`. Read them before acting; they identify the single reserved attempt and its findings. These inputs are read-only. An extractor changes only findings and connected dependencies in the seeded analysis, preserving unrelated accurate claims. A reviewer checks the repaired artifact and regressions without assuming acceptance. The coordinator validates the correction scope before closure.

Validate `evidence_profile` against the three values in `90-Meta/evidence-policy.md` and apply that policy when extracting or reviewing claims. For old cards only, an omitted profile means `production-gate`; an unknown profile is invalid.

Require every path except `source_checkout`, `vault_context`, and `search_roots` to be below `run_root`. `run_root` is a package work area, never the durable run checkpoint. Require `allowed_write` to equal `analysis` for an extractor and `review` for a reviewer. Return `worker-contract-invalid` to the parent when a field is missing or inconsistent; do not start a top-level workflow.

Load [evidence-extraction.md](evidence-extraction.md) for scope, adaptive probes, preservation and semantic review. Apply only your assigned role; the coordinator owns any follow-up dispatch.

## Shared envelope

Use the coordinator bindings as the package authority. Start in `run_root`. Read source blobs only from `old_oid` and `new_oid` in `source_checkout`; never substitute working-tree bytes. Read only the supplied package artifacts, exact `vault_context`, and targeted identifier matches under `search_roots`, and write exactly `allowed_write`. Leave every source checkout and vault file unchanged.

Remote freshness, vault resolution, repository identity, Obsidian binding, inventory, seed-node discovery, checkpointing, gate sealing, projection, application, and closure belong to the coordinator. Persistent memory is not package evidence and is not queried for package context. The package worker loads no other ecosystem branch, performs no fetch or checkout, and launches no helper workflow. Do not delegate.

Inspect credential-suspect paths only through safe non-secret context. Never quote or persist a detected value. Treat an unavailable external deployment body as `not-observed` for dependent fields rather than expanding the assignment.

Apply evidence to the scope of each assertion: versioned production configuration proves what is configured at the analyzed commit; observed deployment evidence is required to assert what is deployed or effective. For example, an inspected production overlay can support “the production configuration sets the inventory URL to X” while the external deployment body remains unavailable. Retain that configuration claim and state the deployment limitation separately. A branch name alone proves neither productive applicability nor deployment. Inspect production and non-production configuration separately and keep each environment explicit.

## Extractor

Start with supplied existing knowledge and the frozen delta. Use repository-map.md to trace affected main flows and connectors at the exact OIDs; for an initial map, trace the main flows directly. Represent every externally significant connector as an atomic claim whose `claim_id` starts with `connection.` and whose statement contains the required local connection record. Reuse the claim ID for an unchanged connector slot during sync. Reopen evidence for changed meaning, missing anchors, contradictions or unresolved main-flow questions, including unchanged callers/configuration dependencies when relevant. Preserve supported unchanged knowledge and provenance. Manifest paths are bookkeeping, not instructions to read every file: account for them from inventory/targeted evidence, mark secondary detail outside this map scope explicitly, and never label uninspected behavior absent. Treat `analysis` as a semantic candidate over the immutable scaffold; the coordinator's deterministic finalizer owns canonical identity, ordering, record shape, status derivation, redaction, and result reconciliation.

Every `{path, anchor}` must resolve inside `source_checkout` at a frozen package OID. Sibling search roots can establish dependent impact, but their files/OIDs are not package evidence paths. Cite an inspected in-repository contract when it supports the assertion; otherwise keep the dependent claim limited for coordinator reconciliation. Never relabel sibling evidence as a local path.

Fill these closed semantic values:

- Every scaffold path gets one `disposition`: `relevant`, `not-documentable`, or `blocked`, plus a non-empty reason and claim references. Only `relevant` paths reference claims.
- Every existing checklist question gets one answer: `observed`, `not-observed`, `not-applicable`, or `blocked`, plus a non-empty dimension reason and exact `{path, anchor}` evidence. Dimension status is `checked`, `not-applicable`, or `blocked`; it is not a question answer. Evidence paths are relative to the owning repository, never absolute sibling paths. Preserve the environment evidence entries seeded by the scaffold, explaining any unresolved fields without exposing credentials. Do not add or remove questions.
- Claims use only `claim_id`, `statement`, and `evidence`. Each claim is one atomic, independently reviewable durable assertion: removing it must not change the meaning of another claim. Keep a claim only when exact evidence meets the frozen profile (versioned source for `documented-source`; the source-map rule or productive applicability for stronger production assertions otherwise); describe contradictory configuration values explicitly instead of collapsing them into one inferred effective value. Distinguish the authority or identity system for a business identifier from an operational replica, cache, event history, or state store; never collapse them into one unqualified location statement. Remove the unused scaffold prototype.
- Nodes use only `basename`, `action`, `reason`, and `claim_ids`. Include every scaffold seed and every affected canonical node once; consider unchanged consumers and preserve unrelated established facts. Justify corrections or removals from evidence, never from omission alone. Actions are `create`, `update`, `consolidate`, `retire`, or `no-change`.
- Use `documentation-change` when qualifying claims exist, `traceability-only` for a changed repository whose scoped delta was inspected and needs no durable update, `no-change` for a new repository whose bounded role/contract inspection establishes no durable node, and `blocked` only when inspection itself could not complete. A no-claim result must explain why each observed change requires no documentation update; unavailable deployment evidence alone does not discard independently supported configuration facts.

Write one candidate per assigned attempt. Missing semantic decisions remain invalid and become deterministic fallback. Structural drift alone is normalized from the scaffold without another agent pass.

The extractor writes only `analysis`. It does not finalize, check, review, checkpoint, reconcile the batch, query Obsidian, project, apply, or write documentation. Completion is one closed `analysis` artifact or one `worker-contract-invalid` response.

## Reviewer

Use a fresh context and no expected verdict. Follow the directed review in repository-map.md. Read the successful `check_result` and candidate, check exact evidence for its main entrypoints, `connection.*` claims and rules, and search registration/composition narrowly for omitted main flows. For sync, compare changed and removed source behavior with the final claims and reject a stale value or assertion contradicted by `new_oid`, even when another claim correctly describes the delta. Deterministic structure/digest/path coverage checks stay coordinator-owned. Review existing accurate facts for preservation; keep unresolved peer relations and secondary details as limitations. Do not require every environment value, utility behavior or full deployment chain to accept a useful local map. Reject unsupported statements according to the source-map evidence rule, not because code behavior lacks proof of deployment.

An omission warrants `revise` when it hides a main flow or materially changes the described behavior. Use a claim target for an isolated unsupported statement; a whole-result target is reserved for defects preventing a safe useful subset. Missing optional detail does not require a correction. A final rejected package remains rejected/partial, not current documentation.

Write exactly this review version 3 shape. Copy the repository and three digests from `check_result`; never derive or rename them. `findings` is an array, but each finding's `evidence` is one object, never an array:

```json
{
  "version": 3,
  "repository": "<full repository name>",
  "manifest_digest": "<check_result manifest_digest>",
  "scaffold_digest": "<check_result scaffold_digest>",
  "analysis_digest": "<check_result analysis_digest>",
  "verdict": "accept | revise | blocked",
  "findings": [
    {
      "target": "<exact analysis target>",
      "category": "<finding category>",
      "reason": "<non-empty reason>",
      "nodes": ["<sorted canonical basename>"],
      "evidence": {"path": "<manifest path>", "anchor": "<non-empty anchor>"}
    }
  ]
}
```

Use an empty `findings` array only with `accept`. Use at least one finding with `revise` or `blocked`; every finding has exactly the five fields shown, a non-empty unique sorted `nodes` array, and one exact evidence object. A defect confined to one claim targets exactly `claims.<claim_id>`, with one finding per invalid claim. Use a non-claim target only when the defect prevents a safe claim subset from being accepted. The reviewer writes only `review`. It does not repair analysis, repeat extraction, broaden scope, checkpoint, run a gate, project, apply, or write documentation. Completion is one closed `review` artifact or one `worker-contract-invalid` response.
