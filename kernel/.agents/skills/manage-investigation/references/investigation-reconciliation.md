# Investigation reconciliation

Use this runbook whenever two contributors changed the same investigation, even when Git reports a clean textual merge. Git resolves lines; it does not resolve meaning.

## Inputs

Bind three complete public snapshots: the common base, canonical current, and contribution. Verify the same investigation ID and preserve the private overlay outside the comparison. Compare `base -> canonical current` and `base -> contribution` across Current state, evidence, questions, decisions, acceptance criteria, handoffs, readiness, and History.

## Semantic review

Build one candidate containing both deltas. For every changed claim or register entry, classify it as compatible, duplicate, superseding, or contradictory. Preserve stable identifiers and both evidence trails. A decision may be replaced only by creating a new decision that explicitly supersedes the old one. Do not accept a candidate while any contradiction is implicit, silently discarded, or represented only by Git conflict markers.

Review the entire combined candidate, not only conflict hunks. Re-run public/private classification and the credential exclusion gate. The reviewer may be a human or an explicitly assigned agent, but the reviewer must see the common base and both deltas.

## Acceptance

Record one History event identifying both source revisions, the reviewed semantic conflicts, and their explicit resolutions. Apply the candidate with the helper's `save` command using the SHA-256 of canonical current. A stale-snapshot result restarts reconciliation from the new canonical snapshot.

Reconciliation is complete only when the full combined case validates, all contradictions have an explicit disposition, public remains understandable without private context, and no source delta was accepted merely because the textual merge was clean.
