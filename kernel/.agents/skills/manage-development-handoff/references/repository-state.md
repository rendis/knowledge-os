# Development handoff repository state

Load this reference for `plan`, `apply`, `validate`, and `deactivate` after the worktree lifecycle has selected one exact target.

## Contents

- [Materialized layout](#materialized-layout)
- [Identity and revisions](#identity-and-revisions)
- [Managed repository surfaces](#managed-repository-surfaces)
- [Plan and apply](#plan-and-apply)
- [Validate](#validate)
- [Deactivate](#deactivate)
- [Completion criterion](#completion-criterion)

## Materialized layout

Each target worktree stores one readable current state plus append-only revision events:

```text
.knowledge-os-handoffs/
├── ACTIVE.yaml
└── issue-3812--schema-repository/
    ├── handoff.yaml
    ├── START.md
    ├── jira.md
    ├── context.md
    ├── scope.md
    ├── implementation-updates.md
    └── history/
        ├── v0001.md
        └── v0002.md
```

There is no directory per revision. An agent reads stable current paths, then the exact `history/vNNNN.md` event referenced by `handoff.yaml`.

## Identity and revisions

The stable identity is:

```text
normalized Jira site + uppercase issue key + normalized repository remote
```

The visible family is `<issue-key-lower>--<repository-basename-lower>`. `handoff.yaml` stores the full SHA-256 identity and normalized remote; an existing visible family with another identity is a collision and blocks the operation.

Each material export compares `jira.md`, `context.md`, and `scope.md` independently:

- Same semantic hashes: no new revision and no rewritten document.
- One or more changed hashes: increment `vNNNN`, write only changed documents, append one event, and update `handoff.yaml` and `ACTIVE.yaml`.
- Same content while another family is active: reactivate the existing revision without creating history.

`START.md` is stable bootstrap material. `history/v0001.md` records the baseline and hashes without duplicating every source document. Later events record the reason, changed and unchanged paths, old/new semantic hashes, and unified diffs for changed documents only. Each event stores the previous event's SHA-256; `handoff.yaml` anchors the current event, making the entire history chain tamper-evident.

`implementation-updates.md` is deliberately outside the content revision and hash chain. It is the single append-only changelog for material changes to the exported definition discovered during any later activity. Its entries use contiguous `UPD-NNN` IDs and the mandatory fields defined in the managed root-instruction block. Existing entries are never edited, deleted, reordered, or renumbered; a later entry names what it supersedes. `Analysis` is present only when analysis actually occurred. A missing legacy changelog is a bootstrap effect and never increments `vNNNN`.

History events are immutable. `handoff.yaml` is the current integrity index and `ACTIVE.yaml` is the worktree-local active pointer. It answers which exact handoff revision governs the current implementation session without scanning every retained family. One worktree has at most one active handoff; concurrent stories use separate worktrees and therefore separate pointers.

## Managed repository surfaces

The workflow owns only these target paths:

- the exact managed block in every physical root instruction file selected from `AGENTS.md` and `CLAUDE.md`;
- the exact root rule `/.knowledge-os-handoffs/` in `.gitignore`;
- `.knowledge-os-handoffs/`.

The managed block uses these markers:

```text
<!-- knowledge-os:managed:start id="development handoff" -->
<!-- knowledge-os:managed:end id="development handoff" -->
```

Select the physical instruction surfaces from the filesystem state observed on the target OS. When one root file is a symlink directly to the other physical root file, preserve the symlink and manage only its physical target. When both are physical files, manage both. When neither exists, create `AGENTS.md`. A broken, external, chained, or mutually recursive instruction symlink blocks the operation.

Zero marker pairs means insert the bundled block after root frontmatter and the first H1 when present. Keep this near the beginning rather than appending it: the handoff preflight remains inside the instruction budget, while the repository's own detailed guidance follows it and retains local authority. One complete current pair means replace only that pair. One complete legacy `BEGIN MANAGED`/`END MANAGED` pair is migrated in place to the current markers. In all cases, when repository content follows the end marker, leave at least one blank line before that content. Duplicate, incomplete, mixed, or inverted markers block the operation. A non-empty root `AGENTS.override.md` also blocks because it hides the root instruction contract.

Preserve encoding, line endings, file mode, existing separator space, symlink topology, and every other byte outside the managed block. Keep each resulting instruction file within 32 KiB. Add the ignore rule without reordering or rewriting existing rules; a negation or effective Git-ignore conflict blocks.

Reconcile the managed block from the current bundled asset during every mutating handoff operation. Compare exact desired bytes before writing: an identical block is a no-op and must not change bytes, mtime, or Git status. A policy-only refresh is `bootstrap`, preserves the current content revision, and creates no history event.

Generated baseline files are read-only to the implementation agent. `implementation-updates.md` is the only writable handoff file and records definition deltas, not routine progress or a duplicate implementation diary. Implementation evidence stays in normal repository code, tests, diff, branch, pull request, and delivery artifacts.

## Plan and apply

For a new worktree and initial handoff, `plan-handoff` projects this file plan from the selected base commit and combines it with the Git effects. `prepare-handoff` consumes the one approved complete token, creates the worktree, and requires the real `plan` token below to equal that projection before applying any file effect. Keep the standalone commands for refresh, activation, recovery from a `worktree-created` boundary, and legacy maintenance.

Run:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> plan --bundle <package-dir> \
  --worktree-path <absolute-worktree-path>
```

`plan` is read-only. It resolves `repository.remote` through `90-Meta/workspace-config.py locate-repository`, verifies that the explicit path is a registered worktree under the configured root and attached to the Jira branch, validates current state, and returns the target, branch, action, revision, changed documents, exact effects, tracked status, and a `plan_token`.

For existing worktrees, plan every package in a multi-repository handoff before the first write. Present one consolidated effect plan and obtain explicit authorization for the exact targets and effects. Then apply each unchanged plan:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> apply --bundle <package-dir> \
  --worktree-path <absolute-worktree-path> --plan-token <approved-token>
```

`apply` recomputes the plan immediately. Any bundle, target branch or `HEAD`, instruction, ignore, active-pointer, changelog, or handoff-state change invalidates the token before writing. Current documents and pointers use atomic file replacement; `handoff.yaml` and `ACTIVE.yaml` are written last so readers can detect and retry an incomplete concurrent view. Never overwrite an existing `implementation-updates.md`.

Multi-repository preparation and application are sequential and have no broad rollback. If a later repository fails, validate and report every prepared, worktree-created, handoff-applied, failed, and untouched target; resume only from newly planned tokens.

## Validate

Run:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> validate --repository-remote <git-remote> \
  --worktree-path <absolute-worktree-path>
```

Validation is read-only. It checks the current exact managed policy, effective ignore behavior, active identity, current revision, contiguous history, normalized remote, every raw and semantic baseline hash, and the changelog's canonical H1, contiguous IDs, required fields, allowed states, backward-only entry references, and secret scan. It cannot prove that no earlier entry was rewritten; closure reconciliation detects missing semantic coverage against repository and Jira evidence. `inactive` is valid when the bootstrap remains but `ACTIVE.yaml` is absent.

## Deactivate

Preview deactivation without a token:

```text
<python> -B <skill-dir>/scripts/development-handoff.py \
  --vault-root <vault-root> deactivate --repository-remote <git-remote> \
  --worktree-path <absolute-worktree-path>
```

After explicit authorization, repeat with the returned `--plan-token`. Deactivation removes `ACTIVE.yaml` and, when stale, replaces only the exact managed policy block in the same approved plan. It preserves the ignore rule, materialized family, changelog, and complete history. If the pointer is already absent but policy is stale, the action is `refresh-policy`; exact current state is a no-op. Reapplying the same package later activates the existing revision.

## Completion criterion

A plan is complete when every exact worktree, branch, revision, and effect is visible. Apply is complete when every authorized worktree validates at its planned revision, or a partial result names applied, failed, and untouched targets without claiming rollback. Deactivation is complete when that worktree's pointer is absent and all family history remains intact.
