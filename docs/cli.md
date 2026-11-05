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
sync start|commit|status|review|verify|acknowledge|finish|pull
kernel status|update [--all] · vaults · version · update
```

## What each area does

Reading and searching notes, code and snapshots is left to the agent's own tools; kos builds the map and checks it.

- **Discovery** (`discover`): connection facts extracted deterministically from repositories (imports, manifests, configuration and IaC), typed by stored judgments and completed by read-only platform snapshots of the cell's clouds (Google Cloud, AWS and Azure: messaging, databases, storage and warehouse, each read with its own CLI and the developer's login). What the providers do not read, the agent inspects with the tools in reach and records (`discover platform --record`). Note gates (`discover check`) verify anchors, coverage and stale citations, and refuse what keeps a note from working as a map (G4): connections it names without a typed relation (topics and queues, HTTP and gRPC services of other repositories, SFTP or FTP integrations), core sections left empty while the claims sit elsewhere, paragraphs copied into another note. `sync verify` applies them to every synced note and refuses a topic, flow or other note on the branch that copies a repository note. A repository note cites its own repository in the short form `path#Lfrom-Lto — text`; `discover shorten` rewrites permalinks into it. The gates run locally in the sync flow; the kernel installs no CI workflow, so a team's pipeline never depends on `kos` ([ADR 0006](adr/0006-ci-stays-agnostic.md)).
- **Investigations** (`investigation`): one case directory per line of work, written only through the CLI and gated on every write. Local work, including edits to published cases, preserves the existing checkout and branch. Branches or worktrees require a concrete workflow need; publication, absorption and retirement retain their authorization, review and publication gates.
- **Development handoffs** (`handoff`): atomic task packages prepared in a repository worktree, with a managed `AGENTS.md` section that tells any harness how to work with them, and a deterministic reconciliation back into the case.
- **Publication through Git** (sync): with an origin, prepare a named candidate from origin/main while the live checkout remains on main. Explicit paths from the isolated draft define the checkpoint. The review record travels with the candidate so published-range verification works after squash, rebase or merge.

Set --vault to the live vault on every command. The draft returned by sync start is an edit directory, not a checkout.

1. `sync start --name <slug>` fetches and pins origin/main. Use `--base <branch>` for a different designated base. Use `sync status --name <slug>` to find the draft, base_commit and head again.
2. Edit files in the returned draft directory, then run `sync commit --name <slug> --path <file> --message "<message>"`. Repeat --path for each selected file, including deletions. Use vault-relative paths.
3. Run `sync verify --name <slug>` to check the committed candidate. After independent review of `git diff <base_commit> <head>`, record the verdict with `sync review --name <slug> --head <reviewed-sha> --verdict accept --reviewer <reviewer>`.
4. Run `sync finish --name <slug>`. It rechecks against the latest upstream base and returns ready_for_pr and push_refspec on success. Push the given refspec to origin and open a PR targeting the reported base according to repository policy. Finish itself does not push, open a PR or advance local main.

To publish pending edits in one published investigation, use `--case <ID>` on start with publication authority. Only that case's pending files are imported. Verify the listed files before checkpointing. Unrelated edits and staging do not affect the run.

With no origin, sync uses the local branch workflow: start switches to the sync branch, normal Git commits checkpoint work, and finish verifies and fast-forwards the base. Continue these runs and already checked-out legacy sync branches without --name. Use `sync verify --base <revision> --allow-no-change` to check existing published ranges.


Discovery judgments are answered by the agent through `discover questions` and `discover answer`.

`config locate --repo NAME` binds a repository named by its note, an alias or its repository name to the local checkout and its reference branch, and says whether the note's `commit-analizado` matches that branch, without contacting a remote.

Failures, unwanted behaviors and proposals come back through the `report-to-distribution` skill as sanitized issues (templates in `.github/ISSUE_TEMPLATE/`).

## Releases

`make release` builds `kos` for macOS, Linux and Windows on ARM64 and AMD64 under `dist/`, with `SHA256SUMS`, the installer scripts, third-party notices and `release.json` (the source fingerprint the installer checks). Each binary embeds the kernel payload. `make publish` tags `v<version>` and publishes `dist/` as the GitHub release of `rendis/knowledge-os` with the `rendis` account.
