# Evidence by cell profile

Read `instance.yaml` `evidence.profile` before a technical documentation decision. A synchronization worker uses the coordinator's frozen `evidence_profile`; legacy cards without that field retain `production-gate`.

| Profile | Required support for a technical claim |
| --- | --- |
| `documented-source` | Inspected versioned implementation, schema, configuration or contract at an exact revision. Describe source behavior at that revision; deployment is not a prerequisite. |
| `production-gate` | Inspected implementation plus evidence of productive applicability, as defined in the Framework production gate. |
| `mixed` | Apply `production-gate` to technical claims; evaluate operational procedures against their declared authority. |

For every profile, evidence must support the precise assertion. Source configuration supports what is configured; asserting effective deployment or runtime behavior requires observed deployment/runtime evidence. A branch name, proposal, accepted design or successful test alone does not prove deployment. Label inference and unavailable evidence explicitly. Future proposals remain investigation context, not confirmed technical facts.

A missing source blocks only dependent claims. Retain independently supported claims and their actual evidence level. Use the note locale for explanations. Profiles do not authorize writes, access, or changes to external systems.
