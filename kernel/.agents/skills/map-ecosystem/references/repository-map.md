# Map repository purpose, main flows and connectors

This is the scope for repository mapping and sync. A map explains what the repository does, how work enters, what meaningful logic executes and which effects leave it. It is not an exhaustive code, security or deployment audit.

## What a useful map answers

The reader can explain the service's behavior, follow its main data movements and locate the evidence for a change or investigation without rediscovering the repository. Answer these across the repository note and its linked flow/integration/topic/event notes.

| ID | Question | Required answer |
| --- | --- | --- |
| P1 | What does this repository do, and what is its responsibility? | Purpose, owned responsibilities, boundaries, language/framework and role within the system. |
| P2 | How does it start, and what makes it act? | Startup/composition path, essential configuration keys, and business triggers: routes, subscriptions, schedules, batches or CLI. Distinguish internal scheduling from external callers and diagnostics from business work. |
| P3 | What happens in each main flow, from trigger to result? | Trigger → relevant decisions/transformations → reads/writes/calls → result or failure. Keep branches with different effects. |
| P4 | Where does it obtain, store, modify or send data? | Every connector with direction, operation, protocol and the evidenced table, topic/subscription/event, endpoint, bucket or configuration key; its owning flow; unresolved destinations. |
| P5 | Which rules determine the outcome? | Material filters, validation, calculations, mappings, state transitions, ordering, retries/ACKs, idempotency or failure handling, with conditions and consequences. |

A missing answer is a gap, not a non-applicable item. For library, schema, IaC and documentation repositories describe their actual role and exposed contracts/resources; do not invent triggers.

## Start from discovery facts

Run `<cli> discover run --vault "<vault>"` (add `--repo <name>` for one repository) and read `<cli> discover report --vault "<vault>" --repo <name>`. The facts are the connector inventory, complete by construction:

- `dependencies` and `channels`: every library and runtime module the code imports (company libraries resolved to what they import), with the files that use each one, plus declared-but-not-imported manifest dependencies.
- `resources`: every topic, subscription, event, database object, HTTP endpoint and bucket named by the repository's configuration, IaC blocks that name the service, or code literals, each with file/key evidence; Pub/Sub resources carry platform wiring (subscription → topic, filtered events) when snapshots exist.
- `pending`: what could not be confirmed (no platform access, name absent from the platform, unjudged dependency), with the exact command that confirms it.
- The comparison with the current note: supported relations, discrepancies and undocumented resources.

Do not rediscover connectors by searching the repository. Explain each fact's role, trace the flows that reach it, and give every fact an owner flow, a documented relation or an explicit limitation. A fact you judge irrelevant is still named once with the reason. Missing platform or source access becomes a `Verificaciones pendientes` item with the fact's confirm command; never present an unconfirmed destination as confirmed.

## Trace flows once

1. Bind the exact revision the facts were computed at (the facts' `commit`). For an existing note, read it first and keep supported facts, relationships and provenance; a missing mention is never deletion evidence. For sync, inspect the delta since `commit-analizado` and the files its cited anchors point to (`discover check` lists stale cited files).
2. Read manifests, startup/composition code and README selectively to answer P1–P2. Documentation adds description only when implementation supports it; report material contradictions.
3. For each business entrypoint, follow the handler to the reads, writes, publications and calls that the facts list (P3–P4). Record shared branches once and keep dynamic destinations explicit.
4. Explain only transformations, conditions, calculations, state changes and failure behavior that change a main flow (P5). For changed filters, time windows and retry limits keep decision order, units and boundary inclusivity from the exact predicate. For claimed ACK/NACK, retry or success behavior follow the return/throw to its terminal effect.
5. Stop when every entrypoint has a flow or explicit gap, every fact has an owner or limitation, and no known contradiction misstates a main flow.

## Evidence format (checked mechanically)

Cite at the bound commit, one verifiable fact per cited sentence, with the reference right after the fact it supports:

```markdown
Publica el ajuste serializado en el topic configurado por `GCP_PUBSUB_TOPIC_IN`. [^e3]

[^e3]: [src/services/gcp.go](https://github.com/<org>/<repo>/blob/<full-sha>/src/services/gcp.go#L27-L48) — L27-L48: `os.Getenv("GCP_PUBSUB_TOPIC_IN")` y `publishToPubsubWithRetry`
```

- Backticked identifiers in a footnote must appear in its cited lines; backticked paths must exist at the commit.
- Platform facts cite the snapshot: `[^p1]: platform gcp-pubsub <project> captured <date> — <subscription> → <topic>, filter <expression>`.
- Limitations, negations and inferences are written as limits, not cited as if the code proved them.
- Distinguish implementation, configured behavior and observed runtime.

## Gates before review

Run `<cli> discover check --vault "<vault>" --note <candidate>` (add `--semantic` when Jev is configured). Fix every `error`: G1 (an anchor that does not resolve, lines that do not exist, an identifier absent from the cited lines) and G2 (a connector category or resource group of the facts that the note neither cites nor names). Carry `pending` items into `Verificaciones pendientes`. `review` items (including G3: a declared topic/event relation the facts do not name) and semantic `says_nothing`/`contradicts` results go to the reviewer with their sentences. Mechanical fixes never reopen source analysis.

## Review

A fresh reviewer receives the candidate, the baseline, the facts and the check output, and checks changed meaning and preservation against the exact sources behind main entrypoints, connectors and rules. Anchors that passed G1 are not re-verified for existence; the reviewer judges interpretation. Request correction for unsupported assertions, wrong destinations/conditions, missing main flows or lost valid knowledge; missing secondary detail is a limitation. A finding repairs only the affected text from available evidence and is re-reviewed; it does not restart extraction. Accept a useful partial map when its limits are explicit.

## Connect after local maps

When an external end matters to the selected map, flow or investigation, use [connection-reconciliation.md](connection-reconciliation.md). Platform snapshots (`discover platform`) usually resolve subscription → topic → publisher without re-reading other repositories.

## Budget

One authoring context per repository delta and one review per candidate. Delegate only an independent unresolved question whose evidence returns compactly. Record model, effort, time and usage before dispatch; stop on overruns and diagnose before expanding. Reuse candidates, facts and reviews instead of re-running analysis because instructions or rendering changed.
