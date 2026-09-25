# documentation-vault — distribution

This checkout is the **installable template**, not a cell knowledge vault. Do not map systems, write `10/`–`70/` notes, or treat `kernel/AGENTS.md` as this repo's router.

Consumer vaults are created with `./install.sh init --dest <vault>`. Work here is packaging, kernel payload, adapters, and evals.

## Before changing anything

1. Read `README.md` (verbs and layout) and `MANAGED_PATHS` (what `update` overwrites in a cell).
2. Decide the layer: installer, kernel payload, adapter, or eval. Edit only that layer.
3. Run `python3 -B evals/bootstrap/test_bootstrap.py` and `python3 -B kernel/90-Meta/test_instance.py` after installer, kernel, or eval changes.

## Layers

- **Installer** — `install.sh`, `scripts/knowledge_os.py`. Init / update / doctor. Update copies managed kernel paths and selected adapters; it never overwrites a cell's `instance.yaml`, `00-Home.md`, or notes under `10/`–`70/`.
- **Kernel payload** — `kernel/`. Copied into every cell. `kernel/AGENTS.md` is the **cell** router (skills, orientation, evidence). Keep it thin; keep product names out.
- **Adapters** — `adapters/`. Opt-in at init, or later by listing them in the cell's `instance.yaml` and running `update`. Ship engines and generic samples, not another team's recipes.
- **Evals** — `evals/`. Never copied into a cell. Adversarial ledgers live under `evals/bootstrap/ledgers/`.

`instance.schema.yaml` is the contract the installer writes into a cell as `instance.yaml`.

## Guardrails

- Cell identity and knowledge live only in `--dest`, never in this tree.
- English for agent docs and skills in this repo. Note locale of a cell is chosen at init.
- `update` refuses kernel files that changed locally (the lock records installed hashes; `git diff` shows the change) until `--force`; cell-specific logic lives in cell-owned files, never in managed ones.
- `AGENTS.md` is the instruction file for this repo and installed cells. The installer removes its former `CLAUDE.md -> AGENTS.md` link during update while preserving a cell-owned `CLAUDE.md`.
