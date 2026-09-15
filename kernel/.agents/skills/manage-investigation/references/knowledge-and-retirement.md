# Absorb useful knowledge; retire only when safe

Closing, absorbing knowledge and retiring a case are independent decisions. Assess useful knowledge at closure and whenever a sufficiently evidenced contribution emerges; no case must live forever and no closed case is automatically deleted. Keep the current purposes and three global states.

## Selective absorption

1. Select bounded claims, not the whole narrative. Ask whether retaining each claim avoids meaningful ambiguity, repeated investigation or future error; difficulty of discovery alone is not the criterion. Classify as useful new/corrective knowledge, already represented, no durable value, or insufficiently supported. Insufficient support is not no value.
2. Inspect the evidence behind each claim: repository revision, environment, measurement, decision, user-supplied document or correspondence. Record source identity/date/location and what it establishes. A decision can support why a choice was made; it cannot prove deployment or functional success. Preserve limits, disputes, access restrictions and redaction. Supplied private correspondence is not automatically shareable.
3. Search canonical notes for the target and existing coverage. Use **Promote**/`map-ecosystem` for technical knowledge and **Learn**/`manage-investigation-derived-learning` for reusable lessons, retaining their authorization, evidence-profile and independent-review gates. Claim subsets can proceed while the rest remains open. Neither route closes or retires the case.
4. Incorporate only the supported delta where it belongs. Preserve reasons and constraints that code cannot provide, with independently usable provenance. Before a source case/local operational run disappears, keep an authorized sanitized durable evidence summary or an accessible stable source with sufficient support in the destination. A private overlay, inaccessible email, ignored `.operations/` path or bare case link cannot be the only support for a shared conclusion. If no safe durable source exists, retain the limitation and do not label the claim absorbed.
5. Return verified destinations, claim/register IDs, source boundaries and review results to the case owner. With case-write authority, update existing outcomes and History through `save`; publish neither private context nor cycle-by-cycle observation noise. A summary saying “absorbed” without verified destination content is not completion.

## Retirement review

Require explicit authorization and `closed`. Retirement is administrative, not a reopen. Review the **entire public directory**, including exports and attachments, and confirm:

- Valuable knowledge is incorporated and verified, already exists with usable evidence, or has a concrete no-value rationale. Pending useful absorption must be resolved or explicitly retained elsewhere, not discarded as unsupported noise.
- Observation schedules have ended/cancelled and their disposition is verified. No active handoff, release, work-item or other dependency requires the live directory. Preserve historical handoff provenance; do not delete development worktrees.
- References from retained notes, learning provenance, other cases, exports and operational records remain usable. Keep stable case IDs resolvable as retired; replace live-file dependencies with canonical destinations or explicit historical snapshot references.
- Every public file to remove exists at an exact reachable Git commit and current bytes/modes match it. Modified, untracked or ignored attachments without that snapshot block retirement. With appropriate Git authority, version them first and repeat review; never force, stash, reset or silently exclude them.
- Summary, reason, source and destinations are safe to version. Decide private-overlay disposition separately: the helper never deletes it and Git cannot recover uncommitted private context.

CLI review attestations record the review; they do not mechanically prove inactive schedules or semantic absorption. Inspect the relevant records and supported scheduler state first. Recheck after material concurrent changes.

## Apply and verify

Use `investigation-case.py --root <VAULT_ROOT>/investigations retire` with explicit authorization, reviewed public SHA-256, exact snapshot commit, concise summary/reason/source, dependency and absorption reviews, and destination references. Inspect `retire --help` for supported arguments. The helper owns the shared lock, stale-snapshot/Git-content checks, recorder attribution, minimal `retired.md` entry and recoverable public removal. It never commits or changes the private overlay.

Version the public deletion and retirement entry **together**, with explicit Git authority. Inspect the staged diff: only the reviewed removal/record and separately authorized changes belong in that commit. Re-run helper `load --id <id>` and verify the deletion commit before declaring retirement complete. Pending commit is not completed retirement; reconcile or recover a failed/interrupted step instead of blindly repeating deletion. Git and filesystem changes are not one atomic transaction.

## History and lookup

Use helper `list` for a derived overview of present cases and retired entries. Active statuses come from case files; retirement is storage disposition, not a fourth state or a second active index. The minimal retirement entry retains identity, short summary, snapshot/provenance, reason/reviews and destinations, never a duplicate archived case tree.

Use helper `load` for exact current or historical IDs. Report `retired` with destinations/snapshot and verified deletion commit when available, instead of missing. Missing/shallow Git history remains an explicit historical-access limitation; never fetch or restore without authority. Inspect historical evidence at the exact snapshot in read-only scratch, not by recreating the active case. Historical IDs remain reserved. A stale branch restoring a retired directory requires explicit semantic reconciliation; a clean textual merge does not authorize resurrection.

Complete retirement only after public removal and minimal record are versioned together, historical lookup works, retained knowledge/references no longer require removed live files, and private disposition is separately handled or explicitly left untouched.
