# Reconcile external connections after local mapping

Use this phase only after a service-local map has passed its independent review or has an explicitly accepted partial subset. Its input is the accepted `connection.*` claims. Preserve that reviewed artifact: external evidence creates separate claims and node decisions rather than changing the completed local extraction or entering its closed Git synchronization package.

Apply [evidence-sufficiency.md](evidence-sufficiency.md) before requesting access or reusing a pending close condition.

When the question requires another domain vault discovered through authorized repository exploration or supplied by the user, follow [cross-vault-consultation.md](cross-vault-consultation.md) before reading its domain content. Return its evidence and limits to this workflow.

## Access gate

Use repository, dependency, IaC and schema evidence first. A discovered connection identifies a question and target; it grants no access. When a material question still requires a live provider, cluster or database, resolve the configured executor or adapter for that procedure and run its bounded read-only target probe. Reuse the existing authenticated identity or session selected by that procedure. The probe must establish both read-only operation and the exact authority and target before the observation proceeds.

- When the exact probe passes, execute only the bounded read-only query needed by the pending item's close condition.
- When a configured fallback can answer less, record that smaller evidence and keep the remaining question partial.
- When the procedure binding is missing, complete independent static work, retain the pending item, and hand the exact capability, target and read-only question to `configure-workspace`; workspace configuration binds the procedure but does not grant target access.
- When the procedure is bound but its probe cannot use the existing authentication, ask for the exact missing executor, profile, role, session, network route or target-specific setup reported by the procedure. Keep credential values outside the vault and preserve the configured access path.

After the binding or access setup becomes available, rerun the same executor or adapter probe and resume the pending `connection.*` item. Reuse the accepted local map and prior external evidence; access acquisition does not trigger repository extraction again.

## Select and resolve

1. Select only connections required for the current system map, flow or investigation. Start from the repository note's `Verificaciones pendientes` item and reassess its close condition for the requested outcome; carry forward its local claim ID, owning flow, direction, operation, protocol, resource/configuration key, environment scope, terminal behavior, unresolved question and close condition. Group selected connections by authoritative repository, provider/project, cluster context or database target so one bounded observation can resolve every directly supported item without repeated access.
2. Search the least expensive authoritative source that can answer the question:
   - related accepted vault notes and their cited evidence at the relevant revision;
   - versioned sibling service or caller/consumer source;
   - source pinned by the service's dependency manifest;
   - versioned IaC, schema or deployment configuration;
   - authorized read-only provider or database observation when source evidence cannot establish current resource identity or state.
3. Match exact contracts. For HTTP use the configured base/route, method and authentication boundary. For messaging use project/resource identity, topic/subscription binding, filter, push target and application-side routing. For databases use instance/database/schema plus the actual operation and object. For files or object stores use the resolved path/bucket and operation. Similar names are leads, not edges.
4. Record one status for every selected connection:
   - `resolved`: the questioned external end or resource identity is established by authoritative evidence;
   - `partial`: useful identity or behavior is established but a material question remains for the selected mapping, infrastructure or investigation scope;
   - `unresolved`: inspected sources do not establish the other end.
5. Create or update the affected repository, flow, topic, integration, data or operational nodes through the ordinary documentation recipe. Bind versioned evidence to its commit and live observations to environment and observation time. Keep configured routing, deployed resources and observed delivery or persistence as separate claims. Close an in-scope question when its sufficient evidence is met. Reclassify an obsolete or out-of-scope demand under evidence-sufficiency.md with an explicit disposition; this is not proof that the old functional test passed. Retain `partial` or `unresolved` only for material questions remaining in scope.

## Publication

Publish external reconciliation through the ordinary [single-unit](single-unit-documentation.md) or [multi-unit](multi-unit-documentation.md) documentation recipe, outside the closed Git synchronization package. Treat that package as immutable accepted context; a later provider, cluster or database observation cannot be appended to it or presented as evidence frozen at its source revision.

Before applying any note change, follow [final-note-review.md](final-note-review.md). Freeze the complete resulting bytes for every target note together with their baselines and the exact versioned or observation evidence hashes. An independent reviewer must accept that complete candidate and evidence binding. Apply only the reviewed resulting bytes; when review rejects or any baseline or evidence binding changes, preserve the current notes and prepare a fresh candidate and review. Durable notes must cite the authoritative technical evidence; ignored local candidate and review artifacts are lineage records, not the sole source for a claim.

## Boundaries

A configured edge proves possible routing. It does not prove deployment, successful delivery, persistence or current data. A provider response proves only the fields and observation time actually returned. Secret values never become connection evidence; retain the secret reference or configuration key.

Do not reopen an accepted service map merely because an external end remains unknown. Reconcile another connection only when it changes the selected map, flow or investigation. A local sync reuses stable connection claim IDs for unchanged connector slots; a changed destination updates the same claim, while a genuinely added or removed connector adds or retires its own claim.

## Completion criterion

For a system map or synchronization campaign, [mapping-completion.md](mapping-completion.md) defines the required scoped pass, evidence-source classification and visible coverage check.

The phase is complete when every in-scope local connection claim has exactly one `resolved`, `partial` or `unresolved` decision, each new relationship has independently resolvable evidence, affected nodes have explicit actions, and every applied note exactly matches an independently accepted final candidate. Each former pending item is answered by sufficient evidence, retained as a material in-scope question, or explicitly retired as obsolete/out of scope under evidence-sufficiency.md. Retired demands are not observed successes; preserve their connection anchors and supported knowledge.
