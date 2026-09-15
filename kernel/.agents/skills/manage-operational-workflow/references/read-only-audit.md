# Read-only operational audit

Use this branch to reconstruct one flow execution or trace an entity within a selected period. The operational procedure owns domain semantics and approved queries; installed inspection adapters own access and execution. Reuse those contracts rather than rediscovering the entire source implementation for each run.

## 1. Bind the audit

Record the procedure and revision, environment, exact targets, entity filters, inclusive/exclusive time boundaries, named timezone, requested questions, and evidence needed to answer each question. Resolve values from explicit user context and the configured catalogs. Ask only for missing consequential inputs. Keep domain-specific services, schemas, entity keys, and status mappings in the procedure or its versioned scripts.

Declare the effect boundary: remote reads and local evidence artifacts only. Classify operations by actual behavior, not HTTP method; a GET that starts a job or sends business data is an external effect and cannot run in Audit. Treat a requested correction as a separate Execute operation.

**Complete when:** scope and targets are unambiguous, every question has a planned evidence source, and every planned remote call has verified read-only semantics.

## 2. Validate access through the owning adapters

Resolve configured capabilities and their installed adapters before connecting. Supply each adapter's required context and follow its target-resolution, credential, query-review, resource-limit, and read-only identity gates. A configuration mismatch is a handoff to its configuration owner; preserve the registry and executor instead of constructing an alternate target or bypassing a failed gate.

When an adapter requires a `map-ecosystem` handoff with primary branch `interrogation`, open a bounded `map-ecosystem/interrogation` subquery with the audit's known targets, questions, period, and filters. That subquery owns the adapter handoff and returns evidence or an explicit blocker. Keep Audit as the parent operation and record the returned evidence in the same `.operations/` run; do not restart discovery or replace the audit's branch.

For cloud and Kubernetes, distinguish identity/session validity, context selection, network reachability, and authorization. Use existing authentication. When evidence identifies expired or missing credentials, provide the owning adapter's exact recovery step and resume after recovery; do not prescribe login for a network or RBAC failure. Authentication, credential refresh, local proxy lifecycle, and kubeconfig setup follow that adapter's authority rules. Audit itself grants no infrastructure changes or permission elevation.

For databases, bind the instance, database, schema, and identity separately from the local port. Apply the executor's plan review and bounded-read controls before result queries. Keep credentials outside evidence files.

Continue independent accessible sources when one source is blocked. Record the access classification, affected question, and safe continuation.

**Complete when:** each required source is ready through its owning adapter or has an explicit access blocker; no blocked source is queried through a substitute mechanism.

## 3. Collect and correlate

Execute the smallest reads that answer the questions, using procedure-owned parameterized scripts where available. Record each step immediately with source, observation time, event time and timezone, filters, identifiers, query/script revision, limits, and a minimal sanitized result. Detect truncation, pagination gaps, retention limits, missing logs, and timeouts; a partial result cannot establish absence.

Distinguish configuration, current database state, historical events, and provider acknowledgements. Preserve original timestamps and normalize a comparison timestamp only when the source timezone is established. A timestamp without timezone remains ambiguous until its convention is verified.

Prefer stable run, transaction, entity, and batch identifiers for joins. Label time-only matches as inference. A scheduled time does not prove execution; a successful Job or HTTP response does not prove business completion; accepted submission does not prove entity-level application. Establish correction through a subsequent observation of the same entity and relevant scope. Current state alone cannot reconstruct an earlier state.

Give each finding one evidence label: `confirmed`, `inferred`, or `unknown`. Track the audited outcome separately: success, error, retry, accepted submission, verified correction, or unresolved, using the procedure's definitions. Negative findings such as “no retry” require complete coverage of the relevant window.

**Complete when:** every collected result is traceable, every correlation states its basis, and every answer has an evidence label and coverage assessment.

## 4. Report and resume

Produce a concise summary with scope, execution timeline, findings, evidence references, and remaining questions. Add a local HTML visualization when it improves inspection and a structured result when the procedure supplies a schema. A visual renders the same findings and labels; it is not an independent source of evidence. Keep compact sanitized summaries and provenance in `.operations/`; store permitted extracts under the executor's retention contract. Never embed secrets or copy unrestricted production payloads into the ledger or report.

Complete the audit when every required question is answered or has reached a procedure-defined terminal unknown, such as expired evidence retention. If a required observation is still obtainable but blocked by access or a future natural execution, keep the run `blocked` with the exact pending step. An observed business failure can be the conclusion of a completed audit.

On resume, preserve historical findings and append new observations. Refresh only evidence whose freshness matters to the pending question. Do not overwrite an earlier snapshot with current data or repeat completed collection without a reason. Future monitoring or scheduling requires a user request; a pending observation alone does not create an automation.

For an explicitly agreed observation campaign, apply [observation.md](observation.md): a completed cycle does not complete the campaign, and waiting for its next planned interval keeps `in-progress`. A currently required inaccessible observation still has an explicit blocked/unknown disposition; scheduled waiting is not a business failure.

**Complete when:** the report matches the ledger, required questions have explicit dispositions, and any continuation names the missing observation without triggering the audited process.
