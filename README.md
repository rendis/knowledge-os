# documentation-vault

A portable **knowledge OS** for a team or cell: an evidence-first vault kernel, agent skills, deterministic gates and a native CLI, installed into a cell's own vault without copying another team's domain knowledge. It works with Claude Code, Codex and Cursor alike.

This repository is the **distribution**. A cell vault is a separate directory created by `install.sh`; do not treat this checkout as a knowledge graph. Agents working on this repository start at [`AGENTS.md`](AGENTS.md).

## What a cell gets

- **An evidence contract** in the vault's `AGENTS.md`, loaded in every session: every technical statement rests on an inspected source and carries its grade (demonstrated, observed within limits, inferred, unresolved). The vault is the map, not the boundary: when a note is missing or stale, the agent follows the trail to the repositories (at their reference branch), platform snapshots, databases and trackers the cell configured. Draft answers naming resources are checked with `discover claims`; new conclusions get independent review.
- **Discovery** (`discover`): connection facts extracted deterministically from repositories (imports, manifests, configuration and IaC), typed by stored judgments and completed by read-only platform snapshots; note gates (`discover check`) verify anchors, coverage and stale citations.
- **Publication through Git** (`sync`): knowledge changes on a `sync/<slug>` branch, with gates, a review bound to the exact content and a fast-forward finish.
- **Investigations** (`investigation`): one case per line of work, written only through the CLI, gated on every write, and published, absorbed or retired on a sync branch.
- **Development handoffs** (`handoff`): atomic task packages prepared in a repository worktree, with a managed `AGENTS.md` section that tells any harness how to work with them, and a deterministic reconciliation back into the case.

## Install

```bash
./install.sh init --dest /path/to/cell-vault \
  --cell-name "Payments" --purpose "Card-present checkout and settlement" \
  --system payments:Payments --yes
./install.sh update --dest /path/to/cell-vault
./install.sh doctor --dest /path/to/cell-vault [--strict]
./install.sh adopt  --dest /path/to/existing-vault
```

Without flags, `init` asks for systems, trackers, cell name and purpose, evidence profile, note locale, the **reference branch order** (the branches tried in order in every repository, default `main` then `master`; teams and older projects differ) and adapters. `--yes` tests unattended installation; the `configure-workspace` skill collects local source roots, source access and capabilities afterwards. A cancelled interactive input creates no vault. With no arguments, the script updates the current directory when it holds a lock and initializes it when empty; knowledge Markdown without a lock is refused. `adopt` installs the kernel into an existing vault without rewriting its notes.

The lock (`.knowledge-os.lock.yaml`) is portable and committed with the cell. `update` refuses kernel files changed locally until `--force`, removes the managed files the distribution no longer ships (as recorded in the lock), and never rewrites cell-owned files.

## Native CLI

`make release` builds `vaultctl` for macOS, Linux and Windows on ARM64 and AMD64, with checksums, a runtime manifest and third-party notices under `dist/`. `init` and `update` copy all six binaries into the cell's versioned `.agents/bin/`, so a clone works without a compiler, Python or a download. Select the binary for the host through [use-vault-cli](kernel/.agents/skills/use-vault-cli/SKILL.md#bind-the-executable-and-vault); `vaultctl --help` lists the commands:

```text
overview · search · index · links · inventory · audit
discover run|questions|answer|platform|report|check|claims|corrections
config status|resolve|workspace|locate|capability|bind|catalog|areas|operation|…
check links|bases|visual|visual-context|obsidian-binding|map-closure
investigation new|list|check|add|state|absorb|close|reopen
handoff start|status|refresh|reconcile
sync start|status|review|verify|acknowledge|finish|pull
```

Discovery judgments are answered by Jev when `TYPESAFE_API_KEY` is set, otherwise by the agent through `discover questions` and `discover answer`. Search keeps a private local SQLite index outside the vault; results are pointers to open and verify, not answers. No hooks or model services are installed.

A team whose developers all use `git-lfs` can keep history small with `git lfs track ".agents/bin/vaultctl-*"`; `update` does not manage `.gitattributes`.

## Ownership in a cell

The cell owns `instance.yaml`, `00-Home.md`, the root Bases, `90-Meta/Alcance.md`, notes under `10/`–`70/`, `investigations/`, the optional `90-Meta/vault-catalog.yaml` of related vaults, and a local, ignored `AGENTS.personal.md` for personal rules. The distribution owns the `AGENTS.md` router, the generic files under `90-Meta/`, the skills, the specialist definitions for Claude Code, Codex and Cursor, and `.agents/bin/`.

Local, ignored stores (unpublished cases, private case material, operational runs, plans, scratch, discovery state and local configuration) are listed in [local-stores](kernel/90-Meta/local-stores.md). Handoff tasks live in each repository worktree's `.handoff/`, excluded from Git.

## Develop this distribution

| Path | Role |
|---|---|
| `install.sh`, `scripts/` | Installer (`knowledge_os.py`, with `instance.py` and `vault_catalog.py`), native runtime packaging, specialist rendering |
| `cmd/`, `internal/`, `tools/` | The `vaultctl` CLI, its packages, release and platform-check tools |
| `kernel/` | Files copied into every cell |
| `adapters/` | Optional skills a cell selects (`reports`) |
| `evals/` | Checks and benchmarks; never installed ([evals](evals/README.md)) |
| `instance.schema.yaml`, `MANAGED_PATHS` | The `instance.yaml` contract and the update allowlist |
| `docs/adr/` | Decisions that are not obvious from the code |

```bash
make test        # go vet and go test
make release     # cross-platform binaries (required before the installer tests)
make test-installer
```

## License

Apache License 2.0.
