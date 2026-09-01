---
name: manage-git-workflow
description: Operate or document repository Git and GitHub workflows using the vault's local policy. Use for branch, commit, synchronization, pull-request, merge, release, hotfix, or recovery work; persistent Jira handoff worktrees belong to manage-development-handoff.
---

# Manage Git workflows

Treat `60-Operacion/Git/Git.md` as the cell-specific policy, applicable repository instructions as the repository-specific policy, and live Git or GitHub state as current operational truth. Use [the generic defaults](references/defaults.md) only for decisions those sources leave unanswered.

## 1. Resolve context and policy

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Bind one canonical `VAULT_ROOT`.
2. Resolve one exact repository identity from an explicit local path, the current Git worktree, a GitHub URL, or `<host>/<owner>/<repo>`; confirm the host for `owner/repo` shorthand. A GitHub-only operation may use that remote identity without a checkout. Stop an ambiguous target before any mutation.
3. For read-only local inspection without a supplied checkout, follow the source-repository binding in `vault-resolution.md`; invoke `configure-workspace` when its discovery configuration is missing or invalid. Keep every checkout resolved from `SOURCE_ROOTS` read-only. A local mutation requires an exact worktree supplied by the user or current task whose mutation is authorized; never acquire a checkout as a side effect of resolution.
4. Read applicable instructions from the local checkout, or from the relevant ref for a GitHub-only operation, plus `${VAULT_ROOT}/60-Operacion/Git/Git.md` when it exists. Apply the most specific compatible rule; the vault policy overrides this skill's defaults, while safety and authorization constraints remain in force.
5. Load [references/defaults.md](references/defaults.md) only when the requested operation needs a decision not answered by the repository or vault policy.
6. Inspect the minimum live context required by the operation: worktree status, branch, upstream, remotes and relevant refs; for GitHub work, also confirm the host, repository, authenticated account, base branch, and applicable checks or rulesets.

Context is resolved when the exact repository, applicable policy, current state, and unresolved conflicts are known without changing target refs, worktree files, or remote state.

## 2. Plan the operation

Choose the smallest operation that reaches the requested outcome. Use local `git` for repository state and history; use authenticated `gh` for GitHub resources. Bind every `gh` invocation to the resolved host and repository: use `--repo` or `GH_REPO` for repository commands, and an exact API route plus host for `gh api`. Never rely on the repository inferred from the current directory. Consult live command help or official documentation for other syntax that may vary instead of maintaining a command catalog here.

Before a mutation, identify its exact repository, branch or ref, affected local state, remote destination, and verification. Treat the user's request as authority only for effects it explicitly includes. Obtain separate authorization for any additional commit, push, force update, merge, tag, release, branch deletion, destructive recovery, or GitHub write.

Planning is complete when every intended effect is explicit, unrelated work is protected, and any missing policy, identity, permission, or target is reported as a blocker.

## 3. Execute and verify

Recheck the state used by the plan, then execute only the authorized operation. Preserve unrelated changes; stage explicit paths and inspect the staged diff before committing. If state changed materially, stop and re-plan rather than adapting a destructive command in place.

Verify through the authoritative surface: inspect local refs and status after local Git operations, the remote ref after a push, and the repository identity plus created or changed GitHub object after a `gh` mutation. Report the exact resulting branch, commit, pull request, tag, release, or blocker.

Execution is complete only when the requested result is observed and every additional or failed effect is visible.

## 4. Maintain vault policy

Create or update `${VAULT_ROOT}/60-Operacion/Git/Git.md` only when the user explicitly asks to document or maintain Git/GitHub policy. Create the area MOC with this minimum frontmatter:

```yaml
---
tipo: indice
tags: [moc, operacion, operacion/area/git]
---
```

Record durable domain rules, exceptions, and escalation points; keep transient repository state and command transcripts out. Link a newly created area note from `60-Operacion/Operacion.md` when that index exists.

Policy maintenance is complete when the note states only supported local rules, distinguishes unknowns from defaults, and no operational Git or GitHub effect was inferred from documentation authority.

## Ownership

Route persistent Jira worktree creation, attachment, materialization, and handoff state through `manage-development-handoff`. This skill may operate inside an existing repository or worktree, but it does not change `.knowledge-os-handoffs/` or investigation records.
