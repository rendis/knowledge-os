# Development handoff input bundle

Load this reference when preparing or validating a package for `plan` or `apply`.

## Contents

- [Authority boundary](#authority-boundary)
- [Intake protocol](#intake-protocol)
- [Canonical package](#canonical-package)
- [Document roles](#document-roles)
- [Implementation sufficiency](#implementation-sufficiency)
- [Material comparison](#material-comparison)
- [Completion criterion](#completion-criterion)

## Authority boundary

The package is a one-way, self-contained export from `manage-investigation`. The development-handoff consumer receives the package directory only; it does not read or modify the investigation case, query the tracker, reconcile a story, or infer missing content.

Create a package only after the selected story-and-repository output is sufficient under the producer's export contract and the copied work-item snapshot is verified current. Investigation status and unrelated pending stories do not establish this result. If a current exact copy is unavailable, stop that target at the producer and report the missing source.

## Intake protocol

Classify each materialization or activation input as exactly one state:

- `exact-package`: an exact existing package directory was supplied and passes this contract; continue with consumer preflight.
- `producer-required`: the request identifies an investigation and work item but no exact package directory exists. Invoke the Export route of `manage-investigation`, let that owner inspect the case and obtain the current work-item snapshot, then continue the same handoff request with every exact directory it produces. Do not ask the user to coordinate or resubmit the workflow.
- `invalid-package`: an existing candidate package fails this contract. Give its exact validation failure to `manage-investigation` and stop the consumer until that owner produces a corrected package.

Absence is `producer-required`, not `invalid-package`. The consumer never opens the case or tracker while resolving any state. Validation and state updates for an existing worktree do not consume a package and therefore bypass this protocol.

Before materializing an exact package, require the producer's [implementation-sufficiency assessment](../../manage-investigation/references/implementation-sufficiency.md) for that content and source snapshot. When it has not already run in the current workflow, route the exact directory to `manage-investigation` for a read-only assessment. Return concrete gaps to that owner; a successful helper command is not a substitute for this assessment.

That producer review retains SHA-256 snapshots of the complete case file and exact source story bytes. Carry those two hashes in the workflow context to **Bind development handoff** together with the validated materialization observation. They are review inputs, not new bundle or handoff manifest fields. For subsequent targets in a shared case or a retry, the producer rechecks the source against the exact package before refreshing snapshots; a previous binding may have changed case History. A stale or missing snapshot leaves the target `materialized-unbound` until that review succeeds.

## Canonical package

Create one package per work item and target repository. Packages from the same investigation and repository may later share one worktree; they never share a family directory:

```text
investigations/<investigation-id>/handoffs/
└── <work-item-token>--<repository-basename-lower>/
    ├── bundle.yaml
    ├── work-item.md
    ├── context.md
    └── scope.md
```

Repeating an export updates that package in place only after confirming that `repository.remote` identifies the same target. A story that affects several repositories gets several packages; `context.md` and `scope.md` must be specific to each repository.

`bundle.yaml` uses this closed schema:

```yaml
schema-version: 2
source:
  investigation-id: "20260723-092916-stale-write-acceptance"
  investigation-updated-at: "2026-07-28T12:00:00-04:00"
  story-id: "S-001"

work-item:
  tracker-id: "delivery"
  provider: "example"
  tracker-url: "https://tracker.example.com"
  reference: "BR-3812"
  url: "https://tracker.example.com/items/BR-3812"
  updated-at: "2026-07-28T11:55:00-04:00"
  captured-at: "2026-07-28T12:01:00-04:00"
  freshness: "current"
  snapshot-source: "connected-readback"

repository:
  remote: "https://github.com/example/SYS002-schema-repository.git"

change:
  summary: "Clarify the stale-write acceptance criterion."
  reasons:
    scope.md: "Add the agreed stale timestamp behavior."
```

Rules:

- Quote every timestamp and include a UTC offset.
- Use the exact provider-native reference, the tracker binding declared in `instance.yaml`, and the investigation's stable `S-NNN` story ID.
- Set `snapshot-source` to `connected-readback` or `user-supplied-export`. Memory, a stale published draft, and an inferred reconstruction are not sources.
- Set `freshness: current` only after comparing the copied fields with the source represented by `work-item.updated-at`.
- Resolve the target by its real Git remote. Do not put a local path in the package.
- Write a concise `change.summary` for every export. Add a per-file reason when it explains the change more precisely; otherwise the summary is the reason for that file.

## Document roles

### `work-item.md`

Copy the current work item without paraphrasing. Include the provider-native reference, summary, description, acceptance criteria, relevant implementation fields and links, and selected clarifications that alter delivery. Preserve the source wording and field boundaries.

Exclude comments, history, watchers, private contact data, and unrelated fields unless one is necessary to implement or verify the story. Record only the non-sensitive URL and timestamps in `bundle.yaml`; never copy credentials, session data, or private tokens.

### `context.md`

Include the minimum investigation context that can change implementation in this repository:

- verified current-state facts and their evidence references;
- proposed behavior, decisions, dependencies, risks, and limitations;
- cross-repository contracts this repository must honor;
- unresolved items only when they are explicitly non-blocking.

Separate facts, proposals, inferences, and limitations. Do not make the target agent reconstruct the investigation chronology.

### `scope.md`

Make the repository boundary executable:

- outcome and deliverables owned by this repository;
- in-scope and out-of-scope behavior;
- repository-specific acceptance criteria;
- affected surfaces and dependencies;
- verification expectations;
- coordination points with other repository packages.

Do not copy another repository's implementation work into this file.

## Implementation sufficiency

The producer owns the [implementation-sufficiency assessment](../../manage-investigation/references/implementation-sufficiency.md). This anchor remains for existing links; the consumer checks integrity and consumes the producer’s result.

## Material comparison

The consumer compares each document independently after Unicode normalization, line-ending normalization, removal of trailing spaces, and removal of outer blank lines. A formatting-only difference is a no-op. A material difference creates one new revision event and rewrites only the changed documents.

The exact input bytes are materialized when a document changes. The producer therefore owns source fidelity and secret removal before handing the package to the consumer.

After materialization, the implementation agent never edits these three documents. Any material information, agreement, discovery, or decision that changes or complements their definition is appended to the materialized `implementation-updates.md` under the repository-state contract. A later producer refresh may create a new immutable content revision; it never rewrites or clears that changelog.

## Post-materialization binding

After `validate` succeeds, the consumer assembles one normalized in-memory observation from the exact package and validated repository state:

- package path, investigation ID, and story ID;
- exact tracker ID, provider, canonical tracker URL, and provider-native reference;
- normalized repository remote and exact branch for persistence, plus the validated absolute worktree path only as runtime context;
- handoff ID, family, revision, and `materialized_at` derived from the validated manifest `updated-at`.

Pass that observation to the **Bind development handoff** route of `manage-investigation`. The consumer never opens or edits the case, and the repository neither returns evidence nor invokes a case workflow: it only persists inspectable evidence under `.knowledge-os-handoffs/`. A repository-side lifecycle request may use `manage-development-handoff` **Set state** because that operation writes only the selected worktree registry entry; it does not bind or reconcile the case. If the case owner cannot validate the binding, classify the target as `materialized-unbound`, preserve its active materialized state, and stop before claiming completion. Retry binding only from a new successful repository-state validation; do not persist a return package or infer identity from branch names or messages.

The case owner records only new or advanced materialized content revisions. An activation, no-op application, or binding retry for the exact already-recorded revision and coordinates validates that binding without changing case bytes or duplicating its History marker.

## Completion criterion

The package is complete only when the schema validates, its tracker binding matches `instance.yaml`, the work-item copy is current and traceable, all three documents are non-empty and secret-free, the remote identifies one configured repository, the context and scope are specific to that repository, and the producer's implementation-sufficiency check passes for this selected target. Helper validation establishes structural and repository integrity, not semantic completeness. Another story or repository may remain pending without blocking this package. Package completeness alone does not make a materialization or activation complete; the post-materialization binding must also validate, and neither event closes the source investigation automatically.

## Optional source context

The essential package must answer implementation questions on its own. When a specific missing detail warrants deeper inspection, `context.md` may include an optional **Read-only source reference** containing the existing vault Git remote, exact investigation ID, and relevant section names or exact vault note paths. Keep local vault paths out of this portable anchor. Omit the reference if no verified remote exists; do not invent one.

Resolve a local vault explicitly with the user-provided location or existing vault resolver and verify its remote before reading only the named case sections or notes. The reference is read-only workflow scope, not an OS sandbox permission or access guarantee. If unavailable, record the concrete question in `implementation-updates.md` and continue independent scoped work. The recipient needs no Obsidian or vault-authoring skills. The anchor does not authorize tracker access, unrelated cases, or a whole-vault scan.

Record implementation modifications, decisions, deviations, questions, and evidence in the affected worktree's `implementation-updates.md`; keep source code and tests in their normal repository locations. Only the vault-side reconciliation workflow updates the canonical investigation. Existing worktrees are not traversed or refreshed automatically.
