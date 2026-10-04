# Harness benchmark

Compares harness settings (harness, model, effort) on the work a cell actually does: answering evidence
questions from its vault and publishing a repository sync. The result is the measured minimum setting per
task shape recorded in `kernel/90-Meta/execution-profiles.md`, the table users start from.

## Protocol

Fairness comes from holding everything except the setting constant:

1. **One fixture.** `prepare` clones each vault at a pinned commit, applies the cell's discovery state
   (classifications, platform snapshots) and local workspace config, installs this distribution's current
   kernel and computes discovery facts deterministically. `fingerprint.json` records the
   distribution commit, kernel version, suite and question hashes, facts hashes, harness versions and the
   judge/reviewer. Results are comparable only under the same fingerprint; re-run `prepare` after a kernel
   change and benchmark every setting again.
2. **Same inputs.** Every setting gets the same prompts in fresh headless sessions whose working directory is
   the fixture vault (a fresh copy per flow run). Nothing in the prompt names or favors a model.
3. **Fixed grading.** Answers are graded by one fixed judge (`evals/regression/judge.py` `JUDGE`)
   that sees the question, expected facts, forbidden claims, answer text and optional independently captured
   source packet. The setting label is withheld; answer text may reveal it, so do not claim perfect blinding. Flow branches are
   reviewed by the same fixed reviewer with the same review prompt and the deterministic gate output. A judge
   from the family under test grades its own family too; report that when it applies. Calibrate the judge
   with both source-supported and demonstrably defective controls. Adjudicate material model findings
   against primary sources: expected-fact coverage and a successful reviewer verdict alone do not certify
   every delivered claim. Record corrections to control labels without replacing historical inputs.
4. **Repetitions.** Questions run three times per setting by default; report mean and range, never a single
   run. Treat differences inside the observed range as noise.
5. **Same accounting.** Usage is normalized to total input tokens (cached included), cached input and output
   tokens; dollars are reported only where the harness reports them.
6. **Safety.** Fixtures and flow copies push nowhere (push URL disabled); sources are read-only by prompt,
   and every flow run compares each source checkout's HEAD, reference targets, Git configuration,
   working-tree diff and untracked bytes (including ignored files) before and after (`sources_unchanged`).
   A permitted fetch that changes these inputs invalidates the frozen comparison; it is not a permission violation.

## Suite (cell-owned)

Questions, flows and fixtures are cell knowledge: keep them in the cell or another private location, never
in this repository.

```json
{
 "questions": "questions.json",
 "heldout": ["<question id>", "..."],
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

## Held-out questions: the reference for quality

Questions that shaped the kernel (the ones a rule was written for, or the quick set used to screen models)
end up answered well by construction: in the reference campaign the quick set scored 1.00 for the top settings,
while ten questions no kernel change had seen scored 0.64–0.67 with or without `kos`. List the questions that no
kernel change has been tuned on under `heldout`, never edit the kernel while looking at their answers, and
report quality claims (a kernel change helps, a setting is good enough) from them:

```bash
python3 -B evals/benchmark/bench.py qa --suite SUITE.json --work WORK --setting claude:opus:medium --heldout
```

When a held-out question has driven a fix, move it out of `heldout` and write a new one in its place.

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

## Measuring a kernel behavior with an arm

`--arm` runs the same questions with one behavior removed and nothing else changed. `no-review` tells the
session no independent reviewer is available, so the router's rule for answering without one applies: compare
score, violations, time and cost with the normal arm on the held-out set before deciding whether review stays
mandatory for answers (publication review is not affected).

```bash
python3 -B evals/benchmark/bench.py qa --suite SUITE.json --work WORK --setting claude:opus:medium --heldout --arm no-review
```

## Metrics

- Questions: source-backed delivery first (model source-audit passes in `verified_answers`, unavailable verification and uncited
  claims), then judge score (share of expected facts stated), violations (forbidden claims asserted without
  reserve), unsupported claims, grading/execution errors and clean answers (complete score with none of
  those failures), agent time, tokens, cost; the score per vault and the lost points by kind (`omitted`, `abstained`,
  `wrong`, `direction`, `path`, `imprecise`), which say what to fix: `direction` points at missing typed
  relations in the notes, `path` at a flow the notes do not assemble, `abstained` at a trail the agent did not
  follow.
  Adjudicate flags and passing answers against actual sources before claiming improved reliability; model
  verdicts and coverage scores alone cannot establish it.
- Flows: the author runs the scenario prompt; deterministic gates (`sync verify`: note and case gates,
  stale neighbour notes, copied paragraphs, structural issues and every other verification problem) and the
  fixed reviewer judge the branch. Only the missing review record is replaced by this reviewer's verdict;
  failed commands and invalid or incomplete gate output cannot pass. The released CLI must match the current
  source fingerprint and checksum. A `revise` verdict or a failing gate gets a
  focused repair from the same setting and a new review, up to `--repairs` (default 2), as the sync protocol
  prescribes. Reported: accepted on the first pass, accepted after repairs, repairs, material findings on the
  first pass, stale neighbours left on the first pass, source integrity, and author time, tokens and cost
  summed over every round, plus reviewer and total agent time, tokens and cost. The recorded wall time
  includes gates and reviews. Unknown cost or usage stays unavailable, including partially reported runs.
  Judge time and cost are reported separately. A failed, missing or malformed grade cannot be a clean answer;
  answers above 128 KiB fail grading without a judge request. A failed or malformed reviewer cannot accept
  a flow. Source integrity streams untracked regular files and rejects special files without opening them.

`clean_answers` is complete expected-fact coverage with no flagged issues, not proof of verified delivery.
Only a valid source packet and a passing claim audit count toward `verified_answers`; see
[`evals/regression`](../regression/README.md#source-backed-delivery). Unsupported flags without a source
packet remain allegations. For a CLI/kernel comparison, hold the toolchain and frozen knowledge constant
and separate baseline, changed CLI with baseline kernel, and changed CLI with changed kernel. Rotate their
execution order across repetitions. Exclude sandbox/toolchain failures from speed conclusions, not from
the recorded artifacts. Accept evidence quality before coverage, consistency and time.

`report` writes `report.md` and `report.json` in the work directory. Publish only the anonymized table
(settings and numbers) in `execution-profiles.md`; raw outputs contain cell material.
