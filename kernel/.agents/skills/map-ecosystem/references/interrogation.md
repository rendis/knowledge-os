# Query the vault

## Input and authority

Accept a question, proposed change, or starting node. Resolve aliases only to find a note; navigate and report using canonical basenames. Keep this branch entirely read-only.

## Navigation

1. If orientation may be incomplete, run `graph-query.py orientation` (or the equivalent `instance.orientation_status`). If `ready` is false, load [orientation.md](orientation.md) and stop. Do not read Home, Convenciones, or Framework as a prelude to classifying a dependency question.
2. First hop — portable graph query (Obsidian app not required; do not use `obsidian-cli` for this):

```text
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" neighbors --node "<stem>"
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" hygiene
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" investigations --node "<stem>"
```

   Use `neighbors` for dependency, topic, flow, and impact. Use `hygiene` for unresolved links and orphans. Use `investigations` for vault ↔ `investigations/` join (on-demand public-case scan; not a second index). `manage-investigation` remains owner of the store.
3. Open the stems/paths named in the JSON. If a node or needed relationship is absent, search the named subject in the relevant notes or configured source before concluding that it does not exist. The graph is a navigation index, not proof of completeness. For change impact, query `neighbors` of the changed unit, then `neighbors` of its `publica-en` / `gatillado-por` / `participa-en` targets. Skip types disabled in `instance.yaml` `graph.enabled_types` (do not expand Pub/Sub when `topic` is disabled). If the question asks where a fact belongs, load [node-selection.md](../../../../90-Meta/node-selection.md) and open the candidate note, not the full Convenciones file.
4. Expand a second named note only when the JSON edge is required by the question:
   - Pub/Sub: producer → topic → consumer (only when `topic` is enabled).
   - HTTP: caller → consumed repository or integration.
   - Data: `lee-de` and `escribe-en`, plus evidence in the opened note body.
   - Runtime: `usa-infra` and backlinks from the JSON.
   - Service: `compuesto-por`; exceptional component: `implementado-por`.
   - Domain language: follow a glossary edge only when the definition changes the interpretation of a contract, rule, or flow.
   - Operation: follow the selected procedure to its linked standards; use `manage-operational-workflow` only when the user asks to execute or resume it.
5. When the conclusion needs database schema or live evidence and `inspect-database` is enabled, load that adapter with the exact question and any known target. Keep this branch primary.
6. Open a Framework **section pointer** only when a claim needs the evidence hierarchy. For implementation questions outside that database handoff, resolve the target repository by remote identity under the ordered `SOURCE_ROOTS`. Treat every non-managed root as read-only. If the repository is missing or the source context is unusable, follow [vault-resolution.md](../../../../90-Meta/vault-resolution.md): repair configuration through its owner skill and never clone without configured authority in an exact `CLONE_ROOT`.
7. Stop expanding when the next hop cannot change the decision, impact, or uncertainties of the question. Root Bases (`Repos.base`, `Auditoria.base`) are derived radar, not authority.

## Recipes

- **Pending work or coverage status**: use [mapping-completion.md](mapping-completion.md#report-and-stop) to compare current summaries and classify remaining questions without reopening completed extraction.

- **Change impact**: start from the changed unit; inspect input/output contracts, topics, callers/callees, data, runtime, and flows; enumerate every node whose contract or behavior may require a change.
- **Dependencies**: distinguish HTTP from messaging; use backlinks for inverse relationships instead of rebuilding manual lists.
- **Data**: identify the resource, read/write mode, transformation rules, and participants; retain low-level strings when no durable node exists.
- **Infrastructure**: separate versioned deployment evidence from GCP metadata; record environment, region, and date when querying GCP.
- **Flow**: traverse the numbered steps and both diagrams; verify that every relevant edge appears in participant notes.

## Output

Return, in this order:

1. Actionable conclusion.
2. Inspected scope and starting node.
3. Affected nodes and flows.
4. Evidence supporting the conclusion.
5. Uncertainties and evidence limitations.
6. Minimum recommended note/source-code context package for implementation.

## Completion criterion

The question is complete when the inspected sources support the answer and remaining uncertainties are explicit. Stop when another read cannot change the answer; preserve read-only scope and create no persistent case unless continuity was requested.
