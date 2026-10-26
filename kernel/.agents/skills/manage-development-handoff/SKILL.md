---
name: manage-development-handoff
description: Move a development investigation from agreed requirements to a task package and repository worktree, prepare its independent development session when authorized, read progress and reconcile results. Own the transition even before a package or worktree exists.
---

# Manage development handoffs

A handoff turns the part of a development investigation that belongs to one repository into a task an agent can execute alone: what to do, what changes where, how it is verified, and only the context it needs. The investigation stays in the vault; the repository gets the task.

- **Package** — `<case>/handoffs/DH-NNN.md` in the investigation (versioned with the case once published). One atomic task in one repository.
- **Worktree** — the repository's working copy on a branch (`issue/<slug>` by default), under the configured worktree root. Packages that name the same repository and branch are tasks of one milestone and share it: related changes to one service, possibly building on each other, land on one branch without a worktree per task. Its `.handoff/` holds the task copies and `deltas.md`; it is excluded from Git locally.
- **Managed segment** — a fixed section in the repository's `AGENTS.md` (and `CLAUDE.md` when it does not import `AGENTS.md`) that tells any harness how to work with `.handoff/`. It is identical in every repository, so it is committed like any shared instruction.
- **Progress** — the branch itself (commits, tests, pull request) plus `deltas.md`: changes to the definition, decisions, deviations, questions and verification results.

Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). `<CLI> handoff --help` lists the commands; all output is JSON.

## Choose the development destination

When requirements are sufficiently defined, evaluate the development destination without waiting for the user to request a handoff. A request to begin development triggers this evaluation before source edits if it has not happened yet.

Use a handoff when the task needs retained decisions, coordination across repositories or tasks, dependencies, or substantial validation that benefits from an autonomous package. Explain briefly that the package preserves scope, decisions, acceptance criteria and limits outside the conversation: another session can implement without reconstructing the investigation, and return auditable results. Finish and validate the package, then propose its worktree and independent session. Reuse existing authorization and request only what is missing. Clarifying desired behavior authorizes preparation, not implementation.

**Small-change exception.** A localized, low-risk change with clear behavior, an unambiguous authorized destination and a simple verification may proceed through [work-in-repository](../work-in-repository/SKILL.md) without a handoff. Explain why it qualifies. Judge risk, dependencies and verification, not line count; a small cross-service contract change can still need a handoff. Follow repository rules and preserve unrelated work. This exception grants no publication or deployment authority.

**Explicit destination.** When the user explicitly selects an exact existing checkout and asks to implement there, use `work-in-repository` with that destination and the existing case without redundant handoff approval. A source lookup, a clean `main`, or an agreed requirement alone does not select that destination. An existing handoff worktree follows the session preparation below.

## Write the package

In the development case ([manage-investigation](../manage-investigation/SKILL.md)), once the requirements are settled, write per repository and task what changes and how it is accepted, citing the requirements it serves (`R-NNN`, checked by the case gate):

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

## Task                — what to achieve and why, in a few sentences
## Changes             — each change: file, module or contract, and what changes
## Acceptance criteria — observable results and how to verify them
## Required context    — only the facts this task needs: contracts, events, data, rules, with permalinks
## Out of scope
## Open questions      — only those that affect this task
## References          — optional: vault remote and case id for read-only lookup
```

Write the headings in the case's language; `investigation check` accepts them in English or Spanish (`Tarea`, `Cambios`, `Criterios de aceptación`, `Contexto necesario`, `Fuera de alcance`, `Preguntas abiertas`, `Referencias`).

Keep it atomic: one repository, one task, the smallest context that answers the implementation questions. Split a milestone into tasks that can each be verified alone; a task that needs another's result names it in `depends-on` (a task of the same worktree starts after it; a task in another repository is a prerequisite whose needed result the package states). The implementing agent has no vault: write the fact or cite a permalink instead of a `[[note]]` link. Before preparing, run the [implementation-sufficiency](../manage-investigation/references/implementation-sufficiency.md) questions against the package and fill what is missing from the case. `<CLI> investigation check` gates packages too (required sections, dependencies that exist and form no cycle, no vault links, no credential or local path, atomic size). Record each package with `<CLI> investigation add --id <case> --kind handoff --package handoffs/DH-NNN.md`.

**TDD acceptance.** For testable behavior changes, put the red–green–refactor method in the package's acceptance criteria: add or extend a focused test, run it against unchanged production code and observe failure for the missing behavior; implement the minimum change, rerun to green, then refactor and run the applicable checks. Require verification evidence with commands, the relevant failing assertion and passing result in `.handoff/deltas.md`. A baseline passing suite or tests added after implementation does not demonstrate TDD. For a task without an executable behavior test, state the reason and the appropriate verification instead. Keep this instruction self-contained in the package because the implementing session has no vault skills.

## Prepare

1. `<CLI> handoff start --vault "<root>" --package <case>/handoffs/DH-NNN.md` shows the effects: worktree and branch to create from the base (the remote-tracking base when present), or the existing worktree reused when it is already on that branch; the task copy; the local exclusion; the managed segment added or updated in `AGENTS.md`.
2. Present the effects to the user and apply with `--apply` once authorized. Start the tasks of a milestone in dependency order; each one joins the same worktree, and a later task can be added while earlier ones are in progress.
3. Recommend an independent session rooted at the worktree. This lets the harness discover the repository's own instructions (`AGENTS.md` or its native equivalent), skills, MCP servers, hooks and local configuration, while the worktree isolates changes. A path in a vault subagent's prompt does not establish that context.
4. Before opening the session, apply [execution profiles](../../../90-Meta/execution-profiles.md): select model and effort for this development task, as required for subagents too. Create or reuse a session for this worktree when the user authorizes opening it and the harness supports it. Respect the requested interface; report a missing capability rather than silently substituting CLI for GUI. Otherwise deliver the worktree path and concrete opening instructions.
5. Give the session the package and authorization limits. Its initial message must also carry this self-contained execution instruction, since the repository session has no vault skills: "Before delegating a bounded task, explicitly select the least costly available model and reasoning effort that can reliably meet its acceptance criteria, considering complexity, uncertainty, risk and verification needs. Respect user preferences and repository instructions; do not inherit your own configuration automatically. Explain the choice briefly and report requested and effective model/effort when the harness exposes them; otherwise mark the effective configuration unverified. Use only supported controls and report unavailable required selections. Escalate for a concrete reasoning-quality failure, not for access or tooling failures." Preparation reads the package, confirms repository, worktree and branch, identifies applicable instructions and skills, and verifies required MCP, hooks and local configuration through available harness evidence. Report missing or unverified capabilities; a session ID alone does not prove readiness. Source code and tests remain unchanged during preparation.
6. Deliver the package, gate result, worktree status, session identifier and link or resume instructions when created, selected model/effort (effective values when observable), readiness and next action. Offer the user a choice: continue directly in the prepared sessions, or authorize the main session to orchestrate them through the follow-up workflow below. Opening it does not authorize implementation; carry forward any implementation authority already given. Keep session identifiers and local paths in runtime coordination, outside portable packages and cases.

Before directing a session, check its current activity and who has control. Keep one active execution per worktree; while the user drives it, the coordinator observes rather than starting competing work. Reuse the session on continuation. Implement from the vault only after the user chooses that alternative in response to the session recommendation; preparation and an earlier implementation request alone do not select it.

The command never commits, pushes, fetches or changes other branches. The segment change in `AGENTS.md` is a tracked file change: the repository's normal workflow commits it (once per repository, since it is identical everywhere).

## Refresh

When the package changes in the case, `<CLI> handoff status` reports `package_changed`. `<CLI> handoff refresh --vault "<root>" --worktree <path> --handoff DH-NNN` shows which sections change; apply with `--apply`. Deltas are kept; when work already done relied on the old definition, the worktree agent records a delta.

## Read progress

`<CLI> handoff status --vault "<root>" [--worktree <path>]` is read-only: per worktree, its tasks with their state (`pending`, `blocked` by an unverified dependency, `in-progress`, `verified` by a verification delta), the commits of each task (by their `Handoff:` trailer), deltas, the next task, uncommitted changes, changed packages and the segment state; all worktrees under the root when `--worktree` is omitted.

## Offer orchestration and scheduled follow-up

Explain why the main session can help: it retains the investigation's broader context while each repository session owns a bounded part. Offer to act as technical coordinator: communicate with the selected sessions, answer questions from established evidence and decisions, resolve in-scope problems, coordinate dependencies and shared contracts, and steer work back to its agreed scope. Opening sessions alone does not authorize this role or recurring execution.

Agree which sessions the coordinator may direct, whether implementation is authorized, the follow-up interval and the stopping point. Propose a two-minute interval; use the user's chosen interval, including later changes. Reuse prior explicit authorization. If the user takes control of a session, suspend automated direction of that session until control is agreed again.

Once authorized, use the harness's supported scheduler, heartbeat or equivalent to resume the coordinating session at the agreed interval. Inspect and update an existing matching schedule instead of creating duplicates. Save the session identifiers, worktrees, handoff identifiers, authorization limits, interval and stopping conditions in runtime scheduling context, outside portable packages and cases. Confirm the schedule identifier and effective interval only after successful creation or update. If the harness cannot support that interval or durable follow-up, explain the limitation and agree an available alternative; do not claim that monitoring is active.

At each follow-up, read current session activity, new messages and handoff progress before sending instructions. Check new changes and verification evidence against the package, dependencies and acceptance criteria; a status message alone does not establish correctness. Answer or redirect within the agreed scope, share relevant confirmed contract decisions between authorized sessions, and avoid duplicate prompts or competing execution in a worktree. Route material definition changes through the case and package refresh. Escalate decisions outside the agreed scope or authority to the user. Keep unchanged or non-actionable checks quiet unless the user requested periodic reports; notify on meaningful progress, completion, failure or required user action.

Stop the task's follow-up schedule when its agreed stopping point is met or the user cancels it, and confirm that it stopped before session cleanup. A blocker does not justify repeated identical interventions: report it and suspend dependent direction until new evidence or a user decision allows progress. Reconciliation and final verification still apply; orchestration grants no extra commit, publication, deployment or access authority.

## What comes back

The implementing agent records in `.handoff/deltas.md` what the cell must know: the task was incomplete or wrong (`definition`), something learned about the system (`finding`), a decision or deviation, a question, and the verification of each task. The managed segment in `AGENTS.md` carries that guide and the entry format; routine progress stays in commits.

## Reconcile into the investigation

`<CLI> handoff reconcile --vault "<root>" --worktree <path>` previews what the worktree produced since each `DH-NNN` record's last mark, with a fixed mapping: commits and `finding`, `verification`, `definition` and `deviation` deltas become evidence citing their source; decisions and questions become their records; a delta without verifiable evidence becomes a question asking for it. `--apply` writes them and moves the mark, so a second run imports nothing new.

The result lists what needs judgment: for each `definition` or `deviation`, record the requirement it changes with `add --kind requirement --supersedes R-NNN`, and update the package (then `handoff refresh`) when pending tasks depend on it. A `finding` that belongs in the vault gets a finding marked `--for-vault`. Evidence of deployment is required before any production claim; a merge alone is not deployment. The worktree is read only; reconciling never closes the investigation.

## Offer session cleanup

After the agreed work, requested reviews and refinements, reconciliation, and any authorized commit or publication steps are complete, offer to archive the independent sessions the coordinator created for this handoff. This keeps the user's sidebar clear while preserving resumable history. A passing test or an idle session alone does not mark the work complete; leave sessions available while follow-up work remains.

Name the sessions and their identifiers in the offer. Archive only those the user authorizes, reusing any explicit cleanup authorization already given, after checking that no execution or user continuation is active. Use the harness's supported reversible archive or close action; if unavailable, provide manual closing instructions and report that cleanup was not performed. Preserve the coordinating session, unrelated sessions, transcripts, worktrees, branches and uncommitted changes. Session cleanup grants no authority to delete repository state or close the investigation. Confirm which sessions were archived and which remain open.

## Guardrails

- The repository's own instructions prevail over the task; the task never authorizes commits, pushes, pull requests, deployment or tracker writes.
- Never copy credentials into a package or `.handoff/`; name where they live instead.
- Keep local paths out of packages and cases; the worktree path is runtime information.
