---
name: inspect-database
description: "Trigger: inspect database schemas or live evidence for a selected target, from knowledge questions or operational audits. Repositories are optional evidence sources."
---

# Inspect database evidence

## 1. Bind the question

Accept a handoff from `map-ecosystem` or `manage-operational-workflow` with the resolved `VAULT_ROOT`, primary branch, exact question and authorized scope. Return findings to that branch. If vault identity is missing, load `../../../90-Meta/vault-resolution.md` and resolve it before continuing.

Classify the request as static evidence, live metadata or live data. Bind the exact target and period from still-applicable user decisions. Static questions do not require a live connection.

Complete when one question, branch and evidence scope are explicit.

## 2. Resolve the destination and optional sources

Inspect declared targets with:

```text
python3 -B 90-Meta/cell-config.py --vault-root "<VAULT_ROOT>" database-targets
python3 -B 90-Meta/cell-config.py --vault-root "<VAULT_ROOT>" database-target --target "<TARGET_ID>"
```

Select by explicit domain/environment/database or target ID, never by local port. If several targets fit, ask only for the missing discriminator. Read the returned procedure: it owns executor selection and access requirements. A configured target is not proof of connectivity or authorization.

Each target has zero or more `repositories`. Resolve relevant remotes through `workspace-config.py locate-repository`, then read their applicable instructions. Repositories, documentation and live catalogs are distinct evidence sources. Missing repositories limit static claims; they do not prevent authorized live inspection with a valid executor. Do not substitute the legacy repository for a target whose repository list is empty.

For cells without a matching target, the existing `database-inspection` capability remains the legacy access contract. For a legacy static-schema question, `workspace-config.py repository` resolves the optional `sources.schema_repository`. Its absence means static source unavailable, not database nonexistent. An unknown explicitly requested target cannot silently fall back to another destination. Hand configuration gaps to `configure-workspace`.

Complete when the requested target or legacy route is unambiguous, and available evidence sources and their limitations are identified.

## 3. Inspect the minimum evidence

Use static evidence when it answers the question. For live evidence, retrieve the local port using the target's `port_key` through `workspace-config.py database-proxy-port`; legacy routes use the key specified by their procedure. The port is a local preference, not destination identity.

Follow the resolved procedure's executor contract. Confirm environment, instance, database, schemas, read-only identity, credential mechanism and query scope. Use minimum metadata or rows; preserve timestamps and distinguish current snapshots from historical evidence. An absent executor, invalid destination, unavailable read-only enforcement or missing authorization blocks only dependent live work. Never replace these with an improvised connection or a different target.

Complete when each live operation has observed target/read-only checks and a bounded query, or an exact access gap is recorded.

## 4. Return evidence

Report static source revisions, live observation time and destination, results, inferences, and limitations separately. Preserve secrets; use the primary workflow's evidence store and retention policy. Configuration changes belong to `configure-workspace`; business mutations require their own authorized workflow.

Complete when the initiating question is answered or its remaining limitation is explicit and control returns to the primary branch.
