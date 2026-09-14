# Story export contract

## Neutral draft

Create every export locally before considering publication. Use [../assets/story-template.md](../assets/story-template.md) and store drafts as `exports/S-<number>-<slug>.md`.

An active investigation may create a provisional draft to obtain feedback. Label it prominently, keep the investigation in its current lifecycle state, and retain unresolved questions. Only a draft produced after the `ready-to-export` gate can support the transition to `exported`.

Required metadata:

```yaml
story-id: S-001
source-investigation: 20260717-163245-weekly-tags-filter
story-kind: technical
audience: developer
platform: local
publication-status: draft
created-at: 2026-07-17T18:00:00-04:00
updated-at: 2026-07-17T18:00:00-04:00
source-updated-at: 2026-07-17T17:55:00-04:00
synchronization-status: current
```

Allowed story kinds are `technical` and `user`. Allowed publication statuses are `draft` and `published`. `platform` remains `local` until `manage-operational-workflow` returns a verified publication result.

Allowed synchronization statuses are `current` and `stale`. Copy the investigation's `updated-at` into `source-updated-at` only after reconciling the draft against that snapshot. Set `synchronization-status: current` only after verifying that every referenced `E`, `Q`, `D`, and `AC` identifier still exists with the same meaning; the draft matches the current objective, scope, decisions, questions, and acceptance criteria; and every factual or implementation-context statement traces to in-scope registered evidence or an active decision. Omit memory-derived claims and unregistered local projects, components, dependencies, or reuse advice.

## Reconciliation

After a material case-file change, inspect every local draft that references the affected material:

- For `publication-status: draft`, update its content and `updated-at`, copy the current case timestamp into `source-updated-at`, and restore `current`. If reconciliation cannot finish in the same interaction, set `stale` and add a prominent reason; do not leave an unlabeled mismatch.
- Treat a `published` draft as an immutable publication snapshot. Do not silently rewrite or republish it. Create a new draft with a new `S` identifier for changed work and record the relationship in History.
- Never hand off a stale draft for publication or use it to satisfy a lifecycle gate.

Append one case History event naming every reconciled, marked-stale, or replacement draft.

## Audience and purpose

- For a product owner or scrum master, emphasize user or business outcome, scope, and acceptance criteria. Include only technical context needed to make the story actionable.
- For a developer or architect, include contracts, affected components, data, dependencies, operational concerns, risks, and verification evidence.
- For mixed purposes, create both kinds only when each serves a distinct consumer.

Create multiple stories only when outcomes are independently deliverable and verifiable. Keep coupled work in one story rather than splitting by repository or technical layer.

## Publication handoff

Keep publication outside this skill. When the user requests external publication, require `synchronization-status: current` and build a package containing:

- source investigation ID and `updated-at`;
- exact draft paths and story IDs;
- exact target platform and destination;
- requested publication action;
- non-sensitive limitations that affect publication or verification.

Hand the package to `manage-operational-workflow`. That skill owns connector selection, the effect plan, explicit authorization, external writes, read-back verification, and its resumable run record. It must not read or modify the investigation case; after it returns a verified result, update the local draft with platform, `publication-status: published`, external reference, publication timestamp, and `updated-at`, then append the publication event to History.

When no compatible integration exists, deliver the local Markdown draft without degrading its content. Keep platform-specific field mappings out of the case-file contract.

## Development handoff

A development handoff is separate from publication. Require a release-ready development or mixed story, one exact current work-item snapshot, and an explicit target remote for every repository that owns implementation work.

Before materialization, require an exact repository remote and explicit overlap in the named component or implementation scope before proposing a shared worktree. A common remote alone is not enough; otherwise use a separate worktree. Compare new packages with existing `DH-NNN` entries by repository and branch. Resolve a selected branch to a local path only at runtime; persist no grouping or local-path artifact.

Load [the development input-bundle contract](../../manage-development-handoff/references/input-bundle.md) and create one canonical package per target under:

```text
investigations/<investigation-id>/handoffs/
└── <work-item-token>--<repository-basename-lower>/
```

Compile `work-item.md` from the current read-only snapshot produced under `VAULT_ROOT/90-Meta/work-item-evidence.md`; never reconstruct it from a published local draft, copied handoff, case History, or memory. Compile `context.md` using [development-context-template.md](../assets/development-context-template.md), populating its read-only source reference when the existing vault remote is verified; compile it and `scope.md` from the current case, registered evidence, decisions, acceptance criteria, and the selected repository boundary. A multi-repository story must produce different context and scope whenever ownership differs.

When the target story is a directly dependent story previously reconciled from another implementation, include its current dependent card in the repository-specific package. Carry the observed contracts, APIs/events/data/configuration, compatibility and verification requirements, what can start, remaining gaps, source repository remote, and observed remote branch/PR state. Re-read the dependent work item before export; do not make its implementation worktree rediscover an already captured source contract.

Before delivering each exact package, perform the [question-based implementation-sufficiency check](../../manage-development-handoff/references/input-bundle.md#implementation-sufficiency). Complete recoverable source-to-package omissions during this export; pass only packages whose necessary questions have supported answers. Structural validity or successful materialization does not establish this result.

Keep repository evidence flow one-way: hand only exact package directories and the user's worktree choice to `manage-development-handoff`. After repository-state validation, it passes one normalized in-memory observation per handoff to **Bind development handoff**. The observation persists repository and branch identity; the validated local path remains in memory.

## Completion criterion

A provisional export is complete when it is current, traceable, and exposes its unresolved gaps. A stale draft is only a recorded follow-up obligation. A release-ready export is complete when its audience, kind, outcome, scope, acceptance criteria, dependencies, evidence, register references, and source investigation snapshot are coherent. A publication handoff is complete when the operational workflow receives the exact current package; publication is complete only when its verified external reference is observed and recorded locally. A development handoff package is complete only when the linked input contract passes independently for every target repository; materialization or activation is complete only after its exact `DH-NNN` binding passes case validation.

For an optional portable, read-only source reference in `context.md`, follow [input-bundle.md](../../manage-development-handoff/references/input-bundle.md#optional-source-context). Include essential context in the package even when an anchor is present.
