# Availability reader — current source snapshot

The task is to export an implementation handoff for the warehouse repository.
The current work item requires a read-only function that receives a warehouse
identifier and SKU, and returns available units plus an observation flag.

## Current decisions and acceptance criteria

- D-001 (active): available units are `max(0, physical - reserved)`.
- D-002 (active): an absent physical observation returns `available: null,
  observed: false`. An observed zero returns `available: 0, observed: true`.
- D-003 (active): read only; do not create observations or adjust reservations.
- D-004 (superseded): reservation quantities arrive in units. Replaced by D-005.
- D-005 (active): the reconciled reservation contract exposes thousandths of a
  unit. Convert `reserved_milliunits / 1000` before subtraction; fractional
  available units remain exact.
- AC-001: physical 10, reservations 1500 milliunits -> available 8.5, observed true.
- AC-002: physical 0, reservations 500 milliunits -> available 0, observed true.
- AC-003: absent physical observation -> available null, observed false.
- AC-004: the existing reservation writer remains unchanged; no HTTP endpoint,
  authentication change, or publication is part of this work.

No response-time target has been agreed. Choosing an internal function name is
left to implementation. These are not missing business decisions.

## Evidence boundary

The reconciled contract above is registered source evidence. It may be carried
in the package. No additional files or links are available to the recipient.
