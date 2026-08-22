# Synchronization evaluation — round 4

## Immutable inputs

- Bundle/tree digest: `99543dcc28fe71c849f4e7e360900c036952e9b44d13d1a0d66632082c994d7d`
- Review input digest: `597113915937b68f59e9989c2f9f40cea9ebf554f11f97c2daec7bc7a1b0971a`
- Criteria digest: `65c738ab3e768f043ca7b59fcb5ef7b91c57ec0f0ace4cbe5831b1707a7bed79`
- Bundle test-receipt digest: `3f47332181fa8343f1042d0d4f336c00a1280ddbd9320b696bbbaf0c08a7ffa5`
- Review receipt digest: `5611d81ab23e04f2f407600a0c55d0b2820a34b7a22207df5ed1d7e8fbf48975`

## Normalized findings

All 12 reported P1/P2 findings reproduce through public CLI or filesystem behavior and are `relevant-fix`. `FUNC-003` and `REC-001` are one package validation cluster: both reject self-attestation, while retaining their independent subcases (an inconsistent embedded gate and fabricated analysis). `REC-002` is `relevant-fix` by explicit authority: traceability-only work must produce an acknowledgement with no write group, never arbitrary Markdown.

| ID | Severity | Review finding | Regression | Disposition |
| --- | --- | --- | --- | --- |
| SYNC-R4-001 | P2 | FUNC-001 | `test_new_run_advertises_checkpoint_before_seal_gate` | relevant-fix |
| SYNC-R4-002 | P2 | FUNC-002 | `test_begin_reuses_active_run_after_source_fingerprint_refresh` | relevant-fix |
| SYNC-R4-003a | P1 | FUNC-003, package-validation cluster | `test_checkpoint_rejects_self_digested_inconsistent_package_gate` | relevant-fix |
| SYNC-R4-003b | P1 | REC-001, package-validation cluster | `test_checkpoint_rejects_self_digested_fabricated_analysis` | relevant-fix |
| SYNC-R4-004 | P2 | FUNC-004 | `test_wrong_projection_unit_uses_retryable_projection_envelope` | relevant-fix |
| SYNC-R4-005 | P2 | FUNC-005 | `test_acknowledgement_projection_uses_the_gate_canonical_date_and_branch` | relevant-fix |
| SYNC-R4-006 | P1 | REC-002 | `test_traceability_only_cannot_create_a_markdown_write_group` | relevant-fix |
| SYNC-R4-007 | P1 | REC-003 | `test_write_group_projection_covers_every_granted_node` | relevant-fix |
| SYNC-R4-008 | P1 | REC-004 | `test_partial_postimage_between_files_blocks_or_recovers_atomically` | relevant-fix |
| SYNC-R4-009 | P2 | REC-005 | `test_reseal_preserves_unaffected_apply_failed_projection` | relevant-fix |
| SYNC-R4-010 | P2 | REC-006 | `test_corrected_projection_metadata_reuses_the_same_patch` | relevant-fix |
| SYNC-R4-011 | P1 | REC-007 | `test_sk_projection_token_is_not_checkpointed_in_a_validated_patch` | relevant-fix |

## Reproduction receipt

```sh
cd evals/sync
python3 -B test_sync_pipeline.py
python3 -B test_sync_state.py
```

Baseline: 49 tests run, 37 green, and exactly 12 expected failures: the 12 rows above. The pipeline suite contributes `SYNC-R4-005` and `SYNC-R4-007`; the state suite contributes the remaining ten. No failure is a fixture, import, or harness error.

```text
functional-review.json  f7d11e431ff6cc95ef74516232de05d4089d346e7a65f03bbb20020e9ca7acd9
recovery-review.json    9e22954c569e693def3490f41de742399edd770643fbba3c41d69bac009a0ed5
test_sync_pipeline.py   4802880d6a47059d9e6405b9c495d47faf3e8824511211ee5226373a3315074a
test_sync_state.py      064ed1914dae64d9643ad406704b29a0f1d798dc6777f4612952d0cdb28a4d7b
```

## Agent receipt

```text
agent_id: /root/sync_eval_round_2_regressions
role: round-4 sync regression author
model: gpt-5.6-terra
reasoning_effort: high
fork_turns: none
status: red-reproduced
input_digest: 597113915937b68f59e9989c2f9f40cea9ebf554f11f97c2daec7bc7a1b0971a
output_digests: test_sync_pipeline.py=4802880d6a47059d9e6405b9c495d47faf3e8824511211ee5226373a3315074a; test_sync_state.py=064ed1914dae64d9643ad406704b29a0f1d798dc6777f4612952d0cdb28a4d7b
outputs: evals/sync/test_sync_pipeline.py, evals/sync/test_sync_state.py, evals/sync/ledgers/round-4.md
```
