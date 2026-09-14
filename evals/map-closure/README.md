# Campaign closure behavioral regression

This isolated scenario tests the gap between accepted service maps and a coherent
campaign close: sibling source answers, duplicated verification questions, missing
runtime evidence, and stale visible coverage. It is a behavioral evaluation, not a
new publication engine or a substitute for existing sync gates.

Prepare a fresh case for each baseline/candidate distribution:

```sh
python3 -B evals/map-closure/prepare.py --output /absolute/fresh/closure-case
python3 -B -m unittest discover -s evals/map-closure -p 'test_*.py' -v
```

Give a fresh worker only the generated `task.txt`, workspace path and the selected
distribution's map-ecosystem skill/references. Do not give it `oracle.md`. Sources
are isolated Git repositories with revision files; vault notes are intentionally
minimal scenario inputs, not full production publication packages. Evaluate the
result with an independent judge using the oracle and the original inputs.
Freeze the first-pass result before feedback. Record distribution commit, model,
effort, evidence, individual criteria, elapsed time and available token telemetry.
One targeted correction may be scored separately; do not blend it with first pass.

The automated test only verifies that the two source functions compose as claimed,
that the stale coverage is present, and that preparation leaves clean committed
sources without leaking the oracle. It does **not** execute an LLM, prove the skill
is followed, validate actual infrastructure, or exercise production gate artifacts.
A behavioral pass requires a recorded worker run and independent semantic review.
