---
name: synchronize-ecosystem
description: "Trigger: inventory, synchronize repository maps, or run a manual or scheduled vault refresh. Queries and single-note documentation belong to map-ecosystem."
license: Apache-2.0
metadata:
  author: knowledge-os maintainers
  version: "2.0"
---

# Synchronize the vault

A sync brings repository notes to the current production commit of their sources. The run is a local Git branch; commits are its checkpoints; the review is a commit that binds the accepted content; publication is a fast-forward merge plus the team's Git policy. Nobody is assumed to review by hand, so the gates and the independent agent review are mandatory.

Use this workflow for an authorized synchronization or publication that needs it. Starting, resuming or updating an investigation, including a published case or handoff preparation and reconciliation, uses its case directory in the existing checkout and does not start sync. Follow the router's branch and checkout exception rule before a sync changes vault Git state.

## Hard rules

- Sources, platforms and databases are read-only evidence. Write to the vault only with vault-update authority; publish to the remote only through the repository's Git policy (`manage-git-workflow`).
- Every technical claim follows `90-Meta/evidence-policy.md` and the evidence format in `../map-ecosystem/references/repository-map.md`.
- One author writes the resulting notes; one fresh reviewer judges them (`../map-ecosystem/references/final-note-review.md`). The author never records its own review.
- A changed byte after an accepted review needs review of that change; a failed gate is fixed in the note, never by re-running source analysis.

## Pending-work queries

Answer a pending-sync query read-only against the vault's published base, accounting for a checkout on another branch and uncommitted acknowledgements. State which revision was inspected. Read inventory outcomes as separate kinds of work:

- **New source revisions** (`new`, `changed`): analyze the selected reference branch and its delta from the last successful baseline.
- **Unresolved reviews** (`acknowledged-review-rejected`, `acknowledged-inspection-limited`): resume the review or missing inspection at the observed branch and SHA. A matching note SHA does not make an unsuccessful attempt accepted. Report this work even when `new` and `changed` are both zero.
- **No analyzable source**: when a reference is missing, check whether the repository is empty or the configured branch is absent. Report the exact condition separately; an empty repository has no commit to synchronize and is not an executable sync task.

Give each count separately and recommend the next executable task. Accepted acknowledgements close their branch and SHA. A selected reference containing only a title or scaffold can close as `no-durable-node` through the normal review and publication gates; implementation on another branch does not change the reference policy. See [reference branches](../../../90-Meta/reference-branches.md).

## Pending published-case edits

Local edits to a published investigation do not authorize publication. Once publication is separately authorized, inspect the current branch, index and pending files, explain the branch effects and reuse an applicable sync run. Otherwise `<cli> sync start --vault "<vault>" --name <slug> --case <ID>` explicitly transfers pending edits in `investigations/<ID>/` into a new sync run from the currently checked-out base. It preserves pending content and the index; it does not commit, stash, discard changes or create a worktree. It refuses pending knowledge changes outside the selected case, unrelated staged files and a requested base different from the current branch. Preserve a refusal's pending work; resolve its scope or authority rather than bypassing the checks.

After the transition, stage and commit only the selected case files, obtain independent review and complete the normal `sync review`, `sync verify` and `sync finish` workflow. The exception changes only the clean-knowledge precondition for starting the run. An unpublished ignored case uses normal clean sync first, then moves into `investigations/`; local case edits never start either workflow automatically.

## Recipe

1. **Bind.** Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). Normal `sync start` requires a clean knowledge tree. The explicit `--case <ID>` transition above permits only the selected published case's pending knowledge changes under its stated constraints. Inspect upstream state and follow repository policy without disturbing pending work; preserve the current branch rather than switching it to prepare publication.
2. **Select.** For case-only publication, inspect the selected case; source synchronization and discovery are outside that scope. For repository synchronization, read `<cli> inventory --vault "<vault>" --format markdown` using **Pending-work queries** above; select new revisions and unresolved reviews for the authorized run. `<cli> discover run --vault "<vault>"` computes facts at each repository's [reference branch](../../../90-Meta/reference-branches.md) as the local checkout knows it, so `git fetch` the selected source checkouts first (a read that updates only remote-tracking refs); add `discover platform --referenced` when the user authorizes cloud reads. Pick small independent groups; a slow repository never holds the others.
3. **Branch.** Inspect `sync status`, the current branch and pending work; reuse an applicable sync run. Otherwise explain the required branch effects and reuse applicable authority; request only missing authority before `<cli> sync start --vault "<vault>" --name <slug>`, adding `--case <ID>` only for the explicit transition above. The session needs write access to the vault's `.git` and `.agents/state/`; use the harness's supported approval route when access is missing. Work in the bound vault's existing checkout under the router's exception rule. A protected-branch PR can justify an authorized branch; it does not automatically justify another checkout. Preserve the current branch and unrelated work rather than switching a live checkout to the primary branch as preparation.
4. **Author.** For case-only publication, follow [manage-investigation](../manage-investigation/SKILL.md#publish-absorb-and-retire) and stage and commit only its case files. For each repository in a synchronization: read the current note, the delta since `commit-analizado` (`git diff`), the facts and `discover check`'s stale cited files. Write the complete resulting note following repository-map.md; update `commit-analizado`, `fecha-analisis` and `rama-analizada`. Create or update topic/event/integration notes the facts require. A corrected or retracted claim is also fixed where neighbouring notes repeat it: check the notes that link to each changed note or that it links to (`[[<stem>]]`). `sync verify` lists `stale_neighbours`, notes that cite lines of the synced repository that changed since their anchor's commit; update each claim against the new code and re-anchor it at the new commit. A repository whose delta changes nothing durable gets `sync acknowledge --decision no-documentation-change`; a new repository without a durable role gets `no-durable-node`. Commit.
5. **Gate.** Run `<cli> investigation check --vault "<vault>" --id <ID>` for each changed case. For each changed repository note, `<cli> discover check --vault "<vault>" --note <note>` passes with no `error`; pending items appear in the note's `Verificaciones pendientes`.
6. **Review.** Dispatch a fresh reviewer (`evidence-reviewer`, or another agent session) with final-note-review.md, the branch diff (`git diff <base>...HEAD`), the facts and the check output. On `revise`, correct only the cited text, commit, re-check, and re-review the change. On `accept`, record it: `<cli> sync review --vault "<vault>" --verdict accept --reviewer "<reviewer>" --summary "<one line>"`.
   A branch that only records discovery state (`90-Meta/discovery/`: classifications, platform snapshots) needs no knowledge review; `sync verify` validates it.
7. **Verify and publish.** `<cli> sync verify --vault "<vault>"` must return `ok` (note gates, no stale neighbour note, no structural issue introduced versus the base, review covering the final content). `<cli> sync finish --vault "<vault>"` fast-forwards the base. Push or open a PR as the repository policy says, and verify the remote ref.

## Concurrency and recovery

- Serialize vault writes in the existing checkout. Before starting or switching branches, inspect `sync status` and local changes. Resume the same run when applicable. If another task owns the branch or pending changes prevent the requested operation, preserve them and continue independent read-only preparation; ask only when the authorized workflow cannot proceed without a user decision. Do not stash, discard, move work, or create a vault worktree or replacement clone as an automatic workaround; any exception follows the router's concrete-need and authority rule. Existing vault worktrees remain untouched unless the user separately authorizes their handling.
- Keep the base current with `<cli> sync pull --vault "<vault>"`: it fast-forwards when possible and never merges meaning automatically.
- Base moved (another developer published): `sync pull` lists the knowledge files changed on both sides. Rebase the branch; for each listed file re-apply this run's facts onto the upstream note, commit, re-run the gates and get the merged meaning reviewed. Never force-push.
- The gates run locally, in this flow: the kernel ships no CI workflow, and the organization's CI stays independent of kos. Do not add kos to it. Before pushing a base branch, `<cli> sync verify --vault "<vault>" --base <previous remote commit> --allow-no-change` checks that every knowledge change since then is covered by an accepted review.
- Published knowledge whose review did not reach the base (`sync verify` reports `changes knowledge that no accepted review covers` after a merge): when the reviewer confirms the published content is the content they accepted, `sync start`, then `<cli> sync review --vault "<vault>" --base <base commit before the merge> --verdict accept --reviewer "<reviewer>" --summary "<the original review commit and PR>"`; `sync verify --base <that commit> --allow-no-change` must pass. Publish that branch like any other: it adds only the review record. Otherwise run a normal sync that reviews the content.
- Source moved during the run: re-run `discover run --repo <name>` and redo only that repository's delta.
- Interrupted run: the branch and its commits are the state; continue from `sync status`.

## Manual or scheduled refresh

The team chooses cadence, source scope, publication path (direct push, PR or review-only) and notification preference; store them in the scheduler's task or the vault, never in this skill, and read them back to confirm. Scheduling grants no push, merge or deployment authority. Each run: steps 1–7 for the selected sources, the notes `<cli> discover corrections --vault "<vault>"` lists (each relation the repository and platform evidence do not support is corrected, retracted or given the evidence that supports it, through the same gates and review), plus investigation knowledge ready to absorb (`manage-investigation` **Promote**/**Absorb**, see `../map-ecosystem/references/investigation-context.md`). Missing authority produces a read-only report. A run with nothing actionable stays quiet unless the saved preference asks for a status.

## Completion

Every selected repository has a published note update or an acknowledgement, `discover corrections` lists no relation left unresolved in the selected scope, `sync verify` passed on the published content, the remote publication (or review-only result) is verified, and no source was modified. Otherwise report the exact unresolved repository, its branch and the gate or review finding. Before declaring a campaign complete, apply `../map-ecosystem/references/mapping-completion.md`.
