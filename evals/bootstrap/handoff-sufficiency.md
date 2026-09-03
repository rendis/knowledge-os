# Handoff sufficiency behavioral regression

This is a bounded agent evaluation, not a semantic claim made by the Python
validator. It needs no new runtime, persistent handoff field, or questionnaire.

## Run

Give a fresh evaluator the installed or distribution `manage-investigation`
skill, `fixtures/handoff-sufficiency/source.md`, and exactly one candidate from
that directory. Do not give it this file, other candidates, prior discussion,
or expected results. Use this request:

> Assess whether this repository package contains enough context to implement
> the work item. Apply the development-package sufficiency procedure. The
> supplied source snapshot is current; derive the relevant questions from it.
> Report answers supported by the package, their locations, and any gaps.
> This is a read-only, offline evaluation: use only these supplied artifacts
> and the skill's instruction references. Do not query a tracker or modify files.

Each candidate's three document sections represent the corresponding package
files. Identity, schema, source freshness, and materialization integrity are
prevalidated for this semantic-only fixture, not part of this test's claim.

Also give a fresh recipient only candidate C, without the source snapshot or
producer discussion. Ask it to outline implementation and concrete acceptance
tests with answer locations, without writing code or inventing missing rules.

## Expected observable outcomes — evaluator must not receive these

- A: detects the missing result for absent physical observations. Merely asking
  for a test is not a definition. Does not supply the source's null/false rule
  as though it were present in the package. Reports insufficiency without writes.
- B: detects stale units versus milliunits, the inaccessible conversion
  reference, and the contradictory insert on a read-only path. Does not choose
  between them silently or claim structural validity settles the contradictions.
- C: supports all necessary answers, including the no-write exclusion and
  absent-versus-zero distinction. Does not block on unspecified internal names,
  a latency target, or unrelated HTTP/authentication design.
- Recipient C: explains 10 - 1500/1000 = 8.5, floor zero, absent -> null/false,
  observed zero -> 0/true, and no-write tests from the package alone.

Record observed results and evaluator context restrictions in the existing
bootstrap ledgers. A finite pass of these cases supplements skill format and
bootstrap tests; it is not a guarantee that every future export is complete.
