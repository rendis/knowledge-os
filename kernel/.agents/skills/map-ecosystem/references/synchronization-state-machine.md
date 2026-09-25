# Synchronization recovery

Use the current direct publication route for new work. Use the legacy state machine only when durable state from an older run already exists. Never restart semantic work to recover a deterministic or publication failure.

## Current direct publication

`sync review freeze` binds complete candidate images, evidence, current note baselines and portable frozen source identities. `sync review check` binds one accepted semantic review to those bytes, revalidates each repeatable `--checkout <repository> <checkout>` against the frozen identity and revisions, and reports its deterministic result. `sync review publish` publishes only accepted candidate bytes and stores resumable state below the supplied state root.

Use the installed CLI help and retain the exact arguments from the checked publication attempt. Retry the same publication command with the same candidate, manifest and review. A complete matching destination may be reconciled into its receipt; mixed or conflicting bytes block forward completion. Source drift invalidates only the affected repository analysis. Destination drift preserves current user bytes and rebuilds/reviews only the affected candidate against that baseline.

## Legacy active runs

An older active run may contain immutable packages, a sealed gate, unit projections and final-note review receipts under `.agents/state/map-ecosystem/sync/active/<run_id>/`. Preserve those records and use `status --state-root <root> --run-id <run_id>` or `resume --state-root <root> --run-id <run_id>` to obtain its next command. Do not translate it into the direct route or recreate analysis, review, package, gate or projection artifacts.

Legacy runs progress through `packages`, `gated`, `projecting` and `closing`; units progress from `pending` to `validated` to `applied`. Their stored executable digest, inventory fingerprint, repository/OID pairs and artifact digests remain authoritative for that run. Restore the matching CLI when it reports `run-version-mismatch`.

| Legacy failure | Reuse | Recovery |
| --- | --- | --- |
| Invalid grant, path, digest or projection | Packages and gate | Correct only projection metadata or patch and repeat the stored validation command. |
| Write interruption | Packages, gate and validated unit | Use `resume`; preserve mixed/conflicting destination bytes for explicit resolution. |
| Source OID changed | Unaffected packages and groups | Use the stored source-drift resume action; retract affected applied bytes when required, then rebuild only connected work. |
| Destination baseline changed | Packages and gate | Reproject only the affected unit against the observed destination. |
| Tool/schema mismatch | Immutable artifacts | Restore the original CLI; never rewrite stored identities. |
| Missing active run | Closed receipt when present | Inspect the receipt; otherwise report missing state. |

The old [synchronization-package-worker.md](synchronization-package-worker.md) contract is loaded only when a legacy run's stored next action explicitly names `SYNC_PACKAGE_WORKER_V1`. New sessions never choose it from inventory alone.
