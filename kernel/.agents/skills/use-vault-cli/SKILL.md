---
name: use-vault-cli
description: Operate the installed kos CLI for vault resolution, retrieval and kernel checks, and interpret its results or failures. Read before the first CLI operation; workflow skills retain ownership of configuration and lifecycle changes.
---

# Use the vault CLI

This is the shared operational reference for `kos`. Keep the current primary workflow; use its procedure for mutations, reviews and recovery. Reuse this reference while it remains in context.

## Bind the executable and vault

`kos` is installed once per machine (usually `~/.local/bin/kos`) and serves every vault; the vault carries no executable. Check it with `kos version`: it reports this kos, the kernel it carries, the vault's kernel and a newer release when there is one. Throughout kernel instructions `<CLI>` means `kos` (or its absolute path when it is not on `PATH`; in PowerShell prefix a quoted path with `&`).

**When `kos` is not installed**, tell the user and offer two paths:
- Install it, with their explicit approval: `curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh` (macOS, Linux, WSL; Windows PowerShell: `irm https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.ps1 | iex`). It verifies the checksum and installs nothing else.
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
| Find every note that speaks about a question, with its source state | `ask --vault "<root>" --query "<terms>" [--budget CHARS]` — see [Use the map](#use-the-map) |
| Read a note's section or lines with their sources checked | `read --vault "<root>" --note NAME [--section TEXT \| --lines FROM-TO] [--match TERMS]` |
| Read a repository at its reference branch | `code --vault "<root>" --repo NAME\|all (--grep REGEX [-i] [--tests] \| --show PATH[:FROM-TO[,FROM-TO…]][,PATH…] \| --func NAME[,NAME…] [--up N] [--down])` |
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

## Use the map

`ask` answers where the notes speak about a question, not what the answer is. Within `--budget` characters (default 24 000) it writes:

- **No note contains** the query's terms that no note holds (what they name is absent or called differently), and the terms too frequent to locate anything, left out.
- **Notes holding the question's terms**, all of them: first those the query names (basename or alias), then by how many distinct terms each holds and their rarity. For each, its summary; **Lines** with every term's line numbers; whether **the paragraphs with these terms** still stand on their sources (`✓` the cited lines read the same on the reference branch, the file may have changed elsewhere; `⚠ L61` the cited lines changed since the cited commit: read them there; `?` not checkable here; `cite nothing`: a claim to confirm in the code); for a repository note, its freshness against `commit-analizado`, a checkout that is not at the reference branch, the relations the last `discover run` found unsupported and the resources in the code it does not name. Notes past the budget are named with how many terms each holds. `90-Meta/` guidance is left out.
- For a topic the query names, its wiring (as `read` gives it, below); for a subscription it names, what it reads from the platform snapshots.
- The cases that hold at least half of the terms, the sources beyond the vault and the legend.

Read the lines it gives in each note that bears on the question: the note with the most matches is not the whole story, and a second note can describe a second path. Opening the file finds the same lines; `read` adds the check of every footnote they cite.

Write the query as terms, not as the user's sentence: names and identifiers plus the words the notes would use, in the notes' language, with synonyms. A word matches by its stem (`deduplica` finds `deduplicación`, never `idempotencia`), a word of three letters as a whole word (`ack`); an identifier with `-`, `.`, `_`, digits or inner capitals matches whole; `"a quoted phrase"` matches exactly.

`read` of a topic note adds its wiring: the repositories that publish to it or consume it (read from where their code or application configuration uses the key that holds its name: `publishers.orders.topic-id: ${ORDERS_TOPIC}`), the infrastructure that declares it, the event type literals each publisher's code writes (with their lines; a constant declared and used only in comments, or never, is marked), the subscriptions each captured platform scope has on it with the repositories that configure them (or none, accounting for every snapshot: those listing subscriptions, those listing none and those whose capture failed, with the provider command that lists the topic's subscriptions from every project; each subscription's ack deadline, redelivery backoff, attempts before its dead letter and retention when the snapshot captured them), and the topics with the same last segment it must not be confused with. `read` of a repository note starts with its freshness and checkout.

`read --note NAME [--section TEXT[|TEXT…] | --lines FROM-TO[,FROM-TO…]] [--match TERMS] [--brief]` takes a basename, alias or path (a near name gets suggestions; an unknown section lists the sections) and returns the paragraphs overlapping those ranges or sections whole, never cut (a single line number takes the paragraph nearest to it), with their lines (a line range skips the note's summary and outline), only those holding a term when `--match` is given, with every footnote they cite resolved and checked the same way, within 24 000 characters by default (cited lines take at most a quarter); a run of near-identical paragraphs (one per environment) shows its first, then each other's line with what differs in it (`L46 [production/acco]`); `… continues at` gives the next range, or `--budget 60000` reads the rest at once.

`code --repo NAME` reads a repository at its reference branch whatever its working tree; `NAME` is the repository, its note's basename or an alias. Tests, fixtures, mocks, build wrappers, generated assets, source maps and lock files are left out, and every credential value it prints is `‹redacted›`. `--func NAME[,NAME…]` lists every definition with that name first when there are several (`--path GLOB` keeps one), shows each whole (up to 150 lines) with its **exits** (every return, throw, error response, process exit, ack or nack, catch that carries on, `continue` and `break` of a loop and falling off its end, each with the condition of the block it is in, and **on redelivery**: a check that returns success early, a write, then a failure after a later write, so a retry leaves at the check without the later writes), the settings it reads and their values, and every caller (a call or the function passed as a value, never the name inside a message; in Go, not a same-named function of another package), the first six with the lines after the call and the function it is in, the rest listed, with how many other definitions share the name (a call through an interface may reach any); a function nothing calls by name gets where its class or type is provided, registered or constructed (`useClass: AuthInterceptor`), since a framework may call it; a function passed as a value in same-shaped lines (`app.get('/x', authMiddleware, handler)`) gets the lines of that shape that do not pass it: the routes a middleware leaves out; a setting whose value is a URL gets the repositories whose code or API definition declares that path (`…/auth/validate, declared in: …`), searching those named like the URL first; `--up N` prints the chain of callers N levels up and, for the first path, each hop with what the caller does with the result; `--down` lists the repository's functions it calls, in call order, each with its exits. A name that is not a function (a constant, a variable, a type, a key) gets its declaration and every use grouped by the function it is in. `--show PATH[:FROM-TO[,FROM-TO…]][,PATH…]` shows files numbered (`a.go:1-5,b.go:3-9,20-30`); a range that does not parse is an error. `--grep REGEX` (Perl-compatible where git has PCRE, POSIX extended otherwise, named in the output; a pattern git rejects is an error, never an empty result) (`-i`, `--tests`, `--path GLOB`) lists every matching line under its file with its enclosing function, code before configuration, and says how many files it searched: no match is an absence stated with its pattern, branch and scope (a name held in configuration under another key is not found by a literal search). `--repo all` greps every checkout and names those without a match.

`discover claims` checks the names a draft writes in backticks and, without them, the ones the vault knows or that have a topic's shape; it lists the `names` it checked, and `names_checked: 0` comes with a note saying nothing was checked.

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

`discover platform --vault "<root>" --referenced` captures read-only listings, names and relations only, of the clouds in `instance.yaml` `platform.providers`: messaging (topics, subscriptions with their delivery settings where the CLI lists them, queues), document and SQL databases (Firestore, DynamoDB, Cosmos DB; Cloud SQL, RDS, Azure SQL and PostgreSQL), object storage and, for Google Cloud, BigQuery datasets and tables. Each service keeps its own status (`kinds`), so a disabled or unreadable one leaves the rest captured. It reads the scopes configuration names with each cloud's own CLI and the developer's login: `gcp` (project id, `gcloud` and `bq`), `aws` (`<account>/<region>`, `aws` with the profile of that account in `AWS_PROFILE`), `azure` (subscription id, `az`). `--provider NAME --scope ID` captures one scope. It contacts the cloud, so run it only with the user's authorization. A scope that cannot be read (`auth-required`, `denied`, `not-found`, `unavailable`) is stored with the exact confirm command and stays as a `platform-access` pending item; after `auth-required`, the user logs in and the capture is repeated. Nothing unverified is presented as confirmed.

The providers are the floor. A `platform-unmanaged` pending item names a service the cell uses that no configured provider reads (for example SFTP, a scheduler, a cluster); inspect it read-only with the tools in reach (the cloud's CLI, `kubectl`, a database or SFTP client, a configured procedure) under the same authorization as `discover platform`: ask the user once per session before contacting a cloud or remote system, then record the names and relations you observed so every reader reuses them: `discover platform --vault "<root>" --record <file>` with `{"provider", "scope", "service", "command", "resources": [{"name", "kind", "links": [{"relation", "target"}]}]}`. It is stored under `90-Meta/discovery/platform/observed/` (publish it on a sync branch) and becomes `platform-observed` evidence in the facts and a known name for `claims`. Names and relations only; a credential is refused.

`discover run` reports `credentials_in_sources`: credentials versioned in a repository's configuration, by file and key (the value is redacted, never judged or stored). Tell the user where they are; fixing the source is its owners' decision.

`discover report --vault "<root>" [--repo NAME]` returns the last summary or one repository's facts and comparison.

`discover check --vault "<root>" --note <path> [--semantic]` gates a repository note or candidate (a candidate outside the vault is matched by its `aliases`): G1 source permalinks resolve at their commit and the backticked identifiers of each footnote are in the cited lines; G2 every connector category and resource group of the facts is cited or named; G3 (`review`) a relation to a topic/event note that the repository's evidence does not name; stale cited files since `commit-analizado`. `error` blocks publication; `pending` becomes `Verificaciones pendientes`; `review` goes to the reviewer. `--semantic` (Jev) flags cited sentences the code does not support; it is a review aid for atomic sentences, not a verdict.

`discover claims --vault "<root>" --file <draft>` (`--file -` reads the draft from standard input, so no file is needed: `kos discover claims --file - <<'EOF' … EOF`) checks a draft answer before delivery: quoted or linked resource names that no discovery fact, platform snapshot or vault text knows (`unknown_names`), and relations the last run found unsupported that the answer mentions (`contradicted_relations`). It needs a previous `discover run`. `discover corrections --vault "<root>"` lists every unsupported note relation as a correction task.

## Refresh and failures

Retrieval refreshes the local SQLite index automatically by comparing file fingerprints and updating changed files. Markdown remains authoritative. Run `index --vault "<root>" --rebuild` only for an explicit rebuild request or diagnosed index recovery; an empty search alone is not evidence of corruption.

Check the exit status and error output before interpreting results. A failed command is not an empty search. Correct the identified input or prerequisite and retry; use the owning workflow's recovery for stateful operations instead of repeating a mutation blindly.

If the executable is missing, incompatible or cannot start, report its exact path and error and restore the matching versioned artifact through the repository or distribution update workflow within existing authority. Do not substitute hand-written state mutations for unavailable CLI operations. Continue only independent work whose required binding and checks are already satisfied. Local search needs no Obsidian, Git, Python or model service; source acquisition and specialized workflows may require their own external tools.
