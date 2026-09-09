# Synchronize the vault

## Authority and outcome

Synchronize only on an explicit vault-update request; report mode is read-only. Source repositories are evidence surfaces and remain read-only. A completed run may produce durable documentation, an acknowledgement, or neither. It always records the decision against the inspected commit.

The coordinator owns inventory, source binding, durable state, gate, projection, application, and closure. Extractors and reviewers own one artifact each and follow [synchronization-package-worker.md](synchronization-package-worker.md). Load [synchronization-state-machine.md](synchronization-state-machine.md) before choosing a command after any interruption or failure.

## Scope and incremental delivery

Apply [repository-map.md](repository-map.md) before building a worker card. The initial deliverable is a repository purpose/main-flow/connector map. Resolve cross-repository and cloud edges after local extraction. Preserve valid existing analyses and notes; instruction/model changes alone do not justify a new full scan. Select small independent inventory groups before `begin`, so one slow repository does not hold every result. Do not split or rewrite an already frozen run to claim it completed; preserve it and report its state.

## Worker execution choices

Follow the bounded pilot and execution limits in repository-map.md. Record a batch ceiling, stop conditions and actual usage; dispatch no automatic escalation or correction during a pilot. Choose each worker's available model and reasoning effort for its task complexity and the consequences of an error; do not automatically inherit the root orchestrator's settings. Distinguish documentation-only inspection from business logic, data contracts, environment differences and cross-repository dependencies. Independent semantic review and targeted correction need enough reasoning for the affected meaning. Keep mechanical validation in the deterministic tools. Record the chosen model, effort and rationale with local execution evidence, without adding provider-specific fields to the semantic artifact schemas.

## Coordinator recipe

1. **Freeze inventory and impact.** Apply [evidence-extraction.md](evidence-extraction.md). Run `90-Meta/vault-inventory.py --format markdown`. Classify each relevant repository, resolve each checkout by configured remote identity, and record its initial `git status --short`. For `changed`, bind the recorded commit and production head; for `new`, bind the empty-tree baseline and production head. An unresolved checkout, unreadable exact evidence, or ambiguous identity is a hard blocker.
2. **Begin or reuse the run.** From the vault, call:

   ```text
   python3 -B 90-Meta/sync-run.py begin --state-root .agents/state/map-ecosystem/sync --tool-digest <sha256> --inventory-digest <sha256> --package <repository> <new_oid> [...]
   ```

   Bind the inventory digest to the selected repository/OID pairs, evidence profile, relevant vault-context file hashes and frozen additional source revisions; record an explicit same-commit audit reason when new dependency evidence or a resolved limitation changes scope. Use only the emitted `run_id`. Equal fingerprints reuse the integrity-checked active run or closed receipt; a different fingerprint is never written over. Keep durable state under `.agents/state/map-ecosystem/sync/`, not in a source checkout or a worker workspace.
3. **Extract and review each package.** Freeze `evidence.profile` from `instance.yaml` as `evidence_profile` in each worker card; load the shared evidence policy before interpreting claims. Build the immutable manifest and analysis scaffold for each frozen range. Dispatch one extractor; deterministically finalize and check its artifact. Dispatch one fresh reviewer for an eligible checked package. If it returns `revise` with actionable findings, use the bounded correction below before checkpointing; use only the final selected artifact pair for closure. Close the validated semantic product with `git-change-manifest.py close-package --repo <checkout> --manifest <manifest> --scaffold <scaffold> --analysis <analysis> --review <review> --production-ref <refs/heads/main-or-master-or-refs/remotes/origin/main-or-master> --analysis-date <frozen-YYYY-MM-DD> --output <package.json>`, then persist that complete finalized, redacted package with `checkpoint-package --state-root <root> --run-id <run_id> --repository <repository> --artifact <package.json>`. Closure re-resolves that same canonical production ref and rejects it when it no longer equals the analyzed `new_oid`; a stale or missing local branch does not override the frozen `origin` ref. Every write-ready package requires its independent review; a cursor fallback may close without one. A digest-only, unreviewed write-ready, stale-source, sensitive, or structurally invalid artifact is not a checkpoint.
4. **Seal semantic authority.** Run `git-change-manifest.py gate-batch --expected-repository <repository> [...] --package <closed-package.json> [...] --output <gate.json>` across the complete frozen inventory, using only the checkpointed package bytes. Then seal with `sync-run.py seal-gate --state-root <root> --run-id <run_id> --gate <gate.json>`. Sealing rejects repository, grant, or acknowledgement authority that differs from a closed package and creates independent `acknowledgements` and ordered `group-NNN` units.
5. **Project each unit.** The coordinator creates one projection and patch for the selected unit. A projection v1 binds the `run_id`, sealed gate digest, unit, base/result-file hashes, patch digest, and accepted grants. Markdown targets stay below the canonical knowledge roots; `.agents/`, `90-Meta/`, and other control surfaces are never authorized by a matching basename. Validate the semantic binding with:

   ```text
   python3 -B 90-Meta/git-change-manifest.py validate-projection --gate <gate.json> --projection <projection.json> --patch <patch>
   python3 -B 90-Meta/sync-run.py validate-unit --state-root <root> --run-id <run_id> --unit-id <unit_id> --projection <projection.json> --patch <patch>
   ```

   The acknowledgement unit changes only `90-Meta/.sync-acknowledgements.json`; each group changes only its authorized nodes and paths. A failed unit does not prevent another independent unit from validating or applying.
6. **Apply independently.** Apply each validated unit with `apply-unit --state-root <root> --run-id <run_id> --unit-id <unit_id> --vault <vault-root>`. Application verifies both path kind (`missing` or regular file) and `before` hash, journals and rolls back multi-file writes, then verifies the complete `after` images and records the receipt. A later resume recognizes a complete matching `after` image if a process stopped before the receipt was saved; a mixed preimage/postimage is blocked and never completed forward by either `apply-unit` or `resume`. Symlinked vault path components are unsafe persistence and are never followed.
7. **Resume precisely.** Use `status --state-root <root> --run-id <run_id>` for the recorded next command. Use `resume --state-root <root> --run-id <run_id>` after a recoverable projection or application failure. Preserve the same `run_id`, packages, and sealed gate whenever their bindings remain valid. On source drift, retract any applied affected unit to its exact preimage before checkpoint, reseal, or reuse of an older receipt; preserve unaffected work by semantic authority even if group ordinals change. On destination drift, preserve the observed user bytes and reproject that unit. Follow the state-machine failure table.
8. **Close and verify.** After every unit is applied, run `close --state-root <root> --run-id <run_id>`. It writes the safe durable receipt and removes only that active run directory. If interruption occurs after receipt persistence, retry `close`; the valid receipt idempotently retires its matching active orphan. Re-run inventory, compare source statuses to the initial snapshot, and run the relevant vault checks (`audit-vault.py`, `verify-links.py`, available Obsidian checks, and `validate-bases.py` only when applicable).

## One targeted correction before checkpointing

If reviewer prose contains a detected credential literal, preserve its original artifact and run `python3 -B 90-Meta/finalize-sync-review.py --repo <checkout> --manifest <manifest> --scaffold <scaffold> --analysis <analysis> --review <original-review> --output <safe-review>`. The checkout HEAD must still equal the analyzed commit. This helper only redacts finding reasons and evidence anchors; it preserves verdicts and all decision metadata, validates bindings and evidence, and refuses exposures elsewhere or a different existing output. Use the validated safe review for correction or closure. This is mechanical redaction, not another semantic review or a detector exemption.

For a checked, finalized analysis with a valid `revise` review, run:

```text
python3 -B 90-Meta/sync-correction.py prepare --repo <checkout> --manifest <manifest> --scaffold <scaffold> --analysis <analysis> --review <review> --current-ref <frozen-production-ref>
```

It reserves exactly `correction-1/` beside the initial analysis, copies digest-bound read-only initial artifacts, and seeds its `analysis.json`. Keep the original artifacts. Use the emitted workspace as the correction worker's `run_root`, its `initial-manifest.json` and `initial-scaffold.json` as manifest/scaffold, and its `analysis.json` as the only extractor output. Supply the optional `correction` card with receipt and initial analysis/review paths. The worker repairs findings and their connected claims and paths; it does not restart repository extraction.

Finalize and check that candidate using the existing manifest CLI, with its own `finalize-result.json`. Dispatch a fresh reviewer on the checked repaired artifact and its correction context, writing `correction-1/review.json`. An added omitted claim may cite newly inspected dependencies at the frozen source revisions when attached to a finding's affected node. The reviewer must establish each new dependency's causal relevance to the finding; a valid source path alone is insufficient. Preserve unrelated nodes and existing accurate claims. Then run:

```text
python3 -B 90-Meta/sync-correction.py check --repo <checkout> --workspace <correction-1> --review <correction-1/review.json> --current-ref <frozen-production-ref>
```

A passing scope/integrity check permits the ordinary `close-package` using those final artifacts; its final independent verdict still controls accepted claims and safe partial/rejected outcomes. Checkpoint only that closed package. A second correction is refused. If preparation is interrupted, the candidate remains unchanged, or finalization or correction validation fails, preserve the original artifacts and all failed correction artifacts. Revalidate the original pair against the frozen source ref, report the failed correction, and close using that original valid review outcome. Live source drift remains blocked; never use fallback to bypass it. Keep the reservation rather than starting another semantic attempt. If a complete workspace already exists after interruption, resume its unfinished step instead of preparing again.

The once-only bound is an artifact/workflow contract for the trusted local coordinator, not protection from a writer who deletes its own run. Publication failures after checkpointing reuse the final package; they never reopen correction.

## Documentation limits

Publish only claims authorized by the sealed gate grants and supported by the frozen cell evidence profile. `traceability-only` has no claim and therefore produces an acknowledgement, never a Markdown write group. Every granted node must be covered by the unit projection. Do not put run, review, gate, grant, package, or cursor process language in durable technical notes. An acknowledgement contains only its gate-bound closed cursor fields; it contains no claim text, findings, source bytes, or sensitive values.

An accepted `traceability-only` package for an existing repository closes as `no-documentation-change` (`acknowledged-no-change` in inventory): the reviewed delta requires no documentation update and the note keeps its previous baseline. An accepted new repository without a durable node closes as `no-durable-node`. Rejected or incomplete reviews and extraction fallbacks retain `review-rejected` or `inspection-limited`; these outcomes do not establish that documentation is current. Preserve historical acknowledgements unless a newly authorized inspection supplies a new gate decision.

The coordinator, kernel, and durable state share one trusted OS principal.
Package hashes prove integrity and lineage, not authorship against that same
principal. Write authority requires the bounded independent review artifact;
cryptographic authentication would require an external signer and is outside
this local workflow.

## Completion criterion

Complete only after all inventory items have a persisted decision, every unit is applied, the durable receipt exists, the second inventory has no repeated inspected SHA, relevant vault checks pass, and source checkouts remain unchanged. Verify the published meaning and preserved knowledge using [evidence-extraction.md](evidence-extraction.md).

Report the run closure separately from its documentation outcome. For each repository, give the inspected commit and one result: documentation updated (with written paths), reviewed with no documentation change required, or inspection limited/review rejected (with the unresolved gap and retained note baseline). A zero `changed`/`new` count means no uninspected source delta; it does not mean every note was updated or every limitation resolved. Summarize accepted no-change and limited/rejected counts separately.
