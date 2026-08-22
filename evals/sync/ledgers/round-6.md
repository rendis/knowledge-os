# Synchronization evaluation — round 6

## Immutable evidence

- Bundle/tree: `97f2c80af2f4f7bcdc4110c33d00f99f5f45935bf62979de1b5dcd6ce8642e84`
- Review input: `cf234155e7a484f7b9a1499b213838b8f341ab701e30fc907a55de7c6bd3b120`
- Bundle test receipt: `1ecd8b6022d46fdbd7b95bc8aeb403f8638929bc964878e1af0d5fc73bf5a04e`
- Review receipt: `097618d37075f378a5bb52afbd84003fd01b91cf664e55d54e6f89ab0a5bac3d`
- Functional review: `6d7182ee25e35fb73f532fd5fb42c89fbb40c187b3423d2dfa28fe10decc4012`
- Recovery review: `1138ed477775f42264db522156b8b47f16d0f7b665e2f9263daab343f2d32111`

## Contracts

| ID | Severity | Requirement | Public regression | Disposition |
| --- | --- | --- | --- | --- |
| FUNC-001 | P1 | REQ-009 | `test_close_package_rechecks_the_declared_branch_oid` | relevant-fix |
| FUNC-002 | P2 | REQ-013 | `test_destination_drift_resume_does_not_start_source_retraction` | relevant-fix |
| REC-001 | P1 | REQ-009 | `test_convergent_receipt_restores_receipt_postimage_before_retiring_run` | relevant-fix |
| REC-002 | P1 | REQ-009 | `test_reseal_keeps_failed_source_retraction_as_a_close_prerequisite` | relevant-fix |

## RED receipt

```sh
cd evals/sync
python3 -B -m unittest \
  test_sync_pipeline.ResumableSyncEval.test_close_package_rechecks_the_declared_branch_oid \
  test_sync_state.SyncRunStateEval.test_destination_drift_resume_does_not_start_source_retraction \
  test_sync_state.SyncRunStateEval.test_convergent_receipt_restores_receipt_postimage_before_retiring_run \
  test_sync_state.SyncRunStateEval.test_reseal_keeps_failed_source_retraction_as_a_close_prerequisite
```

Baseline: 4 tests, 4 contractual REDs, with no fixture/import/harness errors.

Current result after correction: 4 tests green. Source and destination staleness
are causally distinct; source retraction precedes reseal/receipt convergence,
and package closure rechecks the declared branch OID.

```text
test_sync_pipeline.py  12cdb8336e4fe0ad0643ed8cfde5ed4b12c2b26a4993e1e6f639ce55461258ec
test_sync_state.py     a20820c25508bf34aae07c8028033be55a83babdc0acdb4c579498a16f7af51f
```

## Agent receipt

```text
agent_id: /root/sync_eval_round_2_regressions
role: round-6 sync regression author
model: gpt-5.6-terra
reasoning_effort: high
status: red-reproduced
```
