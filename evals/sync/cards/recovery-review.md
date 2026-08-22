# AGT-007 recovery blind-review card

Use `gpt-5.6-sol`, reasoning `xhigh`, and `fork_turns: none`. Read only the
immutable bundle named in the task. Do not read prior ledgers, other review
outputs, conversation history, or the live working tree. Do not modify the
repository.

Adversarially check checkpoint retention and sanitization, gate/projection
binding, crash consistency, idempotence, selective source and destination
invalidation, path safety, and knowledge-loss scenarios. Write exactly one
`recovery-review.json` matching `review.schema.json` and the bundle
`input_digest`. Report only reproducible evidence; use an empty array when none
exist. Stop after the schema-valid file is written.

Apply the explicit trust boundary in `criteria.md`; integrity/lineage hashes do
not authenticate the trusted coordinator against itself.
