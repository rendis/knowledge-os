# Story export contract

## Neutral draft

Create every export locally before considering publication. Use [../assets/story-template.md](../assets/story-template.md) and store drafts as `exports/S-<number>-<slug>.md`.

An investigation may create a provisional draft to obtain feedback. Label it prominently, keep the investigation in its current lifecycle state, and retain unresolved questions. Export has no investigation status transition; a draft becomes eligible for publication only after its selected-output sufficiency gate passes.

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
- Treat a `published` draft as an immutable publication snapshot. Do not silently rewrite or republish it. Create a new draft with a new `S` identifier for changed work and name the draft it replaces in its header.
- Never hand off a stale draft for publication or use it to satisfy an output-sufficiency gate.

Name every reconciled, marked-stale or replacement draft in the case's Current state; Git keeps the history.

## Audience and purpose

- For a product owner or scrum master, emphasize user or business outcome, scope, and acceptance criteria. Include only technical context needed to make the story actionable.
- For a developer or architect, include contracts, affected components, data, dependencies, operational concerns, risks, and verification evidence.
- For mixed purposes, create both kinds only when each serves a distinct consumer.

Create multiple stories only when outcomes are independently deliverable and verifiable. Keep coupled work in one story rather than splitting by repository or technical layer.

## Publication handoff

Keep publication outside this skill. When the user requests external publication, evaluate the selected draft independently. Require `synchronization-status: current`, a resolved case purpose, supported outcome and audience, applicable decisions and acceptance criteria, and no unresolved question or dependency that could change that exact output. Unrelated pending stories or optional investigation branches do not block it. Then build a package containing:

- source investigation ID and `updated-at`;
- exact draft paths and story IDs;
- exact target platform and destination;
- requested publication action;
- non-sensitive limitations that affect publication or verification.

Hand the package to `manage-operational-workflow`. That skill owns connector selection, the effect plan, explicit authorization, external writes, read-back verification, and its resumable run record. It must not read or modify the investigation case; after it returns a verified result, update the local draft with platform, `publication-status: published`, external reference, publication timestamp, and `updated-at`, then append the publication event to History.

When no compatible integration exists, deliver the local Markdown draft without degrading its content. Keep platform-specific field mappings out of the case-file contract.

## Development handoff

A development handoff is separate from story publication. It needs a settled scope for one repository: requirements, changes by component and acceptance criteria in the case, and the repository remote. Case status does not establish or deny sufficiency; evaluate only the selected repository's task, its decisions, criteria, evidence, questions and dependencies. A global blocker permits the handoff only when evidence establishes that it cannot change this task.

Write one package per repository and task at `<case>/handoffs/DH-NNN.md` following [manage-development-handoff](../../manage-development-handoff/SKILL.md). Several tasks may share a repository branch only when their changes overlap in the same component; otherwise give each its own branch. When the task depends on another story already implemented, carry the observed contract (APIs, events, data, configuration, compatibility), what can start and the remaining gaps in **Required context**, so the implementing agent does not rediscover it. When a current work item applies, read it through `../../../90-Meta/work-item-evidence.md` and carry what the task needs; never reconstruct it from memory or an old draft.

Before preparing the worktree, perform the [implementation-sufficiency check](implementation-sufficiency.md) on the package and complete recoverable omissions from the case. Structural validity or a prepared worktree does not establish sufficiency.

## Completion criterion

A provisional export is complete when it is current, traceable, and exposes its unresolved gaps. A stale draft is only a recorded follow-up obligation. An output-sufficient export is complete when its audience, kind, outcome, scope, acceptance criteria, applicable dependencies, evidence, register references, and source investigation snapshot are coherent. Readiness is per selected output: another `S-NNN`, work item, or repository may remain pending. A publication handoff is complete when the operational workflow receives the exact current package; publication is complete only when its verified external reference is observed and recorded locally. A development handoff is complete when its package passes `investigation check` and the sufficiency questions, and its prepared worktree is recorded in the case as `DH-NNN`. None of these events closes the investigation automatically.

An optional **References** section may name the vault remote and case id for read-only lookup; essential context stays in the package itself.
