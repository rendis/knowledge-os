# Deduplication and consolidation

Load this reference before opening a case or consolidating duplicate cases.

## Serialized open gate

Opening is a check-and-create transaction. The helper owns filesystem serialization; the agent owns semantic equivalence.

1. Derive one stable lowercase hyphenated `dedupe-key`, a canonical ID, and any non-generic source references.
2. Resolve possible semantic matches yourself before writing. Present ambiguous candidates to the user; do not delegate equivalence to the helper.
3. Run `investigation-case.py --root <root> open ...`. It acquires the bounded `.open.lock`, checks exact active IDs, lineage, dedupe keys, source references, and normalized titles, then creates through an atomic rename only when there is no exact match. Route `definite_match` to **Resume**.
4. Treat `case_root_locked` or failed lock cleanup as a blocker. Never remove a lock without explicit authorization.

The open gate is complete when the search was performed while creation was serialized, the request either resolves to an existing canonical case or creates exactly one new case, and the gate no longer exists.

## Consolidation protocol

Consolidation preserves lineage in one directory; a retired ID remains resolvable through the canonical case's `consolidated-from`.

1. Require explicit authorization for the exact case IDs. Run the normal preflight, then read every source `investigation.md` completely. Load the export contract when any source has drafts.
2. Select the canonical case explicitly. Default to the oldest `created-at` only when the user did not select one and no lifecycle or evidence condition makes another case authoritative.
3. Classify retiring content before copying it. Preserve a byte-for-byte public snapshot in canonical `artifacts/` only when every byte is shareable; otherwise preserve a formalized lineage mapping without the sensitive bytes. Reconcile drafts through the export contract instead of treating them as evidence.
4. Reconcile into the canonical case:
   - keep every existing canonical identifier and meaning unchanged;
   - map equivalent source items to canonical items without duplicating them;
   - inspect the direct in-scope source again before registering a unique source claim;
   - assign the next canonical identifier to unique questions, decisions, criteria, attachments, or verified evidence;
   - reconcile development handoffs by exact story-and-repository identity: preserve an existing canonical `DH-NNN`, allocate the next ID for a unique target, and block when two entries disagree on branch, handoff ID, family, or current revision;
   - reconcile both private overlays into the canonical overlay, or delete the retiring overlay with a digest-bound `save` after proving it contains no unique necessary context; never leave an overlay under a retired ID;
   - record the source-to-canonical mapping with the lineage attachment and in History;
   - compare the prior learning assessment with the combined evidence and context. Mark it `preserved` only when the combined snapshot cannot change the prior assessment; otherwise mark it `reset`.
5. Update the canonical Current state, `updated-at`, affected registers, draft synchronization states, and append a consolidation event. Add every retired ID to `consolidated-from`. When learning assessment is `reset`, preserve its prior value in History and set `learning-outcome: not-evaluated`; when it is `preserved`, leave the value unchanged.
6. Write `artifacts/consolidation-<retired-id>-mapping.md` in the canonical case. It must contain `retired-id: <id>`, `drafts: none|reconciled|stale`, `learning-assessment: preserved|reset`, and the agent-reviewed identifier mapping.
7. Run `investigation-case.py --root <root> consolidate --canonical <id> --retire <id> --expected-canonical-sha256 <reviewed-sha> --expected-retire-sha256 <reviewed-sha>`. The helper rejects either stale snapshot, archives the exact retired tree, updates lineage and History, validates, and rolls back on failure.

Consolidation is complete when the canonical snapshot is current, no material source item or handoff is lost, exports and private overlays are reconciled, no private directory remains for a retired ID, and the equivalence group has exactly one public case directory.
