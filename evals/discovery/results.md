# Discovery regression — 2026-09-25

`run_real.py` on sandbox clones of three installed cells, with judgments already stored and
read-only Pub/Sub snapshots of the readable projects. Cell-specific names stay in the cells.

| Cell | Repos scanned | Supported topic/event relations | Discrepancies | Logical resources without a topic note |
|---|---:|---:|---:|---:|
| A | 66 (1 empty repo) | 36 | 2 | 6 |
| B | 19 | 12 | 0 | 12 |
| C | 39 | 24 | 0 | 40 |
| **Total** | 124 | **72** | **2** | 58 |

The two discrepancies are errors in the current notes, shown by platform wiring:

- a consumer note declares an outbound topic, while its configured subscription reads a different
  inbound topic in another project;
- a consumer note declares a subscription name as its topic; the service reads another topic through
  its own subscription.

Channel coverage (library-level connectors vs documented edges) measured in the scratch PoC: 188/188.
Cost of the first full classification with Jev: ~2.6k calls per cell, about USD 0.08, under a minute.
Without a key the same questions are answered by the agent (`discover questions` / `discover answer`).
