---
name: manage-git-workflow
description: Analyze or maintain a vault's Git/GitHub policy, or operate source repositories governed by it. Use for policy evidence and source-repository branch, commit, synchronization, pull-request, merge, release, hotfix, or recovery work; use repository instructions for routine local versioning of the vault itself, and manage-development-handoff for persistent work-item handoff worktrees.
---

# Manage Git workflows

Treat `60-Operacion/Git/Git.md` as the Git area index: it supplies a brief scope and links, not policy rules. Discover cell-specific Git/GitHub policy, standards, guides, and procedures from every specific operational note in that area, then select those whose scope covers the target repository and workflow. Apply them to the vault repository itself only when a specific note explicitly includes that repository. Treat applicable repository instructions as repository-specific policy and live Git or GitHub state as current operational truth. Use [the generic defaults](references/defaults.md) only for repository operations whose applicable specific notes and repository instructions leave a decision unanswered.

## 1. Select the workflow

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Bind one canonical `VAULT_ROOT`.
2. When the request targets a repository, resolve its exact Git root. Classify it as the vault repository when that root is `VAULT_ROOT`; otherwise classify it as a source repository.
3. For routine local status, staging, or commit work on the vault repository, follow its applicable repository instructions and end this skill unless the user explicitly invoked it.
4. Inspect `${VAULT_ROOT}/60-Operacion/Git/` when it exists. Read `Git.md` for area scope and navigation when present, and report a missing MOC as a documentation gap. Independently inventory every other Markdown note directly under the area, resolve each through `90-Meta/operational-catalog.py resolve --basename <name>`, and read the notes whose class, state, and declared scope may apply. Compare that inventory with the MOC links and report missing or stale links without excluding an unlinked specific note. For a vault-repository operation, apply a specific note only when it explicitly includes that repository. Detailed rules found only in the index are misplaced content, not effective policy: include their move in a policy-maintenance delta, and block any repository-operation decision that depends on those rules until a specific note exists or the user explicitly resolves the conflict. Independent read-only inspection may continue.
5. Choose one workflow:
   - **Policy evidence** — the request supplies or asks to analyze, propose, document, or maintain Git/GitHub rules. Follow section 2. Repository resolution, live state, and generic defaults are outside this workflow unless the evidence or requested delta is repository-specific.
   - **Repository operation** — the request asks to inspect or change a branch, commit, synchronization, pull request, merge, release, hotfix, or recovery state. Follow sections 3 and 4.

Workflow selection is complete when one branch owns the request; use policy evidence when the request is solely about new rules, even if those rules describe future repository operations.

## 2. Analyze and maintain vault policy

A policy-analysis response is a proposed delta, not a general summary or an operating guide. Extract only durable rules that affect Git or GitHub and compare them with the applicable specific Git notes and any repository policy named by the evidence or request. Bound the proposal to workflows, exceptions, and artifacts named by that evidence, those notes, or the user's request. When examples help define a rule, reuse only examples supplied by those sources.

Present only applicable delta labels: **Create**, **Add**, **Change**, **Remove**, **Contradiction**, **Limitation**, and **Blocker**. Omit empty labels. If no applicable specific policy note exists, **Create** states its proposed path under `60-Operacion/Git/`, exact rules, `clase: politica`, and link from `Git.md`. Use another operational class only when the document is actually a standard, guide, catalog, procedure, or report contract. Each item states its affected workflow or scope and supporting evidence. Preserve explicit source statements unless stronger applicable evidence contradicts them.

Treat the supplied evidence as sufficient for the subset it supports. A broader referenced source may enrich that subset later and does not delay an otherwise supported delta. Keep an explicit source rule in the proposal unless applicable evidence contradicts it. Report a **Limitation** only when the evidence identifies it or the supported rule cannot be stated accurately without it. Report a **Blocker** only when the exact requested Git decision cannot be made.

For an analysis-only request that asks how to proceed, finish with one next policy step: review or authorize the delta, or obtain the evidence named by a **Blocker**. Keep that step within policy maintenance; omit interim repository behavior, additional artifacts, and verification work for an explicit source rule. Then stop without changes.

If any proposed effect is not already authorized—especially a change, removal, contradiction, or blocker—stop for user review. Create or update only the authorized specific operational notes under `${VAULT_ROOT}/60-Operacion/Git/`, preserving unrelated rules. Load `../manage-operational-workflow/references/procedure-contract.md` and follow the selected class contract. Create the area MOC when absent, or update only its scope summary and navigation links, with this minimum frontmatter:

```yaml
---
tipo: indice
tags: [moc, operacion, operacion/area/git]
---
```

Keep the MOC body to a brief scope or high-level synthesis plus links to specific notes. Record durable rules, exceptions, escalation points, and procedures in those notes; keep transient repository state and command transcripts out. Link a newly created area MOC from `60-Operacion/Operacion.md` when that index exists. Never migrate detailed MOC content as a side effect of a repository operation or an unrelated policy edit.

Policy maintenance is complete when every authorized change is accounted for, the created or modified content follows its specific-note or MOC contract, and no repository operation was inferred from documentation authority. Record pre-existing defects outside the authorized scope as explicit pending items; they do not require an incidental migration or prevent closure unless the requested change depends on resolving them. Keep such dependent blockers explicit.

## 3. Plan a repository operation

1. Resolve one exact repository identity from an explicit local path, the current Git worktree, a GitHub URL, or `<host>/<owner>/<repo>`; confirm the host for `owner/repo` shorthand. A GitHub-only operation may use that remote identity without a checkout. Stop an ambiguous target before any mutation.
2. For read-only local inspection without a supplied checkout, follow the source-repository binding in `vault-resolution.md`; invoke `configure-workspace` when its discovery configuration is missing or invalid. Keep every checkout resolved from `SOURCE_ROOTS` read-only. A local mutation requires an exact worktree supplied by the user or current task whose mutation is authorized; never acquire a checkout as a side effect of resolution.
3. Read applicable instructions from the local checkout or relevant remote ref. Apply the most specific compatible rule. For source repositories, the applicable specific Git notes override this skill's defaults; for the vault repository, they do so only when they explicitly include that repository. Safety and authorization constraints remain in force.
4. Load [references/defaults.md](references/defaults.md) only when the requested operation needs a decision not answered by repository instructions or the applicable specific Git notes.
5. Inspect the minimum live context required by the operation: worktree status, branch, upstream, remotes and relevant refs; for GitHub work, also confirm the host, repository, authenticated account, base branch, and applicable checks or rulesets.

Context is resolved when the exact repository, applicable policy, current state, and unresolved conflicts are known without changing target refs, worktree files, or remote state.

Choose the smallest operation that reaches the requested outcome. Use local `git` for repository state and history; use authenticated `gh` for GitHub resources. Bind every `gh` invocation to the resolved host and repository: use `--repo` or `GH_REPO` for repository commands, and an exact API route plus host for `gh api`. Never rely on the repository inferred from the current directory. Consult live command help or official documentation for other syntax that may vary instead of maintaining a command catalog here.

Before a mutation, identify its exact repository, branch or ref, affected local state, remote destination, and verification. Treat the user's request as authority only for effects it explicitly includes. Obtain separate authorization for any additional commit, push, force update, merge, tag, release, branch deletion, destructive recovery, or GitHub write.

Planning is complete when every intended effect is explicit, unrelated work is protected, and any missing policy, identity, permission, or target is reported as a blocker.

## 4. Execute and verify

Recheck the state used by the plan, then execute only the authorized operation. Preserve unrelated changes; stage explicit paths and inspect the staged diff before committing. If state changed materially, stop and re-plan rather than adapting a destructive command in place.

Verify through the authoritative surface: inspect local refs and status after local Git operations, the remote ref after a push, and the repository identity plus created or changed GitHub object after a `gh` mutation. Report the exact resulting branch, commit, pull request, tag, release, or blocker.

Execution is complete only when the requested result is observed and every additional or failed effect is visible.

## Ownership

This skill owns the specific Git/GitHub operational notes and their area-index links, as well as authorized repository operations. The MOC remains navigation; it never becomes the policy document. Policy rules come from the applicable normative authority and do not require implementation or production evidence. Technical mapping consumes the specific notes without rewriting them. Preserve the operational note format and canonical links when maintaining policy.

Route persistent work-item worktree creation, attachment, materialization, and handoff state through `manage-development-handoff`. This skill may operate inside an existing repository or worktree, but it does not change `.knowledge-os-handoffs/` or investigation records.
