# Adapters

Optional skill packs are selected through `install.sh init --adapter <name>` or the destination's `instance.yaml`, then installed by `update`.

| Directory | Skills | When to enable |
|---|---|---|
| `reports/` | `generate-reports` | Operational report contracts exist under `60-Operacion/` |

`inspect-database` is an engine-neutral kernel skill. Destination runbooks own the database engine, executor, access checks and query dialect.

Provider-specific skills (cloud CLIs, trackers, report executors) are developer-selected dependencies installed directly in the destination vault, with their license and provenance files. They are not distribution adapters and are not listed in `instance.yaml.adapters`; bind their runbooks through the capability configuration. Tracker mappings and operational rules are destination procedures, bound through the `work-item-evidence` capability.
