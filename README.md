<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/hero-dark.svg">
    <img src="docs/assets/hero-light.svg" width="100%" alt="knowledge-os: the agent follows the trail from the vault to the code, a cloud snapshot, a database and a tracker, and each claim of its answer comes back with its source and grade.">
  </picture>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/agents-Claude_Code_·_Codex_·_Cursor-3452F5?style=flat-square" alt="Agents: Claude Code, Codex, Cursor">
  <img src="https://img.shields.io/badge/kos-macOS_·_Linux_·_Windows-1C2130?style=flat-square" alt="kos runs on macOS, Linux and Windows">
  <img src="https://img.shields.io/badge/clouds-GCP_·_AWS_·_Azure-1C2130?style=flat-square" alt="Clouds: GCP, AWS, Azure">
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

The vault is the map, not the boundary. When a note is missing or stale, the agent follows the trail to the repositories at their reference branch, the cloud snapshots, databases and trackers the cell configured. Every resource an answer names is confirmed in the notes, discovery facts or cloud snapshots; new conclusions get independent review.

For an ordinary read-only question, fresh evidence can be returned without storing it:
`kos discover run --vault <vault> --read-only` and
`kos discover platform --vault <vault> --provider <provider> --scope <scope> --read-only`.
Reuse the user's applicable read permission; storing snapshots, discovery state or notes requires write authority.
Batch note checks with repeated `--note` and add `--read-only` to prevent cache writes.


## How it works

<p align="center">
  <a href="https://rendis.github.io/knowledge-os/#onboard"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/flow-onboard-dark.svg"><img src="docs/assets/flow-onboard-light.svg" width="32%" alt="Onboard: kos is installed, the vault is created with its identity and kernel, and a first discovery starts the map."></picture></a>
  <a href="https://rendis.github.io/knowledge-os/#discover"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/flow-discover-dark.svg"><img src="docs/assets/flow-discover-light.svg" width="32%" alt="Discover: facts from the code and the cloud are compared with the vault; one is missing from it."></picture></a>
  <a href="https://rendis.github.io/knowledge-os/#ask"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/flow-ask-dark.svg"><img src="docs/assets/flow-ask-light.svg" width="32%" alt="Ask: the answer comes back with a source and a grade for each claim, approved by an independent review."></picture></a>
  <a href="https://rendis.github.io/knowledge-os/#investigate"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/flow-investigate-dark.svg"><img src="docs/assets/flow-investigate-light.svg" width="32%" alt="Investigate: a finding backed by evidence is absorbed into the vault from a shared case."></picture></a>
  <a href="https://rendis.github.io/knowledge-os/#handoff"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/flow-handoff-dark.svg"><img src="docs/assets/flow-handoff-light.svg" width="32%" alt="Hand off: commits and deltas from a worktree come back into the case as evidence."></picture></a>
  <a href="https://rendis.github.io/knowledge-os/#sync"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/flow-sync-dark.svg"><img src="docs/assets/flow-sync-light.svg" width="32%" alt="Sync: reviewed notes join the vault and reach the team."></picture></a>
</p>

| | Step | What happens | With |
|:-:|---|---|---|
| 1 | **Onboard** | The cell says who it is, where its code lives and which clouds it runs on; `onboard-developer` sets up each machine once. | `kos init` |
| 2 | **Discover** | Connections extracted from repositories and read-only cloud snapshots; note gates catch stale citations. | `kos discover` |
| 3 | **Ask** | The agent answers from inspected sources with its own tools, grades each claim and checks the names it cites. | `kos discover check` |
| 4 | **Investigate** | One case per line of work, written only through the CLI and gated on every write. | `kos investigation` |
| 5 | **Hand off** | Atomic tasks in a repository worktree for any harness, reconciled back into the case. | `kos handoff` |
| 6 | **Sync** | A `sync/<slug>` branch with gates, a review bound to the exact content and a fast-forward finish. | `kos sync` |

Click a flow to watch it step by step in [the flow guide](https://rendis.github.io/knowledge-os/) (source in [`docs/flows/`](docs/flows/), published on every push to `main`). In a vault, the agent opens the matching flow when you ask how one works.

## See it run

**Discover** — three repositories and a cloud project become connections, each with its evidence.

<img src="docs/assets/demo-discover.gif" width="100%" alt="kos discover scans three repositories, records the agent's judgments, captures the gcp project read-only and reports a subscription backed by configuration and platform evidence.">

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
<summary><b>Sync</b> — the gate stops an orphan note until it is linked</summary>
<br>
<img src="docs/assets/demo-sync.gif" width="100%" alt="kos sync refuses to finish while a new topic note is orphaned, then fast-forwards main once the note is linked and reviewed again.">
</details>

<sub>Recorded with the real <code>kos</code> against a synthetic cell (<a href="docs/demo/">docs/demo</a>); the cloud listing comes from a stubbed <code>gcloud</code>. Regenerate with <code>make demo</code>.</sub>

## Quickstart

```bash
# 1. Install kos once per machine (macOS, Linux, WSL; Windows: see the installation guide)
curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh

# 2. Create the cell's vault: kos asks who the cell is, where its code lives and where it runs
kos init --vault ~/vaults/payments

# 3. Open the vault with Claude Code, Codex or Cursor: onboard-developer sets up your machine
```

Or let an agent do steps 1 and 2: paste this into Claude Code, Codex or Cursor.

```text
Install knowledge-os on this machine and create my team's cell vault.

1. If `kos version` works, run `kos update`. Otherwise install kos: on macOS, Linux or WSL
   `curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh`;
   on Windows, in PowerShell, `irm https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.ps1 | iex`.
   Confirm it with `kos version`.
2. Read https://raw.githubusercontent.com/rendis/knowledge-os/main/kernel/.agents/skills/onboard-cell/SKILL.md
   and follow its steps 1 to 4 with `kos` as the CLI: propose the cell's identity, repositories,
   reference branches and clouds from evidence, and confirm them with me before writing anything.
   Ask me where the vault goes.
3. Create it without prompts: `kos init --vault <path> --yes` with the confirmed answers as flags
   (`kos init --help` lists them), or `kos adopt --vault <path>` if the folder already holds notes.
4. Run `kos doctor --vault <path>` and report the result. Then tell me to open the vault in a new
   session, where onboard-developer sets up my machine and the first discovery starts.
```

<img src="docs/assets/demo-onboard.gif" width="100%" alt="kos init asks for the cell's name, purpose, systems, GitHub organization, repository prefixes, reference branches, clouds, trackers and language, creates the vault, and kos doctor reports it installed and current.">

More: [installation and updates](docs/installation.md) · [the kos CLI](docs/cli.md) · [decisions](docs/adr/)

<details>
<summary><b>What the cell owns</b></summary>
<br>

The cell owns `instance.yaml`, `00-Home.md`, the root Bases, notes under `10/`–`70/` and `investigations/`. The distribution owns the `AGENTS.md` router, the generic files under `90-Meta/`, the skills and the specialist definitions; it ships no CI workflow, so the organization's pipeline never depends on `kos`. `kos kernel update` refuses kernel files edited in the vault until `--force` and never rewrites cell-owned files. Details in [installation](docs/installation.md#ownership-in-a-cell).

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
| `docs/` | Guides, [decisions](docs/adr/) and the README images, rendered from the flow guide (`python3 -B scripts/render_readme_assets.py`, Chrome required); `docs/demo/` holds the synthetic cell and VHS tapes behind the recordings; `docs/flows/` the animated flow guide (one script per flow in `docs/flows/flows/`) |

```bash
make test            # go vet and go test
make release         # cross-platform binaries (required before the installation tests)
make test-installer
make demo            # re-record the README GIFs (vhs and jq required)
```

</details>

## License

[Apache License 2.0](LICENSE)
