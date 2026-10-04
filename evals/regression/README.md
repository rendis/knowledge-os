# Regression sample for kernel changes

A small, frozen set of real questions per cell, answered by a real harness inside an installed vault,
graded by an independent judge. Run it before and after a kernel change (router, skills, CLI) to show
that answer quality holds while time and tokens change. It is a regression sample, not a catalog of
what a cell can ask; add a question whenever an agent fails on a real one.

The question set is cell knowledge: keep it in the cell (for example `90-Meta/regression/questions.json`,
cell-owned) or another private location, never in this repository. Each entry:

```json
{"id": "Q1", "vault": "<key>", "question": "…", "expected": ["fact the answer must state", "…"], "must_not": ["claim that would be wrong"]}
```

Prefer questions whose answers are objective (discovery facts, platform wiring, verified note anchors),
plus a few that test verification over a note known to be wrong and abstention when evidence is missing.
Keep each expected item atomic: split distinct conditions and outcomes so a covered success path cannot
hide an omitted error path. Confirm partial and full-credit decisions against the delivered answer.

```bash
python3 -B evals/regression/run.py --harness codex --questions Q.json --out OUT --vault <key>=<vault-path>
python3 -B evals/regression/judge.py --questions Q.json --answers OUT
```

`judge.py` also names why each lost point was lost (`omitted`, `abstained`, `wrong`, `direction`, `path`,
`imprecise`) and prints the score per vault, so a change can be read as "fewer direction errors in one cell"
rather than a single mean. `run.py --arm no-review` runs the questions without the independent reviewer.

`run.py` supports `claude` (`-p`, default Opus at medium effort), `codex` (default gpt-5.5 at medium
reasoning, read-only sandbox) and `cursor` (its default model); `--model` and `--effort` compare cheaper
settings. `judge.py` grades with Codex so Claude answers are judged by another model family.
Run on clones of the vaults with pushes disabled; sources stay read-only.

## Source-backed delivery

Expected-fact scores measure coverage. They do not verify every assertion. To evaluate delivered
information, add an `evidence` packet to each private question:

```json
"evidence": [{"id": "S1", "source": "repository/file@<commit>:10-20", "content": "10: actual source text…"}]
```

Capture these excerpts independently from the frozen reference revision or bounded runtime observation.
Include the relevant branches and counterevidence, with scope, revision and line numbers. Do not use the
expected answer as evidence, include secrets, or claim an excerpt is a complete inventory. Packets are
limited to 256 KiB; malformed, duplicate-ID or oversized packets fail without a judge request.

The fixed judge audits every material assertion against the packet. Each `claims` entry records its kind
(fact, inference or evidence boundary), support, source IDs and whether the delivered answer cites a concrete supporting reference. A contradicted,
unverified or uncited assertion blocks `evidence_verified`; full coverage cannot override it. Explicit
inferences and unresolved boundaries must stay within the evidence. A grounded boundary needs no extra
citation beyond the assertions it qualifies; it can never establish a positive runtime assertion.
Process narration is checked separately against execution logs when material. A missing packet leaves verification
unavailable, including for older grades. `verified_answers` and coverage are reported separately. Model
audits can miss claims or misread sources: adjudicate flags and inspect passing answers against the sources
before drawing a reliability conclusion, and report the judge family. `evidence_verified` is the model's
source-audit verdict; it cannot serve alone as a delivery or release gate.

For reproducible tools in Codex sessions, set `BENCH_TOOL_PATH` to the same toolchain PATH for every arm.
The runner pins it and disables login shells, which can otherwise replace PATH. Check `command -v git`,
`git --version` and a source read inside the read-only sandbox before timing. This is especially relevant
on macOS, where Apple's Git launcher can attempt sandbox-denied cache writes. Use absolute `kos` paths
and confirm the versions in actual execution logs; an inherited PATH alone is insufficient.

## Calibrate the evaluator with known answer defects

Keep positive and negative controls beside the private questions. `acceptance.py` replays saved answer
text against the existing fixed source judge; it does not generate new answers or alter the kernel.
Use independently inspected source excerpts and an atomic expected fact for each known defect. Include
a corrected positive control so an evaluator that rejects everything cannot pass.

```json
{"cases": [
  {"id": "corrected", "question": {"question": "What does this source establish?",
    "expected": ["implementation behavior only"], "must_not": [],
    "evidence": [{"id": "S1", "source": "service@<commit>:10-20", "content": "actual source text"}]},
   "answer_file": "corrected.txt", "expect": "accept"},
  {"id": "scope-error", "question": {"question": "What does this source establish?",
    "expected": ["implementation behavior only"], "must_not": [],
    "evidence": [{"id": "S1", "source": "service@<commit>:10-20", "content": "actual source text"}]},
   "answer_file": "scope-error.txt", "expect": "reject", "failed_facts": [0], "evidence_failure": true}
]}
```

```bash
python3 -B evals/regression/acceptance.py --suite PRIVATE/suite.json --out PRIVATE/attempt-1 \
  --repetitions 3 --parallel 3
```

Paths resolve relative to the suite. Expected decisions and case labels never reach the judge.
`failed_facts` identifies zero-based expected-fact indices that must lose credit; `evidence_failure`
requires a failed source audit. A rejection for an unrelated omission does not satisfy that control.
Grading errors or unavailable audits are inconclusive, never successful detection. Acceptance requires
every fact, a passing source audit and a correct verdict; a rounded coverage score cannot hide an omission.
Inputs, hashes and each repetition are saved; an existing result directory cannot be overwritten.
The command exits nonzero on any mismatch or inconclusive run. This calibrates known defects, not
held-out kernel quality. Inspect flags and passing controls against the sources before using the result.
