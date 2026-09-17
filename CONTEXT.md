# Investigation visibility

A cell investigation is a case file with one contract. Visibility and local machine material are separate from that contract.

## Language

**Investigation**:
A case that records request, evidence, decisions, and acceptance criteria under the investigation contract.
_Avoid_: dossier, ticket, chat log

**Published investigation**:
The versioned, shareable case under `investigations/`. After publish it is the sole authority for status, evidence, decisions, and acceptance criteria.
_Avoid_: public investigation as internet-visible, canonical overlay

**Unpublished investigation**:
The same case contract stored only under `.investigations/`, ignored by Git. New investigations start here.
_Avoid_: draft overlay, private overlay, legacy case

**Private overlay**:
A non-authoritative sensitive complement at `.investigations-private/<id>/private.md`. It never restates or overrides investigation knowledge.
_Avoid_: private investigation, second case file

**Local working material**:
Machine- or tool-specific files needed to continue locally (HTTP collections, smoke dumps, host paths). They live under `.investigations-private/<id>/local/` and are not case knowledge.
_Avoid_: public artifact, overlay note, gitignored case attachment

**Investigation case file**:
The authoritative `investigation.md`, unpublished or published. Distinct from the private overlay and from local working material.
_Avoid_: public case as a synonym of published investigation

**Publish**:
The one-way, sanitized move of an unpublished investigation into `investigations/`. One ID keeps at most one `investigation.md`. There is no unpublish.
_Avoid_: sync, copy both ways, promote-to-vault, learning-note write, ticket or story platform publication
