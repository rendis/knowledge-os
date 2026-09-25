---
name: map-ecosystem
description: "Trigger: query, document, related-vault catalog, or mapping-completion status. Orient first when bootstrap is incomplete. Inventory and synchronization belong to synchronize-ecosystem."
license: Apache-2.0
metadata:
  author: documentation-vault maintainers
  version: "1.0"
---

## Activation Contract

Use this skill for a cell-vault query, documentation update, related-vault catalog maintenance, or a pending-work / coverage question. Workspace configuration belongs to `onboard-developer`; Git policy belongs to `manage-git-workflow`. Inventory and lifecycle synchronization belong to `synchronize-ecosystem`.

## Hard Rules

- Resolve the vault before reading relative vault paths; keep remote sources read-only.
- Write only with explicit vault-update authority and the cell evidence profile for every technical claim.
- Load only the selected branch and the references it names.

## Mapping scope

When mapping discovers a related domain vault, use [references/vault-catalog.md](references/vault-catalog.md) to validate and propose its entry. Keep catalog registration separate from ordinary map publication authority.

For repository mapping, [references/repository-map.md](references/repository-map.md) defines the default: purpose, main entrypoints → meaningful logic → connectors/effects, stack and evidence. Reuse valid existing knowledge. Accepted local maps expose stable connection claims; resolve selected external ends afterward with [references/connection-reconciliation.md](references/connection-reconciliation.md).

## Decision Gates

Choose the next action from the existing knowledge, not from the number of pending items:

- Answer from the vault and its cited evidence first. A pending-work or cost question is read-only; it does not start extraction or reconciliation.
- Before another source read, identify the unanswered question and the missing evidence. Inspect the relevant file, dependency or configuration. A targeted symbol/resource search across configured repositories is appropriate when the owner is unknown after consulting the vault; it is not authority to extract those repositories again.
- Reuse accepted maps. Changed source calls for delta analysis; an identified defect calls for review of the affected scope. Pending connections, new sessions and changed instructions alone do not justify remapping.
- Before creating or reusing pending items, apply [references/evidence-sufficiency.md](references/evidence-sufficiency.md). Close material in-scope questions with sufficient evidence; explicitly retire obsolete demands without claiming they were tested. Mapping a relationship does not require observed traffic, delivery or business rows. Preserve the distinction between configured, deployed and functionally verified claims.
- Estimate effort only from a measured relevant sample, stating its scope and uncertainty. Without one, report that cost and the deduplicated count are not yet measured; offer a bounded sample instead of invented ranges.

| Request or condition | Load / action |
| --- | --- |
| Full onboarding requested, or the selected task requires source access and roots/acquisition choice are missing | Use `onboard-developer` (this machine's roots and clone choice; `onboard-cell` for the cell's sources) to collect unresolved decisions before source acquisition; return to the selected map branch afterward. |
| Incomplete bootstrap or where-to-start request | Load `references/orientation.md`. |
| Which service publishes/consumes a topic, event, table or endpoint | Read the discovery facts (`<cli> discover report --vault "<vault>" --repo <name>`, or run `discover run` when absent) and the platform wiring they cite; open the owning notes for behavior. |
| Context, dependency, or impact question | Load `references/interrogation.md`. |
| A related vault is discovered in authorized repository exploration or supplied by the user | Load `references/cross-vault-consultation.md` before consulting it; return to the active branch. |
| Register, update, list or remove related vaults; or a mapping discovers a candidate to register | Load `references/vault-catalog.md`; discovery requires confirmation before registration. |
| One durable node | Load `references/single-unit-documentation.md`. |
| Several related units | Load `references/multi-unit-documentation.md`. |
| Pending-work or campaign-completion question | Load `references/mapping-completion.md`; keep the query read-only. |
| Inventory, lifecycle, synchronization, or resume of a sync run | Use `synchronize-ecosystem`. |
| An accepted local map has an unresolved external connection material to the task | Load `references/connection-reconciliation.md`; preserve the accepted local artifact and create separately reviewed external claims. |
| External reconciliation reaches a required provider, cluster or database | Finish available static reconciliation, resolve its configured executor or adapter, and require a bounded read-only probe against the exact target. Use `onboard-cell` only for a missing procedure binding; ask for the exact missing access setup reported by a bound procedure, then resume the same pending connection after its probe passes. |
| Tooling, source, or vault resolution issue | Load `references/operational-readiness.md`. |

## Execution Steps

1. Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault), reusing the session's binding. Bind the resolved vault and configured source roots; a failed resolution blocks root-dependent work.
2. Select one primary branch from the table. Load only its recipe and supporting references it explicitly requires.
3. For documentation, load `references/repository-map.md`: start from the discovery facts, write with the checkable evidence format and pass `discover check` before review. Before a source read or delegation, bind its checkout by configured remote identity. Before a technical write, load `../../../90-Meta/evidence-policy.md` and `../../../90-Meta/node-selection.md`.
4. After local-map acceptance, reconcile only external connections required by the task. Exhaust versioned evidence before requesting a missing live capability, so the request names the exact provider/project, cluster context or database target and read-only question. A discovered connection grants no access: use the configured executor or adapter to prove bounded read-only access against that target with existing authentication. Use workspace configuration only to bind a missing procedure; after missing setup becomes available, rerun the same probe and resume the pending connection without remapping the repository. Load `references/deployment-evidence.md` when that reconciliation or an explicit deployment audit needs current platform evidence. Publish reconciliation results through the ordinary single-unit or multi-unit recipe and `references/final-note-review.md`.
5. Before declaring a system map complete, follow `references/mapping-completion.md`: reconcile the scoped cross-repository questions and align current visible coverage with publication outcomes.
6. Report inspected evidence, changes, limitations, and observed checks. Stop when the selected recipe's completion criterion is met.

## Output Contract

Return the resolved scope, evidence used, decisions and written paths, limitations, and checks actually observed.

## References

- `references/mapping-completion.md` — scoped reconciliation, visible coverage alignment and pending-work reporting.
- `references/connection-reconciliation.md` — resolve selected external ends after local-map acceptance.
- `references/final-note-review.md` — independently bind complete resulting note bytes to evidence before ordinary documentation writes.
- `../../../90-Meta/vault-resolution.md` — vault and source binding.
- `../../../90-Meta/node-selection.md` — canonical node and lifecycle choice.

## Native runtime

Bind `<cli>` through [use-vault-cli](../use-vault-cli/SKILL.md) before resolution; that shared reference defines platform selection and invocation. Use `--vault "<VAULT_ROOT>"` on vault operations and the explicit roots required by synchronization.
