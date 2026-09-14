# Investigation reconciliation

Use this runbook whenever two contributors changed the same investigation, even when Git reports a clean textual merge. Git resolves lines; it does not resolve meaning.

## Inputs

Bind three complete public snapshots: the common base, canonical current, and contribution. Verify the same investigation ID and preserve the private overlay outside the comparison. Compare `base -> canonical current` and `base -> contribution` across Current state, evidence, questions, decisions, acceptance criteria, handoffs, readiness, and History.

## Semantic review

Build one candidate containing both deltas. For every changed claim or register entry, classify it as compatible, duplicate, superseding, or contradictory. Preserve stable identifiers and both evidence trails. A decision may be replaced only by creating a new decision that explicitly supersedes the old one. Do not accept a candidate while any contradiction is implicit, silently discarded, or represented only by Git conflict markers.

Review the entire combined candidate, not only conflict hunks. Re-run public/private classification and the credential exclusion gate. The reviewer may be a human or an explicitly assigned agent, but the reviewer must see the common base and both deltas.

## Acceptance

Record one concise History event identifying both source revisions, the reviewed semantic conflicts, and their explicit resolutions, including the agreed lifecycle disposition. Separate that disposition from the content candidate: keep `status`, `blocked-on`, `closure-outcome`, and `resume-to` identical to canonical current when calling `save`. Apply the content with the SHA-256 of canonical current, both revisions as the portable source, and every affected stable ID as a target. The helper attributes the reconciliation to the effective Git recorder; that identity is not approval.

When the agreed lifecycle differs, load [readiness-and-lifecycle.md](readiness-and-lifecycle.md). After the content save, load the new snapshot and apply the explicit `transition` or `close` with that SHA-256, both source revisions, and the reviewed reason and evidence. Follow valid transitions: changing a closed outcome requires reopening before a new closure; changing a blocked dependency requires unblocking before blocking with the new dependency. Load a fresh snapshot between each operation. A lifecycle-only contribution can omit the content save and record both revisions in the lifecycle event.

A stale snapshot restarts semantic review from the new canonical state. These are separate transactions: if content succeeds but a lifecycle step fails or execution is interrupted, retain the attributed content, report reconciliation as incomplete, and re-review the pending disposition against the latest snapshot before resuming. Accept only after the final state and content match the reviewed disposition and validation passes.

Reconciliation is complete only when the full combined case validates, all contradictions have an explicit disposition, public remains understandable without private context, and no source delta was accepted merely because the textual merge was clean.
