# documentation-vault

A portable **knowledge OS** for a team or cell: an Obsidian vault kernel, agent skills, evidence gates, and an installer that bootstraps a new vault without copying another team's domain knowledge.

This repository is the **distribution / template**. A cell vault is a separate directory created by `install.sh`. Do not treat this checkout as a knowledge graph. Agents working **on this repo** start at [`AGENTS.md`](AGENTS.md). Evals stay here and are never copied into a cell.

## Install

```bash
./install.sh init --dest /path/to/cell-vault \
  --cell-name "Payments" \
  --purpose "Card-present checkout and settlement" \
  --system payments:Payments \
  --yes
```

Without flags, `init` asks for cell name, purpose, systems, optional trackers, adapters, evidence profile, and note locale. Non-interactive init accepts repeatable `--tracker id:provider:https://url`; omit it when the cell has no external work tracker.

```bash
./install.sh update --dest /path/to/cell-vault
./install.sh doctor --dest /path/to/cell-vault
```

An existing vault that already has `00-Home.md` and `10-Sistemas/` (no lock) is adopted, not initialized. Write `instance.yaml` first; Home and notes are never rewritten:

```bash
./install.sh adopt --dest /path/to/existing-vault
```

With no arguments, the script looks at the current directory: a `.knowledge-os.lock.yaml` selects `update`; an empty destination selects `init`. Knowledge Markdown without a lock is refused. The lock is portable, contains no local paths or secrets, and must be committed with the cell so a clean clone remains an installed consumer.

A future published tag can be installed with:

```bash
curl -fsSL <raw-tag>/install.sh | bash -s -- init --dest /path/to/cell-vault
```

The current distribution channel is local-only. The script is written so that one-liner can clone a tag into cache and re-enter the same verbs.

## After init: where to start

The cell vault owns:

- `instance.yaml` — who the cell is, systems, trackers, source prefixes, evidence profile
- `00-Home.md` — orientation; a later `update` does not overwrite it
- `10-Sistemas/` — one stub note per declared system
- `10/`–`70/` — the cell's knowledge graph

The distribution owns the thin `AGENTS.md` router, every generic file it ships under `90-Meta/`, kernel skills, selected adapter skills, `VERSION`, and the `CLAUDE.md` / `.claude/skills` symlink topology. `update` refreshes matching distribution files without deleting cell-only Meta files, skills, recipes, or overlays. The cell owns `instance.yaml`, `00-Home.md`, the root Bases, `90-Meta/Alcance.md`, and notes under `10/`–`70/`; those files are seeded or authored locally and never rewritten by `update`.

`update` refreshes only the consumer vault; it never traverses development worktrees. An existing worktree receives the current managed instruction block during its next authorized handoff mutation.

Versions 0.5 and 0.6 use work-item bundle and manifest schema 2. Jira-specific schema-1 packages or materialized families are not reinterpreted in place; configure their tracker and export a new schema-2 package. The legacy schema-1 `ACTIVE.yaml` pointer remains readable only as a registry wrapper around a current schema-2 family.

When adopting or upgrading a vault, a pre-existing file that becomes distribution-owned must already match the distribution or be explicitly authorized with an overlay or `--force`. This prevents a newly managed runtime file from being overwritten silently.

A minimal sync starts at `00-Home.md` and `instance.yaml`. If bootstrap is incomplete, `map-ecosystem` takes its **orientation** branch instead of walking an empty graph.

Synchronization stores its resumable local state under `.agents/state/map-ecosystem/sync/` in the cell; that state is ignored by Git and is never installed from this distribution. A sealed gate produces separate acknowledgement and documentation-group units. If a projection or application is recoverable, resume the emitted run rather than repeating its source analysis or review; the installed `map-ecosystem` references define the public commands and failure routing.

Local implementation plans live under `.plan/`. Init, adopt, and update keep that
directory ignored by Git and merge it into Obsidian's excluded-file settings;
never create a visible `plan/` directory inside a cell vault.

## Layout of this repo

| Path | Role |
|---|---|
| `install.sh` | init / update / doctor |
| `kernel/` | Files copied into a cell vault |
| `adapters/` | Optional GCP, database, and report engines |
| `evals/` | Bootstrap and adversarial checks; never installed |
| `instance.schema.yaml` | Contract for `instance.yaml` |
| `MANAGED_PATHS` | Update allowlist |

## License

Apache License 2.0.

## Minimal workflow

Requires Python 3.9+ and Git. Obsidian is optional for filesystem queries. Each optional adapter declares its own tool dependencies.

1. Initialize identity, systems, trackers, evidence profile and note locale with `init`.
2. Use `configure-workspace` only when a task needs local source roots or an optional capability. Bind team procedures through `90-Meta/cell-config.py`; `instance.yaml` owns portable bindings, local config owns machine paths.
3. Ask a bounded question through `map-ecosystem`; it returns sources and limitations without creating a case.
4. Open an investigation when the work needs continuity. Close it with a reason and limitations even when no story is exported.
5. Export a self-sufficient development package only when implementation is requested. Its source anchor permits targeted read-only context lookup; implementation deltas remain in the worktree for later vault-owned reconciliation.

Existing note names, relationships, investigations and handoff families are preserved by update. Optional capabilities are unconfigured until onboarding binds real procedures; selecting an adapter alone does not prove live readiness.

Version 0.6 adds optional capability bindings and transactional investigation commands without changing existing note or handoff schemas. `doctor --strict` is the opt-in installation integrity gate.
