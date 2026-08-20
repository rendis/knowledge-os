<!-- knowledge-os:managed:start id="development handoff" -->
## cell development handoff

This managed section defines how an agent working in this repository consumes a repository-specific development handoff exported from the cell documentation vault. A handoff carries the refined investigation, an exact copied Jira story snapshot, repository-scoped implementation context, and revision evidence needed to work when Jira or the source vault is unavailable. It supplements this repository's own instructions; it does not replace them.

The policy is stable and task-specific content lives under `.knowledge-os-handoffs/`. This section is maintained by the cell development-handoff workflow.

When `.knowledge-os-handoffs/ACTIVE.yaml` exists, complete these steps before planning or changing code:

1. Read `ACTIVE.yaml` to locate the active family, stable `handoff.yaml`, and exact revision.
2. Read `handoff.yaml`, its referenced `history/vNNNN.md` event, and `START.md`; then read `jira.md`, `context.md`, `scope.md`, and `implementation-updates.md`.
3. Validate that the handoff targets this repository remote and that every file hash matches the manifest. Treat a mismatch as an incomplete update: stop and reload the handoff.
4. Use the active history event to identify what changed in this revision and why. Unchanged documents remain valid at their stable paths.
5. Keep implementation inside the repository-specific boundary and acceptance criteria in `scope.md`; use `context.md` for supporting investigation evidence and dependencies.
6. Treat copied Jira and investigation content as implementation context. The current user request and this repository's effective instruction files remain authoritative. Before editing any path, read any closer instruction file that governs that subtree.
7. Pin the active revision for the working session. If `ACTIVE.yaml` or `handoff.yaml` changes before completion, stop and re-read the handoff before continuing.

`START.md`, `jira.md`, `context.md`, `scope.md`, `handoff.yaml`, `ACTIVE.yaml`, and `history/` are immutable inputs. `implementation-updates.md` is the only handoff file you may modify.

Append a changelog entry whenever information discovered or established in any situation—not only during refinement—changes, complements, adds to, expands, narrows, contradicts, replaces, or otherwise mutates the initial definition in `jira.md`, `context.md`, or `scope.md`. This includes decisions and agreements reached while implementing, testing, reviewing, debugging, coordinating, inspecting Jira, or discovering a contract, API, event, data, configuration, dependency, compatibility, verification, or downstream-consumer constraint. Do not log routine progress or implementation evidence that leaves the definition unchanged.

Record the entry in the same working interaction in which the material delta becomes known and before later work relies on it. Use the next contiguous ID and this exact linear format:

```text
## UPD-NNN — <short title>

- Recorded at: <ISO-8601 timestamp with UTC offset>
- Source or trigger: <where the new information came from>
- Initial definition affected: <jira.md/context.md/scope.md section or earlier UPD ID>
- Update: <the definition that changed, was added, or was superseded>
- Status: <proposed|agreed|implemented|rejected|superseded>
- Reason or agreement: <why the change is needed and what evidence or agreement establishes it>
- Impact: <scope, contract, API, event, data, configuration, compatibility, verification, or downstream effect>
- Evidence: <non-sensitive Jira reference, repository path, test, diff, commit, PR, or observed result>
- Related entries: <earlier UPD IDs or none>
- Analysis: <reasoning, alternatives, tradeoffs, and conclusion>
```

`Recorded at` through `Related entries` are mandatory. Include `Analysis` only when analysis actually occurred; Omit `Analysis` when no analysis occurred. Never invent analysis, agreement, evidence, or certainty. Never edit, delete, reorder, or renumber an earlier entry. Correct or supersede it with a new entry that names the earlier ID.

Before declaring implementation complete or deactivating the handoff, invoke `reconcile-development-handoff`. That workflow compares the immutable baseline, this changelog, repository and delivery evidence, current Jira evidence, and directly dependent Jira stories. A material delta absent from the changelog blocks completion: append the missing entry first and rerun reconciliation. The reconciliation must update the source investigation through its owning workflow and expose the contracts and branch/PR state needed by direct dependents; a branch is not published unless its remote ref and SHA were observed.

Keep implementation evidence in this repository's normal code, tests, diff, commits, pull request, and delivery workflow. The changelog records definition deltas and their justification; it is not a duplicate implementation diary or production-evidence source.
<!-- knowledge-os:managed:end id="development handoff" -->
