# Ecosystem tooling readiness

Load this branch when the user asks whether the vault, repository, or ecosystem-map tooling is initialized, prepared, or ready for a specific operation. It defines a capability-scoped preflight; it does not establish whether cell runtimes are healthy.

The readiness check itself is read-only and non-publishing: do not change versioned content, source working trees, external systems, Git authentication, or user configuration. When it exposes a workspace-configuration gap, hand off to `configure-workspace`; that skill may write local configuration only after its confirmation gate, then control returns here for a fresh preflight. Python checks use `-B` to avoid bytecode caches, Git checks use `--no-optional-locks`, and tooling tests may create and remove disposable fixtures under the host temporary directory.

## When to run

Run this branch after **attempting** vault resolution with the resolver bundled in the installed skill directory. A readiness-only request selects **operational readiness** as its primary branch. If resolution is not `resolved`, classify the requested capability as blocked, report the resolver status and smallest remediation, and stop every check that depends on `VAULT_ROOT`.

This branch may also support another primary branch when that branch observes a signal that can block its requested capability: unavailable source evidence, clone authority mismatching required acquisition, a required gate failure, or a GitHub identity error. An unavailable optional integration does not trigger the full preflight by itself; report its fallback and limitation.

Typical requests include:

- "is the repo initialized / set up / bootstrapped?"
- "is the vault tooling ready / good to go / can we start?"
- "can we query / analyze / author / close / synchronize the ecosystem map?"
- "why can't the tooling find the source repos / vault / Obsidian / GitHub identity?"

## Capabilities and dimensions

Classify the requested operation before checking it:

| Capability | Includes |
|---|---|
| **Vault query** | Read/search notes, links, backlinks, Bases, and implementation context already present in the vault |
| **Source-backed analysis** | Inspect source code, schemas, deployment evidence, or cross-repository references without editing the vault |
| **Vault authoring** | Create or update versioned vault content; this can proceed before closing dependencies or gates are repaired |
| **Vault closure** | Validate an authored change through every applicable Framework gate |
| **Synchronization** | Inventory remote repositories, detect changes, acquire required evidence, and optionally propagate vault updates |

Apply each dimension only where marked required (`R`), conditional on the concrete operation (`C`), or optional with a documented fallback (`O`). A dash means the dimension does not gate that capability.

| Dimension | Vault query | Source analysis | Vault authoring | Vault closure | Synchronization | Observable signal |
|---|:---:|:---:|:---:|:---:|:---:|---|
| Vault resolution | R | R | R | R | R | Resolver status is `resolved`; canonical remote and markers match |
| Source access | — | R | C | — | C | Required for update/analysis of changed sources; not for a report-only remote inventory |
| Python 3 | R | R | R | R | R | The selected interpreter executes the resolver and applicable tooling |
| Pinned gate dependencies | — | — | — | R | C | Installed versions satisfy `90-Meta/requirements-ci.txt`; required for update-mode closing gates, not a report-only inventory |
| Framework gates | — | — | — | R | C | Required commands are available and applicable baseline/closing gates exit `0` |
| Local `main` baseline | — | — | R | R | C | Required for synchronization that authors/closes vault changes; report-only inventory remains read-only |
| Obsidian binding | O | O | O | O | O | Exact normalized vault path matches; otherwise use the documented filesystem fallback |
| GitHub identity | — | C | C | — | R | Inventory preflight can access the organization without changing global authentication |
| Clone authorization | — | C | C | — | C | Required only when missing or stale evidence must be cloned or fetched |

A failure in a closing dimension does not retroactively block authoring. Report combinations explicitly, for example: `Ready for vault authoring; Blocked for vault closure: ruamel.yaml missing`.

## How to check

First run the resolver from `SKILL_DIR`, the installed `map-ecosystem` skill directory, not from a presumed vault copy. Use the interpreter available on the host:

```text
macOS/Linux (bash, zsh, fish): python3 -B "<installed-skill-dir>/scripts/resolve-vault.py"
Windows PowerShell:            py -3 -B "<installed-skill-dir>\scripts\resolve-vault.py"
Windows fallback:              python -B "<installed-skill-dir>\scripts\resolve-vault.py"
```

Only after a `resolved` result, change to `VAULT_ROOT` and run the applicable commands below. In these examples, `<python>` means the same selected interpreter command (`python3`, `py -3`, or `python`). Quoted placeholders must be replaced with the exact resolver values.

```text
# Interpreter and pinned dependency (read-only checks).
<python> --version
<python> -B -c "import ruamel.yaml; print(ruamel.yaml.__version__)"

# Framework gates, only for vault closure or update-mode synchronization.
<python> -B 90-Meta/test_workspace_config.py
<python> -B 90-Meta/test-vault-tooling.py
<python> -B 90-Meta/audit-vault.py
<python> -B 90-Meta/verify-links.py
<python> -B 90-Meta/validate-bases.py

# Exact Obsidian binding must pass before native checks.
<python> -B 90-Meta/check-obsidian-binding.py --vault-root "<vault_root>" --vault-name "<obsidian_vault>"
obsidian "vault=<obsidian_vault>" unresolved
obsidian "vault=<obsidian_vault>" orphans

# Local baseline and informational working-tree state.
git --no-optional-locks -C "<vault_root>" show-ref --verify --quiet refs/heads/main
git --no-optional-locks -C "<vault_root>" status --porcelain

# GitHub identity, only when required by the capability.
<python> -B 90-Meta/vault-inventory.py --format markdown
```

The argument-vector form above is shell-neutral; do not copy the literal angle-bracket placeholders. CI uses the `python` executable installed by `actions/setup-python`. Local Windows hosts may use `py -3` or `python`; macOS/Linux commonly use `python3`.

Read `source_context` as defined in [vault-resolution.md](vault-resolution.md). An arbitrary directory is not proof that the target repository is usable. Report source configuration independently from readiness; mixed postures are valid:

- **Configured discovery** — reusable roots came from the semantic workspace view with `origin: config`.
- **Configured clone authority** — `clone_authorized: true` and `clone_origin: config`; the exact clone root is also marked `managed` in `roots`.
- **Unavailable** — the local configuration is missing or has no usable source path for the requested evidence.
- **Inconsistent** — the semantic view rejected a path, remote identity, cache, permission, or managed-root invariant.

`roots[].managed`, `clone_origin`, and `clone_authorized` describe acquisition authority independently from discovery. Consumers do not infer roots from the current directory or environment.

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
<python> -B 90-Meta/workspace-config.py --vault-root "<vault_root>" status --format json
```

If it is not `initialized`, load `configure-workspace`. The observed gap opens demand-triggered onboarding: the owner skill may perform read-only discovery and present the required roots and proxy-port map, but the readiness question does not authorize a write. That skill owns confirmation, ambiguity resolution, initialization, repair, and every write to `.knowledge-os-config.yaml`. Readiness consumers must not parse or patch the file themselves. After a confirmed change, rerun status and the vault resolver; no process restart is required.

### Dimension-specific remediation

**Source access unavailable or inconsistent.** Load `configure-workspace`, report the semantic status/error, and collect only the exact root or candidate choice it requires. If cloning is needed, obtain approval for an exact existing managed root and let the owner skill record it before acquisition. A vault-only query does not require source access.

**Vault resolution not `resolved`.** Follow [vault-resolution.md](vault-resolution.md): `invalid` means report the wrong path; `ambiguous` means present verified candidates and request a choice; `not_found` means request an explicit local path or separate authorization and destination to clone the vault.

**Pinned dependencies or gates unavailable.** Report the missing/mismatched dependency and affected closing gates. CI installs `90-Meta/requirements-ci.txt` in an isolated runner. Locally, run `<python> -m pip install --no-deps -r 90-Meta/requirements-ci.txt` only when the user asked to prepare/fix the environment or explicitly authorized installation, then re-run the failed gates.

**Git baseline.** This vault uses local `main` as its only baseline. A missing `refs/heads/main` blocks authoring, closure, and synchronization. A dirty working tree does not affect readiness: report changed/untracked paths as delivery context and preserve unrelated work.

**Obsidian binding.** Run `check-obsidian-binding.py` before `unresolved` or `orphans`; Obsidian may print `Vault not found` with exit `0`. If no exact normalized registration matches, use `verify-links.py` and report that native backlinks/rendering were not verified.

**GitHub identity.** Suggest `--github-user <login>` for a stored account, a non-interactive `GH_TOKEN`/`GITHUB_TOKEN`, or authenticating `gh` and checking SSO authorization. Never persist or print tokens.

## Managed clone authority

Use the single normative protocol in [vault-resolution.md](vault-resolution.md). In summary, configured discovery is read-only by default; an exact resolved `CLONE_ROOT` grants only the documented clone/fetch and detached-temporary-worktree operations. The clone root must be one configured repository root; parent/child overlap does not transfer authority, and a clone root equal to or inside `VAULT_ROOT` is invalid.

## Completion criterion

The readiness check is complete when the requested capability and source posture are named, every required/applicable dimension has an observed signal, optional limitations and fallbacks are explicit, every blocker has a remediation, and no unobserved check or runtime-health claim is presented as passing.
