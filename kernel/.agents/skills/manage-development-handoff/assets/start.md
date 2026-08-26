# Development handoff

Use `handoff.yaml` as the integrity and identity index for this repository-specific task.

- `jira.md` is the exact copied Jira story snapshot available at export time.
- `context.md` contains only the investigation context and dependencies relevant to this repository.
- `scope.md` defines this repository's implementation boundary, acceptance criteria, and verification expectations.
- The current `history/vNNNN.md` event records what changed in this revision and why.
- `implementation-updates.md` is the single append-only changelog for material definition changes discovered after the immutable baseline was exported.

Keep every file except `implementation-updates.md` read-only. Follow the managed root instruction block for the mandatory update-entry format. Implement and verify through the repository's normal workflow, persist material definition deltas and their evidence references in the changelog, and leave `ACTIVE.yaml` intact. The vault later locates this worktree and pulls the evidence needed for reconciliation and closure.
