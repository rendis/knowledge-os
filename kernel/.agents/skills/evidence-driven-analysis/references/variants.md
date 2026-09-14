# Related manifestations

Use after a cause or pattern has been confirmed, when finding related occurrences is within scope.

- Define the failing condition semantically, including the preconditions that make it harmful. Search related code or flows using available repository tools; verify that the search can locate the known occurrence before relying on its coverage.
- Broaden to equivalent inputs, transformations, callers, or consumers as needed. Keep expansion tied to the confirmed cause rather than treating every textual resemblance as a defect.
- Inspect each candidate's surrounding contract and execution path. A guard, different unit, unreachable path, or enforced precondition may disprove the match. Separate confirmed variants, rejected look-alikes, and unresolved candidates.
- Stop at the requested boundary and report relevant coverage limits. Search results are leads, not a finding count; neither an automatic repository-wide audit nor a prevention-rule artifact is required.
