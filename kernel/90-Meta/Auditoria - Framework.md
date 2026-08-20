---
tags: [meta]
---

# Documentary audit framework

Authority for evidence, scripts, and gates. Schema lives in [[Convenciones]]. `instance.yaml` selects the evidence profile. `map-ecosystem` maintains the map.

## Obsidian-native principle

The vault is the durable graph. Scripts only read Markdown, frontmatter, and verifiable external evidence. They do not keep a parallel relationship store.

`.sync-acknowledgements.json` is the only versioned procedural exception for synchronization cursors. It is not part of the graph and must not store secrets, findings, or free-form reasons.

## Evidence hierarchy

Use this order until a claim is supported or its absence is bounded:

1. Analyzed repository: code, tests, migrations, schemas, configs; README as a hint only.
2. Versioned deploy: workflows, manifests, Dockerfiles. Resolve trigger, artifact, project, platform, location, and workload per environment; a filename inventory is not completion.
3. Cross-repository search under configured `SOURCE_ROOTS`. Unmanaged roots are read-only.
4. Optional adapter evidence (GCP control-plane, schema repository) when enabled in `instance.yaml`.
5. Vault: audited notes, backlinks, Bases as derived views.

Categories: `verificado-codigo`, `verificado-cross-repo`, `verificado-runtime`, `verificado-vault`, `inferido`, `no-verificado`.

Keep raw logs, productive rows, tickets, and conversations out of the technical sync. After exhausting allowed sources, write “not observed in reviewed static sources”. Reserve `#por-confirmar` for limits that affect business understanding.

## Evidence profiles (`instance.yaml` `evidence.profile`)

### production-gate

Before creating or changing a technical graph claim, prove:

1. Observed implementation.
2. Productive applicability (deploy chain, productive config, control-plane, or schema authority).

`main`/`master` is the analysis baseline, not proof of deploy. Investigations, Jira, designs, PRs, and undeployed commits do not satisfy this gate. Failure means **no change**.

### documented-source

Accept a versioned source artifact (code, schema, config, or owned document) without a deploy chain. Still require an observed implementation. Future/proposed state stays out of the technical vault.

### mixed

Apply `production-gate` to runtime and integration claims. Apply `documented-source` to glossary, operational drafts, and architecture intent that does not assert production behavior.

## Durable-learning gate

A conclusion is `extractable` only when all applicable conditions pass: reusable identity, durable revisitable evidence, verifiable comparison, sustained conclusion, explicit limits, dedupe/accumulation, implementation when adopted, and safe publication. Results: `extractable`, `no-learning`, `already-covered`, `insufficient-evidence`. The last three write nothing.

Local stores (`.investigations/`, `.operations/`, `.knowledge-os-handoffs/`, `plan/`) cannot occupy durable sources.

## Operational evidence

`60-Operacion/` notes stay `borrador` until destinations, owners, and rules are checked against the relevant authority. Tickets and messages prove a run, not the versioned procedure.

## Scripts

Read-only derivatives of Markdown. They never become a second source of truth.

| Script | Purpose |
|---|---|
| `audit-vault.py` | Closed contracts for indexes, systems, repos, architecture, topics, integrations, flows, glossary, operation, learning |
| `verify-links.py` | Broken wikilinks, alias targets, hidden-path links, unexpected orphans |
| `validate-bases.py` | Root `.base` YAML structure |
| `workspace-config.py` | Local discovery roots and optional worktree/proxy preferences |
| `vault-inventory.py` | Freshness/lifecycle using `instance.yaml` prefixes and org |
| `operational-catalog.py` | Resolve operational notes and report-ids |
| `check-obsidian-binding.py` | Exact Obsidian vault name to path |

## Closure gates

After a documentation change, run `audit-vault.py`, `verify-links.py`, and `validate-bases.py` from the vault root. Synchronization also runs `vault-inventory.py` when source access is configured.
