# Inspect logs and metrics

Load this reference only after the target card identifies every component and environment involved.

## Logs

1. Define the incident window, timezone, severity or error concept, and maximum rows before generating a query.
2. Load `cloud-logging-query-generation` and its service-specific reference. Treat its output as a candidate LQL filter, not as an executable command.
3. Reject a candidate that contains an unresolved `<PLACEHOLDER>`, guesses an audit `protoPayload.methodName`, omits `resource.type`, or lacks a resource label or log identifier that binds it to the target card.
4. Validate `gcloud help logging read`, compose a bounded `gcloud logging read`, and pass it through `validate-runtime-command.py`.
5. Summarize the minimum fields needed to correlate the error. Redact credentials, tokens, direct personal data, and sensitive payload values; do not persist raw log rows in the vault.

Use separate queries when flow participants have different monitored resource types. Correlate their bounded windows after each query succeeds.

## Metrics

Strict `gcloud` does not expose arbitrary Cloud Monitoring time-series reads. Apply these branches:

- Use `gcloud` read commands for Monitoring configuration such as dashboards, alert policies, or uptime checks only when that configuration answers the question.
- For GKE point-in-time CPU or memory, use bounded `kubectl top` when the Metrics API and RBAC permit it. Do not describe this as a Cloud Monitoring time series.
- `cloud-monitoring-metric-selection` selects metric descriptors; it does not retrieve metric values. Use it only when `list_metric_descriptors` is already available and the user explicitly accepts that non-`gcloud` read for descriptor discovery. Do not install or configure an MCP server from this skill.
- When the request requires actual historical values and no approved capability exists, return `query-unsupported` with the exact metric concept, resource, project, and interval needed.
- For the cell PostgreSQL internals, hand off to `inspect-database`; its production safeguards remain authoritative.

Never turn unavailable metrics into an inferred healthy state.

## Completion criterion

The observability branch is complete when every requested log window or metric signal is either bounded and observed or marked unsupported with the exact missing capability, and all returned evidence is minimized and sanitized.
