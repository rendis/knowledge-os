# Review complete note changes before publication

The single semantic review for technical notes: repository maps, synchronization, external connections and ordinary documentation. The author commits the complete resulting notes on a `sync/` branch first; the reviewer judges the text that will be consumed against the baseline and the exact evidence. It does not reconstruct the repository map.

## Reviewer inputs

- `git diff <base>...HEAD` of the branch (complete new and old note content);
- the discovery facts of the affected repositories (`discover report --repo`) and platform snapshots they cite;
- the `discover check` output: anchors already verified mechanically are not re-checked for existence; `review` items and semantic flags are;
- [evidence-sufficiency.md](evidence-sufficiency.md) for pending items.

## What to check

- Affected P1–P5 answers and their citations; every changed or removed source value that could leave stale prose.
- Blanket statements (`all`, `always`, `never`, `only`) in a flow whose delta adds an exception; changed filters, time windows and retry limits keep decision order, units and boundary inclusivity.
- Main-flow triggers, effects, destinations, rules and failure behavior within scope; every discovery fact has an owner, relation or explicit limit.
- Preservation of unaffected knowledge, links, provenance and limitations; removed connections are supported by evidence.
- Each pending item's material question and close condition.

Accept useful partial documentation with explicit limits. Reject unsupported statements, wrong conditions or destinations, omitted main flows, stale changed values and loss of valid knowledge. Optional detail and an unresolved external end do not invalidate an accurate local map.

## Verdict

Return `accept` or `revise` with one evidence-backed reason per defect (note path and sentence). The coordinator records it with `<cli> sync review --vault "<vault>" --verdict <v> --reviewer "<reviewer>" --summary "<reasons>"`. On `revise` the author corrects only the cited text, commits, re-runs the gates and asks for review of that change; a defect that survives one correction, missing evidence or a required scope expansion stops that note as unresolved. `sync verify` refuses content that changed after the accepted review.
