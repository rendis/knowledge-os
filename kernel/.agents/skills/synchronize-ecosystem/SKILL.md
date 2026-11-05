---
name: synchronize-ecosystem
description: "Trigger: inventory, synchronize repository maps, or run a manual or scheduled vault refresh. Queries and single-note documentation belong to map-ecosystem."
license: Apache-2.0
metadata:
  author: knowledge-os maintainers
  version: "2.0"
---

# Synchronize the vault

A sync brings repository notes to the configured reference revisions of their sources. With an origin, it prepares a reviewed PR candidate from the published base. The vault stays on its base branch, usually main. Drafts live in the directory returned by sync start; all checkpoints use explicit paths from that draft.

Use this workflow for an authorized synchronization or publication. Ordinary investigation and handoff work stays in its case directory and does not start sync.

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

Obtain separate authority to publish. Start with `<CLI> sync start --vault "<vault>" --name <slug> --case <ID>`. This copies the selected case's pending files into the draft and returns their paths. Inspect those files for shareability before checkpointing them. The run accepts paths only from that case. Other case edits and the live index stay in place. If the local case's committed content differs from the published base, reconcile that case before importing.

## Recipe

1. **Bind and select.** Bind the vault with [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). Use **Pending-work queries** to choose the authorized scope. Fetch the selected source refs before analysing their deltas. Use `discover run --read-only` on the vault for source facts and `discover platform --read-only` only with applicable cloud read authority. A case-only publication uses its own evidence.
2. **Start or resume.** Run `<CLI> sync start --vault "<vault>" --name <slug>`, or `sync status --vault "<vault>" --name <slug>` for an existing run. For a vault with origin the default is origin/main. Use --base <branch> only for a vault whose designated publication base differs. Keep the live checkout on its base branch. Use the returned draft directory for edits and read the candidate via its base_commit and head SHAs. A vault without origin still supports the local branch workflow: commit on the sync branch, omit --name after start, and finish to fast-forward its local base.
3. **Author and checkpoint.** Update the complete notes following [repository-map.md](../map-ecosystem/references/repository-map.md), including commit-analizado, fecha-analisis and rama-analizada. Check neighbouring notes for repeated claims or stale citations. Where no durable change is needed, write a draft acknowledgement via `sync acknowledge --name <slug>` and checkpoint its returned file. Run `<CLI> sync commit --vault "<vault>" --name <slug> --path <vault-relative-file> --message "<commit message>"`, repeating --path for every file in the authorized change. List removed files too. Checkpoint only the intended files.
4. **Gate and review.** Run `<CLI> sync verify --vault "<vault>" --name <slug>`. It checks the committed candidate, including changed notes, cases and neighbours. Before review, resolve findings other than the expected missing verdict. Give an independent reviewer the exact head, `git diff <base_commit> <head>`, the source evidence and the gate output. Record the verdict with `sync review --name <slug> --head <reviewed-sha> --verdict <accept|revise> --reviewer "<reviewer>" --summary "<one line>"`. On revise or a later checkpoint, obtain a fresh verdict. Discovery-state-only runs still pass their existing gate without a knowledge review.
5. **Prepare and publish.** Run `<CLI> sync finish --vault "<vault>" --name <slug>`. It fetches the base and returns ready_for_pr and push_refspec only after verification. If the base advanced on other paths it checks the combined tree. For a collision, start from the new base, reconcile the relevant files and get a new review. Where authorized, push the returned refspec to origin and open the PR to the reported base under the vault's Git policy. Verify the remote head. The local base update after merge is separate (sync pull).

## Concurrency and recovery

- Keep one author per named draft. Other work may continue in the live checkout. Unrelated pending or staged files do not enter or block a candidate. Resolve any reported overlap with an active local edit in the same target file before publication.
- A failed gate or interrupted run leaves the draft and candidate available via `sync status --name <slug>`. Use a new run for a conflicting upstream change. Never force-push.
- Validate existing published ranges with `sync verify --base <previous remote commit> --allow-no-change`. The kernel ships no CI workflow; the team's CI stays independent of kos.
- When the source revision changes, redo only its delta and repeat the candidate review.

## Manual or scheduled refresh

The team chooses cadence, source scope, publication path (direct push, PR or review-only) and notification preference; store them in the scheduler's task or the vault, never in this skill, and read them back to confirm. Scheduling grants no push, merge or deployment authority. Each run: steps 1–5 for the selected sources, the notes `<cli> discover corrections --vault "<vault>"` lists (each relation the repository and platform evidence do not support is corrected, retracted or given the evidence that supports it, through the same gates and review), plus investigation knowledge ready to absorb (`manage-investigation` **Promote**/**Absorb**, see `../map-ecosystem/references/investigation-context.md`). Missing authority produces a read-only report. A run with nothing actionable stays quiet unless the saved preference asks for a status.

## Completion

Every selected repository has a published note update or an acknowledgement, `discover corrections` lists no relation left unresolved in the selected scope, `sync verify` passed on the published content, the remote publication (or review-only result) is verified, and no source was modified. Otherwise report the exact unresolved repository, its branch and the gate or review finding. Before declaring a campaign complete, apply `../map-ecosystem/references/mapping-completion.md`.
