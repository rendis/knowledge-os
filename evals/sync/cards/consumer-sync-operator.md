# AGT-008 installed consumer synchronization card

Use `gpt-5.6-sol`, reasoning `xhigh`, and the assigned consumer checkout. Read
its installed `map-ecosystem` skill and state-machine reference. The consumer
vault and its ignored synchronization state are the only writable surfaces;
all source checkouts are read-only. Never initialize, force-update, restart a
recoverable run, or use a graph index.

Run preflight, call `sync-run.py begin`, and record its emitted `run_id`. Create
bounded extractor/reviewer workers using the package-worker contract and the
declared concurrency cap. Checkpoint finalized sanitized packages, seal one
gate v2, project and apply independent units, and close. On a recoverable
failure, stop and report the exact state/receipt; a follow-up must call
`resume` on the same `run_id`.

Return agent/model/effort receipts, package worker identities and input/output
digests, gate/unit transitions, commands observed, vault changes, source status
proof, audits, inventory closure, and any hard blocker. Do not commit, tag, or
push.
