---
name: manage-investigation
description: Keep the thread of an investigation or a new development in one case file. Use to open, resume, update, close, publish, absorb into the vault or retire a case; to understand what happened, how something works or why it fails; or to gather the requirements and changes a development needs before handing it off.
---

# Manage investigations

A case holds what a line of work adds beyond the vault: new evidence, decisions, open questions and, for a development, what must change and how to verify it. It lets another session, person or agent continue without the conversation. The vault stays the reference: a case links to notes (`[[note]]`) instead of copying them.

Two types, chosen when opening:

- **understanding** — explain what happened, how something works, or diagnose a failure. Sections: Objective and scope, Current state, Evidence, Conclusions, Decisions, Open questions, Absorption.
- **development** — accumulate what a change needs: requirements and their origin, what changes in which component, acceptance criteria, then one handoff per repository. Sections: Objective and scope, Current state, Requirements, Evidence, Changes by component, Acceptance criteria, Decisions, Open questions, Handoffs, Absorption.

Answering a question does not need a case. Recommend one when the work will continue in another session or with another person or agent, depends on an external answer or access still pending, produces decisions that feed stories or handoffs, or yields a conclusion meant for the vault that still lacks production evidence; open it only when the user asks or accepts. Evidence is gathered with [evidence-driven-analysis](../evidence-driven-analysis/SKILL.md); this skill owns the case.

Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). `<CLI> investigation --help` lists the three commands.

## Where cases live

| Store | Holds |
| --- | --- |
| `.investigations/<id>/` | Unpublished case (ignored by Git): `investigation.md`, `artifacts/`, `exports/` |
| `investigations/<id>/` | Published case, versioned; Git is its history |
| `.investigations-private/<id>/private.md` | Optional sensitive context needed to continue; never authoritative |
| `.investigations-private/<id>/local/` | Machine-specific files, temporary outputs, drafts under review |

Parallel cases do not interfere: each has its own directory. One agent edits a case at a time; re-read the file right before editing it.

## Open

1. `<CLI> investigation list --vault "<root>"` and read any case with the same subject; resume it instead of opening a duplicate.
2. `<CLI> investigation new --vault "<root>" --title "<title>" --type understanding|development [--source-ref "<ticket, message or document>"]`. It creates the case from its type's template and reports cases with a similar title.
3. Write the objective and scope from the request (formalized, not the transcript) and a first Current state.

## Resume

`<CLI> investigation list --vault "<root>" --id <id>` returns the path, visibility, status and whether a private overlay or local store exists. Read **Current state** first, then the registers it cites. For a published case, earlier versions and who changed what are in `git log -p investigations/<id>/`. Report status, what is known, what is missing and the next action; answer a narrow question about a case without rewriting it.

## Update

Edit the case file directly, then run `<CLI> investigation check --vault "<root>" --id <id>` and fix every `error` before continuing.

- **Records.** One record per fact, decision or question, with an immutable ID that is never renumbered or reused: `E-` evidence, `F-` conclusions, `D-` decisions, `Q-` questions, `R-` requirements, `CH-` changes, `AC-` acceptance criteria, `DH-` handoffs, `A-` artifacts, `S-` stories. A materially different claim gets a new ID; a superseded one stays with a pointer to its replacement.
- **Evidence** states the fact, its level (demonstrated, or observed within stated limits: sample, environment, time window) and its source: permalink or `file@commit`, platform snapshot, query, work item, artifact `A-NNN`, or the requester's statement with its date. `check` rejects evidence without a source. What the vault already documents is linked, not repeated; `check` flags paragraphs copied from notes.
- **Conclusions** state their level: demonstrated by `E-…`, inferred from `E-…` (the step explicit), or unresolved (what is missing). Relations that discovery found unsupported come back from `check` as `review`: confirm them at the source or keep them unverified.
- **Current state** is a short living summary that cites record IDs; rewrite it on every update. Each detail has one home: questions own what is missing, evidence owns findings and limits, decisions own choices.
- **Development.** Requirements cite their origin. Each change names the component (`[[repository]]`), what changes (file, module or contract) and which requirement it serves. Each acceptance criterion is observable. When the scope per repository is settled, prepare one handoff per repository with [manage-development-handoff](../manage-development-handoff/SKILL.md) and record it as `DH-NNN`.
- **Artifacts.** Keep in `artifacts/A-NNN-<name>` only methods and outputs that support a claim or let someone repeat it; register each as an `A-` record with what it shows and its limits. When a source cannot be copied, record a summary from [source-summary-template](assets/source-summary-template.md) with its coverage. A visual produced with `explain-visually` is a derived `A-` record, never primary evidence.
- **What goes where.** Case: relevant context, methods, selected outputs. Local store: machine-specific configuration and temporary output. Private overlay: only sensitive context needed to continue that cannot be generalized. Omit transcripts, reasoning and chatter. Never persist credentials, tokens, keys or cookies anywhere: record only that they exist, where they are protected and why they matter.

Ambiguity that blocks the objective: [questioning-protocol](references/questioning-protocol.md). Story drafts and development packages: [export-contract](references/export-contract.md); sufficiency of a development package: [implementation-sufficiency](references/implementation-sufficiency.md); component progress and production handover: [components-and-release](references/components-and-release.md). Observation of a running system goes through `manage-operational-workflow`.

## Close

Set `status: closed` and `outcome:` `completed`, `abandoned` or `superseded-by:<id>`, and state the reason and remaining limits in Current state. `completed` needs the evidence or conclusions that answer the objective; a merge, a deployment claim or a terminal handoff alone does not complete a case. `abandoned` keeps its unresolved questions. New material evidence reopens it (`status: open`). Run `check`.

## Publish, absorb and retire

These change versioned knowledge, so they run on a sync branch with its gates and independent review ([synchronize-ecosystem](../synchronize-ecosystem/SKILL.md)):

1. `<CLI> sync start --vault "<root>" --name case-<id>`.
2. **Publish** (on explicit request): review every file for shareability, keep excluded material in `.investigations-private/<id>/local/`, move `.investigations/<id>/` to `investigations/<id>/` and commit. There is no unpublish.
3. **Absorb** what the vault should keep: edit the notes under `10/`–`70/` from the case's evidence, following the note workflow, and fill the case's **Absorption** table (claim, destination `[[note]]`, status). Investigation-derived lessons go through [manage-investigation-derived-learning](../manage-investigation-derived-learning/SKILL.md); corrections of an existing map through [map-correction](references/map-correction.md).
4. **Retire** a closed case whose useful knowledge is absorbed: `git rm -r investigations/<id>` in a commit whose message carries `Retired-Case: <id>` and the destinations. `investigation list` keeps resolving retired IDs from history.
5. Review, `sync verify` (runs `investigation check` on every changed case: errors introduced on the branch block), `sync finish`.

## Guardrails

- A case is context and provenance, never proof that a behavior exists in production.
- After publication the published case is the authority; the private overlay and local store only supplement it.
- Keep this skill in English and case content in the cell's note locale.
- Ticket creation, story publication and other external writes go through `manage-operational-workflow`; commits, pushes and pull requests need their own authorization.
- Legacy cases (older schema, History sections) load, list and check as they are; `check` reports their schema gaps as warnings. Convert one only when editing it substantially.
