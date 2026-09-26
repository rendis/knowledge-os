<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/hero-dark.svg">
    <img src="docs/assets/hero-light.svg" width="100%" alt="knowledge-os: a question enters the vault, which follows the trail to repositories, cloud snapshots, databases and trackers; the answer comes back sourced and graded, and what was learned returns to the vault through a gated sync.">
  </picture>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/agents-Claude_Code_·_Codex_·_Cursor-534AB7?style=flat-square" alt="Agents: Claude Code, Codex, Cursor">
  <img src="https://img.shields.io/badge/kos-macOS_·_Linux_·_Windows-0F6E56?style=flat-square" alt="kos runs on macOS, Linux and Windows">
  <img src="https://img.shields.io/badge/clouds-GCP_·_AWS_·_Azure-0F6E56?style=flat-square" alt="Clouds: GCP, AWS, Azure">
  <img src="https://img.shields.io/badge/license-Apache_2.0-59636E?style=flat-square" alt="License: Apache 2.0">
</p>

**knowledge-os** gives a team's AI agents a shared, evidence-first map of its systems. It installs a vault kernel, agent skills and deterministic gates into the team's own vault — never another team's knowledge — and ships `kos`, one native CLI per machine. Agents answer from sources they inspected, and what they learn goes back to the vault through Git.

## Every claim carries its grade

<p>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/grades-dark.svg">
    <img src="docs/assets/grades-light.svg" width="100%" alt="Evidence grades: demonstrated, observed within limits, inferred, unresolved.">
  </picture>
</p>

The vault is the map, not the boundary. When a note is missing or stale, the agent follows the trail to the repositories at their reference branch, the cloud snapshots, databases and trackers the cell configured. Answers that name resources are checked with `kos discover claims`; new conclusions get independent review.

## How it works

| | Step | What happens | With |
|:-:|---|---|---|
| 1 | **Onboard** | The cell says who it is, where its code lives and which clouds it runs on; `onboard-developer` sets up each machine once. | `kos init` |
| 2 | **Discover** | Connections extracted from repositories and read-only cloud snapshots; note gates catch stale citations. | `kos discover` |
| 3 | **Ask** | The agent answers from inspected sources, grades each claim and checks the names it cites. | `kos discover claims` |
| 4 | **Investigate** | One case per line of work, written only through the CLI and gated on every write. | `kos investigation` |
| 5 | **Hand off** | Atomic tasks in a repository worktree for any harness, reconciled back into the case. | `kos handoff` |
| 6 | **Publish** | A `sync/<slug>` branch with gates, a review bound to the exact content and a fast-forward finish. | `kos sync` |

Open [`docs/flows/index.html`](docs/flows/index.html) in a browser to watch each flow animated step by step.

## See it run

**Discover** — three repositories and a cloud project become connections, each with its evidence.

<img src="docs/assets/demo-discover.gif" width="100%" alt="kos discover scans three repositories, records the agent's judgments, captures the gcp project read-only and reports a subscription backed by configuration and platform evidence.">

**Ask** — before an answer is delivered, every resource it names is checked; an invented topic is caught.

<img src="docs/assets/demo-claims.gif" width="100%" alt="kos discover claims flags refund-requested as a name no repository, snapshot or note knows.">

<details>
<summary><b>Investigate</b> — a claim without a source is refused</summary>
<br>
<img src="docs/assets/demo-investigation.gif" width="100%" alt="kos investigation refuses evidence without a source, accepts it with file@commit, and records a finding inferred from it.">
</details>

<details>
<summary><b>Hand off</b> — a task goes to a worktree and comes back as graded evidence</summary>
<br>
<img src="docs/assets/demo-handoff.gif" width="100%" alt="kos handoff starts a worktree for a task package, reads its commits and deltas, and reconciles them into the case as demonstrated evidence.">
</details>

<details>
<summary><b>Publish</b> — the gate stops an orphan note until it is linked</summary>
<br>
<img src="docs/assets/demo-sync.gif" width="100%" alt="kos sync refuses to finish while a new topic note is orphaned, then fast-forwards main once the note is linked and reviewed again.">
</details>

<sub>Recorded with the real <code>kos</code> against a synthetic cell (<a href="docs/demo/">docs/demo</a>); the cloud listing comes from a stubbed <code>gcloud</code>. Regenerate with <code>make demo</code>.</sub>

## Quickstart

```bash
# 1. Install kos once per machine (the repository is private: use an account that can read it)
gh release download --repo rendis/knowledge-os --pattern install-kos.sh --output - | sh

# 2. Create the cell's vault: kos asks who the cell is, where its code lives and where it runs
kos init --vault ~/vaults/payments

# 3. Open the vault with Claude Code, Codex or Cursor: onboard-developer sets up your machine
```

<img src="docs/assets/demo-onboard.gif" width="100%" alt="kos init asks for the cell's name, purpose, systems, GitHub organization, repository prefixes, reference branches, clouds, trackers and language, creates the vault, and kos doctor reports it installed and current.">

More: [installation and updates](docs/installation.md) · [the kos CLI](docs/cli.md) · [decisions](docs/adr/)

<details>
<summary><b>What the cell owns</b></summary>
<br>

The cell owns `instance.yaml`, `00-Home.md`, the root Bases, notes under `10/`–`70/` and `investigations/`. The distribution owns the `AGENTS.md` router, the generic files under `90-Meta/`, the skills and the specialist definitions. `kos kernel update` refuses kernel files edited in the vault until `--force` and never rewrites cell-owned files. Details in [installation](docs/installation.md#ownership-in-a-cell).

</details>

<details>
<summary><b>Develop this distribution</b></summary>
<br>

This repository is the **distribution**, not a cell vault; agents working on it start at [`AGENTS.md`](AGENTS.md).

| Path | Role |
|---|---|
| `cmd/`, `internal/`, `tools/` | The `kos` CLI (cells are created in `internal/cell`), its packages, release and platform-check tools |
| `scripts/` | The machine installers (`install-kos.sh`, `install-kos.ps1`), specialist and README asset rendering |
| `kernel/` | Files copied into every cell |
| `adapters/` | Optional skills a cell selects (`reports`) |
| `evals/` | Checks and benchmarks; never installed ([evals](evals/README.md)) |
| `instance.schema.yaml`, `MANAGED_PATHS` | The `instance.yaml` contract and the update allowlist |
| `docs/` | Guides, [decisions](docs/adr/) and the README images (`python3 -B scripts/render_readme_assets.py`); `docs/demo/` holds the synthetic cell and VHS tapes behind the recordings; `docs/flows/` the animated flow guide (one script per flow in `docs/flows/flows/`) |

```bash
make test            # go vet and go test
make release         # cross-platform binaries (required before the installation tests)
make test-installer
make demo            # re-record the README GIFs (vhs and jq required)
```

</details>

## License

[Apache License 2.0](LICENSE)
