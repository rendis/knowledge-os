# Local stores

Versioned knowledge lives under `10/`–`70/`, `investigations/` and `90-Meta/`. Everything below is
local to one checkout and ignored by Git.

| Store | Holds | Owner |
|---|---|---|
| `.investigations/` | Unpublished investigation cases | `manage-investigation` |
| `.investigations-private/<id>/` | What a case needs but never shares: sensitive notes and scratch (scripts, outputs, drafts) | `manage-investigation` |
| `.operations/` | Operational runs | `manage-operational-workflow` |
| `<worktree>/.handoff/` | Task copy and deltas of a development handoff, excluded from Git in each repository worktree | `manage-development-handoff` |
| `.agents/state/discovery/` | Facts, questions, comparison and gaps of the last `discover run` | `kos discover` |
| `.plan/` | Local implementation plans (a visible `plan/` directory would become graph content) | any workflow |
| `.scratch/<task>/` | Helper scripts or code with no selected investigation or workflow location | any workflow |
| `.knowledge-os-config.yaml` | Machine paths and local capability settings | `onboard-developer` |

`investigations/` is versioned and searchable but outside the technical graph traversal and note audit;
`sync verify` gates changed cases with `investigation check`.

When temporary work in `.scratch/<task>/` later belongs to an investigation, move it to the case's
private directory; a file that is the source or method of a result a record cites is attached to that
record instead (`investigation add … --file`). Open an investigation for continuity of evidence and
decisions, not to hold temporary code.
