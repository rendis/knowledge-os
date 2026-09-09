---
name: manage-operational-workflow
description: Orchestrate documented operational runbooks across Jira, email, team messaging, and other connected capabilities, including Jira work-item estimation, decomposition, and relationships. Use to advise on, estimate, draft, decompose, create, update, or validate Jira work; draft or execute a runbook; publish a validated artifact package; resume a recorded `.operations/` run; or close it.
---

# Manage operational workflows

Treat the selected procedure in `60-Operacion/` as the runbook, connected capabilities as replaceable executors, and `.operations/` as the resumable ledger for one run.

## 1. Select the branch and source

- **Advise**: explain or classify from the runbook with read-only work.
- **Draft**: prepare artifacts and validate them against the procedure without publishing them.
- **Execute**: open a run, authorize its external effects, and perform the approved steps.
- **Resume**: reconcile the exact recorded run with external state, then continue its pending steps.
- **Close**: verify completion evidence and finish the exact recorded run.

When the input is a validated publication package, accept its exact artifacts, source anchor, target, requested action, and limitations as run inputs. Do not read or modify its source workspace or invoke its producer; return the verified publication result to the caller. This keeps the dependency one-way.

Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Pass any user-supplied path through that resolver and continue only with one canonical `VAULT_ROOT`. Read `60-Operacion/Operacion.md`, resolve the selected note with `python3 90-Meta/operational-catalog.py resolve --basename <name>` (or `--report-id <id>`), then read only that `tipo: operacional` note and its linked dependencies.

Load [references/procedure-contract.md](references/procedure-contract.md) when interpreting, authoring, or changing an operational note. For Execute, Resume, Close, or a persistent Draft, load [references/execution-record.md](references/execution-record.md). Copy [assets/operation-run-template.md](assets/operation-run-template.md) only when opening a new run.

When the request advises on, estimates, drafts, creates, updates, decomposes, converts, links, reparents, or validates Jira work items, load `../../../90-Meta/work-item-evidence.md`, then `../../../90-Meta/jira-evidence.md` and [references/jira-work-items.md](references/jira-work-items.md) before building the effect plan. Use the shared contract plus the Jira mapping for current read-only evidence; keep the versioned Jira notes under `60-Operacion/Jira/` as the semantic authority and this skill as the sole owner of planned Jira effects.

Create `.operations/<run-id>/run.md` for Execute or a Draft that must survive the current session. Resume and Close require the exact existing run record.

If no runbook matches, prepare a labeled ad-hoc plan and verify that existing authorization covers its complete effects; request only missing authorization. Recommend promoting recurring behavior through the vault documentation workflow.

**Complete when:** one branch, one exact runbook or labeled ad-hoc plan, its inputs, and its targets are identified; otherwise return the explicit blocker.

## 2. Build the effect plan

Map every runbook step to:

- stable step ID and intended outcome;
- required inputs and completion evidence;
- target capability and exact destination;
- effect class: read-only, local-state, draft, or external-effect;
- current state and blocker, if any.

Resolve capabilities by function rather than provider-specific tool name. Use `<run-id>:<step-id>` as the idempotency seed for external effects. Mark every missing input, destination, or capability as `blocked`.

**Complete when:** every required runbook step is represented, ordered, and either ready or explicitly blocked.

## 3. Authorize external effects

Before the first external write or send, present one compact preview with:

- runbook and intended outcome;
- ordered external-effect steps;
- exact systems, projects, recipients, or channels;
- artifact summaries or drafts;
- optional, irreversible, and blocked steps.

Use existing explicit authorization when it already covers the exact preview and material effects; request approval only for uncovered scope. Advise and Draft authorize no external publication. A material change to target, recipient, content, action type, or effect order returns the run to `awaiting-approval`.

For Advise or Draft with no external effects, complete this stage after confirming the effect plan contains none.

**Complete when:** the approved preview exactly matches every pending external-effect step, the plan has no external effects, or the run remains `awaiting-approval`.

## 4. Execute the ledger

For each external-effect step:

1. Reconcile its recorded and external state read-only.
2. Perform the pending approved action once.
3. Capture the stable key, ID, URL, or message identifier returned by the authoritative system.
4. Read the created artifact back from that system and verify its exact target, type, approved content, and required relationships.
5. Resolve every required reference by its stable identifier and verify that it exists, expresses the intended relationship, and is accessible to its intended consumers through a permitted check or authoritative confirmation.
6. Record status, timestamp, identifiers, verification method, and a non-sensitive evidence summary immediately.

A success response or returned identifier is not completion evidence by itself. A missing, mismatched, inaccessible, or unverifiable created artifact or required reference moves the step to `blocked`; preserve its identifiers and reconcile before retrying. Read-only preparation may run in parallel. Preserve runbook order for external effects unless it explicitly marks steps independent. Partial success, ambiguous state, rejected authorization, missing capability, or a runbook contradiction also moves the run to `blocked` with the exact safe next action. Compensation, deletion, retraction, or overwrite requires a procedure-backed step and separate approval.

When the selected branch has no approved external effects, skip execution and continue to the handoff.

**Complete when:** every attempted step has a verified `completed`, `not-applicable`, `blocked`, or `failed` state; every created artifact and required reference has an existence, correspondence, and access result; and the evidence records the verification method.

## 5. Close and hand off

Close when every required step is `completed` or procedure-backed `not-applicable`, every created artifact and required reference has passed the integrity checks, external identifiers and completion evidence are recorded, optional omissions are visible, and no approval or blocker remains pending. Return the resulting keys or links, publication timestamp, verification result, and every unresolved limitation to the caller.

For Advise or a non-persistent Draft, return the validated guidance or artifacts and their limitations without creating a completed run.

**Complete when:** the user-facing handoff matches the completed run record, or the requested advice or draft has been delivered and validated against the runbook.

## Guardrails

- Operational notes define expected behavior; connected systems define current external state; the run record captures one execution.
- Store only non-sensitive summaries and external identifiers. Keep credentials, tokens, cookies, private contact details, and sensitive message bodies outside `.operations/`.
- Keep `.operations/` ignored and uncommitted.
- Vault-edit authorization does not authorize Jira changes, email sends, team messages, or any other external effect.
