# Development handoff worktree lifecycle

Load this reference when preparing a new handoff worktree or selecting an existing one.

## Configuration and layout

Resolve the destination only through:

```text
<python> -B <vault-root>/90-Meta/workspace-config.py \
  --vault-root <vault-root> development-worktree-root --format json
```

If the view returns `worktree_root_not_configured` or `worktree_root_unavailable`, invoke `configure-workspace`, let it own the repair, and resume only after the semantic view succeeds. Never parse `.knowledge-os-config.yaml`.

The helper derives:

```text
branch: <issue-prefix>/<work-item-token>-<description-slug>
path:   <worktree-root>/<remote-repository-basename>/<work-item-token>-<description-slug>
```

Derive `remote-repository-basename` from the configured URL of the matching Git remote in the configuration-resolved source checkout after removing an optional `.git` suffix. Preserve that URL's basename casing for the filesystem path; use the normalized remote only for identity comparison. The result is independent of the local checkout directory name. When that canonical repository container is absent, include its `create-directory` effect in the plan and create it only with the authorized worktree operation. Reuse an existing container only when it is a real directory outside any Git worktree; reject files, symlinks, and Git worktree parents.

The default issue prefix is `issue/`. Ask the user for the short description; do not infer and silently accept a branch name. Pass `--branch-prefix <prefix>` only when the user explicitly requests another valid prefix. Present the derived branch, canonical repository container, and absolute path before creation. A collision requires a new user-confirmed description or selection of the exact existing worktree for refresh; never delete, move, prune, or reuse a conflicting path automatically.

## Base selection and freshness

Inspect local branches read-only. Suggest `main` when it exists and `master` only when `main` does not; do not guess when both, neither, or deployment evidence make the baseline ambiguous. Obtain the user's base-branch confirmation.

Run a provisional complete remote plan to compare the exact local and remote commits and project the initial handoff files:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> plan-handoff --bundle <package-dir> \
  --base-branch <branch> --base-source remote --description <short-description> \
  [--branch-prefix <prefix>]
```

Interpret `base.freshness` as follows:

- `current`: the local and remote base refs identify the same commit.
- `local_missing`: only the remote base exists.
- `local_differs_from_remote`: the refs differ; the helper does not claim whether the local ref is merely behind, ahead, or diverged.

When the refs differ, show both commits and ask whether to start from the exact remote commit or the exact local commit. Selecting `remote` authorizes a targeted fetch during creation when required; it does not move, merge, reset, pull, or switch the local base branch. Selecting `local` preserves its current commit. If the user wants the local base branch itself updated, stop and hand that separate Git operation to the repository's normal workflow before re-planning.

Re-run `plan-handoff` with the selected `--base-source`. Plan every repository before the first persistent Git write, present the exact fetch/ref/path and materialization effects together, and obtain one explicit authorization for the complete plan. When the exact remote commit is absent locally, planning inspects it in disposable temporary storage and leaves the source repository unchanged.

## Complete preparation

Prepare each unchanged approved plan with:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> prepare-handoff --bundle <package-dir> \
  --base-branch <branch> --base-source <local|remote> \
  --description <short-description> [--branch-prefix <prefix>] \
  --plan-token <approved-token>
```

The helper revalidates the complete token before any persistent write. It then performs the Git phase, rebuilds the file plan from the created worktree, requires its token to equal the authorized projection, applies the handoff, and validates it. A mismatch stops before file writes. A materialization failure preserves the created worktree and reports `worktree-created`; a later validation failure preserves the materialized handoff and reports `handoff-applied`. Both are exact resume boundaries and neither broadens authority into destructive rollback.

## Worktree-only creation

Use this narrower route only when the user explicitly requests a persistent worktree without its handoff package. Create each unchanged approved plan with:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> create-worktree --bundle <package-dir> \
  --base-branch <branch> --base-source <local|remote> \
  --description <short-description> [--branch-prefix <prefix>] \
  --plan-token <approved-token>
```

For a remote base, the only permitted acquisition is the exact approved commit object. The helper updates `refs/remotes/<matching-remote>/<base-branch>` to that commit with a compare-and-swap guard, then creates the planned local branch (`issue/...` by default) and one persistent worktree at the approved path. It verifies the resulting branch and `HEAD`.

Complete preparation and worktree-only creation do not authorize source edits, commits, pushes, pull requests, merges, resets, local-base movement, worktree removal, branch deletion, or pruning. A later cleanup is a separate repository operation with its own exact effects and authorization.

Multi-repository creation is sequential and has no broad rollback. On failure, report created, failed, and untouched worktrees and re-plan only the unfinished targets.

## Existing worktrees

Refresh, attachment, validation, and state updates use the exact absolute `--worktree-path`. The helper verifies that it is under the configured root, follows `<root>/<remote-repository-basename>/<work-item-description>`, uses the canonical repository basename derived from the supplied remote, is registered to the repository resolved by that remote, and is attached to a branch. The first handoff defines the anchor branch and must match its collision-safe work-item token; later handoffs reuse that worktree and branch only when `ACTIVE.yaml` names their same investigation. Selecting a later handoff is not a branch operation.

Every refresh, attachment, validation, and state update requires the exact absolute worktree path. A missing or non-canonical path blocks the operation; the helper does not search historical checkouts or relocate state.

## Completion criterion

Complete preparation finishes when every authorized worktree exists under its canonical remote-derived repository container at the planned path, is attached to its planned branch, its `HEAD` equals the selected local or remote commit, and its handoff validates at the planned revision. Worktree-only creation finishes at the Git postconditions. A partial result is complete only as a resume boundary that names prepared, worktree-created, handoff-applied, failed, and untouched targets.
