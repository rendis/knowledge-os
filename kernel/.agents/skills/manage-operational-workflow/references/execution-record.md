# Execution record

Store resumable runs only under `VAULT_ROOT/.operations/<run-id>/run.md`. Keep the directory ignored by Git and outside Obsidian's durable graph.

## Run identity

Build the ID as `YYYYMMDD-HHMMSS-<short-slug>`. Before creating it, search active records for the same procedure, requester, target, and source anchor. Matching stable anchors select the existing run.

For Audit, a match also requires the same branch, environment, period boundaries, timezone, and entity filters. A difference creates a separate run unless the user explicitly expands the existing scope; record that expansion and preserve the original scope and observations.

For an agreed observation campaign, [observation.md](observation.md) defines the overall period and plan revision once. Its bounded cycles are observations/steps in that run, not separate campaign identities. Reload the plan in each session, preserve interval/revision identity on retries and keep one writer. Record scheduler disposition locally without duplicating the plan's authority.

## Statuses

Use exactly:

- `draft`: preparing inputs or artifacts; no external effect is authorized.
- `awaiting-approval`: execution preview is ready.
- `in-progress`: authorized evidence collection is underway or an approved external step may execute, within the recorded branch's effect boundary.
- `blocked`: progress requires user input, a capability, or external state change.
- `completed`: all required completion evidence is present.
- `cancelled`: the user ended the run; existing external artifacts remain recorded.

For individual steps use `pending`, `approved`, `completed`, `not-applicable`, `blocked`, or `failed`.

## Recording rules

- Preserve the original procedure basename and its verification date.
- Record the branch (`Audit`, `Execute`, or `Draft`) and procedure revision. Resume preserves that branch and its effect boundary.
- Record authorization scope and timestamp without copying hidden reasoning.
- Record external keys, URLs, message IDs, or timestamps immediately after success.
- Apply the skill guardrails and store only a short non-sensitive artifact summary.
- Record how each created artifact and required reference was resolved, matched, and access-checked without copying sensitive content.
- Mark an external-effect step completed only when its created artifacts and required references are verified against external authoritative state.
- For an audit read, mark the step completed when its observation, provenance, coverage, and finding are recorded under `read-only-audit.md`; no external artifact creation is required.
- On resume, reconcile external-effect steps against the external source when read access is available. Preserve historical audit observations and refresh only evidence required for pending questions.
- Preserve failed and partial states; append the new state and explain the safe next action.

## Idempotency

Use `<run-id>:<step-id>` as the stable idempotency seed. When an external capability lacks native idempotency, search by the recorded target and artifact identifiers before retrying. Treat an ambiguous result as `blocked`.

## Completion

For Execute, require all mandatory steps to be `completed` or procedure-backed `not-applicable`, a populated completion-evidence section with existence, correspondence, and access results for every created artifact and required reference, and no pending approval or blocker.

For Audit, completion evidence consists of the questions, findings, coverage, and provenance defined in `read-only-audit.md`. A procedure-defined terminal unknown completes the collection step with that limitation; an obtainable but pending required observation leaves the run blocked. Audit completion and audited business outcome are separate.

For persistent Draft, require validated and delivered local artifacts, recorded validation evidence and limitations, and no pending required drafting step. Do not require publication or external read-back to close a Draft run.
