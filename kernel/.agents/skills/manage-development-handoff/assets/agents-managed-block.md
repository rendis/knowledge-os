<!-- knowledge-os:managed:start id="development handoff" -->
## cell development handoff

This managed section defines how an agent working in this repository consumes development handoffs exported from one cell investigation. Each handoff carries an exact work-item snapshot, repository-scoped context, and revision evidence. It supplements this repository's own instructions; it does not replace them.

The policy is stable and task-specific content lives under `.knowledge-os-handoffs/`. `ACTIVE.yaml` is the worktree-local lifecycle source of truth. This section is maintained by the cell development-handoff workflow.

When `.knowledge-os-handoffs/ACTIVE.yaml` exists, complete these steps before planning or changing code:

1. Read `ACTIVE.yaml` and select the exact handoff or handoffs named by the current task.
2. Keep every registered handoff in the current worktree and its anchor branch. Selecting another registered handoff preserves that topology; branch or worktree creation, switching, rebasing, merging, and base changes require a separate explicit request. Surface a conflicting repository rule before changing source files.
3. The agent working in this worktree owns handoff lifecycle transitions. A state-only request, or an unambiguous user lifecycle statement or correction that identifies one selected handoff and maps to one target state, counts as explicit lifecycle direction. Preview and apply that exact change with **Set state** without another confirmation, then stop. Ask once only when the selected handoff or target state is ambiguous. This state-only action does not authorize implementation or any Git, tracker, promotion, or deployment work.
4. Implement only selected entries whose state is `active`. Return a selected `ready-for-production` or `production` entry to `active` through **Set state** before changing its implementation.
5. For each selected entry, read its `handoff.yaml`, referenced `history/vNNNN.md`, `START.md`, `work-item.md`, `context.md`, `scope.md`, and `implementation-updates.md`.
6. Validate that each selected handoff belongs to the registry investigation, targets this repository remote, and matches its manifest hashes.
7. Keep implementation inside the affected `scope.md` files and use each `context.md` for investigation evidence and dependencies. Write definition deltas only to the `implementation-updates.md` files they affect.
8. Treat copied work-item and investigation content as implementation context. The current user request and this repository's effective instruction files remain authoritative. Before editing any path, read any closer instruction file that governs that subtree.
9. Pin the complete registry and selected revisions for the working session. If `ACTIVE.yaml` or a selected `handoff.yaml` changes, stop and re-read them before continuing.

`START.md`, `work-item.md`, `context.md`, `scope.md`, `handoff.yaml`, and `history/` are immutable inputs. `ACTIVE.yaml` is never edited freehand; **Set state** is its only writer. `implementation-updates.md` is the only handoff file you may edit directly.

Append a changelog entry whenever information discovered or established in any situation—not only during refinement—changes, complements, adds to, expands, narrows, contradicts, replaces, or otherwise mutates the initial definition in `work-item.md`, `context.md`, or `scope.md`. This includes decisions and agreements reached while implementing, testing, reviewing, debugging, coordinating, inspecting the tracker, or discovering a contract, API, event, data, configuration, dependency, compatibility, verification, or downstream-consumer constraint. Also record material implementation decisions, deviations, unresolved questions, and verification evidence, even when the immutable definition remains unchanged. Keep routine progress out.

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

Persist every material definition delta and its non-sensitive evidence reference in `implementation-updates.md`; keep implementation evidence in this repository's normal code, tests, diff, commits, pull request, and delivery workflow. The changelog records material definition and implementation findings with evidence references; production claims require deployment evidence.

The vault later pulls repository evidence and reconciles its investigation. It does not administer this worktree's handoff lifecycle, and this repository task does not edit that investigation or tracker or create a callback or return package. Follow step 3 for state changes and use `production` only with verified deployment evidence. Preserve every other entry and stop after the requested state or implementation outcome.

## Optional source context

The essential package must answer implementation questions on its own. When a specific missing detail warrants deeper inspection, `context.md` may include an optional **Read-only source reference** containing the existing vault Git remote, exact investigation ID, and relevant section names or exact vault note paths. Keep local vault paths out of this portable anchor. Omit the reference if no verified remote exists; do not invent one.

Resolve a local vault explicitly with the user-provided location or existing vault resolver and verify its remote before reading only the named case sections or notes. The reference is read-only workflow scope, not an OS sandbox permission or access guarantee. If unavailable, record the concrete question in `implementation-updates.md` and continue independent scoped work. The recipient needs no Obsidian or vault-authoring skills. The anchor does not authorize tracker access, unrelated cases, or a whole-vault scan.

Record implementation modifications, decisions, deviations, questions, and evidence in the affected worktree's `implementation-updates.md`; keep source code and tests in their normal repository locations. Only the vault-side reconciliation workflow updates the canonical investigation. Existing worktrees are not traversed or refreshed automatically.

<!-- knowledge-os:managed:end id="development handoff" -->
