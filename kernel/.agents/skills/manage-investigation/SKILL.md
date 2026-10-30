---
name: manage-investigation
description: Take on a development work item (ticket, story or HT) by finding and resuming or opening its case. Keep an investigation's evidence, conclusions, questions, decisions and requirements through the CLI; resume, close, publish, absorb into the vault or retire; gather what a development needs before handing it off.
---

# Manage investigations

A case is the living record of one line of work: what was asked, what was found and how it is backed, what was decided and what is still missing. It lets another session, person or agent continue without the conversation. The vault stays the reference; the case holds only what this work adds.

Two types, chosen when opening:

- **understanding** — what happened, how something works, why it fails.
- **development** — what a change needs: requirements and their origin, then one handoff package per repository task, which defines what changes there and how it is accepted.

Bind the vault through [use-vault-cli](../use-vault-cli/SKILL.md#bind-the-executable-and-vault). Every change goes through `<CLI> investigation`; `--help` lists the commands and the fields each record requires. The CLI assigns record IDs, logs each change, and refuses a write that would introduce a gate error (evidence without a source, an undefined reference, a broken link, a copied vault paragraph, a credential or a local path). Read the case file freely; write it only through the CLI.

Case directories provide investigation isolation. Open, resume and update cases in the existing vault checkout under the router's branch and checkout rules, including published cases and handoff records. A local edit to a published case needs its case gate, not a sync run; review and publication remain separate steps.

## When a case is worth it

Taking on a development work item (ticket, story or HT) requires a case before substantive requirements analysis or implementation. Follow **Open** to find and resume its existing case or open one of type `development`. The request authorizes these local case records without a separate confirmation; publication and source changes retain their own authorization. An explicit read-only or no-record instruction limits this to inspecting existing cases and reporting the recording constraint.

Other questions do not need a case. Recommend one when the work will continue in another session or with another person or agent, depends on an external answer or access still pending, produces decisions that feed stories or handoffs, or yields a conclusion meant for the vault that still lacks production evidence. Open it when the user asks or accepts.

## From requirements to development

While defining a development work item, keep follow-up clarifications, narrowed scope and accepted designs in its requirements and current state. They settle **what** to build; source edits require a request to implement and a bound development destination. Once the task is sufficiently defined, use [manage-development-handoff](../manage-development-handoff/SKILL.md) to evaluate its need for a handoff and, when warranted, prepare the package and propose the worktree and development session without waiting for a reminder. If the user asks to begin development before that evaluation, perform it before editing source code. Explain the benefit for this task, or why the small-change exception fits. That skill owns the proportional criteria and preserves an explicit choice of an exact existing checkout; neither an absent package nor an absent `.handoff/` skips the evaluation.

## What goes where

| The information | Goes to |
| --- | --- |
| What the requester needs, the expected result, scope | the case objective, formalized when opening |
| A fact the vault already documents, and `discover check` shows fresh for it | a `[[note]]` reference inside the record that relies on it; never copied |
| A fact the vault lacks, contradicts, or holds in a stale note | an evidence record, confirmed at the source |
| A conclusion, decision, open question or pending search | its record |
| Durable knowledge about how the system works, true beyond this case | a finding marked `--for-vault [[note]]`, absorbed into that note on publication |
| A file that is the source or the method of a result a record cites (a log, an export, the script that produced a query result) | attached to that record with `--file` |
| A sanitized synthesis of sources that cannot enter the vault (an inventory of restricted files, a meeting analysis without names or off-topic talk) that records rely on | attached with `--file` to the evidence record that summarizes it; the originals stay out, named by file and hash in the source |
| Everything else the work needs: scratch scripts, raw outputs, drafts, sensitive context | `.investigations-private/<id>/`, never shared |
| Conversation, reasoning, routine progress, dead ends, tone | nowhere; a discarded hypothesis that changes the conclusion is recorded as a finding |

Write at **events**, not at turns: opening, each confirmed fact, a changed conclusion, a decision, a question opened or resolved, a requester contribution (evidence or a request to search), a prepared handoff, closing. After a batch of records, rewrite the current state.

**Language.** The case records what is needed and what is known in neutral, practical language. Formalize the requester's words into the need, the expected result and constraints; attribute requester input as a dated statement of fact or need. Frustration, emphasis, profanity and verbatim transcripts stay out, including in quoted text.

## Open

1. `<CLI> investigation list --vault "<root>"`; match the work-item reference and subject against existing cases and resume the matching case instead of opening a duplicate. Read its current state and cited records through **Resume** before continuing; record new requirements and decisions in that case.
2. Formalize the request: what is needed and for what, the expected result, what is in and out of scope. Ask only for what blocks the objective ([questioning-protocol](references/questioning-protocol.md)).
3. `<CLI> investigation new --vault "<root>" --title "<title>" --type understanding|development --objective "<formalized request>" [--source-ref "<ticket, message or document>"]`.

## Record

`<CLI> investigation add --vault "<root>" --id <id> --kind <kind> --text "<one fact, conclusion, question…>" <fields>`:

- **evidence** — `--source` (permalink or `file@commit`, platform snapshot, query, work item key, record, or the requester statement with its date) and `--level demonstrated`, or `--level observed --limits "<sample, environment, window>"`. One level per record: a fact a document demonstrates and a meeting only corroborates is two records.
- **finding** — `--level demonstrated|inferred --from E-…`, or `--level unresolved --missing "<source or check that settles it>"`; `--for-vault [[note]]` when it belongs in the vault. Relations discovery contradicts come back from `check` as `review`: confirm them at the source or keep them unverified.
- **question** — `--resolve-by` (source, access or person). A request to search something is a question; evidence, findings and decisions close it with `--resolves Q-…`.
- **decision** — `--by <role>`, with the reason in the text.
- **requirement** (`--origin`) and **handoff** (`--package handoffs/DH-NNN.md`, once it passes its gate) — development cases; what changes where and how it is accepted lives in the package ([manage-development-handoff](../manage-development-handoff/SKILL.md)). Acceptance criteria agreed before a package exists are requirements whose text says so; the package takes them over.

Attribute only the requester's actual choices to the requester. Record an agent's inference or proposal as such; a case record cannot create implementation authority that the conversation did not grant.

`--file <path>` (repeatable) attaches files to an evidence or finding record as `artifacts/<ID>-<name>` and refuses credentials. A materially different claim is a new record with `--supersedes <ID>`; records are never renumbered or edited in place, so a file a record relies on and lacks comes with a record that supersedes it (`--supersedes <ID> --file <path>`). An identifier of a kind cases no longer define (an earlier version's attachment `` `A-001` ``, export `` `S-004` `` or acceptance criterion `` `AC-002` ``) is written as inline code; bare, the gate reads it as a reference. Current kinds (E, F, Q, D, R, DH) are always checked, in code or not. `state --text "<what is known, what is missing, next step>"` rewrites the current state, citing the records it summarizes. Every write returns `gate_review` reminders that do not block: a component the vault documents named without its `[[note]]` (reference the note once `discover check` shows it fresh for the claim and it describes the branch or revision the record relies on, instead of restating it) and relations discovery contradicts. `<CLI> investigation check --id <id>` runs the same gate over the whole case, including its handoff packages. Handoff progress comes back through `handoff reconcile`.

Story drafts: [export-contract](references/export-contract.md); sufficiency of a development package: [implementation-sufficiency](references/implementation-sufficiency.md); component progress and production handover: [components-and-release](references/components-and-release.md). Observation of a running system goes through `manage-operational-workflow`.

## Resume

`<CLI> investigation list --vault "<root>" --id <id>` returns the path, visibility, status and the private directory. Read the current state first, then the records it cites; the log gives the sequence, and `git log -p investigations/<id>/` the history of a published case. Report status, what is known, what is missing and the next action; answer a narrow question about a case without changing it.

A case that `investigation check` rejects because it predates the current format is **recreated**, not patched: copy it whole to `.investigations-private/<id>/earlier/`, remove it from its store in the existing checkout, then `investigation new --id <id> --date <opened>` and record its content through `add` with `--as <ID>`, so each record keeps its ID (a dropped record leaves a gap), and `--date <day it was first added>`, from the opening day on; a fact's own date belongs in its source, never in `--date`. Every fact keeps its source; files the records rely on are attached with `--file`; scratch stays in the private directory. Restore a previously published case to `investigations/<id>/` after its gate passes; publication of that change still needs review and authorization.

## Close

`<CLI> investigation close --id <id> --outcome completed|abandoned|superseded-by:<id> --reason "<why, and the limits that remain>"`. `completed` needs the evidence or conclusions that answer the objective; a merge, a deployment claim or a terminal handoff alone does not complete a case. `abandoned` keeps its unresolved questions. New material evidence reopens it with `reopen --reason`.

## Where cases live

| Store | Holds |
| --- | --- |
| `.investigations/<id>/` (unpublished, ignored by Git) or `investigations/<id>/` (published, versioned) | `investigation.md`, `artifacts/`, `handoffs/`, `exports/` |
| `.investigations-private/<id>/` | What the case needs but never shares |

Each case has its own directory; one agent writes a case at a time. Serialize shared vault writes and preserve other cases' pending work.

## Publish, absorb and retire

Publication, absorption and retirement require separate applicable authority and use the canonical sync gates and independent review ([synchronize-ecosystem](../synchronize-ecosystem/SKILL.md)). Local case updates alone do not trigger this workflow. An authorized publication that needs this recipe is a concrete workflow reason for a sync branch under the router's exception rule.

1. Confirm publication authority and inspect `sync status`, the current branch and pending work. Reuse an applicable sync run; otherwise explain the branch effects and reuse applicable authority. For pending edits in an already-published case, use `<CLI> sync start --vault "<root>" --name case-<id> --case <id>` under [synchronize-ecosystem's published-case transition](../synchronize-ecosystem/SKILL.md#pending-published-case-edits). It branches from the current base and preserves pending content and the index; it refuses pending knowledge outside that case, unrelated staged files and a different base. It does not commit, stash, discard or create a worktree. For an unpublished ignored case, use normal `sync start` with its clean-knowledge precondition. Request only missing authority and preserve unrelated work; any checkout exception still needs its own concrete reason under the router's rule.
2. **Publish** (on explicit request): review every file for shareability and language. Move an unpublished case from `.investigations/<id>/` to `investigations/<id>/`; an already-published case stays in place. After the transition, stage and commit only the selected case files, preserving unrelated pending work. There is no unpublish.
3. **Absorb** the findings marked for the vault: edit their notes under `10/`–`70/` following the note workflow, then `investigation absorb --id <id> --finding F-NNN`. Investigation-derived lessons go through [manage-investigation-derived-learning](../manage-investigation-derived-learning/SKILL.md); corrections of an existing map through [map-correction](references/map-correction.md).
4. **Retire** a closed case whose useful knowledge is absorbed: `git rm -r investigations/<id>` in a commit whose message carries `Retired-Case: <id>` and the destinations. `investigation list` keeps resolving retired IDs from history.
5. Complete the canonical sync review and run `sync verify`, which includes the case gate on every changed case (introduced errors block) and requires review coverage of the final content. Only after it passes, `sync finish` completes the local run. Verify remote publication separately when authorized.

## Guardrails

- A case is context and provenance, never proof that a behavior exists in production.
- After publication the published case is the authority; the private directory only supplements it.
- Keep this skill in English and case content in the cell's note locale.
- Ticket creation, story publication and other external writes go through `manage-operational-workflow`; commits, pushes and pull requests need their own authorization.
- Never persist credentials, tokens, keys or cookies: record only that they exist, where they are protected and why they matter.
