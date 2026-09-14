# Dispatch

Accepted local map. Publishes shipment_id events to shipment-ready.
Evidence: sources/dispatch/dispatch.py, publish_shipment.

## Bounded reconciliation — 2026-09-13

D1 — resolved (repository source): Ledger's `register` subscribes to the same literal topic `shipment-ready` and routes each event to `database.insert(TABLE, event)`, with `TABLE = "shipment_ledger"`. Dispatch publishes `{"shipment_id": shipment_id}` to that literal topic. This identifies the source-level consumer and its programmed persistence effect; actual successful production insertion remains unobserved. Related accepted map: [Ledger](ledger.md).

Evidence inspected: [dispatch.py](../../sources/dispatch/dispatch.py), lines 1–5, `publish_shipment`, revision `fbfca68be65e5430a4ca392d34f80deedfd37cd2`; [ledger.py](../../sources/ledger/ledger.py), lines 1–6, `register` and `TABLE`, revision `2c9aed07322d25a74e3f536a6b043bc8899bc66d`. Source HEADs match the supplied revision files. These establish application source wiring, not production resource identity, deployment, delivery or persisted data.

## Pending

D2 — unresolved: What retention duration is deployed for `shipment-ready` in production? Same question as L2 in [Ledger](ledger.md); one shared remaining group. No deployment configuration or live provider evidence is supplied. Next evidence: versioned production deployment configuration, if available, to establish the exact provider/project/resource binding and configured retention; then an authorized read-only observation of that exact deployed topic to establish active retention. Close only with evidence identifying the production resource and its deployed retention duration at an observation time. Live access is unavailable and excluded from this fixture; no access setup was requested.
