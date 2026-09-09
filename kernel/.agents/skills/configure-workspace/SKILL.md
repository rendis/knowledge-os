---
name: configure-workspace
description: "Trigger: onboard cell identity/capabilities or inspect and change local workspace config when `.knowledge-os-config.yaml` is missing or invalid, the user asks whether the workspace is initialized or configured, explicitly requests a configuration change, or a skill cannot resolve a repository, worktree root, or proxy port."
---

# Configure the workspace

Treat `<VAULT_ROOT>/.knowledge-os-config.yaml` as local state. This skill is the sole writer of local workspace configuration. Portable identity, evidence profile and procedure bindings live in `instance.yaml`; local roots and access preferences stay in `.knowledge-os-config.yaml`. Use the semantic CLI; never edit the YAML directly.

Only the canonical `.knowledge-os-config.yaml` is configuration. `status` and every semantic read are read-only; they neither inspect nor migrate similarly named files. Development worktree root and database proxy ports are independent capabilities; their absence does not make repository discovery incomplete.

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

Completion criterion: `status` is `initialized` and no consumer wrote the YAML by hand.

## 3. Hand off

Resume the exact task that exposed the gap with the semantic value it needs.

## Cell capabilities

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
