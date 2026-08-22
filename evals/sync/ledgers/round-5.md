# Synchronization evaluation — round 5

## Immutable evidence

- Bundle/tree: `c0b4b91e32eb6a5b189f0df7b166bcf3793b76ec1cc6e8b9176ac432675a8964`
- Review input: `648ef30bb7ed4fd2c4f593b71fa68eef48d8bedba9f517a839c8b63b2f8827ad`
- Bundle test receipt: `359de8cb732ae31e0e6c1ce2557d8b136ab3c8723e035d10a7391d93e2570f7a`
- Functional review: `a7634dc6eaf7d326776460f61aab625419716e497e50cdbe5ed1737443010605`
- Recovery review: `f31d538d3df6a8e0b86ece489ce518861da8e36414fa9c441aeb46d9694679ed`

## Contracts

| ID | Severity | Requirement | Public regression | Current result |
| --- | --- | --- | --- | --- |
| FUN-001 | P1 | REQ-008 | `test_checkpoint_requires_review_or_fallback_for_write_authority` | historical RED; green current |
| FUN-002 | P1 | REQ-002 | `test_sealed_spaced_node_name_remains_resumable` | historical RED; green current |
| FUN-003 | P1 | REQ-003 | `test_direct_apply_retry_rejects_mixed_crash_postimage` | historical RED; green current |
| FUN-004 | P2 | REQ-006 | `test_backslash_projection_path_uses_retryable_invalid_projection_envelope` | historical RED; green current |
| REC-001 | P1 | REQ-004 | `test_checkpoint_does_not_claim_same_principal_authentication` | out-of-scope; trust boundary frozen |
| REC-002 | P2 | REQ-008 | `test_checkpoint_rejects_embedded_absolute_source_path` | historical RED; green current |
| REC-003 | P1 | REQ-005 | `test_write_group_cannot_target_agent_instruction_paths` | historical RED; green current |
| REC-004 | P1 | REQ-009 | `test_reseal_cannot_close_while_withdrawn_applied_postimage_remains` | historical RED; green current |
| REC-005 | P2 | REQ-009 | `test_reseal_preserves_validated_group_when_ordinal_changes` | historical RED; green current |
| REC-006 | P2 | REQ-001 | `test_source_drift_reuses_matching_closed_receipt` | historical RED; green current |
| REC-007 | P2 | REQ-013 | `test_status_prefers_valid_closed_receipt_after_interrupted_cleanup` | historical RED; green current |

FUN-002 uses the bundle's exact `fun-002-repro.py` fixture and is now green in
the shared runtime: the persisted spaced node reloads successfully. Its bundle
reproduction remains the historical RED evidence. REC-004 now requires resume
to retract the invalidated unit to its exact preimage before reseal and permits
close only while that preimage remains restored. REC-005 uses the bundle's
exact cursor-ready package and generated-gate route and reaches the reviewed
reseal contract before failing.

## RED receipt

```sh
cd evals/sync
python3 -B -m unittest \
  test_sync_pipeline.ResumableSyncEval.test_write_group_cannot_target_agent_instruction_paths \
  test_sync_state.SyncRunStateEval.test_checkpoint_requires_review_or_fallback_for_write_authority \
  test_sync_state.SyncRunStateEval.test_direct_apply_retry_rejects_mixed_crash_postimage \
  test_sync_state.SyncRunStateEval.test_backslash_projection_path_uses_retryable_invalid_projection_envelope \
  test_sync_state.SyncRunStateEval.test_checkpoint_does_not_claim_same_principal_authentication \
  test_sync_state.SyncRunStateEval.test_checkpoint_rejects_embedded_absolute_source_path \
  test_sync_state.SyncRunStateEval.test_sealed_spaced_node_name_remains_resumable \
  test_sync_state.SyncRunStateEval.test_reseal_cannot_close_while_withdrawn_applied_postimage_remains \
  test_sync_state.SyncRunStateEval.test_reseal_preserves_validated_group_when_ordinal_changes \
  test_sync_state.SyncRunStateEval.test_source_drift_reuses_matching_closed_receipt \
  test_sync_state.SyncRunStateEval.test_status_prefers_valid_closed_receipt_after_interrupted_cleanup
```

Current baseline: 11 tests green with no fixture/import/harness errors.
REC-001 remains as a positive boundary test: self-hashes do not pretend to
authenticate the trusted coordinator against itself.
Historical fixture
digests: `fun-002-repro.py=658778fa42b26d6ac716ca53e55a1a5af418d6d9803075da88c57d33aaddf590`,
`rec-004-005-repro.py=878bc65814d9c248e042eff3699256480b352e24cf8380650583730f056b28ec`.
Test-source digests: `test_sync_pipeline.py=ae22536172b34ff0b98d21cf6ca571fb5b9d3f024fd23733abd9f43ae6265055`,
`test_sync_state.py=43a4deef1489f81daa1342e726606e311e031fa35357ca390ec5d4f9c0fae2ec`.
