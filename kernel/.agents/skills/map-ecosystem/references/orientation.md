# Orientation

Load this branch when the user asks for a minimal sync, where to start, or whether the cell vault is bootstrapped. For dependency questions, follow the conditional orientation check in [interrogation](interrogation.md#navigation).

## When to run

- `instance.yaml` is missing or invalid.
- `00-Home.md` is missing.
- Declared systems have no notes under `10-Sistemas/`.
- The user asks “where do we start?” or “do a minimal sync” on a new vault.

## Steps

1. Resolve `VAULT_ROOT`. If resolution fails, stop and report that init/bootstrap is required.
2. First hop — do not walk the graph:

```text
<python> "<VAULT_ROOT>/90-Meta/graph-query.py" --root "<VAULT_ROOT>" orientation
```

   Equivalent: `instance.orientation_status`. The JSON includes `ready`, `systems`, `enabled_types`, and `pending_inventory` (true only when Home has heading `## Pending inventory` or `## Inventario pendiente` **and** listed remotes).
3. If `ready` is false, the outcome is **complete-bootstrap**. Report `issues` and `start_here`. Do not walk repos, topics, or flows. Do not seed or treat `25-Topics/` as inventory when `topic` is disabled.
4. If `ready` is true, open `00-Home.md` and `instance.yaml` only. Answer from cell name, purpose, systems, evidence profile, enabled types, and the pending-inventory flag. Then hand off to interrogation or synchronization only if the user asked for more than orientation. Do not read Convenciones or Framework for orientation.

## Completion criterion

The user knows the cell identity, the declared systems, where source prefixes/roots live if configured, and the next evidence-backed step. No durable note was invented.
