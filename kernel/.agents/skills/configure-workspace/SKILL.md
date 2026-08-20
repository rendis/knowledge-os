---
name: configure-workspace
description: "Trigger: inspect or change local workspace config when `.knowledge-os-config.yaml` is missing or invalid, the user asks whether the workspace is initialized or configured, explicitly requests a configuration change, or a skill cannot resolve a repository, worktree root, or proxy port."
---

# Configure the workspace

Treat `<VAULT_ROOT>/.knowledge-os-config.yaml` as local state. This skill is the sole writer. Use the semantic CLI; never edit the YAML directly.

If another ignored `.*-config.yaml` exists and the canonical file does not, `status` migrates it once. Development worktree root and database proxy ports are independent capabilities; their absence does not make repository discovery incomplete.

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

Collect exact discovery roots. Each root is one Git repository or a directory whose immediate children are Git repositories. During a full initialization, also ask where persistent development worktrees should live; that path may be deferred if only discovery is required. Persist only after confirmation:

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
