---
name: use-vault-cli
description: Operate the installed kos CLI for vault resolution, retrieval and kernel checks, and interpret its results or failures. Read before the first CLI operation; workflow skills retain ownership of configuration and lifecycle changes.
---

# Use the vault CLI

This is the shared operational reference for `kos`. Keep the current primary workflow; use its procedure for mutations, reviews and recovery. Reuse this reference while it remains in context.

## Bind the executable and vault

`kos` is installed once per machine (usually `~/.local/bin/kos`) and serves every vault; the vault carries no executable. Check it with `kos version`: it reports this kos, the kernel it carries, the vault's kernel and a newer release when there is one. Throughout kernel instructions `<CLI>` means `kos` (or its absolute path when it is not on `PATH`; in PowerShell prefix a quoted path with `&`).

**When `kos` is not installed**, tell the user and offer two paths:
- Install it, with their explicit approval: `gh release download --repo rendis/knowledge-os --pattern install-kos.sh --output - | sh` (macOS, Linux, WSL; Windows: the same with `install-kos.ps1`). It needs the GitHub CLI logged in with an account that can read that repository; nothing else is installed.
- Continue without it: notes, sources and platforms can still be read, but no gate runs (`discover check`, `claims`, `investigation check`, `sync verify`), so every statement is unverified and nothing is published or written to a case. Say so in the answer.

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
| Initial vault orientation | `config status --vault "<root>"` |
| Propose this person's workspace (clones, worktrees, cloud logins, ports) | `config --vault "<root>" detect` |
| Choose which notes to open | `overview --vault "<root>" [--folder 20-Repos]` — one line per note: type, relations, first sentence; always current |
| Find a term the overview does not reveal | `search --vault "<root>" --query "<subject terms>" --limit 5` |
| Inspect a known note's relationships | `links --vault "<root>" --node "<note basename>"` |
| Inspect an identified source | Read its file directly with the available file tool |
| Discover connections of the cell's repositories | `discover run --vault "<root>"` (see [Discovery](#discovery)) |
| Validate structure | Use the workflow-required `check` or `audit` command |
| Change configuration, investigations, handoffs or synchronization state | Follow the owning skill, then invoke its CLI commands |

Form search terms from the substantive subject and identifiers in the request and established conversation. A confirmation or continuation adds no reason to repeat a completed search. Search again when a new question, changed source or unresolved evidence gap requires it. The CLI provides lexical retrieval, not semantic interpretation of conversational intent.

## Use retrieval results

`search` returns bounded `cards` with vault, path, title, section, line, excerpt, origin, visibility and source hash. Open the relevant source at that location and inspect enough surrounding content to resolve the question. BM25 orders lexical matches; it is neither a confidence percentage nor a test of factual support. Preserve source provenance: an investigation is case context, and a note is not automatically canonical.

Default retrieval includes eligible local/private investigation material. Use `--visibility public` when private material must be excluded from retrieval; public visibility alone does not authorize external disclosure. For any case used as evidence, follow `manage-investigation`'s read-only load to obtain the complete case context and overlays.

An empty `cards` array after a successful command means no results for that query and scope. Check the subject terms and visibility, try a concrete identifier or documented alternative term when relevant, and inspect a known source directly. The index covers eligible vault Markdown, not external repositories, databases or live services. Route remaining evidence gaps to the appropriate source workflow.

`links` returns recorded incoming/outgoing relationships and investigation pointers. Inspect truncation indicators and source notes when completeness matters; recorded relationships alone do not establish runtime behavior.

## Discovery

`discover run` scans every tracked repository at an exact commit (default: remote default branch; `--at note` uses each note's `commit-analizado`), parses code imports, build manifests and every configuration/IaC file by format, applies stored judgments and platform snapshots, and writes facts, pending items and the comparison with notes under `.agents/state/discovery/`. It prints a summary. Facts are deterministic evidence pointers: repository, commit, file and key.

Some judgments need semantic understanding (what a dependency talks to, what a configuration key holds). They are asked once and stored, versioned, in `90-Meta/discovery/classifications.json`:

- With `TYPESAFE_API_KEY` set, `discover run` answers them automatically.
- Without it, run `discover questions --vault "<root>" --limit 100`, answer each question with exactly one of its `options` (judge only from the question's `state`), save `[{"id":..,"choice":..,"confidence":0-1}]` to a file, record it with `discover answer --vault "<root>" --file <file>`, and repeat until `pending` is 0, then `discover run --vault "<root>" --classify off`. Tell the user once that setting the key makes this automatic; missing acceleration never blocks the task. To correct a recorded judgment (for example after a review finding), answer its id the same way; the choice must be one of its kind's options.

`discover platform --vault "<root>" --referenced` captures read-only listings, names and relations only, of the clouds in `instance.yaml` `platform.providers`: messaging (topics, subscriptions, queues), document and SQL databases (Firestore, DynamoDB, Cosmos DB; Cloud SQL, RDS, Azure SQL and PostgreSQL), object storage and, for Google Cloud, BigQuery datasets and tables. Each service keeps its own status (`kinds`), so a disabled or unreadable one leaves the rest captured. It reads the scopes configuration names with each cloud's own CLI and the developer's login: `gcp` (project id, `gcloud` and `bq`), `aws` (`<account>/<region>`, `aws` with the profile of that account in `AWS_PROFILE`), `azure` (subscription id, `az`). `--provider NAME --scope ID` captures one scope. It contacts the cloud, so run it only with the user's authorization. A scope that cannot be read (`auth-required`, `denied`, `not-found`, `unavailable`) is stored with the exact confirm command and stays as a `platform-access` pending item; after `auth-required`, the user logs in and the capture is repeated. Nothing unverified is presented as confirmed.

The providers are the floor. A `platform-unmanaged` pending item names a service the cell uses that no configured provider reads (for example SFTP, a scheduler, a cluster); inspect it read-only with the tools in reach (the cloud's CLI, `kubectl`, a database or SFTP client, a configured procedure), then record the names and relations you observed so every reader reuses them: `discover platform --vault "<root>" --record <file>` with `{"provider", "scope", "service", "command", "resources": [{"name", "kind", "links": [{"relation", "target"}]}]}`. It is stored under `90-Meta/discovery/platform/observed/` (publish it on a sync branch) and becomes `platform-observed` evidence in the facts and a known name for `claims`. Names and relations only; a credential is refused.

`discover run` reports `credentials_in_sources`: credentials versioned in a repository's configuration, by file and key (the value is redacted, never judged or stored). Tell the user where they are; fixing the source is its owners' decision.

`discover report --vault "<root>" [--repo NAME]` returns the last summary or one repository's facts and comparison.

`discover check --vault "<root>" --note <path> [--semantic]` gates a repository note or candidate (a candidate outside the vault is matched by its `aliases`): G1 source permalinks resolve at their commit and the backticked identifiers of each footnote are in the cited lines; G2 every connector category and resource group of the facts is cited or named; G3 (`review`) a relation to a topic/event note that the repository's evidence does not name; stale cited files since `commit-analizado`. `error` blocks publication; `pending` becomes `Verificaciones pendientes`; `review` goes to the reviewer. `--semantic` (Jev) flags cited sentences the code does not support; it is a review aid for atomic sentences, not a verdict.

`discover claims --vault "<root>" --file <draft>` checks a draft answer before delivery: quoted or linked resource names that no discovery fact, platform snapshot or vault text knows (`unknown_names`), and relations the last run found unsupported that the answer mentions (`contradicted_relations`). It needs a previous `discover run`. `discover corrections --vault "<root>"` lists every unsupported note relation as a correction task.

## Refresh and failures

Retrieval refreshes the local SQLite index automatically by comparing file fingerprints and updating changed files. Markdown remains authoritative. Run `index --vault "<root>" --rebuild` only for an explicit rebuild request or diagnosed index recovery; an empty search alone is not evidence of corruption.

Check the exit status and error output before interpreting results. A failed command is not an empty search. Correct the identified input or prerequisite and retry; use the owning workflow's recovery for stateful operations instead of repeating a mutation blindly.

If the executable is missing, incompatible or cannot start, report its exact path and error and restore the matching versioned artifact through the repository or distribution update workflow within existing authority. Do not substitute hand-written state mutations for unavailable CLI operations. Continue only independent work whose required binding and checks are already satisfied. Local search needs no Obsidian, Git, Python or model service; source acquisition and specialized workflows may require their own external tools.
