# Confirmation trace audit

Scope: all 12 completed synthetic confirmation outcomes, parent and independent-review phases; no final semantic-answer scoring. Evidence: `${CONFIRMATION_CAMPAIGN}/evidence/<case>/<round-arm>/<phase>/events.jsonl`.

| Outcome | Commands/nonzero exits by phase | Parent graph queries | Captured resolved statuses | Persistent changes |
| --- | --- | --- | --- | --- |
| holdout-impact/r01-alternative | parent 5/1; review 8/1 | 0 | parent True; review True | [] |
| holdout-impact/r01-baseline | parent 6/1; review 6/1 | 3 | parent True; review True | [] |
| holdout-impact/r02-alternative | parent 11/1; review 5/0 | 0 | parent True; review True | [] |
| holdout-impact/r02-baseline | parent 10/0; review 8/1 | 3 | parent True; review True | [] |
| holdout-impact/r03-alternative | parent 6/1; review 8/1 | 0 | parent True; review True | [] |
| holdout-impact/r03-baseline | parent 10/1; review 6/1 | 1 | parent True; review True | [] |
| holdout-known/r01-alternative | parent 7/1; review 7/0 | 1 | parent True; review True | [] |
| holdout-known/r01-baseline | parent 7/2; review 3/0 | 0 | parent True; review True | [] |
| holdout-known/r02-alternative | parent 5/0; review 3/0 | 1 | parent True; review True | [] |
| holdout-known/r02-baseline | parent 5/1; review 6/1 | 0 | parent True; review True | [] |
| holdout-known/r03-alternative | parent 7/1; review 3/1 | 1 | parent True; review True | [] |
| holdout-known/r03-baseline | parent 9/2; review 5/1 | 0 | parent True; review True | [] |

## Findings

- All parent and review phases expose resolver status=resolved before the observed domain content reads. There are no missing captured preflight statuses in this confirmation campaign.
- Route contrast holds in all three repetitions: holdout-known baseline reads the supplied UI note directly with zero graph queries; alternative performs one neighbors query before reading it. Holdout-impact baseline performs 3, 3 and 1 parent graph queries; alternative performs zero, using rg content searches and decisive note reads. These establish strategy adherence, not quality parity or savings.
- Every reviewer reads decisive UI metadata for holdout-known, and producer/consumer/topic evidence for holdout-impact. Final answer semantic quality is deliberately unscored here.
- No nested-agent/connector items, shell-launched Codex workers, network commands or persistent changes are observed. All changed-file lists are empty. Some bootstrap invocation, absent-path, inventory-argument and verify-links checks fail; those costs remain included. Verify-links failure is a hygiene check result, not a source-access failure.
- Exact accounting reconciles all 24 independent phase event records to phase summaries, phases to 12 outcome totals, and outcomes to campaign input/cached/output totals. No usage is missing. Completed command scope: 156 commands, 20 nonzero shell exits; individual earlier failures can be hidden by a later successful statement.
- trace_verified means the listed observed procedural checks passed; it does not certify every prescribed reference-loading order, effective model metadata or semantic correctness. Requested Sol/medium remains explicit; effective settings are unexposed.

Mechanical audit exceptions: []

Main development gap remains: two-repositories/r02-alternative/review has five completed commands, a single resolver invocation with empty aggregated_output and no later resolver or canonical doctor. Successful resolution remains unknown; it was not rerun.
