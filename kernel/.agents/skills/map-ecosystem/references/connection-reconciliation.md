# Reconcile external connections after local mapping

Use this phase only after a service-local map has passed its independent review or has an explicitly accepted partial subset. Its input is the accepted `connection.*` claims. Preserve that reviewed artifact: external evidence creates separate claims and node decisions rather than changing the completed local extraction.

## Select and resolve

1. Select only connections required for the current system map, flow or investigation. Carry forward the local claim ID, owning flow, direction, operation, protocol, resource/configuration key, environment scope, terminal behavior and unresolved question.
2. Search the least expensive authoritative source that can answer the question:
   - versioned sibling service or caller/consumer source;
   - source pinned by the service's dependency manifest;
   - versioned IaC, schema or deployment configuration;
   - authorized read-only provider or database observation when source evidence cannot establish current resource identity or state.
3. Match exact contracts. For HTTP use the configured base/route, method and authentication boundary. For messaging use project/resource identity, topic/subscription binding, filter, push target and application-side routing. For databases use instance/database/schema plus the actual operation and object. For files or object stores use the resolved path/bucket and operation. Similar names are leads, not edges.
4. Record one status for every selected connection:
   - `resolved`: the questioned external end or resource identity is established by authoritative evidence;
   - `partial`: useful identity or behavior is established but a material environment, deployment or runtime question remains;
   - `unresolved`: inspected sources do not establish the other end.
5. Create or update the affected repository, flow, topic, integration, data or operational nodes through the ordinary documentation recipe. Bind versioned evidence to its commit and live observations to environment and observation time. Keep configured routing, deployed resources and observed delivery or persistence as separate claims.

## Boundaries

A configured edge proves possible routing. It does not prove deployment, successful delivery, persistence or current data. A provider response proves only the fields and observation time actually returned. Secret values never become connection evidence; retain the secret reference or configuration key.

Do not reopen an accepted service map merely because an external end remains unknown. Reconcile another connection only when it changes the selected map, flow or investigation. A local sync reuses stable connection claim IDs for unchanged connector slots; a changed destination updates the same claim, while a genuinely added or removed connector adds or retires its own claim.

## Completion criterion

The phase is complete when every selected local connection claim has exactly one `resolved`, `partial` or `unresolved` decision, each new relationship has independently resolvable evidence, affected nodes have explicit actions, and remaining runtime questions are stated without weakening the accepted local map.
