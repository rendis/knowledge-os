# Resolve and interact with the canonical vault

Load this reference as bootstrap context, then resolve the vault before reading domain notes or operating on the cell map. Router, personal instructions and skill/access references may be read to perform this bootstrap. The skill may run from the vault, a source repository, or an unrelated working directory.

## Contents

- [Canonical identity](#canonical-identity)
- [Resolution](#resolution)
- [Source configuration and selection](#source-configuration-and-selection)
- [Development destinations](#development-destinations)
- [Managed clone authorization](#managed-clone-authorization)
- [Interaction protocol](#interaction-protocol)
- [Filesystem interaction](#filesystem-interaction)
- [Completion criterion](#completion-criterion)

## Canonical identity

Identity comes from the cell vault's `instance.yaml`:

- Required markers: `AGENTS.md`, `00-Home.md`, `instance.yaml`, `90-Meta/Convenciones.md`, and `90-Meta/Auditoria - Framework.md`.
- If `vault.remote` is set, the Git origin must match it.
- If `vault.remote` is empty, markers plus a valid `instance.yaml` identify the vault (typical right after init).

Never identify the vault by directory basename alone. The local folder and Obsidian name may differ between machines.

## Resolution

Select `<VAULTCTL>` through `use-vault-cli` (the executable-selection section in `.agents/skills/use-vault-cli/SKILL.md`) before resolution; it defines platform selection and shell invocation. For a skill installed at `<vault>/.agents/skills/<skill>`, its `../../..` directory supplies a candidate root; validate it before binding identity. A source checkout is not automatically the documentation vault.

```text
<VAULTCTL> config resolve --vault "<EXPLICIT_PATH>"
```

Supply a user-selected vault path, or the root derived from the active installed skill. The resolver checks that path and its ancestors for `instance.yaml`, validates required markers and the declared Git origin, and returns one `resolved` object. It does not search sibling repositories or select a vault from recency. If the explicit path cannot resolve, request the exact local vault path or authorization and destination to clone its canonical remote; continue independent work that needs no vault binding.

Bind `VAULT_ROOT` only from the returned `vault_root`. Its `source_context` contains ordered configured `roots`, `clone_root`, `clone_origin`, `clone_authorized` and warnings. An unavailable source context does not block vault-only work.

The resolver makes a bounded optional `obsidian vaults verbose` query. `obsidian_vault` is set only for a unique registration matching the canonical filesystem path; `interaction_mode` is then `obsidian-cli`. Missing CLI, unavailable application or ambiguous registration leaves filesystem mode available. This probe does not launch Obsidian or select another vault.

Bind `SOURCE_ROOTS` to `source_context.roots`, `CLONE_ROOT` to the configured clone root and `OBSIDIAN_VAULT` only to the verified registration name. Resolve again after a new user-supplied path, configuration change, clone or registration change. Re-establish machine-local bindings in a new session.

## Source configuration and selection

This section binds repositories consulted as evidence sources. For an explicitly selected implementation checkout, use Development destinations below instead; it does not require discovery configuration.

`VAULT_ROOT/.knowledge-os-config.yaml` is the single local source of reusable repository roots and clone authority. The resolver does not interpret or repair it; it uses the native configuration API and forwards its `source_context`. Environment variables, conventional sibling discovery, and resolver-level source-root overrides are not valid configuration sources.

`SOURCE_ROOTS` are discovery inputs; a successful semantic response is the only checkout binding. Before reading a source repository or assigning it to a worker, obtain its expected Git remote and run `<VAULTCTL> config locate --vault "<VAULT_ROOT>" --remote "<GIT_REMOTE>"`. Use only the `path` returned by that command, and repeat the resolution for every required repository. A configured discovery root represents itself when it is a Git repository; otherwise only its immediate child directories are candidates. The semantic API normalizes SSH and HTTPS remotes, performs no recursive scan, and rejects zero or multiple matches without writing configuration.

A failed checkout binding keeps source reads blocked; a vault-only task may continue with that limitation. Recover through these supported paths:

- **Unavailable/invalid configuration or ambiguous matches:** load `configure-workspace`. Ask which repository root/checkout should be configured and for authorization to make the required configuration change. If configuration changes are forbidden, state that the source question cannot be completed under that constraint and ask whether the user wants to authorize that bounded configuration repair. Resume source reading only after the repaired configuration returns one successful checkout binding.
- **`not_found` under valid roots:** request an existing matching checkout within those roots, or authorization to configure another root. Repeat checkout resolution before reading.

The recovery question requests configuration or a matching checkout, never permission to skip identity resolution. Confirming an ambiguous path without resolving its configuration does not complete recovery. `configure-workspace` remains the sole configuration writer.

The order of `SOURCE_ROOTS` is the configured order and every record has `origin: config`. A root whose exact resolved path is also `CLONE_ROOT` is marked `managed: true`; parent/child overlap does not transfer authority. Duplicate or inconsistent candidates require the configuration workflow and an explicit user choice; consumers must not invent a preference.

In the evidence-source role, repositories under non-managed roots are read-only: do not fetch, checkout, reset, merge, or write in them. A root with `managed: true` may clone or fetch according to the managed-clone protocol below, but its existing working trees remain read-only. Database schema repositories follow the same selection rules as application repositories.

## Development destinations

A checkout selected for an explicitly requested implementation is a separate task role from an evidence source. Bind an exact user-supplied path or an already established task destination with Git: verify its root, expected remote when supplied or registered, current branch and working changes. A repository with no remote may be used when explicitly selected as local-only. A registered handoff also requires its exact branch and handoff identity. This path need not be added to discovery configuration.

A successful source lookup alone grants no edit authority. If the request identifies a repository but leaves multiple possible implementation checkouts, resolve that choice before writing. An explicit instruction to edit the exact existing source checkout authorizes that scoped implementation without changing reusable clone authority or allowing unrelated source writes. Keep other repositories in their evidence-source role.

Use work-in-repository for repository context and implementation, manage-git-workflow for Git effects, and manage-development-handoff for persistent work-item worktrees and handoff state. Existing task authority carries forward; configuration changes, acquisition and publication retain their own scopes.

## Managed clone authorization

A valid configured `CLONE_ROOT` authorizes shallow clones and `git fetch` only inside that exact resolved directory, which must also be one of `SOURCE_ROOTS`. Authority applies only to in-scope source repositories and never to `VAULT_ROOT`, even when the vault is a child of `CLONE_ROOT`. It also authorizes the detached temporary-worktree protocol below: Git may register/remove `.git/worktrees` metadata in the managed source clone and materialize the selected commit only under a host temporary directory. It never authorizes remote writes, branch changes, resets, merges, source edits, or changes to an existing checkout. `CLONE_ROOT` must not equal or descend from `VAULT_ROOT`; an invalid value makes workspace configuration inconsistent and must be repaired through its owner skill.

When a managed clone is missing, determine the production branch using the inventory rule, then create its shallow clone under `CLONE_ROOT`. When it exists and current remote evidence is required, fetch the required remote refs without changing any existing checkout. Materialize the selected remote commit in a detached temporary worktree created with the host's safe temporary-directory API (`mktemp -d` on POSIX or the platform equivalent), pass that exact worktree through `--source-repo`, and remove that exact worktree in guaranteed cleanup on success or failure. If interrupted cleanup leaves metadata, report the exact path and retry only `git worktree remove --force <exact-temporary-path>` against that managed clone. Never use broad `git worktree prune`: it may remove unrelated stale registrations.

If `clone_authorized=false`, never clone. Ask for explicit approval and an exact existing root, then hand off to `configure-workspace` to record managed-clone authority before acquisition. If the user does not want persistent clone authority, require them to provide an already available checkout under a configured read-only root.

## Interaction protocol

When `OBSIDIAN_VAULT` is available, verify the normalized filesystem binding before the first query. Run the versioned helper from `VAULT_ROOT`; do not trust the Obsidian exit code alone because a missing vault can still return `0`:

```text
<VAULTCTL> check obsidian-binding --vault "<VAULT_ROOT>" --vault-name "<OBSIDIAN_VAULT>"
```

Then always target the vault explicitly:

```text
obsidian "vault=<obsidian_vault>" search "query=<term>"
obsidian "vault=<obsidian_vault>" read "file=<canonical basename>"
obsidian "vault=<obsidian_vault>" backlinks "file=<canonical basename>"
obsidian "vault=<obsidian_vault>" unresolved
obsidian "vault=<obsidian_vault>" orphans
```

Use the native CLI for bounded indexed retrieval: `<VAULTCTL> search --vault "<VAULT_ROOT>" --query "<terms>"`. Read the returned source before using it as evidence. Obsidian CLI can additionally inspect canonical resolution, backlinks, unresolved links and orphans. Use the host's safe filesystem editing mechanism for versioned Markdown and skill files so Git can review the exact diff. Pass the resolved `VAULT_ROOT` explicitly to CLI operations.

Never issue an Obsidian command without the explicit `"vault=<obsidian_vault>"` argument; the implicit target is the most recently focused vault and is not safe evidence.

## Filesystem interaction

If the resolver finds the vault but no matching Obsidian registration or CLI is available:

1. Use native CLI search and targeted filesystem reads; `rg` remains a direct text-search fallback.
2. Use `<VAULTCTL> check links --vault "<VAULT_ROOT>"` for filesystem link checks.
3. Run all other Framework gates that are available.
4. Report that Obsidian-native resolution, backlinks, or rendering were not verified.

The companion `obsidian-cli`, `obsidian-markdown`, and `obsidian-bases` skills improve tool-specific operation when installed, but vault identity, routing, and evidence boundaries remain valid in either interaction mode.

## Completion criterion

Resolution is complete only when the canonical remote and markers identify one `VAULT_ROOT`, every path-dependent command is rooted there, any required source access has a usable configuration-backed `source_context`, and every Obsidian command names the verified `OBSIDIAN_VAULT` explicitly.
