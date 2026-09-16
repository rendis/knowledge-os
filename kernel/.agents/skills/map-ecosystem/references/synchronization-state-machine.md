# Synchronization state machine

Use this contract after a synchronization interruption, validation failure, or source/destination change. The coordinator reads `status` or `resume`; it does not infer state from logs or rerun semantic work speculatively.

Active-run responses retain `next_command` and add `next_action`: the same `command`, an `argv` array beginning with that subcommand and its known CLI arguments, the selected `repository`/`oid` or `unit_id` when applicable, and `missing_inputs` listing required CLI flags whose values are not known. Supply those inputs before invoking `90-Meta/sync-run.py` with the arguments; an empty list only means CLI inputs are complete, not that validation or authorization has passed. Hints do not execute anything or reuse potentially stale projection, gate, or review artifacts. Closed receipts retain their existing format.

## Durable records

`STATE_ROOT` defaults to `.agents/state/map-ecosystem/sync` relative to the vault.

| Record | Location | Contract |
| --- | --- | --- |
| Active run | `active/<run_id>/run.json` | Run schema v1. `run_id` is the supervision handle; its current fingerprint binds tool, inventory, and ordered repository/OID pairs and is refreshed only for explicit source drift. |
| Closed package | `active/<run_id>/packages/<repository>/artifact.json` | Exact `package-closed` semantic product: finalized manifest, scaffold, analysis, independent review for write authority or deterministic cursor fallback, single-package gate, and recomputable lineage digests. |
| Sealed authority | `active/<run_id>/gate.json` | Gate schema v2; its digest and accepted grants bind every projection. |
| Unit projection and patch | Paths emitted by `status` | Projection schema v1: `version`, `run_id`, `gate_digest`, `unit_id`, `unit_type`, `patch_digest`, `base_files`, `result_files`, and `grants`. |
| Final-note review receipt | `active/<run_id>/units/<unit_id>/note-review.json` | Binds accepted review and manifest digests to the validated projection; required before applying a documentation unit. |
| Safe completion receipt | `receipts/<run_id>.json` plus `receipts/fingerprints/<fingerprint>.json` | Closed statuses, identities, OIDs, and digests only. The fingerprint index reuses the completed current identity even when source drift changed it during the supervised run. |

The active checkpoint contains only finalized, validated, redacted package artifacts, validated contracts, digests, and closed receipts. It must be safely persisted atomically before a transition is reported.

## States and commands

The run progresses monotonically through `packages`, `gated`, `projecting`, and `closing`; the closed receipt reports `complete`. A unit moves from `pending` to `validated` to `applied`. A documentation unit additionally requires a bound final-note review receipt before application; structural validation alone is insufficient. Its recoverable states are `projection-invalid`, `apply-failed`, and `stale`. Durable `stale_reason` distinguishes `source`, `source-retracted`, and `destination`; recovery never guesses the cause from bytes alone.

| Command | Required public arguments | Result |
| --- | --- | --- |
| `tool-digest` | None | Computes the installed Python tool bundle identity. |
| `begin` | `--state-root --inventory-digest --package REPOSITORY OID` | Computes tool identity and creates or reuses the integrity-checked run for the fingerprint. |
| `checkpoint-package` | `--state-root --run-id --repository --artifact` | Persists one exact `package-closed` artifact; rejects digest-only or unverifiable receipts. |
| `seal-gate` | `--state-root --run-id --gate` | Persists gate v2 and independent units. |
| `status` | `--state-root --run-id` | Returns `run_id`, status, gate digest, ordered units, package counters, receipt digest, and `next_command`. |
| `validate-unit` | `--state-root --run-id --unit-id --projection --patch` | Records the selected unit as validated or projection-invalid. |
| `review-unit` | `--state-root --run-id --unit-id --vault --candidate --evidence-root --manifest --review` | Checks independent final-note acceptance against the stored projection and records its bound receipt. |
| `apply-unit` | `--state-root --run-id --unit-id --vault` | Applies one validated unit idempotently, requiring a final-note review for documentation units; a mixed preimage/postimage is never completed forward. |
| `resume` | `--state-root --run-id` | Returns the exact next command plus invalidated, reused, reconciled, and retracted units/packages. |
| `close` | `--state-root --run-id` | Requires all units applied, writes the durable receipt, and removes only `active/<run_id>/`. A retry with an existing valid receipt idempotently retires a matching orphan active checkpoint. |

Every command emits stable JSON. Exit `0` is success, `2` is a contract/blocking condition, and `1` is an unexpected operational failure.

## Failure routing

| Failure | State | Reusable artifacts | Next command |
| --- | --- | --- | --- |
| Invalid or invented grant, path, digest, or projection | `projection-invalid` | Packages and gate | Correct the selected projection metadata or patch, then `validate-unit` with the same `run_id`. Only an unchanged projection+patch attempt is rejected; corrected metadata may reuse unchanged valid patch bytes. |
| Write interruption or failed application | `apply-failed` | Packages, gate, validated projection | The apply journal rolls back files written by a caught failure. `resume` reconciles only a complete `after` image; a mixed preimage/postimage from a hard crash is blocked and never completed forward. |
| Source OID changed | `stale` | Unaffected packages and groups | `resume --source-oid REPOSITORY OID`; it commits stale state before removing the old package, retracts an already-applied affected unit to its exact preimage, refreshes the run fingerprint, and directs rebuilding only connected package/group work before resealing. Retraction is required before checkpoint/reseal or convergence to an existing receipt. If the refreshed fingerprint already has a valid closed receipt, that receipt wins only after the superseded bytes are retracted. |
| Vault baseline changed for a pending unit | `stale` | Packages and gate | `resume --destination-digest PATH SHA256`; reproject only that unit. |
| Deterministic semantic fallback | `packages` or `gated` | Finalized package and its closed fallback receipt | Continue to `checkpoint-package` or `seal-gate`; no new extractor or reviewer. |
| Unresolved checkout/identity, unreadable exact evidence, invalid binding/integrity, or unsafe persistence | blocked | Only records proven valid by `status` | Stop and report the exact code; do not apply. |
| Tool or schema mismatch | terminal (`run-version-mismatch`) | Existing immutable artifacts | Restore the original compatible tool bundle before continuing an active run. Mutations compare actual installed bytes, including when `--tool-digest` is omitted. Status and completed-receipt replay remain available; existing identities are never rewritten. |
| Missing active run | terminal (`run-state-missing`) | Closed receipt, if present | Inspect the receipt; otherwise report the missing state. |

`source-stale` reopens only connected packages/groups. Applied affected bytes are
retracted by `resume` before fresh authority can be checkpointed/resealed;
changed bytes that cannot be safely retracted remain an explicit conflict and
`status` keeps `resume` as the next command. Destination-stale bytes are never
treated as source publication and remain untouched for reprojection. Unaffected validated/applied groups are
rebound by their repositories, nodes, and grants even when deterministic group
ordinals change. `vault-baseline-stale` preserves semantic authority and
reprojects only the destination unit. Neither permits another semantic
pass on a checkpointed package. The optional single correction occurs only before checkpointing, under the coordinator recipe.

A finding-directed correction is local package preparation, not a new run state. `sync-correction.py` preserves its first artifacts and enforces one reserved correction before the final package checkpoint. The durable run continues to own one selected finalized package per repository.

## Invariants

- `acknowledgements` and every `group-NNN` are atomic and idempotent. A write group may also retire a prior acknowledgement for its repository at the same analyzed commit; that process-file change is applied atomically with the reviewed node update. All other write paths and nodes remain disjoint.
- Final-note acceptance is an additional check, never a grant; missing review blocks documentation application and cannot be bypassed by resume.
- The gate remains the only publication authority: projections must declare grants present in gate v2.
- Markdown projections stay below canonical knowledge roots (`10-Sistemas/` through `70-Aprendizajes/`); agent, state, and metadata surfaces are never basename-authorized.
- The coordinator uses the same emitted `run_id` for recoverable work. It preserves all still-valid package and gate digests.
- A run becomes complete only through `close`; the closed receipt is preferred immediately. `close` retry and `begin` retire any matching active orphan before another fingerprint is admitted, while best-effort deletion may leave only a detached retired directory.
