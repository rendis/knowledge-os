# Model and manage Jira work items

## When to load

Load this reference whenever the requested advice, draft, execution, resume, or validation estimates, creates, updates, decomposes, converts, links, or reparents Jira work items.

## Authority and inputs

Use the versioned operational notes as the semantic authority:

- `60-Operacion/Jira/Jira - Tipos y granularidad.md` for work intent and the checklist, subtask, or independent-ticket decision.
- `60-Operacion/Jira/Jira - Estimacion de historias.md` for Story Points, applicable testing effort, confidence, and split-or-Spike decisions.
- `60-Operacion/Jira/Jira - Convenciones y plantillas.md` for content and relationship structure.
- `60-Operacion/Jira/Jira - Validacion de tickets.md` for read-back and integrity checks.
- `60-Operacion/Jira/Jira - Catalogo System BR.md` only when System BR is the selected destination.

Treat current read-only Jira metadata and existing work items as the authority for project availability, hierarchy, fields, workflows, link types, and external state. A versioned catalog never replaces that preflight.

Require an intended outcome, exact destination project, source anchor, requested action, and any known parent or related work item. Missing input that can change scope, hierarchy, content, or destination blocks an external effect.

## Inspect the destination

Before proposing a structure or estimate:

1. Resolve the exact project and confirm the requested action is available.
2. Inspect current work types, hierarchy levels, required fields, Parent behavior, available link types, and the exact field that can store checklist items.
   For every directed dependency, read the current link-type name plus its
   outward and inward descriptions and model the semantic edge as
   `BLOCKER → BLOCKED` before preparing any Jira transport fields. Apply the
   canonical relationship contract in
   `60-Operacion/Jira/Jira - Convenciones y plantillas.md`.
3. Confirm that every proposed parent-child pair is compatible, belongs to an allowed scope, and cannot introduce a hierarchy cycle. Apply this to subtasks, independent tickets with Parent, and reparenting; do not infer compatibility from a type name or hierarchy level alone.
4. Resolve every supplied parent, related key, or source URL and verify that it identifies the intended artifact.
5. Search reasonably for existing work that may already express the same result.
6. For a conversion or reparenting, read the current work type, workflow, status, required and populated fields, Parent, children and subtasks, links, and inherited planning or access fields. Resolve the target type or Parent and identify every required mapping or inherited-value change before previewing the effect.
7. Block a conversion to a subtask while the source has children or subtasks unless an explicitly authorized preceding plan safely relocates or converts them. Do not silently discard or recreate descendants.
8. When Story Points are requested, inspect the current value, field availability, affected components and contracts, repository test paths, and comparable completed work available to the same team.

Do not assume a universal hierarchy such as `Epic → Task → Sub-task`. Jira configuration and project type determine the valid parent-child structures.

## Apply the classification standard

Apply the ordered decision in `Jira - Tipos y granularidad.md` to one primary outcome and each proposed fragment. Record the evidence used for its tracking need, autonomy, acceptance and delivery boundary, assignee needs, and semantic relationships.

The selected form must be exactly `checklist`, `subtask`, or `independent ticket`. Parent expresses the compatible hierarchy verified in Jira; typed links express blocking, duplication, or another additional semantic relationship. Apply the standard's assignee rule without promoting work to an independent ticket solely because the assignee differs.

If the evidence supports more than one classification, present the alternatives and their effect on planning and acceptance. Do not create until the material ambiguity is resolved.

## Estimate the complete work

Apply `Jira - Estimacion de historias.md` after the acceptance boundary is stable:

1. Account for implementation, data/configuration, applicable automated tests, focused regression, and acceptance validation exactly once.
2. Build the standard's per-item breakdown. Give every block an exact code, test, contract, database, or Jira evidence anchor and separate implementation and test/validation effort in half-day increments.
3. Sum block effort once, state the total in days, and map that total to the local scale. Never assign or add partial Story Points.
4. Cite an exact comparable key and known effort when available; otherwise record the search scope and state that no valid comparable was found.
5. Explain why the proposed value fits and why the immediately lower value does not.
6. State confidence and evidence gaps. With low confidence, return only a range and block a Jira write unless the user or team explicitly authorizes the documented provisional exception.
7. Justify every 5 against 4, a split, and a Spike. First attempt decomposition; retain 5 only for an inseparable result within five business days.

Complete this step only when every affected layer and applicable test type has a decision, the numerical breakdown derives the total, exact evidence supports each block, the mapping and lower-value comparison are explicit, and confidence is medium or high. A low-confidence result completes only as an unpointed provisional range or as an explicitly authorized exception.

## Preview the exact structure

Before an external effect, include the complete Jira decomposition in the workflow effect plan:

| Local ID | Form | Work type or transition | Summary | Storage field | Parent change | Links | Acceptance boundary | Reason |
|---|---|---|---|---|---|---|---|---|

When estimation is requested, add:

| Local ID | Evidence | Implementation days | Test/validation days | Total probable | Comparable | Story Points | Why not lower | Confidence | Split or Spike |
|---|---|---:|---:|---:|---|---:|---|---|---|

Attach the complete per-item breakdown from `Jira - Estimacion de historias.md`; the comparison table is not sufficient evidence by itself.

Show checklist items inside their owning ticket draft and name the exact Jira field that will store them; do not present them as planned Jira artifacts. For every subtask or independent ticket, include the exact proposed content, destination, required fields, and relationship identifiers.

For a conversion or reparenting, also preview:

- source key, current and target work type, confirmation that the Jira key stays the same, and how all pre-existing history will be retained while the conversion or reparenting is recorded as a new history event;
- current and target Parent, including the disposition of every child or subtask;
- current and target workflow/status plus every required-field mapping;
- populated fields, links, planning values, or access values that will change, inherit, become unavailable, or require explicit input;
- exact operation order and read-back evidence for the changed item and the old and new parents.

An in-place conversion must preserve the same Jira key and all pre-existing history, and the conversion itself must add a new event to the history or changelog. If the connected capability cannot perform or verify both properties, block it; never emulate conversion by creating a replacement or deleting the source without a separately authorized procedure-backed action.

A change to form, work type, parent, link type, destination, content, Story Points, or execution order is material and requires a refreshed preview and authorization.

For every proposed `Blocks` relation, add this directed table to the preview:

| Blocker | Blocked | Live type | Live outward | Live inward | Transport payload | Two-sided read-back |
|---|---|---|---|---|---|---|

Build that table and the transport payload only from the output of
[`prepare-jira-blocks-links.py`](../scripts/prepare-jira-blocks-links.py).
Pass the exact live type metadata and each authorized semantic edge as
`--edge <BLOCKER> <BLOCKED>`. Do not hand-author or swap `outwardIssue` and
`inwardIssue`. A blocked helper result, a metadata mismatch, a duplicate edge,
or contradictory opposite edges blocks the external write.

## Apply and verify

After authorization:

1. Reconcile the destination and every supplied artifact read-only.
2. Resolve the exact field that will store checklist items, then persist and read them back from that approved field on the owning ticket.
3. Create or update parents before their children.
4. Create subtasks only after the exact compatible parent exists.
5. Create independent tickets in the approved hierarchy and add typed links only after both endpoints exist.
   Immediately before creating `Blocks` links, rerun
   `prepare-jira-blocks-links.py` against the current live link-type metadata
   and the authorized blocker/blocked pairs. Use each emitted `transport`
   object without reordering its endpoints.
6. Immediately before a conversion or reparenting, re-read the source, its history or changelog, its descendants, and the affected old and new parents. Retain enough non-sensitive event identity and ordering evidence to compare the pre-effect and post-effect histories. Stop if any type, status, field, Parent, child, link, or history baseline differs materially from the approved preview.
7. Perform the approved in-place conversion or reparenting once. Read the changed item, its history or changelog, and every affected parent back; verify the same key, target type, mapped workflow/status and fields, descendant disposition, Parent, links, inherited values, accessibility, retention of every pre-existing history event, and the new event that records the approved conversion or reparenting.
8. Read every other changed ticket back and apply `Jira - Validacion de tickets.md`, including content, type, Parent, children, links, accessibility, and duplicate checks.
   For each directed `Blocks` edge, read both endpoints and satisfy both
   emitted `readBackAssertions`, including each
   `issueScopedCounterpartField`; preserve the returned link ID when available.
   In an issue-scoped `issuelinks` response, the property names the other
   endpoint, not the sentence seen from the current ticket: the blocker view
   contains `inwardIssue=BLOCKED`, while the blocked view contains
   `outwardIssue=BLOCKER`. Only a standalone issue-link response exposes both
   endpoints directly as `outwardIssue=BLOCKER` and
   `inwardIssue=BLOCKED`. Never classify an existing link as inverted from an
   `inwardIssue` or `outwardIssue` property name without also binding the
   current ticket, the counterpart, the live descriptions, and the same link
   ID.
   A one-sided read-back or a relation with the opposite sentence is a failed
   verification, even when the link exists.
9. When authorized Story Points are changed, confirm the approved estimate has medium or high confidence and the exact breakdown shown in the preview. A low-confidence estimate remains unwritten unless the approved preview records the explicit provisional exception.
10. Set the exact verified field once and read the value back.
11. Record stable keys or URLs and non-sensitive verification evidence in the run ledger when the selected branch requires one.

If any create, update, convert, reparent, or link operation fails or returns ambiguous state, stop dependent actions, preserve every known identifier, and mark the step blocked. Reconcile before retrying; do not create a replacement ticket to resolve uncertainty.

## Completion

Jira work-item handling is complete when every planned item is either a persisted checklist entry in its approved field or a verified Jira artifact, every parent and link resolves with the authorized meaning, every directed `Blocks` link was generated from a semantic blocker/blocked edge and passed its two-sided assertions, each independent ticket owns an independently testable result, every subtask remains subordinate to its parent result, every conversion preserves its approved identity and mappings, every requested estimate exposes its evidence, numerical derivation, lower-value comparison, confidence and split-or-Spike decision, every persisted value is verified, and every low-confidence estimate remains an unpointed range unless its provisional exception was explicitly authorized.
