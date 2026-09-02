# Development handoff

Use this family's `handoff.yaml` as the integrity and identity index for its repository-specific story. `ACTIVE.yaml` is the worktree-local lifecycle source of truth and lists every story registered in the shared worktree with its current state.

- `work-item.md` is the exact copied work-item snapshot available at export time.
- `context.md` contains only the investigation context and dependencies relevant to this repository.
- `scope.md` defines this repository's implementation boundary, acceptance criteria, and verification expectations.
- The current `history/vNNNN.md` event records what changed in this revision and why.
- `implementation-updates.md` is the single append-only changelog for material definition changes discovered after the immutable baseline was exported.

Keep the immutable family files read-only and follow the managed root instruction block for the mandatory update-entry format. Implement and verify through the repository's normal workflow and persist material definition deltas in each affected changelog. Change lifecycle only through the managed block's **Set state** route; never edit `ACTIVE.yaml` freehand. The source cell later locates this worktree, pulls its evidence, and reconciles its investigation without becoming a second state owner.
