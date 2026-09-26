# kos creates and adopts cells; the checkout installer is retired

Creating a cell needed a clone of this repository and Python: `./install.sh init` wrote the cell's identity
and then called `kos kernel update` for the kernel. A developer already has `kos` from `install-kos.sh`, and
the binary already embeds the kernel, the skeleton and the templates. Two validators of `instance.yaml` (Python
`scripts/instance.py` and Go `config.ValidateInstance`) could drift, and `onboard-cell` could only create a
vault from a distribution checkout.

`install-kos.sh` (or `install-kos.ps1`) runs once per machine. Every later step is a `kos` command:

| Before | After |
|---|---|
| `./install.sh init --dest V …` | `kos init --vault V …`: the same questions and flags, asked on stderr when stdin is a terminal, skipped by flags or `--yes`; stdout stays JSON |
| `./install.sh adopt --dest V` | `kos adopt --vault V`: the same preconditions (`00-Home.md`, `10-Sistemas/*.md`, a valid `instance.yaml`) |
| `./install.sh update --dest V` | `kos kernel update --vault V` (unchanged) |
| `./install.sh doctor --dest V [--strict]` | `kos doctor --vault V [--strict]`: read-only; instance, catalog, kernel drift, adapters, `.claude/skills` topology, `AGENTS.personal.md` ignored and untracked, orientation |

`--vault` defaults to the current directory, as in every other command. `init` and `adopt` remember the vault
(`kos vaults`) and write the kernel through the same code as `kos kernel update`. The Go validator is the only
one. `install.sh`, `scripts/knowledge_os.py`, `scripts/instance.py`, `scripts/vault_catalog.py`,
`scripts/native_runtime.py` and `scripts/test_instance.py` are deleted; the bootstrap evals drive the released
`kos` instead.

**Considered options**: keep `install.sh` as a thin wrapper over `kos` (a compatibility layer with no
external contract: no cell and no lock refers to it); make `kos init` call the Python installer (machines
and cells receive no Python); fold doctor into `kos kernel status` and `kos config status` (a person checking a
new vault wants one read-only answer).

**Consequences**: a cell is created, adopted, updated and checked with the binary alone, and `onboard-cell`
runs `<CLI> init` from anywhere. `--dest` becomes `--vault`. `--discovery-root` is dropped: machine paths
belong to the developer's configuration (`kos config detect`), and Home no longer lists remotes found at init
(`kos inventory` does). `doctor` no longer checks the distribution checkout's Git state; the lock keeps the
revision of the release that installed the kernel. Python remains only for evals and development scripts.
