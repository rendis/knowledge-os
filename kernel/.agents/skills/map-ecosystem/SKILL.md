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
- Write only with explicit vault-update authority and production evidence for every technical claim.
- A package worker writes only its assigned analysis or review artifact; the coordinator alone owns checkpoints, gate, projections, units, and vault writes.
- A synchronization run keeps one extraction and at most one semantic review per repository/OID pair. Resume recoverable work with its emitted `run_id`.
- Write-ready authority requires its independent review. Markdown units target only canonical knowledge roots; source drift retracts affected applied bytes before reseal.

## Decision Gates

| Request or condition | Load / action |
| --- | --- |
| Complete `SYNC_PACKAGE_WORKER_V1` card | Load `references/synchronization-package-worker.md`; return one bounded artifact or contract error. |
| Incomplete bootstrap or where-to-start request | Load `references/orientation.md`. |
| Context, dependency, or impact question | Load `references/interrogation.md`. |
| One durable node | Load `references/single-unit-documentation.md`. |
| Several related units | Load `references/multi-unit-documentation.md`. |
| Inventory, lifecycle, or sync request | Load `references/vault-synchronization.md`, then `references/synchronization-state-machine.md` for a transition or failure. |
| Tooling, source, or vault resolution issue | Load `references/operational-readiness.md`. |

## Execution Steps

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Bind the resolved vault and configured source roots; a failed resolution blocks root-dependent work.
2. Select one primary branch from the table. Load only its recipe and supporting references it explicitly requires.
3. Before a source read or delegation, bind its checkout by configured remote identity. Before a technical write, apply the production-evidence gate and load `../../../90-Meta/node-selection.md`.
4. For deployable repositories, load `references/deployment-evidence.md`. For synchronization, use sealed gate grants and the per-unit state contract; use `status` or `resume` rather than inferring a next step.
5. Report inspected evidence, changes, limitations, and observed checks. Stop when the selected recipe's completion criterion is met.

## Output Contract

Return the resolved scope, evidence used, decisions and written paths, limitations, and checks actually observed. A package worker returns only its assigned artifact or `worker-contract-invalid`.

## References

- `references/vault-synchronization.md` — coordinator recipe.
- `references/synchronization-state-machine.md` — durable run, unit, and recovery contract.
- `references/synchronization-package-worker.md` — bounded extractor/reviewer contract.
- `../../../90-Meta/vault-resolution.md` — vault and source binding.
- `../../../90-Meta/node-selection.md` — canonical node and lifecycle choice.
