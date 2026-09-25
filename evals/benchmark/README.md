# Harness benchmark

Compares harness settings (harness, model, effort) on the work a cell actually does: answering evidence
questions from its vault and publishing a repository sync. The result is the measured minimum setting per
task shape recorded in `kernel/90-Meta/execution-profiles.md`, the table users start from.

## Protocol

Fairness comes from holding everything except the setting constant:

1. **One fixture.** `prepare` clones each vault at a pinned commit, applies the cell's discovery state
   (classifications, platform snapshots) and local workspace config, installs this distribution's current
   kernel and computes discovery facts deterministically (`--classify off`). `fingerprint.json` records the
   distribution commit, kernel version, suite and question hashes, facts hashes, harness versions and the
   judge/reviewer. Results are comparable only under the same fingerprint; re-run `prepare` after a kernel
   change and benchmark every setting again.
2. **Same inputs.** Every setting gets the same prompts in fresh headless sessions whose working directory is
   the fixture vault (a fresh copy per flow run). Nothing in the prompt names or favors a model.
3. **Blind, fixed grading.** Answers are graded by one fixed judge (`evals/regression/judge.py` `JUDGE`)
   that sees only the question, the expected facts, the forbidden claims and the answer text. Flow branches are
   reviewed by the same fixed reviewer with the same review prompt and the deterministic gate output. A judge
   from the family under test grades its own family too; report that when it applies.
4. **Repetitions.** Questions run three times per setting by default; report mean and range, never a single
   run. Treat differences inside the observed range as noise.
5. **Same accounting.** Usage is normalized to total input tokens (cached included), cached input and output
   tokens; dollars are reported only where the harness reports them.
6. **Safety.** Fixtures and flow copies push nowhere (push URL disabled); sources are read-only by prompt,
   and every flow run compares each source checkout's HEAD and working tree before and after
   (`sources_unchanged`).

## Suite (cell-owned)

Questions, flows and fixtures are cell knowledge: keep them in the cell or another private location, never
in this repository.

```json
{
 "questions": "questions.json",
 "vaults": {"<key>": {"source": "<vault checkout>", "commit": "<sha>", "branch": "<default branch>",
                      "config": "fixtures/<key>/config.yaml", "overlay": "fixtures/<key>/overlay"}},
 "quick": ["<question id>", "..."],
 "flows": [{"id": "sync-<repo>", "vault": "<key>", "kind": "sync", "repo": "<repo>",
            "source_target": "<expected reference commit>", "prompt": "<same prompt for every setting>"}]
}
```

Question entries follow `evals/regression/README.md`. Include questions over a note known to be wrong: they
measure whether a setting verifies before it asserts. A flow scenario stays reproducible while its source
reference branch still points at `source_target`; record a new scenario when it moves.

## Run

```bash
python3 -B evals/benchmark/bench.py prepare --suite SUITE.json --work WORK
python3 -B evals/benchmark/bench.py qa   --suite SUITE.json --work WORK --setting claude:sonnet:low [--quick]
python3 -B evals/benchmark/bench.py flow --suite SUITE.json --work WORK --setting claude:sonnet:low
python3 -B evals/benchmark/bench.py report --work WORK
```

Settings: `claude:<model>:<effort>`, `codex:<model>:<effort>`, `cursor:<model id>:` (Cursor encodes effort
in the model id, e.g. `grok-4.7-medium`; `agent --list-models` lists them). Log in to each harness CLI first
(`claude`, `codex login`, `agent login`). Run different harnesses in parallel if needed, but compare time only
between runs made under similar load.

## Screening a new model (quick check)

Do not re-run every setting when a model appears. The suite's `quick` list names the most discriminating
questions (in the reference campaign five of twelve: the two over wrong notes, where every violation
occurred, and the three with the lowest mean score). One pass ranks settings like the full three-run
campaign (Spearman 0.99 over eight settings), catches the same violations and costs about a seventh of it:

```bash
python3 -B evals/benchmark/bench.py qa --suite SUITE.json --work WORK --setting cursor:grok-4.8-medium: --quick
```

Compare its score and violations with the quick column of the baseline in
`kernel/90-Meta/execution-profiles.md`. Adopt it for questions when it has no violation and scores at least
like the recommended minimum; run one `flow` before using it to publish. Re-baseline (full campaign) only
after a kernel change that alters answering or publication.

## Measuring one behavior

`--ids` runs named questions only. Keep in the suite at least one question whose answer is in no note, only
in a source the vault reaches (a repository, a platform snapshot, a database): it measures whether a setting
follows the trail beyond the notes or stops at "unknown". Mark such questions with a `purpose` field.

```bash
python3 -B evals/benchmark/bench.py qa --suite SUITE.json --work WORK --setting codex:gpt-6-sol:low --ids S6
```

## Metrics

- Questions: judge score (share of expected facts stated), violations (forbidden claims asserted without
  reserve), time, tokens, cost.
- Flows: the author runs the scenario prompt; deterministic gates (`sync verify`: note gates, stale neighbour
  notes, structural issues) and the fixed reviewer judge the branch. A `revise` verdict or a failing gate gets a
  focused repair from the same setting and a new review, up to `--repairs` (default 2), as the sync protocol
  prescribes. Reported: accepted on the first pass, accepted after repairs, repairs, material findings on the
  first pass, stale neighbours left on the first pass, source integrity, and author time, tokens and cost
  summed over every round.

`report` writes `report.md` and `report.json` in the work directory. Publish only the anonymized table
(settings and numbers) in `execution-profiles.md`; raw outputs contain cell material.
