---
name: manage-development-handoff
description: Prepare persistent Jira worktrees and materialize repository-specific development handoffs. Use for plan, create, apply, refresh, validate, activate, or deactivate requests from an investigation/Jira story or exact package; route missing package production through `manage-investigation`.
---

# Manage development handoffs

Treat one persistent worktree as the implementation boundary for one Jira story and repository. `ACTIVE.yaml` is the single active handoff pointer inside that worktree, not a repository-global registry.

## Preflight

1. Resolve the canonical vault and source context through `map-ecosystem`; keep its interrogation branch read-only.
2. For every route that materializes or activates content, load [the input-bundle contract](references/input-bundle.md) and complete its intake protocol until every target is `exact-package`. Resolve `producer-required` through `manage-investigation` and resume this workflow with the returned directories; stop `invalid-package` at the producer with its exact failure. Never inspect or change the source investigation or Jira. Validation and deactivation use an exact repository remote and worktree path and do not require a package.
3. Resolve the development worktree root through the configuration API. If it is missing or unavailable, invoke `configure-knowledge-os-workspace`; resume only after `development-worktree-root --format json` succeeds.
4. Resolve `scripts/development-handoff.py` and bundled assets relative to this skill. Use the helper for every worktree and handoff operation.

Complete preflight only when the vault, every input required by the selected route, source repositories, and worktree root resolve semantically and no target or Git write has occurred.

## Route

- **Prepare and materialize**: default for a request to prepare a new handoff; plan worktree and handoff effects together, authorize once, then execute both phases.
- **Worktree only**: create only the persistent Jira worktree when the user explicitly requests that narrower outcome.
- **Refresh or activate**: reuse one exact existing Jira worktree and plan a no-op, activation, bootstrap, or selective update.
- **Validate**: verify an active or inactive handoff in one exact worktree without writing.
- **Deactivate**: preview and remove that worktree's active pointer while reconciling the managed instruction policy.
- **Legacy maintenance**: validate or retire handoffs already present in historical main checkouts; never create new ones there.

Load [the worktree lifecycle](references/worktree-lifecycle.md) when preparing or selecting worktrees. Load [the repository-state contract](references/repository-state.md) before planning, applying, validating, or deactivating materialized state.

## Prepare new handoffs

1. Inspect local branch candidates, ask the user to confirm the base branch, and ask for one short description used to derive `issue/<JIRA>-<slug>`. Use `issue/` by default; pass a different validated prefix only when the user explicitly requests it.
2. Run a provisional `plan-handoff` with `--base-source remote`. It may inspect a missing remote commit only in disposable temporary storage; present the exact local and remote commits, freshness classification, derived branch, canonical repository container, its `create-directory` effect when absent, absolute destination, and complete materialization effects.
3. When the commits differ, ask whether to start from `remote` or `local`. A remote selection may fetch only that exact base ref; it never updates the local base branch. If the user wants the local branch itself updated, stop for a separately authorized repository Git workflow.
4. Re-run `plan-handoff` with the selected source for every package. Reject the batch if any source, remote, branch, configured root, path, managed file effect, or token is unresolved; do not omit a target.
5. Present every Git effect, exact managed-file effect, ignored handoff path, and sequential failure boundary as one complete effect plan and one explicit authorization. The user's request for the handoff includes its canonical local package; the physical root instruction file or files and `.gitignore` remain visible effects in that same preview.
6. Apply every unchanged token with `prepare-handoff`. The helper revalidates the complete token before Git writes, verifies the created branch and `HEAD`, requires the real file plan to match the authorized projection, materializes it, and validates the active handoff.

Skip creation only when refresh targets an exact existing registered worktree that already matches the repository container and Jira key. Complete this route when every target validates at the planned revision, or a partial result names prepared, worktree-created, handoff-applied, failed, and untouched targets.

## Prepare only worktrees

Use `plan-worktree` and `create-worktree` only when the user explicitly requests a worktree without materialization. Present and authorize only its Git effects, then stop after verifying its path, branch, and selected `HEAD`; a later handoff request starts the existing-worktree planning route.

Complete this route when every authorized worktree satisfies the lifecycle postconditions, or a partial result names created, failed, and untouched targets.

## Plan existing handoffs

1. Run `plan --worktree-path <exact-path>` once for every package and prepared or selected worktree. Finish all plans before presenting any effect.
2. Reject the batch when any package, target registration, Jira branch, managed block, ignore rule, implementation-update changelog, or existing handoff fails validation.
3. Present every exact worktree path, normalized remote, branch, action, revision, changed document, policy-only root-instruction refresh, tracked `.gitignore` effect, and ignored local effect. The current bundled managed block is always the desired policy; an exact byte match produces no effect and no Git diff.
4. Wait for explicit authorization of this consolidated file plan. A changed target, branch, action, document set, or token requires a new preview. Do not request another authorization merely because an unchanged complete new-handoff plan moved from its Git phase to its file phase.

Complete planning only when every target has a valid read-only plan and the user has authorized its exact effects or declined them.

## Apply

1. Apply each authorized package with its exact `--worktree-path` and approved `plan_token`; never substitute a path, token, or package.
2. After each apply, run `validate` with the same remote and worktree path and record the observed family, revision, and `implementation-updates.md` entry count.
3. On failure, stop the batch. Report applied, failed, and untouched targets; retain successful handoffs and re-plan only the remaining work.
4. Hand the user each validated worktree path, branch, and revision so development can begin in a fresh session rooted there.

Complete apply only when every authorized target validates at its planned revision, or a partial result identifies the exact resume boundary without claiming rollback.

## Validate

Run `validate` for each requested remote and exact worktree path. Report `valid` with branch, family, revision, and implementation-update summary, `inactive` when no pointer exists, or the exact failed invariant. A stale managed block is invalid; refresh it through an authorized no-content-change plan rather than editing the target manually.

Complete validation only when every requested worktree has an observed status and no mismatch is described as usable context.

## Deactivate

1. When deactivation represents completed development, require a successful `reconcile-development-handoff` result first. An explicitly abandoned or paused handoff may deactivate without that result, but must not be reported as reconciled or complete.
2. Run `deactivate` with the exact worktree path and without a token; present the pointer deletion and any current-policy root-instruction refresh as separate effects.
3. Wait for explicit authorization, then repeat with the returned token.
4. Confirm `ACTIVE.yaml` is absent and the materialized family, history, `implementation-updates.md`, managed block, and ignore rule remain.

Complete deactivation only when the pointer removal is observed and all preserved state remains intact.

## Authority

After an exact complete handoff plan is authorized, this workflow may fetch only the selected remote base ref, create its planned local branch (`issue/...` by default), register its planned persistent worktree, create the planned destination directory under the configured root, and change only the managed block in that worktree's physical root `AGENTS.md`, physical root `CLAUDE.md`, or both, plus the root `.gitignore` rule and `.knowledge-os-handoffs/` state. A counterpart symlink is preserved and never written as a second surface. The helper keeps independent phase tokens and postconditions inside that single authorization. An explicit worktree-only plan grants only the Git subset.

It does not authorize source-code edits, Jira or investigation writes, commits, pushes, pull requests, local-base movement, pulls, merges, resets, worktree removal, branch deletion, pruning, or any other target. Keep those actions in their owning workflow and behind separate authority.
