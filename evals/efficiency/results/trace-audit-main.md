# Corrected campaign r01 trace audit

Completed outcomes only. Raw evidence: `${VALID_CAMPAIGN}/evidence/<case>/r01-<arm>/<phase>/events.jsonl`. No final semantic answer scoring. Counts use completed command events once.

| Outcome | Commands/nonzero exits by phase | Graph queries by phase (help excluded) | Official successful loads | Changed files |
| --- | --- | --- | --- | --- |
| ambiguous/r01-alternative | parent 10/1; review 7/1 | none | parent 1; review 1 | [] |
| ambiguous/r01-baseline | parent 15/3; review 9/2 | parent 2 | parent 1; review 1 | [] |
| dependencies/r01-alternative | parent 8/1; review 4/0 | none | none | [] |
| dependencies/r01-baseline | parent 8/1; review 6/1 | parent 4 | none | [] |
| false-premise/r01-alternative | parent 6/1; review 3/0 | parent 1; review 1 | none | [] |
| false-premise/r01-baseline | parent 10/0; review 6/1 | parent 2; review 1 | none | [] |
| known-file/r01-alternative | parent 9/0; review 6/2 | parent 1 | none | [] |
| known-file/r01-baseline | parent 4/0; review 3/0 | none | none | [] |
| reuse-investigation/r01-alternative | parent 7/3; review 7/0 | none | parent 1; review 1 | [] |
| reuse-investigation/r01-baseline | parent 5/0; review 4/1 | none | parent 1; review 1 | [] |
| two-repositories/r01-alternative | child-01 15/1; child-02 6/0; parent 3/0; review 3/0 | child-01 1; child-02 1 | child-01 1 | [] |
| two-repositories/r01-baseline | parent 14/1; review 3/0 | parent 3 | none | [] |

## Material findings

- Corrected access is demonstrated: reuse baseline and alternative parent/reviewer all load the official case successfully (`status: loaded`, exit 0), then read investigation.md and the learning. No lock PermissionError remains in these phases. Transient helper locks disappear and before/after fingerprints report no changes. Alternative parent recovers two invocation errors (missing root, then wrong root); their cost remains included.
- Actual known-file contrast: baseline parent directly reads the supplied note without graph; alternative parent executes neighbors on routing-planner before reading it. Actual dependencies contrast: baseline parent executes four neighbors queries; alternative parent uses targeted rg content searches and note reads without graph. Independent reviewers inspect decisive notes in both arms. These are realized route contrasts, not proof either strategy saves cost or preserves semantic quality.
- Reuse has the same official-load-and-read route in both arms. This is valid observed strategy convergence; it cannot support a reuse-versus-reopening advantage. False-premise both arms use graph, with two versus one parent queries; this is a focused-premise hint comparison rather than graph versus direct.
- Two-repositories alternative has two separately captured child phases, synthesis and review. Child01 explores additional investigation evidence but successfully loads it before direct case reading. Parent rereads planner/ingest/topic/Dispatch to verify synthesis; that overlap is real coordination expense, not missing telemetry.
- Review evidence access is real: known-file planner; dependencies planner/topic/ingest/UI; reuse case+learning; false-premise planner/API/Maps and supporting topic/flow evidence; two-repositories planner/ingest/topic/Dispatch; ambiguous alternative planner/case/learning/flow. Review success prose is not treated as blind semantic acceptance.
- Resolver status=resolved precedes observed domain-content reads in the audited phases. Reference-loading ordering is still imperfect: some workers execute resolution before loading vault-resolution.md. This is an adherence deviation; successful identity resolution alone does not certify every routing instruction.
- No nested agent/connector calls, shell-launched Codex workers, network commands or successful unauthorized authored mutations appear in the completed traces. Only fixture reads, local helper execution and bounded bootstrap executable discovery were observed. Every completed outcome reports changed=[]. Investigation locks are transient local helper effects expressly accommodated by this campaign.
- Remaining command failures are recovered shell parsing, absent AGENTS.personal.md, missing executable alias or missing/wrong helper arguments. Report included recovery cost; do not silently exclude failed commands. Known-file alternative reviewer first invokes nonexistent python, then recovers to python3.
- Requested Sol/medium is explicit for all phases; effective settings remain unexposed. Shared cache, fixed arm order within repetitions and concurrent case pairs limit causal latency/cache claims. Current evidence supports a controlled synthetic hinted-strategy comparison, not unrestricted native-agent operation or global optimality.


Completed r01 outcomes captured: 12. Later completions and repetitions are outside this audit.

## Rounds 2 and 3 supplement

Completed command events only; no instruction-dump rereading or semantic answer scoring. Holdout traces excluded.

| Outcome | Commands/nonzero by phase | Graph query calls by phase | Official successful/failed load invocations | Changed |
| --- | --- | --- | --- | --- |
| ambiguous/r02-alternative | parent 15/1; review 6/0 | parent 1 | parent 1/0; review 1/0 | [] |
| ambiguous/r02-baseline | parent 18/2; review 6/0 | parent 1 | parent 1/1; review 1/0 | [] |
| ambiguous/r03-alternative | parent 9/1; review 7/1 | parent 3 | parent 1/1; review 1/2 | [] |
| ambiguous/r03-baseline | parent 8/0; recheck 7/0; repair 5/0; review 6/0 | parent 3; review 1 | parent 1/1; recheck 1/0; repair 1/0; review 1/0 | [] |
| dependencies/r02-alternative | parent 9/0; review 4/0 | none | none | [] |
| dependencies/r02-baseline | parent 5/0; review 5/0 | parent 4 | none | [] |
| dependencies/r03-alternative | parent 7/1; review 5/1 | none | none | [] |
| dependencies/r03-baseline | parent 19/1; review 6/1 | parent 4 | none | [] |
| false-premise/r02-alternative | parent 6/0; review 6/1 | review 1 | none | [] |
| false-premise/r02-baseline | parent 12/0; review 5/1 | parent 2 | none | [] |
| false-premise/r03-alternative | parent 5/0; review 4/0 | parent 1; review 1 | none | [] |
| false-premise/r03-baseline | parent 12/1; review 4/0 | parent 1; review 1 | none | [] |
| known-file/r02-alternative | parent 6/0; review 4/0 | parent 1 | none | [] |
| known-file/r02-baseline | parent 5/0; review 6/1 | none | none | [] |
| known-file/r03-alternative | parent 6/1; review 5/2 | parent 1 | none | [] |
| known-file/r03-baseline | parent 4/0; review 4/0 | none | none | [] |
| reuse-investigation/r02-alternative | parent 9/1; review 4/0 | none | parent 1/0; review 1/0 | [] |
| reuse-investigation/r02-baseline | parent 4/0; review 7/1 | none | parent 1/0; review 1/0 | [] |
| reuse-investigation/r03-alternative | parent 3/0; review 4/0 | none | parent 1/0; review 1/0 | [] |
| reuse-investigation/r03-baseline | parent 3/0; review 7/1 | none | parent 1/0; review 1/0 | [] |
| two-repositories/r02-alternative | child-01 10/0; child-02 11/0; parent 3/0; review 5/0 | child-01 1; child-02 1 | none | [] |
| two-repositories/r02-baseline | parent 16/2; review 3/1 | parent 3 | none | [] |
| two-repositories/r03-alternative | child-01 16/2; child-02 6/0; parent 5/1; review 6/1 | child-01 1; child-02 1 | child-01 1/1 | [] |
| two-repositories/r03-baseline | parent 12/1; review 9/0 | parent 2 | none | [] |

Rounds 2/3 scope: 24 outcomes, 384 completed command events, 27 nonzero command exits. Full usage reconciliation scope: 36 completed development outcomes across r01-r03.

### Material findings from the supplement

- Realized strategies hold in every repetition: known-file baseline parent makes zero graph queries and alternative exactly one; dependencies baseline parent makes four graph queries and alternative zero, using rg and direct note reads. Reuse remains official-load/read in both arms.
- Official case loading now succeeds wherever recovery is required. Rounds2/3 contain recoverable missing/wrong --root invocations in ambiguous/r02-baseline parent, ambiguous/r03-alternative reviewer and two-repositories/r03-alternative child01. Each later obtains status=loaded. There are no lock PermissionError or cache-write-denial traces in these rounds.
- Reviewer command captures contain the relevant decisive evidence for every rounds2/3 outcome: planner for known, planner/topic/ingest/UI for dependencies, case plus learning for reuse, planner/API/Maps for false premise, planner/ingest for two-repositories, and case content for ambiguous. This verifies evidence access only.
- One preflight observability gap: two-repositories/r02-alternative reviewer runs resolve-vault.py with exit 0, but captured aggregated_output is empty; it then reads notes. Invocation is proven, successful canonical-resolution status is not exposed in that phase. Do not substitute exit 0 for resolved-status evidence. All other audited domain phases expose prior resolved output; earlier reference-order adherence caveat remains.
- Across all 36 development outcomes: 565 completed shell commands and 47 nonzero shell exits. These counts include recoveries; a multi-statement shell command can hide an earlier statement failure behind final exit 0, so counts are not the total number of failed individual statements.
- Across all 36 outcomes, every changed-file list is empty; no unexpected nested-agent/connector items, shell-launched Codex workers or network commands were found. Workspace-write permits transient official helper locks, which are cleaned up; no persistent mutation is observed.
- Exact accounting scope: every captured phase usage matches its turn.completed record, all phases sum exactly to their outcome usage, and all 36 outcome usages sum exactly to campaign input/cached/output totals. No usage is missing. Task wall time differs legitimately from summed phase time for concurrent children.

Full completed development campaign audited: 6 cases × 2 arms × 3 repetitions = 36 outcomes. Rounds2/3 supplement covers 24 outcomes and preserves r01 findings above. Separate holdout campaign was not inspected. Blind semantic acceptance was not evaluated.
