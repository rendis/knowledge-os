# Development reconciliation contract

Load this reference after one exact `DH-NNN` investigation entry resolves to one registry handoff and both identities validate.

## Comparison inputs

Reconcile these three independently observed layers:

1. **Exported baseline**: `jira.md`, `context.md`, `scope.md`, current `handoff.yaml`, and its referenced history event.
2. **Definition changelog**: every valid entry in `implementation-updates.md`, in ID order.
3. **Current evidence**: local implementation and tests, observed remote branch and pull request, current Jira story and evidence, and direct dependent stories.

The baseline and changelog are context and provenance, not proof of implementation or production. Repository, repository-host, CI, Jira, and deployment authorities establish their own current states.

## Material-delta test

A delta is material when it changes, complements, adds to, expands, narrows, contradicts, replaces, or otherwise mutates any initial outcome, scope boundary, acceptance criterion, decision, contract, API, event, data behavior, configuration, dependency, compatibility rule, verification obligation, rollout/rollback condition, or downstream-consumer requirement.

Routine code progress, refactoring that preserves the definition, additional passing evidence for an unchanged criterion, and delivery mechanics with no contract or scope effect are implementation evidence, not changelog entries.

Classify every observed delta exactly once:

| Result | Condition | Action |
|---|---|---|
| `unchanged` | Evidence remains within the baseline and adds no definition. | Keep as implementation evidence. |
| `logged-consistent` | One or more `UPD-NNN` entries fully represent the delta and agree with current evidence. | Carry entries and evidence to the investigation. |
| `logged-contradicted` | An entry exists but current evidence disagrees or supersedes it. | Require a new changelog entry before continuing. |
| `unlogged-material` | A material delta is absent from the changelog. | Block reconciliation; report a deficiency card without drafting the repository entry. |
| `unresolved` | Available evidence cannot determine whether the definition changed. | Preserve the question and block any affected readiness claim. |

Never reinterpret routine progress as a scope change to make the changelog look active. Never treat an empty changelog as proof that no change occurred.

## Normalized in-memory context

Pass one context object conceptually containing:

- source identity: investigation ID and timestamp, story ID, Jira site/key/URL and current timestamp, handoff family/revision, repository remote;
- vault binding: `DH-NNN`, exact absolute worktree path, registered handoff ID, and the identity comparison against `ACTIVE.yaml`;
- local snapshot binding: the `closure_fingerprint` returned before inspection and revalidated immediately before the case write;
- baseline summary and cited current history event;
- every changelog entry with its source, justification/agreement, optional real analysis, impact, evidence, and relationships;
- local repository evidence: absolute worktree, branch, `HEAD`, base used for comparison, status, commits, changed surfaces, diff summary, tests and results;
- remote/delivery evidence: observed remote ref and SHA, pull request URL/head/base/state/checks, merge state, deployment state, and observation methods;
- current Jira delta: material field, comment, evidence, attachment, status, relationship, and timestamp changes relevant to the investigation;
- direct-dependent source cards from the downstream contract;
- contradictions, unmatched evidence, access limits, and open questions;
- semantic case facts and affected identities: claims, evidence, decisions, criteria, questions, dependent stories, and draft identities whose meaning may have changed.

Keep this object in memory during the direct vault-side handoff to the case writer. The writer maps semantic facts to its own sections and registers and records the closure fingerprint with the reconciliation History event. Do not create a `return/` tree, reconciliation Markdown, generated case patch, or second ledger. The fingerprint binds mutable local handoff and Git-visible state; it does not attest that the case writer ran, so the same-interaction writer and post-write validation requirements remain procedural authority gates.

For `unlogged-material`, the deficiency card contains only:

- the observed delta and why it is material;
- the baseline identity or section it affects;
- exact non-sensitive repository, delivery, or Jira evidence;
- the missing `UPD-NNN` coverage and the readiness claims it blocks.

The card never invents an update title, status, agreement, analysis, or exact entry body. Repository work owns any later changelog append.

## Fail-closed rules

- A material delta is absent from the changelog: stop before changing the investigation.
- The case has no unique `DH-NNN` binding or it disagrees with the selected registry handoff: stop before reading the worktree as authoritative evidence.
- Jira cannot establish the current source story or link direction: mark the affected Jira and dependent portions blocked; do not infer.
- The remote branch cannot be observed: report only the local branch.
- Pull-request state is unavailable: do not infer it from a branch name or commit.
- Merge and deployment are independent; report each separately.
- A secret or sensitive attachment is encountered: retain only redacted existence, location, and behavioral relevance.

## Completion criterion

The comparison is complete only when every observed material delta has one classification, every non-unchanged delta is covered by a valid changelog entry, current external state is distinguished from local intent, and the in-memory context is sufficient for `manage-investigation` to update the case without re-reading the implementation worktree. A later authorized state update is never evidence that this comparison succeeded.
