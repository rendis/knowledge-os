# Inspect managed GCP services

Load this reference after the target card identifies the exact project, service, resource, and location when applicable. Validate the current leaf syntax through `gcloud` before constructing a command.

## Minimum checks

| Platform | Control-plane evidence | State to interpret | Boundary |
|---|---|---|---|
| Cloud Run | service and revision descriptions | Ready conditions, latest ready revision, traffic allocation, update time | Do not deploy, update traffic, or read application data. |
| Cloud Functions | function description | State, runtime, update time, build or service linkage | Do not deploy, call the function, or change triggers. |
| Pub/Sub | topic or subscription description | Existence, retention, dead-letter and retry configuration relevant to the flow | Message bodies, pulls, seeks, acks, and publishes are data or mutations. |
| Cloud Scheduler | job description | Enabled or paused state, schedule, target identity, last attempt metadata when exposed | Do not run, pause, resume, or update a job. |
| Eventarc | trigger description | Conditions, destination, transport topic, active conditions | Do not create, update, or delete triggers. |
| Cloud SQL | instance description | Instance state, region, database version, availability configuration | SQL contents and PostgreSQL internals belong to `inspect-database` only after the parent skill's adapter-availability gate passes. |
| Firestore | database description | Database identity, location, type, concurrency mode, deletion protection when exposed | Documents, collections, exports, and writes are outside this inspection branch. |
| GKE | cluster description followed by explicit-context Kubernetes reads | Control-plane and workload state | Read [gke.md](gke.md). |

For another managed service, traverse `gcloud help` to a leaf `list` or `describe` command only after the target card is exact. If the installed CLI does not expose a read-only leaf for the required signal, return `query-unsupported`.

## Correlation

Use the business flow order to correlate participant state. Prefer timestamps, resource conditions, revision or generation identifiers, and bounded errors. A downstream failure is not proof that the upstream component caused it; state the observed sequence and the missing edge separately.

## Completion criterion

This branch is complete when each selected service has its identity and relevant control-plane state, any requested observability handoff, and a clear data or mutation boundary.
