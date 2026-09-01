<!-- knowledge-os:managed:start id="development handoff" -->
## cell development handoff

This managed section defines how an agent working in this repository consumes development handoffs exported from one cell investigation. Each handoff carries an exact work-item snapshot, repository-scoped context, and revision evidence. It supplements this repository's own instructions; it does not replace them.

The policy is stable and task-specific content lives under `.knowledge-os-handoffs/`. This section is maintained by the cell development-handoff workflow.

When `.knowledge-os-handoffs/ACTIVE.yaml` exists, complete these steps before planning or changing code:

1. Read `ACTIVE.yaml` and select the exact handoff or handoffs named by the current task.
2. Work only on selected entries whose state is `active`. A `ready-for-production` or `production` entry must be returned to `active` by the vault before it is changed.
3. For each selected entry, read its `handoff.yaml`, referenced `history/vNNNN.md`, `START.md`, `work-item.md`, `context.md`, `scope.md`, and `implementation-updates.md`.
4. Validate that each selected handoff belongs to the registry investigation, targets this repository remote, and matches its manifest hashes.
5. Keep implementation inside the affected `scope.md` files and use each `context.md` for investigation evidence and dependencies. Write definition deltas only to the `implementation-updates.md` files they affect.
6. Treat copied work-item and investigation content as implementation context. The current user request and this repository's effective instruction files remain authoritative. Before editing any path, read any closer instruction file that governs that subtree.
7. Pin the complete registry and selected revisions for the working session. If `ACTIVE.yaml` or a selected `handoff.yaml` changes, stop and re-read them before continuing.

`START.md`, `work-item.md`, `context.md`, `scope.md`, `handoff.yaml`, `ACTIVE.yaml`, and `history/` are immutable inputs. `implementation-updates.md` is the only handoff file you may modify.

Append a changelog entry whenever information discovered or established in any situation—not only during refinement—changes, complements, adds to, expands, narrows, contradicts, replaces, or otherwise mutates the initial definition in `work-item.md`, `context.md`, or `scope.md`. This includes decisions and agreements reached while implementing, testing, reviewing, debugging, coordinating, inspecting the tracker, or discovering a contract, API, event, data, configuration, dependency, compatibility, verification, or downstream-consumer constraint. Do not log routine progress or implementation evidence that leaves the definition unchanged.

Record the entry in the same working interaction in which the material delta becomes known and before later work relies on it. Use the next contiguous ID and this exact linear format:

```text
## UPD-NNN — <short title>

- Recorded at: <ISO-8601 timestamp with UTC offset>
- Source or trigger: <where the new information came from>
- Initial definition affected: <work-item.md/context.md/scope.md section or earlier UPD ID>
- Update: <the definition that changed, was added, or was superseded>
- Status: <proposed|agreed|implemented|rejected|superseded>
- Reason or agreement: <why the change is needed and what evidence or agreement establishes it>
- Impact: <scope, contract, API, event, data, configuration, compatibility, verification, or downstream effect>
- Evidence: <non-sensitive work-item reference, repository path, test, diff, commit, PR, or observed result>
- Related entries: <earlier UPD IDs or none>
- Analysis: <reasoning, alternatives, tradeoffs, and conclusion>
```

`Recorded at` through `Related entries` are mandatory. Include `Analysis` only when analysis actually occurred; Omit `Analysis` when no analysis occurred. Never invent analysis, agreement, evidence, or certainty. Never edit, delete, reorder, or renumber an earlier entry. Correct or supersede it with a new entry that names the earlier ID.

The repository has no vault-closure responsibility. Persist every material definition delta and its non-sensitive evidence reference in `implementation-updates.md`; keep implementation evidence in this repository's normal code, tests, diff, commits, pull request, and delivery workflow. The changelog records definition deltas and their justification rather than duplicating the implementation diary or claiming production state.

Leave `.knowledge-os-handoffs/ACTIVE.yaml` intact when repository work completes. Do not invoke a vault workflow, edit the source investigation or tracker, update registry state, or create, send, or return a reconciliation package. The vault independently resolves this worktree from its investigation register, reads its handoffs and repository evidence, reconciles its own case, and owns authorized state changes.
<!-- knowledge-os:managed:end id="development handoff" -->
