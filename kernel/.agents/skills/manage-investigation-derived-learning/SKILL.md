---
name: manage-investigation-derived-learning
description: Critically assess whether an exact investigation yields durable, reusable engineering learning; report valid no-learning outcomes; and, when explicitly requested and evidence-qualified, create, enrich, challenge, supersede, or revalidate cumulative learning notes in the canonical cell Obsidian vault.
---

# Manage investigation-derived learning

Turn completed or sufficiently evidenced investigation work into bounded, cumulative learning. Invocation never implies that a publishable learning exists.

## Preflight

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory before reading any vault-relative path. Require one canonical remote- and marker-verified result, then bind every path below to its `VAULT_ROOT`; an unresolved or ambiguous vault blocks the workflow.
2. Resolve the exact `VAULT_ROOT/.investigations/<id>/investigation.md`. Read the complete case, not only its current summary or last History entry.
3. From `VAULT_ROOT`, confirm `.investigations/` is ignored with `git check-ignore .investigations/` and that `git ls-files '.investigations/**'` is empty. Treat the case as local provenance and a source map, never as durable proof by itself.
4. Load [references/assessment-contract.md](references/assessment-contract.md), [references/learning-note-contract.md](references/learning-note-contract.md), `VAULT_ROOT/90-Meta/Convenciones.md`, and the **Gate de aprendizaje durable** in `VAULT_ROOT/90-Meta/Auditoria - Framework.md`.
5. Load `../../../90-Meta/node-selection.md` before selecting or planning a durable target. When the conclusion depends on cell implementation context or source repositories, also load the `map-ecosystem` read-only interrogation branch and resolve every source repository through that workflow before inspecting it.
6. Identify the requested mode:
   - **Assess**: evaluate and report only.
   - **Publish or revalidate**: assess first, then prepare durable effects only if the result is `extractable`.

Assessment is always read-only: do not edit the investigation, create a draft, or change `70-Aprendizajes/` during this phase.

**Complete when:** one canonical `VAULT_ROOT`, one exact case, and one mode are selected, the required contracts are loaded, and every source path to be inspected has an explicit authority boundary.

## Assess

1. State the stable problem or question and the applicability context independently of the investigation title, story, implementation, or session.
2. Trace every candidate claim to directly inspected evidence. Use the case to locate experiments, measurements, stories, implementation revisions, deployments, and prior decisions; do not cite its narrative as a substitute for those sources. No file under `VAULT_ROOT/.investigations/` may fill `Fuentes durables`, including a manifest, runner, hash, or result bundle. The same prohibition applies to ignored local execution, handoff, and plan workspaces such as `.operations/`, `.knowledge-os-handoffs/`, and `.plan/`. Locate a durable/versioned source or reproducible versioned tooling for the decisive claim; when the local artifact is the only support, select `insufficient-evidence`.
3. Search `70-Aprendizajes/` by question, context, `aplica-a`, dimensions, aliases in prose, and backlinks. Compare the candidate with the full existing note, including its limits and evidence entries.
4. Apply the durable-learning gate claim by claim. Distinguish missing evidence from a fully evidenced conclusion that has no reusable teaching.
5. Select exactly one assessment outcome and one lifecycle action from the assessment contract.
6. Present the complete outcome card even when the action is `none`. Name the evidence reviewed, the decisive reason, the applicability boundary, the matching note when one exists, and the exact retry condition for `insufficient-evidence`.

**Complete when:** the user receives one explicit outcome, one compatible action, a bounded rationale, and no file has changed.

## Plan the durable effect

Continue only for `extractable`.

1. Select the canonical learning identity and target from the assessment. Prefer the existing note for `enrich` or `challenge`; use `create` only for a genuinely new identity or a materially different context that can change the conclusion.
2. For `supersede`, prove that the previous conclusion is no longer the active guidance in its stated context. A new investigation or newer date alone is insufficient.
3. Prepare an exact effect plan naming every note to create or modify, the new `EV-###` evidence entry, state/result changes, relationship changes, and index or technical-node propagation decisions.
4. If the user requested only assessment, present the plan and stop. A publication instruction in the current request authorizes the planned local vault effect; any material target or scope expansion requires a new decision.
5. When an adopted change asserts current technical behavior, require the production-evidence gate independently. If that technical state also belongs in the ecosystem map, hand it to the appropriate `map-ecosystem` documentation workflow rather than copying it into technical nodes as a side effect.

**Complete when:** the effect is minimal, deduplicated, gate-qualified, and either explicitly authorized by the request or left as a no-write proposal.

## Publish or revalidate

1. Create a new note from [assets/learning-template.md](assets/learning-template.md), or edit the canonical note using the learning-note contract.
2. Keep the current teaching readable in place and append new evidence with the next immutable `EV-###` identifier. Never rewrite an older evidence entry to make it agree with the latest conclusion.
3. Update `investigaciones-origen`, `ultima-validacion`, applicability, limitations, traceability, and state/result only when the newly inspected evidence supports each change.
4. For `challenge`, preserve both sides and set `estado: cuestionado` until evidence supports a current conclusion. For `supersede`, mark the prior note `superado` and link it from the replacement through `supersede-a`.
5. Do not mutate the source investigation. Return the observed outcome, action, target notes, and checks to `manage-investigation` if its owner needs to record `learning-outcome`.

**Complete when:** the canonical note contains the cumulative evidence and current bounded teaching, older evidence remains traceable, and no local case content was copied as unsupported proof.

## Verify

1. Run the versioned gates from `90-Meta/Auditoria - Framework.md`.
2. Inspect the complete diff and confirm that a failed or no-op assessment produced no durable change.
3. When Obsidian is available, verify the exact vault binding, unresolved links, backlinks, and unexpected orphans using the explicit-vault protocol. In filesystem interaction mode, report the unobserved native checks and run the versioned filesystem checks.
4. Report outcome, lifecycle action, changed notes, evidence boundary, checks observed, and remaining revalidation triggers.

**Complete when:** all applicable gates pass, every changed link resolves canonically, and the final report distinguishes durable learning from investigation-local provenance.

## Guardrails

- Keep assessment critical and pragmatic. `no-learning`, `already-covered`, and `insufficient-evidence` are successful, user-visible outcomes.
- Never publish from memory, another case, a story, an approval, or a local-only result that cannot be independently revisited.
- Keep secrets, private contact data, raw production data, and sensitive logs out of learning notes and responses.
- Prefer accumulation over note proliferation. Investigation IDs provide provenance; they never define learning identity.
- Do not create external tickets, messages, commits, pushes, or pull requests through this skill.
