# Inspect GKE

Load this reference only after the target card contains an environment, project, cluster, zone or region, and exact kubeconfig context. Use the catalog in `60-Operacion/GCP/GCP - Catalogo de ambientes y runtimes.md` for known cell context names and the guide `60-Operacion/GCP/GKE - Acceso con kubectl y Lens.md` for local setup or network recovery.

## Evidence ladder

1. Use a bounded `gcloud container clusters describe` to establish control-plane status, version, location, and endpoint mode.
2. Test Kubernetes API reachability with the selected `--context` and a bounded `/version` read.
3. Inspect only the namespaces and workloads resolved from deployment evidence.
4. Check Deployment or StatefulSet availability, Pod phase and readiness, Service selectors, EndpointSlices, and recent Events only as required by the incident.
5. Use bounded pod or workload logs for the error window. Use `kubectl top` only when the Metrics API is available and current CPU or memory helps answer the question.

`RUNNING` at the GKE control plane does not prove workload health. A Service count does not prove that selectors have ready endpoints.

## Command boundary

Use only commands accepted by `scripts/validate-runtime-command.py`. Every cluster API command must name `--context` and `--request-timeout`; namespaced reads must name `--namespace` or an intentional all-namespace scope. Project collection output with `custom-columns`, `jsonpath`, or `name`.

The inspection branch permits bounded reads such as:

- Kubernetes version;
- Deployments, StatefulSets, Pods, Services, EndpointSlices, Jobs, CronJobs, Events, or nodes;
- one projected read of a named resource;
- bounded workload logs;
- `kubectl top`;
- `kubectl auth can-i`;
- non-watching rollout status.

Treat kubeconfig writes, credential refresh, plugin installation, `exec`, `cp`, `port-forward`, proxying, secret reads, and every workload or cluster mutation as an operational handoff.

## Access classification

| Observation | Classification | Meaning |
|---|---|---|
| GKE describe denied | `iam-denied` | The active gcloud identity cannot read cluster metadata. |
| GKE describes but API endpoint times out | `network-unreachable` | Control-plane metadata is readable, but the selected endpoint is unreachable from the current network. |
| API responds, workload read is forbidden | `kubernetes-rbac-denied` | Authentication reached Kubernetes; RBAC blocks the requested resource or namespace. |
| Context is absent | `local-access-setup-required` | The guide must prepare a reviewed local-state change; inspection does not write kubeconfig. |
| Metrics API is absent or forbidden | `query-unsupported` | Continue with workload status and logs; do not present missing metrics as zero. |

Lens is a consumer of kubeconfig, not an independent source of truth. Reproduce its connectivity or health result with the explicit CLI context before using it as evidence.

## Completion criterion

The GKE branch is complete when cluster metadata, API reachability, relevant workload health, Service-to-endpoint state, requested logs or usage, and every IAM/network/RBAC limitation have an explicit result.
