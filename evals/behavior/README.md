# Visual behavior checks

Deterministic checks of the `explain-visually` assets and checkers; they run offline and are never
installed in a cell.

```sh
python3 -B evals/behavior/test_visual_checks.py
python3 -B evals/behavior/test_visual_context.py
python3 -B evals/behavior/test_visual_icons.py
node evals/behavior/test_visual_focus.cjs
node evals/behavior/test_visual_scenario.cjs
node evals/behavior/test_visual_spatial.cjs
node evals/behavior/test_visual_text_fit.cjs
```

The `.cjs` checks use DOM stand-ins; rendering in a browser remains unverified by them.
`visual-presentation-cases.md` and `results/explain-visually/` record the presentation cases and past
observations. Agent behavior with the current kernel is measured by `evals/benchmark` and `evals/regression`.
