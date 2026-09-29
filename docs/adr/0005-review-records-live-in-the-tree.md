# Review records live in the tree

**Status**: accepted (v0.22.5)

**Context**: `sync review` recorded the verdict only in the trailers of an empty commit, and the push gate
(`sync verify --allow-no-change`, ADR 0004) looked for it on the first-parent history. Every GitHub merge method
but a fast-forward lost it: a rebase merge drops empty commits, a squash may drop the trailers, and a merge commit
keeps the review on its second parent. A reviewed pull request merged by rebase failed the gate on the base
although the published content matched the reviewed digest.

**Decision**:

- `sync review` also writes `90-Meta/sync-reviews/<time>-<verdict>-<digest>.json` in the review commit: verdict,
  reviewer, summary, base, digest and, per knowledge file, its blob before and after the reviewed change.
- The push gate checks content: every knowledge file the range changes must go from its blob at the range's base
  to its blob at HEAD through the transitions of the accepted records the range adds. An edit no review saw
  breaks the chain; several syncs in one push chain.
- A range is also accepted when the trailer check on the first-parent history covers it, so branches reviewed
  with an earlier kos still publish by fast-forward.
- A published range whose review was lost is recovered by recording a review of it against the base before the
  merge, on a sync branch that adds only the record.

**Considered options**: requiring fast-forward merges only (a repository setting kos cannot enforce); following
merge parents and parsing squash messages (history still loses the rebase case); git notes (not pushed or
fetched by default, not kept by GitHub merges).

**Consequences**: the review survives any merge method that keeps the content. The records accumulate in the
cell under `90-Meta/`, beside the distribution's files, and a kernel update keeps them. A rebase onto a base that
changed a reviewed file breaks the chain and needs a new review, as the merged meaning does.
