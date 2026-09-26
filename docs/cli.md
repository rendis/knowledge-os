# The kos CLI

`kos --help` lists the commands:

```text
init · adopt · doctor
overview · search · index · links · inventory · audit
discover run|questions|answer|platform|report|check|claims|corrections
config status|resolve|workspace|locate|capability|bind|catalog|areas|operation|…
check links|bases|obsidian-binding|map-closure
investigation new|list|check|add|state|absorb|close|reopen
handoff start|status|refresh|reconcile
sync start|status|review|verify|acknowledge|finish|pull
kernel status|update [--all] · vaults · version · update
```

## What each area does

- **Discovery** (`discover`): connection facts extracted deterministically from repositories (imports, manifests, configuration and IaC), typed by stored judgments and completed by read-only platform snapshots of the cell's clouds (Google Cloud, AWS and Azure: messaging, databases, storage and warehouse, each read with its own CLI and the developer's login). What the providers do not read, the agent inspects with the tools in reach and records (`discover platform --record`). Note gates (`discover check`) verify anchors, coverage and stale citations; `discover claims` checks a draft answer's resource names.
- **Investigations** (`investigation`): one case per line of work, written only through the CLI, gated on every write, and published, absorbed or retired on a sync branch.
- **Development handoffs** (`handoff`): atomic task packages prepared in a repository worktree, with a managed `AGENTS.md` section that tells any harness how to work with them, and a deterministic reconciliation back into the case.
- **Publication through Git** (`sync`): knowledge changes on a `sync/<slug>` branch, with gates, a review bound to the exact content and a fast-forward finish.

Discovery judgments are answered by Jev when `TYPESAFE_API_KEY` is set, otherwise by the agent through `discover questions` and `discover answer`. Search keeps a private local SQLite index outside the vault; results are pointers to open and verify, not answers. No hooks or model services are installed.

Failures, unwanted behaviors and proposals come back through the `report-to-distribution` skill as sanitized issues (templates in `.github/ISSUE_TEMPLATE/`).

## Releases

`make release` builds `kos` for macOS, Linux and Windows on ARM64 and AMD64 under `dist/`, with `SHA256SUMS`, the installer scripts, third-party notices and `release.json` (the source fingerprint the installer checks). Each binary embeds the kernel payload. `make publish` tags `v<version>` and publishes `dist/` as the GitHub release of `rendis/knowledge-os` with the `rendis` account.
