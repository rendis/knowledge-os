---
name: manage-development-handoff
description: Prepare persistent work-item worktrees and materialize repository-specific development handoffs. Use for plan, create, apply, refresh, validate, attach, or state-change requests from an investigation work item or exact package; route missing package production through `manage-investigation`.
---

# Manage development handoffs

Treat one persistent worktree as the implementation boundary for one repository and one investigation. `ACTIVE.yaml` is that worktree's lifecycle source of truth and lists one or more repository-specific work-item handoffs from the investigation; every handoff retains its own family directory. The worktree agent owns lifecycle transitions and the helper is its sole mechanical state writer.

For a disputed repository-state claim, apply [evidence-driven-analysis](../evidence-driven-analysis/SKILL.md) within this workflow's permitted inputs. Package sufficiency stays with the producer; the method does not authorize investigation or tracker reads here, reroute the request, or replace helper validation.

## Preflight

Bind `<vaultctl>` through [use-vault-cli](../use-vault-cli/SKILL.md) before the preflight.

1. Load `../../../90-Meta/vault-resolution.md` and run `<vaultctl> config resolve --vault <vault-root>`. Bind `VAULT_ROOT` and source context only from one canonical `resolved` result.
2. For every route that materializes or activates content, load [the input-bundle contract](references/input-bundle.md) and complete its intake protocol until every target is `exact-package`. Resolve `producer-required` through `manage-investigation` and continue this workflow with the exact directories it produced; stop `invalid-package` at the producer with its exact failure. Never inspect or change the source investigation or tracker. Validation and state updates use an exact repository remote and worktree path and do not require a package.
3. Treat the producer's sufficiency result as scoped to the selected story and repository package. Never infer it from the investigation's `investigating`, `blocked`, or `closed` state, and never change or close the investigation because of a worktree lifecycle event.
4. Resolve the development worktree root through the configuration API. If it is missing or unavailable, invoke `configure-workspace`; resume only after `<vaultctl> config worktree-root --vault <vault-root>` returns an existing configured directory.
5. Resolve the installed native `vaultctl` executable once. Use `<vaultctl> handoff <operation> --vault-root <vault-root>` for every worktree and handoff operation; the executable embeds its managed policy and templates. All commands return JSON.

Complete preflight only when the vault, every input required by the selected route, source repositories, and worktree root resolve semantically and no target or Git write has occurred.

## Route

- **Prepare and materialize**: default for a request to prepare a new handoff; plan worktree and handoff effects together, authorize once, then execute both phases.
- **Worktree only**: create only the persistent work-item worktree when the user explicitly requests that narrower outcome.
- **Refresh or attach**: reuse one exact existing worktree and plan a no-op, activation, bootstrap, selective update, or a new family from the same investigation.
- **Bind development handoff**: after validation, pass one normalized materialization observation to the case-owning route in `manage-investigation`; never write the case here.
- **Validate**: verify an active or inactive handoff in one exact worktree without writing.
- **Resolve branch**: map portable repository and branch identity to a locally verified worktree without writing.
- **Set state**: update one exact handoff to `active`, `ready-for-production`, or `production` without removing it or changing another handoff.

Load [the worktree lifecycle](references/worktree-lifecycle.md) when preparing or selecting worktrees. Load [the repository-state contract](references/repository-state.md) before planning, applying, validating, or changing materialized state.

## Prepare new handoffs

1. Separate the user's selection into new standalone worktrees, new shared groups, and packages that reuse an exact existing worktree. A shared group has one first package as its worktree and branch anchor; present that shared topology before authorization and do not plan another worktree or branch for its remaining packages. Use separate worktrees when the user requires independent per-item branches.
2. For every standalone package and new shared-group anchor, inspect local branch candidates, use an already specified base branch or resolve it with the user, and derive a short description from the requested work for `issue/<work-item-token>-<slug>`. Use `issue/` by default; pass a different validated prefix only when the user explicitly requests it.
3. Run a provisional `plan-handoff` with `--base-source remote` only for those new-worktree targets. It may inspect a missing remote commit only in disposable temporary storage; present the exact local and remote commits, freshness classification, derived branch, canonical repository container, its `create-directory` effect when absent, absolute destination, and complete materialization effects.
4. When the commits differ, ask whether to start from `remote` or `local`. A remote selection may fetch only that exact base ref; it never updates the local base branch. If the user wants the local branch itself updated, stop for a separately authorized repository Git workflow.
5. Re-run `plan-handoff` with the selected source for each new-worktree target. Reject the phase if any source, remote, branch, configured root, path, managed file effect, or token is unresolved. Present its complete effects and sequential failure boundary, verify existing exact authorization or obtain it for uncovered effects, then apply every unchanged token with `prepare-handoff`.
6. After each shared-group anchor exists, use the **Plan existing handoffs** and **Apply** routes for its remaining packages and for every package assigned to a previously existing worktree. Preserve the selected path and first branch and require the same investigation and repository. Within one worktree, process one package completely before planning the next because each apply changes `ACTIVE.yaml` and invalidates every earlier token for that worktree.
7. For every validated handoff, invoke **Bind development handoff** in `manage-investigation` with the normalized in-memory observation and producer-reviewed case/story snapshots defined by the input-bundle contract. If binding fails, stop at `materialized-unbound`, preserve `ACTIVE.yaml` and all materialized evidence, and identify the exact target and case failure; do not roll back or describe that target as complete.

Skip creation when the exact existing worktree matches the repository and its `ACTIVE.yaml` names the same investigation. Complete this route when every target validates at the planned revision and its `DH-NNN` binding validates, or a partial result names prepared, worktree-created, handoff-applied, materialized-unbound, failed, and untouched targets.

## Prepare only worktrees

Use `plan-worktree` and `create-worktree` only when the user explicitly requests a worktree without materialization. Present and authorize only its Git effects, then stop after verifying its path, branch, and selected `HEAD`; a later handoff request starts the existing-worktree planning route.

Complete this route when every authorized worktree satisfies the lifecycle postconditions, or a partial result names created, failed, and untouched targets.

## Plan existing handoffs

1. Run `plan --worktree-path <exact-path>` for one package. Do not pre-plan multiple packages for the same worktree.
2. Reject the batch when any package, target registration, investigation identity, managed block, ignore rule, implementation-update changelog, or existing handoff fails validation.
   A schema-v1 single-entry `ACTIVE.yaml` is adopted into the shared registry by the next authorized materialization; do not create a separate migration record.
3. Present every exact worktree path, normalized remote, branch, action, revision, changed document, policy-only root-instruction refresh, tracked `.gitignore` effect, and ignored local effect. The current bundled managed block is always the desired policy; an exact byte match produces no effect and no Git diff.
4. Check existing user authorization against this exact file plan; obtain authorization only for uncovered effects. Apply and validate it before planning the next package for that worktree. A changed target, branch, action, document set, or token requires a new preview. Do not request another authorization merely because an unchanged complete new-handoff plan moved from its Git phase to its file phase.

Complete planning only when every target has a valid read-only plan and the user has authorized its exact effects or declined them.

## Apply

1. Apply each authorized package with its exact `--worktree-path` and approved `plan_token`; never substitute a path, token, or package.
2. After each apply, run `validate` with the same remote and worktree path and select the exact handoff ID from the returned list; record its family, revision, state, and `implementation-updates.md` entry count.
3. Invoke **Bind development handoff** in `manage-investigation` with the package source identity, exact normalized validation observation, and producer-reviewed case/story snapshots. The case owner alone reads and mutates the case. If it fails, stop at `materialized-unbound` and leave the active repository state intact. Retry from fresh repository validation and follow the input-bundle protocol for producer re-review of stale or missing snapshots.
4. On any earlier failure, stop the batch. A handled apply failure restores its prior family and registry bytes. If validation reports a pending handoff transaction, retry the exact previously authorized apply token so it restores the interrupted attempt before revalidation; do not plan another handoff in that worktree first. Report applied, failed, and untouched targets; retain successful handoffs and re-plan only the remaining work.
5. Hand the user each bound worktree path, branch, and revision so development can begin in a fresh session rooted there.

No materialization or activation is complete until its exact `DH-NNN` binding passes case validation. Complete apply only when every authorized target validates at its planned revision and is bound, or a partial result identifies the exact resume boundary without claiming rollback.

## Validate

Run `validate` for each requested remote and exact worktree path. Report `valid` with the shared investigation and every handoff's ID, family, revision, state, and implementation-update summary, `inactive` when no registry exists, or the exact failed invariant. A stale managed block is invalid; refresh it through an authorized no-content-change plan rather than editing the target manually.

For a portable investigation binding, first run `resolve-branch --repository-remote <remote> --branch <branch>`. A returned `derived_path` is diagnostic only; use a path only when availability is `local-verified`. Persist remote and branch back to the investigation, never the local path.

Describe `valid` as repository-state and package integrity, not proof that the content is sufficient to implement. When the user asks about implementation completeness, route the exact source package to the read-only **Validate** assessment in `manage-investigation`; report any materialized-content difference separately. If that source package cannot be resolved, report sufficiency as unverified with the exact missing source. Keep the consumer out of the investigation and tracker, and preserve the read-only scope.

Complete validation only when every requested worktree has an observed status and no mismatch is described as usable context.

## Set state

1. Select the exact handoff ID and requested state. Materialization, continued implementation, and reopening use `active`; an explicitly identified production candidate uses `ready-for-production`; verified deployment uses `production`. An unambiguous user lifecycle statement or correction that identifies one handoff and maps to one of these states authorizes this lifecycle-only route. It does not authorize source, Git, tracker, promotion, or deployment work.
2. `ready-for-production` requires the exact closure fingerprint from an unchanged post-write **Validate** result. `production` requires the current state to be `ready-for-production`.
3. Run `set-state` without a token. When the preview changes only the already-authorized handoff to the requested state, plus any policy refresh, repeat with the unchanged token without another confirmation. Otherwise present the single effect and ask once; apply only after the exact handoff and state are resolved.
4. Confirm the selected state changed and every other entry and family remained unchanged.

Complete the route only when the selected state is observed and every other entry remains intact, then stop. A changed snapshot, invalid transition, or stale plan leaves `ACTIVE.yaml` unchanged.

## Authority

After an exact complete handoff plan is authorized, this workflow may fetch only the selected remote base ref, create its planned local branch (`issue/...` by default), register its planned persistent worktree, create the planned destination directory under the configured root, and change only the managed block in that worktree's physical root `AGENTS.md`, physical root `CLAUDE.md`, or both, plus the root `.gitignore` rule and `.knowledge-os-handoffs/` state. A counterpart symlink is preserved and never written as a second surface. The helper keeps independent phase tokens and postconditions inside that single authorization. An explicit worktree-only plan grants only the Git subset.

It does not authorize source-code edits, tracker or investigation writes, commits, pushes, pull requests, local-base movement, pulls, merges, resets, worktree removal, branch deletion, pruning, or any other target. Keep those actions in their owning workflow and behind separate authority.
