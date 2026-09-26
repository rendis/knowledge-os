---
tags: [meta]
---

# Vault documentation audit

This framework is the source of truth for the evidence, scripts and gates of the cell's map, its operational documentation domain and its engineering learnings. The schema and relationships live in [[Convenciones]]; `map-ecosystem` maintains the map, `manage-operational-workflow` runs procedures with external effects, `generate-reports` produces registered reports, `manage-development-handoff` prepares atomic tasks per repository and reconciles their progress into the investigation, and `manage-investigation-derived-learning` maintains cumulative teachings.

## The vault is the graph

The vault is the durable graph. The CLI derives relationships from Markdown/frontmatter and inspected external evidence; it keeps no parallel authority. A confirmed relationship is written in the corresponding note.

`.sync-acknowledgements.json` is the only versioned synchronization process record and stays outside the graph. It records an exact branch and commit only as `no-durable-node` (accepted new repository without a node) or `no-documentation-change` (accepted existing-repository delta requiring no documentation update). A rejected or limited attempt creates no acknowledgement and advances no successful cursor. Acknowledgements preserve the note baseline and contain no technical facts, relationships, findings, free-form reasons, or secrets.

## Evidence hierarchy

Use this order until the claim is supported or the absence is bounded:

1. The analyzed repository: code, tests, migrations, schemas and configuration. A README and other documentation add description when consistent with the implementation; titles or empty templates do not prove behavior.
2. Versioned deployment definitions: build, release, provisioning and runtime configuration. Inspect the definitions needed to resolve connectors, triggers and configuration that changes the mapped flow. A complete per-environment deployment chain belongs to an explicitly requested deployment audit; unresolved indirection does not block independent local-map claims.
3. Cross-repository search in the ordered `SOURCE_ROOTS` of the `source_context` derived from local configuration: module, repository, topic, subscription, environment variable, endpoint, data, scheduler and service name. Unmanaged roots are read-only.
4. Read-only control-plane metadata through the configured provider access procedure, limited to the exact target and metadata needed for the claim.
5. The vault: audited notes, backlinks and Bases as derived views.

Evidence categories: `verificado-codigo`, `verificado-cross-repo`, `verificado-runtime`, `verificado-vault`, `inferido` and `no-verificado`. Runtime evidence records the observed provider, target, executor and time. Existing evidence labels retain their original meaning; changing terminology does not establish a new observation.

Outside the technical sync: logs, runtime or manual tests, production data, trackers, external documentation and conversations with owners. A configured runtime procedure may query bounded state and logs as ephemeral evidence for an operational investigation; that does not make them durable facts or update the graph. An investigation may query production data only through the explicit read-only exception of `inspect-database`; that result is evidence for the investigation, not an automatic source for the sync. When the bounded scope or an access limit is reached, write the absence, in the note's locale, as not observed in the reviewed static sources; keep `#por-confirmar` for limits that affect business understanding.

## Production reality gate

Applicability: this section governs `production-gate` and the technical half of `mixed`. For `documented-source`, use [[evidence-policy]]: inspected versioned sources suffice for source-level assertions; actual deployment claims still require deployment evidence.

Source repository maps follow the source-map rule in [[evidence-policy]]: code and configuration at the exact repository reference-branch commit can describe purpose, main flows and connectors without asserting deployment. This exception does not promote future proposals or prove productive execution. For production assertions and investigation promotion under the production profiles, establish both conditions:

1. **Observed implementation**: code, schema or migration, configuration, contract or resource that materializes the behavior.
2. **Production applicability**: evidence that ties that implementation to production according to its nature, such as the deployment chain per environment, versioned production configuration, control-plane metadata, an identified runtime or a schema state validated through the corresponding database authority.

The repository baseline follows [[reference-branches]]; its branch name alone does not prove deployment. Neither does an investigation, a decision, a work item, a conversation, an approved design, an isolated test, a pull request or a commit without a production link satisfy the gate. For contracts between units, the evidence must support the participants and the production link being documented, not only one of its ends.

Investigations are classified as knowledge-oriented, development-oriented or mixed. They may record evidence, decisions and documentation candidates, but `map-ecosystem` must inspect the authoritative sources again after deployment. In a mixed investigation, only the current state that passes this gate can be a candidate; the future state stays out of the vault.

If either condition fails, the documentation decision is **no change**. `#por-confirmar` expresses a limitation about something already observed in production; it does not keep proposals, future designs or pending implementations.

For a configuration assertion, versioned production configuration satisfies applicability at the analyzed commit: describe what it configures and qualify unresolved deployment separately. Claims about effective runtime, deployed destinations, or executed behavior require evidence for those stronger assertions. An inaccessible deployment definition limits only dependent claims; it does not suppress independently observed configuration changes.

## Durable learning gate

This gate decides whether an investigation contributes a versionable teaching to `70-Aprendizajes/`. Its assessment is always read-only and does not presume that knowledge exists. The investigation points to sources, stories, tests, implementation and deployment, but its narrative is not proof on its own.

A conclusion is `extractable` only when it meets every applicable condition:

1. **Reusable identity**: it states a stable question or problem and a material context of applicability; it is not a log, a session summary, an obvious fact or a detail exclusive to one case.
2. **Durable, revisitable evidence**: every decisive claim traces to inspected sources another authorized agent can revisit, or to a method whose procedure, inputs and criteria are kept in a durable or versioned source or can be rerun from versioned tooling. A versioned case under `investigations/` keeps provenance and decisions, but its narrative does not prove production behavior. Memory and transient local artifacts are only leads; the same limit applies to `.investigations/`, `.investigations-private/`, `.operations/`, worktree `.handoff/` stores and `.plan/`.
3. **Verifiable comparison**: it identifies the baseline, alternatives, environment, scale or sample/window, controls, observable metrics or criteria and exclusions able to change the result. It keeps positive, negative and neutral results.
4. **Supported conclusion**: the decision and its justification follow from the results without hiding contradictions or generalizing beyond what was measured.
5. **Explicit limits**: it states when it applies, when it does not, accepted limitations and the events that require revalidation.
6. **Deduplication and accumulation**: it looks for existing knowledge by question and context and selects exactly one action: `create`, `enrich`, `challenge` or `supersede`. A new investigation or date does not justify another note.
7. **Implementation when it applies**: `cambio-adoptado` identifies the exact implementation and also passes the [[#Production reality gate|production reality gate]]. `baseline-conservado`, `alternativa-descartada` and `hallazgo-metodologico` do not require a new deployment, but do require the authoritative baseline and the durable comparison.
8. **Safe publication**: it keeps no secrets, direct personal data, production rows, raw logs or sensitive attachments; it summarizes only what is needed and links safe references.

Select the assessment outcome and lifecycle action through the `manage-investigation-derived-learning/references/assessment-contract.md`, **Assessment outcomes**, under `.agents/skills/`. That contract owns the effect of unresolved case questions, valid no-write outcomes, deduplication and challenge eligibility. Apply the evidence conditions above to the selected conclusion; publication remains subject to the user's authorization.

## Operational evidence

Notes under `60-Operacion/` are verified against the relevant authority: read-only state of the configured platform, current corporate documentation or explicit confirmation from the responsible owner. Record a real owner and update `ultima-verificacion` only after that check. While projects, dashboards, recipients, channels, permissions or corporate rules are missing, keep the note as `borrador` and describe the limitation without inventing destinations.

For `clase: reporte`, use the `manage-operational-workflow/references/procedure-contract.md`, **Report**, under `.agents/skills/`. It defines mode-specific completeness without imposing an executor or output format. Before executing any procedure, apply the same contract's **Execution eligibility** section.

Tickets, emails and messages prove the result of a run, but do not replace the versioned rule. `.operations/` is resumable local state, outside the graph and the documentation gates, and must hold no secrets or sensitive bodies.

## Development reconciliation evidence

`.handoff/deltas.md` keeps only how the task definition changed or was complemented: kind, detail and evidence. The repository records those deltas there and progress lives in the branch (commits, tests, pull request); it does not write to the vault. `manage-development-handoff` reads the worktree with `handoff status` (state, commits by `Handoff:` trailer and deltas of each task), and `handoff reconcile` imports into the investigation, with a fixed mapping, the commits and deltas after the last reconciliation mark of the `DH-NNN` record; a delta without evidence becomes a question. A material change without a delta blocks reconciliation; an empty deltas file never proves on its own that the scope did not change.

Reconciliation follows at most one hop of typed relations, when the tracker supports them, to identify directly dependent work items, and delivers the contract each one needs, its readiness and the exact repository, branch and pull request state. Local branch, remote ref, pull request, merge and deployment are separate states. All this material remains future or undeployed context: updating the case does not satisfy the production reality gate or authorize a technical write to the vault.

## Operational views

- [[Repos.base]]: repository index and freshness.
- [[Arquitectura.base]]: services, exceptional components and runtime.
- [[Auditoria.base]]: repository coverage and relationships.
- [[Operacion.base]]: operations by area, class, state and reports.
- [[Aprendizajes]]: curated index of reusable conclusions and their states.

## Native kernel operations

The shared contracts [[vault-resolution]], [[node-selection]] and [[work-item-evidence]] define vault identity, node selection and bounded work-item evidence. Provider-specific access remains configured through procedures in the destination vault.

Select `<cli>` through `use-vault-cli` (`.agents/skills/use-vault-cli/SKILL.md`); it defines the executable path and shell invocation. Resolve the vault before using relative paths. Command help supplies argument details; the owning skill supplies the investigation and authorization workflow.

| Operation | When and output | Boundary |
|---|---|---|
| `<cli> config resolve --vault "<vault_root>"` | Resolve canonical identity and configured sources before vault-dependent work. | Read-only; configuration changes belong to `onboard-developer`. |
| `<cli> config status --vault "<vault_root>"` | Validate instance identity and inspect configured capabilities. | Missing capabilities remain explicit; no procedure is executed. |
| `<cli> config workspace --vault "<vault_root>"` | Read local repository, worktree and proxy configuration. | Local settings preserve consumer namespaces; credentials and remote target identities stay outside this file. |
| `<cli> config areas --vault "<vault_root>"`, `config reports`, `config operation` | Discover areas/reports and resolve a procedure by basename or report ID. | Derived catalog; executing a procedure requires its skill and authority. |
| `<cli> audit --vault "<vault_root>"` | Check closed note schemas, sections, learning evidence structure, operational topology and process leakage. | Structure does not prove business truth, experimental quality or external state. |
| `<cli> check links --vault "<vault_root>"` | Find broken links, alias targets, hidden-agent links and unexpected orphans. | Filesystem fallback does not replicate all Obsidian parsing. |
| `<cli> check bases --vault "<vault_root>"` | Validate Bases after changing a Base, schema or property. | Does not execute expressions or plugin-defined functions. |
| `<cli> inventory --vault "<vault_root>" [--github-user <login>]` | Compare repository freshness and synchronization cursors before and after sync. | Requires authenticated `gh` and network; cursors are process state, not technical evidence. |
| `<cli> discover run`, `discover check --note <note>` | Extract connection facts at an exact commit; gate a note: anchors resolve, identifiers are in the cited lines, every discovered connector and resource is addressed. | Facts are evidence pointers; the checks prove anchors and coverage, not the correctness of the prose. |
| `<cli> sync start`, `review`, `verify`, `finish` | Publish knowledge on a `sync/` branch: review bound to the exact content by digest, gates versus the base, fast-forward merge. | A recorded review is the reviewer's verdict, not proof; remote publication follows the Git policy. |
| `<cli> investigation new\|list\|check\|add\|state\|absorb\|close\|reopen --vault "<vault_root>"` | Every case change: open from the formalized request; record evidence (source and level, files attached to the record), conclusions (level and basis, marked for the vault), questions, decisions, requirements and handoffs with assigned IDs and a log. Each write is refused when it would introduce a gate error (unsourced evidence, undefined reference, broken link, copied vault text, credential, local path). | `sync verify` runs `check` on changed cases (introduced errors block). The CLI does not judge whether evidence is sufficient or the language neutral; the independent review does. |
| `<cli> handoff start\|status\|refresh\|reconcile --vault "<vault_root>"` | Prepare a worktree for task packages (tasks of one branch share it, dependencies start first), read per-task state, commits and deltas, refresh a changed task, and import commits and deltas since the last mark into the case with a fixed mapping (preview, then `--apply`). | Never commits, pushes or fetches; the managed segment in `AGENTS.md` is identical in every repository. |

For deployment/configuration evidence, inspect the exact source files and relevant cross-repository references using the source context. Search hits identify inspection targets; they do not establish a deployment chain or verified dependency.

## GitHub identity resolution

`kos inventory` does not assume that the globally active `gh` account belongs to the current repository. It resolves one identity per run, validates access to the organization and uses the same ephemeral token for the inventory, branches and resolution of missing repositories. It never runs `gh auth switch`, persists users or tokens, or prints secrets.

The precedence is:

1. `--github-user <login>`: explicit selection of an account already stored by `gh`; it wins over environment tokens.
2. `GH_TOKEN` or `GITHUB_TOKEN`: non-interactive identity for CI or automation. If it cannot access the organization, the command fails without trying personal accounts.
3. The active `gh` account, only when the access check succeeds.
4. The single inactive authenticated account that does access the organization.

If several inactive accounts have access, the command exits with code 2 and lists only their logins so the user repeats with `--github-user`. If none works, it says to authenticate `gh` and check permissions or SSO authorization. If `gh` cannot validate the accounts because of connectivity or another operational error, it reports that separately and does not present it as missing authentication. Reference branches follow [[reference-branches]]. Their commit SHAs are read through paginated GraphQL with that same identity, independently of the Git credential helper.

## Gates

Run the installed binary against the resolved absolute vault root. Consumer closure uses installed operations; distribution unit tests and Python evaluation tooling run only in the distribution checkout. A normal sync runs the final inventory, structural audit and link check. Run Bases validation when a Base, schema or property changed.

```text
<cli> config status --vault "<vault_root>"
<cli> config workspace --vault "<vault_root>"
<cli> audit --vault "<vault_root>"
<cli> check links --vault "<vault_root>"
<cli> check bases --vault "<vault_root>"
```

After creating or materially revising a complex skill, also run a blind test in a fresh agent context: give only the skill's path and a realistic request against temporary fixtures, without the expected answer, prior diagnosis or the author's conclusions. A skill that writes (cases, handoffs, notes) is tested on temporary copies, with hashes or state before and after, never touching real vaults or repositories. If the environment offers no agent isolation, record this validation as not observed; do not simulate it in the same context.

Consumer validation requires the installed native binary. Development dependencies, compiler checks and regression suites belong to the distribution checkout, never to the cell. Review changed diagrams and notes in Obsidian or the authorized browser when available, and report any unobserved rendering check.

The common allowlist of expected orphans is `00-Home.md` and `README.md`: they are the vault's entry points, not failures. `AGENTS.md` and any cell-owned `CLAUDE.md` stay outside the documentation graph. Scripts, templates and meta documents must be linked from this framework or from another index.

Published cases under `investigations/`, unpublished `.investigations/`, their private directory `.investigations-private/`, `.operations/` runs and handoffs (`.handoff/` in each worktree) stay outside the technical graph and its gates. Versioning a case does not make it a parallel source of technical truth.

## Closure criterion

A mapping pass or synchronization ends when:

- every unit and inventory item has a supported decision;
- every created or modified technical claim passes the production reality gate and no future state was published;
- every created or modified learning passes its independent gate and keeps cumulative evidence and applicability, and every non-extractable assessment ended without a write;
- the notes follow [[Convenciones]] and separate facts from limits;
- affected topics, integrations, architecture, flows and MOCs were propagated;
- when the operational domain changes, its index, MOCs, Base, policies, guides, catalogs, standards, procedures, reports, resolver and skill routing were propagated;
- when the learning domain changes, its index, contract, routing, related notes and validator were propagated;
- each published documentation update records its analyzed production branch and commit; accepted no-change acknowledgements and unaccepted reviews preserve the note baseline;
- each accepted repository without a published documentation change has an operational cursor matching its branch and SHA; `no-durable-node` follows an accepted review and cannot coexist with a note; `no-documentation-change` follows an accepted existing-repository review and requires a note; rejected or limited attempts leave the prior successful cursor unchanged;
- each updated deployable repository note preserves one row per environment/deployable with the deployment chain resolved or explicitly limited;
- the gates relevant to the modified files and contracts pass; a sync does not turn the full CI suite into analysis steps;
- a sync uses one source analysis to produce complete final-note candidates and one independent semantic review per repository/OID; candidate repairs remain local, materially changed meaning receives targeted re-review, and mechanical checks never reopen extraction; accepted identical baselines receive the appropriate acknowledgement while rejected or limited attempts remain unresolved without advancing a cursor; publication is resumable and writes only exact reviewed bytes;
- the final report separates reviewed, changed, no durable contribution, evidence limits and the run's real pending items, without assigning remediations to the source repositories.
