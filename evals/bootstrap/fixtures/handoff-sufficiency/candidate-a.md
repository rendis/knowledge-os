# Candidate A — repository package

## work-item.md

Implement a read-only availability function for warehouse identifier and SKU.
Return available units and an observation flag. Verify fractional availability,
zero, missing observations, and unchanged reservation writes. This repository
does not own an HTTP endpoint or authentication changes.

## context.md

Available units are `max(0, physical - reserved)` using the current reconciled
reservation contract: divide `reserved_milliunits` by 1000 before subtraction,
preserving fractions. An observed zero yields available 0 and observed true.
The implementation chooses its internal function name. No latency target has
been agreed; neither choice blocks this story.

## scope.md

Deliver the reader and regression tests. It must never create observations,
adjust reservations, or modify the existing reservation writer. HTTP,
authentication, and publication are outside scope.

Acceptance examples: physical 10 and 1500 reserved milliunits yield available
8.5 and observed true; physical 0 and 500 reserved milliunits yield available 0
and observed true. Add a missing-observation test as well.
