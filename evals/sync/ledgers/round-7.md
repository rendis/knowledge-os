# Synchronization evaluation — round 7

## Immutable evidence

- Bundle/tree: `0c203819ece19bf9e2c5086c7cd0ae08cac802cafdd47c7580cf0a38af973b24`
- Review input: `40edbca6bd74ae168fb6c1afce248c33e22ba8e7cd9ebb7e081c6e67bc693261`
- Bundle test receipt: `64b94ac36bd7db385b01d7ae5da8b5b430551b977be00cf2b6fc0fcdd3c32564`
- Review receipt: `5cd4b70c4c769844a99a287d1391ccbbcea3efd86cb863d59b56da068d289520`
- Functional review: `390e050374d5b9ccdf2f645d824dbe2f92a32473c433cb78e61f315925c36d00`
- Recovery review: `541ce4fe4eba0e8a38d37f86bae1088b2809c4a5649476f1e66d380e50319e94`

## Bounded disposition

| ID | Severity | Requirement | Disposition |
| --- | --- | --- | --- |
| FUN-001 | P2 | REQ-005 | Deferred: byte-identical projections can be misclassified as mixed images. Outside the final P0/P1 correction limit. |
| REC-001 | P1 | REQ-013 | Relevant-fix: a matching durable receipt now idempotently retires an orphan active checkpoint; a new fingerprint can begin without manual state deletion. |

No round 8 is opened. Terminal verification is limited to the final diff,
repository checks, installer promotion, and the real consumer synchronization.

## Focused regression

```sh
cd evals/sync
python3 -B -m unittest \
  test_sync_state.SyncRunStateEval.test_status_prefers_valid_closed_receipt_after_interrupted_cleanup
```

Current result: 1 test green. The test restores a partially cleaned active run
after its durable receipt exists, retries `close`, proves the active orphan is
retired, and begins a different inventory fingerprint successfully.
