# Process one frozen synchronization package

## Activation

Use this mode only when the parent supplies one complete JSON card with this shape:

```json
{
  "marker": "SYNC_PACKAGE_WORKER_V1",
  "role": "extractor | reviewer",
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

Require every path except `source_checkout`, `vault_context`, and `search_roots` to be below `run_root`. `run_root` is a package work area, never the durable run checkpoint. Require `allowed_write` to equal `analysis` for an extractor and `review` for a reviewer. Return `worker-contract-invalid` to the parent when a field is missing or inconsistent; do not start a top-level workflow.

## Shared envelope

Use the coordinator bindings as the package authority. Start in `run_root`. Read source blobs only from `old_oid` and `new_oid` in `source_checkout`; never substitute working-tree bytes. Read only the supplied package artifacts, exact `vault_context`, and targeted identifier matches under `search_roots`, and write exactly `allowed_write`. Leave every source checkout and vault file unchanged.

Remote freshness, vault resolution, repository identity, Obsidian binding, inventory, seed-node discovery, checkpointing, gate sealing, projection, application, and closure belong to the coordinator. Persistent memory is not package evidence and is not queried for package context. The package worker loads no other ecosystem branch, performs no fetch or checkout, and launches no helper workflow. Do not delegate.

Inspect credential-suspect paths only through safe non-secret context. Never quote or persist a detected value. Treat an unavailable external deployment body as `not-observed` for dependent fields rather than expanding the assignment.

## Extractor

Inspect every manifest path once at the exact OIDs. Treat `analysis` as a semantic candidate over the immutable scaffold; the coordinator's deterministic finalizer owns canonical identity, ordering, record shape, status derivation, redaction, and result reconciliation.

Fill these closed semantic values:

- Every scaffold path gets one `disposition`: `relevant`, `not-documentable`, or `blocked`, plus a non-empty reason and claim references. Only `relevant` paths reference claims.
- Every existing checklist question gets one answer: `observed`, `not-observed`, `not-applicable`, or `blocked`, plus a non-empty dimension reason and exact `{path, anchor}` evidence. Do not add or remove questions.
- Claims use only `claim_id`, `statement`, and `evidence`. Each claim is one atomic, independently reviewable durable assertion: removing it must not change the meaning of another claim. Keep a claim only when exact evidence proves both implementation and productive applicability; describe contradictory configuration values explicitly instead of collapsing them into one inferred effective value. Distinguish the authority or identity system for a business identifier from an operational replica, cache, event history, or state store; never collapse them into one unqualified location statement. Remove the unused scaffold prototype.
- Nodes use only `basename`, `action`, `reason`, and `claim_ids`. Include every scaffold seed and every discovered canonical node once. Actions are `create`, `update`, `consolidate`, `retire`, or `no-change`.
- Use `documentation-change` when qualifying claims exist, `traceability-only` for a fully inspected changed repository without claims, `no-change` for a fully inspected new repository without a durable node, and `blocked` only when inspection itself could not complete.

Write the candidate once. Missing semantic decisions remain invalid and become deterministic fallback. Structural drift alone is normalized from the scaffold without another agent pass.

The extractor writes only `analysis`. It does not finalize, check, review, checkpoint, reconcile the batch, query Obsidian, project, apply, or write documentation. Completion is one closed `analysis` artifact or one `worker-contract-invalid` response.

## Reviewer

Use a fresh context and no expected verdict. Read the successful `check_result`, manifest, scaffold, and analysis. Reinspect the exact evidence needed to verify path coverage, every claim, every environment config, productive applicability, and node completeness. Inspect a `not-documentable` path only when its manifest identity or content kind plausibly contradicts that disposition; deterministic structure and digest checks remain coordinator-owned.

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
