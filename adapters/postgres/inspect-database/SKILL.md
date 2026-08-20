---
name: inspect-database
description: "Trigger: inspect schema or live database evidence after map-ecosystem hands off a database-dependent question. Adapter."
---

# Inspect the cell database evidence

Use a static-first evidence ladder. When `map-ecosystem` hands off the request, keep its `interrogation` branch primary; this skill supplies database evidence and returns control to that recipe.

Use the Python 3 command selected during `map-ecosystem` bootstrap as `<python>` below.

## 1. Bind the primary branch

Require a `map-ecosystem` handoff containing the resolved `VAULT_ROOT`, the selected primary branch, and the exact database-dependent question. When any item is absent, return a map-context requirement to the caller; branch selection and vault bootstrap remain owned by `map-ecosystem`.

Completion criterion: one remote-verified `VAULT_ROOT` is bound, `interrogation` is the primary branch, and the exact database-dependent question is stated.

## 2. Resolve the schema repository

From `VAULT_ROOT`, run exactly:

```text
<python> -B 90-Meta/workspace-config.py --vault-root "<VAULT_ROOT>" repository schema-repository
```

Use only the path returned by this view. When the command reports missing, uninitialized, ambiguous, stale, or inconsistent configuration, load `configure-workspace`, complete its applicable status, initialize, repair, or refresh branch, and rerun the same command. This skill never edits `.knowledge-os-config.yaml`, scans alternate roots, or substitutes an environment variable or manually supplied repository path.

A failed semantic view is the trigger for configuration handoff. Do not preselect or propose proxy ports merely because database evidence might be useful later; let the configuration skill open its confirmed onboarding only after this command exposes the gap or the user explicitly asks about workspace readiness.

Read the resolved repository's `AGENTS.md` and any closer instructions before reading paths or executing its tooling.

Completion criterion: the config API returns exactly one `schema-repository` path, that repository's applicable instructions are loaded, and no fallback location was used.

## 3. Climb the evidence ladder

Inspect versioned evidence first, following the repository's evidence and navigation order. Classify the least-powerful evidence branch that can answer the question:

- **Static schema or model**: use versioned schema, migration, context, and recorded synchronization evidence. Do not request an environment.
- **Live metadata**: use current catalog or schema metadata only when static evidence cannot establish the required current state.
- **Live data**: query current values only when they are necessary to answer the question and the active authority permits that scope.

Stop at static evidence when it answers the question. Before escalating, state which unresolved fact requires live evidence and whether metadata or data is the minimum sufficient branch.

Completion criterion: every relevant static source named by the repository contract has been inspected or reported unavailable, and any live escalation is justified by one concrete unresolved fact.

## 4. Gate live evidence

Require the user to select `dev`, `uat`, or `prod` explicitly when live evidence is necessary. Accept an environment already explicit in the current request; otherwise ask before opening a connection. Infer no environment from a port, credential, previous turn, or repository state.

Require the database name as a separate explicit input. Resolve the confirmed local port through the semantic API:

```text
<python> -B 90-Meta/workspace-config.py --vault-root "<VAULT_ROOT>" database-proxy-port "<ENVIRONMENT>" --format value
```

If this view reports `proxy_port_not_configured`, hand the selected environment to `configure-workspace`, let it configure that one preference, and retry the same view. For any other configuration error, hand off its exact diagnostic. Do not parse `.knowledge-os-config.yaml`, invent a default, or pick another port.

Before any remote SQL, load the resolved schema repository's query-analyzer skill and follow it completely. Hand it only the explicit environment, database, and resolved port; every remote runner invocation must receive that exact value as `--proxy-port`. Do not reconstruct host, user, access, instance, proxy commands, registry paths, credential checks, or other runner behavior in this vault. The repository skill and its versioned target registry own SQL classification, target validation, proxy lifecycle, credential preflights, read-only enforcement, review evidence, timeouts, confirmations, and execution. Credentials must resolve through the user's standard `.pgpass`; do not accept password values or alternate credential files. If the analyzer is absent or a gate cannot pass, stop instead of issuing ad-hoc SQL. Keep metadata queries scoped to the required catalog fields and data queries scoped to the minimum rows and columns required by the question.

Completion criterion: environment and database are explicit, the semantic port view succeeds, the repository analyzer is loaded before every remote SQL operation, every remote runner receives the resolved `--proxy-port`, all repository-owned gates pass, and no live operation runs after a failed or unavailable gate.

## 5. Return the evidence

Return the result to the primary `interrogation` recipe and distinguish:

- versioned schema/model evidence, with repository revision and paths;
- live metadata evidence, with environment, observation time, and inspected catalogs;
- live data evidence, with environment, observation time, and the minimum disclosed scope;
- inference, contradictions, unavailable evidence, and remaining limitations.

Preserve secrets and sensitive values outside the report. Keep durable vault, shared configuration, and source-repository content unchanged; handle analyzer-owned temporary evidence under its contract.

Completion criterion: the conclusion identifies its evidence level, every factual claim is traceable to an inspected source, current-state claims use observed live evidence, limitations are explicit, and control returns to `map-ecosystem` without a durable content change.
