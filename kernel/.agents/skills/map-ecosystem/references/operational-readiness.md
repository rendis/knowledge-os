# Ecosystem tooling readiness

Load this branch to check prerequisites for a specific vault query, source analysis, authoring, closure, synchronization or external reconciliation. Questions about stored configuration, initialization choices or source roots belong to `configure-workspace`; reuse its result here only when the requested operation needs it. It defines a capability-scoped preflight; it does not establish whether cell runtimes are healthy.

The readiness check itself is read-only and non-publishing: do not change versioned content, source working trees, external systems, Git authentication, or user configuration. When it exposes a workspace-configuration gap, hand off to `configure-workspace`; that skill may write local configuration only after its confirmation gate, then control returns here for a fresh preflight. Git checks use `--no-optional-locks`; native validation reads the selected vault and reports bounded diagnostics.

## When to run

Run this branch after **attempting** vault resolution with `<CLI> config resolve --vault "<root>"`. A readiness request naming a specific operation selects **operational readiness** as its primary branch. Resolve which operation an ambiguous “ready?” refers to before selecting checks. If resolution is not `resolved`, classify the requested capability as blocked, report the resolver status and smallest remediation, and stop every check that depends on `VAULT_ROOT`.

This branch may also support another primary branch when that branch observes a signal that can block its requested capability: unavailable source evidence, clone authority mismatching required acquisition, a required gate failure, or a GitHub identity error. An unavailable optional integration does not trigger the full preflight by itself; report its fallback and limitation.

Typical requests include:

- "can we query / analyze / author / close / synchronize the ecosystem map?"
- "which prerequisite prevents this requested synchronization or closure?"

## Capabilities and dimensions

Classify the requested operation before checking it:

| Capability | Includes |
|---|---|
| **Vault query** | Read/search notes, links, backlinks, Bases, and implementation context already present in the vault |
| **Source-backed analysis** | Inspect source code, schemas, deployment evidence, or cross-repository references without editing the vault |
| **Vault authoring** | Create or update versioned vault content; this can proceed before closing dependencies or gates are repaired |
| **Vault closure** | Validate an authored change through every applicable Framework gate |
| **Synchronization** | Inventory remote repositories, detect changes, acquire required evidence, and optionally propagate vault updates |
| **External reconciliation** | Resolve a selected provider, cluster or database question through a configured read-only procedure after static evidence is exhausted |

Apply each dimension only where marked required (`R`), conditional on the concrete operation (`C`), or optional with a documented fallback (`O`). A dash means the dimension does not gate that capability.

| Dimension | Vault query | Source analysis | Vault authoring | Vault closure | Synchronization | External reconciliation | Observable signal |
|---|:---:|:---:|:---:|:---:|:---:|:---:|---|
| Vault resolution | R | R | R | R | R | R | Resolver status is `resolved`; canonical remote and markers match |
| Source access | — | R | C | — | C | C | Required when versioned evidence may answer the selected question |
| Installed CLI | R | R | R | R | R | R | The platform binary executes the resolver and applicable tooling |
| Framework gates | — | — | — | R | C | C | Required when the resulting documentation will be applied and closed |
| Local `main` baseline | — | — | R | R | C | C | Required when reconciliation will author and close vault changes |
| Obsidian binding | O | O | O | O | O | O | Exact normalized vault path matches; otherwise use the documented filesystem fallback |
| GitHub identity | — | C | C | — | R | C | Required only when the selected versioned evidence is remote |
| Clone authorization | — | C | C | — | C | C | Required only when missing or stale evidence must be cloned or fetched |
| Procedure binding | — | — | — | — | — | R | The semantic workspace view resolves the configured executor or adapter for the exact procedure and target |
| Read-only target access | — | — | — | — | — | R | That executor or adapter proves a bounded read-only operation against the exact authority and target using existing authentication |

A failure in a closing dimension does not retroactively block authoring. Report combinations explicitly, for example: `Ready for vault authoring; Blocked for vault closure: required review missing`.

## How to check

Bind `<cli>` through [use-vault-cli](../../use-vault-cli/SKILL.md). The candidate is the user-supplied path or the root containing this installed skill. Resolve before reading domain content:

```text
<cli> config resolve --vault "<candidate-root>"
```

After a `resolved` result, use its canonical `VAULT_ROOT` and run only applicable checks:

```text
<cli> version
<cli> config workspace --vault "<vault_root>"
<cli> audit --vault "<vault_root>"
<cli> check links --vault "<vault_root>"
<cli> check bases --vault "<vault_root>"

# Check exact Obsidian identity before native application queries.
<cli> check obsidian-binding --vault "<vault_root>" --vault-name "<obsidian_vault>"
obsidian "vault=<obsidian_vault>" unresolved
obsidian "vault=<obsidian_vault>" orphans

git --no-optional-locks -C "<vault_root>" show-ref --verify --quiet refs/heads/main
git --no-optional-locks -C "<vault_root>" status --porcelain

# Only when remote repository inventory is required.
<cli> inventory --vault "<vault_root>" --format markdown
```

The installed CLI requires neither Python nor Go. Git, GitHub CLI, Obsidian and procedure-specific tools are external dependencies only for operations that use them.

Read `source_context` as defined in [vault-resolution.md](../../../../90-Meta/vault-resolution.md). An arbitrary directory is not proof that the target repository is usable. Report source configuration independently from readiness; mixed postures are valid:

- **Configured discovery** — reusable roots came from the semantic workspace view with `origin: config`.
- **Configured clone authority** — `clone_authorized: true` and `clone_origin: config`; the exact clone root is also marked `managed` in `roots`.
- **Unavailable** — the local configuration is missing or has no usable source path for the requested evidence.
- **Inconsistent** — the semantic view rejected a path, remote identity, cache, permission, or managed-root invariant.

`roots[].managed`, `clone_origin`, and `clone_authorized` describe acquisition authority independently from discovery. Consumers do not infer roots from the current directory or environment.

### External target access

The workspace view binds a team procedure to an executor or adapter; it does not authorize or prove access to the discovered connection. Resolve the binding from the semantic workspace view, then use the procedure's own status or bounded read-only probe against the exact provider/project, cluster context or database target. Reuse the existing authenticated identity or session named by the procedure. A successful probe establishes only that bounded access at the observed target and time.

If no procedure is bound, hand the exact capability, target and read-only question to `configure-workspace`. If the procedure is bound but access fails, report its exact missing executor, profile, role, session, network route or target-specific setup and retry the same probe after that setup is available. Discovery, a configured connection string or secret reference, and workspace initialization alone are never a passing access signal.

## Status classification

- **Ready for `<capability>`** — every required and applicable conditional dimension passes. List unavailable optional integrations as limitations without broadening the claim.
- **Degraded for `<capability>`** — the operation can proceed through a documented fallback, but a material native verification surface is unavailable. State what the fallback cannot verify.
- **Blocked for `<capability>`** — a required or applicable conditional dimension fails. Do not perform the blocked operation; offer the smallest remediation.

Never return a bare "Operational", "ready", or "not ready". Name the capability, configuration posture, observed dimensions, exact failing/degraded signals, fallback limitations, and offered remediation. Never apply these classifications to cell runtime health.

## How to suggest preparation

Offer the smallest fix for the failing dimension. Never edit user-level environment variables or launch configuration without explicit authorization; `.knowledge-os-config.yaml` may be written only by `configure-workspace` after explicit confirmation within onboarding opened by an initialization/configuration request, an incomplete readiness result, or an operation that requires the missing view.

### Workspace configuration

Run the read-only status view from `VAULT_ROOT`:

```text
<cli> config workspace --vault "<vault_root>"
```

If it is not `initialized`, load `configure-workspace`. The observed gap opens demand-triggered onboarding: the owner skill may perform read-only discovery and present the required roots and proxy-port map, but the readiness question does not authorize a write. That skill owns confirmation, ambiguity resolution, initialization, repair, and every write to `.knowledge-os-config.yaml`. Readiness consumers must not parse or patch the file themselves. After a confirmed change, rerun status and the vault resolver; no process restart is required.

### Dimension-specific remediation

**Source access unavailable or inconsistent.** Load `configure-workspace`, report the semantic status/error, and collect only the exact root or candidate choice it requires. If cloning is needed, obtain approval for an exact existing managed root and let the owner skill record it before acquisition. A vault-only query does not require source access.

**Vault resolution not `resolved`.** Follow [vault-resolution.md](../../../../90-Meta/vault-resolution.md): `invalid` means report the wrong path; `ambiguous` means present verified candidates and request a choice; `not_found` means request an explicit local path or separate authorization and destination to clone the vault.

**CLI or gates unavailable.** Report the missing executable or exact failing gate. Restore the matching platform release through the distribution installer; rerun the affected checks. Do not replace a failed semantic review with a structural check.

**Git baseline.** This vault uses local `main` as its only baseline. A missing `refs/heads/main` blocks authoring, closure, and synchronization. A dirty working tree does not affect readiness: report changed/untracked paths as delivery context and preserve unrelated work.

**Obsidian binding.** Run `<cli> check obsidian-binding` before `unresolved` or `orphans`; Obsidian may print `Vault not found` with exit `0`. If no exact normalized registration matches, use `<cli> check links` and report that native backlinks/rendering were not verified.

**GitHub identity.** Suggest `--github-user <login>` for a stored account, a non-interactive `GH_TOKEN`/`GITHUB_TOKEN`, or authenticating `gh` and checking SSO authorization. Never persist or print tokens.

**External target access.** Preserve the configured procedure and existing authentication. Ask only for the exact missing setup emitted by its executor or adapter, then rerun the same bounded read-only probe. Infrastructure, credential, role or network changes remain separate operations requiring their own authority.

## Managed clone authority

Use the single normative protocol in [vault-resolution.md](../../../../90-Meta/vault-resolution.md). In summary, configured discovery is read-only by default; an exact resolved `CLONE_ROOT` grants only the documented clone/fetch and detached-temporary-worktree operations. The clone root must be one configured repository root; parent/child overlap does not transfer authority, and a clone root equal to or inside `VAULT_ROOT` is invalid.

## Completion criterion

The readiness check is complete when the requested capability and source posture are named, every required/applicable dimension has an observed signal, optional limitations and fallbacks are explicit, every blocker has a remediation, and no unobserved check or runtime-health claim is presented as passing.
