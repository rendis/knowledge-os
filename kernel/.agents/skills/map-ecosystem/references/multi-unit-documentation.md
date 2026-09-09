# Document several related units

## Input and authority

Accept a domain, flow, or explicit unit set. Confirm write authorization, load [node-selection.md](../../../../90-Meta/node-selection.md), and define the boundary: source units, system, environments, and synthesis question.

Use [evidence-extraction.md](evidence-extraction.md) as the common extraction contract.

## Per-unit analysis

1. Build the initial inventory from MOCs, services, flows, operational indices, and backlinks.
2. Group units by changed contracts and direct impact; include unchanged participants when needed. Split write ownership into non-overlapping groups. For a batch, use the bounded pilot and small-group policy in repository-map.md; workers return local maps before cross-unit reconciliation.
3. Apply the [single-unit documentation recipe](single-unit-documentation.md) to every unit without synthesizing global relationships before local analyses are complete.
4. Record per unit: commit/branch when applicable, contracts, data, stack/runtime basics, connector-relevant environment differences, observed relationships, and limitations.

## Synthesis

1. Normalize logical topic and resource names according to Convenciones; retain real variants in `nombre-raw` or the note body.
2. Resolve asynchronous edges through topics and HTTP edges through direct links.
3. Update composite services, runtime, topics, integrations, flows, glossary, operational notes, MOCs, and indices only when the [node-selection standard](../../../../90-Meta/node-selection.md) and evidence threshold are satisfied.
4. Detect contradictions across units; preserve both pieces of evidence and record the issue as a limitation until resolved.
5. Validate source-derived cross-unit questions against the resulting notes, then run the gates once per group and again for the complete synthesis.

## Output

Report analyzed units, changes per node, new/modified/removed relationships, evidence, contradictions, limitations, and observed checks.

## Completion criterion

The unit set is complete when every unit has an individual decision, every main-flow connector has evidence or an explicit limitation, every cross-unit relationship was evaluated during synthesis, derived nodes were propagated, and the complete vault passes its gates.
