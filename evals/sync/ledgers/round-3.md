# Synchronization evaluation — round 3

## Immutable inputs

- Bundle/tree digest: `9d7ccbf06b0800ae0a28608c99335a7b2e6c6c81b351c3a96436c68ca72582ef`
- Review input digest: `fde71875be6f20d10ffab3c476f8c214f21324668314a2a1970a8df72ca6cb5e`
- Bundle test-receipt digest: `89a46e8efdb46e8bd3c4a2faef8406484ee3c5c3839ad734d544443594938d37`
- Review receipt digest: `d6e40f71e083970c8736945500dd06641f46e1f340d53eb785655013541b9aad`

## Normalized findings

All reported P1/P2 defects reproduce as public CLI or filesystem behavior and
are `relevant-fix`. `FUNC-003` and `REC-001` are one checkpoint recovery
cluster, not a duplicate: the former requires verifiable finalization lineage;
the latter requires retaining enough finalized material to rebuild authority.
Both aspects remain separate regressions.

| ID | Severity | Review finding | Regression | Disposition |
| --- | --- | --- | --- | --- |
| SYNC-R3-001 | P1 | FUNC-001 | `test_recheckpointed_stale_run_advances_to_seal_gate` | relevant-fix |
| SYNC-R3-002 | P1 | FUNC-002 | `test_closed_receipt_never_reuses_a_run_with_changed_source_identity` | relevant-fix |
| SYNC-R3-003a | P1 | FUNC-003 | `test_checkpoint_rejects_unverifiable_closed_digest_receipt` | relevant-fix |
| SYNC-R3-003b | P1 | REC-001 | `test_checkpoint_preserves_finalized_package_material_for_gate_recovery` | relevant-fix |
| SYNC-R3-004 | P1 | FUNC-004 | `test_acknowledgement_projection_preserves_prior_cursor_records` | relevant-fix |
| SYNC-R3-005 | P2 | FUNC-005 | `test_empty_file_create_and_delete_change_path_kind` | relevant-fix |
| SYNC-R3-006 | P2 | FUNC-006 | `test_malformed_unit_inputs_use_retryable_projection_envelope` | relevant-fix |
| SYNC-R3-007 | P1 | REC-002 | `test_seal_rejects_claim_authority_not_bound_to_checkpoint` | relevant-fix |
| SYNC-R3-008 | P2 | REC-003 | `test_write_projection_binds_declared_postimage_to_patch_bytes` | relevant-fix |
| SYNC-R3-009 | P1 | REC-004 | `test_token_like_patch_is_not_checkpointed_as_a_validated_unit` | relevant-fix |
| SYNC-R3-010 | P2 | REC-005 | `test_matching_applied_postimage_digest_is_reused` | relevant-fix |
| SYNC-R3-011 | P1 | REC-006 | `test_missing_checkpoint_after_source_invalidation_remains_recoverable` | relevant-fix |
| SYNC-R3-012 | P1 | REC-007 | `test_vault_symlink_parent_is_rejected_before_apply` | relevant-fix |

## Reproduction receipt

```sh
cd evals/sync
python3 -B -m unittest test_sync_pipeline.py test_sync_state.py
```

Baseline: 38 tests run, 24 inherited green, and 14 expected unittest failure
records for the 13 rows above. `SYNC-R3-006` deliberately has two malformed
input variants (`invalid-json` and `non-utf8`); neither is a fixture or import
error. No other failure is present.

```text
functional-review.json  674f1b5091b2146c0b89bcef1656ea9d20e6bd1a7ea4a5ed621ce7d92007f13b
recovery-review.json    152e2560e249884b2344f708aeec45c39e6de48e420dd9ea2afbf8a304e130a2
test_sync_pipeline.py   ca8decce3d82cbd3c900b853140fa5a9c2524f59861d0c4b69a5897cad3e17fd
test_sync_state.py      e014731a97b2a684e3a965a3bcedad68ab6db403d2829eca7d9852d5298a9c6a
focused-red-transcript  9b242008fefa9dd78301abc69922261547e0a80b05fc9f6130ab49e186db2a97
```

## Agent receipt

```text
agent_id: /root/sync_eval_round_2_regressions
role: round-3 sync regression author
model: gpt-5.6-terra
reasoning_effort: high
fork_turns: none
status: red-reproduced
input_digest: fde71875be6f20d10ffab3c476f8c214f21324668314a2a1970a8df72ca6cb5e
output_digests: test_sync_pipeline.py=ca8decce3d82cbd3c900b853140fa5a9c2524f59861d0c4b69a5897cad3e17fd; test_sync_state.py=f62db2077025fe89bdb5c77db2833213a34a2d863bb9d9d49de4efa751e18878
outputs: evals/sync/test_sync_pipeline.py, evals/sync/test_sync_state.py, evals/sync/ledgers/round-3.md
```
