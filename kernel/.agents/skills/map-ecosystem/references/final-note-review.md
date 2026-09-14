# Review complete note candidates before publication

Use this final check for mapped technical notes, sync documentation units and external-connection updates. Source-package review establishes acceptable claims; this review checks their actual rendering and preservation in the complete notes. It is a bounded review of the changed meaning, not another repository extraction.

Apply [evidence-sufficiency.md](evidence-sufficiency.md) before preparing or reviewing pending items. Check scope-based reclassifications against the user objective and preserved evidence; do not insist on obsolete functional tests or count retired demands as observed successes.

## Prepare

Stage only the affected complete Markdown images in a local ignored work area, preserving their vault-relative paths. Preserve the current vault bytes until review passes. Gather the accepted source package and delta, or an external-evidence record with authority, exact target/environment, observation time or source revision, safe retrieval reference, observed facts and limits. The record must distinguish configured identity from observed runtime; omit credentials, raw logs and production rows. The cell evidence profile and authorized scope still govern publication. Hashes establish integrity, not the truth of the evidence or permission to query it.

For an external-only update, keep the repository's source commit and analysis dates unchanged. Retain connection anchors for resolved as well as pending connections. Duplicate anchors block the check, including duplicates in the existing baseline; resolve that identity ambiguity explicitly before preparing a new candidate. Existing notes without anchors may adopt them for the inspected scope; the reviewer must check existing prose for duplicate identities and preserved knowledge.

Freeze the candidate and evidence with the installed helper:

```text
python3 -B 90-Meta/review-note-candidate.py freeze --vault <vault> --candidate <candidate-dir> --evidence-root <evidence-dir> --evidence <relative-record> --output <manifest.json>
```

Repeat `--evidence` for required evidence files. Declare a removed note with repeatable `--delete <vault-relative.md>`; do not also stage an image for that path. For a sync documentation unit also pass `--projection <projection.json>` to bind the candidate to the existing projection. Keep the manifest and review outside the graph. Durable notes must carry authoritative source references and observation dates; a temporary evidence record alone is not a durable citation.

## Independent decision

Give a fresh reviewer the manifest, full candidate images, original notes and bound evidence. Require it to check affected P1-P5 answers, obsolete values surviving elsewhere in the note, unchanged useful knowledge, and the actual evidence required to close each pending item. It must decide each connection anchor in the union of old and new images: create, preserve, update or retire, with a reason. Moving or renaming an identity requires an explicit retirement/create explanation; a changed destination normally keeps the existing identity.

The reviewer writes:

```json
{
  "version": 1,
  "manifest_digest": "<digest emitted by freeze>",
  "verdict": "accept",
  "findings": [],
  "connection_decisions": {
    "20-Repos/service.md#connection.orders.publish": {
      "action": "update",
      "reason": "The endpoint changed; the connector slot and remaining verification are preserved."
    }
  }
}
```

Use `revise` with specific findings (`{"reason": "<evidence-backed defect>"}` per finding) for incorrect prose, lost knowledge or unsupported closure. Correct actionable findings within the existing candidate and evidence scope, preserving each rejected attempt. Before each correction, identify the exact defect and its available authoritative evidence; after any candidate/evidence change, freeze a new manifest and obtain an independent review bound to it. Verify cited revisions and paths before review; changing a citation also requires checking that its content supports the claim.

Continue targeted corrections while distinct findings can be resolved from available evidence without expanding the task. Stop the affected publication when the same defect survives a correction, the scope must expand, or required evidence is unavailable; report the precise remaining defect and retain reusable work. An unavailable source leaves the dependent pending item open. A second review finding alone does not require a new cycle, extraction or user approval. This final-note repair policy does not change the separate source-package correction limit enforced by `sync-correction.py`.

## Check and publish

```text
python3 -B 90-Meta/review-note-candidate.py check --vault <vault> --candidate <candidate-dir> --evidence-root <evidence-dir> --manifest <manifest.json> --review <review.json>
```

For sync, validate the projection first, then register the accepted review against the stored projection:

```text
python3 -B 90-Meta/sync-run.py review-unit --state-root <root> --run-id <run_id> --unit-id <group-NNN> --vault <vault> --candidate <candidate-dir> --evidence-root <evidence-dir> --manifest <manifest.json> --review <review.json>
```

The command invokes the checker against the checkpointed projection, compares reviewed file presence and deletions with the patch-derived path kinds, and persists its bound receipt. An empty file cannot stand in for an absent file. Only then use `apply-unit`; acknowledgement units are exempt. For external-only documentation, after a passing check copy only the reviewed full images to their corresponding authorized note paths and verify destination hashes against the manifest. Explicitly declared deletions follow the ordinary authorized retirement procedure; the manifest represents their result with the empty-content digest, and publication must verify absence. Run the ordinary vault/link gates and inspect the final diff. Recheck immediately before writing; destination drift requires a new candidate and review, never overwriting changed user bytes.

Before declaring completion or committing the scoped notes, verify the current bytes:

```text
python3 -B 90-Meta/review-note-candidate.py verify-published --vault <vault> --reviewed <manifest.json> <review.json>
```

For successive accepted updates, repeat `--reviewed` in publication order; the latest accepted full image governs each overlapping path. Include all in-scope published notes. A mismatch blocks closure for that path: preserve the current bytes, inspect the delta, and reuse a later valid review if one exists. Otherwise freeze the affected complete note and obtain independent review focused on the addition and preservation of the accepted content. Reuse unaffected reviews and source evidence; then verify again. Administrative summaries remain subject to their inventory/configuration checks.

Completion requires the accepted review, passing integrity/identity checks and published bytes matching the reviewed images. A structural pass cannot detect false prose; the independent reviewer owns that decision. The helper does not query providers, grant access, publish notes or replace sync authority. Report partial external coverage separately from successful publication and source freshness.
