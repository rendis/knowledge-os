---
name: configure-workspace
description: "Trigger: inspect or change local workspace config when `.knowledge-os-config.yaml` is missing, invalid, or a skill cannot resolve a repository, worktree root, or proxy port."
---

# Configure the workspace

Treat `<VAULT_ROOT>/.knowledge-os-config.yaml` as local state. This skill is the sole writer. Use the semantic CLI; never edit the YAML directly.

## 1. Inspect

```text
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" status --format json
```

An `initialized` status with roots means discovery is ready. A missing worktree root does not make discovery incomplete. For a status-only request, report the observed roots and stop.

```text
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" locate-repository "<GIT_REMOTE>"
```

Completion criterion: current status is observed; the run either stops or enters repair with one gap.

## 2. Initialize or repair

Collect exact discovery roots. Each root is one Git repository or a directory whose immediate children are Git repositories. Persist only after confirmation:

```text
<python> -B "<VAULT_ROOT>/90-Meta/workspace-config.py" --vault-root "<VAULT_ROOT>" initialize --repository-root "<ROOT>" [--managed-clone-root "<ROOT>"] [--development-worktree-root "<WORKTREE_ROOT>"]
```

Completion criterion: `status` is `initialized` and no consumer wrote the YAML by hand.

## 3. Hand off

Resume the interrupted skill with the semantic value it needs.
