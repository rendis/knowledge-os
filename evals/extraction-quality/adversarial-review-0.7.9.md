# Adversarial review of mapping integration

Reviewed revision: `baf2f98`; range: `b6ae0cc..baf2f98`. Two independent Sol-medium judges plus coordinator source inspection. Review only; no release or repair commit.

Verdict: **REVISE before final release**. Service extraction is useful, but the integrated external-evidence lifecycle is incomplete. The standalone PoC is directional evidence, not validation of the shipped analysis-to-publication path.

## Confirmed findings

1. **P1 — External reconciliation has no defined sync publication path.** `references/connection-reconciliation.md:3,28` requires separate reviewed external claims and note updates; `references/vault-synchronization.md:11` forbids changing the closed local package. `kernel/90-Meta/git-change-manifest.py:2185-2222` requires evidence paths to exist in the package's repository revision, and `kernel/90-Meta/sync-run.py:956-979` requires grants derived from closed packages. A live observation therefore cannot enter this flow as currently specified. Define an explicit reviewed external-evidence publication path, preserving the existing source package and evidence-policy boundaries. Sibling Git evidence can use its own package; it must not be relabeled as local source.

2. **P1 — Access readiness routes to the wrong capability.** `references/connection-reconciliation.md:7-13` asks generic operational readiness to establish provider/cluster/database access. Its capability matrix covers vault/source/Git readiness, while `configure-workspace/SKILL.md:71-88` binds procedures and explicitly does not authorize live execution. Route actual access checks through the configured adapter/executor; use workspace configuration only for missing bindings. A bounded read against the exact target should establish access before resuming.

3. **P2 — Final prose correctness remains an unbound coordinator check.** The package reviewer checks analysis before the coordinator builds Markdown. Projection validation (`git-change-manifest.py:1943-2049`) verifies structural authority and hashes, not semantic equivalence. `evidence-extraction.md:19` does require coordinator inspection of final prose, so the check is not absent; however, no independently bound final-prose review prevents a correct analysis from producing stale text. The datasource PoC failure makes a regression at this boundary necessary. Do not describe the current gate as deterministic stale-prose detection.

4. **P2 — Connection format is a prose convention.** `repository-map.md:30` requires many connection attributes, but persisted claims remain `{claim_id, statement, evidence}`. No typed fields validate status, close condition or connection identity. This does not alone invalidate semantic review; it means the shipped representation differs from the structured PoC and its machine-verification claims must be limited. Choose and test the smallest representation that supports the promised lifecycle before adding another framework.

5. **P2 — Stable connection IDs lack a persisted baseline reconciliation contract.** The scaffold starts with a prototype and finalization takes the submitted claim list (`git-change-manifest.py:2488-2494,817-827`). Prompt-level preservation exists, but prior IDs have no required preserve/update/retire decisions. Exercise destination changes, removal and unchanged pending items across real publication and subsequent sync; explicitly retain the minimum identity needed for resumption.

Reference paths beginning with `references/` above are relative to `kernel/.agents/skills/map-ecosystem/`.

## Verification and limits

- Coordinator: sync test suite **154 passed**, cell capability tests **6 passed**.
- Access judge: bootstrap **31 passed**, instance **8 passed**, extraction fixture **1 passed**, diff check passed.
- These checks verify existing mechanics; they do not exercise the new live evidence -> reviewed publication -> pending closure path.
- No final version bump, repair commit, consumer update or source-repository mutation was performed during this review.

## Finite release criteria

Repair the two lifecycle blockers and accurately define the three review/representation limits. Then run one integrated scenario: initial map -> reviewed publication with a pending connection -> missing access -> adapter readiness -> external evidence -> reviewed update and pending closure -> source delta -> final prose and identity preservation. Include a deliberately stale sentence and an unchanged commit with newly available external evidence. Reuse the existing test engine and allow one directed verification of repairs; do not restart a broad repository benchmark.

## Corrections applied (working tree)

- External observations now use the ordinary documentation route after local sync, with their own evidence and complete-note review. Closed Git packages stay immutable; source metadata is preserved for external-only updates.
- Target access is checked through the configured executor/adapter. Missing bindings and missing access have separate remedies. Cloud and infrastructure names are examples, not authorization or a platform restriction.
- `90-Meta/review-note-candidate.py` binds baseline, complete candidate, evidence, optional sync projection and independent review. Drift and a revision verdict block publication. The helper neither judges truth nor publishes notes.
- Connection attributes are explicitly a semantic prose convention. Durable note anchors plus required create/preserve/update/retire decisions establish the minimum persisted identity contract, without a second inventory. Local contract resolution does not automatically close an external pending verification.

Validation uses disposable local fixtures. The pending-to-closed lifecycle and stale-prose verdict are simulated review artifacts; they validate enforcement of a review decision, not an LLM's ability to discover stale prose. Real provider authentication, adapter execution and live evidence publication remain unverified. No consumer vault or source service was changed.

### Bounded repair review

The access judge found no remaining P1/P2 issue in the external publication/access scope. The map-contract judge confirmed the representation limits are stated honestly, but identified two follow-up defects: duplicate anchors were silently collapsed, and sync `apply-unit` could skip the instructional final review. Duplicate anchors now fail explicitly. `sync-run.py review-unit` binds acceptance to the stored projection; documentation application and postimage reconciliation require that receipt. Acknowledgement units remain exempt.

### Verification after repairs

- Bootstrap: 31 passed. Instance configuration: 8 passed. Extraction-quality suite, including final-note bindings: 12 passed.
- Sync suite: 156 executed, initially 154 passed and two inherited empty-file fixture cases failed because the synthetic review helper had not created the vault root. After fixing only that preparation, both failed instances plus final-review bypass and crash/resume regressions passed (4/4). The entire 156-test suite was not repeated after this fixture-only correction.
- Direct documentation apply without review and resume from an unreviewed postimage are rejected; reviewed apply succeeds. Existing acknowledgement units remain independent.
- Python compilation and `git diff --check` passed. No release/version bump or commit was performed in this correction pass.

The scoped corrections are implemented. The earlier end-to-end live scenario remains unverified: simulated evidence and supplied verdicts do not establish real access readiness or semantic judge accuracy.
