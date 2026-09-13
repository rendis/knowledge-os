---
name: configure-workspace
description: "Trigger: onboard cell identity/capabilities or inspect and change local workspace config when `.knowledge-os-config.yaml` is missing or invalid, the user asks whether the workspace is initialized or configured, explicitly requests a configuration change, or a skill cannot resolve a repository, worktree root, or proxy port."
---

# Configure the workspace

Treat `<VAULT_ROOT>/.knowledge-os-config.yaml` as local state. This skill is the sole writer of local workspace configuration. Portable identity, evidence profile and procedure bindings live in `instance.yaml`; local roots and access preferences stay in `.knowledge-os-config.yaml`. Use the semantic CLI; never edit the YAML directly.

Only the canonical `.knowledge-os-config.yaml` is configuration. `status` and every semantic read are read-only; they neither inspect nor migrate similarly named files. Development worktree root and database proxy ports are independent capabilities; their absence does not make repository discovery incomplete.

## Collect onboarding decisions

For a new vault or a request for full onboarding, collect missing decisions before invoking non-interactive installers or configuration mutations. Reuse explicit user choices and valid existing configuration; ask only for missing choices. A directory's existence, its name, a sibling `repos/` or `worktrees/`, and a general request to “do everything” do not select paths or grant managed-clone authority.

| Decision | Resolve before acting |
| --- | --- |
| Vault identity and systems | Reuse the requested destination and systems; draft a purpose consistent with the stated scope. |
| Evidence profile and note language | Ask for missing choices, briefly explaining the proposed defaults; pass the selected values explicitly. |
| Trackers and adapters | During full onboarding, offer selection or explicit deferral. Deferred means unconfigured, not a confirmed absence or operational readiness. |
| Source identity and scope | Obtain the organization/remotes and exact repository scope needed for discovery; a missing inventory is not an empty successful sync. |
| Local source roots | Obtain exact paths. Suggest an existing directory if useful, then wait for the user's selection before recording it. |
| Source acquisition mode | Distinguish read-only existing checkouts from permission to clone/fetch in an exact managed root. Confirmation of a location alone does not enable acquisition. |
| Development worktrees | Ask for an exact root or explicit deferral during full onboarding. Discovery-only work may defer this capability. |

For an initial inventory, a supplied organization plus repository prefixes selects the matching repositories; reuse that scope unless the user narrows it or an observed ambiguity requires clarification.

Group related missing choices into concise questions. Stop only the actions that depend on an unanswered choice; continue independent read-only inspection. Do not ask again for choices already supplied or reopen valid existing configuration. A specific instruction to clone the scoped repositories into an exact managed root supplies that acquisition choice; do not add another approval ceremony.

`--yes` suppresses terminal prompts; it is not evidence that the user selected defaults. Use it after resolving the applicable choices. The workspace CLI is a non-interactive executor: this skill owns the conversation and supplies its parameters. Do not add a second competing configuration wizard or write configuration YAML by hand.

On a missing vault, first initialize it through the distribution installer once portable identity/profile/locale and optional selections or deferrals are resolved; then use the resolver and local configuration steps below. Keep local paths in workspace configuration. Cloning, source inspection and sync wait for their source scope and access choices. External vault publication and live-environment access remain separate capabilities, required only when requested.

Before reading or changing configuration, load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Pass any user-supplied path through that resolver. Bind `VAULT_ROOT` only from a single `resolved` result; an `invalid`, `ambiguous`, or `not_found` result blocks configuration.

## 1. Inspect

```text
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" status --format json
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" locate-repository "<GIT_REMOTE>"
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" repository
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" development-worktree-root --format json
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" development-worktree-root --format value
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" database-proxy-port "<ENVIRONMENT>" --format value
```

An `initialized` status with roots means discovery is ready. A missing worktree root or proxy port is a capability gap, not a failed discovery.

Completion criterion: current status is observed; the run either stops or enters repair with one gap.

## 2. Initialize or repair

Collect exact discovery roots. Each root is one Git repository or a directory whose immediate children are Git repositories. During a full initialization, also ask where persistent development worktrees should live; that path may be deferred if only discovery is required. Use already supplied exact values and authorization; ask only for a missing consequential choice:

```text
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" initialize --repository-root "<ROOT>" [--managed-clone-root "<ROOT>"] [--development-worktree-root "<WORKTREE_ROOT>"]
```

Refresh without wiping ports. Present only the candidate for the selected environment when a live consumer needs a proxy port:

```text
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" update [--repository-root "<ROOT>"] [--managed-clone-root "<ROOT>" | --disable-managed-clone] [--development-worktree-root "<WORKTREE_ROOT>" | --disable-development-worktree-root] [--proxy-port "ENVIRONMENT=PORT"]
```

Completion criterion: the configured values match the resolved user choices or reused existing configuration, `status` is `initialized`, and no consumer wrote the YAML by hand. Report deferred capabilities separately; do not report full onboarding or sync completion from this status alone.

## 3. Hand off

Resume the exact task that exposed the gap with the semantic value it needs.

## Cell capabilities

For database access, `instance.yaml` may declare `database_targets`: each has a unique `id`, declared `system`, `environment`, `instance`, `database`, `schemas`, canonical access `procedure`, local `port_key`, and zero or more credential-free HTTPS `repositories`. Repositories provide optional evidence; the procedure selects the executor. Inspect through `cell-config.py database-targets` and `database-target --target <ID>`. Add or update cell-owned declarations only within user-authorized configuration work, validate with the semantic read, and preserve existing fields. Resolve procedure basenames before marking a target configured. Keep credentials out of declarations.

Use the target's `port_key` with the existing `--proxy-port KEY=PORT` update and `database-proxy-port KEY` view. Preserve legacy environment keys and `sources.schema_repository` for existing consumers. An empty repository list never inherits that legacy source. Port configuration does not create an executor or establish live access.

During initial onboarding, use the installer's identity/system/tracker/profile/locale options. Collect the team's optional tools, environment names, procedures and executors only when needed. The kernel has no default company, environment list, credential mechanism or procedure basename.

Existing capabilities are read-only with:

```text
<python> -B "<VAULT_ROOT>/90-Meta/cell-config.py" --vault-root "<VAULT_ROOT>" status
<python> -B "<VAULT_ROOT>/90-Meta/cell-config.py" --vault-root "<VAULT_ROOT>" resolve --capability <id>
```

Bind an existing operational note (or first prepare the requested team procedure using the operational note contract) with:

```text
<python> -B "<VAULT_ROOT>/90-Meta/cell-config.py" --vault-root "<VAULT_ROOT>" bind --capability <id> --procedure "<canonical basename>"
```

Repeat `--procedure` for required companion catalogs. Use `runtime-inspection` for runtime policy/catalogs and `database-inspection` for the database executor contract. Procedures own permitted environments, target selection, authentication mechanism, repository/skill entrypoint and any runner flags. Record mechanisms and references, never secrets. Binding configures guidance and does not authorize live execution. Report separately: identity configured, repositories discoverable, and each requested capability configured or its exact gap; resume the initiating task.
