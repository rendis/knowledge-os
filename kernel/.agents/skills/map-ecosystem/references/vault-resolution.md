# Resolve and interact with the vault

Load this reference before reading any repository-relative instruction or operating on the cell map. The skill may run from the vault, a source repository, or an unrelated working directory.

## Contents

- [Canonical identity](#canonical-identity)
- [Resolution](#resolution)
- [Source configuration and selection](#source-configuration-and-selection)
- [Managed clone authorization](#managed-clone-authorization)
- [Interaction protocol](#interaction-protocol)
- [Filesystem fallback](#filesystem-fallback)
- [Completion criterion](#completion-criterion)

## Canonical identity

Identity comes from the cell vault's `instance.yaml`:

- Required markers: `AGENTS.md`, `00-Home.md`, `instance.yaml`, `90-Meta/Convenciones.md`, and `90-Meta/Auditoria - Framework.md`.
- If `vault.remote` is set, the Git origin must match it.
- If `vault.remote` is empty, markers plus a valid `instance.yaml` identify the vault (typical right after init).

Never identify the vault by directory basename alone. The local folder and Obsidian name may differ between machines.

## Resolution

Run the bundled resolver from the installed skill directory. Use `python3` on macOS/Linux, `py -3` or `python` on native Windows, and quote paths on every platform:

```text
<python> -B "<installed-skill-dir>/scripts/resolve-vault.py"
```

If the user supplied a path, pass it explicitly:

```text
<python> -B "<installed-skill-dir>/scripts/resolve-vault.py" --path "<path>"
```

The resolver is read-only and applies this order:

1. Explicit path or one of its ancestors.
2. Current directory or one of its ancestors.
3. Paths returned by `obsidian vaults verbose`.
4. Canonical remote and marker validation for every candidate.

Interpret the JSON status:

- `resolved`: bind `vault_root`, `source_context`, and optional `obsidian_vault`; continue. `source_context.status=unavailable` does not block a vault-only question.
- `invalid`: the explicit path is not the canonical vault; stop and report it.
- `ambiguous`: present the verified candidates and request a choice; never choose by recency or basename.
- `not_found`: request an explicit local path or authorization and destination to clone the canonical remote. Do not invent a persistent clone location.

Treat the returned paths as task-local bindings:

- `VAULT_ROOT` = `vault_root`, the documentation repository.
- `SOURCE_CONTEXT` = `source_context`, with `status`, ordered `roots`, optional `clone_root`, `clone_origin` (`config` or `null`), `clone_authorized`, and `warnings`.
- `SOURCE_ROOTS` = the ordered `source_context.roots`; each item declares `path`, `origin`, and whether it is `managed`.
- `CLONE_ROOT` = `source_context.clone_root`, only when an existing managed directory was explicitly configured.
- `OBSIDIAN_VAULT` = `obsidian_vault`, only when that exact path is registered in Obsidian.

Resolve again after a new user-supplied vault path, configuration change, clone, or Obsidian registration change. Do not carry a path from another machine or session as fact.

## Source configuration and selection

`VAULT_ROOT/.knowledge-os-config.yaml` is the single local source of reusable repository roots and clone authority. The resolver does not interpret or repair it; it calls the versioned semantic API and forwards its `source_context`. Environment variables, conventional sibling discovery, and resolver-level source-root overrides are not configuration fallbacks.

`SOURCE_ROOTS` are discovery inputs; a successful semantic response is the only checkout binding. Before reading a source repository or assigning it to a worker, obtain its expected Git remote and run `python3 -B 90-Meta/workspace-config.py --vault-root "<VAULT_ROOT>" locate-repository "<GIT_REMOTE>" --format json`. Use only the `path` returned by that command, and repeat the resolution for every required repository. A configured discovery root represents itself when it is a Git repository; otherwise only its immediate child directories are candidates. The semantic API normalizes SSH and HTTPS remotes, performs no recursive scan, and rejects zero or multiple matches without writing configuration.

When `source_context.status` is `unavailable` or `invalid`, a vault-only task may continue with that limitation. A task that needs source evidence must load `configure-workspace`, complete onboarding or repair, rerun the resolver, and then resume. No consumer may write the YAML directly.

The order of `SOURCE_ROOTS` is the configured order and every record has `origin: config`. A root whose exact resolved path is also `CLONE_ROOT` is marked `managed: true`; parent/child overlap does not transfer authority. Duplicate or inconsistent candidates require the configuration workflow and an explicit user choice; consumers must not invent a preference.

Repositories under non-managed roots are read-only: do not fetch, checkout, reset, merge, or write in them. A root with `managed: true` may clone or fetch according to the managed-clone protocol below, but its existing working trees remain read-only. Database schema repositories follow the same selection rules as application repositories.

## Managed clone authorization

A valid configured `CLONE_ROOT` authorizes shallow clones and `git fetch` only inside that exact resolved directory, which must also be one of `SOURCE_ROOTS`. Authority applies only to in-scope source repositories and never to `VAULT_ROOT`, even when the vault is a child of `CLONE_ROOT`. It also authorizes the detached temporary-worktree protocol below: Git may register/remove `.git/worktrees` metadata in the managed source clone and materialize the selected commit only under a host temporary directory. It never authorizes remote writes, branch changes, resets, merges, source edits, or changes to an existing checkout. `CLONE_ROOT` must not equal or descend from `VAULT_ROOT`; an invalid value makes workspace configuration inconsistent and must be repaired through its owner skill.

When a managed clone is missing, determine the production branch using the inventory rule, then create its shallow clone under `CLONE_ROOT`. When it exists and current remote evidence is required, fetch the required remote refs without changing any existing checkout. Materialize the selected remote commit in a detached temporary worktree created with the host's safe temporary-directory API (`mktemp -d` on POSIX or the platform equivalent), pass that exact worktree through `--source-repo`, and remove that exact worktree in guaranteed cleanup on success or failure. If interrupted cleanup leaves metadata, report the exact path and retry only `git worktree remove --force <exact-temporary-path>` against that managed clone. Never use broad `git worktree prune`: it may remove unrelated stale registrations.

If `clone_authorized=false`, never clone. Ask for explicit approval and an exact existing root, then hand off to `configure-workspace` to record managed-clone authority before acquisition. If the user does not want persistent clone authority, require them to provide an already available checkout under a configured read-only root.

## Interaction protocol

When `OBSIDIAN_VAULT` is available, verify the normalized filesystem binding before the first query. Run the versioned helper from `VAULT_ROOT`; do not trust the Obsidian exit code alone because a missing vault can still return `0`:

```text
<python> -B 90-Meta/check-obsidian-binding.py --vault-root "<vault_root>" --vault-name "<obsidian_vault>"
```

Then always target the vault explicitly:

```text
obsidian "vault=<obsidian_vault>" search "query=<term>"
obsidian "vault=<obsidian_vault>" read "file=<canonical basename>"
obsidian "vault=<obsidian_vault>" backlinks "file=<canonical basename>"
obsidian "vault=<obsidian_vault>" unresolved
obsidian "vault=<obsidian_vault>" orphans
```

Use Obsidian CLI for search, canonical resolution, backlinks, unresolved links, and orphan checks. Use the host's safe filesystem editing mechanism for versioned Markdown and skill files so Git can review the exact diff. Run repository scripts from `VAULT_ROOT`.

Never issue an Obsidian command without the explicit `"vault=<obsidian_vault>"` argument; the implicit target is the most recently focused vault and is not safe evidence.

## Filesystem fallback

If the resolver finds the vault but no matching Obsidian registration or CLI is available:

1. Use targeted filesystem reads and `rg` for discovery.
2. Use `<python> -B 90-Meta/verify-links.py` from `VAULT_ROOT` for links and orphan fallback.
3. Run all other Framework gates that are available.
4. Report that Obsidian-native resolution, backlinks, or rendering were not verified.

The companion `obsidian-cli`, `obsidian-markdown`, and `obsidian-bases` skills improve tool-specific operation when installed, but this skill does not depend on them for vault identity, routing, evidence boundaries, or fallback behavior.

## Completion criterion

Resolution is complete only when the canonical remote and markers identify one `VAULT_ROOT`, every path-dependent command is rooted there, any required source access has a usable configuration-backed `source_context`, and every Obsidian command names the verified `OBSIDIAN_VAULT` explicitly.
