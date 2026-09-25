# Review complete note candidates before publication

Use this as the single semantic review for mapped technical notes, repository synchronization and external-connection updates. The author prepares the complete resulting Markdown images first. The reviewer judges the text that would be consumed, against the current notes and the exact evidence that supports the changed meaning. It does not reconstruct the repository map.

Apply [evidence-sufficiency.md](evidence-sufficiency.md) before preparing or reviewing pending items. Preserve stronger existing evidence and supported knowledge beyond the immediate P1–P5 delta.

## Prepare one candidate

Stage every affected complete Markdown image in a local ignored candidate directory, preserving its vault-relative path. Leave the vault bytes unchanged. Start from the current note images and preserve unaffected prose, links, provenance and connection anchors. A deletion is an explicit candidate outcome, never an omitted file.

For source mapping or sync, gather the frozen repository identity, production ref, old and new OIDs, analysis date, current complete note baseline, affected P1–P5 questions, exact source anchors and explicit limits. For an external-only update, bind the external authority, target/environment, observation time or revision, safe retrieval reference, observed facts and limits; keep repository source commit and analysis dates unchanged.

Run the available cheap deterministic candidate checks before spending a semantic review. Resolve every reported mechanical issue in the candidate first. These checks establish only the properties they report, not factual correctness.

Freeze the candidate, evidence and exact source identities:

```text
<cli> sync review freeze --vault <vault> --candidate <candidate-dir> \
  --evidence-root <evidence-dir> --evidence <relative-record> \
  --source <repository> <checkout> <production-ref> <old-oid-or-empty> <new-oid> <outcome> \
  --analysis-date <YYYY-MM-DD> --output <manifest.json>
```

Repeat `--evidence` and `--source` when required; pass an explicit empty string for `old-oid-or-empty` on a new repository. Set `outcome` to `write`, `no-documentation-change`, or `no-durable-node` for that repository. At least one source has `write` when Markdown changes; the other bound sources may advance their own cursors without claiming that their repository note changed. `no-documentation-change` requires an existing repository note, while `no-durable-node` requires its absence. Declare a removed note with repeatable `--delete <vault-relative.md>`; omission alone is not a deletion. The manifest stores portable repository identity and revisions, never the checkout's absolute path. Durable notes still carry their authoritative citations. Hashes bind bytes and provenance; they do not prove that prose is true.

For ordinary non-source or external-only documentation, omit `--source` and `--analysis-date`; freeze retains its version-1 manifest contract and the repository source metadata in the notes remains unchanged.

## One independent semantic decision

Give one fresh reviewer the manifest, complete candidate images, original note images and the bound evidence. Require it to check:

- affected P1–P5 answers and exact citations;
- every changed or removed source value that could leave stale prose;
- retained blanket statements in the same affected flow (`all`, `always`, `never`, `only`) when the delta adds an exception, without revalidating the whole baseline;
- changed filters, time windows and retry limits in the affected flow preserve decision order, units and boundary inclusivity from the exact predicate rather than variable names or prose handoffs;
- main-flow triggers, significant effects, destinations, rules and failure behavior within the frozen scope;
- preservation of unaffected useful knowledge, links, provenance and limitations;
- every old/new connection anchor as create, preserve, update or retire, with retirement supported by evidence; and
- each pending item's material question and sufficient close condition.

The reviewer accepts useful partial documentation when its limits are explicit. It rejects unsupported statements, materially wrong conditions or destinations, omitted main flows, stale changed values, and loss of valid knowledge. Optional detail and an unresolved external end do not invalidate an otherwise accurate local map.

The reviewer writes:

```json
{
  "version": 2,
  "manifest_digest": "<digest emitted by freeze>",
  "verdict": "accept",
  "findings": [],
  "retired_connections": {
    "20-Repos/service.md#connection.legacy.publish": "The frozen source removes this connector and no replacement wiring retains it."
  }
}
```

`retired_connections` contains exactly the old anchors absent from the candidate, with an evidence-backed reason for each. The complete old/new images determine created, preserved and updated anchors without duplicating them in the review artifact.

For a version-1 ordinary/external-only manifest, keep the existing review version 1 and `connection_decisions` map. It accounts for every anchor in the old/new union as `create`, `preserve`, `update` or `retire`, with a reason. This is an alternate artifact shape for the same single semantic review, not another review stage.

Use `revise` with one evidence-backed reason per actionable semantic defect. Correct only the affected candidate text from the already inspected evidence. Freeze the resulting complete images again and re-review the materially changed meaning plus preservation around it. This is a targeted candidate repair, not another extraction. Stop the affected publication when the same semantic defect survives repair, the required evidence is unavailable, or the source scope must expand; retain the reusable candidate and report the exact limitation.

## Deterministic check and local repair

After semantic acceptance, bind the review to the frozen candidate bytes:

```text
<cli> sync review check --vault <vault> --candidate <candidate-dir> \
  --evidence-root <evidence-dir> --manifest <manifest.json> --review <review.json> \
  --checkout <repository> <checkout>
```

Repeat `--checkout` for every bound repository. The checker revalidates the frozen repository identities and revisions, then returns any deterministic issues and locations. Do not infer a semantic defect from a mechanical failure. Candidate bytes do not change under an accepted review. If any post-review correction changes them, freeze the new complete images and obtain targeted semantic re-review of the changed portion and its preservation boundary before checking again. Deterministic failure never triggers repository extraction.

For a version-1 ordinary/external-only manifest, run the same check without `--checkout`.

## Publish exact images

Publish only after semantic acceptance and a passing deterministic check:

```text
<cli> sync review publish --state-root <state-root> --vault <vault> \
  --candidate <candidate-dir> --evidence-root <evidence-dir> \
  --manifest <manifest.json> --review <review.json> \
  --checkout <repository> <checkout>
```

Repeat `--checkout` for every bound repository and use the installed command's help for optional arguments. Publication writes only the candidate bytes bound to the accepted review and records resumable state. An accepted unchanged existing-repository baseline records `no-documentation-change`; an accepted new repository with no durable node records `no-durable-node`. A later accepted write for the same repository and commit retires that acknowledgement. Resume an interrupted publication from its recorded state with the same candidate, manifest and review. Source drift requires a new delta analysis; destination drift requires rebuilding only the affected candidate against the new baseline.

For a version-1 ordinary/external-only candidate, after a passing check copy only its reviewed complete images to authorized destinations, verify destination hashes with `sync review verify-published --vault <vault> --reviewed <manifest.json> <review.json>`, and run the same vault gates. Recheck the destination baseline immediately before writing; destination drift rebuilds and reviews only the affected candidate.

Run the ordinary audit/link/Bases checks that apply and inspect the final diff. Completion requires accepted changed meaning, passing deterministic checks, exact reviewed images at their authorized destinations or an accepted no-change acknowledgement, and a durable publication receipt. Report partial coverage separately from successful publication and source freshness.
