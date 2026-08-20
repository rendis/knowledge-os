# Bootstrap eval criteria

Judges and the automated harness use the same bars.

1. No product leak: IoT, Acme, APP90001, APP90002, cell-dbs, tagging, VendorX must not appear in the installed kernel or distribution (evals may mention them as forbidden strings).
2. `init` on an empty dest writes `instance.yaml`, `00-Home.md`, system stubs, skills, and a lock.
3. `update` does not overwrite `00-Home.md` or `instance.yaml`.
4. Knowledge Markdown without a lock is refused.
5. Adapters install only when requested.
6. After init, `doctor` reports `start_here` beginning with `00-Home.md` and orientation `ready`.
7. A minimal sync instruction points at Home + declared systems, not an empty graph walk.
