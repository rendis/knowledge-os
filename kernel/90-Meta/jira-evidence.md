# Read Jira evidence

Use this shared contract whenever a vault workflow needs current Jira evidence without creating an operational run or acquiring write authority.

## Inputs

Require all of the following before reading Jira:

- one canonical Jira site plus exact issue key, or one canonical issue URL from which both are derived;
- the vault workflow and decision that require the evidence;
- the smallest material field, comment, attachment, history, or relationship set needed for that decision;
- an observation timestamp captured for the read.

An absent or ambiguous identity blocks the read. Do not select a site, project, issue, or relationship from textual similarity.

## Read-only procedure

1. Use an already connected Jira capability only for reads. Do not create `.operations/` state, select an operational runbook, or invoke an effect workflow.
2. Read the source issue's current key, URL, type, status, summary, description, acceptance criteria, requested material fields, update timestamp, and only the comments, evidence, or attachments relevant to the stated purpose.
3. Redact secrets, credentials, private personal data, and sensitive attachment contents. Preserve only non-sensitive identity, location, behavioral relevance, and the fact that restricted material exists.
4. When relationships matter, read the live link metadata and issue links. Preserve the exact link type name, inward description, outward description, endpoint keys, and issue-scoped direction.
5. Follow at most one observed relationship hop. Read a related issue only when the caller's contract selects that exact typed edge and direction; never infer dependency from hierarchy, labels, prose, shared components, or proximity.
6. Record unavailable fields, denied content, stale timestamps, missing link metadata, and partial reads as explicit limitations. Do not substitute cached text, a copied handoff snapshot, a case narrative, or memory for current Jira state.

## Normalized in-memory snapshot

Expose only the facts required by the caller:

- canonical site, key, URL, type, status, and update timestamp;
- requested material fields with their observed field identities;
- selected comments, evidence, and attachments with stable identity, timestamp, non-sensitive summary, and access limitation;
- live link metadata and exact inward/outward endpoint observations;
- one-hop related issue facts selected by the caller's contract;
- observation method, timestamp, redactions, contradictions, and unavailable evidence.

Keep the snapshot in the current vault-side interaction. This contract creates no Jira write, operation ledger, case patch, handoff file, cache, or second evidence store. The calling workflow decides how normalized facts affect its own state; Jira reads do not grant that workflow any external-write authority.

## Completion criterion

The read is complete when the exact issue identity is current, every requested fact is observed or explicitly unavailable, every relationship retains its live type and direction, sensitive material is minimized, and the caller can distinguish Jira evidence from copied baseline context and local inference.
