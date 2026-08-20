# Execution record

Store resumable runs only under `VAULT_ROOT/.operations/<run-id>/run.md`. Keep the directory ignored by Git and outside Obsidian's durable graph.

## Run identity

Build the ID as `YYYYMMDD-HHMMSS-<short-slug>`. Before creating it, search active records for the same procedure, requester, target, and source anchor. Matching stable anchors select the existing run.

## Statuses

Use exactly:

- `draft`: preparing inputs or artifacts; no external effect is authorized.
- `awaiting-approval`: execution preview is ready.
- `in-progress`: at least one approved external step may execute.
- `blocked`: progress requires user input, a capability, or external state change.
- `completed`: all required completion evidence is present.
- `cancelled`: the user ended the run; existing external artifacts remain recorded.

For individual steps use `pending`, `approved`, `completed`, `not-applicable`, `blocked`, or `failed`.

## Recording rules

- Preserve the original procedure basename and its verification date.
- Record authorization scope and timestamp without copying hidden reasoning.
- Record external keys, URLs, message IDs, or timestamps immediately after success.
- Apply the skill guardrails and store only a short non-sensitive artifact summary.
- Record how each created artifact and required reference was resolved, matched, and access-checked without copying sensitive content.
- Mark a step completed only when its created artifacts and required references are verified against external authoritative state.
- On resume, reconcile every non-pending step against the external source when read access is available.
- Preserve failed and partial states; append the new state and explain the safe next action.

## Idempotency

Use `<run-id>:<step-id>` as the stable idempotency seed. When an external capability lacks native idempotency, search by the recorded target and artifact identifiers before retrying. Treat an ambiguous result as `blocked`.

## Completion

Require all mandatory steps to be `completed` or procedure-backed `not-applicable`, a populated completion-evidence section with existence, correspondence, and access results for every created artifact and required reference, and no pending approval or blocker.
