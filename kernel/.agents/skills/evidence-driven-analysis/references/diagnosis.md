# Diagnosis

Use when explaining a failure, not as an automatic mandate to fix it.

- Establish the actual symptom and expected result from the relevant contract. Align the failing observation with its input, execution identity, revision, and environment; a nearby error may be unrelated.
- Inspect the narrow path from input through transformation to outcome. Compare a working case or recent relevant change when available. Identify assumptions at component boundaries instead of attributing failure from a component name.
- Treat explanations as provisional. Choose a permitted check whose possible outcomes distinguish the plausible causes, including evidence against the leading explanation. Revise or reject it when its prediction fails; do not merely accumulate confirming logs.
- Prefer existing tests, traces, code, and read-only observations. A safe reproduction can strengthen a diagnosis but is not required to answer a source-level question. If the needed check requires instrumentation, writes, or inaccessible runtime data, describe that boundary and request the specific authorization or sanitized evidence; do not perform it implicitly.
- Conclude at the supported level: demonstrated cause, plausible explanation with a missing discriminator, or unresolved cause. Keep suggested remediation separate from implemented and verified remediation.
