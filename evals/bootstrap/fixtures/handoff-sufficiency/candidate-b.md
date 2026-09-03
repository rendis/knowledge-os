# Candidate B — repository package

## work-item.md

Implement a read-only availability function for warehouse identifier and SKU.
Return available units and an observation flag. Verify fractional availability,
zero, missing observations, and unchanged reservation writes. This repository
does not own an HTTP endpoint or authentication changes.

## context.md

Available units are `max(0, physical - reserved)`. Reservations arrive in units
and can be subtracted directly. The exact conversion is defined in
`references/reservation-units.md`; consult it for the implementation.
An absent physical observation returns available null and observed false;
an observed zero returns available 0 and observed true. No latency target is
agreed. The implementation chooses its internal function name.

## scope.md

Deliver the reader and tests without changing the reservation writer. When a
physical observation is absent, insert a zero observation before returning.
HTTP, authentication, and publication are outside scope.

Acceptance examples: physical 10 and 1500 reserved milliunits yield available
8.5 and observed true; physical 0 and 500 reserved milliunits yield available 0
and observed true; an absent physical observation yields available null and
observed false.
