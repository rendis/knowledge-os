---
name: manage-investigation
description: Maintain persistent local case files. Use when a user wants to open or resume an investigation from a request or attachment, refine it with evidence and decisions, reconcile a development handoff after implementation, consolidate duplicate cases, validate readiness, export technical or user stories or repository-specific development packages, assess investigation-derived learning, or assess a post-deployment candidate for independent promotion to the cell vault.
---

# Manage investigations

Treat each investigation as a durable case file: keep its current understanding ready to consume and its chronology append-only.

## Preflight

1. Resolve the repository root and use `<root>/.investigations/` as the case-file root.
2. Before the first write, run `git check-ignore .investigations/` from the repository root and require output proving the path is ignored.
3. Run `git ls-files '.investigations/**'` and require empty output. Stop and report any tracked path.
4. Load [references/record-contract.md](references/record-contract.md) before creating or changing a case file.
5. Resolve `scripts/investigation-case.py` relative to this skill. Use it for Open, Consolidate, and mechanical validation; do not reproduce its locking, archive, or rollback logic manually.

Complete preflight only when the repository root is known, the local store is ignored, no investigation file is tracked, and the record contract is loaded.

## Route the request

- **Open**: create a case file from a message, bug, ticket, issue, or attachment.
- **Resume**: find a case file by exact `id`, then `consolidated-from`, source reference, title, or keywords. Present candidates only when several match.
- **Investigate**: gather evidence, refine the current understanding, resolve contradictions, and record decisions.
- **Reconcile development**: consume one normalized result from `reconcile-development-handoff` and update the exact source case.
- **Consolidate**: reconcile duplicate case files into one canonical directory while retaining retired IDs in its lineage.
- **Validate**: apply [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md).
- **Export**: load [references/export-contract.md](references/export-contract.md) and create one or more local story drafts.
- **Learn**: hand an exact case to `manage-investigation-derived-learning` for critical read-only assessment and optional authorized durable publication.
- **Promote**: assess a post-deployment documentation candidate and hand it to `map-ecosystem`; this skill never writes the vault.

Use no parallel index. Interrogation may call `90-Meta/graph-query.py investigations --node <stem>` (on-demand scan of `investigation.md` files). That query is not a second store. Resume inside this skill still searches case files directly.

## Open

1. Load [references/deduplication-and-consolidation.md](references/deduplication-and-consolidation.md), derive the immutable ID and stable `dedupe-key`, classify `purpose`, set the initial `vault-outcome` and `learning-outcome: not-evaluated`, then invoke the helper's `open` command.
2. Resolve semantic ambiguity before invoking the helper. Route `definite_match` to **Resume** and continue only from `created`.
3. Preserve the original request when safe, capture its source, and process attachments under the record contract.
4. Set `status: intake`. Resolve `purpose: undecided` before moving to `investigating`; `vault-outcome: not-evaluated` may remain while the investigation still lacks the evidence needed to assess a durable current-state candidate.

Complete when the case file has no equivalent active case, has a unique ID, original request, source, objective, scope, purpose, initial vault and learning outcomes, attachment decisions, and its creation event in History, and the open gate is released.

## Resume

1. Read the entire matching `investigation.md` before acting.
2. When `exports/` contains drafts, load [references/export-contract.md](references/export-contract.md) and inspect their source timestamps and register references.
3. Reconstruct the current state from frontmatter and **Current state**; use **History** only for provenance.
4. For a legacy case without `purpose`, `vault-outcome`, or `learning-outcome`, classify them before the next material change and record the classification in History.
5. Report the active status, purpose, vault outcome, learning outcome, current understanding, blockers, open questions, stale drafts, and next useful action.

Complete when one case file is selected, draft freshness is known, and the next action follows the current state without reviving superseded understanding.

## Investigate

1. Prefer repository evidence, supplied sources, and available domain procedures over recall. Separate facts, inferences, contradictions, and limitations.
2. Treat persistent memory, prior cases, and neighboring workspaces only as discovery leads. Before using their content, confirm the source is explicitly in scope, inspect it directly, and register the verified evidence. Never treat memory, path proximity, or an unrelated local project as evidence or authorization.
3. For cell ecosystem evidence, load `map-ecosystem`, select its read-only interrogation branch, and keep resolved source repositories read-only.
4. Maintain an explicit boundary between **current productive state** and **future/proposed state**. For `purpose: development` or `mixed`, represent both independently; decisions and acceptance criteria for future work never become facts.
5. Maintain `vault-outcome` as evidence changes. Development or undeployed behavior is `deferred-until-production`; a possible current-state fact is at most `candidate-for-audit` until `map-ecosystem` independently verifies it. In a mixed case, this field follows the current-state candidate when one exists, while the future portion remains explicitly deferred and outside the vault.
6. Resolve evident defaults directly. When a material decision remains ambiguous, load [references/questioning-protocol.md](references/questioning-protocol.md).
7. After every material finding or answer, update **Current state**, `updated-at`, the affected registers, and append one History event.
8. Preserve stable identifiers and replacement links; never renumber, recycle, or silently change the meaning of a registered item.
9. When existing drafts reference changed material, load [references/export-contract.md](references/export-contract.md) and reconcile or mark every affected draft stale in the same interaction. Record the outcome in History.
10. When new evidence, context, or a decision could change a completed learning assessment, preserve that assessment in History and reset `learning-outcome` to `not-evaluated`.

Complete the iteration when every new material fact, inference, contradiction, question, decision, and scope change is represented in both the current state and chronology, current and future states remain separated, `vault-outcome` is accurate, `learning-outcome` reflects the current evidence snapshot, and every affected draft has an explicit synchronization state.

## Reconcile development

1. Accept only one complete normalized in-memory context from `reconcile-development-handoff`; require the exact case, story, Jira, repository, family, and revision identity.
2. Load [references/development-reconciliation.md](references/development-reconciliation.md), read the entire current case, and reject any identity mismatch or missing comparison surface.
3. Apply the returned baseline deltas, changelog provenance, implementation/delivery/Jira evidence, and direct-dependent cards to Current state and the stable registers without re-reading or mutating the worktree.
4. Reconcile affected drafts, reset a stale learning assessment when required, append one material History event, and run the case validator. Preserve a byte-level no-op when nothing in the case changed.

Complete when the selected case alone represents the reconciled implementation and downstream context, its current/future boundary remains correct, every affected draft has an explicit synchronization state, structural validation passes, and no source repository, Jira, remote Git, pull request, deployment, or technical-vault write occurred.

## Consolidate

Load [references/deduplication-and-consolidation.md](references/deduplication-and-consolidation.md) and apply its semantic reconciliation protocol. Keep the oldest case as canonical unless the user selects another case or current evidence requires a different target. After producing the required mapping artifact, invoke the helper's `consolidate` command for the transactional archive, lineage update, exact removal, validation, and rollback.

Complete when exactly one case directory remains for the equivalence group, its current state contains the reconciled material, `learning-outcome` was preserved only against an unchanged assessment basis or reset otherwise, every source identifier remains traceable, drafts have an explicit synchronization state, and searching a retired ID in `consolidated-from` resolves to the canonical case.

## Validate

Run the helper's `validate` command for structural and transactional invariants, then apply the lifecycle gate from [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md). A missing `dedupe-key` in a legacy case is a warning, not an instruction to rewrite it. Move backward when new evidence invalidates readiness. When a required source is unreadable, report the observable failure, set `status: blocked`, and wait for the user to decide how to proceed.

Complete when the status is supported by its gate, or `blocked` names the dependency, prior state, and user action required.

## Export

1. Generate platform-neutral local drafts in `exports/` from [assets/story-template.md](assets/story-template.md).
2. Match story kind and detail to the audience and purpose; split only independently deliverable outcomes.
3. Stamp each draft with the current investigation `updated-at`, verify every referenced register item, and set its synchronization state from the export contract.
4. Before `ready-to-export`, label drafts provisional and keep the lifecycle state unchanged; use them only to refine the investigation.
5. When the user requests external publication of a release-ready draft, build the publication package defined by the export contract and hand it to `manage-operational-workflow`.
6. Keep connector selection, external-effect authorization, publication, and read-back verification inside that operational workflow.
7. After the handoff returns a verified result, update the local draft and History with the non-sensitive external reference and publication status.
8. When the user requests development handoff for a release-ready story with a current Jira snapshot, build one repository-specific package per target from the development section of the export contract.
9. Hand only the exact package directories to `manage-development-handoff`; that skill owns target resolution, preview, authorization, materialization, and validation and must not read or change this case. When Export resolves its `producer-required` intake state, return the directories to the initiating workflow so it resumes without asking the user to submit the handoff again.

Complete when each draft is current and traces to the case file and evidence, every requested external publication has either been handed off with an exact current package or returned with a verified reference recorded locally, and every requested development handoff has one current package per target repository or an explicit source blocker.

## Learn

1. Require one selected case and read its entire current snapshot before the handoff.
2. Hand the exact investigation path and requested mode to `manage-investigation-derived-learning`. Use **Assess** unless the user explicitly requested durable publication or revalidation.
3. Let that skill inspect sources, search existing learnings, apply its independent gate, and present one of `extractable`, `no-learning`, `already-covered`, or `insufficient-evidence`. Do not pre-classify the answer or treat case status as proof.
4. Keep the dependency one-way during assessment: the learning skill reads but never mutates the case. After it returns, map `no-learning`, `already-covered`, and `insufficient-evidence` directly; map an unpublished `extractable` result to `candidate`; map a verified durable write to `documented`.
5. When persisting the returned status, update `learning-outcome`, `updated-at`, Readiness, and History together with the assessed case snapshot, canonical target if any, evidence boundary, lifecycle action, and observed checks. A blind test or explicit assess-only request that forbids persistence ends before this step.
6. Do not change `vault-outcome`: technical production documentation and investigation-derived learning are independent paths.

Complete when the user has the explicit assessment result and action, no forbidden test or assess-only state was persisted, and any authorized case update accurately records the returned snapshot without replacing its evidence.

## Promote

1. Require an explicit candidate claim and a classified `purpose`; a development case can reach this branch only after the change is reported as deployed.
2. Separate the candidate's observed current state from every original proposal, decision, acceptance criterion, and story. Set `vault-outcome: deferred-until-production` and stop when deployment remains pending or ambiguous.
3. When productive implementation is plausible, set `vault-outcome: candidate-for-audit` and build a context package containing the exact candidate claims, in-scope source references, likely canonical nodes, and known limitations. Do not present the case file, Jira, or an approval as evidence.
4. Hand the package to `map-ecosystem` with the user's vault-write authorization as an explicit post-deployment production audit, including when the repository HEAD is unchanged since its last source analysis. That skill re-inspects authoritative code, deploy, infrastructure, database, and runtime evidence as applicable and owns the production-evidence gate, node lifecycle, write, propagation, and verification.
5. Set `vault-outcome: documented` only after `map-ecosystem` reports that its independent production audit passed and the canonical vault either already represented the fact correctly or was updated and verified. Record canonical notes, evidence boundary, lifecycle result, and observed checks in Readiness and History. Otherwise keep `candidate-for-audit` with the exact missing productive evidence for a purported current state, set `none` when the audit disproves or removes the durable candidate, or return to `deferred-until-production` while deployment remains pending or ambiguous.

Complete when every candidate has an explicit outcome, no future-state claim entered the vault, and any documented claim traces to an independent `map-ecosystem` production audit.

## Guardrails

- Keep `.investigations/` local and ignored.
- Preserve source language and use the user's working language for generated records and drafts.
- Persist secret existence, location, and behavior only with values redacted. Stop before copying sensitive values into the case file.
- Treat the investigation as context and provenance, never as evidence that a behavior exists in production.
- Route publication, ticket creation, and other external writes through `manage-operational-workflow`; keep commits, pushes, and pull requests behind their own explicit authorization.
