# Offline blind behavior checks

Create the same synthetic fixture from each distribution under anonymous run names:

```sh
python3 -B evals/behavior/prepare.py --distribution /path/to/distribution --output /tmp/run-a --scenario workflow
python3 -B evals/behavior/prepare.py --distribution /path/to/distribution --output /tmp/run-b --scenario onboarding
```

Each output is private evaluation scratch. Start a fresh agent in that output directory, without parent history, and pass only `task.txt`. Use the same supported model, effort, and tool permissions for both variants. Never point these tasks at a real consumer. Fixture setup requires Python 3.9+ and Git and refuses an existing output.

For the recorded comparison, each independent worker used `codex exec --ephemeral --skip-git-repo-check --ignore-user-config -m gpt-6-astra -c 'model_reasoning_effort="low"' -c 'approval_policy="never"' -s workspace-write --json --output-last-message result.md - < task.txt > events.jsonl`. CLI availability and host permissions are external prerequisites. Do not pass audit conclusions or another worker's results.

Score each requested outcome independently: correct source-based explanation; accepted knowledge investigation closed without a fabricated export; reader contract implemented and tested while writer preserved; cell-specific procedure configured through the supported onboarding path. Confirm changes and executed checks from artifacts and events, not self-reported completion. Use a separate blind reviewer with anonymized copies and state what evidence the reviewer was given.

The workflow package intentionally uses the existing `handoff-sufficiency/candidate-c.md` fixture unchanged in both variants. It tests recipient implementation sufficiency, not end-to-end handoff materialization or operating-system enforcement of vault read-only access. The onboarding scenario uses non-default environment names, team SSO, and a team executor; no live access is requested.

## Recorded comparison

`2026-09-09-metrics.json` records raw usage and command counts for one run per variant/scenario, plus identical prompt hashes within each pair. Candidate artifacts completed 4/4 requested outcomes; baseline completed 2/4 with honest partial outcomes for closure and onboarding. Both reader implementations passed the same six behavioral scenarios. A fifth fresh Astra-low reviewer independently checked anonymized artifacts and reran their tests; framework execution claims were checked separately against the original events.

Onboarding command calls fell from 10 to 8. Workflow calls stayed at 9 and reported command output bytes increased. Input usage includes repeated/cached prefixes; these are not wall time, cost, or measured filesystem bytes. This small sample supports these concrete behavior changes, not generalized efficiency or statistical claims. Raw runs remain local evaluation artifacts and contain no production data.
