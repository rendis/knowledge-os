---
name: synchronize-ecosystem
description: "Trigger: inventory, synchronize or resume a knowledge-map run, or execute a SYNC_PACKAGE_WORKER_V1 card. Queries and ordinary documentation belong to map-ecosystem."
license: Apache-2.0
metadata:
  author: documentation-vault maintainers
  version: "1.0"
---

## Activation Contract

Use this skill for vault inventory, lifecycle synchronization, resuming a recorded sync run, or a complete `SYNC_PACKAGE_WORKER_V1` card. Read-only questions, catalog maintenance, and ordinary note publication belong to `map-ecosystem`. Workspace configuration belongs to `configure-workspace`.

Shared mapping recipes live under `../map-ecosystem/references/`. Durable sync state remains `.agents/state/map-ecosystem/sync/`.

## Hard Rules

- Resolve the vault before reading relative vault paths; keep remote sources read-only.
- Write only with explicit vault-update authority and the cell evidence profile for every technical claim.
- A package worker writes only its assigned analysis or review artifact; the coordinator alone owns checkpoints, gate, projections, units, and vault writes.
- A synchronization package has one initial extraction/review and at most one finding-directed correction with a fresh review before checkpointing. Resume publication failures with the same `run_id` without repeating semantic work.
- Write-ready authority requires its independent review. Markdown units target only canonical knowledge roots; source drift retracts affected applied bytes before reseal.
- External reconciliation publishes through an ordinary documentation transaction and an independent final-note review; later observations never enter a closed Git synchronization package.

## Decision Gates

| Request or condition | Load / action |
| --- | --- |
| Complete `SYNC_PACKAGE_WORKER_V1` card | Load `../map-ecosystem/references/synchronization-package-worker.md`; return one bounded artifact or contract error. |
| Inventory, lifecycle, synchronization, or resume | Load `../map-ecosystem/references/vault-synchronization.md`, then `../map-ecosystem/references/synchronization-state-machine.md` for a transition or failure. |
| Query, catalog, or ordinary documentation | Use `map-ecosystem`. |

## Execution Steps

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Bind the resolved vault and configured source roots; a failed resolution blocks root-dependent work.
2. Select one primary branch from the table. Load only its recipe and supporting references it explicitly requires.
3. For coordinator synchronization, follow the vault-synchronization recipe: sealed gate grants, per-unit state, and `status` or `resume` rather than inferring a next step. Load `../map-ecosystem/references/evidence-extraction.md` before freezing inventory. Before a source read or delegation, bind its checkout by configured remote identity. Before a technical write, load `../../../90-Meta/evidence-policy.md` and `../../../90-Meta/node-selection.md`.
4. After local-package acceptance, reconcile only external connections required by the task through `../map-ecosystem/references/connection-reconciliation.md`, published with `map-ecosystem`'s single-unit or multi-unit recipe and `../map-ecosystem/references/final-note-review.md`, outside the immutable closed Git package.
5. Before declaring a sync campaign complete, follow `../map-ecosystem/references/mapping-completion.md`.
6. Report inspected evidence, changes, limitations, and observed checks. A package worker returns only its assigned artifact or `worker-contract-invalid`. Stop when the selected recipe's completion criterion is met.

## Output Contract

Return the resolved scope, evidence used, decisions and written paths, limitations, and checks actually observed. A package worker returns only its assigned artifact or `worker-contract-invalid`.

## References

- `../map-ecosystem/references/vault-synchronization.md` — coordinator recipe.
- `../map-ecosystem/references/synchronization-state-machine.md` — durable run, unit, and recovery contract.
- `../map-ecosystem/references/synchronization-package-worker.md` — bounded extractor/reviewer contract.
- `../../../90-Meta/vault-resolution.md` — vault and source binding.
