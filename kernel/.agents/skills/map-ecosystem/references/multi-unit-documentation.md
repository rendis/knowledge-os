# Document several related units

## Input and authority

Accept a domain, flow, or explicit unit set. Confirm write authorization, load [node-selection.md](../../../../90-Meta/node-selection.md), and define the boundary: source units, system, environments, and synthesis question.

## Per-unit analysis

1. Build the initial inventory from MOCs, services, flows, operational indices, and backlinks.
2. Split units into non-overlapping groups. For massive work, delegate complete analyses by group: source code → note → detected topics/integrations.
3. Apply the [single-unit documentation recipe](single-unit-documentation.md) to every unit without synthesizing global relationships before local analyses are complete.
4. Record per unit: commit/branch when applicable, contracts, data, runtime, deployment matrix by environment when applicable, observed relationships, and limitations.

## Synthesis

1. Normalize logical topic and resource names according to Convenciones; retain real variants in `nombre-raw` or the note body.
2. Resolve asynchronous edges through topics and HTTP edges through direct links.
3. Update composite services, runtime, topics, integrations, flows, glossary, operational notes, MOCs, and indices only when the [node-selection standard](../../../../90-Meta/node-selection.md) and evidence threshold are satisfied.
4. Detect contradictions across units; preserve both pieces of evidence and record the issue as a limitation until resolved.
5. Run the gates once per group and again for the complete synthesis.

## Output

Report analyzed units, changes per node, new/modified/removed relationships, evidence, contradictions, limitations, and observed checks.

## Completion criterion

The unit set is complete when every unit has an individual decision, every deployable unit has a resolved or explicitly limited row for each observed environment, every cross-unit relationship was evaluated during synthesis, derived nodes were propagated, and the complete vault passes its gates.
