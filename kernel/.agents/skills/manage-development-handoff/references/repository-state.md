# Development handoff repository state

Load this reference for `plan`, `apply`, `validate`, and `set-state` after the worktree lifecycle has selected one exact target.

## Contents

- [Materialized layout](#materialized-layout)
- [Identity and revisions](#identity-and-revisions)
- [Managed repository surfaces](#managed-repository-surfaces)
- [Plan and apply](#plan-and-apply)
- [Validate](#validate)
- [Set state](#set-state)
- [Completion criterion](#completion-criterion)

## Materialized layout

Each target worktree stores one registry plus one complete family per work item:

```text
.knowledge-os-handoffs/
├── .ACTIVE.lock
├── .APPLY.transaction/        # only while an apply is incomplete
├── ACTIVE.yaml
├── delivery-br-3812-a1b2c3d4e5--schema-repository/
    ├── handoff.yaml
    ├── START.md
    ├── work-item.md
    ├── context.md
    ├── scope.md
    ├── implementation-updates.md
    └── history/
        ├── v0001.md
        └── v0002.md
└── delivery-br-3813-f6e7d8c9b0--schema-repository/
    └── ...
```

There is no directory per revision. An agent reads stable current paths, then the exact `history/vNNNN.md` event referenced by `handoff.yaml`.
`.ACTIVE.lock` and `.APPLY.transaction/` are operational safeguards only; they carry no handoff state or history. The transaction snapshots only paths that the current apply may change and is removed after commit or rollback.

## Identity and revisions

The stable identity is:

```text
exact tracker ID + exact provider-native reference + normalized repository remote
```

The visible family is `<readable-work-item-slug>-<identity-digest>--<repository-basename-lower>`. The ten-character digest is derived from the full stable identity, so different native references that normalize to the same slug remain distinct. `handoff.yaml` stores the full SHA-256 identity and normalized remote; an existing visible family with another identity is a collision and blocks the operation.

Each material export compares `work-item.md`, `context.md`, and `scope.md` independently:

- Same semantic hashes: no new revision and no rewritten document; when the selected entry is not `active`, reactivate it without creating history.
- One or more changed hashes: increment `vNNNN`, write only changed documents, append one event, and update `handoff.yaml` and `ACTIVE.yaml`.

`START.md` is stable bootstrap material. `history/v0001.md` records the baseline and hashes without duplicating every source document. Later events record the reason, changed and unchanged paths, old/new semantic hashes, and unified diffs for changed documents only. Each event stores the previous event's SHA-256; `handoff.yaml` anchors the current event, making the entire history chain tamper-evident.

`implementation-updates.md` is deliberately outside the content revision and hash chain. It is the single append-only changelog for material changes to the exported definition discovered during any later activity. Its entries use contiguous `UPD-NNN` IDs and the mandatory fields defined in the managed root-instruction block. Existing entries are never edited, deleted, reordered, or renumbered; a later entry names what it supersedes. `Analysis` is present only when analysis actually occurred. The file is created with a new family; its absence from an existing family is invalid.

History events are immutable. Each `handoff.yaml` is its family's integrity index. `ACTIVE.yaml` is the worktree-local lifecycle source of truth for one investigation and lists every handoff's exact revision and current state: `active` permits implementation, `ready-for-production` identifies the selected production candidate, and `production` records verified deployment. `activated-at` records the latest materialization or state transition that placed that entry in `active`. A shared worktree never combines investigations or repositories.

A schema-v1 `ACTIVE.yaml` is accepted only as its original single active handoff. The next materialization or state change writes the schema-v2 registry; no separate migration artifact or history is created.

## Managed repository surfaces

New plans own only these target paths:

- the exact managed block in the physical root `AGENTS.md`;
- the exact root rule `/.knowledge-os-handoffs/` in `.gitignore`;
- `.knowledge-os-handoffs/`.

The managed block uses these markers:

```text
<!-- knowledge-os:managed:start id="development handoff" -->
<!-- knowledge-os:managed:end id="development handoff" -->
```

Manage only the physical root `AGENTS.md`; create it when absent. Preserve existing `CLAUDE.md` files and links, including any earlier managed blocks, without reading, validating, or updating them as handoff policy. An `AGENTS.md` symlink or other non-regular entry blocks the operation rather than writing through it or replacing it. The repository owner controls agent configuration and instruction discovery. Recovery of a previously authorized interrupted transaction may restore its recorded `CLAUDE.md` preimage, or remove that file if the transaction created it. This exception requires the exact interrupted token and unchanged recorded content; subsequent user edits block recovery and remain preserved. It does not add `CLAUDE.md` to new plans.

Zero marker pairs means insert the bundled block after root frontmatter and the first H1 when present. Keep this near the beginning rather than appending it: the handoff preflight remains inside the instruction budget, while the repository's own detailed guidance follows it and retains local authority. One complete current pair means replace only that pair. Any obsolete, duplicate, incomplete, mixed, or inverted marker set blocks the operation. In all cases, when repository content follows the end marker, leave at least one blank line before that content. A non-empty root `AGENTS.override.md` also blocks because it hides the root instruction contract.

Preserve encoding, line endings, file mode, existing separator space, symlink topology, and every other byte outside the managed block. Keep each resulting instruction file within 32 KiB. Add the ignore rule without reordering or rewriting existing rules; a negation or effective Git-ignore conflict blocks.

Reconcile the managed block from the current bundled asset during every mutating handoff operation. Compare exact desired bytes before writing: an identical block is a no-op and must not change bytes, mtime, or Git status. A policy-only refresh is `bootstrap`, preserves the current content revision, and creates no history event.

Generated baseline files are read-only to the implementation agent. `ACTIVE.yaml` changes only through `set-state`; it is never edited freehand. `implementation-updates.md` is the only handoff file the implementation agent edits directly and records definition deltas, material implementation decisions, deviations, questions, and evidence references. Implementation evidence stays in normal repository code, tests, diff, branch, pull request, and delivery artifacts.

## Plan and apply

For a new worktree and initial handoff, `plan-handoff` projects this file plan from the selected base commit and combines it with the Git effects. `prepare-handoff` consumes the one approved complete token, creates the worktree, and requires the real `plan` token below to equal that projection before applying any file effect. Keep the standalone commands for refresh, activation, and recovery from a `worktree-created` boundary.

Run:

```text
<vaultctl> handoff plan --vault-root <vault-root> --bundle <package-dir> \
  --worktree-path <absolute-worktree-path>
```

`plan` is read-only. It resolves `repository.remote` through `vaultctl config locate --vault <vault-root> --remote <remote>`, verifies that the explicit path is a registered worktree under the configured root and attached to a branch, and requires the first handoff's collision-safe work-item token to match on every operation. Later handoffs preserve that anchor branch and must match the registry investigation; selecting one never authorizes a branch or worktree change. The plan validates current state and returns the target, branch, action, revision, changed documents, exact effects, tracked status, and a `plan_token`.

For existing worktrees, `plan` may attach a new family only when `ACTIVE.yaml` names the package's investigation. It preserves the existing branch and every registered family. Present the exact registry and family effects, then apply each unchanged plan:

```text
<vaultctl> handoff apply --vault-root <vault-root> --bundle <package-dir> \
  --worktree-path <absolute-worktree-path> --plan-token <approved-token>
```

`apply` serializes one worktree, restores any interrupted apply, and then recomputes the approved plan. Any bundle, target branch or `HEAD`, instruction, ignore, registry, changelog, or handoff-state change invalidates the token before writing. Before its first write, it snapshots only the exact root instructions, ignore file, family files, and registry file that operation may touch and binds that snapshot to the approved token, exact bundle, and handoff. Before each write it records that file's expected hash. A handled write or validation failure restores those files immediately; an interrupted process leaves the snapshot so only the exact authorized apply can restore it before token validation. Recovery accepts a planned path only when it still has its prior content or that exact expected hash; other edits and unplanned content block recovery and remain preserved for explicit resolution. Current files still use atomic replacement, and successful validation marks the transaction committed before cleanup. Applying a changed or reopened family sets only that entry to `active`. Never overwrite an existing `implementation-updates.md`.

Multi-repository preparation and application are sequential and have no broad rollback. If a later repository fails, validate and report every prepared, worktree-created, handoff-applied, failed, and untouched target; resume only from newly planned tokens.

## Validate

Run:

```text
<vaultctl> handoff validate --vault-root <vault-root> --repository-remote <git-remote> \
  --worktree-path <absolute-worktree-path>
```

Validation is read-only. It checks the managed policy, ignore behavior, anchor branch, shared investigation, unique registry identities, every current revision and family, normalized remote, history, hashes, and changelogs. It returns one result per handoff, including state and a `closure_fingerprint` binding that family, the complete `ACTIVE.yaml`, `HEAD`, Git status, binary diff, and non-ignored untracked content. `inactive` is valid when the bootstrap remains but `ACTIVE.yaml` is absent.

## Set state

Preview one state update without a token:

```text
<vaultctl> handoff set-state --vault-root <vault-root> --repository-remote <git-remote> \
  --worktree-path <absolute-worktree-path> \
  --handoff-id <handoff-id> \
  --state <active|ready-for-production|production> \
  [--closure-fingerprint <sha256>]
```

An unambiguous current user request that identifies one handoff and maps to the requested state authorizes that exact lifecycle change. When the preview matches it, repeat with the returned `--plan-token` without another confirmation; otherwise present the effect and ask once. A change to `ready-for-production` requires an `active` entry and the exact current closure fingerprint; `production` requires `ready-for-production`; `active` reopens any selected entry. Repeating the current state is a no-op. Apply recomputes the plan, so mutation during authorization fails as `plan_stale`. Only the selected state and any stale managed policy change; families and other registry entries remain intact.

## Completion criterion

A plan is complete when every exact worktree, branch, revision, registry change, and effect is visible. Apply is complete when every authorized worktree validates at its planned revisions, or a partial result names applied, failed, and untouched targets without claiming rollback. A state update is complete when only the selected entry changed and validation preserves every family.
