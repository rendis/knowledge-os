# Discovery regression — first field run (2026-09-25)

These are historical results from that distribution snapshot. The current CLI no longer uses Jev or the
semantic citation check; see [ADR 0003](../../docs/adr/0003-kos-builds-and-checks-the-map.md).

`run_real.py` on sandbox clones of three installed cells (124 repositories in Go, Java, TypeScript,
JavaScript and Python; Kubernetes/Kustomize, Cloud Functions and Terraform configuration), with
stored judgments and read-only Pub/Sub snapshots of the readable projects. Cell-specific names stay
in the cells.

| Measure | Result |
|---|---|
| Documented code connections whose channel is found (library/runtime level) | 188/188 |
| Documented topic/event relations supported by discovered names or platform wiring | 72/74 |
| Remaining relations | 2, both errors in the notes shown by platform wiring (a subscription name documented as the topic; a note naming a topic its configured subscription does not read) |
| Configuration naming resources absent from the captured project | reported as pending, not asserted |
| First full classification with Jev | ~2.6k calls per cell, ~USD 0.08, < 1 minute; afterwards only new items |
| Run time per cell with stored judgments | 1.5–6.5 s |

Note gates over all repository notes (`discover check`): 4,741 source anchors, 99% verified; the
remaining errors are citation defects (identifiers claimed from a file that does not contain them,
line ranges past the end of the file, files absent at the cited commit). Notes written before
permalinks were required (one cell) have no mechanically verifiable anchors and fail coverage until
their next sync. At that time, the semantic citation check (Jev) was noisy on paragraphs that grouped several
facts and references, and was kept as a review aid.
