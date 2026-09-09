---
name: inspect-gcp-runtime
description: "Trigger: inspect GCP/GKE status, logs, or access after the flow and deployed targets are known. Adapter."
---

# Inspect GCP runtime

Treat the vault and versioned deployment evidence as the flight plan. Reach the control plane only after the affected business flow, environment, and exact runtime targets are known.

## 1. Build the target card

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Continue only with one canonical `VAULT_ROOT`, then use the `map-ecosystem` **interrogation** branch as the primary evidence workflow.
2. Read the named or inferred flow, its participant notes, backlinks, and each participant's **Infraestructura y scheduling** section.
3. Resolve `runtime-inspection` with `90-Meta/cell-config.py --vault-root "<VAULT_ROOT>" resolve --capability runtime-inspection` and read the returned procedures/catalogs. A missing binding triggers `configure-workspace` for that capability only; static evidence collection can continue. Live access requires the configured target and read policy.
4. If a deployable target is incomplete, inspect its versioned Actions, Cloud Build, manifests, overlays, or reusable deployment workflows through `map-ecosystem` before considering a live query.
5. Produce one target-card row per component with: flow, component, environment, project, platform, resource, region or zone, namespace or workload when applicable, and evidence source.

Keep unknown values explicit. An environment name or project naming pattern is not enough to invent a resource.

**Complete when:** every component relevant to the question has an exact target-card row or a named missing field that prevents live access.

## 2. Classify the requested effect

- **Runtime read:** continue with metadata, status, logs, or supported metrics.
- **Application or database data:** read `VAULT_ROOT/instance.yaml` and require `postgres` in `adapters`, then require `VAULT_ROOT/.agents/skills/inspect-database/SKILL.md` as a regular installed file. Only after both checks pass may `inspect-database` own the PostgreSQL evidence request. If either check fails, stop with `database-evidence-adapter-unavailable`, name the failed condition, and do not substitute a GCP query, ad-hoc database access, or another skill.
- **Mutation or local access setup:** load `manage-operational-workflow`, select the matching runbook or draft an effect plan, and stop this skill before any changing command. This includes deploys, restarts, scaling, configuration, IAM, API enablement, GKE endpoint changes, `get-credentials`, and credential installation.

Do not turn a mutation request into a permission probe by attempting the change.

**Complete when:** each requested outcome has exactly one available read, database, or operational-handoff branch and no changing action remains in the inspection branch.

## 3. Prove current access safely

1. Load `gcloud` before planning or running any `gcloud` command. Use its required leaf-level help lookup.
2. Consume only the already-active identity. A narrowly formatted `gcloud auth list` may establish that an active identity exists; keep the account value out of durable notes and user-facing output unless it is necessary.
3. Run every candidate `gcloud` or `kubectl` command through:

   ```text
   <python> -B <inspect-gcp-runtime-dir>/scripts/validate-runtime-command.py --command "<candidate>"
   ```

4. Execute an allowed read once. Treat its real result as the permission check; role names alone do not prove effective access.
5. Classify failure as `authentication-required`, `iam-denied`, `api-unavailable`, `network-unreachable`, `kubernetes-rbac-denied`, `target-not-found`, or `query-unsupported`.

Preserve the current identity and configuration. Return the classified blocker when authentication, project configuration, impersonation, component installation, API enablement, IAM, or network changes would be needed.

**Complete when:** every target is marked `readable` or has one evidence-backed access classification, without changing credentials, configuration, APIs, IAM, kubeconfig, or infrastructure.

## 4. Select only the needed evidence branch

- For GKE cluster, workload, Service, event, or pod evidence, read [references/gke.md](references/gke.md).
- For Cloud Run, Functions, Pub/Sub, Scheduler, Eventarc, Cloud SQL, Firestore, or another managed service, read [references/service-checks.md](references/service-checks.md).
- For logs or metrics, read [references/observability.md](references/observability.md). Load `cloud-logging-query-generation` only after the target card is exact. Load `cloud-monitoring-metric-selection` only under the capability and policy gate defined there.

Use the smallest branch set that can answer the original question.

**Complete when:** every requested signal maps to one loaded reference or is explicitly `query-unsupported`.

## 5. Gather bounded observations

1. Validate leaf syntax, then run one command at a time.
2. Scope every query to the exact project and location when applicable. Use the explicit Kubernetes context, never the active context.
3. Project only required fields. Bound lists, log time ranges, log rows, and Kubernetes logs.
4. Keep secret resources, credentials, tokens, raw production data, and unredacted sensitive log values outside commands, notes, and responses.
5. Stop a target after an access blocker; continue independent readable targets when they still answer part of the question.

Runtime observations are ephemeral investigation evidence. They do not update the durable ecosystem graph automatically.

**Complete when:** each readable target has the minimum status, log, or metric evidence needed for the question, and every skipped signal has a reason.

## 6. Answer at flow level

Report:

- flow, environment, observation time, and resolved components;
- project and runtime target for each component;
- status and relevant error evidence, correlated across participants when timestamps permit;
- access coverage and exact blockers;
- unsupported signals and what evidence would be needed next;
- stable deployment facts discovered from versioned sources that should be promoted through an authorized `map-ecosystem` documentation branch.

Separate stable deployment mapping from volatile runtime state. Summarize logs instead of persisting raw payloads.

**Complete when:** the answer resolves the user's flow-level question or states the smallest remaining blocker, with no target, permission result, or limitation omitted.
