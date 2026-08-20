---
name: map-ecosystem
description: "Trigger: query, document, sync, or check readiness of the cell knowledge vault. First step is orientation when bootstrap is incomplete."
---

# Map the ecosystem

Use the canonical vault as an evidence-backed map. A top-level invocation resolves it before reading relative paths, selects one operating branch, loads only its recipe, and stops when its completion criterion is satisfied. A synchronization package worker takes the bounded fast path below.

## Synchronization package-worker fast path

When the invocation contains a complete `SYNC_PACKAGE_WORKER_V1` card, the parent already owns the synchronization branch and its global evidence. Read only [references/synchronization-package-worker.md](references/synchronization-package-worker.md), validate the card, and skip the top-level Bootstrap and Common contract. Do not select another branch. Complete the assigned extractor or reviewer artifact and return it to the parent. An invalid card returns a bounded contract error to the parent; it never falls back to an independent ecosystem workflow.

## Bootstrap

1. Load [references/vault-resolution.md](references/vault-resolution.md) and run the bundled `scripts/resolve-vault.py` from the installed skill directory. Preserve the complete status and evidence. For a readiness-only request, continue to the readiness branch when resolution is not `resolved`; every other branch requires one remote- and marker-verified `VAULT_ROOT`.
2. When resolution succeeds, bind `SOURCE_CONTEXT`, ordered configuration-backed `SOURCE_ROOTS`, optional managed `CLONE_ROOT`, and optional `OBSIDIAN_VAULT` from the resolver output. Treat every path below as relative to `VAULT_ROOT`, not to the process working directory. When readiness resolution fails, do not run root-dependent checks.
3. Use the explicit Obsidian target and fallback protocol from the resolution reference. This minimal protocol is part of this skill; companion Obsidian skills are optional enhancements.

## Common contract

1. Classify the request **before** opening `90-Meta/` or other vault notes:
   - Where to start, minimal sync, or incomplete bootstrap → **orientation**.
   - Context, query, dependency, or impact → **interrogation**.
   - Create or refresh one durable node or source repository → **single-unit documentation**.
   - Analyze a domain or several related units → **multi-unit documentation**.
   - Detect and propagate ecosystem changes → **synchronization**.
   - Confirm local tooling readiness or diagnose vault/source/Obsidian/GitHub resolution → **operational readiness**.
   - Assess or publish a conclusion derived from an investigation → hand off to `manage-investigation-derived-learning`.
   Complete this step when there is one primary branch and its authorized side effects are explicit. A readiness branch diagnosing failed resolution cannot read root-relative vault files; it must stop root-dependent checks and use only the installed resolution/readiness references to report the blocker.
2. Load only the matching branch reference and execute it through its completion criterion. Orientation and interrogation start with `90-Meta/graph-query.py` JSON, then open the named notes. Do not walk the graph by reading Home, Convenciones, and Framework first. Open `AGENTS.md` only as the cell router when a guardrail is unknown. Open a **section pointer** in `90-Meta/Convenciones.md` or `90-Meta/Auditoria - Framework.md` only when schema, node type, or an evidence gate is required for that claim.
3. Before the selected branch reads or delegates a source repository, follow **Source configuration and selection** in [references/vault-resolution.md](references/vault-resolution.md). Bind the checkout from a successful `locate-repository` response for the expected Git remote. Complete this preflight when every source path used by commands or workers is the returned path for that remote; a failed or ambiguous resolution blocks source access.
4. Before any technical documentation or synchronization write, partition candidate claims into **current productive state** and **future/proposed state**, then apply the production-evidence gate from `90-Meta/Auditoria - Framework.md` claim by claim. Re-inspect authoritative sources behind contextual inputs. Select **no change** for failed claims and keep every future-state claim outside the technical vault. A learning write follows its separate domain skill and gate; adopted technical claims still pass this production gate independently.
5. Load [references/node-selection.md](references/node-selection.md) before any authorized write, or whenever the question depends on where a fact belongs. Use it to select the canonical node type and lifecycle action; treat `90-Meta/Convenciones.md` as the normative catalog, opened by section when needed.
6. Load another branch only when the primary recipe explicitly hands work to it.
7. Report what was inspected, what changed, evidence used, limitations, observed checks, and out-of-scope work. Complete when every affected node has a decision, every written technical claim passed the production-evidence gate, and no unobserved check is claimed.

## Branch pointers

- For a minimal sync or “where do we start?”, read [references/orientation.md](references/orientation.md). Run this before interrogation when bootstrap may be incomplete. Orientation reports identity from `graph-query` / `instance.yaml`; it does not walk repos, topics, or flows.
- For questions, implementation context, dependencies, data/infrastructure, or impact, read [references/interrogation.md](references/interrogation.md). This branch is read-only. First hop is `graph-query`; then open only the named notes.
- When an interrogation depends on a schema repository and `inspect-database` is an enabled adapter, load it as evidence handoff. Keep interrogation primary.
- To create or refresh one unit, read [references/single-unit-documentation.md](references/single-unit-documentation.md).
- For a domain or several related units, read [references/multi-unit-documentation.md](references/multi-unit-documentation.md).
- For inventory, lifecycle, or full updates, read [references/vault-synchronization.md](references/vault-synchronization.md).
- For local tooling readiness or resolution diagnosis, read [references/operational-readiness.md](references/operational-readiness.md).

Supporting reference: read [references/node-selection.md](references/node-selection.md) for node placement, deduplication, propagation, and lifecycle. It supports a primary branch; it is not an operating branch.

When a documentation or synchronization branch analyzes a deployable repository, also read [references/deployment-evidence.md](references/deployment-evidence.md) and resolve the deployment chain for every observed environment before closing that unit.

The operational-readiness branch may also be loaded as a supporting preflight when another branch observes a bootstrap signal that can block its requested capability. In that case, keep the original primary branch and use readiness only to classify the blocker and remediation.

## Authority boundary

Keep resolution, interrogation, and operational readiness read-only. Write during documentation and synchronization only when the user authorizes a vault update and each technical claim passes the production-evidence gate; authorization never substitutes evidence. `manage-investigation-derived-learning` owns assessment and learning-note writes under its separate gate, while this skill supplies node selection, source resolution, and read-only context as requested. A missing local vault requires an explicit path or authorization and destination for a clone. Keep remote source repositories and GCP read-only; commits, pushes, and pull requests require separate authorization.
