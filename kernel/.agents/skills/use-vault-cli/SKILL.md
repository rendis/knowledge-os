---
name: use-vault-cli
description: Operate the installed vaultctl CLI for vault resolution, retrieval and kernel checks, and interpret its results or failures. Read before the first CLI operation; workflow skills retain ownership of configuration and lifecycle changes.
---

# Use the vault CLI

This is the shared operational reference for `vaultctl`. Keep the current primary workflow; use its procedure for mutations, reviews and recovery. Reuse this reference while it remains in context.

## Bind the executable and vault

Each vault carries the same generic CLI builds in its versioned `.agents/bin/` directory. The executable reads that vault's configuration and Markdown through explicit root arguments; no central registration or per-vault compilation is needed. A clone includes all six builds.

Choose the executable once for the current execution host (inside a container or remote shell, use that environment's platform):

| Host | Filename under `.agents/bin/` |
|---|---|
| macOS ARM64 / AMD64 | `vaultctl-darwin-arm64` / `vaultctl-darwin-amd64` |
| Linux ARM64 / AMD64 | `vaultctl-linux-arm64` / `vaultctl-linux-amd64` |
| Windows ARM64 / AMD64 | `vaultctl-windows-arm64.exe` / `vaultctl-windows-amd64.exe` |

Use observed harness environment information; when unknown, inspect `uname -sm` on macOS/Linux or `$env:PROCESSOR_ARCHITEW6432` (when set), otherwise `$env:PROCESSOR_ARCHITECTURE`, in Windows PowerShell. Map `aarch64` to `arm64` and `x86_64`/`AMD64` to `amd64`. Prefer the native build; report an unsupported platform rather than guessing. Re-select only when the execution host changes.

An installed copy of this skill provides a candidate vault root three directories above its directory. A source repository or the current working directory is not automatically the vault. Bind the executable beneath that candidate before resolving the vault's identity.

Throughout kernel instructions, `<CLI>`, `<cli>`, `<VAULTCTL>` and `<vaultctl>` denote this executable's quoted absolute path. In PowerShell, prefix the quoted path with `&`. Replace placeholders before execution. Shell examples using `$VAULTCTL` assume it has been assigned that exact path.

```text
<CLI> config resolve --vault "<candidate-path>"
```

Bind the returned `vault_root` on successful resolution and pass it explicitly as `--vault` on vault-scoped commands. Read [vault resolution](../../../90-Meta/vault-resolution.md) when resolution fails or the task requires a source checkout. Bootstrap instructions can be read before resolution; domain evidence requires the binding.

Execute the commands documented in this reference or the owning workflow, substituting the resolved paths and required inputs. Consult CLI help only to resolve a specific syntax, option or version mismatch that the procedure does not answer. Help describes the interface; the owning workflow defines mutation prerequisites, review and recovery.

## Choose the operation

| Need | Operation |
|---|---|
| Initial vault orientation | `config status --vault "<root>"` |
| Find an unknown starting document | `search --vault "<root>" --query "<subject terms>" --limit 5` |
| Inspect a known note's relationships | `links --vault "<root>" --node "<note basename>"` |
| Inspect an identified source | Read its file directly with the available file tool |
| Validate structure | Use the workflow-required `check` or `audit` command |
| Change configuration, investigations, handoffs or synchronization state | Follow the owning skill, then invoke its CLI commands |

Form search terms from the substantive subject and identifiers in the request and established conversation. A confirmation or continuation adds no reason to repeat a completed search. Search again when a new question, changed source or unresolved evidence gap requires it. The CLI provides lexical retrieval, not semantic interpretation of conversational intent.

## Use retrieval results

`search` returns bounded `cards` with vault, path, title, section, line, excerpt, origin, visibility and source hash. Open the relevant source at that location and inspect enough surrounding content to resolve the question. BM25 orders lexical matches; it is neither a confidence percentage nor a test of factual support. Preserve source provenance: an investigation is case context, and a note is not automatically canonical.

Default retrieval includes eligible local/private investigation material. Use `--visibility public` when private material must be excluded from retrieval; public visibility alone does not authorize external disclosure. For any case used as evidence, follow `manage-investigation`'s read-only load to obtain the complete case context and overlays.

An empty `cards` array after a successful command means no results for that query and scope. Check the subject terms and visibility, try a concrete identifier or documented alternative term when relevant, and inspect a known source directly. The index covers eligible vault Markdown, not external repositories, databases or live services. Route remaining evidence gaps to the appropriate source workflow.

`links` returns recorded incoming/outgoing relationships and investigation pointers. Inspect truncation indicators and source notes when completeness matters; recorded relationships alone do not establish runtime behavior.

## Refresh and failures

Retrieval refreshes the local SQLite index automatically by comparing file fingerprints and updating changed files. Markdown remains authoritative. Run `index --vault "<root>" --rebuild` only for an explicit rebuild request or diagnosed index recovery; an empty search alone is not evidence of corruption.

Check the exit status and error output before interpreting results. A failed command is not an empty search. Correct the identified input or prerequisite and retry; use the owning workflow's recovery for stateful operations instead of repeating a mutation blindly.

If the executable is missing, incompatible or cannot start, report its exact path and error and restore the matching versioned artifact through the repository or distribution update workflow within existing authority. Do not substitute hand-written state mutations for unavailable CLI operations. Continue only independent work whose required binding and checks are already satisfied. Local search needs no Obsidian, Git, Python or model service; source acquisition and specialized workflows may require their own external tools.
