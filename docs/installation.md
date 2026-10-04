# Installation and updates

## Install kos

`kos` is one executable per machine that serves every vault; vaults carry no binaries. The installer downloads the latest release and checks its checksum:

```bash
curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh   # macOS, Linux, WSL
kos version          # this kos, the kernel it carries, the vault's kernel, a newer release if any
kos update           # replace this kos with the latest release (checksum verified)
```

Windows: `irm https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.ps1 | iex` in PowerShell. `sh scripts/install-kos.sh --local` builds from a checkout, and a private fork set in `KOS_REPO` is read with the GitHub CLI. When a newer kos exists, every command prints one `kos notice:` line on stderr, and the agent offers the update.

## Create a cell vault

```bash
kos init --vault ~/vaults/payments                  # asks what the flags leave out
kos init --vault ~/vaults/payments --yes \
  --cell-name "Payments" --purpose "Card-present checkout and settlement" \
  --system payments:Payments --platform gcp --locale en
kos adopt  --vault /path/to/existing-vault          # existing notes, no kernel yet
kos doctor --vault ~/vaults/payments [--strict]     # read-only health
```

`--vault` defaults to the current directory. Questions go to stderr and stdout stays JSON, as in every `kos` command; `kos init --help` lists the flags. Nothing is written until every answer is in and valid, and a cancelled question creates no vault. `init` refuses a directory that already holds a vault (`--force` re-initializes) or knowledge without a kernel lock (use `adopt`, which installs the kernel without rewriting Home or the notes). `--yes` asks nothing: it needs `--system` and takes the defaults for the rest.

## Onboarding

Onboarding has two levels.

- **The cell's**, once and shared. Without flags, `init` asks who the cell is (name, purpose, systems), where its code is (GitHub organization, repository prefixes, the **reference branch order**: the branches tried in each repository, default `main` then `master`), the **clouds** it runs on (`gcp`, `aws`, `azure`, one or several), its issue trackers and the notes' language. The evidence profile and adapters keep their defaults unless passed as flags. The `onboard-cell` skill runs the same conversation with an agent, proposing answers from the organization's repositories, and ends with the first inventory and discovery.
- **Each developer's**, on their machine. When someone opens the vault for the first time, the `onboard-developer` skill runs `kos config detect` (clones of the cell's repositories, a worktree root, cloud logins, database ports), asks for one confirmation, records it and gives an access card.

## Kernel updates

`kos init` and `kos adopt` write the kernel through `kos kernel update`, the single implementation of installing and updating a kernel. The lock (`.knowledge-os.lock.yaml`) is portable and committed with the cell.

- `kos kernel status` and `kos kernel update --dry-run` show how a vault's kernel differs from the one kos carries; `kos kernel update` applies it.
- An update refuses kernel files changed locally until `--force`, removes the managed files the distribution no longer ships (as recorded in the lock), and never rewrites cell-owned files.
- Before writing, the update saves the previous files in a private directory inside the vault. Recovery files have their own ignore rule and stay out of Git even if the cell's ignore rules are restored. If a write fails, it restores modified and retired files, permissions, links and the lock, and removes files and empty directories created by the failed update. The lock is written last. If restoration itself fails, the error identifies the recovery directory; its backups are retained and another update refuses to overwrite them.
- Atomic replacements keep an existing file's permissions. If cleanup fails after an update or restoration, the error reports which state is installed and names the finished backup directory to remove before retrying. Remove only the directory named by a cleanup error; backups from an interrupted update or failed restoration are still needed for recovery.
- A vault whose kernel is newer than kos refuses to publish until `kos update`.

## Vaults on a machine

Every vault a `kos` command works on is remembered on the machine (`kos vaults`; `scan DIR` finds the rest). The list is checked at each recorded path whenever it is shown: a vault that moved, disappeared or now holds another cell is reported with its fix, never updated.

After `kos update`, the list shows the vaults whose kernel is older than the new one; `kos kernel update --all --dry-run` previews them and `--commit` updates and commits each on its current branch, skipping the ones already current (a teammate's pushed update arrives by pull).

## Ownership in a cell

The cell owns `instance.yaml`, `00-Home.md`, the root Bases, `90-Meta/Alcance.md`, notes under `10/`–`70/`, `investigations/`, the optional `90-Meta/vault-catalog.yaml` of related vaults, and a local, ignored `AGENTS.personal.md` for personal rules. The distribution owns the `AGENTS.md` router, the generic files under `90-Meta/`, the skills, the specialist definitions for Claude Code, Codex and Cursor, and `.github/workflows/knowledge-gates.yml`, which runs `kos sync verify --allow-no-change` on every push and pull request so knowledge committed outside a reviewed sync branch fails on the remote. It runs on `instance.yaml` `ci.runner` (default `ubuntu-latest`; set the organization's self-hosted runner label when hosted runners are not available, then `kos kernel update --vault <vault>`).

Local, ignored stores (unpublished cases, private case material, operational runs, plans, scratch, discovery state and local configuration) are listed in [local-stores](../kernel/90-Meta/local-stores.md). Handoff tasks live in each repository worktree's `.handoff/`, excluded from Git.
