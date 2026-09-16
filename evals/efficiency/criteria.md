# Efficiency comparison criteria

The original [registered criteria](results/criteria-registered.md) are preserved byte-for-byte with the campaign hash. This copy additionally clarifies exclusion of unobservable preflight results discovered during trace audit; questions, prompts and semantic rubric were not changed. Registered before execution: Distribution: 0.10.2 (`ca5a698`). The separate user guide is excluded. Model requested for every task, child and reviewer: `gpt-5.6-sol`, medium. Effective settings must be reported only when exposed by runtime evidence.

## Design

Eight synthetic Norte Logistics cases, two arms, three repetitions: 48 task outcomes. Six development cases and two reserved confirmation cases. Baseline follows the installed routing; alternative receives a focused strategy instruction. The task question and quality requirements remain identical. This measures an explicit strategy hint, not a shipped policy replacement. Arm order alternates across repetitions; scheduling and shared infrastructure are potential confounders.

The two-repository alternative uses two explicit, independently metered workers plus synthesis. This tests fixed decomposition, not autonomous delegation or cheaper-model selection. All other arms run a single worker. Workers do not recursively delegate. The runner provides an independent evidence review for both arms after the draft, one bounded repair and recheck if needed; these phases are included in task cost and time. This standardized review is stricter than author-only review for simple cases, so absolute costs do not represent unrestricted default operation.

The corrected campaign contrasts graph-assisted lookup against normal direct reading for known files, and directed text search against normal graph discovery for relationships. Trace auditing must confirm actual contrast. Fresh fixture copies and sessions are used for every outcome. No real consumer vault or source repository is an input. Responses must be concise Spanish with decisive source references. No real applications, network operations or content/configuration mutations are authorized. Official read-command transient locks are permitted inside disposable workspaces and must be removed. Infrastructure failures and semantic failures stay in the ledger; any rerun is separately identified.

## Per-case acceptance

| Case | Required content | Unsupported conclusions to reject |
| --- | --- | --- |
| known-file | Builds dispatch plans and publishes route-events; cite routing-planner note; at most two sentences. | Claims of verified runtime behavior. |
| dependencies | routing-planner publishes route-events; fleet-ingest consumes it and updates assignments; fleet-ui reads fleet-ingest; cite relevant notes; distinguish documented chain from actual outage propagation. | Guaranteed transitive operational impact. |
| two-repositories | Compare planner and ingest roles and event connection; dated documentation is not proof of current production functionality. | Calling configuration, coverage or a commit field live functional proof. |
| reuse-investigation | Old GPS overwrites live assignment according to the prior case; learning treats older-than-horizon GPS as stale; investigating status does not prove correction; cite case and learning. | Claim that a documented learning means deployed remediation. |
| ambiguous | Identify planner as a possible referent; ask what 'bien'/closure means as needed; available investigation remains investigating and lacks closure proof. A bounded clarification without domain assertions is acceptable. | Unqualified health or closure confirmation; invented business criteria. |
| false-premise | Direct Maps caller documented is routing-api, while planner publishes route-events; claimed necessary failure chain is unproven; cite decisive notes. | Agreeing with false premise or asserting undocumented coupling is impossible. |
| holdout-known | fleet-ui reads fleet-ingest; coverage fields are por-confirmar and audit date is 1970, so current documentary coverage is unestablished; cite note. | Actual operational failure or current completeness inferred from placeholder metadata. |
| holdout-impact | Publisher routing-planner and consumer fleet-ingest; inspect both for contract change; notes cannot guarantee absence of current operational impact; cite sources. | Unqualified operational safety guarantee. |

Every acceptance requires correct claims, relevant citations, sufficient answer coverage, clear readable language, and no unauthorized effects. A safe clarification is accepted only for the ambiguous case; answering an otherwise fully answerable question with a blocker fails coverage. Source notes are documentation evidence, not live observations.

## Review and measurements

Reviewers receive shuffled opaque answer IDs, the fixture evidence and rubric, without arm names or usage/time. Record pass/fail, critical errors and a concise reason per outcome. Mechanical keyword checks are not semantic acceptance. Inspect execution separately for strategy adherence and preservation. Non-adherence remains a result rather than being silently excluded.

Report input tokens, its cached subset, uncached input, output and task wall time. Independent CLI phases are summed once; parent prompts include child outputs and that consumption is a real coordination cost. Task time includes child execution and synthesis but excludes fixture setup; record setup separately. Missing usage is unknown, not zero. Include failed runs and fixed independent evaluation overhead separately. Internal answer review cannot be presumed observable from prose.

For each case, retain all paired deltas and compare medians only among quality-accepted, telemetry-complete outcomes. A missing decisive preflight output remains an observability gap: semantic acceptance is recorded separately and that pair is excluded from fully verified comparisons. Also report arm pass counts and total expenditure divided by accepted outcomes, including failures. Do not combine input and cached counts twice, translate tokens into currency without valid pricing, or claim statistical equivalence from three repetitions. No absolute optimum or universal saving can be established. An observed benefit must preserve quality and be confirmed on the reserved cases before recommending a routing change; contradictory or mixed outcomes warrant no change.

## Instrument correction before accepted campaign

The initial read-only smoke and interrupted main attempt are excluded from strategy comparisons: investigation load requires a transient `.open.lock`, which the sandbox denied. macOS system Python also emitted cache-write diagnostics. The corrected runner uses workspace-write only for disposable fixtures, permits official temporary locks, supplies a verified Homebrew Python on the fixture PATH, and still requires identical final file fingerprints. The early audit also found same-route arms for direct/graph questions; strategy hints were revised before the accepted campaign to create explicit contrasts. Initial attempts remain retained as setup expenditure, not favorable performance samples. No kernel rule was changed.
