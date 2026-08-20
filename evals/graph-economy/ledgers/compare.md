# Graph economy: baseline vs after

Fixture: Norte Logistics (`evals/graph-economy/`). Harness: `harness.py --mode baseline|after`. Obsidian app: not used (`obsidian_cli_used: false`).

## Gate

KEEP if, on the same gold set:

- recall of questions 3–7 does not drop
- `bytes_read` of questions 3–5 drops clearly (stop loading Convenciones + Framework + Home for dependency questions)
- question 7 goes from miss to hit without reading every investigation case file

## Numbers

| Metric | Baseline | After |
|---|---|---|
| recall q3–q7 | 1.0 / 1.0 / 1.0 / 1.0 / 1.0 | 1.0 / 1.0 / 1.0 / 1.0 / 1.0 |
| bytes q3–q5 | 48180 | 5092 (~10.6% of baseline) |
| q7 investigation join | miss (both case files opened) | hit (`20260820-090000-stale-gps-write` only) |
| q9 `25-Topics/` seeded with `--disable-topics` | true | false |
| economy_fail (Convenciones/Framework on q3–q5, q7) | q3, q4, q5, q7 | none |

Raw JSON: `evals/graph-economy/results/baseline.json` and `after.json` (gitignored).

## Personas (same gold, deterministic)

Messaging cell (topics enabled): a dependency/impact question that opens `Convenciones.md` or `Auditoria - Framework.md` is an economy fail. After: **pass**.

`--disable-topics` cell: do not expand Pub/Sub; do not treat `25-Topics/` as inventory. After: directory not seeded; orientation answers from Home + `instance.yaml` only. **pass**.

## Decision

**KEEP** `exp/graph-economy`.

Recall did not drop. Bytes for q3–q5 fell by ~43k. q7 join is a hit without the billing control case. Init no longer copies `25-Topics/` when topics are disabled.

INFO (not a revert reason): hygiene still lists Home `[[*.base]]` targets as unresolved because `verify-links.py` indexes markdown stems only. That is pre-existing validator behavior, not a recipe regression.
