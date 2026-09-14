---
name: manage-investigation
description: Maintain shareable investigation case files with an optional private overlay. Use when a user wants to open, resume, migrate, reconcile, merge, validate, export, learn from, or promote an investigation.
---

# Manage investigations

Treat `investigations/<id>/investigation.md` as the canonical, versionable case. An optional `.investigations-private/<id>/private.md` may add necessary sensitive context but never overrides public status, evidence, decisions, acceptance criteria, or history.

Own documentary persistence and traceability, not the general inquiry method. Answering a question does not require a case. Open only when the user requests one or accepts a recommendation; recommend one when continuity or collaboration would benefit from retained evidence, decisions, or pending work. For already-supported updates, proceed directly to the documentary checks. Consult [evidence-driven-analysis](../evidence-driven-analysis/SKILL.md) only when evidence still needs to be obtained or evaluated; retain this workflow as owner and consume its result without routing back.

## Preflight

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Bind one canonical `VAULT_ROOT`; use `VAULT_ROOT/investigations/` as the public root.
2. Require `investigations/` to be eligible for tracking. Require both `.investigations-private/` and the legacy `.investigations/` to be ignored and absent from `git ls-files` when they exist.
3. Load [references/record-contract.md](references/record-contract.md) before creating or changing a case file.
4. Resolve `scripts/investigation-case.py` relative to this skill. Use it for Open, Load, Save, Transition, Consolidate, Bind, Close, and validation; do not reproduce its discovery, locking, attribution, or rollback logic manually.
5. Before a mutating route, confirm that the vault checkout has effective Git `user.name` and `user.email`. The helper enforces this gate and never changes Git configuration.

Complete preflight when the public root is trackable, private and legacy roots are ignored, and the record contract is loaded.

## Route the request

- **Open**: create a case file from a message, bug, ticket, issue, or attachment.
- **Resume**: find a case file by exact `id`, then `consolidated-from`, source reference, title, or keywords. Present candidates only when several match.
- **Migrate**: when explicitly requested, load [references/migration.md](references/migration.md); convert prior public lifecycle states and transform selected ignored legacy cases without changing their sources.
- **Reconcile collaboration**: before accepting concurrent contributions, load [references/investigation-reconciliation.md](references/investigation-reconciliation.md).
- **Update case**: persist supported findings, decisions, questions, and changes to the current understanding; obtain missing analysis through the shared method only when needed.
- **Bind development handoff**: own the case binding decision for one validated materialization or activation result; mutate only for a new or advanced revision.
- **Reconcile development**: consume one normalized result from `reconcile-development-handoff` and update the exact source case.
- **Consolidate**: reconcile duplicate case files into one canonical directory while retaining retired IDs in its lineage.
- **Validate**: assess investigation readiness or, for an exact development package, read-only implementation sufficiency.
- **Transition**: block, unblock, or explicitly reopen through the current-snapshot transition command and [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md).
- **Close**: apply the objective-based closure gate in [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md); record the outcome, reason, evidence, and outstanding limitations. Output and implementation are required only when the objective requires them.
- **Export**: load [references/export-contract.md](references/export-contract.md) and create one or more local story drafts.
- **Learn**: hand an exact case to `manage-investigation-derived-learning` for critical read-only assessment and optional authorized durable publication.
- **Promote**: assess a documentation candidate under the cell evidence profile and hand eligible claims to `map-ecosystem`; this skill never writes the vault.

Use no parallel index. Interrogation may call `90-Meta/graph-query.py investigations --node <stem>` (on-demand scan of `investigation.md` files). That query is not a second store. Resume inside this skill still searches case files directly.

## Open

1. Load [references/deduplication-and-consolidation.md](references/deduplication-and-consolidation.md), derive the immutable ID and stable `dedupe-key`, classify `purpose`, set the initial `vault-outcome` and `learning-outcome: not-evaluated`, then invoke the helper's `open` command.
2. Resolve semantic ambiguity before invoking the helper. Route `definite_match` to **Resume** and continue only from `created`.
3. Formalize only the relevant request. Do not preserve the conversation transcript or literal informal wording. Capture a durable source reference when available and process attachments under the record contract.
4. Set `status: investigating`. `purpose: undecided` may remain while the requested outcome is being classified, but resolve it before completing the case or producing an external or development output. `vault-outcome: not-evaluated` may remain while the investigation still lacks the evidence needed to assess a durable current-state candidate.

Complete when the case has no equivalent active case, has a unique ID, professional request summary, source, objective, scope, purpose, initial outcomes, attachment decisions, and creation history.

## Resume

1. Search `investigations/` first. If a public match uses a prior lifecycle state or obsolete lifecycle metadata, require **Migrate** before ordinary mutation. If only `.investigations/` contains the match, read it as legacy and require **Migrate** before any mutation.
2. Invoke `load --id <id>` after selecting the case. Read the entire returned public path and, when `private.available` is true, the returned private path. This lookup is mandatory even when the user does not mention private context. Label private provenance and keep it supplementary and non-authoritative. When it is absent, state that the requested private fact is unavailable instead of inferring it; the public case must remain intelligible.
3. When `exports/` contains drafts, load [references/export-contract.md](references/export-contract.md) and inspect their source timestamps and register references.
4. Reconstruct authoritative state from the public frontmatter and **Current state**; use public **History** only for provenance.
5. Report the status, closure outcome when closed, purpose, independent outcomes, current understanding, blockers, open questions, handoffs, stale drafts, private-overlay availability, and next useful action.

For a question about an existing case, use the same load and overlay discovery, then answer only the requested scope without writes, lifecycle transitions, or a mandatory full status report. Read-only lookup does not require Git author identity or the mutating preflight. Public/shareable responses exclude restricted details; an authorized local response may use necessary private context with its provenance identified.

Complete when one case file is selected, draft freshness is known, and the next action follows the current state without reviving superseded understanding.

## Update case

1. Before writing, classify every proposed item: put relevant shareable knowledge in public; put only necessary sensitive context in private; omit irrelevant process chatter; exclude credential values from both. When a person's, customer organization's, or tenant's identity is not established as shareable, generalize it in public and retain the exact value privately only when continuation needs it. Prefer a safe abstraction in public over moving ordinary investigation content to private. Profanity, sarcasm, frustration, and unsupported personal accusations are omitted rather than moved private; preserve only the technical requirement, impact, uncertainty, or material disagreement. An independent auditor is optional when classification remains ambiguous.
2. Rewrite informal input as concise professional findings, decisions, questions, and history. Use the smallest complete set: one source entry, only independent evidence, only questions that change the next action, only decisions actually made for this case, and only criteria needed to test the objective. Do not register obvious arithmetic as a separate inference or turn this skill's policy into a case decision. Keep each detail in one register, Current state as a brief synthesis, and History about changes rather than restating the case. Never persist raw conversation, hidden reasoning, embarrassment-prone phrasing, or a verbatim user request.

   Before saving, remove repetition: refer to register IDs instead of restating their explanations in Current state and Readiness. Do not create domain decisions just to repeat the request to open the case, or fill records with workflow-compliance boilerplate. Keep an unanswered question and the minimum evidence needed to resolve it, not an unsolicited future troubleshooting runbook. Delete a sentence when it adds no fact, source, material decision, limitation, or next action beyond what is already recorded. Empty optional content is preferable to filler; preserve required headings.
3. Consume supported conclusions with their sources, limits, and unresolved questions. Keep facts, inferences, contradictions, and decisions distinct in the existing registers. Do not repeat a sufficient analysis simply to persist it. If support is missing, use the shared method before recording the claim as established.
4. Preserve the boundary between **current productive state** and **future/proposed state**. Maintain `vault-outcome` as evidence changes; a case is context and provenance, never proof of productive behavior. If supported evidence establishes a material error or omission in an existing map, apply [references/map-correction.md](references/map-correction.md) in this iteration; existing vault-update authority enables the bounded correction through Promote.
5. When a material documentary decision remains ambiguous, use [references/questioning-protocol.md](references/questioning-protocol.md).
6. For each authorized material update, prepare complete reviewed candidate snapshots and invoke `save` with the SHA-256 values returned by `load`, one portable `--source`, every affected public stable ID as repeated `--target` arguments, and only IDs whose restricted context changed as repeated `--private-target` arguments. Never edit a case in place. The helper resolves the effective Git identity, appends the attributed History events, advances timestamps, rejects stale inputs, and rolls back a failed coordinated write. Do not create a private candidate unless it contains necessary material.
7. Compare the proposed snapshots with the loaded bytes before saving. Repeated input with no new material keeps both snapshots byte-identical; the helper returns `unchanged` and adds no timestamp or History event.
8. Preserve stable identifiers and replacement links; never renumber, recycle, or silently change meaning.
9. Reconcile or mark every affected draft stale in the same interaction.
10. When new evidence could change a completed learning assessment, preserve the old assessment in History and reset `learning-outcome` to `not-evaluated`.

Complete the iteration when each material change has one canonical register entry, Current state briefly reflects the result, History identifies the affected IDs without duplicating their contents, independent outcomes reflect the evidence snapshot, and affected drafts have an explicit synchronization state.

## Bind development handoff

1. Accept only one normalized in-memory observation assembled by `manage-development-handoff` after repository-state validation succeeds. Require the package identity, investigation and story, tracker identity, normalized repository remote, exact branch, handoff ID, family, revision, and materialization timestamp. Never persist the local worktree path.
2. Resolve the exact source case from the investigation ID, read the entire current case, and verify that the package source identity and story still match it. Reject a missing case, missing story, mismatched work-item identity, or a story-and-repository identity already bound to different immutable coordinates.
3. Use the public-case and exact story SHA-256 snapshots retained by the producer's pre-materialization sufficiency review. Serialize the validated observation as JSON using the record contract's binding marker fields except `dh`. Invoke `scripts/investigation-case.py --root "$VAULT_ROOT/investigations" bind --id <case-id> --observation <local-json-path> --expected-public-sha256 <reviewed-case-sha> --expected-story-sha256 <reviewed-story-sha>`. The helper checks both snapshots before any write or retry no-op. If either is stale or the review snapshots are missing, keep the result `materialized-unbound`: re-review the current case and story against the exact materialized package before capturing replacement hashes. Refresh the package when its content no longer matches; never refresh hashes solely to bypass the check. The helper owns locking, validation, rollback, and exact-retry no-op.
4. Binding changes only the handoff register and its History event. Preserve the case's semantic `updated-at` so registering unchanged exported content does not stale its own package or stories. The manifest timestamp remains the binding's `Materialized at` and event time. Report any binding failure to the initiating workflow; keep the materialized worktree available for exact retry.

Complete only when the canonical case contains exactly one mechanically valid binding for the observed story-and-repository identity and its ordered History records every materialized content revision once. This route never reads or writes repository state, a tracker, remote Git, or the technical vault.

## Reconcile development

1. Accept only one complete normalized in-memory context assembled by `reconcile-development-handoff`; require the exact case, `DH-NNN`, story, work item, repository, branch, handoff ID, family, revision, and validated local closure fingerprint.
2. Load [references/development-reconciliation.md](references/development-reconciliation.md), read the entire current case, and reject any identity mismatch or missing comparison surface.
3. Apply the assembled baseline deltas, changelog provenance, implementation/delivery/work-item evidence, and direct-dependent cards to Current state and the stable registers without re-reading or mutating the worktree. Record the closure fingerprint in the single reconciliation History event as the local snapshot binding.
4. Reconcile affected drafts, reset a stale learning assessment when required, append one material History event, and run the case validator. Preserve a byte-level no-op when nothing in the case changed.

After updating the affected registers, reevaluate the global objective and closure criteria. Reconciliation never closes the investigation automatically and a handoff terminal state is not a closure decision. Leave the current investigation status unchanged unless the same interaction separately applies an explicit lifecycle transition with its own evidence and current snapshot.

Complete when the selected case alone represents the reconciled implementation and downstream context, its current/future boundary remains correct, every affected draft has an explicit synchronization state, global closure criteria were reevaluated without an automatic close, structural validation passes, and no source repository, tracker, remote Git, pull request, deployment, or technical-vault write occurred.

## Consolidate

Load [references/deduplication-and-consolidation.md](references/deduplication-and-consolidation.md) and apply its semantic reconciliation protocol. Keep the oldest case as canonical unless the user selects another case or current evidence requires a different target. After producing the required mapping artifact, invoke the helper's `consolidate` command for the transactional archive, lineage update, exact removal, validation, and rollback.

Complete when exactly one case directory remains for the equivalence group, its current state contains the reconciled material, `learning-outcome` was preserved only against an unchanged assessment basis or reset otherwise, every source identifier remains traceable, drafts have an explicit synchronization state, and searching a retired ID in `consolidated-from` resolves to the canonical case.

## Validate

For a request to assess whether an exact development package is complete enough to implement, read its source case and apply the [question-based implementation-sufficiency check](references/implementation-sufficiency.md). Report supported answers and concrete gaps without changing the case, package, tracker, or worktree. This assessment does not change investigation readiness or handoff lifecycle state.

For investigation readiness instead, run the helper's `validate` command for structural and transactional invariants, then apply the lifecycle gate from [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md). Keep `investigating` while useful work remains. When a required source prevents useful progress on the global objective, report the observable failure and invoke the helper's `transition --to blocked` command with `blocked-on`, a portable source, reason, and the current public SHA-256. A target-specific gap does not block the whole case when other useful work can continue.

Complete a package assessment when the supported answers and gaps are reported without writes. Complete lifecycle validation when the status is supported by its gate, or `blocked` names the concrete dependency and user or external action required.

## Close

1. Load [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md) and evaluate the current objective, scope, registers, criteria, independent vault outcome, draft synchronization, and limitations. Do not infer completion from the case purpose, an export, a terminal handoff state, a merge, or a deployment claim.
2. For `complete`, require a resolved purpose and cite one or more existing register IDs that support the objective-specific closure. Require implementation, publication, or deployment only when the objective or an applicable criterion requires it.
3. For `abandoned`, require an explicit discontinuation decision and preserve the unresolved limitations. Evidence IDs are optional and `purpose` may remain undecided.
4. Invoke the helper's `close` command with the exact SHA-256 returned by the latest load, decision, formalized reason, limitations, portable source, and every closure evidence ID. Never set `status` or `closure-outcome` through a normal save.
5. If a closed case receives material new evidence, invoke `transition --to investigating` with the current SHA-256 and preserve the former closure in History before applying the evidence.

Complete when the helper validates and atomically records `status: closed`, the explicit `closure-outcome`, attribution, reason, evidence boundary, and limitations, or when a failed semantic gate leaves the loaded bytes unchanged.

## Export

1. Generate platform-neutral local drafts in `exports/` from [assets/story-template.md](assets/story-template.md).
2. Match story kind and detail to the audience and purpose; split only independently deliverable outcomes.
3. Stamp each draft with the current investigation `updated-at`, verify every referenced register item, and set its synchronization state from the export contract.
4. Label a draft provisional until its selected-output sufficiency gate passes. Keep the lifecycle state unchanged; draft readiness never advances or closes the investigation.
5. When the user requests external publication of an output-sufficient draft, build the publication package defined by the export contract and hand it to `manage-operational-workflow`.
6. Keep connector selection, external-effect authorization, publication, and read-back verification inside that operational workflow.
7. After the handoff returns a verified result, update the local draft and History with the non-sensitive external reference and publication status.
8. When the user requests development handoff for one or more output-sufficient stories, load `../../../90-Meta/work-item-evidence.md`, obtain current exact work-item snapshots, and build one repository-specific package per target from the development section of the export contract. Complete its question-based implementation-sufficiency check before handing any package to the consumer. Evaluate each selected story and repository independently; unrelated open work does not block the package.
9. Treat an exact repository remote as mandatory but not sufficient for sharing. Require overlap in the named component or implementation scope. Include compatible existing `DH-NNN` entries as reuse candidates using repository and branch identity; never persist a local path.
10. Hand the exact package directories, choice, and selected branch to `manage-development-handoff`; that skill resolves the local path from configuration and validates Git before use.
11. Observe the consumer's terminal state. A target is complete only after **Bind development handoff** validates its stable `DH-NNN`; `materialized-unbound` blocks that target, with the repository state left inspectable and active for an exact retry. Change the global investigation to `blocked` only when that target prevents all useful progress on the case objective.

Complete when each draft is current and traces to the case file and evidence, every requested external publication has either been handed off with an exact current package or observed with a verified reference recorded locally, and every materialized development target has one current package plus one exact `DH-NNN` binding or an explicit source blocker.

## Learn

1. Require one selected case and read its entire current snapshot before the handoff.
2. Hand the exact investigation path and requested mode to `manage-investigation-derived-learning`. Use **Assess** unless the user explicitly requested durable publication or revalidation.
3. Let that skill inspect sources, search existing learnings, apply its independent gate, and present one of `extractable`, `no-learning`, `already-covered`, or `insufficient-evidence`. Do not pre-classify the answer or treat case status as proof.
4. Keep the dependency one-way during assessment: the learning skill reads but never mutates the case. After it returns, map `no-learning`, `already-covered`, and `insufficient-evidence` directly; map an unpublished `extractable` result to `candidate`; map a verified durable write to `documented`.
5. When persisting the returned status, update `learning-outcome`, `updated-at`, Readiness, and History together with the assessed case snapshot, canonical target if any, evidence boundary, lifecycle action, and observed checks. A blind test or explicit assess-only request that forbids persistence ends before this step.
6. Do not change `vault-outcome`: technical production documentation and investigation-derived learning are independent paths.

Complete when the user has the explicit assessment result and action, no forbidden test or assess-only state was persisted, and any authorized case update accurately records the returned snapshot without replacing its evidence.

## Promote

1. Require an explicit candidate claim and a classified `purpose`. Read `../../../90-Meta/evidence-policy.md` and the cell's `evidence.profile` before deciding eligibility.
2. Separate inspected implementation from proposals and deployment assertions. Under `documented-source`, an implemented claim at an exact inspected revision may proceed before deployment; describe only source behavior. For corrections of existing source maps under [references/map-correction.md](references/map-correction.md), apply the source-map rule in evidence-policy.md under every profile and describe only inspected source/configuration. For other promotion under `production-gate` or `mixed`, set `vault-outcome: deferred-until-production` when required productive applicability remains pending or ambiguous. Unimplemented proposals remain deferred under every profile.
3. Set an eligible claim to `vault-outcome: candidate-for-audit` and build a context package with exact claims, source revisions, likely canonical nodes, evidence profile and limitations. Case files, trackers and approvals provide context, not technical evidence.
4. Hand the package and established vault-write authorization to `map-ecosystem` for an independent profile-specific audit. An explicit post-deployment audit may recheck productive applicability even when source HEAD is unchanged. That skill owns evidence assessment, node lifecycle, writes, propagation and verification.
5. Set `vault-outcome: documented` only after the independent audit passes and the canonical vault already represents the claim correctly or was updated and verified. Record canonical notes, evidence profile, evidence boundary, lifecycle result and observed checks in Readiness and History. Otherwise retain the exact missing evidence with `candidate-for-audit`, use `none` for a disproved or removed candidate, or defer a proposal or required deployment.

Complete when every candidate has an explicit outcome and each documented claim traces to an independent audit at the cell's required evidence level. Source-only claims remain explicitly scoped to their inspected revision.

## Guardrails

- Keep `investigations/` versionable. Keep `.investigations-private/` and legacy `.investigations/` local and ignored.
- Use the cell's configured note locale for records and drafts; keep this skill and its references in English.
- Public is the sole source of investigation knowledge and decisions. Private is exceptional, supplementary, and safe to omit when sharing.
- Git identity names the recorder only. Record the decision-maker or approver separately only when an inspected source establishes that role; never infer approval from the recorder.
- Persist secret existence, location, and behavior only with values redacted. Stop before copying sensitive values into the case file.
- Treat the investigation as context and provenance, never as evidence that a behavior exists in production.
- Route publication, ticket creation, and other external writes through `manage-operational-workflow`; keep commits, pushes, and pull requests behind their own explicit authorization.
