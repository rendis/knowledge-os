# knowledge-os — distribution

This checkout is the **installable template**, not a cell knowledge vault. Do not map systems, write `10/`–`70/` notes, or treat `kernel/AGENTS.md` as this repo's router.

Consumer vaults are created with `./install.sh init --dest <vault>`. Work here is the CLI, packaging, kernel payload, adapters, and evals.

## Before changing anything

1. Read `README.md` (commands and layout) and `MANAGED_PATHS` (what `update` overwrites in a cell).
2. Decide the layer: CLI, installer, kernel payload, adapter, or eval. Edit only that layer.
3. Verify: `make test` after CLI changes; `make release` then `make test-installer` after installer, kernel or CLI changes (the installer tests need a fresh release). After editing the evidence contract in `kernel/AGENTS.md` or `kernel/90-Meta/specialists/`, run `python3 -B scripts/render_specialists.py`. A change to skill behavior is measured with real agents through `evals/benchmark`.

## Layers

- **CLI** — `cmd/kos`, `internal/`, `tools/`, `payload.go`. The `kos` every developer installs once per machine; it embeds the kernel and installs or updates it in a cell (`kos kernel update`). Cells receive no Python and no binaries.
- **Installer** — `install.sh`, `scripts/`. Init / adopt / doctor write and check the cell's identity; the kernel itself is installed and updated by `kos kernel update` (from `make release`), which removes the files the lock recorded that the distribution no longer ships and never overwrites a cell's `instance.yaml`, `00-Home.md`, or notes under `10/`–`70/`. `scripts/install-kos.sh` installs kos on a machine.
- **Kernel payload** — `kernel/`. Copied into every cell. `kernel/AGENTS.md` is the **cell** router (evidence contract, navigation, skills). Keep it thin; keep product names out.
- **Adapters** — `adapters/`. Opt-in at init, or later by listing them in the cell's `instance.yaml` and running `update`. Ship engines and generic samples, not another team's recipes.
- **Evals** — `evals/`. Never copied into a cell; see `evals/README.md`.

`instance.schema.yaml` is the contract the installer writes into a cell as `instance.yaml`.

## Guardrails

- Cell identity and knowledge live only in `--dest`, never in this tree; cell-specific benchmark questions and fixtures stay outside the repository. The product-leak test in `evals/bootstrap` blocks the development cells' names by hash.
- English for agent docs and skills in this repo. Note locale of a cell is chosen at init.
- `update` refuses kernel files that changed locally (the lock records installed hashes; `git diff` shows the change) until `--force`; cell-specific logic lives in cell-owned files, never in managed ones.
- `AGENTS.md` is the instruction file for this repo and installed cells; a cell-owned `CLAUDE.md` is preserved.
