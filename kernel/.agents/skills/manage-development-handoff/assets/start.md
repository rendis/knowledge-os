# Development handoff

Use this family's `handoff.yaml` as the integrity and identity index for its repository-specific story. `ACTIVE.yaml` lists every story registered in the shared worktree and its current state.

- `work-item.md` is the exact copied work-item snapshot available at export time.
- `context.md` contains only the investigation context and dependencies relevant to this repository.
- `scope.md` defines this repository's implementation boundary, acceptance criteria, and verification expectations.
- The current `history/vNNNN.md` event records what changed in this revision and why.
- `implementation-updates.md` is the single append-only changelog for material definition changes discovered after the immutable baseline was exported.

Keep every file except this family's `implementation-updates.md` read-only. Follow the managed root instruction block for the mandatory update-entry format. Implement and verify through the repository's normal workflow, persist material definition deltas and their evidence references in each affected changelog, and leave `ACTIVE.yaml` intact. The vault later locates this worktree, pulls its evidence, and updates the selected story state.
