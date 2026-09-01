# Normalize Jira evidence

Load `work-item-evidence.md` first. This reference adds only Jira-specific identity and relationship rules to that shared read-only contract.

## Inputs

Require all of the following before reading Jira:

- one configured Jira tracker ID plus exact issue key, or one canonical issue URL that resolves to exactly one configured Jira tracker;
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

## Jira normalization

Map the observed Jira item into the shared work-item snapshot. Set `tracker ID`, `provider: jira`, canonical tracker URL, and provider-native reference from the exact configured tracker and observed issue. Expose only the facts required by the caller:

- canonical site, key, URL, type, status, and update timestamp;
- requested material fields with their observed field identities;
- selected comments, evidence, and attachments with stable identity, timestamp, non-sensitive summary, and access limitation;
- live link metadata and exact inward/outward endpoint observations;
- one-hop related issue facts selected by the caller's contract;
- observation method, timestamp, redactions, contradictions, and unavailable evidence.

Set relationships to `observed` only when the live metadata and issue-scoped direction were read successfully. Use `unavailable` for denied, partial, or directionally ambiguous Jira relationship reads. Jira supports typed relationships, so this reference never returns `unsupported` for that surface.

Keep the snapshot in the current vault-side interaction. This reference creates no Jira write, operation ledger, case patch, handoff file, cache, or second evidence store. Jira reads grant no external-write authority.

## Completion criterion

The read is complete when the exact issue identity is current, every requested fact is observed or explicitly unavailable, every relationship retains its live type and direction, sensitive material is minimized, and the caller can distinguish Jira evidence from copied baseline context and local inference.
