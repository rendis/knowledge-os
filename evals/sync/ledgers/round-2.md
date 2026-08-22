# Synchronization evaluation — round 2

## Immutable inputs

- Bundle digest: `b06b04f42fa390a894012bc92decc0d2c1d19a821c7582a849e7250b40ddcdae`
- Review input digest: `286bc84f74b8c75c6d86e23434ba89bca0a2b95bc76278e67d111a14cf230c96`
- Functional review: `functional-review.json` (SHA-256 recorded below)
- Recovery review: `recovery-review.json` (SHA-256 recorded below)

## Normalized findings

All 11 distinct P0–P2 findings are `relevant-fix`. The evaluation additions
are public-CLI and temporary-filesystem regressions; they neither import nor
patch the runtime.

| ID | Severity | Reviewer mapping | Public regression | Disposition |
| --- | --- | --- | --- | --- |
| SYNC-R2-001 | P1 | FUNC-001, REC-002 | `test_seal_requires_a_finalized_checkpoint_and_complete_gate_inventory` | relevant-fix |
| SYNC-R2-002 | P1 | FUNC-002, REC-004 | `test_begin_reuses_an_integrity_checked_closed_receipt` | relevant-fix |
| SYNC-R2-003 | P1 | FUNC-003, REC-001 | `test_validate_unit_requires_the_exact_supervised_run_id` | relevant-fix |
| SYNC-R2-004 | P1 | FUNC-004, REC-005 | `test_applied_unit_is_invalidated_when_authority_or_postimage_drifts` | relevant-fix |
| SYNC-R2-005 | P1 | REC-003 | `test_acknowledgement_projection_binds_its_result_to_the_gate` | relevant-fix |
| SYNC-R2-006 | P2 | FUNC-005, REC-007 | `test_validator_rejects_patch_syntax_the_applier_cannot_apply` | relevant-fix |
| SYNC-R2-007 | P1 | FUNC-006, REC-008 | `test_write_error_between_files_leaves_no_partial_publication_and_is_recoverable` | relevant-fix |
| SYNC-R2-008 | P2 | REC-006 | `test_reseal_preserves_an_unaffected_validated_unit` | relevant-fix |
| SYNC-R2-009 | P1 | REC-009 | `test_nested_state_symlink_is_rejected_before_checkpoint_write` | relevant-fix |
| SYNC-R2-010 | P1 | REC-010 | `test_checkpoint_rejects_raw_token_like_claim_material` | relevant-fix |
| SYNC-R2-011 | P1 | REC-011 | `test_closed_receipt_requires_its_exact_run_identity_and_digest` | relevant-fix |

## Reproduction receipt

Command (from `evals/sync/`):

```sh
python3 -B -m unittest test_sync_pipeline.py test_sync_state.py
```

Result: 25 tests run; 14 existing tests pass; the exact red evidence is the
11 normalized entries above. `SYNC-R2-004` has independent `source` and
`destination` subcases, and `SYNC-R2-006` has independent `missing-hunk` and
`trailing-garbage` subcases, yielding 13 unittest failure records without a
fixture, import, or setup error.

| Finding | Observed public result |
| --- | --- |
| SYNC-R2-001 | `seal-gate` returns `gate-sealed` with `checkpointed: 0` and an empty gate. |
| SYNC-R2-002 | identical `begin` after close returns `reused: false`, `status: packages`. |
| SYNC-R2-003 | a projection bearing another valid run ID returns `projection-valid`. |
| SYNC-R2-004 | applied group is omitted from both source and destination `invalidated_units`. |
| SYNC-R2-005 | acknowledgement patch containing only `{"version":1}` returns `projection-valid`. |
| SYNC-R2-006 | validator returns `projection-valid` for both absent-hunk and trailing-garbage patches. |
| SYNC-R2-007 | a later filesystem write failure leaves the first file at its postimage. |
| SYNC-R2-008 | after source reseal, an unaffected validated group returns to `pending`. |
| SYNC-R2-009 | nested symlink checkpoint succeeds and writes outside the state root. |
| SYNC-R2-010 | a raw GitHub-token-shaped claim checkpoints successfully. |
| SYNC-R2-011 | forged closed receipt is returned successfully by `status`. |

## Digests and agent receipt

The `shasum -a 256` values below bind the immutable reviews and the evaluator
artifacts as observed for this round.

```text
agent_id: /root/sync_eval_round_2_regressions
role: AGT-002 regression author
model: gpt-5.6-terra
reasoning_effort: high
fork_turns: none
status: red-reproduced
inputs: criteria.md, review-contract.md, functional-review.json, recovery-review.json
outputs: evals/sync/test_sync_pipeline.py, evals/sync/test_sync_state.py, evals/sync/ledgers/round-2.md
```

The exact SHA-256 values are appended after the evaluator artifacts are
written, so the receipt is auditable without mutating runtime or consumer
files.

```text
functional-review.json  49547c08f2c409f13b7a38c5401dfef788174d59e865669ad0b7d7601cafee50
recovery-review.json    d209ddccee4e1a69c1c778b24948067c83292d84f484b3620e197b0d25810ba9
test_sync_pipeline.py   7072b41e80728d0936c8b5e0cce69956072a23fd4979b9e660870c3daa4bb02d
test_sync_state.py      084cd748c5f48898f373a7462aa2a4f2b8d6ca5644f84dae19975367ba4dc9b9
focused-red-transcript  adf02b8f3cf4157dc74a316c6e8ed3f61e242d47ad9d5cd95f29ff7a4a5791e8
```
