# Ledger

Accepted local map. register subscribes to shipment-ready and inserts each event
into shipment_ledger. Evidence: sources/ledger/ledger.py, register and TABLE.
Source wiring does not prove this subscriber is deployed or running.

## Bounded reconciliation — 2026-09-13

L1 — resolved (repository source): Dispatch's `publish_shipment` publishes `{"shipment_id": shipment_id}` to literal topic `shipment-ready`, which Ledger's `register` subscribes to. This connects the source-level consumer to its ecosystem publisher. Related accepted map: [Dispatch](dispatch.md).

Evidence inspected: [dispatch.py](../../sources/dispatch/dispatch.py), lines 1–5, `publish_shipment`, revision `fbfca68be65e5430a4ca392d34f80deedfd37cd2`; [ledger.py](../../sources/ledger/ledger.py), lines 1–6, `register` and `TABLE`, revision `2c9aed07322d25a74e3f536a6b043bc8899bc66d`. Source HEADs match the supplied revision files. These establish application source wiring, not production resource identity, deployment, delivery or persisted data.

## Pending

L2 — unresolved: What retention duration is deployed for `shipment-ready` in production? Same question as D2 in [Dispatch](dispatch.md); one shared remaining group. No deployment configuration or live provider evidence is supplied. Next evidence: versioned production deployment configuration, if available, to establish the exact provider/project/resource binding and configured retention; then an authorized read-only observation of that exact deployed topic to establish active retention. Close only with evidence identifying the production resource and its deployed retention duration at an observation time. Live access is unavailable and excluded from this fixture; no access setup was requested.
