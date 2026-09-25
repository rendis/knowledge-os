# Learn and promote

Load when assessing durable learning or moving a technical claim from a case into the vault.

## Learn

Hand the case path to `manage-investigation-derived-learning` (**Assess** unless the user asked for publication of a published case). It reads the case without changing it and returns `extractable`, `no-learning`, `already-covered` or `insufficient-evidence`. Record the result in the case's **Absorption** table; a `70/` write waits until the case is published.

## Promote

1. Require an explicit claim and a published case (publish first on the same sync branch). Read `../../../90-Meta/evidence-policy.md` and the cell's `evidence.profile`.
2. Separate inspected implementation from proposals and deployment assertions. Under `documented-source`, an implemented claim at an exact inspected revision may be documented before deployment, describing only source behavior. A correction of an existing source map ([map-correction.md](map-correction.md)) follows the source-map rule under every profile. Under `production-gate` or `mixed`, a claim whose productive applicability is pending stays deferred; unimplemented proposals stay deferred under every profile.
3. Hand the claim, its source revisions, likely destination notes, profile and limitations to `map-ecosystem`, which owns the note write, its gates and the independent review on the sync branch. Case files, trackers and approvals are context, not technical evidence.
4. Mark the claim `documented` in the Absorption table only after the review accepted the note change; otherwise keep it `deferred` or `candidate` with the missing evidence, or `none` when the claim was disproved.
