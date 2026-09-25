---
name: manage-investigation
description: Maintain investigation case files. New cases are unpublished by default; publish moves a sanitized copy into the versioned tree. Use to open, resume, publish, reconcile, validate, prepare a production handover, absorb knowledge, or explicitly retire an investigation, as well as migrate, export, learn from or promote it.
---

# Manage investigations

Treat `investigations/<id>/investigation.md` as the published, versionable case. New cases open under `.investigations/<id>/` with the same contract. Keep case-specific methods that explain or repeat an analysis and outputs whose contents support a claim in the case `artifacts/`; a one-time report delivery does not require its generated file there. Review selected contents and dependencies before publication. An optional `.investigations-private/<id>/private.md` may add necessary sensitive context but never overrides case status, evidence, decisions, acceptance criteria, or history. Keep machine-specific files, temporary outputs and unpublished revision candidates in `.investigations-private/<id>/local/`.

Own documentary persistence and traceability, not the general inquiry method. Answering a question does not require a case. Open only when the user requests one or accepts a recommendation; recommend one when continuity or collaboration would benefit from retained evidence, decisions, or pending work. For already-supported updates, proceed directly to the documentary checks. When evidence still needs to be obtained or evaluated, read [evidence-driven-analysis](../evidence-driven-analysis/SKILL.md) in full before that analysis unless it is already loaded in the current context. Retain this workflow as owner, reuse valid checks, and consume the result without routing back.

## Preflight

Always:

1. Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault), reusing the session's binding. Bind one canonical `VAULT_ROOT`; use `VAULT_ROOT/investigations/` as the published root. The helper also resolves `.investigations/` and `.investigations-private/` beside it.
2. Require `investigations/` to be eligible for tracking. Require both `.investigations-private/` and `.investigations/` to be ignored and absent from `git ls-files` when they exist.
3. Use the selected `<VAULTCTL> investigation` for Open, Load, List, Snapshot, Save, Save resources, Publish, Transition, Consolidate, Bind, Close, Retire, and validation; do not reproduce its discovery, locking, attribution, or recovery logic manually. `--root` is always `$VAULT_ROOT/investigations` even when the live case is unpublished.

Read-only lookup is complete when the published root is trackable and ignored investigation roots stay untracked. It does not require Git author identity or the record contract.

Before a mutating route, additionally:

4. Load [references/record-contract.md](references/record-contract.md).
5. Confirm that the vault checkout has effective Git `user.name` and `user.email`. The helper enforces this gate and never changes Git configuration.

## Route the request

| Request | Load / action |
| --- | --- |
| **Open** | [deduplication-and-consolidation.md](references/deduplication-and-consolidation.md), then the Open steps below |
| **Resume** / read-only case question | Resume steps below |
| **Publish** | Publish steps below |
| **Migrate** | [migration.md](references/migration.md) |
| **Reconcile collaboration** | [investigation-reconciliation.md](references/investigation-reconciliation.md) |
| **Update case** | [update-case.md](references/update-case.md) |
| **Component progress / production preparation** | [components-and-release.md](references/components-and-release.md) |
| **Observe** | Hand an agreed proposal to `manage-operational-workflow` and its `references/observation.md`; observation alone does not authorize case updates |
| **Bind development handoff** | [bind-handoff.md](references/bind-handoff.md) |
| **Reconcile development** | Consume one normalized result from `reconcile-development-handoff`; load [development-reconciliation.md](references/development-reconciliation.md) |
| **Consolidate** | [deduplication-and-consolidation.md](references/deduplication-and-consolidation.md), then helper `consolidate` |
| **Validate** | Helper `validate`; for an exact development package, [implementation-sufficiency.md](references/implementation-sufficiency.md) |
| **Transition** | [readiness-and-lifecycle.md](references/readiness-and-lifecycle.md) |
| **Close** | [readiness-and-lifecycle.md](references/readiness-and-lifecycle.md); helper `close` with the loaded SHA-256, decision, reason, limitations, source, and evidence IDs |
| **Export** | [export-contract.md](references/export-contract.md) |
| **Learn** / **Promote** | [learn-and-promote.md](references/learn-and-promote.md) |
| **Absorb / Retire** | [knowledge-and-retirement.md](references/knowledge-and-retirement.md) |

Use no parallel active index. The helper's `list` derives the overview from current cases and the minimal retirement register; `retired` describes storage disposition, not a fourth lifecycle state. Interrogation may call `<VAULTCTL> links --vault "<VAULT_ROOT>" --node "<stem>"` for the on-demand public-case/node join. That query is not a second store and does not replace exact-ID retirement lookup.

## Open

1. Load [references/deduplication-and-consolidation.md](references/deduplication-and-consolidation.md), derive the immutable ID and stable `dedupe-key`, classify `purpose`, set the initial `vault-outcome` and `learning-outcome: not-evaluated`, then invoke the helper's `open` command. Open creates an unpublished case unless the user explicitly requests a published case.
2. Resolve semantic ambiguity before invoking the helper. Route `definite_match` to **Resume** and continue only from `created`.
3. Formalize only the relevant request. Do not preserve the conversation transcript or literal informal wording. Capture a durable source reference when available and process attachments under the record contract.
4. Set `status: investigating`. `purpose: undecided` may remain while the requested outcome is being classified, but resolve it before completing the case or producing an external or development output. `vault-outcome: not-evaluated` may remain while the investigation still lacks the evidence needed to assess a durable current-state candidate.

Complete when the case has no equivalent active case, has a unique ID, professional request summary, source, objective, scope, purpose, initial outcomes, attachment decisions, and creation history.

## Resume

1. Search unpublished `.investigations/` and published `investigations/`. For an exact ID, use helper `load` to distinguish present unpublished, present published, legacy unpublished, consolidated, retired and missing. A retired result supplies historical provenance and destinations, not an active path; follow the retirement reference rather than recreating or reopening it automatically. If a published match uses a prior lifecycle state or obsolete lifecycle metadata, require **Migrate** before ordinary mutation. If `load` returns `legacy`, require **Migrate** into an unpublished current-schema case before any mutation.
2. Invoke `load --id <id>` after selecting the case. For a present case, read `public.path` (the case file at the returned `visibility`, not a published-only path) and, when `private.available` is true, the returned private path. When `local.available` is true, treat that directory as local work and unpublished revision staging, not case evidence. For a retired result, use the historical lookup above and stop the live-case route. This lookup is mandatory even when the user does not mention private context. Label private and local provenance and keep them supplementary and non-authoritative. When they are absent, state that the requested private or local fact is unavailable instead of inferring it; the case file must remain intelligible without them.
3. When `exports/` contains drafts, load [references/export-contract.md](references/export-contract.md) and inspect their source timestamps and register references.
4. Reconstruct authoritative state from the case frontmatter and **Current state**; use **History** only for provenance. After publish, the published case is the sole authority for status, evidence, decisions, and acceptance criteria.
5. Report the status, visibility, closure outcome when closed, purpose, independent outcomes, current understanding, blockers, open questions, handoffs, stale drafts, private-overlay availability, local-working availability, and next useful action.

For a question about an existing case, use the same load and overlay discovery, then answer only the requested scope without writes, lifecycle transitions, or a mandatory full status report. Read-only lookup does not require Git author identity or the mutating preflight. Shareable responses exclude restricted details and local tool workspaces; an authorized local response may use necessary private or local context with its provenance identified.

Complete when one case file is selected, draft freshness is known, and the next action follows the current state without reviving superseded understanding.

## Publish

1. Require an explicit request to make the unpublished case versionable. Load the case and review every file in the proposed published set, including methods, generated outputs, embedded data, metadata and dependencies. Record any excluded dependency and its effect on the case's claims. Keep selected local material under `.investigations-private/<id>/local/`, necessary sensitive context in the overlay and protected data at its authorized source. Exclude secret values.
2. Review shareability by content and audience. Apply the retention criteria in [record-contract.md](references/record-contract.md) to each method and output; keep host-specific settings and disposable outputs local. Acceptance criteria remain about the verifiable result. Publication does not by itself establish that a method runs elsewhere or reproduces historical results.
3. Use `snapshot --case-dir <unpublished-case-dir>` to review the full tree and `load` to obtain `public.sha256` and `public.tree_sha256`. Invoke `publish --id <id> --expected-public-sha256 <public.sha256> --expected-tree-sha256 <public.tree_sha256> --source <portable source>`, repeating `--retain-local <case-relative path>` for each reviewed file or directory excluded from publication. The helper copies retained files into `.investigations-private/<id>/local/publish-retained/<reviewed-tree-sha256>/`, publishes the selected tree, appends the event and removes the unpublished directory. Review references to excluded files before invocation. After publication, check resulting paths, bytes and links against the reviewed set. Overlay and `local/` stay in `.investigations-private/`. There is no unpublish.
4. Durable vault writes under `10/`–`70/` and investigation-derived learning that needs a versioned source require this published case. Local exports and development handoffs may exist before publish.

Complete when `load` reports `visibility: published`, the unpublished directory for that ID is gone, validation passes, the published file set matches the review, and excluded local material remains available at its recorded location.

## Reconcile development

1. Accept only one complete normalized in-memory context assembled by `reconcile-development-handoff`; require the exact case, `DH-NNN`, story, work item, repository, branch, handoff ID, family, revision, and validated local closure fingerprint.
2. Load [references/development-reconciliation.md](references/development-reconciliation.md), read the entire current case, and reject any identity mismatch or missing comparison surface.
3. Apply the assembled baseline deltas, changelog provenance, implementation/delivery/work-item evidence, and direct-dependent cards to Current state and the stable registers without re-reading or mutating the worktree. Record the closure fingerprint in the single reconciliation History event as the local snapshot binding.
4. Reconcile affected drafts, reset a stale learning assessment when required, append one material History event, and run the case validator. Preserve a byte-level no-op when nothing in the case changed.

After updating the affected registers, reevaluate the global objective and closure criteria. Reconciliation never closes the investigation automatically and a handoff terminal state is not a closure decision. Leave the current investigation status unchanged unless the same interaction separately applies an explicit lifecycle transition with its own evidence and current snapshot.

Complete when the selected case alone represents the reconciled implementation and downstream context, its current/future boundary remains correct, every affected draft has an explicit synchronization state, global closure criteria were reevaluated without an automatic close, structural validation passes, and no source repository, tracker, remote Git, pull request, deployment, or technical-vault write occurred.

## Close

1. Load [references/readiness-and-lifecycle.md](references/readiness-and-lifecycle.md) and evaluate the current objective, scope, registers, criteria, independent vault outcome, draft synchronization, and limitations. Do not infer completion from the case purpose, an export, a terminal handoff state, a merge, or a deployment claim.
2. For `complete`, require a resolved purpose and cite one or more existing register IDs that support the objective-specific closure. Require implementation, publication, or deployment only when the objective or an applicable criterion requires it.
3. For `abandoned`, require an explicit discontinuation decision and preserve the unresolved limitations. Evidence IDs are optional and `purpose` may remain undecided.
4. Invoke the helper's `close` command with the exact SHA-256 returned by the latest load, decision, formalized reason, limitations, portable source, and every closure evidence ID. Never set `status` or `closure-outcome` through a normal save.
5. If a closed case receives material new evidence, invoke `transition --to investigating` with the current SHA-256 and preserve the former closure in History before applying the evidence.

Closure makes the case a candidate for the selective retention assessment in [references/knowledge-and-retirement.md](references/knowledge-and-retirement.md), not automatic investigation Publish or deletion. When the objective includes observation, apply the agreed coverage and outcome conditions from the operational run; elapsed time or a completed audit alone does not satisfy them.

Complete when the helper validates and atomically records `status: closed`, the explicit `closure-outcome`, attribution, reason, evidence boundary, and limitations, or when a failed semantic gate leaves the loaded bytes unchanged.

## Guardrails

- Keep `investigations/` versionable. Keep `.investigations/` and `.investigations-private/` local and ignored.
- Use the cell's configured note locale for records and drafts; keep this skill and its references in English.
- After publish, the published case is the sole source of investigation knowledge and decisions. Unpublished is the working case until then. Private overlay is exceptional, supplementary, and safe to omit when sharing. Local working material and revision candidates are never authoritative.
- Git identity names the recorder only. Record the decision-maker or approver separately only when an inspected source establishes that role; never infer approval from the recorder.
- Persist secret existence, location, and behavior only with values redacted. Stop before copying sensitive values into the case file, overlay, or local working store.
- Treat the investigation as context and provenance, never as evidence that a behavior exists in production.
- Investigation **Publish** stays in this skill. Durable `10/`–`70/` writes go through **Promote** or **Learn**. Ticket creation, story/export platform writes, and other external writes go through `manage-operational-workflow`. Keep commits, pushes, and pull requests behind their own explicit authorization.
