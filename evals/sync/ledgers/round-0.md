# Sync evaluation round 0

Candidate tree digest before this ledger:
`c9ba99d2a38b71a199cbb4943cd0f94ace33319e0c1cce6154465293a3c6681c`.

## Commands and results

```text
python3 -B evals/sync/test_sync_pipeline.py
exit 1; 4 tests, 5 expected assertion failures
```

- `test_accepted_cross_repository_grant_survives_rejected_target_review`
  received gate `version: 1`, not the required v2 grant-bearing gate. It uses
  generic `APP00000-source-adapter` accepted claim `claim-source-001` to write
  `target-service`, while the target package is `review-rejected`. (REQ-004,
  REQ-007)
- `test_projection_invalid_for_rejected_or_invented_grants` received
  `usage-error` because `validate-projection` is absent instead of the required
  `projection-invalid`, `retryable: true`, `resume_from: projection` response
  for both rejected and invented grants. (REQ-004, REQ-006, REQ-007)
- `test_acknowledgement_and_write_group_validate_as_independent_units` received
  the same absent-command `usage-error` instead of independently passing the
  acknowledgement and `group-001` units. (REQ-003, REQ-007)
- `test_begin_is_deterministic_and_resume_requires_durable_run_state` could not
  execute `kernel/90-Meta/sync-run.py`, which does not exist. (REQ-002,
  REQ-010)

```text
python3 -B evals/bootstrap/test_sync_tooling.py
exit 1; 5 tests, 4 expected assertion failures
```

- Public help lacks `validate-projection` and still exposes `validate-write`.
  (REQ-007)
- The retired source markers remain: `validate-write`,
  `content.count(basename)`, and `rejected-repository-added`. (REQ-004,
  REQ-007)

```text
python3 -B -m unittest \
  evals.bootstrap.test_sync_tooling.SyncToolingEval.test_short_credential_literals_are_detected_and_redacted \
  evals.bootstrap.test_sync_tooling.SyncToolingEval.test_credential_surfaces_exclude_canonical_identity \
  evals.bootstrap.test_sync_tooling.SyncToolingEval.test_promoted_clis_expose_help
exit 0; 3 tests passed

python3 -B evals/bootstrap/test_bootstrap.py
exit 0; 9 tests passed

git diff --check
exit 0
```

## Receipt

| Field | Value |
|---|---|
| role | AGT-002 |
| agent_id | /root/sync_eval_round_0 |
| model | gpt-5.6-terra |
| reasoning_effort | high |
| fork_turns | none |
| status | expected-red-baseline |
| input_digest | b53c36f8cb095b15ac8fb3abc22f06e9ce9cd3e8581a4b67191597d8b3f1960f |
| output_digest | 327c67f1459bbb18896c11b7b04d4fcbf4aa167d4be3e187a8a858c5b692f0d8 |

The input digest covers `AGENTS.md`, `README.md`, `MANAGED_PATHS`, the confirmed
architecture plan, and the pre-existing manifest helper. The output digest
covers the four test/fixture/criteria outputs, excluding this self-referential
ledger.
