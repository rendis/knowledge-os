# Bounded observation across sessions

Use the Audit branch for an agreed observation campaign; this is not a new scheduler or lifecycle. Default effects are obtaining evidence, evaluating it and reporting. Use `execution-record.md`, `read-only-audit.md` and the selected procedures. Read the evidence-analysis method before interpreting results.

## Agree one persistent plan

Consult applicable observation/access guides before proposing coverage. Ask the user to validate a concise plan in `.operations/<run-id>/run.md` with:

- Existing run identity, plan revision and approval source; source investigation/component IDs when applicable, exact target revisions and environments.
- Procedures and permitted sources, expected signals and their coverage, known access gaps and exclusions.
- Start/end, cadence and timezone, reporting destination, evaluation/stop conditions and who handles decisions.
- Any explicitly authorized extra action: exact condition, destination and effect. Absence means report only.

Use the existing operational states. Preparing is `draft`, unapproved effects use `awaiting-approval`, an authorized campaign uses `in-progress`, and `completed`/`cancelled` are terminal. A `blocked` campaign may recheck only its named access/dependency at the next agreed cycle; it does not expand scope. Record each cycle as steps/observations within this campaign, bounded by its own interval; do not create a parallel run store or duplicate plan per cycle. A changed plan increments its revision and preserves prior observations; material scope/effect changes require user validation.

Validate available access while preparing and disclose coverage gaps. Ask whether missing access will be supplied, evidence provided another way, or a bounded exclusion accepted. Exclusion remains a limitation, not successful verification. No full-flow claim is justified by partial access.

Reuse choices the user already specified; do not request duplicate approval. Any unresolved material choice, such as the first covered interval or a pre-release overlap, remains proposed: ask that concrete question and keep collection pending until resolved. Access-discovery checks may establish feasibility, but are not an approved observation cycle or evidence of the proposed window's outcome.

Offer a scheduled task in a context separate from the user's originating conversation. After user acceptance, use the harness's supported scheduling/session mechanism. The scheduled instruction identifies the canonical vault, run ID and where to load the current plan; it must not become another editable copy of the plan. Keep provider/session handles and local paths in local operational state, not investigation case records. If the scheduler necessarily stores cadence/end too, verify they match the agreed plan after each authorized change; neither copy silently overrides the other. Validate that the selected execution environment can resolve the vault and read the local run. If it cannot, explain the limitation and agree an available workflow; do not build a scheduler, daemon or infrastructure workaround.

## Every execution

1. Resolve the vault and reload the campaign, its current revision, authorization and period. A cancelled/completed campaign performs no source reads. Once the end boundary is reached, perform only final assessment/reporting from collected evidence; never extend observation implicitly. Reconcile any scheduler update/cancellation through its owner.
2. Ensure one writer for the run. Use the harness's non-overlap mechanism, or atomically acquire a local run-directory lock for this bounded execution. If ownership is uncertain, skip/report the overlapping cycle; inspect interrupted work before recovering a stale lock. Do not silently remove another execution's lock. Identify cycles by run, plan revision and covered interval; a completed retry reuses its recorded result rather than duplicating it.
3. **At the start of every new execution context, validate actual access** to required sources with minimum permitted read-only checks: identity, environment, permission and availability. Every scheduled tick opening a fresh session repeats this preflight. A prior access record is not proof of current access. Within one continuous context, revalidate on expiry, errors or relevant changes. Apply source contracts; never copy credential values.
4. Collect only the agreed observations. Record observation time separately from the data's covered interval, freshness, pagination/sampling and missing sources. Continue independent permitted observations when one source fails, but do not label the complete flow healthy. Ask for unresolved access or exclusion decisions through the agreed channel.
5. Compare with the agreed expectations and previous comparable intervals. Distinguish observed conformity, a supported anomaly and inconclusive evidence; lack of access, no traffic, stale data or an empty partial result is not proof of success or failure. A negative release recommendation requires a violated agreed condition, not merely a missing observation.
6. Append a concise cycle result and deliver the agreed summary **even when unchanged**: scope/interval, observed behavior, differences/anomalies, coverage/limits and needed decisions. Keep raw logs, repeated transcripts and sensitive payloads out of the ledger. On reporting failure, preserve the result and reconcile/retry delivery without repeating completed source reads; do not claim the report was delivered.
7. Deliver the agreed report through its authorized reporting channel. For any additional conditional notification or corrective effect, require the user's exact trigger, destination and effect and a supported integration, then hand it to a separate **Execute** operation under its owning procedure. Keep this campaign read-only; even authorized remediation never executes inside Audit. Offer supported notification options during planning; detecting an error is not authorization. Release execution ownership after recording/reconciling the cycle.

## Case updates and end of observation

Reports belong to this operational run. By default, offer material findings for incorporation; do not update the investigation, close it, publish knowledge or retire it. If the user established case-update authority, return only material findings, source identities, times, limits and affected IDs to `manage-investigation`; it owns snapshot checks and attributed writes. Unchanged cycles never create case History noise.

At the agreed end/condition, summarize observed coverage and outcome, including unresolved anomalies and inaccessible evidence. Operational collection may complete with procedure-defined terminal unknowns; this does not establish component stability or satisfy an investigation's unmet criteria. Ask for any extension or change of criteria explicitly. Confirm the schedule is ended/disabled through the supported mechanism; if that is unavailable, report the scheduling limitation and retain the no-source-read terminal guard. Do not report the scheduling lifecycle fully finalized until its disposition is verified. Cancellation ends future reads, not historical records.
