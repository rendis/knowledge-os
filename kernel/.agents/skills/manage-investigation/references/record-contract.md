# Case-file contract

Load this reference before creating or changing an investigation.

## Identity and layout

Generate an immutable ID as `YYYYMMDD-HHmmss-<slug>` in local time. Normalize the slug to lowercase ASCII words separated by hyphens. If the directory exists, append `-02`, `-03`, and the first available two-digit suffix.

```text
investigations/
└── <investigation-id>/
    ├── investigation.md
    ├── artifacts/
    ├── exports/
    └── handoffs/
```

The public directory is canonical and versionable. A necessary private overlay may exist at `.investigations-private/<investigation-id>/private.md`; it is ignored, supplementary, and may contain only `id`, `authority: private-overlay`, `updated-at`, and the ordered sections **Sensitive context**, **Private references**, and **History**. It must not restate or override public decisions, evidence, status, scope, acceptance criteria, or history. Do not create an empty overlay.

Do not maintain a separate index. Resume by searching `investigation.md` frontmatter and content in this order: exact `id`, exact entry in `consolidated-from`, exact `source-ref`, then title or keywords. Require user selection only for multiple matches.

Use `handoffs/` only for repository-specific development input packages that conform to the `manage-development-handoff` input contract. Create one package per target repository; the consumer receives the package directory, never authority to read or mutate the investigation case.

Development reconciliation creates no reverse package under `handoffs/` or elsewhere. The vault selects a registered worktree, assembles normalized context in memory, and `manage-investigation` applies it directly to the canonical case through [development-reconciliation.md](development-reconciliation.md).

## Frontmatter

Use the canonical restricted YAML form shown below: one unquoted lowercase key per
line, with the colon immediately after the key. This keeps the helper and other
YAML readers from assigning different meanings to ambiguous or duplicate keys.

Required fields:

```yaml
id: 20260717-163245-weekly-tags-filter
title: Weekly TAGS reporting filter
dedupe-key: weekly-tags-filter
status: investigating
created-at: 2026-07-17T16:32:45-04:00
updated-at: 2026-07-17T16:45:00-04:00
source-type: message
source-ref: customer conversation
requester-role: product-owner
export-intent: user-stories
purpose: development
vault-outcome: deferred-until-production
learning-outcome: not-evaluated
```

`dedupe-key` is required. Derive it from the stable problem or outcome, not from incidental wording.

`purpose` classifies the intended outcome:

- `knowledge`: answer or understand without defining a software delivery.
- `development`: define, validate, or export a future change.
- `mixed`: investigate current behavior and define a future change; keep both states separate.
- `undecided`: allowed while `investigating` or `blocked` as the requested outcome is classified. Resolve it before completed closure or any external or development output; an abandoned case may retain it.

`vault-outcome` records the independent documentation path for the case's current durable candidate, if one exists:

- `not-evaluated`: evidence is not yet sufficient to assess a current-state candidate; allowed while investigating or blocked, but not for a completed closure.
- `none`: no durable technical fact is a candidate.
- `deferred-until-production`: the only candidate is a proposal or lacks deployment required by the cell evidence profile. Implemented source behavior may qualify before deployment under `documented-source`; corrections of existing source maps follow the source-map rule in `90-Meta/evidence-policy.md` under every profile, as bounded by [map-correction.md](map-correction.md).
- `candidate-for-audit`: a fact may qualify under the cell evidence profile, but `map-ecosystem` has not independently applied that profile's evidence gate.
- `documented`: `map-ecosystem` independently verified the fact at the cell's required evidence level and confirmed that the canonical vault already represented it correctly or updated and verified the affected notes. Record the canonical notes, lifecycle result, evidence boundary, and observed checks in Readiness and History.

For `purpose: mixed`, the field follows the current-state candidate when one exists; every future-state portion remains explicitly deferred and outside the vault regardless of that value. Purpose never proves eligibility for the vault.

`learning-outcome` records the latest explicit assessment by `manage-investigation-derived-learning`; it is independent of `purpose`, investigation status, story export, and `vault-outcome`:

- `not-evaluated`: no current assessment exists, or material new evidence invalidated the previous snapshot.
- `no-learning`: the reviewed evidence was sufficient but contained no reusable learning.
- `already-covered`: an existing canonical note already covered the teaching and context.
- `insufficient-evidence`: a named source, context, or reproducible result was missing; this is retryable.
- `candidate`: the assessment was `extractable`, but no durable publication was completed.
- `documented`: the skill created, enriched, challenged, superseded, or revalidated the canonical note and the vault gates passed.

The assessment itself is read-only. `manage-investigation` may record its observed outcome, target note, evidence boundary, case snapshot, and checks in Readiness and History after the assessment completes. When a later material input could change that result, preserve the prior event in History and reset the field to `not-evaluated`.

Use `unknown` only when the value cannot be derived and does not justify a question. Add `blocked-on` only while blocked:

```yaml
blocked-on: A-002 cannot be read
```

Add `closure-outcome` only while closed:

```yaml
closure-outcome: completed
```

Allowed closure outcomes are `completed` and `abandoned`. `resume-to` is obsolete metadata and is invalid; unblocking and reopening always return to `investigating` while History preserves the prior state and reason.

Allowed statuses are defined only in [readiness-and-lifecycle.md](readiness-and-lifecycle.md).

A canonical case that absorbed duplicate cases records their IDs:

```yaml
consolidated-from:
  - 20260717-163300-equivalent-case
```

After its exact snapshot, unique artifacts, drafts, and register mapping are preserved in the canonical case, remove the retired directory authorized for consolidation. Resolve its old ID by matching `consolidated-from`. Lineage never authorizes using another case file as evidence.

## Required sections

Keep these sections in this order, translated to the user's working language when instantiated:

1. Request summary
2. Current state
   - Objective
   - Scope
   - Out of scope
   - Current productive state
   - Future/proposed state
3. References and attachments
4. Evidence
   - Facts
   - Inferences
   - Contradictions
5. Affected surfaces
6. Development handoffs
7. Open questions
8. Decisions
9. Acceptance criteria
10. Readiness
11. History

**Current state** is the brief consumable snapshot. Update it in place and keep detail in its single register entry. A development or mixed case must keep **Current productive state** and **Future/proposed state** as separate subsections; a future proposal may cite current facts for context but must not blur their status. **History** is append-only and records material changes without repeating the resulting snapshot.

## Registers and traceability

Assign stable identifiers:

- Evidence: `E-001`, `E-002`, ...
- Attachments or source summaries: `A-001`, `A-002`, ...
- Questions: `Q-001`, `Q-002`, ...
- Decisions: `D-001`, `D-002`, ...
- Acceptance criteria: `AC-001`, `AC-002`, ...
- Exports: `S-001`, `S-002`, ...
- Development handoffs: `DH-001`, `DH-002`, ...

An identifier is immutable after assignment. Never renumber, recycle, delete, or change the meaning of an existing identifier. A materially different claim, question, decision, criterion, attachment, or export receives the next available identifier. Preserve inactive entries with their state and replacement or resolution links so older History events and exports remain interpretable.

Every material write is attributed to the vault checkout's effective Git `user.name` and `user.email`, an offset timestamp, a portable source, and all affected stable IDs. The transactional helper writes that compact History attribution. It identifies the recorder, not the person who decided, agreed, approved, or supplied the evidence. Record those roles in the relevant register only when the cited source establishes them. A migrated historical entry with no attributable source remains explicitly `recorder not recorded`; the migrator is recorded only for the migration event and is never assigned as the historical author.

Before calling `save`, provide every added or changed public register ID with repeated `--target` arguments, only IDs whose restricted context changed with `--private-target`, and one `--source` that another collaborator can resolve without a local absolute path. A repeated input that changes no material meaning preserves the loaded candidate bytes exactly and produces the helper's `unchanged` result. A single new fact updates only its register, the brief Current state when needed, and one attributed History event.

Each question records `open`, `resolved`, or `superseded` state. Keep open questions first. Retain resolved and superseded questions in a clearly labeled subsection of **Open questions**, with the resolving decision or evidence; a replacement question receives a new ID and reciprocal `supersedes`/`superseded-by` links.

Each evidence entry states its claim, category (`fact`, `inference`, `contradiction`, or `limitation`), source, and relevant location such as file, section, page, line, URL, or revision. Link decisions and acceptance criteria to supporting identifiers when available.

Each materialized development target has one stable entry under **Development handoffs**. Allocate a new `DH-NNN` for a new story-and-repository identity; update that same entry when a later materialization advances its current revision. Entries from the same investigation and repository may share a branch; their story, work-item, handoff, family, and revision identities remain distinct. Record these fields exactly:

```text
### DH-001 — <tracker-id>:<work-item-reference> / <repository basename>

- Story ID: <S-NNN>
- Tracker ID: <configured tracker ID>
- Provider: <provider name>
- Tracker URL: <canonical tracker URL>
- Work item reference: <exact provider-native reference>
- Repository remote: <normalized remote>
- Branch: <exact Git branch>
- Handoff ID: <exact handoff ID>
- Family: <exact family>
- Revision: <vNNNN>
- Materialized at: <ISO-8601 timestamp with UTC offset>
```

The same History event that creates or advances a binding includes one exact marker on its own two-space-indented line:

```text
- <materialized-at> — <action in the cell note locale> development handoff `DH-001`; story `S-NNN`; work item `<tracker-id>:<reference>`; repository `<remote>`; branch `<branch>`; handoff `<handoff-id>`; revision `<vNNNN>`.
  <!-- knowledge-os:development-handoff-binding {"branch":"<branch>","dh":"DH-001","family":"<family>","handoff-id":"<handoff-id>","materialized-at":"<timestamp>","provider":"<provider>","repository-remote":"<remote>","revision":"<vNNNN>","story-id":"<S-NNN>","tracker-id":"<tracker-id>","tracker-url":"<tracker-url>","work-item-reference":"<reference>"} -->
```

Use canonical compact UTF-8 JSON: keep the keys in the exact lexicographic order shown, omit structural whitespace, and allow no duplicate key. Reserved `knowledge-os:development-handoff-binding` markers may appear only in History. The surrounding History bullet starts with the marker's exact `materialized-at`, remains human-readable, names the action, and includes the exact `DH-NNN`, story, work-item identity, repository remote, branch, handoff ID, and revision through the labelled fragments shown above. Keep exactly one event and marker per materialized content revision, in actual `v0001` through current-revision order. The current marker must match every field in the current register entry; older markers preserve their observed branch and timestamp while retaining stable identity.

An activation or idempotent binding retry of the exact already-recorded revision and coordinates is a byte-level case no-op: validate the existing current entry and marker, but append no History event and no duplicate marker. A same-revision observation that changes any recorded coordinate or timestamp is a conflict, not an activation.

This register is the vault-owned locator for later reconciliation. Populate it only from the same vault-side materialization result after validation; do not infer availability from a branch name, repository message, or callback. History records every new or advanced binding so prior branches and revisions remain interpretable.

For a reconciled implementation, keep one source implementation card and one card per directly dependent work item in **Affected surfaces**. Each dependent card names the exact typed-relation direction, consumer contracts, what can start, remaining gaps, repository/branch/PR observations, and `ready`, `partial`, `still-blocked`, or `not-applicable` readiness. These cards are case context, not tracker status changes or production evidence.

Persistent memory, another case file, and a neighboring local project are discovery aids, not evidence. Register a claim only after inspecting an explicitly in-scope source directly. Cite that observed source rather than memory, and omit any unscoped project, component, or claim from the case and its exports.

## Decisions

Treat decision statements as immutable. A decision records date, state (`active` or `superseded`), statement, reason, and evidence. Replacing `D-001` creates a new decision with `supersedes: D-001` and updates the old entry with `superseded-by: <new-id>`; preserve both statements and append the replacement event to History.

## Attachments

Classify an attachment before copying it. Put an exact byte-for-byte copy in public `artifacts/` only after reviewing that the entire file is shareable and contains no credential value or local-environment detail. For sensitive necessary material, keep only a protected reference in the private overlay; otherwise omit it. Name public copies `A-<number>-<safe-original-name>` and record a portable origin, capture time, repository-relative copied path, and SHA-256.

If copying fails but reading succeeds, offer a source summary created from [../assets/source-summary-template.md](../assets/source-summary-template.md). Mark it explicitly as a summary and record:

- Original source and name
- Copy failure
- Coverage: `complete`, `partial`, or `selected`
- Reviewed and omitted sections or ranges
- Selection basis
- Fidelity limitations

For a large source, summarize the material relevant to the investigation objective rather than an arbitrary percentage. If omitted material could change a conclusion, keep the conclusion provisional and fail the applicable readiness gate.

If reading fails, record only observable metadata and the access failure, move to `blocked`, and ask the user to grant access, upload again, replace, remove, or explicitly continue without the source. Do not infer or summarize unread content.

## Persistence classification

Before every write, classify proposed content:

- Public: relevant, shareable, professionally worded investigation knowledge needed to understand state, evidence, decisions, or next steps.
- Private: only sensitive context necessary to continue the case that cannot be safely generalized in public. It is never authoritative.
- Omit: transcripts, hidden reasoning, incidental local details, informal phrasing, and process chatter that do not improve the investigation.
- Forbidden: credentials, tokens, private keys, cookies, and equivalent secret values in either store.

Use repository remote and branch as portable development identity. Record an observed commit only when it identifies the exact evidence behind a claim. Never persist an absolute worktree path. A derived location is only a candidate until current Git inspection proves the repository and branch are present there.

## Sensitive material

Inspect before persisting. When a source exposes credentials, tokens, private keys, cookies, or equivalent secrets, record only their redacted existence, protected location, and behavioral relevance. Keep the value out of public and private case files, artifacts, summaries, exports, logs, and responses.

## Completion criterion

The record conforms when identity and register meanings are immutable, the current snapshot matches the latest material evidence and decisions, registers are traceable without reused IDs, every materialized development target has one exact `DH-NNN` binding, History preserves chronology, attachment and consolidation handling is explicit, affected exports have an explicit synchronization state, and no secret value is persisted.

Binding metadata does not change the case semantic `updated-at`. Its independent materialization/event timestamp preserves export freshness; material evidence or decision changes still advance `updated-at`.
