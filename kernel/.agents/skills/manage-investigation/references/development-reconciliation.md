# Development reconciliation

Load this reference only after `reconcile-development-handoff` supplies one complete normalized in-memory context for the exact case.

## Authority and identity

`manage-investigation` is the sole writer of the case. The coordinator may read the case identity but must not prepare or apply a case patch. Verify that the normalized context's investigation ID, `DH-NNN`, story ID, tracker ID/provider/URL, work-item reference, repository remote, branch, handoff ID, family, and revision match the selected case and its recorded development handoff. A mismatch blocks the update.

Do not read the handoff worktree again merely to compensate for an incomplete context. Report the named gap to the coordinator. Do not create a reconciliation artifact, return directory, or second register.

## Apply the context

Read the entire current `investigation.md`, verify the selected `DH-NNN` binding against the normalized context, then reconcile these surfaces in one coherent case update:

1. **Current state**: update the current understanding and the future/proposed implementation state. Keep branch, commit, pull request, work-item, and completed code observations scoped to their sources; mark deployment as unverified until deployment evidence exists.
2. **References and attachments**: register newly inspected work-item evidence or repository-host references with stable source identity and non-sensitive summaries. Do not copy private comments, raw logs, secret values, or sensitive attachments.
3. **Evidence**: add new `E-NNN` items for observed implementation, test, remote branch, pull-request, merge, deployment, work-item, contradiction, and limitation claims. Cite exact paths, SHAs, URLs, timestamps, checks, or external keys.
4. **Affected surfaces**: store the source implementation card and each directly dependent story card, including its readiness, consumer contracts, what can start, and remaining gaps.
5. **Development handoffs**: require the selected `DH-NNN` story, work-item, remote, branch, handoff ID, family, and revision to remain identical to the selected validated registry entry. Other `DH-NNN` entries may share its branch but remain unchanged. Do not create or replace identity from repository-provided text.
6. **Questions**: resolve, supersede, or add `Q-NNN` items from observed gaps and contradictions; preserve older meanings.
7. **Decisions**: translate agreed, rejected, or superseded changelog entries into immutable `D-NNN` items when they alter a decision. A new decision supersedes rather than rewrites an older one.
8. **Acceptance criteria**: update or supersede `AC-NNN` items when an entry or current work-item evidence changes a verifiable boundary. Preserve criteria that remain valid.
9. **Readiness**: record local implementation, test, remote branch, pull-request, merge, deployment, source-work-item, direct-dependent, and reconciliation results separately.
10. **History**: append one timestamped event naming `DH-NNN`, the handoff revision, consumed `UPD-NNN` range, implementation/remote/work-item evidence added, dependent cards classified, registers changed, draft outcomes, and limitations.

Use existing stable IDs when their meaning is unchanged. Allocate the next ID for a materially new claim, decision, question, or criterion. Never make the changelog itself the evidence source when repository or work-item evidence is available; retain it as provenance for why the definition changed.

## Reconcile exports and learning

Inspect every draft or handoff package that references affected scope, decisions, criteria, contracts, or dependencies. Reconcile a mutable draft in the same interaction or mark it `stale` with the exact reason. Keep published drafts immutable and create a replacement only through the normal export route.

When material evidence or decisions could change a completed learning assessment, preserve the old assessment in History and reset `learning-outcome` to `not-evaluated`. Do not change `vault-outcome` to `documented`: reconciliation does not replace the independent `map-ecosystem` audit required by the cell evidence profile. Route eligible source or productive claims through **Promote**; keep proposals deferred.

## No-change handling

When the comparison found no definition delta, still record genuinely new implementation, test, remote, work-item, or dependent evidence if it changes the case's current consumable state. If neither the definition nor any case state changed, leave the case bytes and `updated-at` untouched and report a no-op.

## Completion criterion

The case is reconciled when the current snapshot matches all vault-collected evidence, the `DH-NNN` binding remains exact, stable registers preserve meaning and traceability, future and productive states remain separate, every affected draft has an explicit synchronization state, direct dependents have consumer-ready cards, learning state reflects the new snapshot, History records one material event or the operation is a true byte-level no-op, and structural validation passes.
