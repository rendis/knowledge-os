# Local stores

Versioned knowledge lives under `10/`–`70/`, `investigations/` and `90-Meta/`. Everything below is
local to one checkout and ignored by Git.

| Store | Holds | Owner |
|---|---|---|
| `.investigations/` | Unpublished investigation cases | `manage-investigation` |
| `.investigations-private/<id>/` | Private overlay (`private.md`) and local working material (`local/`) of a case | `manage-investigation` |
| `.operations/` | Operational runs | `manage-operational-workflow` |
| `.knowledge-os-handoffs/` | Handoff working state | `manage-development-handoff` |
| `.agents/state/discovery/` | Facts, questions, comparison and gaps of the last `discover run` | `vaultctl discover` |
| `.plan/` | Local implementation plans (a visible `plan/` directory would become graph content) | any workflow |
| `.scratch/<task>/` | Helper scripts or code with no selected investigation or workflow location | any workflow |
| `.knowledge-os-config.yaml` | Machine paths and local capability settings | `configure-workspace` |

`investigations/` is versioned and searchable but outside the technical graph traversal and audit
gates; `links` still returns investigation pointers.

When temporary work in `.scratch/<task>/` later belongs to an investigation, move it through
`manage-investigation`: working material into the case's private `local/` store, reviewed methods or
evidence worth retaining into its `artifacts/` register. Remove only the files the task created after
verifying the move. Open an investigation for continuity of evidence and decisions, not to hold
temporary code.
