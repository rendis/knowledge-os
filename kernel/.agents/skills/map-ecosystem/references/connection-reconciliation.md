# Reconcile external connections after local mapping

Use this phase only after a service-local map has passed its independent review or has an explicitly accepted partial subset. Its input is the accepted `connection.*` claims. Preserve that reviewed artifact: external evidence creates separate claims and node decisions rather than changing the completed local extraction.

## Access gate

Use repository, dependency, IaC and schema evidence first. When a material question still requires a live provider, cluster or database, run the capability-scoped readiness check from `operational-readiness.md` for that exact authority and target.

- When ready, execute only the bounded read-only query needed by the pending item's close condition.
- When a configured fallback can answer less, record that smaller evidence and keep the remaining question partial.
- When access or its local binding is unavailable, complete independent static work, retain the pending item, and hand the exact capability, target and read-only question to `configure-workspace`. Ask after static evidence has narrowed the request; never ask for broad platform access without a concrete target.

After the binding becomes available, rerun readiness and resume the same pending `connection.*` item. Reuse the accepted local map and prior external evidence; access acquisition does not trigger repository extraction again.

## Select and resolve

1. Select only connections required for the current system map, flow or investigation. Start from the repository note's `Verificaciones pendientes` item and carry forward its local claim ID, owning flow, direction, operation, protocol, resource/configuration key, environment scope, terminal behavior, unresolved question and close condition. Group selected connections by authoritative repository, provider/project, cluster context or database target so one bounded observation can resolve every directly supported item without repeated access.
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
5. Create or update the affected repository, flow, topic, integration, data or operational nodes through the ordinary documentation recipe. Bind versioned evidence to its commit and live observations to environment and observation time. Keep configured routing, deployed resources and observed delivery or persistence as separate claims. Remove a pending item only when its stated close condition is met; otherwise refine it with the remaining question and retain `partial` or `unresolved`.

## Boundaries

A configured edge proves possible routing. It does not prove deployment, successful delivery, persistence or current data. A provider response proves only the fields and observation time actually returned. Secret values never become connection evidence; retain the secret reference or configuration key.

Do not reopen an accepted service map merely because an external end remains unknown. Reconcile another connection only when it changes the selected map, flow or investigation. A local sync reuses stable connection claim IDs for unchanged connector slots; a changed destination updates the same claim, while a genuinely added or removed connector adds or retires its own claim.

## Completion criterion

The phase is complete when every selected local connection claim has exactly one `resolved`, `partial` or `unresolved` decision, each new relationship has independently resolvable evidence, affected nodes have explicit actions, resolved items were removed from the verification list, and partial or unresolved items state their next check and close condition without weakening the accepted local map.
