# Development handoff input bundle

Load this reference when preparing or validating a package for `plan` or `apply`.

## Contents

- [Authority boundary](#authority-boundary)
- [Intake protocol](#intake-protocol)
- [Canonical package](#canonical-package)
- [Document roles](#document-roles)
- [Material comparison](#material-comparison)
- [Completion criterion](#completion-criterion)

## Authority boundary

The package is a one-way, self-contained export from `manage-investigation`. The development-handoff consumer receives the package directory only; it does not read or modify the investigation case, query Jira, reconcile a story, or infer missing content.

Create a package only after the investigation and story are release-ready and the copied Jira snapshot is verified current. If a current exact copy is unavailable, stop at the producer and report the missing source.

## Intake protocol

Classify each materialization or activation input as exactly one state:

- `exact-package`: an exact existing package directory was supplied and passes this contract; continue with consumer preflight.
- `producer-required`: the request identifies an investigation and Jira story but no exact package directory exists. Invoke the Export route of `manage-investigation`, let that owner inspect the case and obtain the current Jira snapshot, then resume the same handoff request with every returned exact directory. Do not ask the user to coordinate or resubmit the workflow.
- `invalid-package`: an existing candidate package fails this contract. Return its exact validation failure to `manage-investigation` and stop the consumer until the producer returns a corrected package.

Absence is `producer-required`, not `invalid-package`. The consumer never opens the case or Jira while resolving any state. Validation and deactivation of an existing worktree do not consume a package and therefore bypass this protocol.

## Canonical package

Create one package per Jira issue and target repository:

```text
.investigations/<investigation-id>/handoffs/
└── <issue-key-lower>--<repository-basename-lower>/
    ├── bundle.yaml
    ├── jira.md
    ├── context.md
    └── scope.md
```

Repeating an export updates that package in place only after confirming that `repository.remote` identifies the same target. A story that affects several repositories gets several packages; `context.md` and `scope.md` must be specific to each repository.

`bundle.yaml` uses this closed schema:

```yaml
schema-version: 1
source:
  investigation-id: "20260723-092916-stale-write-acceptance"
  investigation-updated-at: "2026-07-28T12:00:00-04:00"
  story-id: "S-001"

jira:
  site: "https://example.atlassian.net"
  issue-key: "System BR-3812"
  url: "https://example.atlassian.net/browse/System BR-3812"
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
- Use an uppercase Jira key and the investigation's stable `S-NNN` story ID.
- Set `snapshot-source` to `connected-readback` or `user-supplied-export`. Memory, a stale published draft, and an inferred reconstruction are not sources.
- Set `freshness: current` only after comparing the copied fields with the source represented by `jira.updated-at`.
- Resolve the target by its real Git remote. Do not put a local path in the package.
- Write a concise `change.summary` for every export. Add a per-file reason when it explains the change more precisely; the summary is the fallback reason.

## Document roles

### `jira.md`

Copy the current Jira story without paraphrasing. Include the key, summary, description, acceptance criteria, relevant implementation fields and links, and selected clarifications that alter delivery. Preserve the source wording and field boundaries.

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

## Material comparison

The consumer compares each document independently after Unicode normalization, line-ending normalization, removal of trailing spaces, and removal of outer blank lines. A formatting-only difference is a no-op. A material difference creates one new revision event and rewrites only the changed documents.

The exact input bytes are materialized when a document changes. The producer therefore owns source fidelity and secret removal before handing the package to the consumer.

After materialization, the implementation agent never edits these three documents. Any material information, agreement, discovery, or decision that changes or complements their definition is appended to the materialized `implementation-updates.md` under the repository-state contract. A later producer refresh may create a new immutable content revision; it never rewrites or clears that changelog.

## Completion criterion

The package is complete only when the schema validates, the Jira copy is current and traceable, all three documents are non-empty and secret-free, the remote identifies one configured repository, and the context and scope are specific to that repository.
