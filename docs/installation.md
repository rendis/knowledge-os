# Installation and updates

## Install kos

`kos` is one executable per machine that serves every vault; vaults carry no binaries. The repository is private, so the release is read with the GitHub CLI and an account that can read it:

```bash
gh release download --repo rendis/knowledge-os --pattern install-kos.sh --output - | sh   # macOS, Linux, WSL
kos version          # this kos, the kernel it carries, the vault's kernel, a newer release if any
kos update           # replace this kos with the latest release (checksum verified)
```

Windows uses `install-kos.ps1` the same way; `sh scripts/install-kos.sh --local` builds from a checkout. When a newer kos exists, every command prints one `kos notice:` line on stderr, and the agent offers the update.

## Create a cell vault

```bash
./install.sh init --dest /path/to/cell-vault \
  --cell-name "Payments" --purpose "Card-present checkout and settlement" \
  --system payments:Payments --yes
./install.sh update --dest /path/to/cell-vault
./install.sh doctor --dest /path/to/cell-vault [--strict]
./install.sh adopt  --dest /path/to/existing-vault
```

`install.sh` runs from a checkout of this repository. With no arguments, it updates the current directory when it holds a lock and initializes it when empty; knowledge Markdown without a lock is refused. `adopt` installs the kernel into an existing vault without rewriting its notes. `--yes` tests unattended installation. A cancelled interactive input creates no vault.

## Onboarding

Onboarding has two levels.

- **The cell's**, once and shared. Without flags, `init` asks who the cell is (name, purpose, systems), where its code is (GitHub organization, repository prefixes, the **reference branch order**: the branches tried in each repository, default `main` then `master`), the **clouds** it runs on (`gcp`, `aws`, `azure`, one or several), its issue trackers and the notes' language. The evidence profile and adapters keep their defaults unless passed as flags. The `onboard-cell` skill runs the same conversation with an agent, proposing answers from the organization's repositories, and ends with the first inventory and discovery.
- **Each developer's**, on their machine. When someone opens the vault for the first time, the `onboard-developer` skill runs `kos config detect` (clones of the cell's repositories, a worktree root, cloud logins, database ports), asks for one confirmation, records it and gives an access card.

## Kernel updates

The installer writes the cell's identity and delegates the kernel itself to `kos kernel update`, the single implementation of installing and updating a kernel. The lock (`.knowledge-os.lock.yaml`) is portable and committed with the cell.

- `kos kernel status` and `kos kernel update --dry-run` show how a vault's kernel differs from the one kos carries; `kos kernel update` applies it.
- An update refuses kernel files changed locally until `--force`, removes the managed files the distribution no longer ships (as recorded in the lock), and never rewrites cell-owned files.
- A vault whose kernel is newer than kos refuses to publish until `kos update`.

## Vaults on a machine

Every vault a `kos` command works on is remembered on the machine (`kos vaults`; `scan DIR` finds the rest). The list is checked at each recorded path whenever it is shown: a vault that moved, disappeared or now holds another cell is reported with its fix, never updated.

After `kos update`, the list shows the vaults whose kernel is older than the new one; `kos kernel update --all --dry-run` previews them and `--commit` updates and commits each on its current branch, skipping the ones already current (a teammate's pushed update arrives by pull).

## Ownership in a cell

The cell owns `instance.yaml`, `00-Home.md`, the root Bases, `90-Meta/Alcance.md`, notes under `10/`–`70/`, `investigations/`, the optional `90-Meta/vault-catalog.yaml` of related vaults, and a local, ignored `AGENTS.personal.md` for personal rules. The distribution owns the `AGENTS.md` router, the generic files under `90-Meta/`, the skills, and the specialist definitions for Claude Code, Codex and Cursor.

Local, ignored stores (unpublished cases, private case material, operational runs, plans, scratch, discovery state and local configuration) are listed in [local-stores](../kernel/90-Meta/local-stores.md). Handoff tasks live in each repository worktree's `.handoff/`, excluded from Git.
