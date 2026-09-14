# Ledger

Accepted local map. register subscribes to shipment-ready and inserts each event
into shipment_ledger. Evidence: sources/ledger/ledger.py, register and TABLE.
Source wiring does not prove this subscriber is deployed or running.

Pending L1: Connect the shipment-ready consumer to a publisher in the ecosystem.
Next evidence: sibling source repository. Previously labelled external verification.

Pending L2: Verify production retention duration for shipment-ready.
No deployment configuration or live provider evidence is available locally.
