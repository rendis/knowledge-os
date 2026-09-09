# Resumable synchronization evaluation criteria

These criteria freeze the public contract for the resumable synchronization
change. They deliberately exercise the installed helper CLIs; fixtures contain
only generic `APP00000-*` repositories and no consumer knowledge.

1. A repository/OID package has one initial extraction/review and at most one finding-directed correction with a fresh review before checkpointing. A checkpointed package is never re-extracted for a publication retry.
2. `all-ready` is a sealed semantic checkpoint; a projection or apply failure resumes from its affected unit.
3. Acknowledgements and each write group are independently validated and atomically applied units.
4. Publication authority is explicit `repository + claim_id + node` lineage, never a basename substring heuristic.
5. Only unreadable exact evidence, invalid identity/binding/integrity, affected-source staleness, or unsafe persistence are hard blockers.
6. Invalid projections return `projection-invalid`, `retryable: true`, and `resume_from: projection`.
7. `validate-write`, unified patches, and legacy gate readers are absent.
8. Active checkpoints contain only finalized, validated, redacted artifacts and closed receipts.
9. Source-OID changes invalidate only their package and connected groups; destination changes invalidate only their writing unit.
10. Equal inputs produce stable bytes and do not repeat semantic work.
11. Evaluation is fixture-based, deterministic, ledgered, and rerun only after a material candidate-tree change.
12. Promotion uses the installer update path and preserves cell identity and knowledge byte for byte.
13. A real installed run is supervised by its exact `run_id` and resumes recoverable failures on that run.
14. Delivery ends with clean, separately versioned distribution runtime and consumer knowledge changes.
15. Delegated work records role, agent identity, model, effort, input digest, output digest, and status.

## Trust boundary

The coordinator, installed kernel, and state directory run under one trusted OS
principal. Hashes prove deterministic integrity and lineage; they are not an
authorship signature against that same principal. Semantic authority comes from
the bounded independent reviewer artifact. A write-ready package without that
review is invalid. Deliberate fabrication of analysis, review, and all matching
digests by the trusted coordinator is out of scope unless a future deployment
adds an external signer or a distinct security principal.

## Failure taxonomy

Hard blockers are only the conditions in criterion 5. A semantic shortfall is a
safe fallback, not a hard blocker. Projection validation failures are
recoverable and must preserve the sealed gate. Apply failures preserve the
validated unit and checkpoint. Stale sources invalidate only the affected
package. A missing or mismatched run state is a contract error and must never
be silently replaced.

## Finding relevance

Record P0, P1, or P2 when a defect causes loss of knowledge or checkpoint,
publication without lineage, delegation outside the declared matrix, or a
failure of the definition of done. Duplicates and findings outside this scope
are documented as invalid or out of scope rather than promoted.
