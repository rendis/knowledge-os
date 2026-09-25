---
name: manage-development-handoff
description: Prepare a repository worktree for atomic development tasks from a development investigation, so an agent working there knows exactly what to do without further context; read their progress; and reconcile what was implemented back into the investigation. Use to prepare, refresh, check or reconcile a handoff.
---

# Manage development handoffs

A handoff turns the part of a development investigation that belongs to one repository into a task an agent can execute alone: what to do, what changes where, how it is verified, and only the context it needs. The investigation stays in the vault; the repository gets the task.

- **Package** — `<case>/handoffs/DH-NNN.md` in the investigation (versioned with the case once published). One atomic task in one repository.
- **Worktree** — the repository's working copy on a branch (`issue/<slug>` by default), under the configured worktree root. Packages that name the same repository and branch are tasks of one milestone and share it: related changes to one service, possibly building on each other, land on one branch without a worktree per task. Its `.handoff/` holds the task copies and `deltas.md`; it is excluded from Git locally.
- **Managed segment** — a fixed section in the repository's `AGENTS.md` (and `CLAUDE.md` when it does not import `AGENTS.md`) that tells any harness how to work with `.handoff/`. It is identical in every repository, so it is committed like any shared instruction.
- **Progress** — the branch itself (commits, tests, pull request) plus `deltas.md`: changes to the definition, decisions, deviations, questions and verification results.

Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). `<CLI> handoff --help` lists the commands; all output is JSON.

## Write the package

In the development case ([manage-investigation](../manage-investigation/SKILL.md)), once requirements, changes by component and acceptance criteria for a repository are settled:

```markdown
---
handoff: DH-001
case: <case id>
repository: <git remote of the repository>
base: <base branch, e.g. main>
branch: issue/<short-slug>       # the same branch for every task of the milestone
depends-on: DH-NNN               # optional: tasks this one builds on
---

# <The task in one line>

## Tarea            (Task)                — what to achieve and why, in a few sentences
## Cambios          (Changes)             — each change: file, module or contract, and what changes
## Criterios de aceptación (Acceptance criteria) — observable results and how to verify them
## Contexto necesario (Required context)  — only the facts this task needs: contracts, events, data, rules, with permalinks
## Fuera de alcance (Out of scope)
## Preguntas abiertas (Open questions)     — only those that affect this task
## Referencias      (References)          — optional: vault remote and case id for read-only lookup
```

Keep it atomic: one repository, one task, the smallest context that answers the implementation questions. Split a milestone into tasks that can each be verified alone; a task that needs another's result names it in `depends-on` (a task of the same worktree starts after it; a task in another repository is a prerequisite whose needed result the package states). The implementing agent has no vault: write the fact or cite a permalink instead of a `[[note]]` link. Before preparing, run the [implementation-sufficiency](../manage-investigation/references/implementation-sufficiency.md) questions against the package and fill what is missing from the case. `<CLI> investigation check` gates packages too (required sections, dependencies that exist and form no cycle, no vault links, no credential or local path, atomic size). Record each package with `<CLI> investigation add --id <case> --kind handoff --package handoffs/DH-NNN.md`.

## Prepare

1. `<CLI> handoff start --vault "<root>" --package <case>/handoffs/DH-NNN.md` shows the effects: worktree and branch to create from the base (the remote-tracking base when present), or the existing worktree reused when it is already on that branch; the task copy; the local exclusion; the managed segment added or updated in `AGENTS.md`.
2. Present the effects to the user and apply with `--apply` once authorized. Start the tasks of a milestone in dependency order; each one joins the same worktree, and a later task can be added while earlier ones are in progress.
3. Hand over the worktree path: the user (or `work-in-repository`) opens a new agent session there. Preparing a handoff does not authorize implementing it.

The command never commits, pushes, fetches or changes other branches. The segment change in `AGENTS.md` is a tracked file change: the repository's normal workflow commits it (once per repository, since it is identical everywhere).

## Refresh

When the package changes in the case, `<CLI> handoff status` reports `package_changed`. `<CLI> handoff refresh --vault "<root>" --worktree <path> --handoff DH-NNN` shows which sections change; apply with `--apply`. Deltas are kept; when work already done relied on the old definition, the worktree agent records a delta.

## Read progress

`<CLI> handoff status --vault "<root>" [--worktree <path>]` is read-only: per worktree, its tasks with their state (`pending`, `blocked` by an unverified dependency, `in-progress`, `verified` by a verification delta), the commits of each task (by their `Handoff:` trailer), deltas, the next task, uncommitted changes, changed packages and the segment state; all worktrees under the root when `--worktree` is omitted. Worktrees prepared by earlier versions (`.knowledge-os-handoffs/`) are listed with their legacy store and their `implementation-updates.md` entries read as deltas; the legacy verbs keep working for them.

## Reconcile into the investigation

1. `handoff status --worktree <path>`; per task, take the commits and deltas after the `DH-NNN` record's last reconciliation mark.
2. Record each in the case with `investigation add`: implemented changes as evidence citing commits or pull requests; changed definitions as new requirement, change or acceptance records that `--supersedes` the old ones; decisions and questions as their records. Evidence of deployment is required before any production claim; a merge alone is not deployment.
3. Mark the point read with `--reconciles DH-NNN --through "<commit> / DELTA-NNN"` on the last evidence record, and answer with what changed in the case. The worktree is read only; reconciling never closes the investigation by itself.

## Guardrails

- The repository's own instructions prevail over the task; the task never authorizes commits, pushes, pull requests, deployment or tracker writes.
- Never copy credentials into a package or `.handoff/`; name where they live instead.
- Keep local paths out of packages and cases; the worktree path is runtime information.
