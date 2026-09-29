# The kos CLI

`kos --help` lists the commands:

```text
init · adopt · doctor
inventory · audit
discover run|questions|answer|platform|report|check|corrections
config status|resolve|workspace|locate|capability|bind|catalog|areas|operation|…
check links|bases
investigation new|list|check|add|state|absorb|close|reopen
handoff start|status|refresh|reconcile
sync start|status|review|verify|acknowledge|finish|pull
kernel status|update [--all] · vaults · version · update
```

## What each area does

Reading and searching notes, code and snapshots is left to the agent's own tools; kos builds the map and checks it.

- **Discovery** (`discover`): connection facts extracted deterministically from repositories (imports, manifests, configuration and IaC), typed by stored judgments and completed by read-only platform snapshots of the cell's clouds (Google Cloud, AWS and Azure: messaging, databases, storage and warehouse, each read with its own CLI and the developer's login). What the providers do not read, the agent inspects with the tools in reach and records (`discover platform --record`). Note gates (`discover check`) verify anchors, coverage and stale citations, and refuse what keeps a note from working as a map (G4): connections it names without a typed relation (topics and queues, HTTP and gRPC services of other repositories, SFTP or FTP integrations), core sections left empty while the claims sit elsewhere, paragraphs copied into another note. `sync verify` applies them to every synced note and refuses a topic, flow or other note on the branch that copies a repository note. A repository note cites its own repository in the short form `path#Lfrom-Lto — text`; `discover shorten` rewrites permalinks into it. The kernel installs `.github/workflows/knowledge-gates.yml`, which runs `sync verify --allow-no-change` on every push and pull request, so a knowledge change committed outside a reviewed sync branch fails on the remote.
- **Investigations** (`investigation`): one case per line of work, written only through the CLI, gated on every write, and published, absorbed or retired on a sync branch.
- **Development handoffs** (`handoff`): atomic task packages prepared in a repository worktree, with a managed `AGENTS.md` section that tells any harness how to work with them, and a deterministic reconciliation back into the case.
- **Publication through Git** (`sync`): knowledge changes on a `sync/<slug>` branch, with gates, a review bound to the exact content and a fast-forward finish. The review is also a record under `90-Meta/sync-reviews/`, so the push gate finds it by content after a rebase, squash or merge-commit merge.

Discovery judgments are answered by the agent through `discover questions` and `discover answer`.

`config locate --repo NAME` binds a repository named by its note, an alias or its repository name to the local checkout and its reference branch, and says whether the note's `commit-analizado` matches that branch, without contacting a remote.

Failures, unwanted behaviors and proposals come back through the `report-to-distribution` skill as sanitized issues (templates in `.github/ISSUE_TEMPLATE/`).

## Releases

`make release` builds `kos` for macOS, Linux and Windows on ARM64 and AMD64 under `dist/`, with `SHA256SUMS`, the installer scripts, third-party notices and `release.json` (the source fingerprint the installer checks). Each binary embeds the kernel payload. `make publish` tags `v<version>` and publishes `dist/` as the GitHub release of `rendis/knowledge-os` with the `rendis` account.
