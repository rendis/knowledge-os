---
name: manage-investigation
description: Keep the thread of an investigation or a new development in one case, written through the CLI. Use to open a case from a request, record evidence, conclusions, questions, decisions, requirements and changes, resume, close, migrate earlier cases, publish, absorb into the vault or retire; to understand what happened, how something works or why it fails; or to gather what a development needs before handing it off.
---

# Manage investigations

A case is the living record of one line of work: what was asked, what was found and how it is backed, what was decided and what is still missing. It lets another session, person or agent continue without the conversation. The vault stays the reference; the case holds only what this work adds.

Two types, chosen when opening:

- **understanding** — what happened, how something works, why it fails.
- **development** — what a change needs: requirements and their origin, what changes in which component, acceptance criteria, then handoffs.

Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). Every change goes through `<CLI> investigation`; `--help` lists the commands and the fields each record requires. The CLI assigns record IDs, logs each change, and refuses a write that would introduce a gate error (evidence without a source, an undefined reference, a broken link, a copied vault paragraph, a credential or a local path). Read the case file freely; write it only through the CLI.

## When a case is worth it

Answering a question does not need a case. Recommend one when the work will continue in another session or with another person or agent, depends on an external answer or access still pending, produces decisions that feed stories or handoffs, or yields a conclusion meant for the vault that still lacks production evidence. Open it when the user asks or accepts.

## What goes where

| The information | Goes to |
| --- | --- |
| What the requester needs, the expected result, scope | the case objective, formalized when opening |
| A fact the vault already documents, and `discover check` shows fresh for it | a `[[note]]` reference inside the record that relies on it; never copied |
| A fact the vault lacks, or contradicts, or that its note is stale for | a new evidence record, confirmed at the source; a contradiction also becomes an absorption entry |
| A conclusion, decision, open question or pending search | its record |
| Durable knowledge about how the system works, true beyond this case | a vault note on publication (absorption); the case keeps the pointer |
| Files the requester provides or that support a claim | `attach` (artifact record) |
| Sensitive context needed to continue | the private overlay |
| Conversation, reasoning, routine progress, dead ends, tone | nowhere; a discarded hypothesis that changes the conclusion is recorded as a finding |

Write at **events**, not at turns: opening, each confirmed fact, a changed conclusion, a decision, a question opened or resolved, a requester contribution (evidence or a request to search), a prepared handoff, closing. After a batch of records, rewrite the current state.

**Language.** The case records what is needed and what is known in neutral, practical language. Formalize the requester's words into the need, the expected result and constraints; attribute requester input as a dated statement of fact or need. Frustration, emphasis, profanity and verbatim transcripts stay out, including in quoted text.

## Open

1. `<CLI> investigation list --vault "<root>"`; resume a case with the same subject instead of opening a duplicate.
2. Formalize the request: what is needed and for what, the expected result, what is in and out of scope. Ask only for what blocks the objective ([questioning-protocol](references/questioning-protocol.md)).
3. `<CLI> investigation new --vault "<root>" --title "<title>" --type understanding|development --objective "<formalized request>" [--source-ref "<ticket, message or document>"]`.

## Record

`<CLI> investigation add --vault "<root>" --id <id> --kind <kind> --text "<one fact, conclusion, question…>" <fields>`:

- **evidence** — `--source` (permalink or `file@commit`, platform snapshot, query, work item key, record, or the requester statement with its date) and `--level demonstrated`, or `--level observed --limits "<sample, environment, window>"`.
- **finding** — `--level demonstrated|inferred --from E-…`, or `--level unresolved --missing "<source or check that settles it>"`. Relations discovery contradicts come back from `check` as `review`: confirm them at the source or keep them unverified.
- **question** — `--resolve-by` (source, access or person). A request from the requester to search something is a question. Evidence, findings and decisions close it with `--resolves Q-…`.
- **decision** — `--by <role>`, with the reason in the text.
- **requirement** `--origin`, **change** `--component [[repository]] --serves R-…`, **acceptance** `--proves R-…` — development cases.
- **handoff** — `--package handoffs/DH-NNN.md` once the package passes its gate ([manage-development-handoff](../manage-development-handoff/SKILL.md)).
- **absorption** — `--target [[note]] --status pending|absorbed|deferred|discarded`.

A materially different claim is a new record with `--supersedes <ID>`; records are never renumbered or edited in place. `attach --file <path> [--file <path>…] --text "<what it shows and its limits>"` copies provided files as one `A-` record and refuses credentials. `state --text "<what is known, what is missing, next step>"` rewrites the current state; it must cite the records it summarizes. `<CLI> investigation check --id <id>` runs the same gate over the whole case, including its handoff packages.

Story drafts and development packages: [export-contract](references/export-contract.md); sufficiency of a development package: [implementation-sufficiency](references/implementation-sufficiency.md); component progress and production handover: [components-and-release](references/components-and-release.md). Observation of a running system goes through `manage-operational-workflow`.

## Resume

`<CLI> investigation list --vault "<root>" --id <id>` returns the path, visibility, status and the private overlay or local store. Read the current state first, then the records it cites; the log gives the sequence, and `git log -p investigations/<id>/` the history of a published case. Report status, what is known, what is missing and the next action; answer a narrow question about a case without changing it.

## Close

`<CLI> investigation close --id <id> --outcome completed|abandoned|superseded-by:<id> --reason "<why, and the limits that remain>"`. `completed` needs the evidence or conclusions that answer the objective; a merge, a deployment claim or a terminal handoff alone does not complete a case. `abandoned` keeps its unresolved questions. New material evidence reopens it with `reopen --reason`.

## Earlier cases

Cases written before this format list and check as they are (their debt reported as warnings) and must be migrated before they change: `<CLI> investigation migrate --vault "<root>" [--id <id>]` previews the section mapping and the debt the gate will report; `--apply` converts it, keeping all content (an unpublished case keeps its earlier copy in its local store; a published one migrates on a sync branch). The reported errors are existing debt: sources to cite and links to fix on the case's next records.

## Where cases live

| Store | Holds |
| --- | --- |
| `.investigations/<id>/` | Unpublished case (ignored by Git): `investigation.md`, `artifacts/`, `exports/`, `handoffs/` |
| `investigations/<id>/` | Published case, versioned; changes only on a sync branch |
| `.investigations-private/<id>/private.md` | Optional sensitive context needed to continue; never authoritative |
| `.investigations-private/<id>/local/` | Machine-specific files, temporary outputs, drafts under review |

Parallel cases do not interfere; one agent writes a case at a time.

## Publish, absorb and retire

These change versioned knowledge, so they run on a sync branch with its gates and independent review ([synchronize-ecosystem](../synchronize-ecosystem/SKILL.md)):

1. `<CLI> sync start --vault "<root>" --name case-<id>`.
2. **Publish** (on explicit request): review every file for shareability, keep excluded material in `.investigations-private/<id>/local/`, move `.investigations/<id>/` to `investigations/<id>/` and commit. There is no unpublish.
3. **Absorb** what the vault should keep: edit the notes under `10/`–`70/` from the case's evidence, following the note workflow, and record each destination with `add --kind absorption`. Investigation-derived lessons go through [manage-investigation-derived-learning](../manage-investigation-derived-learning/SKILL.md); corrections of an existing map through [map-correction](references/map-correction.md).
4. **Retire** a closed case whose useful knowledge is absorbed: `git rm -r investigations/<id>` in a commit whose message carries `Retired-Case: <id>` and the destinations. `investigation list` keeps resolving retired IDs from history.
5. Review, `sync verify` (runs the case gate on every changed case: errors introduced on the branch block), `sync finish`.

## Guardrails

- A case is context and provenance, never proof that a behavior exists in production.
- After publication the published case is the authority; the private overlay and local store only supplement it.
- Keep this skill in English and case content in the cell's note locale.
- Ticket creation, story publication and other external writes go through `manage-operational-workflow`; commits, pushes and pull requests need their own authorization.
- Never persist credentials, tokens, keys or cookies: record only that they exist, where they are protected and why they matter.
