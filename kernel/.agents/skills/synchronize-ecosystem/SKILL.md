---
name: synchronize-ecosystem
description: "Trigger: inventory, synchronize repository maps, or run a manual or scheduled vault refresh. Queries and single-note documentation belong to map-ecosystem."
license: Apache-2.0
metadata:
  author: documentation-vault maintainers
  version: "2.0"
---

# Synchronize the vault

A sync brings repository notes to the current production commit of their sources. The run is a local Git branch; commits are its checkpoints; the review is a commit that binds the accepted content; publication is a fast-forward merge plus the team's Git policy. Nobody is assumed to review by hand, so the gates and the independent agent review are mandatory.

## Hard rules

- Sources, platforms and databases are read-only evidence. Write to the vault only with vault-update authority; publish to the remote only through the repository's Git policy (`manage-git-workflow`).
- Every technical claim follows `90-Meta/evidence-policy.md` and the evidence format in `../map-ecosystem/references/repository-map.md`.
- One author writes the resulting notes; one fresh reviewer judges them (`../map-ecosystem/references/final-note-review.md`). The author never records its own review.
- A changed byte after an accepted review needs review of that change; a failed gate is fixed in the note, never by re-running source analysis.

## Recipe

1. **Bind.** Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). Start from a clean knowledge tree up to date with its upstream (fetch/pull per repository policy; preserve unrelated local work).
2. **Select.** `<cli> inventory --vault "<vault>" --format markdown` lists `new` and `changed` repositories. `<cli> discover run --vault "<vault>"` computes facts at each repository's [reference branch](../../../90-Meta/reference-branches.md) as the local checkout knows it, so `git fetch` the selected source checkouts first (a read that updates only remote-tracking refs); add `discover platform --referenced` when the user authorizes cloud reads. Pick small independent groups; a slow repository never holds the others.
3. **Branch.** `<cli> sync start --vault "<vault>" --name <slug>`. The session must be allowed to write the vault's `.git` and `.agents/state/` (some harness sandboxes, such as Codex `workspace-write`, keep them read-only: ask for approval of those commands). Work in the bound vault; a copy elsewhere is a different vault.
4. **Author.** For each repository: read the current note, the delta since `commit-analizado` (`git diff`), the facts and `discover check`'s stale cited files. Write the complete resulting note following repository-map.md; update `commit-analizado`, `fecha-analisis` and `rama-analizada`. Create or update topic/event/integration notes the facts require. A corrected or retracted claim is also fixed where neighbouring notes repeat it: check the notes `<cli> links` returns for each changed note. `sync verify` lists `stale_neighbours`, notes that cite lines of the synced repository that changed since their anchor's commit; update each claim against the new code and re-anchor it at the new commit. A repository whose delta changes nothing durable gets `sync acknowledge --decision no-documentation-change`; a new repository without a durable role gets `no-durable-node`. Commit.
5. **Gate.** `<cli> discover check --vault "<vault>" --note <each changed repository note>` passes with no `error`; pending items appear in the note's `Verificaciones pendientes`.
6. **Review.** Dispatch a fresh reviewer (`evidence-reviewer`, or another agent session) with final-note-review.md, the branch diff (`git diff <base>...HEAD`), the facts and the check output. On `revise`, correct only the cited text, commit, re-check, and re-review the change. On `accept`, record it: `<cli> sync review --vault "<vault>" --verdict accept --reviewer "<reviewer>" --summary "<one line>"`.
   A branch that only records discovery state (`90-Meta/discovery/`: classifications, platform snapshots) needs no knowledge review; `sync verify` validates it.
7. **Verify and publish.** `<cli> sync verify --vault "<vault>"` must return `ok` (note gates, no stale neighbour note, no structural issue introduced versus the base, review covering the final content). `<cli> sync finish --vault "<vault>"` fast-forwards the base. Push or open a PR as the repository policy says, and verify the remote ref.

## Concurrency and recovery

- Keep the base current with `<cli> sync pull --vault "<vault>"`: it fast-forwards when possible and never merges meaning automatically.
- Base moved (another developer published): `sync pull` lists the knowledge files changed on both sides. Rebase the branch; for each listed file re-apply this run's facts onto the upstream note, commit, re-run the gates and get the merged meaning reviewed. Never force-push.
- To make the remote enforce the gates on every pull request, the team can copy `90-Meta/ci/knowledge-gates.yml` to `.github/workflows/`; it runs `sync verify` with the vault's own Linux binary.
- Source moved during the run: re-run `discover run --repo <name>` and redo only that repository's delta.
- Interrupted run: the branch and its commits are the state; continue from `sync status`.
- Branches made by the legacy run state machine (`.agents/state/map-ecosystem/sync/`) are not resumable with this kernel; their completed receipts remain history.

## Manual or scheduled refresh

The team chooses cadence, source scope, publication path (direct push, PR or review-only) and notification preference; store them in the scheduler's task or the vault, never in this skill, and read them back to confirm. Scheduling grants no push, merge or deployment authority. Each run: steps 1–7 for the selected sources, plus investigation knowledge ready to absorb (`manage-investigation` **Promote**/**Absorb**, see `../map-ecosystem/references/investigation-context.md`). Missing authority produces a read-only report. A run with nothing actionable stays quiet unless the saved preference asks for a status.

## Completion

Every selected repository has a published note update or an acknowledgement, `sync verify` passed on the published content, the remote publication (or review-only result) is verified, and no source was modified. Otherwise report the exact unresolved repository, its branch and the gate or review finding. Before declaring a campaign complete, apply `../map-ecosystem/references/mapping-completion.md`.
