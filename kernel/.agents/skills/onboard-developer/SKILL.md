---
name: onboard-developer
description: "Trigger: `config status` reports `workspace: uninitialized` (a person's first session with this vault), a skill cannot resolve a repository, worktree root, cloud login or proxy port, or the user asks to set up or change their local workspace. Sole writer of `.knowledge-os-config.yaml`."
---

# Onboard a developer

Each person brings their own machine: where the cell's repositories are cloned, where development worktrees go, which clouds they are logged in to, which local ports reach the databases. That lives in the ignored `.knowledge-os-config.yaml`, written only through `<CLI> config`. The cell's shared setup (systems, sources, clouds, database targets) belongs to [onboard-cell](../onboard-cell/SKILL.md).

**Propose, then confirm.** The machine answers most questions; the user confirms once. Bind `<CLI>` and the vault root through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault) first; when `kos` is not installed yet, installing it (with approval) is the first step of this onboarding.

## 1. Detect

```text
<CLI> config --vault "<root>" detect
```

Read-only. It returns the directories holding clones of the cell's repositories (with how many each holds), a worktree root beside them, each configured cloud's CLI and login, the database targets' proxy ports, `propose` (the command that records the proposal) and `missing` (each gap with its fix).

## 2. Confirm once

Show one short table and ask one question, confirm or correct:

| | Proposed |
|---|---|
| Repositories | `<root>` — N of the cell's repositories |
| Worktrees | `<worktree root>` (created on first use) |
| Clouds | account per cloud, or the login command |
| Databases | port per target, or ask for it |

With no clone found, ask where the repositories are, or whether to clone them into a directory the user names; naming it grants clone authority for that directory only.

## 3. Record

Run the confirmed command, adding what the user corrected:

```text
<CLI> config --vault "<root>" workspace-init --repository-root "<ROOT>" [--development-worktree-root "<DIR>"] [--managed-clone-root "<DIR>"] [--proxy-port ENV=PORT]
```

Later changes use `workspace-update`, which keeps every value it is not given (`--disable-managed-clone` and `--disable-development-worktree-root` remove one). Logins are the user's to run: give the exact command (`gcloud auth login`, `aws sso login --profile <profile>` then `AWS_PROFILE`, `az login`) and never ask for a credential.

Done when `config status` shows `workspace: initialized`.

## 4. Access card

Run `detect` again and give the card in a few lines, one mark per line:

- kos: version, current or the update it needs (`kos version`)
- repositories: N found under `<root>`
- worktrees: `<dir>`
- each cloud: account, or its login command
- each database target: port, or what is missing

Then resume the task that led here. Done when the card is delivered and every open line carries its fix.
