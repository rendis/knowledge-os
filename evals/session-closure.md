# Framework review — decisions and remaining work

Recorded 2026-09-16 after the user accepted retaining 0.10.2. This is a distribution-only session closure, not a new consumer instruction or a release.

## Accepted decision

Keep the 0.10.2 behavior and tools. The framework's purpose is shared, extensible team workflows and dependable information for decisions. Model discretion operates within evidence, authorization and output-quality requirements. Unrestricted autonomy and minimal token counts are not goals in themselves.

Retain evidence-backed claims, proportional answer review, clear concise language, deterministic checks, direct reading for known sources, graph discovery where relevant and selective delegation. Do not remove a quality control merely to reduce context. The efficiency campaign found no consistently superior replacement for the current overall policy and does not establish an absolute optimum or monetary savings. The user explicitly accepted this decision after reviewing the results.

Reopen a specific decision only for a reproduced defect, a material capability/workflow change, or a measured alternative that preserves quality on relevant cases. There is no standing obligation to keep rerunning the same successful tests or redesign the harness.

## Reconciliation of the session agreements

| Agreement | Delivered state and evidence |
| --- | --- |
| Facts before conclusions; correct unsupported user premises; resolve locally answerable subclaims rather than hiding behind uncertainty | Implemented in 0.10.0. [Response-quality campaign](response-quality/results.md) and [breadth campaign](response-quality/breadth/results.md) preserve failures, corrections and final acceptance. |
| Review answers when new evidence or operational conclusions require it; reuse applicable prior reviews; avoid unnecessary review for bounded literal work | Implemented in `kernel/90-Meta/response-quality.md`; tested for operational decisions, missing reviewers, stale review evidence and changed scope. Independent review is a quality control, not a truth guarantee. |
| Formal, direct, understandable language | Included in the response rubric. Tested through reviewer assessment and output constraints; no formal ISO conformity or real-user comprehension claim. |
| Recommend suitable models/efforts and economical subagents without making examples an allowlist | Implemented in `kernel/90-Meta/execution-profiles.md`, including personal overrides, current-user precedence, explicit availability and selective delegation. No general cross-model cost/quality equivalence is claimed. |
| Reduce mechanical LLM coordination while retaining publication safeguards | Existing `review-note-candidate.py` freeze/check/verify-published commands and `sync-run.py` next-action/missing-input output were extended and tested. No additional universal orchestration wrapper was selected. [Write-flow evidence](response-quality/breadth-write-results.md). |
| Audit and justify the 22 installed executables; correct and simplify where contracts permit | Completed in 0.10.1. All 22 retained; quality-report completeness, configuration parsing, graph ambiguity/hygiene, tracked-source scanning, closure uniqueness and next-step consistency corrected. [Tooling evidence](tooling/results.md). Larger refactors were deliberately not selected without a demonstrated need. |
| Resolve source identity before domain reads and handle ambiguity/missing configuration without bypasses | Corrected in 0.10.2; [8/8 scoped observable source-access cases accepted](source-access/closure.md). |
| Preserve acceptance criteria and continue authorized work until scoped completion | The requested global stopping/continuity guidance is present in the current local Codex AGENTS.md. This is local execution guidance, not a distributed kernel payload. |
| Measure tokens and time instead of treating bytes or smaller principal context as total savings | Completed in the [efficiency comparison](efficiency/results.md): 48 outcomes, 104 measured CLI phases, runtime review/repair costs and independent scoring. No kernel change selected. |
| Version and propagate the accepted functional changes without carrying unrelated work | Completed through local release/consumer commits below. No remote push was performed as part of those releases. |

## Local release record

| Version | Distribution | Cell A consumer | Cells B/C consumer |
| --- | --- | --- | --- |
| 0.10.0: response quality and workflow helpers | `61a8fe7` | `8c9087a` | `453cec4` |
| 0.10.1: installed-tool corrections | `715beda` | `fc165e5` | `abfec73` |
| 0.10.2: source resolution and recovery | `ca5a698` | `56d1d51` | `33c81a4` |

The subsequent distribution commit `7ba0c24` records the 0.10.2 closure; it does not change consumer payload. Release-time strict doctor, audit, links and Bases checks are recorded in the [source-access closure](source-access/closure.md). This record is not a claim that live production systems were tested or that current unrelated consumer work is clean.

## Remaining work

1. **Repository closure completed:** this local eval-only commit records the efficiency suite/results, consolidated decision and release follow-up notes in older eval reports. No version bump or consumer propagation is required.
2. **Conditional follow-up, not a release blocker:** the rejected fixed-delegation alternative produced two confusing syntheses and one reviewer preflight with missing captured output. They remain failures/unknown evidence in the comparison; they are not missing implementation in the selected 0.10.2 baseline. Before reconsidering that alternative, improve the experiment's final-answer separation and capture, then repeat the affected cases under unchanged quality criteria. Do not count the existing outcomes as passes or add global rules without evidence.

There is no other agreed functional implementation outstanding in this session. Comparing cheaper models, broader real-workload economics, long-run reliability or other harnesses remains optional future investigation, not a promise established by these results. Remote push/publication is also outside this closure; local versioning/propagation was the recorded delivery.

Later kernel routing thinning and the type-census rule are recorded in [kernel-thinning-closure.md](kernel-thinning-closure.md) (0.10.4 and 0.10.5). That file does not change the 0.10.2 decision above.
