# Offline blind behavior checks

Create the same synthetic fixture from each distribution under anonymous run names:

```sh
python3 -B evals/behavior/prepare.py --distribution /path/to/distribution --output /tmp/run-a --scenario workflow
python3 -B evals/behavior/prepare.py --distribution /path/to/distribution --output /tmp/run-b --scenario onboarding
python3 -B evals/behavior/prepare.py --distribution /path/to/distribution --output /tmp/run-c --scenario lifecycle
python3 -B evals/behavior/prepare.py --distribution /path/to/distribution --output /tmp/run-d --scenario analysis --evaluation-output /tmp/evaluator-d
```

Each output is private evaluation scratch. Start a fresh agent in that output directory, without parent history, and pass only `task.txt`. Use the same supported model, effort, and tool permissions for both variants. Never point these tasks at a real consumer. Fixture setup requires Python 3.9+ and Git and refuses an existing output.

For `analysis`, select one natural request from `tasks/` per fresh fixture and agent. Only the three numbered continuity tasks share a fixture and conversation. The separate evaluator directory contains the rubric, baseline file hashes, and coordinator protocol; never give it to an executor. The absent-overlay variant requires the coordinator to remove the synthetic overlay before the fresh execution. Use the same current generator with `--distribution` pointing at either revision to keep source content and prompts constant. Sources include a local Git repository, simulated paginated cloud output, and a valid public/private case. No live cloud execution is needed. The linked-workflow prompt checks routing only; use the focused handoff/reconciliation/learning tests for execution guarantees. Run `python3 -B evals/behavior/test_analysis_fixture.py` to verify setup isolation, reproducible prompts, overlay discovery, and the synthetic code behavior.

The repository has a synthetic origin and workspace roots configured through the installed CLI, so identity-based resolution is exercised. The `operational` task explicitly executes a registered offline audit and should create its `.operations/` ledger; its procedure defines unavailable second-page and receipt evidence as terminal unknowns for this bounded export. The separate `cloud` task is informational and should not create that ledger.

Run one analysis scenario with the process runner:

```sh
python3 -B evals/behavior/run_analysis.py --distribution /path/to/distribution --output /tmp/analysis-run --evaluation-output /tmp/analysis-evaluator --task diagnosis --model <available-model> --effort low
```

The runner prepares the fixture, removes irrelevant legacy artifacts and answer-bearing notes from diagnostic inputs, and stores results, events, prompt hashes, exit codes and before/after vault fingerprints in the evaluator directory. Independent scenarios use ephemeral sessions. `--task continuity` starts one session and resumes the actual returned thread ID for the next two requests, passing only the new request rather than replaying a transcript. `--variant alternate` changes the numerical diagnostic inputs for a fresh equivalent check. Use equal model, effort and permissions when comparing revisions. `python3 -B evals/behavior/test_analysis_runner.py` checks runner mechanics with real fixture preparation and a fake Codex process; this does not substitute for observed agent runs. Report scenario outcomes and their evidence, without inferring general performance or efficiency from a small comparison.

For the recorded comparison, each independent worker used `codex exec --ephemeral --skip-git-repo-check --ignore-user-config -m gpt-6-astra -c 'model_reasoning_effort="low"' -c 'approval_policy="never"' -s workspace-write --json --output-last-message result.md - < task.txt > events.jsonl`. CLI availability and host permissions are external prerequisites. Do not pass audit conclusions or another worker's results.

Score each requested outcome independently: correct source-based explanation; accepted knowledge investigation closed without a fabricated export; reader contract implemented and tested while writer preserved; cell-specific procedure configured through the supported onboarding path. Confirm changes and executed checks from artifacts and events, not self-reported completion. Use a separate blind reviewer with anonymized copies and state what evidence the reviewer was given.

For `lifecycle`, score each request independently: the global source dependency becomes `blocked` with `blocked-on`; a specification-only development objective closes as `completed` without fabricated implementation or export; new contradictory evidence explicitly reopens a closed case and preserves its former closure in History; an S-002-only gap does not block the bounded S-001 assessment, while the absent current work-item/package remains explicit; and the informal new request opens as concise `investigating` content without profanity, transcript, or unnecessary private overlay. Require helper-mediated, attributed transitions and final structural validation.

The workflow package intentionally uses the existing `handoff-sufficiency/candidate-c.md` fixture unchanged in both variants. It tests recipient implementation sufficiency, not end-to-end handoff materialization or operating-system enforcement of vault read-only access. The onboarding scenario uses non-default environment names, team SSO, and a team executor; no live access is requested.

## Recorded comparison

`2026-09-09-metrics.json` records raw usage and command counts for one run per variant/scenario, plus identical prompt hashes within each pair. Candidate artifacts completed 4/4 requested outcomes; baseline completed 2/4 with honest partial outcomes for closure and onboarding. Both reader implementations passed the same six behavioral scenarios. A fifth fresh Astra-low reviewer independently checked anonymized artifacts and reran their tests; framework execution claims were checked separately against the original events.

Onboarding command calls fell from 10 to 8. Workflow calls stayed at 9 and reported command output bytes increased. Input usage includes repeated/cached prefixes; these are not wall time, cost, or measured filesystem bytes. This small sample supports these concrete behavior changes, not generalized efficiency or statistical claims. Raw runs remain local evaluation artifacts and contain no production data.
