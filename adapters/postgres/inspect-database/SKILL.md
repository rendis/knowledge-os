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
<python> -B 90-Meta/workspace-config.py --vault-root "<VAULT_ROOT>" repository
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

Resolve the team's executor contract:

```text
<python> -B "<VAULT_ROOT>/90-Meta/cell-config.py" --vault-root "<VAULT_ROOT>" resolve --capability database-inspection
```

Read the returned procedure(s). If unconfigured, hand off that exact capability to `configure-workspace`; continue any independent static analysis. The procedure defines environment names, databases, target registry, executor repository/skill, authentication mechanism and required parameters. Follow its referenced source-repository instructions before accessing a live target. The kernel prescribes no environment names, proxy flags, credential file or analyzer name.

Bind the exact environment, database and query scope from the current request or still-applicable explicit decisions. Ask only for missing consequential inputs. Use only the configured executor's read-only path and minimum metadata or rows needed. If a required port is configured locally, retrieve it through `workspace-config.py database-proxy-port`; never infer a target from a port. An absent executor, target, read-only enforcement or required authorization blocks live access. Return the gap instead of constructing an alternative connection. Preserve credentials and infrastructure; setup or mutation follows the operational workflow.

Completion criterion: the configured execution contract and existing authority cover the exact live read, the executor establishes its read-only safeguards, and evidence identifies the target and observation time.

## 5. Return the evidence

Return the result to the primary `interrogation` recipe and distinguish:

- versioned schema/model evidence, with repository revision and paths;
- live metadata evidence, with environment, observation time, and inspected catalogs;
- live data evidence, with environment, observation time, and the minimum disclosed scope;
- inference, contradictions, unavailable evidence, and remaining limitations.

Preserve secrets and sensitive values outside the report. Keep durable vault, shared configuration, and source-repository content unchanged; handle executor-owned temporary evidence under its contract.

Completion criterion: the conclusion identifies its evidence level, every factual claim is traceable to an inspected source, current-state claims use observed live evidence, limitations are explicit, and control returns to `map-ecosystem` without a durable content change.
