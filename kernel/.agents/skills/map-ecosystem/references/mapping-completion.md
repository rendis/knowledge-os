# Complete a mapping campaign

Use this contract before declaring an authorized system map or synchronization campaign complete. A package worker stops at its artifact contract. A single-node edit follows its selected recipe. A pending-work query uses the read-only reporting section below.

## Reconcile the mapped scope

After accepted local maps are published, make one bounded reconciliation pass over their pending connections and the directly affected flow notes. Service-local completion does not establish cross-repository completion. Reuse the accepted maps and frozen source revisions; preserve closed packages and receipts.

1. Collect the in-scope questions, their connection IDs, exact targets, environments, and close conditions. Group duplicates only when the target, operation, environment and question match. Preserve links to every affected note; a count of checkboxes is not a count of independent tasks.
2. Check the related accepted vault notes first, following their cited evidence. If they do not answer the question, inspect the exact related repository, pinned dependency, schema or deployment manifest. A note is a navigation aid; an unsupported assertion or a similarly named resource is not closure evidence.
3. Split mixed questions into what static evidence establishes and what remains unknown. Classify each remaining question by its next required evidence source:

   | Evidence source | Examples of the next check |
   | --- | --- |
   | Existing vault evidence | A related note and its exact cited contract already answer the question. |
   | Repository source | A consumer import, shared-library publisher, payload, schema or reusable workflow. |
   | Versioned deployment configuration | An environment binding, IaC resource or manifest value at a specific revision. |
   | Live observation or access | The actual deployed resource, active subscription, delivery or persistence in a named environment. |

   Outside one repository does not mean live infrastructure. An unavailable source repository remains a source-access gap. Record the exact missing target or artifact, not a generic cloud-access request.
4. Resolve the groups answerable from available evidence through [connection-reconciliation.md](connection-reconciliation.md). Technical note updates retain the ordinary publication and independent final-note review gates. Update every affected duplicate reference consistently; keep its original connection identity and useful knowledge.
5. Stop this pass when each group is resolved, partial, or unresolved with a concrete remaining question, next evidence source and close condition. Record inspected evidence or the reason access is unavailable/deferred. An available, directly relevant source left uninspected is unfinished reconciliation, not a completed external backlog. Expand only to sources named by an in-scope question; do not scan unrelated repositories or remap accepted services.

Explicit user deferral limits the pass: retain the deferred scope and reason, and report local maps complete with reconciliation deferred. Existing access rules still apply; campaign closure never grants new infrastructure access. New evidence can resume a pending group later without reopening extraction.

## Align the visible state

After publication and reconciliation, compare the current inventory and accepted publication outcomes with the existing Home coverage summary, the coverage/status note linked from Home, and the current campaign checkpoint. Discover these cell-owned summaries from the vault; do not create a parallel status registry or change historical receipts.

- Use one dated scope and denominator. Distinguish documented repository notes, source freshness, accepted no-change/no-node decisions, and limited/rejected outcomes. `current` alone is not proof of map quality or runtime completeness.
- With vault-update authority, update existing administrative summaries to the observed result. Keep old snapshots explicitly historical with their date and evidence; remove obsolete instructions to resume already completed mapping. Installer updates preserve Home and cell-owned coverage files, so copying the kernel does not perform this step.
- Keep portable current coverage readable in the vault itself. An ignored checkpoint or handoff may supplement it, but must not be the only accurate account. Administrative count/status updates are checked against the inventory and receipts; technical claims still require their ordinary review and write authority. A sync gate does not grant arbitrary writes to Home or Meta files.
- Verify the final visible summaries and checkpoint agree, links remain valid, and unrelated user changes are preserved. An old summary labelled current is a closure defect even when Git and structural checks pass.
- Derive current kernel metadata from the installed `.knowledge-os.lock.yaml`: `kernel_version` and `distribution_revision`. Check installed `VERSION` agrees. Refresh equivalent current fields in the existing checkpoint at closure; never copy them from an old handoff, historical receipt or the vault's own Git HEAD. Preserve historical kernel versions as historical. Missing or inconsistent installation metadata remains explicitly unverified; resolve it through the installer rather than inventing values.

## Report and stop

For repository campaigns with pending checklists, run `python3 -B 90-Meta/check-map-closure.py --vault <vault> --checkpoint <existing-checkpoint>` before closure or commit. In the existing checkpoint, keep current counts under `visible_coverage.remaining_verification_items` and `visible_coverage.notes_with_verifications`, and `paths` pointing to Home and its linked coverage note. Keep dated pass receipts historical; current fields describe the latest aggregate. Avoid duplicating current numbers in a free-text checkpoint scope.

Each visible summary states the current aggregate once as `Remaining: N verification items in M notes.` or `Permanecen N verificaciones en M notas.` (Markdown emphasis is allowed). The checker compares those counts and the checkpoint with unchecked repository-note items; missing or conflicting counts block closure. Correct only stale summaries; preserve technical notes and historical receipts. This count check does not establish semantic completion, deduplicate questions, or replace review of the summary's scope and claims.

Report separately: local mapping outcome, static reconciliation outcome, and remaining live/access/deferred questions. State whether visible coverage is aligned. Declare the campaign complete only when the scoped pass has an evidenced outcome and current summaries agree; partial or deferred knowledge is acceptable when explicit. An incomplete local pass or stale current summary remains an actionable closure task.

For a read-only pending-work query, inspect the current linked summary and latest checkpoint first, compare dates and scope, then read only the relevant pending sections. Report disagreements without editing. Count raw verification items separately from deduplicated groups; classify by evidence source rather than labelling every pending item external. State the observation date of a saved inventory instead of implying a fresh remote check. Stop when this evidence answers the question; old batch logs and broad memory scans are not a prerequisite.
