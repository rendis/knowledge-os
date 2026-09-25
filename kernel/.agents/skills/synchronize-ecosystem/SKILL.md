---
name: synchronize-ecosystem
description: "Trigger: inventory, synchronize or resume a knowledge-map run. Queries and ordinary documentation belong to map-ecosystem."
license: Apache-2.0
metadata:
  author: documentation-vault maintainers
  version: "1.0"
---

## Activation Contract

Use this skill for vault inventory, lifecycle synchronization, or resuming a recorded sync run. Read-only questions, catalog maintenance, and ordinary note publication belong to `map-ecosystem`. Workspace configuration belongs to `configure-workspace`.

Shared mapping recipes live under `../map-ecosystem/references/`. Durable sync state remains `.agents/state/map-ecosystem/sync/`. Bind `<cli>` through [use-vault-cli](../use-vault-cli/SKILL.md).

## Hard Rules

- Resolve the vault before reading relative vault paths; keep remote sources read-only.
- Write only with explicit vault-update authority and the cell evidence profile for every technical claim.
- For each repository delta, one author prepares the complete resulting note images and one fresh reviewer checks their changed meaning against the baseline and frozen evidence.
- Correct a semantic finding only in the affected candidate text and re-review the materially changed meaning. Deterministic formatting, reference, pattern, integrity, or redaction repairs return to CLI checks without source extraction.
- Publish only exact reviewed images below canonical knowledge roots. Resume publication failure from its recorded state without repeating analysis or review.
- External reconciliation uses the same complete-candidate review and publication path while preserving source-analysis metadata when the source was not re-analyzed.

## Decision Gates

| Request or condition | Load / action |
| --- | --- |
| Inventory, lifecycle, synchronization, or resume | Load `../map-ecosystem/references/vault-synchronization.md`, then `../map-ecosystem/references/synchronization-state-machine.md` for a transition or failure. |
| Query, catalog, or ordinary documentation | Use `map-ecosystem`. |

## Execution Steps

1. Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault), reusing the session's binding. Bind the resolved vault and configured source roots; a failed resolution blocks root-dependent work.
2. Select one primary branch from the table. Load only its recipe and supporting references it explicitly requires.
3. For coordinator synchronization, follow the vault-synchronization recipe: one complete candidate, one independent semantic review, deterministic checks and resumable exact publication. Load `../map-ecosystem/references/evidence-extraction.md` before freezing inventory. Before a source read or delegation, bind its checkout by configured remote identity. Before a technical write, load `../../../90-Meta/evidence-policy.md` and `../../../90-Meta/node-selection.md`.
4. After the source-backed candidate is accepted and published, reconcile only external connections required by the task through `../map-ecosystem/references/connection-reconciliation.md`. Publish that separately observed evidence with `map-ecosystem`'s single-unit or multi-unit recipe and the same final-note review, while preserving the repository analysis metadata.
5. Before declaring a sync campaign complete, follow `../map-ecosystem/references/mapping-completion.md`.
6. Report inspected evidence, changes, limitations, and observed checks. Stop when the selected recipe's completion criterion is met.

## Output Contract

Return the resolved scope, evidence used, decisions and written paths, limitations, and checks actually observed.

## References

- `../map-ecosystem/references/vault-synchronization.md` — coordinator recipe.
- `../map-ecosystem/references/synchronization-state-machine.md` — durable run, unit, and recovery contract.
- `../map-ecosystem/references/synchronization-package-worker.md` — legacy package-worker contract, loaded only to resume a recorded old run that names it.
- `../../../90-Meta/vault-resolution.md` — vault and source binding.
