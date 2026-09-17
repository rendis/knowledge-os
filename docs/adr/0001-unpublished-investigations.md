# Unpublished investigations are full cases until publish

The earlier collaboration model kept every new case in versioned `investigations/` and limited `.investigations-private/` to a non-authoritative overlay, rejecting a complete private copy because two `investigation.md` files would compete. Agents then wrote machine-local tool workspaces into shareable artifacts, and every opened case became team-visible before it deserved to be.

New investigations are unpublished full cases under `.investigations/` with the same contract as published ones. Local tool configuration and dumps go to `.investigations-private/<id>/local/`, not gitignore rules inside `artifacts/`. Publish is an explicit one-way sanitized move; afterwards the published case is canonical and the unpublished directory for that ID is gone. The overlay stays supplementary. This reverses “never a complete private copy” only *before* the first publish.

**Considered Options**: keep public-from-open and add more gitignores; keep two full copies after publish; use a visibility flag inside one tree.

**Consequences**: ignored unpublished cases cannot be the sole durable source for `10/`–`70/` learnings; collaboration through Git starts at publish; leftover pre-cutover files in `.investigations/` that are not the current schema remain migrate-only.
