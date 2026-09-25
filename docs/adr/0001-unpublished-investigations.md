# Unpublished investigations are full cases until publish

Keeping every new case in versioned `investigations/` made each case team-visible before it deserved to be,
and machine-local material leaked into shareable artifacts.

New investigations are unpublished full cases under `.investigations/`, with the same contract and gate as
published ones. Material a case needs but must not share (sensitive notes, scratch scripts, raw outputs)
lives in `.investigations-private/<id>/`. Publishing is an explicit one-way move on a sync branch; afterwards
the published case is the authority and changes only on sync branches.

**Considered options**: keep public-from-open and add more ignore rules; keep two full copies after publish;
use a visibility flag inside one tree.

**Consequences**: an unpublished case cannot be the sole durable source for vault knowledge; collaboration
through Git starts at publish.
