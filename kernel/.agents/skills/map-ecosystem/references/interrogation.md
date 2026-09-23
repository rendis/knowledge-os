# Query the vault

## Input and authority

Accept a question, proposed change, or starting node. Resolve aliases only to find a note; navigate and report using canonical basenames. Keep this branch entirely read-only.

When evidence needs to be obtained or evaluated, read [evidence-driven-analysis](../../evidence-driven-analysis/SKILL.md) in full before that analysis unless it is already loaded in the current context. Reuse the active method and valid checks; navigation alone does not require another analysis pass. This reference owns navigation and source identity; the shared method does not restart routing. If called as an auxiliary by another workflow, return observations, sources and limits to that owner.

When the question requires another domain vault discovered through authorized repository exploration or supplied by the user, follow [cross-vault-consultation.md](cross-vault-consultation.md) before reading its domain content. Return its evidence and limits to this workflow.

## Navigation

For a bounded question about an already identified source, read it directly when no relationship discovery is needed. Preserve vault/repository identity, private-case loading and access checks. Use the graph steps below when locating context or relationships; a direct read does not prove graph completeness.

1. If orientation may be incomplete, run `graph-query.py orientation` (or the equivalent `instance.orientation_status`). If `ready` is false, load [orientation.md](orientation.md) and stop. Do not read Home, Convenciones, or Framework as a prelude to classifying a dependency question.
2. First hop — portable graph query (Obsidian app not required; do not use `obsidian-cli` for this):

```text
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" neighbors --node "<stem>"
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" hygiene
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" investigations --node "<stem>"
```

   Use `neighbors` for dependency, topic, flow, and impact. Use `hygiene` for unresolved links and orphans. Use `investigations` for vault ↔ published `investigations/` join (on-demand published-case scan; not a second index). When consulting a case, use the read-only loading mechanism in `manage-investigation` to discover unpublished or published path, overlay and local working store by ID; preserve private provenance and disclosure restrictions. `manage-investigation` remains owner of the store.
3. Open the stems/paths named in the JSON. If a node or needed relationship is absent, search the named subject in the relevant notes or configured source before concluding that it does not exist. The graph is a navigation index, not proof of completeness. For change impact, query `neighbors` of the changed unit, then `neighbors` of its `publica-en` / `gatillado-por` / `participa-en` targets. Skip types disabled in `instance.yaml` `graph.enabled_types` (do not expand asynchronous messaging when `topic` is disabled). If the question asks where a fact belongs, load [node-selection.md](../../../../90-Meta/node-selection.md) and open the candidate note, not the full Convenciones file.
4. Expand a second named note only when the JSON edge is required by the question:
   - asynchronous messaging: producer → topic → consumer (only when `topic` is enabled).
   - HTTP: caller → consumed repository or integration.
   - Data: `lee-de` and `escribe-en`, plus evidence in the opened note body.
   - Runtime: `usa-infra` and backlinks from the JSON.
   - Service: `compuesto-por`; exceptional component: `implementado-por`.
   - Domain language: follow a glossary edge only when the definition changes the interpretation of a contract, rule, or flow.
   - Operation: follow the selected procedure to its linked standards; use `manage-operational-workflow` only when the user asks to execute or resume it.
5. When the conclusion needs database schema or live evidence, load `inspect-database` with the exact question and any known target. Keep this branch primary.
6. Open a Framework **section pointer** only when a claim needs the evidence hierarchy. For implementation questions outside that database handoff, resolve the target repository by remote identity under the ordered `SOURCE_ROOTS`. Treat every non-managed root as read-only. If the repository is missing or the source context is unusable, follow [vault-resolution.md](../../../../90-Meta/vault-resolution.md): repair configuration through its owner skill and never clone without configured authority in an exact `CLONE_ROOT`.
7. Stop expanding when the next hop cannot change the decision, impact, or uncertainties of the question. Root Bases (`Repos.base`, `Auditoria.base`) are derived radar, not authority.

## Recipes

- **Pending work or coverage status**: use [mapping-completion.md](mapping-completion.md#report-and-stop) to compare current summaries and classify remaining questions without reopening completed extraction.

- **Change impact**: start from the changed unit; inspect input/output contracts, topics, callers/callees, data, runtime, and flows; enumerate every node whose contract or behavior may require a change.
- **Dependencies**: distinguish HTTP from messaging; use backlinks for inverse relationships instead of rebuilding manual lists.
- **Data**: identify the resource, read/write mode, transformation rules, and participants; retain low-level strings when no durable node exists.
- **Infrastructure**: separate versioned deployment evidence from live provider metadata; record the exact target, environment, location, and observation time.
- **Flow or process**: traverse the numbered steps and both diagrams. For each material step, branch and result, verify direction, precondition and effect against participant notes and the bound implementation when exact behavior matters. State whether the answer describes intended design, versioned implementation, configuration or observed execution; a note or graph edge alone does not establish a live outcome. Resolve contradictions with the smallest authorized source check, or identify the affected claim as unresolved. Apply [response quality](../../../../90-Meta/response-quality.md) to the final prose or visual, including new implications introduced by simplification.

## Output

Return the conclusion, decisive evidence and material limits. Include affected nodes and flows for impact questions, and the minimum note/source context package only when implementation context is requested. Scale the answer to the question. Do not turn a glob, filename prefix, or neighbor sample into a vault-wide type inventory; if the question does not need a census, omit it.

## Completion criterion

The question is complete when the inspected sources support the answer and remaining uncertainties are explicit. Stop when another read cannot change the answer; preserve read-only scope and create no persistent case unless continuity was requested. A typed census is a separate exhaustive query, not a by-product of answering one unit.

If this read-only inquiry establishes a material error or omission in a canonical map during an investigation, return the exact discrepancy and evidence to `manage-investigation` and its `references/map-correction.md` route. The query itself grants no write authority; an authorized correction switches explicitly to the ordinary documentation and independent-review recipe without repeating extraction.
