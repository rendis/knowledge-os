# Blind synchronization review contract

Each review round is bound to one immutable evaluation bundle. Both reviewers
receive the same `review-input.json`, snapshot, diff, criteria, and test receipt.
They do not receive prior ledgers, authorship, expected findings, or the other
reviewer's output.

The reviewer writes exactly one JSON object accepted by
`review.schema.json`. `input_digest` must equal the SHA-256 recorded in
`review-input.json`. A clean review uses an empty `findings` array; it does not
invent a success finding.

Functional review checks public contracts, state transitions, isolation,
determinism, installation behavior, and test gaps. Recovery review checks
checkpoint preservation, sanitization, binding, crash reconciliation,
selective invalidation, path safety, and knowledge-loss risks.

Reviews apply the trust boundary in `criteria.md`: checkpoint hashes establish
integrity and lineage, not authentication against the trusted coordinator
running as the same OS user. Report missing/mismatched independent review or
lineage as a defect; do not treat deliberate same-principal fabrication of all
semantic artifacts and hashes as an implementable local-kernel finding.

Reviewers report evidence, not fixes. The root agent reproduces every P0-P2
finding and assigns the ledger disposition `relevant-fix`, `invalid`, or
`out-of-scope`. A finding does not require reviewer consensus to be relevant.

Review files are written once beneath the bundle directory. A new review uses
fresh agents and a new bundle only after the candidate tree digest changes.
