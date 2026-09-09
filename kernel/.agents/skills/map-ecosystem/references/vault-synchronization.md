# Synchronize the vault

## Authority and outcome

Synchronize only on an explicit vault-update request; report mode is read-only. Source repositories are evidence surfaces and remain read-only. A completed run may produce durable documentation, an acknowledgement, or neither. It always records the decision against the inspected commit.

The coordinator owns inventory, source binding, durable state, gate, projection, application, and closure. Extractors and reviewers own one artifact each and follow [synchronization-package-worker.md](synchronization-package-worker.md). Load [synchronization-state-machine.md](synchronization-state-machine.md) before choosing a command after any interruption or failure.

## Coordinator recipe

1. **Freeze inventory.** Run `90-Meta/vault-inventory.py --format markdown`. Classify each relevant repository, resolve each checkout by configured remote identity, and record its initial `git status --short`. For `changed`, bind the recorded commit and production head; for `new`, bind the empty-tree baseline and production head. An unresolved checkout, unreadable exact evidence, or ambiguous identity is a hard blocker.
2. **Begin or reuse the run.** From the vault, call:

   ```text
   python3 -B 90-Meta/sync-run.py begin --state-root .agents/state/map-ecosystem/sync --tool-digest <sha256> --inventory-digest <sha256> --package <repository> <new_oid> [...]
   ```

   Use only the emitted `run_id`. Equal fingerprints reuse the integrity-checked active run or closed receipt; a different fingerprint is never written over. Keep durable state under `.agents/state/map-ecosystem/sync/`, not in a source checkout or a worker workspace.
3. **Process each package once.** Build the immutable manifest and analysis scaffold for each frozen range. Dispatch one extractor; deterministically finalize and check its artifact. Dispatch at most one fresh reviewer for an eligible checked package. Close the validated semantic product with `git-change-manifest.py close-package --repo <checkout> --manifest <manifest> --scaffold <scaffold> --analysis <analysis> --review <review> --production-ref <refs/heads/main-or-master-or-refs/remotes/origin/main-or-master> --analysis-date <frozen-YYYY-MM-DD> --output <package.json>`, then persist that complete finalized, redacted package with `checkpoint-package --state-root <root> --run-id <run_id> --repository <repository> --artifact <package.json>`. Closure re-resolves that same canonical production ref and rejects it when it no longer equals the analyzed `new_oid`; a stale or missing local branch does not override the frozen `origin` ref. Every write-ready package requires its independent review; a cursor fallback may close without one. A digest-only, unreviewed write-ready, stale-source, sensitive, or structurally invalid artifact is not a checkpoint.
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

## Documentation limits

Publish only claims authorized by the sealed gate grants and supported by production evidence. `traceability-only` has no claim and therefore produces an acknowledgement, never a Markdown write group. Every granted node must be covered by the unit projection. Do not put run, review, gate, grant, package, or cursor process language in durable technical notes. An acknowledgement contains only its gate-bound closed cursor fields; it contains no claim text, findings, source bytes, or sensitive values.

An accepted `traceability-only` package for an existing repository closes as `no-documentation-change` (`acknowledged-no-change` in inventory): the reviewed delta requires no documentation update and the note keeps its previous baseline. An accepted new repository without a durable node closes as `no-durable-node`. Rejected or incomplete reviews and extraction fallbacks retain `review-rejected` or `inspection-limited`; these outcomes do not establish that documentation is current. Preserve historical acknowledgements unless a newly authorized inspection supplies a new gate decision.

The coordinator, kernel, and durable state share one trusted OS principal.
Package hashes prove integrity and lineage, not authorship against that same
principal. Write authority requires the bounded independent review artifact;
cryptographic authentication would require an external signer and is outside
this local workflow.

## Completion criterion

Complete only after all inventory items have a persisted decision, every unit is applied, the durable receipt exists, the second inventory has no repeated inspected SHA, relevant vault checks pass, and source checkouts remain unchanged.

Report the run closure separately from its documentation outcome. For each repository, give the inspected commit and one result: documentation updated (with written paths), reviewed with no documentation change required, or inspection limited/review rejected (with the unresolved gap and retained note baseline). A zero `changed`/`new` count means no uninspected source delta; it does not mean every note was updated or every limitation resolved. Summarize accepted no-change and limited/rejected counts separately.
