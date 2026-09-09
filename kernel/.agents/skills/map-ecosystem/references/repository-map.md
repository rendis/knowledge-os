# Map repository purpose, main flows and connectors

This is the default scope for repository mapping and sync extraction. A map explains what the repository does, how work enters, what meaningful logic executes and which effects leave it. It is not an exhaustive code, security or deployment audit.

## Trace once

1. Bind the exact configured main/master revision. Reuse valid existing notes and analysis at that revision; use a diff for an existing map. Preserve supported facts, relationships and provenance even when they exceed today's scope. Missing mention is never deletion evidence.
2. Read stack manifests, composition/registration code and README or other local documentation selectively. Use documentation when it adds a description supported by implementation; report material contradictions. An empty or generic README needs no separate investigation.
3. Find API registrations, subscriptions, push handlers, schedules, batch/CLI and other triggers. Distinguish business entrypoints from health/diagnostic endpoints. Follow each main entrypoint through its handler into reads, writes, publications and outbound calls. Record shared branches once.
4. For each connector record direction/operation, protocol, resource or config key, environment when resolved, and source reference. Include database reads as well as writes, HTTP/gRPC/FTP/file/object-store effects and messaging as observed. Mark unused/example wiring separately. Keep unresolved dynamic destinations explicit.
5. Explain only transformations, validation, routing/filter conditions, calculations, state changes or failure behavior that materially changes a main flow. Describe passthrough briefly. Summarize mappings and special rules; field-by-field DTO copies and generic utility internals remain in source.
6. Return a concise map with purpose and stack; entrypoints; main flow sequences; connectors; relevant rules/mappings; evidence and limits. Cite the bound commit, complete repository-relative path and line range or exact symbol beside assertions. Keep each reference independently resolvable; ellipses or abbreviated paths are not evidence anchors. Keep code/configuration observations distinct from observed runtime. Stop when every discovered main entrypoint has a flow or explicit gap, significant effects have an owner or gap, and no known contradiction misstates a main flow. Unresolved peer identities and full deployment topology do not block this result.

For documentation-only, library, schema and IaC repositories, describe their actual role and exposed contracts/resources. Do not invent a service or triggers. Node eligibility is a later publication decision, not a reason to discard inventory evidence.

## Verify proportionally

Use existing manifest/scanner tools for file inventory, frozen revision, shape and references. Scanner matches are leads, not proof of reachability or completeness. Reuse a successful scan. Structural failure goes to deterministic normalization before another model call.

Freeze a completed candidate before review and record its content hash; changes during review invalidate that review binding. A fresh reviewer checks the submitted map and the exact sources behind its main entrypoints, connectors and meaningful rules. Use targeted registration/composition searches to detect a missed main flow; do not reconstruct the whole repository or require every configuration value and utility rule. A README alone cannot validate an implementation assertion. Verify remote read/write direction from the operation actually called, separately from local variable assignments. For claimed ACK/NACK, retry or success behavior, follow the relevant return/throw through its caller and catch to the terminal effect; do not generalize across different branches.

Accept a useful partial map when its limits are explicit and it accurately describes the observed main flows. Request correction for unsupported assertions, materially wrong destinations/conditions, missing main flows or loss of valid knowledge. Missing secondary detail is a limitation, not a whole-map rejection. Target isolated defects to their claims so supported subsets remain usable. Review verdicts never override evidence or publication authorization.

## Connect after local maps

The coordinator matches proven endpoints across repositories, retaining environment/project/resource identities. A local worker can finish with a topic publication and an unresolved subscription-to-topic relation. With authorized read-only access, resolve that relation centrally from cloud configuration: for Pub/Sub, list the known topic's subscriptions and inspect subscription topic, filter and push target when relevant. Reuse each observation with resource identity and observation time; distinguish source revision from live configuration time. Resolve the consumer from its subscription configuration or evidenced push endpoint. A topic name resemblance is not an edge.

Keep broker/subscription filters distinct from application-side attribute checks, payload conditions and routing/discard rules. Describe which events can reach which consumer under those conditions. Missing cloud access leaves only dependent edges unresolved. A configured edge proves possible routing, not a delivered event. Use the selected provider's read-only capability; do not assume GCP for other cells.

## Read efficiently

Use inventory and targeted registration/connector searches before opening implementation ranges. Batch independent narrow reads; avoid dumping complete files or whole source trees into the context. Reuse already inspected evidence. Keep a compact list of main entrypoints, effects and unresolved questions as you work.

Default pilot envelope: extraction up to 8 source-read tool calls, review up to 6; at most 1,500 output tokens per call and 8 minutes per worker. A call may batch targeted ranges. These are experiment bounds, not a completeness claim: at the boundary, write the supported result and explicit gaps. Record any coordinator-approved extension before dispatch. Schema/format validation runs separately and does not spend semantic read calls. The coordinator checks actual call/output counts and interrupts overruns; prompt limits alone are not enforced quotas.

## Execution budget and reuse

Choose model and effort explicitly for the bounded task. Test an available lighter model on a representative service and a complex case before a batch. A peer using the same model in a fresh context may review it; a different or larger model is not inherently required. Escalate a specific unresolved reasoning problem, not every code repository.

Before dispatch record scope, model/effort, time and usage limits in local execution records. Use runner-enforced limits when available; otherwise limit dispatch count/concurrency and monitor usage, explicitly reporting that a hard token cap is unavailable. Start with at most two pilot extractions and one fresh review per map, no automatic corrections. Stop the pilot on timeout or excessive usage and diagnose before expanding. Report actual time and usage separately from quality.

For an established batch, select small independent groups, persist each completed result promptly and avoid waiting for unrelated repositories to publish the group. Keep the existing sync state/grant checks; choose the group before beginning its run. Reuse valid artifacts instead of rerunning extraction because instructions, rendering or publication changed. Reassess only conclusions actually affected by the change, keeping original bytes and verdict history.
