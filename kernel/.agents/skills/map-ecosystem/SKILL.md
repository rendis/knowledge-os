---
name: map-ecosystem
description: "Trigger: query, document, synchronize, or check readiness of a cell knowledge vault. Orient first when bootstrap is incomplete."
license: Apache-2.0
metadata:
  author: documentation-vault maintainers
  version: "1.0"
---

## Activation Contract

Use this skill for a cell-vault query, documentation update, ecosystem synchronization, or readiness check. A `SYNC_PACKAGE_WORKER_V1` card selects the package-worker path only.

## Hard Rules

- Resolve the vault before reading relative vault paths; keep remote sources read-only.
- Write only with explicit vault-update authority and the cell evidence profile for every technical claim.
- A package worker writes only its assigned analysis or review artifact; the coordinator alone owns checkpoints, gate, projections, units, and vault writes.
- A synchronization package has one initial extraction/review and at most one finding-directed correction with a fresh review before checkpointing. Resume publication failures with the same `run_id` without repeating semantic work.
- Write-ready authority requires its independent review. Markdown units target only canonical knowledge roots; source drift retracts affected applied bytes before reseal.

## Mapping scope

For repository mapping and synchronization, [references/repository-map.md](references/repository-map.md) defines the default: purpose, main entrypoints → meaningful logic → connectors/effects, stack and evidence. Reuse valid existing knowledge. Accepted local maps expose stable connection claims; resolve selected external ends afterward with [references/connection-reconciliation.md](references/connection-reconciliation.md).

## Decision Gates

| Request or condition | Load / action |
| --- | --- |
| Complete `SYNC_PACKAGE_WORKER_V1` card | Load `references/synchronization-package-worker.md`; return one bounded artifact or contract error. |
| Full onboarding requested, or the selected task requires source access and roots/acquisition choice are missing | Use `configure-workspace` to collect unresolved decisions before source acquisition; return to the selected map branch afterward. |
| Incomplete bootstrap or where-to-start request | Load `references/orientation.md`. |
| Context, dependency, or impact question | Load `references/interrogation.md`. |
| One durable node | Load `references/single-unit-documentation.md`. |
| Several related units | Load `references/multi-unit-documentation.md`. |
| Inventory, lifecycle, or sync request | Load `references/vault-synchronization.md`, then `references/synchronization-state-machine.md` for a transition or failure. |
| An accepted local map has an unresolved external connection material to the task | Load `references/connection-reconciliation.md`; preserve the accepted local artifact and create separately reviewed external claims. |
| External reconciliation reaches a required provider, cluster or database with missing access | Finish available static reconciliation, run the capability-scoped `operational-readiness` check, hand the exact missing local binding to `configure-workspace`, then resume the same pending connection after readiness passes. |
| Tooling, source, or vault resolution issue | Load `references/operational-readiness.md`. |

## Execution Steps

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Bind the resolved vault and configured source roots; a failed resolution blocks root-dependent work.
2. Select one primary branch from the table. Load only its recipe and supporting references it explicitly requires.
3. For documentation or synchronization, load `references/evidence-extraction.md` for bounded main-flow scope, preservation and directed review. Before a source read or delegation, bind its checkout by configured remote identity. Before a technical write, load `../../../90-Meta/evidence-policy.md` and `../../../90-Meta/node-selection.md`.
4. After local-map acceptance, reconcile only external connections required by the task. Exhaust versioned evidence before requesting a missing live capability, so the request names the exact provider/project, cluster context or database target and read-only question. After configuration, rerun readiness and resume the pending connection without remapping the repository. Load `references/deployment-evidence.md` when that reconciliation or an explicit deployment audit needs current platform evidence. For synchronization, use sealed gate grants and the per-unit state contract; use `status` or `resume` rather than inferring a next step.
5. Report inspected evidence, changes, limitations, and observed checks. Stop when the selected recipe's completion criterion is met.

## Output Contract

Return the resolved scope, evidence used, decisions and written paths, limitations, and checks actually observed. A package worker returns only its assigned artifact or `worker-contract-invalid`.

## References

- `references/vault-synchronization.md` — coordinator recipe.
- `references/synchronization-state-machine.md` — durable run, unit, and recovery contract.
- `references/synchronization-package-worker.md` — bounded extractor/reviewer contract.
- `references/connection-reconciliation.md` — resolve selected external ends after local-map acceptance.
- `../../../90-Meta/vault-resolution.md` — vault and source binding.
- `../../../90-Meta/node-selection.md` — canonical node and lifecycle choice.
