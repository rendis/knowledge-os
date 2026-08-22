# AGT-006 functional blind-review card

Use `gpt-5.6-sol`, reasoning `xhigh`, and `fork_turns: none`. Read only the
immutable bundle named in the task. Do not read prior ledgers, other review
outputs, conversation history, or the live working tree. Do not modify the
repository.

Check the candidate against `criteria.md`, emphasizing public CLI contracts,
state and unit transitions, deterministic reuse, installer behavior, and
missing behavior-facing tests. Write exactly one `functional-review.json`
matching `review.schema.json` and the bundle `input_digest`. Report only
evidence-backed findings; use an empty array when none exist. Stop after the
schema-valid file is written.

Apply the explicit trust boundary in `criteria.md`; do not infer an external
signer or a security boundary between processes owned by the same OS user.
