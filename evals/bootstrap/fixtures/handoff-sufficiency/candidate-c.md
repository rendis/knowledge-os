# Candidate C — repository package

## work-item.md

Implement a read-only availability function for warehouse identifier and SKU.
Return available units and an observation flag. Verify fractional availability,
zero, missing observations, and unchanged reservation writes. This repository
does not own an HTTP endpoint or authentication changes.

## context.md

Available units are `max(0, physical - reserved)`. The current reconciled
reservation contract exposes `reserved_milliunits`: divide by 1000 before
subtraction and preserve fractions. This replaces the old units convention.

An absent physical observation yields available null and observed false;
an observed zero yields available 0 and observed true. These are distinct cases.
Do not create observations or adjust reservations as part of reading.

The implementation chooses its internal function name. No latency target has
been agreed; neither is a pending business decision.

## scope.md

Deliver the reader and regression tests. Keep the reservation writer unchanged.
HTTP, authentication, and publication are outside scope.

Acceptance examples: physical 10 and 1500 reserved milliunits yield available
8.5 and observed true; physical 0 and 500 reserved milliunits yield available 0
and observed true; an absent physical observation yields available null and
observed false. Verify no writes occur in all three cases.
