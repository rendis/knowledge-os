# Adapters

Optional skill packs copied into a cell vault only when `install.sh init --adapter <name>` (or the interactive prompt) selects them.

| Directory | Skills | When to enable |
|---|---|---|
| `gcp/` | `inspect-gcp-runtime`, `gcloud`, logging query helper, monitoring helper | The cell inspects GCP/GKE |
| `postgres/` | `inspect-database` | A schema repository is declared in `instance.yaml` |
| `reports/` | `generate-reports` | Operational report recipes exist under `60-Operacion/` |

Never copied by a default init. Never part of a kernel `update` unless already listed in `instance.yaml` `adapters`.
