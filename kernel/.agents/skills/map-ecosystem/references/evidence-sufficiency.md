# Evidence sufficient for the requested task

Apply this boundary before extraction, reconciliation and review. A map explains a system; an investigation tests a specific hypothesis. Fix the requested outcome before deciding what remains pending.

| Outcome | Sufficient evidence | Stop boundary |
| --- | --- | --- |
| Repository/system map | Versioned implementation and configuration establish purpose, startup, main flows, meaningful rules, operations, destinations and peer relationships. | Describe implemented/configured behavior. Successful executions, traffic, messages and business rows are not prerequisites. |
| Infrastructure enrichment | Authorized metadata identifies the selected deployed resources and their relevant bindings in the named environment. | A topic/subscription binding or configured sink establishes that relationship, not delivery or persistence. Functional tests are not required. |
| Investigation or operational verification | Evidence answers the explicitly requested hypothesis: for example delivery failure, actual persistence, runtime health or deployment success. | Acquire only the observations needed for that hypothesis, under the existing access rules. |

Keep evidence labels precise even when a map is complete. A main-branch commit and production configuration support the configured schema/pipeline, not a successful production execution. Subscription metadata supports its topic/filter/destination, not actual consumption. Source registration and exact resource bindings can identify a caller or consumer without observing traffic. A name resemblance, unexplained dynamic target or contradictory binding remains a material mapping gap.

## Pending items and limits

A mapping pending item identifies missing information material to understanding a main flow, destination, relationship or rule under P1–P5. Record its exact question and the least evidence sufficient to answer it. Missing access stays pending only for a selected, material question requiring that access; preserve explicit user deferral.

Unverified functional behavior is a limitation, not an automatic checklist item. For example, a known publisher, topic, subscription and consumer need no message inspection to close their mapping relationship. Known DDL and production bindings need no successful Liquibase job to explain the intended schema. If the user explicitly requests verification of execution or delivery, retain that investigation question separately from mapping completion. Do not create an investigation or an optional-work backlog merely because more could be tested.

Before reusing an old checklist, reassess its close conditions against the current task. Keep genuine information gaps. Refine mixed items to the missing mapping information; move out-of-scope functional demands to concise limitations. Remove obsolete/duplicate demands with an explicit scope-based disposition, preserving supported facts, connection anchors and evidence. A retired verification is not a newly observed success; keep closure counts distinct from tests performed. Review this reclassification against the old note and the requested scope instead of demanding the obsolete test.

An owner/user confirmation can establish that an old integration or delivery channel is retired. Record the confirmation and retain useful history; exclude that legacy path from current pending work. File presence alone does not establish current use, and file age alone does not establish retirement. This cell-specific confirmation must not become a global product/provider rule.

The reviewer rejects unsupported claims and material P1–P5 gaps, not missing optional runtime evidence. It also rejects a pending list that silently expands a map into functional testing. Preserve stronger existing evidence when available; this boundary changes what must be acquired, not what may truthfully be claimed.
