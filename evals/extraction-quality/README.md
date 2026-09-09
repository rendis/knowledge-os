# Semantic extraction evaluation

One compact Cedar Fulfilment scenario combines six interacting pressures: fractional units, unchanged consumer impact, relationship deletion, preservation of valid vault knowledge, conflicting environment settings, and no runtime evidence. It uses two actual producer commits, one unchanged consumer commit, brief existing vault notes, and the selected distribution's actual manifest/scaffold/card contract. No expected answers or oracle are copied into the worker directory.

```sh
python3 evals/extraction-quality/prepare.py --distribution /absolute/distribution --output /absolute/fresh/case
python3 -m unittest discover -s evals/extraction-quality -p 'test_*.py' -v
```

Send only the resulting task.txt to a fresh extractor. The output must be package/analysis.json, not a simplified Q&A answer. The coordinator should run its ordinary finalizer/checker, then give an independent judge oracle.md plus the produced artifact and package evidence. Keep score records outside the worker directory. Prepare baseline and candidate independently with the same script and different --distribution values; commit contents and fixture inputs are deterministic, while paths vary. The producer retains the unchanged consumer contract so own-repository claim evidence can cite a frozen producer path even when the consumer has no diff.

For first-pass comparison, do not provide judge feedback until both outputs are frozen. The rubric specifies one targeted correction and separate recovery scores; do not silently merge first-pass and corrected results. Test code executes the frozen arithmetic and unchanged consumer predicate, checks removed relation and environment facts, verifies unchanged contracts and clean source state, and smoke-tests production scaffold creation. It does not score agent output with keywords.

Limitations: this is synthetic documented-source evidence, not runtime validation, and one compound scenario is not a statistical benchmark. One worker per distribution gives directional evidence only. Judge entailment requires independent inspection; there is no automated semantic oracle. No full coordinator gate/projection or durable vault application is exercised here. Check preservation through explicit node/claim reasoning; do not demand unchanged knowledge be restated as new claims. Efficiency needs runner telemetry and cannot be inferred reliably from output size.
