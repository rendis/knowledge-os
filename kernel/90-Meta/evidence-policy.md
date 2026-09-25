# Evidence by cell profile

Read `instance.yaml` `evidence.profile` before a technical documentation decision. A synchronization worker uses the coordinator's frozen `evidence_profile`; legacy cards without that field retain `production-gate`.

The router's evidence contract governs how evidence is obtained and graded; `evidence-driven-analysis` (`../.agents/skills/evidence-driven-analysis/SKILL.md`) adds the diagnosis and audit methods. This policy owns publication thresholds; adapters and configured procedures own source access and environment resolution. Applying the method alone authorizes no instrumentation, code changes, processing, deployment or publication.

| Profile | Required support for a technical claim |
| --- | --- |
| `documented-source` | Inspected versioned implementation, schema, configuration or contract at an exact revision. Describe source behavior at that revision; deployment is not a prerequisite. |
| `production-gate` | Inspected implementation plus evidence of productive applicability, as defined in the Framework production gate. |
| `mixed` | Apply `production-gate` to technical claims; evaluate operational procedures against their declared authority. |

## Source repository maps

Across these profiles, a map may describe implementation on the selected repository reference branch (see [[reference-branches]]), connectors and versioned configuration at an exact revision without proving production applicability. Label these as source/code/configuration observations; this rule covers their evidence-backed local flows and connections, not assertions that they are deployed, enabled or executed. Missing deployment evidence limits those stronger assertions only. Keep future proposals separate and preserve already supported knowledge. This source-map rule also covers investigation-discovered corrections to existing canonical source maps; the investigation routes them through map-ecosystem review rather than promoting its case as evidence. Production reality, other investigation promotion and learning gates retain their stronger requirements.

For every profile, evidence must support the precise assertion. Source configuration supports what is configured; asserting effective deployment or runtime behavior requires observed deployment/runtime evidence. A branch name, proposal, accepted design or successful test alone does not prove deployment. Label inference and unavailable evidence explicitly. Future proposals remain investigation context, not confirmed technical facts.

A missing source blocks only dependent claims. Retain independently supported claims and their actual evidence level. Use the note locale for explanations. Profiles do not authorize writes, access, or changes to external systems.
