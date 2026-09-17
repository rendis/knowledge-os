# Learn and promote

Load this reference only when assessing durable learning or promoting a technical claim from a case.

## Learn

1. Require one selected case and read its entire current snapshot before the handoff.
2. Hand the exact investigation path and requested mode to `manage-investigation-derived-learning`. Use **Assess** unless the user explicitly requested durable publication or revalidation of a **published** case. An unpublished case may be assessed; a `70/` write waits for investigation **Publish**.
3. Let that skill inspect sources, search existing learnings, apply its independent gate, and present one of `extractable`, `no-learning`, `already-covered`, or `insufficient-evidence`. Do not pre-classify the answer or treat case status as proof.
4. Keep the dependency one-way during assessment: the learning skill reads but never mutates the case. After it returns, map `no-learning`, `already-covered`, and `insufficient-evidence` directly; map an unpublished `extractable` result to `candidate`; map a verified durable write (published case only) to `documented`.
5. When persisting the returned status, update `learning-outcome`, `updated-at`, Readiness, and History together with the assessed case snapshot, canonical target if any, evidence boundary, lifecycle action, and observed checks. A blind test or explicit assess-only request that forbids persistence ends before this step.
6. Do not change `vault-outcome`: technical production documentation and investigation-derived learning are independent paths.

Complete when the user has the explicit assessment result and action, no forbidden test or assess-only state was persisted, and any authorized case update accurately records the returned snapshot without replacing its evidence.

## Promote

1. Require an explicit candidate claim, a classified `purpose`, and a published source case. An unpublished case takes investigation **Publish** before this route writes `10/`–`70/`. Read `../../../90-Meta/evidence-policy.md` and the cell's `evidence.profile` before deciding eligibility.
2. Separate inspected implementation from proposals and deployment assertions. Under `documented-source`, an implemented claim at an exact inspected revision may proceed before deployment; describe only source behavior. For corrections of existing source maps under [map-correction.md](map-correction.md), apply the source-map rule in evidence-policy.md under every profile and describe only inspected source/configuration. For other promotion under `production-gate` or `mixed`, set `vault-outcome: deferred-until-production` when required productive applicability remains pending or ambiguous. Unimplemented proposals remain deferred under every profile.
3. Set an eligible claim to `vault-outcome: candidate-for-audit` and build a context package with exact claims, source revisions, likely canonical nodes, evidence profile and limitations. Case files, trackers and approvals provide context, not technical evidence.
4. Hand the package and established vault-write authorization to `map-ecosystem` for an independent profile-specific audit. An explicit post-deployment audit may recheck productive applicability even when source HEAD is unchanged. That skill owns evidence assessment, node lifecycle, writes, propagation and verification.
5. Set `vault-outcome: documented` only after the independent audit passes and the canonical vault already represents the claim correctly or was updated and verified. Record canonical notes, evidence profile, evidence boundary, lifecycle result and observed checks in Readiness and History. Otherwise retain the exact missing evidence with `candidate-for-audit`, use `none` for a disproved or removed candidate, or defer a proposal or required deployment.

Complete when every candidate has an explicit outcome and each documented claim traces to an independent audit at the cell's required evidence level. Source-only claims remain explicitly scoped to their inspected revision.
