---
name: use-vault-cli
description: Operate the installed kos CLI for vault resolution, discovery, gates and kernel checks, and interpret its results or failures. Read before the first CLI operation; workflow skills retain ownership of configuration and lifecycle changes.
---

# Use the vault CLI

This is the shared operational reference for `kos`. Keep the current primary workflow; use its procedure for mutations, reviews and recovery. Reuse this reference while it remains in context.

## Bind the executable and vault

`kos` is installed once per machine (usually `~/.local/bin/kos`) and serves every vault; the vault carries no executable. Check it with `kos version`: it reports this kos, the kernel it carries, the vault's kernel and a newer release when there is one. Throughout kernel instructions `<CLI>` means `kos` (or its absolute path when it is not on `PATH`; in PowerShell prefix a quoted path with `&`).

**When `kos` is not installed**, tell the user and offer two paths:
- Install it, with their explicit approval: `curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh` (macOS, Linux, WSL; Windows PowerShell: `irm https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.ps1 | iex`). It verifies the checksum and installs nothing else.
- Continue without it: notes, sources and platforms can still be read, but no gate runs (`discover check`, `investigation check`, `sync verify`), so report the unavailable mechanical checks. Directly inspected evidence can still support bounded claims; publication waits for its required gates. Creating or updating a case requires user write authority.

**Notices** arrive on stderr as `kos notice: …` lines; stdout stays JSON. A newer kos: tell the user and, with their approval, run `kos update`; its result lists the vaults on this machine that now carry an older kernel. Offer `kos kernel update --all --dry-run` for them, show what changes, and on approval run it with `--commit` (one commit per vault, not pushed); a vault not found at its recorded path is reported with its fix. The vault's kernel older than kos: offer `kos kernel update --vault "<root>" --dry-run`, show what changes, and apply it on approval as one commit. The vault's kernel newer than kos: `kos update` first; publishing gates refuse to run until then.

An installed copy of this skill provides a candidate vault root three directories above its directory. A source repository or the current working directory is not automatically the vault. `--vault` defaults to `KOS_VAULT` or the vault that contains the working directory; pass it explicitly from anywhere else.

```text
<CLI> config resolve --vault "<candidate-path>"
```

Bind the returned `vault_root` on successful resolution and pass it explicitly as `--vault` on vault-scoped commands. Read [vault resolution](../../../90-Meta/vault-resolution.md) when resolution fails or the task requires a source checkout. Bootstrap instructions can be read before resolution; domain evidence requires the binding.

Execute the commands documented in this reference or the owning workflow, substituting the resolved paths and required inputs. Consult CLI help only to resolve a specific syntax, option or version mismatch that the procedure does not answer. Help describes the interface; the owning workflow defines mutation prerequisites, review and recovery.

## Choose the operation

| Need | Operation |
|---|---|
| Read notes, code or snapshots | Your own file and search tools; code at the repository's reference branch: `config locate --repo NAME` (note, alias or repository name) returns the checkout (`path`), the reference branch with its `ref` and whether the note matches it, without the network; then `git -C <path> show <ref>:<file>` |
| Initial vault orientation | `config status --vault "<root>"` |
| Propose this person's workspace (clones, worktrees, cloud logins, ports) | `config --vault "<root>" detect` |
| Discover connections of the cell's repositories | `discover run --vault "<root>" --read-only` for a question; omit `--read-only` only with discovery-state write authority (see [Discovery](#discovery)) |
| Validate structure | Use the workflow-required `check` or `audit` command |
| Change configuration, investigations, handoffs or synchronization state | Follow the owning skill, then invoke its CLI commands |

## Discovery

`discover run` scans every tracked repository at an exact commit (default: the configured reference branch or the first available branch in `sources.reference_branch_order`; `--at note` uses each note's `commit-analizado`), parses code imports, build manifests and every configuration/IaC file by format, applies stored judgments and platform snapshots, and writes facts, pending items and the comparison with notes under `.agents/state/discovery/`. It prints a summary. For a read-only question, add `--read-only`: it returns `report`, `facts`, `questions`, `comparison` and `gaps` without writing state or pruning stored judgments. Facts are deterministic evidence pointers: repository, commit, file and key.

Some judgments need semantic understanding (what a dependency talks to, what a configuration key holds). They are asked once, answered by you and stored, versioned, in `90-Meta/discovery/classifications.json`: run `discover questions --vault "<root>" --limit 100`, answer each question with exactly one of its `options` (judge only from the question's `state`), save `[{"id":..,"choice":..,"confidence":0-1}]` to a file, record it with `discover answer --vault "<root>" --file <file>`, and repeat until `pending` is 0, then `discover run --vault "<root>"`. To correct a recorded judgment (for example after a review finding), answer its id the same way; the choice must be one of its kind's options.

`discover platform --vault "<root>" --referenced` captures read-only listings, names and relations only, of the clouds in `instance.yaml` `platform.providers`: messaging (topics, subscriptions with their delivery settings where the CLI lists them, queues), document and SQL databases (Firestore, DynamoDB, Cosmos DB; Cloud SQL, RDS, Azure SQL and PostgreSQL), object storage and, for Google Cloud, BigQuery datasets and tables. Each service keeps its own status (`kinds`), so a disabled or unreadable one leaves the rest captured. It reads the scopes configuration names with each cloud's own CLI and the developer's login: `gcp` (project id, `gcloud` and `bq`), `aws` (`<account>/<region>`, `aws` with the profile of that account in `AWS_PROFILE`), `azure` (subscription id, `az`). `--provider NAME --scope ID` captures one scope. Reuse the user's still-applicable read authorization for that target and scope; request authorization only for a new scope or missing permission. Use `--read-only` to return snapshots without storing them. Persisting snapshots requires write authority even when the cloud commands only read. A scope that cannot be read (`auth-required`, `denied`, `not-found`, `unavailable`) is stored with the exact confirm command and stays as a `platform-access` pending item; after `auth-required`, the user logs in and the capture is repeated. Nothing unverified is presented as confirmed.

The providers are the floor. A `platform-unmanaged` pending item names a service the cell uses that no configured provider reads (for example SFTP, a scheduler, a cluster); inspect it read-only with the tools in reach (the cloud's CLI, `kubectl`, a database or SFTP client, a configured procedure) under the same authorization as `discover platform`: reuse still-applicable read permission and the configured procedure for the same target and scope. Return observations without writes for a question. With explicit retention authority, record names and relations so every reader can reuse them: `discover platform --vault "<root>" --record <file>` with `{"provider", "scope", "service", "command", "resources": [{"name", "kind", "links": [{"relation", "target"}]}]}`. It is stored under `90-Meta/discovery/platform/observed/` (publish it on a sync branch) and becomes `platform-observed` evidence in the facts. Names and relations only; a credential is refused.

`discover run` reports `credentials_in_sources`: credentials versioned in a repository's configuration, by file and key (the value is redacted, never judged or stored). Tell the user where they are; fixing the source is its owners' decision.

`discover report --vault "<root>" [--repo NAME]` returns the last summary or one repository's facts and comparison.

`discover check --vault "<root>" --note <path> [--note <other path> ...]` gates a repository note or candidate (a candidate outside the vault is matched by its `aliases`): G1 source permalinks resolve at their commit and the backticked identifiers of each footnote are in the cited lines; G2 every connector category and resource group of the facts is cited or named; G3 (`review`) a relation to a topic/event note that the repository's evidence does not name; G4 (`error`) whether the note works as a map: undeclared typed relations (topics and queues, HTTP and gRPC services, SFTP/FTP integrations), empty core sections beside cited claims elsewhere, paragraphs copied into another note; stale cited files since `commit-analizado`. A repository note cites its own repository at `commit-analizado` in the short form `path#L1-L9 — text` (a permalink there fails `G1-format`; `discover shorten --vault "<root>" --note <path>` rewrites them), and a note without anchors fails G1. `error` blocks publication (a note touched only to re-anchor may keep the errors its base had, never add one). Pending G1/G2 source validation also fails the check and publication, including pre-existing unavailable evidence; other `pending` items become `Verificaciones pendientes`, and `review` goes to the reviewer. Batch related notes in one command to reuse source lookups. For a question add `--read-only` to prevent cache writes. `ok` means no blocking gate errors; `verified` also requires no pending evidence or unresolved review. Neither flag proves live execution or semantic acceptance without the required evidence review.

`discover corrections --vault "<root>"` lists every note relation the last run found unsupported, as a correction task.

## Failures

Check the exit status and error output before interpreting results. A failed command is not an empty result. Correct the identified input or prerequisite and retry; use the owning workflow's recovery for stateful operations instead of repeating a mutation blindly.

If the executable is missing, incompatible or cannot start, report its exact path and error and restore the matching versioned artifact through the repository or distribution update workflow within existing authority. Do not substitute hand-written state mutations for unavailable CLI operations. Continue only independent work whose required binding and checks are already satisfied. Source acquisition and specialized workflows may require their own external tools.
