# Reconciliation pass — three installed cells, 2026-09-25

`discover run` + `discover check` over every repository note (sandbox clones, read-only sources,
read-only Pub/Sub snapshots). Nothing in the real vaults was modified. Cell-specific names stay in the
cells.

## Relations contradicted by platform wiring (2)

Both in cell A: a consumer note declares an outbound topic that its configured subscription does not
read, and another declares a subscription name as its topic.

## Source anchors (G1)

| Cell | Anchors | Verified | Errors | Notes failing G1/G2 |
|---|---:|---:|---:|---:|
| A | 85 | 85 | 0 | 38 of 64 (56 notes have no permalinks; coverage gaps) |
| B | 1,664 | 1,660 | 2 | 2 of 19 |
| C | 2,992 | 2,966 | 28 | 6 of 28 |

Anchor errors are citation defects in existing notes: identifiers claimed from a file that does not
contain them (e.g. tables named from `pipeline.yml`, `Run` cited from `go.mod`), line ranges beyond the
file and files absent at the cited commit. They are corrected on each note's next sync.

## Configuration naming resources absent from the platform

One repository configures seven topics and subscriptions in its UAT project that do not exist there;
another configures per-country subscriptions absent from the snapshot. Pending confirmation, not
asserted.

## Semantic citation check (Jev)

On two notes of cell C (126 anchors) most footnotes were judged `says_nothing`: paragraphs group
several facts and references at their end, and limits are cited. The check is kept as a review aid;
the mapping contract now requires one verifiable fact per cited sentence.
