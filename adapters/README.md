# Adapters

Optional skill packs are selected through `install.sh init --adapter <name>` or the destination's `instance.yaml`, then installed by `update`.

| Directory | Skills | When to enable |
|---|---|---|
| `reports/` | `generate-reports` | Operational report contracts exist under `60-Operacion/` |

`inspect-database` is an engine-neutral kernel skill. Destination runbooks own the database engine, executor, access checks and query dialect.

Provider-specific skills are developer-selected dependencies installed directly in the destination vault. They are not distribution adapters and are not listed in `instance.yaml.adapters`. Bind their runbooks through the existing capability configuration when needed.

## Existing destinations

Before updating a vault that selected the retired `gcp` or `postgres` packs, remove those names from its `instance.yaml.adapters`; preserve the other fields. The installer never rewrites that cell-owned file. The update installs the generic database skill and removes retired files recorded in the previous managed lock; locally changed managed files still require the existing drift resolution. Cell-only files remain untouched.

If the developer wants to retain GCP tooling, preserve the chosen pack outside the vault before updating, then install it as a cell-owned dependency after the update. Keep third-party license and provenance files with the selected tooling. This migration does not install tools or configure access automatically.


## Retired report engine and tracker mappings

The reports pack now contains a generic workflow only. Its former monthly spreadsheet engine, sample query and source adapter are retired managed files. Before updating a destination that uses that engine, the developer must preserve its required implementation and dependencies outside the managed distribution and link the chosen executor from each report’s procedure. Update removes unchanged retired managed files and retains cell-only recipes; remaining recipes alone do not prove that an executor is installed.

Provider-specific tracker mappings and operational rules are also destination-owned. Preserve any required prior mapping before update, put it in a destination procedure and bind it through the `work-item-evidence` capability. Existing tracker IDs, URLs, cases, item references and handoff schemas are unchanged. Generic metadata reads remain available; ambiguous relationship semantics are reported as unavailable.
