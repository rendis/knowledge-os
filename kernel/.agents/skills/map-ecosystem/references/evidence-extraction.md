# Extract a useful, evidence-backed map

For repository mapping and synchronization, use [repository-map.md](repository-map.md) as the scope and sufficiency contract. Publication mechanics remain in the selected recipe.

## Scope and evidence

For a new repository, trace main entrypoints through relevant logic to connectors and effects. Record each externally significant connector as the stable `connection.*` claim defined by repository-map.md. For an existing map, start from the frozen diff and existing knowledge; inspect unchanged dependencies only where they explain changed meaning. Resolve each source by configured identity and exact revision. Code and configuration support source-level assertions; stronger runtime assertions need their own evidence under `90-Meta/evidence-policy.md`.

The existing checklist dimensions (inputs, outputs, data, business behavior, infrastructure, deployment) organize the map; they are not six exhaustive audits. Answer applicable questions to main-flow depth. Record secondary or inaccessible details as limits. Read environment configuration selectively to resolve connectors, schedules and behavior-changing flags; omit secret values rather than excluding all safe facts in the file. Preserve seeded evidence entries as bookkeeping without reconstructing every deployment chain.

## Preservation

Compare against existing notes and reusable analysis. Preserve supported facts and relationships, including details beyond the current map scope. Missing mention, inaccessible evidence or absent search results are not proof of removal. A removed source file requires checking replacement wiring before retiring behavior. Keep conflicting environment values scoped. Instruction or model changes alone do not invalidate accurate source observations.

## Review and closure

Apply the directed review in repository-map.md. Check material assertions, main-flow omissions, connection claims and changed values that survive incorrectly after sync, not exhaustive implementation coverage. One optional finding-directed correction preserves the original and unrelated accurate facts; sync uses `<cli> sync correction`. A structural problem goes to deterministic tooling. An unresolved isolated defect limits only dependent claims. An unresolved external end is a recorded reconciliation lead and does not fail an otherwise complete local map. An omitted secondary detail does not establish that a valid map is wrong.

Before publishing, apply [final-note-review.md](final-note-review.md) to the complete resulting note images, not only the analysis claims. The fresh reviewer checks purpose, main triggers, significant effects/destinations, conditions, stale statements, pending close conditions and evidence. Bind its decision to the exact candidate, baseline and evidence hashes. Structural checks do not prove semantic correctness. A partial map stays explicit about limits; do not claim exhaustive coverage or verified deployment.
