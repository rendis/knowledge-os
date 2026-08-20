# Select and maintain vault nodes

## When to load

Load this reference before any authorized vault write and for any read-only question about where a fact belongs. Keep the request's interrogation, single-unit, multi-unit, or synchronization branch as the primary branch.

## Authority

From `VAULT_ROOT`, use `90-Meta/Convenciones.md` as the only normative catalog. Read **Estructura**, **Selección del tipo de nodo**, **Ciclo de vida común**, **Nombres de notas**, and the contract for the selected type. Use `AGENTS.md` for guardrails and `90-Meta/Auditoria - Framework.md` for evidence and gates. Do not copy a nearby note when it conflicts with the current contract.

## Selection procedure

1. State the observed subject independently of any folder.
2. Search exact basenames, aliases, `nombre-raw`, wikilinks, and backlinks. Prefer updating the canonical node over creating a synonym or environment/country duplicate.
3. Apply the ordered identity tests below, then confirm the choice against the matrix in Convenciones:
   - In-scope source repository → repository note.
   - Stable capability composed of multiple units → service.
   - Independently identifiable deployable inside a multi-deployable repository → exceptional component.
   - Productive execution/trigger resource without an identifiable repository → runtime resource.
   - Logical asynchronous message contract → topic.
   - System, provider, platform, or API outside the cell boundary → external integration.
   - End-to-end sequence with a business outcome → flow.
   - Shared domain term needed to interpret nodes or a critical contract → glossary.
   - Human/team guide, catalog, standard, ordered procedure, or individual report contract → operational; place it under the single owning area and represent secondary areas with typed links.
   - Reusable, evidence-bounded conclusion derived from an investigation → learning; route assessment and publication to `manage-investigation-derived-learning`.
   - Curated navigation or system-wide aggregation → index or system MOC.
   - Documentation contract, validator, reusable helper, or derived view → Meta or Base.
4. If two tests appear to match, use the distinction rules in Convenciones. Do not create a node until the ambiguity is resolved by evidence; record a business-relevant limitation in the nearest canonical node when it cannot be resolved.
5. Select exactly one lifecycle action: create, update, consolidate/rename, retire, or no change. The production-evidence gate is a prerequisite for technical create or update; when it fails, select no change even if the candidate comes from an approved investigation or planned implementation. For a learning, defer to the domain actions `create`, `enrich`, `challenge`, `supersede`, or `none` and its independent durable-learning gate.

## Placement and writing

1. Place the note in the folder owned by the selected type. Never introduce a new folder or type as a local workaround.
2. Apply its canonical filename, closed frontmatter, required sections, language, and relationship direction from Convenciones.
3. Keep low-level tables, buckets, datasets, subscriptions, endpoints, and minor infrastructure as properties or prose unless the selection matrix qualifies them as runtime or integration nodes.
4. Do not add repository analysis fields to non-repository notes. Do not change repository commit traceability unless that repository was actually re-analyzed.

## Propagation

Evaluate every affected node and record a decision:

- Repository or component behavior → topics, integrations, flows, service/runtime, system MOC, and relevant glossary terms.
- Topic contract → publishers/subscribers through their properties, affected flows, and infrastructure evidence; never add manual consumer lists.
- Integration contract → callers, affected flows, and glossary only when the external term meets the glossary threshold.
- Flow → participant notes and `participa-en` where supported by their contracts.
- Service/runtime → system MOC, flows, and architecture views.
- Glossary → canonical wikilinks at meaningful uses; keep usage discovery backlink-driven.
- Operational note → owning-area MOC, operational index/Base, linked operational notes, AGENTS routing, catalog resolver, and execution-skill references; do not add it to business-flow participation.
- Learning note → learning index, canonical `aplica-a` links, supersession links, AGENTS routing, validator, and any independently qualified technical-map candidate; do not mutate technical node contracts merely to add a forward link.
- Schema or folder change → AGENTS routing, README structure, affected Bases, validators, and Framework documentation.

## Completion criterion

Node selection is complete when the chosen type and lifecycle action satisfy Convenciones, no canonical duplicate exists, placement and schema are valid, every affected node has a propagation decision, and the applicable Framework gates pass.
