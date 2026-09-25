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

```bash
python3 -B evals/regression/run.py --harness codex --questions Q.json --out OUT --vault <key>=<vault-path>
python3 -B evals/regression/judge.py --questions Q.json --answers OUT
```

`run.py` supports `claude` (`-p`, Opus, medium effort), `codex` (gpt-5.5, medium reasoning, read-only
sandbox) and `cursor`. `judge.py` grades with Codex so Claude answers are judged by another model family.
Run on clones of the vaults with pushes disabled; sources stay read-only.
