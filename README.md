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

Interactive init collects portable cell choices. Local discovery paths and read-only versus managed source access are collected by the `configure-workspace` skill, then executed by its non-interactive CLI. Supplying `--yes` tests unattended installation, not whether an agent asks for missing onboarding decisions. Full onboarding must distinguish configured choices, explicitly deferred capabilities and source access still pending. A cancelled interactive input creates no vault.

A future published tag can be installed with:

```bash
curl -fsSL <raw-tag>/install.sh | bash -s -- init --dest /path/to/cell-vault
```

The current distribution channel is local-only. The script is written so that one-liner can clone a tag into cache and re-enter the same verbs.

## Native CLI

Build release artifacts before native installation:

```bash
make release
```

The build produces macOS, Linux and Windows binaries for ARM64 and AMD64,
checksums, a portable runtime manifest and third-party notices under `dist/`.
Init/update copies **all six binaries** plus the manifest and notices into the
consumer's versioned `.agents/bin/`. Commit that directory with the kernel update;
a clone or pull then receives every supported platform without this template,
a compiler, Python, an extra download or a session hook. The generic executable
reads the selected vault's own identity/configuration; there is no central registry.

Select the filename for the execution host using
[use-vault-cli](kernel/.agents/skills/use-vault-cli/SKILL.md#bind-the-executable-and-vault).
For example, on macOS ARM64, from an installed vault:

```bash
.agents/bin/vaultctl-darwin-arm64 config resolve --vault .
.agents/bin/vaultctl-darwin-arm64 search --vault . --query "repository retry policy" --limit 5
.agents/bin/vaultctl-darwin-arm64 links --vault . --node "Canonical note basename"
.agents/bin/vaultctl-darwin-arm64 audit --vault .
.agents/bin/vaultctl-darwin-arm64 check links --vault .
```

Linux uses `vaultctl-linux-arm64` or `vaultctl-linux-amd64`; Windows uses
`vaultctl-windows-arm64.exe` or `vaultctl-windows-amd64.exe` under the same directory.
Prefix a quoted executable path with `&` in PowerShell. Preserve executable file
modes for macOS/Linux when committing the bundle. Skills specify lifecycle and
synchronization commands with explicit targets and existing review/authorization
requirements. Existing ignored `.bin/` files from older installations are obsolete;
new instructions use only `.agents/bin/`.

The release adds the size of six builds to each consumer repository. Git transfers
binary updates with the kernel; this deliberately trades repository size for
self-contained clones while no public artifact download channel exists.

Search hashes eligible Markdown on each explicit retrieval call and updates
only changed files in one SQLite transaction. The index is local, disposable,
and outside the vault; Markdown remains authoritative. `index --rebuild`
recreates it. Private investigation context is included locally and labeled;
`search --visibility public` omits it. Results contain bounded excerpts, source
paths, headings, origin and lexical rank. Rank is not a probability or proof:
open and verify the source before answering. No hooks, model services or
automatic per-message injection are installed.

Browser-specific skill resources remain with their skills. Legacy Python
implementations retained in this distribution serve compatibility evals; the
native retirement policy prevents installing them in consumer vaults.

## After init: where to start

The cell vault owns:

- `instance.yaml` — who the cell is, systems, trackers, source prefixes, evidence profile
- `00-Home.md` — orientation; a later `update` does not overwrite it
- `10-Sistemas/` — one stub note per declared system
- `10/`–`70/` — the cell's knowledge graph

The distribution owns the thin `AGENTS.md` router, every generic file it ships under `90-Meta/`, kernel skills, selected adapter skills, the six-platform `.agents/bin/` bundle, `VERSION`, and the `.claude/skills` symlink. Claude Code 2.1.277+ reads `AGENTS.md` when the project has no `CLAUDE.md` on supported backends; `update` removes only the former distribution link `CLAUDE.md -> AGENTS.md` and preserves any cell-owned `CLAUDE.md`. `update` refreshes matching distribution files without deleting cell-only Meta files, skills, recipes, or overlays. The cell owns `instance.yaml`, `00-Home.md`, the root Bases, `90-Meta/Alcance.md`, and notes under `10/`–`70/`; those files are seeded or authored locally and never rewritten by `update`.

Repository implementation can continue from a vault session through [work-in-repository](kernel/.agents/skills/work-in-repository/SKILL.md). It binds an authorized development checkout, loads its instructions and relevant skills, and respects existing handoffs. Source discovery alone remains read-only; the workflow does not configure the coding agent or require a new investigation.

Related domain vaults may be recorded in the consumer-owned, versioned `90-Meta/vault-catalog.yaml`. It is created on first authorized registration and preserved by `init`, `adopt`, and `update`; absence is valid. `map-ecosystem` manages entries: discovered candidates require user confirmation, while a direct registration request already authorizes the named entry. The catalog stores portable repository identities and domain descriptions, never local paths, credentials or user access state. `doctor` validates existing catalogs; consultation still checks current access and destination instructions. See the [catalog procedure](kernel/.agents/skills/map-ecosystem/references/vault-catalog.md).

Each checkout may also have a root `AGENTS.personal.md`. It is optional, local, and ignored by Git. The managed `AGENTS.md` has a dedicated personal-instructions section whose `@AGENTS.personal.md` reference imports it in compatible harnesses and whose explicit load instruction covers the others. Agents create or update it when the user establishes or changes a reusable rule for how future work should be done for them or in their local environment, even without explicitly asking to save it. One-time task instructions do not create persistent rules. Within kernel guardrails, current user direction has priority and the personal file specializes generic skills without changing their versioned content. Store credentials in a secret manager; the personal file records only how the local environment reaches them. `init`, `adopt`, and `update` preserve this file, while `doctor --strict` rejects a missing ignore rule or a tracked copy.

Personal customizations belong exclusively in `AGENTS.personal.md`; never edit the vault's root `AGENTS.md` or versioned skills to persist them. Updating a personal rule preserves unrelated preferences.

The managed router includes the evidence, review and initiative contract directly. Specialized investigation and publication procedures remain in skills. Project-local evidence investigator and reviewer definitions are installed for Codex, Claude Code and Cursor; their discovery and permission differences are documented in `90-Meta/specialists.md`. No hooks are installed.

`update` refreshes only the consumer vault; it never traverses development worktrees. An existing worktree receives the current managed instruction block during its next authorized handoff mutation.

Versions 0.5 through 0.7 use work-item bundle and handoff manifest schema 2. Jira-specific schema-1 packages or materialized families are not reinterpreted in place; configure their tracker and export a new schema-2 package. The legacy schema-1 `ACTIVE.yaml` pointer remains readable only as a registry wrapper around a current schema-2 family.

When adopting or upgrading a vault, a pre-existing file that becomes distribution-owned must already match the distribution or be explicitly authorized with an overlay or `--force`. This prevents a newly managed runtime file from being overwritten silently.

A minimal sync starts at `00-Home.md` and `instance.yaml`. If bootstrap is incomplete, `map-ecosystem` takes its **orientation** branch instead of walking an empty graph.

Synchronization stores its resumable local state under `.agents/state/map-ecosystem/sync/` in the cell; that state is ignored by Git and is never installed from this distribution. A sealed gate produces separate acknowledgement and documentation-group units. If a projection or application is recoverable, resume the emitted run rather than repeating its source analysis or review; the installed `map-ecosystem` references define the public commands and failure routing.

`scheduled-vault-refresh` is the inherited guide for a manual or scheduled mapping cycle. Each team supplies its own cadence, source scope, any applicable procedure, and authorized review or publication path in its consumer vault or scheduler; installation alone creates no task or remote Git authority. The skill reuses `synchronize-ecosystem`, checks associated investigation claims against verified destinations, and consults applicable repository instructions and specific notes under `60-Operacion/Git/` before Git publication. Concurrent contributions require both Git reconciliation and review of the combined documented meaning.

Local implementation plans live under `.plan/`. Init, adopt, and update keep that
directory ignored by Git and merge it into Obsidian's excluded-file settings;
never create a visible `plan/` directory inside a cell vault.

Vault-local helper scripts without a selected investigation or workflow-defined
working location go under a unique `.scratch/<task>/` directory in the cell
vault. Init, adopt, and update ignore `.scratch/` in Git and Obsidian. When a
task is later attached to an investigation, move the relevant local work or
reviewed methods into that case under its record contract.

## Layout of this repo

| Path | Role |
|---|---|
| `install.sh` | init / update / doctor |
| `kernel/` | Files copied into a cell vault |
| `adapters/` | Optional report workflow |
| `evals/` | Bootstrap and adversarial checks; never installed |
| `instance.schema.yaml` | Contract for `instance.yaml` |
| `MANAGED_PATHS` | Update allowlist |

## License

Apache License 2.0.

## Minimal workflow

Consumers obtain their vault with `git clone`; Git is a workspace prerequisite. The distribution installer and retained compatibility evals use Python 3.9+ and Git. Installed vault operations use their matching native executable under `.agents/bin/`, without Python or Go. Git, GitHub CLI and Obsidian remain external tools for the operations that need them. Each optional adapter declares its own tool dependencies.

For a provider-independent local Python gate, create `.venv`, install the
fully locked `kernel/90-Meta/requirements-ci.txt`, and run
`python -B kernel/90-Meta/check-code-quality.py --root .` with that interpreter.
Ruff and Bandit load their standard configuration files from `90-Meta`.
These are distribution-only developer checks. Installed cells use the native
checks described in `90-Meta/code-quality.md`; Python tooling is not shipped.

1. Initialize identity, systems, trackers, evidence profile and note locale with `init`.
2. Use `configure-workspace` only when a task needs local source roots or an optional capability. Bind team procedures through `<CLI> config bind`; `instance.yaml` owns portable bindings, local config owns machine paths.
3. Ask a bounded question through `map-ecosystem`; it returns sources and limitations without creating a case.
4. Open an investigation when the work needs continuity. Close it with a reason and limitations even when no story is exported.
5. Export a self-sufficient development package only when implementation is requested. Its source anchor permits targeted read-only context lookup; implementation deltas remain in the worktree for later vault-owned reconciliation.

Existing note names, relationships, investigations and handoff families are preserved by update. Optional capabilities are unconfigured until onboarding binds real procedures; selecting an adapter alone does not prove live readiness.

Cells may declare credential-free `database_targets` in `instance.yaml` when one database capability has several explicit destinations. Each target binds a system, environment, instance, database, schemas, optional evidence repositories, procedure basename and an optional local proxy-port key. The target identifies where evidence belongs; its procedure and adapter still establish read-only access.

Version 0.6 adds optional capability bindings and transactional investigation commands without changing existing note or handoff schemas. `doctor --strict` is the opt-in installation integrity gate and also detects configured adapters awaiting installation or removal. Version 0.6.1 preserves configuration during updates and aligns investigation promotion with the selected evidence profile.

For retired provider-pack migration and developer-owned integrations, see [adapters](adapters/README.md).

## Extraction quality

For the expected map contents, five acceptance questions and completion criteria, read [the repository mapping contract](kernel/.agents/skills/map-ecosystem/references/repository-map.md#what-a-useful-map-answers). This is the shared reference for initial mapping and incremental sync.

Map and sync share a bounded contract: repository purpose and stack, main entrypoints through meaningful logic to connectors/effects, exact source evidence and explicit limits. README/docs contribute when consistent with implementation. Existing valid knowledge is preserved. Each service-local map records externally significant connections as stable atomic claims. Accepted local maps may then drive selective cross-repository, infrastructure or read-only runtime reconciliation without reopening their extraction. Partial and unresolved connections remain as concrete verification items in the repository note until their stated evidence closes them. Independent review checks main-flow correctness, connector coverage, stale changed values and useful-knowledge preservation without repeating extraction. Pilot model/effort choices with execution limits before a batch; persist and publish small independent groups. New sync analyses use a provider-neutral schema-3 checklist; persisted schema-2 analyses remain readable without rewriting their artifacts. Cell procedures and observed technologies determine the concrete probes.

These rules ship through `init` and `update` as kernel behavior. They do not import, replace or rewrite another cell's `10/`–`70/` knowledge. A new cell starts with the mapping method and an empty domain graph; an existing cell keeps its own notes and uses them as the preservation baseline on later maps and syncs.

A valid initial `revise` review may receive one targeted correction through `<CLI> sync correction prepare` and `<CLI> sync correction check` before checkpointing. Original artifacts remain available; publication/recovery continues with one final package. Rejected or limited outcomes stay explicit. Repeated publication attempts never trigger fresh semantic work.

`evals/extraction-quality/` prepares frozen synthetic repositories and vault context for blind production-artifact extraction. The separate semantic rubric checks recovered rules, dependent impact, justified removal, preservation and evidence precision; it is never provided to the extractor. One scenario is a regression, not a general quality or efficiency guarantee.

Campaign closure also requires [bounded connection reconciliation and consistent visible coverage](kernel/.agents/skills/map-ecosystem/references/mapping-completion.md). The [closure behavioral fixture](evals/map-closure/README.md) exercises sibling-source answers, duplicate pending questions, a genuine live-evidence gap, and stale summaries; its fixture checks alone are not proof of agent behavior.
