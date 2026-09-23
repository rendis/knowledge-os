# Investigation migration

Use this runbook only when the user asks to upgrade already-published cases or migrate one or more cases from `.investigations/`. The installer never migrates cases and this is not a synchronization mechanism.

Current-schema files already in `.investigations/` are unpublished working cases; mutate them through the normal routes. Only a legacy format in that tree (obsolete statuses, `resume-to`, missing current required fields, or **Original request** without **Request summary**) requires this runbook.

## Convert prior published lifecycle states

An earlier shareable format may already have versioned case files in `investigations/` with `intake`, `scoped`, `validating`, `ready-to-export`, `exported`, a closed case without `closure-outcome`, or `resume-to`. Convert these cases before ordinary published mutations; do not recreate or move them.

1. Load every affected published case, preserve its bytes and SHA-256, and review the semantic state. Process all affected published cases requested for the vault so normal root-wide validation can finish.
2. For `intake`, `scoped`, `validating`, `ready-to-export`, or `exported`, invoke `transition --to investigating` with the loaded SHA-256, migration reason, and portable source. The helper permits one attributed, CAS-guarded conversion at a time and may report that other pre-existing cases still require conversion.
3. For an old `blocked` case with `resume-to`, retain it as blocked only when `blocked-on` still prevents useful progress. Invoke `transition --to investigating` during conversion either way. If it must remain blocked, retain the reviewed dependency in working context and, after every affected published case has been converted, apply a current-snapshot transition back to blocked without `resume-to`.
4. For an old `closed` case without an explicit outcome, invoke `transition --to investigating`. After every affected published case has a valid current state, apply the normal closure gate and `close` it again only when the preserved evidence supports `completed` or an explicit decision supports `abandoned`.
5. Run root validation after the final conversion. Do not continue with save, export, handoff, reconciliation, or closure while another affected published case keeps the root invalid.

## Transform one selected case

1. Read the complete legacy case and its referenced local artifacts. Do not modify it.
2. Preserve its investigation ID, creation time, decision meanings, stable register identifiers, useful chronology, and durable references. Preserve an original recorder only when the legacy source identifies one. Otherwise mark the historical recorder as `not recorded`; never assign legacy entries to the migrating agent.
3. Create the corresponding unpublished case in `.investigations/` unless the user explicitly asks to publish immediately. Replace **Original request** with a professional **Request summary**; omit transcripts, hidden reasoning, irrelevant process chatter, and absolute local paths. Do not silently place a migrated legacy case into `investigations/`.
4. Translate each development location to normalized repository remote plus exact branch. Retain an observed commit only when it identifies evidence. If repository or branch cannot be verified, preserve the reference as an explicit unresolved limitation rather than inventing availability.
5. Apply the normal persistence classification. Generalize sensitive but shareable meaning into the case. Retain case-specific methods and deliverables with reviewed provenance; keep machine-specific configuration, temporary outputs and unreviewed candidates under `.investigations-private/<id>/local/`. Create a private overlay only for necessary sensitive context. Never persist credential values.
6. Map lifecycle conservatively. Legacy `intake`, `scoped`, `investigating`, `validating`, `ready-to-export`, and `exported` become `investigating`; legacy `blocked` remains `blocked` only when a concrete current dependency still prevents useful progress, otherwise it becomes `investigating`. Preserve a legacy closed outcome only when its reason and evidence support `closure-outcome: completed` or `abandoned`; otherwise reopen it as `investigating` and record the uncertainty. Remove `resume-to`. Do not infer completion from an export or handoff.
7. Open the destination as `investigating` and save the reviewed content candidate through the helper so the migration event is attributed to the effective Git identity, timestamp, source legacy case ID, and affected register IDs. This attribution describes the transformation, not historical authorship. Do not put mapped lifecycle fields in the save candidate: when the mapped result is `blocked`, invoke `transition --to blocked` after the content save; when it is `closed`, invoke `close` with the preserved outcome, reason, limitations, evidence IDs, and latest load `public.sha256`.
8. Validate the destination case (unpublished unless the user asked to publish immediately) and, when present, the private overlay. Compare old and new registers explicitly: every decision and reference must be preserved, intentionally superseded, or documented as omitted with reason.

Migration is complete when the new case is understandable without private context, its ID and decision semantics are preserved, structural validation passes, and the legacy bytes remain unchanged. Do not mark, delete, or rewrite the legacy case automatically.
