# Adversarial closure review

Scope: current uncommitted template changes on `baf2f98`, including the final-note helper and sync receipt enforcement. Two fresh Sol-medium judges plus coordinator triage. Review only; no implementation changes or release.

## Initial verdict

REVISE: one P2 remained before the closure correction below.

### P2 — final review does not bind empty-file versus deletion kind

`kernel/90-Meta/review-note-candidate.py:218-224` compares only projection base/result hashes. An absent result and an empty Markdown file both use SHA-256 of empty bytes. The same deletion projection can therefore freeze against either a staged empty file or an explicit `--delete`; both pass with the same projection digest. `review-unit` does not compare manifest deletion/presence information with the patch-derived `base_kinds`/`result_kinds`, while `apply-unit` executes that patch kind. Approval can describe an empty retained file while publication removes it, or the inverse.

A complete temporary-fixture reproduction passed projection validation, freeze, review-unit and apply-unit; the reviewed zero-byte file was absent after application.

Impact is limited to empty-result path identity, not arbitrary nonempty-content substitution. Bind manifest presence/deletion to the stored unit kinds and add both mismatch directions as regressions. No broader redesign is needed.

## Triaged observations

- Unaffected-unit rebinding is not a semantic blocker. Reuse requires the same repositories, nodes, authority and exact patch bytes; only transport identifiers change. The historical manifest/new receipt relationship is a provenance limitation, not permission to publish changed content.
- Already-applied legacy units can close without the new review receipt. This concerns pre-upgrade publication, not bypassing review for a new write. The migration comment should describe that compatibility exception accurately; pending validated units do require review.
- Live executor/authentication and semantic-review accuracy remain outside these fixture tests. Their absence is not a newly discovered code defect.

## Verification

- Full sync suite: 156/156 passed in 156 seconds.
- Bootstrap: 31/31 passed.
- Instance configuration: 8/8 passed.
- Extraction-quality/final-note suite: 12/12 passed.
- `git diff --check`: passed.

Passing suites do not cover the identified kind mismatch. Consumer vaults, service repositories, infrastructure and version/commit state were not changed.

## Closure correction — 0.7.10

`review-unit` now compares manifest `base_present` and `deleted_files` against the validated patch's `base_kinds` and `result_kinds` before saving acceptance. Kind mismatches block review and subsequent application. The regression exercises both empty-file/deletion inversions, verifies the existing file stays untouched on rejection, and confirms correctly reviewed operations can publish. The legacy migration comment now explicitly preserves already-applied publication status.

The independent engine judge verified this specific repair and reported no remaining finding. The earlier legacy-state observation is documented as compatibility for already-published bytes.

Final verification for 0.7.10: sync 158/158, bootstrap 31/31, instance 8/8, extraction-quality 12/12, and staged diff checks passed. Verdict: CLOSED for the scoped mapping corrections. Live infrastructure remains an explicitly unverified capability, not a claimed test result.
