# knowledge-os

A portable **knowledge OS** for a team or cell: an evidence-first vault kernel, agent skills, deterministic gates and a native CLI, installed into a cell's own vault without copying another team's domain knowledge. It works with Claude Code, Codex and Cursor alike.

This repository is the **distribution**. A cell vault is a separate directory created by `install.sh`; do not treat this checkout as a knowledge graph. Agents working on this repository start at [`AGENTS.md`](AGENTS.md).

## What a cell gets

- **An evidence contract** in the vault's `AGENTS.md`, loaded in every session: every technical statement rests on an inspected source and carries its grade (demonstrated, observed within limits, inferred, unresolved). The vault is the map, not the boundary: when a note is missing or stale, the agent follows the trail to the repositories (at their reference branch), platform snapshots, databases and trackers the cell configured. Draft answers naming resources are checked with `discover claims`; new conclusions get independent review.
- **Discovery** (`discover`): connection facts extracted deterministically from repositories (imports, manifests, configuration and IaC), typed by stored judgments and completed by read-only platform snapshots of the cell's clouds (Google Cloud, AWS and Azure: messaging, databases, storage and warehouse, each read with its own CLI and the developer's login); what the providers do not read, the agent inspects with the tools in reach and records (`discover platform --record`); note gates (`discover check`) verify anchors, coverage and stale citations.
- **Publication through Git** (`sync`): knowledge changes on a `sync/<slug>` branch, with gates, a review bound to the exact content and a fast-forward finish.
- **Investigations** (`investigation`): one case per line of work, written only through the CLI, gated on every write, and published, absorbed or retired on a sync branch.
- **Development handoffs** (`handoff`): atomic task packages prepared in a repository worktree, with a managed `AGENTS.md` section that tells any harness how to work with them, and a deterministic reconciliation back into the case.

## Install kos

`kos` is one executable per machine that serves every vault; vaults carry no binaries. The repository is private, so the release is read with the GitHub CLI and an account that can read it:

```bash
gh release download --repo rendis/knowledge-os --pattern install-kos.sh --output - | sh   # macOS, Linux, WSL
kos version          # this kos, the kernel it carries, the vault's kernel, a newer release if any
kos update           # replace this kos with the latest release (checksum verified)
```

Windows uses `install-kos.ps1` the same way; `sh scripts/install-kos.sh --local` builds from a checkout. When a newer kos exists, every command prints one `kos notice:` line on stderr, and the agent offers the update. `kos kernel status` and `kos kernel update --dry-run` show how a vault's kernel differs from the one kos carries; `kos kernel update` applies it (refusing kernel files edited in the vault until `--force`). A vault whose kernel is newer than kos refuses to publish until `kos update`.

## Create a cell vault

```bash
./install.sh init --dest /path/to/cell-vault \
  --cell-name "Payments" --purpose "Card-present checkout and settlement" \
  --system payments:Payments --yes
./install.sh update --dest /path/to/cell-vault
./install.sh doctor --dest /path/to/cell-vault [--strict]
./install.sh adopt  --dest /path/to/existing-vault
```

Onboarding has two levels. **The cell's**, once and shared: without flags, `init` asks who the cell is (name, purpose, systems), where its code is (GitHub organization, repository prefixes, the **reference branch order**: the branches tried in each repository, default `main` then `master`), the **clouds** it runs on (`gcp`, `aws`, `azure`, one or several), its issue trackers and the notes' language; the evidence profile and adapters keep their defaults unless passed as flags. The `onboard-cell` skill runs the same conversation with an agent, proposing answers from the organization's repositories, and ends with the first inventory and discovery. **Each developer's**, on their machine: when someone opens the vault for the first time, the `onboard-developer` skill runs `kos config detect` (clones of the cell's repositories, a worktree root, cloud logins, database ports), asks for one confirmation, records it and gives an access card. `--yes` tests unattended installation. A cancelled interactive input creates no vault. With no arguments, the script updates the current directory when it holds a lock and initializes it when empty; knowledge Markdown without a lock is refused. `adopt` installs the kernel into an existing vault without rewriting its notes.

The installer writes the cell's identity and delegates the kernel itself to `kos kernel update`, the single implementation of installing and updating a kernel. The lock (`.knowledge-os.lock.yaml`) is portable and committed with the cell. An update refuses kernel files changed locally until `--force`, removes the managed files the distribution no longer ships (as recorded in the lock), and never rewrites cell-owned files.

## Native CLI

`make release` builds `kos` for macOS, Linux and Windows on ARM64 and AMD64 under `dist/`, with `SHA256SUMS`, the installer scripts, third-party notices and `release.json` (the source fingerprint the installer checks). Each binary embeds the kernel payload. `make publish` tags `v<version>` and publishes `dist/` as the GitHub release of `rendis/knowledge-os` with the `rendis` account. `kos --help` lists the commands:

```text
overview · search · index · links · inventory · audit
discover run|questions|answer|platform|report|check|claims|corrections
config status|resolve|workspace|locate|capability|bind|catalog|areas|operation|…
check links|bases|obsidian-binding|map-closure
investigation new|list|check|add|state|absorb|close|reopen
handoff start|status|refresh|reconcile
sync start|status|review|verify|acknowledge|finish|pull
kernel status|update · version · update
```

Discovery judgments are answered by Jev when `TYPESAFE_API_KEY` is set, otherwise by the agent through `discover questions` and `discover answer`. Search keeps a private local SQLite index outside the vault; results are pointers to open and verify, not answers. No hooks or model services are installed.

Failures, unwanted behaviors and proposals come back through the `report-to-distribution` skill as sanitized issues (templates in `.github/ISSUE_TEMPLATE/`).

## Ownership in a cell

The cell owns `instance.yaml`, `00-Home.md`, the root Bases, `90-Meta/Alcance.md`, notes under `10/`–`70/`, `investigations/`, the optional `90-Meta/vault-catalog.yaml` of related vaults, and a local, ignored `AGENTS.personal.md` for personal rules. The distribution owns the `AGENTS.md` router, the generic files under `90-Meta/`, the skills, and the specialist definitions for Claude Code, Codex and Cursor.

Local, ignored stores (unpublished cases, private case material, operational runs, plans, scratch, discovery state and local configuration) are listed in [local-stores](kernel/90-Meta/local-stores.md). Handoff tasks live in each repository worktree's `.handoff/`, excluded from Git.

## Develop this distribution

| Path | Role |
|---|---|
| `install.sh`, `scripts/` | Installer (`knowledge_os.py`, with `instance.py` and `vault_catalog.py`), native runtime packaging, specialist rendering |
| `cmd/`, `internal/`, `tools/` | The `kos` CLI, its packages, release and platform-check tools |
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
