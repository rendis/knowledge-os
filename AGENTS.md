# knowledge-os — distribution

This checkout is the **installable template**, not a cell knowledge vault. Do not map systems, write `10/`–`70/` notes, or treat `kernel/AGENTS.md` as this repo's router.

Consumer vaults are created with `kos init --vault <vault>`. Work here is the CLI, packaging, kernel payload, adapters, and evals.

## Before changing anything

1. Read `README.md` (commands and layout) and `MANAGED_PATHS` (what a kernel update overwrites in a cell).
2. Decide the layer: CLI, kernel payload, adapter, or eval. Edit only that layer.
3. Verify: `make test` after CLI changes; `make release` then `make test-installer` after kernel or CLI changes (the installation tests drive the fresh release). After editing the evidence contract in `kernel/AGENTS.md` or `kernel/90-Meta/specialists/`, run `python3 -B scripts/render_specialists.py`. `make test-platform` (Docker) after touching the platform providers or `tools/platformcheck`. A change to skill behavior is measured with real agents through `evals/benchmark`.
4. A versioned fix bumps `VERSION` and `kernel/VERSION` together; `make release` reads `kernel/VERSION`.

## Layers

- **CLI** — `cmd/kos`, `internal/`, `tools/`, `payload.go`. The `kos` every developer installs once per machine with `scripts/install-kos.sh`; it embeds the kernel. `kos init` / `adopt` / `doctor` (`internal/cell`) write and check a cell's identity; `kos kernel update` installs and updates the kernel, removes the files the lock recorded that the distribution no longer ships and never overwrites a cell's `instance.yaml`, `00-Home.md`, or notes under `10/`–`70/`. Cells receive no Python and no binaries.
- **Kernel payload** — `kernel/`. Copied into every cell. `kernel/AGENTS.md` is the **cell** router (evidence contract, navigation, skills). Keep it thin; keep product names out.
- **Adapters** — `adapters/`. Opt-in at init, or later by listing them in the cell's `instance.yaml` and running `kos kernel update --vault <vault>`. Ship engines and generic samples, not another team's recipes.
- **Evals** — `evals/`. Never copied into a cell; see `evals/README.md`.

Design decisions are recorded in `docs/adr/`.

`instance.schema.yaml` is the contract `kos init` writes into a cell as `instance.yaml`.

## Guardrails

- Cell identity and knowledge live only in the cell's vault, never in this tree; cell-specific benchmark questions and fixtures stay outside the repository. The product-leak test in `evals/bootstrap` blocks the development cells' names by hash.
- English for agent docs and skills in this repo. The notes' locale is chosen per cell at init.
- `kos kernel update` refuses kernel files that changed locally (the lock records installed hashes; `git diff` shows the change) until `--force`; cell-specific logic lives in cell-owned files, never in managed ones.
- `make publish` pushes `main`, moves the version tag and creates the GitHub release: run it only when asked. `kos update` replaces the machine's kos binary; it never touches a cell.
- `AGENTS.md` is the instruction file for this repo and installed cells; a cell-owned `CLAUDE.md` is preserved.
