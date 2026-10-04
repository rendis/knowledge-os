---
name: inspect-database
description: "Trigger: inspect database schemas or live evidence for a selected target, for any database engine, from questions or operational audits. Repositories are optional evidence sources."
---

# Inspect database evidence

## 1. Bind the question

Accept a handoff from the current primary workflow with the resolved `VAULT_ROOT`, primary branch, exact question and authorized scope. Return findings to that branch. If vault identity is missing, load `../../../90-Meta/vault-resolution.md` and resolve it before continuing.

Classify the request as static evidence, live metadata or live data. Bind the exact target and period from still-applicable user decisions. Static questions do not require a live connection.

Complete when one question, branch and evidence scope are explicit.

## 2. Resolve the destination and optional sources

Bind `<CLI>` through [use-vault-cli](../use-vault-cli/SKILL.md). Inspect declared targets with:

```text
<CLI> config --vault "<VAULT_ROOT>" database-targets
<CLI> config --vault "<VAULT_ROOT>" database-target --target "<TARGET_ID>"
```

Select by explicit domain/environment/database or target ID, never by local port. If several targets fit, ask only for the missing discriminator. Read the returned procedure: it owns engine and version identification, executor selection, authentication, read-only checks, query dialect and engine-specific catalog commands. Use the destination’s runbook for that engine; this skill ships no drivers or connection recipes. A configured target is not proof of connectivity or authorization.

Each target has zero or more `repositories`. Resolve relevant remotes through `<CLI> config locate --vault "<VAULT_ROOT>" --remote "<REMOTE>"`, then read their applicable instructions. Repositories, documentation and live catalogs are distinct evidence sources. Missing repositories limit static claims; they do not prevent authorized live inspection with a valid executor. Do not substitute the schema repository for a target whose repository list is empty.

For cells without a matching target, the `database-inspection` capability is the access contract. For a static-schema question, `<CLI> config schema-repository --vault "<VAULT_ROOT>"` resolves the optional `sources.schema_repository`. Its absence means static source unavailable, not database nonexistent. An unknown explicitly requested target cannot silently fall back to another destination. Hand configuration gaps to `onboard-developer`.

Complete when the requested target or capability route is unambiguous, and available evidence sources and their limitations are identified.

## 3. Inspect the relevant evidence

Use static evidence when it answers the question. For live evidence, resolve connection requirements from the runbook. Retrieve a local port through `<CLI> config proxy-port --vault "<VAULT_ROOT>" --environment "<KEY>"` only when that executor requires one, using the target's `port_key` or the capability procedure's key. Direct connections, sockets and API executors do not require a proxy port. The port is a local preference, not destination identity.

Follow the resolved procedure's executor contract. Confirm environment, instance, database, schemas, read-only identity, credential mechanism and query scope. Cover every material part of the question with bounded, read-only queries: relevant schemas, conditions, periods and result pages. Prefer projected columns, indexed predicates and aggregates to broad row dumps. A sample supports only a sampled observation; verify full coverage before claiming absence or totals. Reuse still-applicable read authorization from user decisions and the procedure; configuration alone grants none. Preserve timestamps and distinguish current snapshots from historical evidence. An absent executor, invalid destination, unavailable read-only enforcement or missing authorization blocks only dependent live work. Never replace these with an improvised connection or a different target.

Complete when each live operation has observed target/read-only checks and a bounded query, or an exact access gap is recorded.

## 4. Return evidence

Report static source revisions, live observation time and destination, results, inferences, and limitations separately. Preserve secrets; use the primary workflow's evidence store and retention policy. Configuration changes belong to `onboard-developer`; business mutations require their own authorized workflow.

Complete when the initiating question is answered or its remaining limitation is explicit and control returns to the primary branch.
