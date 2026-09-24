---
name: generate-reports
description: List, generate or validate a registered report using its destination-owned execution procedure. Return local artifacts and verification; external publication belongs to manage-operational-workflow.
---

# Generate registered reports

The report note owns meaning; its linked execution procedure owns the source, parameters, executor and output format. This skill coordinates generation without shipping a query engine or renderer.

## 1. Resolve the report and mode

Resolve `VAULT_ROOT` through `../../../90-Meta/vault-resolution.md`. Keep the calling workflow as owner when invoked as an auxiliary.

Bind `<VAULTCTL>` through the installed `use-vault-cli` skill.

- **List:** run `<VAULTCTL> config reports --vault "<VAULT_ROOT>"` from the resolved vault and return the catalog. No period, executor or live access is required.
- **Generate:** resolve one exact `report-id` through `<VAULTCTL> config operation --vault "<VAULT_ROOT>" --report-id "<id>"`, read its note and [report contract](references/report-contract.md).
- **Validate:** resolve the report contract and exact existing artifacts; require only the validation inputs named by its procedure.

Complete when the selected mode and report are unambiguous, or the requested catalog has been returned.

## 2. Bind the execution contract

Read the report’s source and maintenance sections and follow its linked execution procedure. Resolve the actual executor, source identity, input parameters, output format, validation and access requirements. Reuse the developer-selected installed capability or referenced implementation; its mere presence does not prove current access.

Resolve periods, dimensions, filters and destinations from the report contract. Ask only for missing consequential inputs. A report may use dates, snapshots or other parameters; no calendar grain or file format is imposed by this skill.

If no executor or validation procedure exists, return the exact capability gap. Configuration guidance does not authorize installation, source mutation or infrastructure setup.

Complete when the selected mode has a usable execution or validation contract and all required inputs, or a precise blocker.

## 3. Generate or validate

Follow the selected procedure within the caller’s authorization. For live extraction, verify exact source identity, bounded scope and read-only access; perform cost previews or other preflight checks when the executor requires them. For supplied input, establish its identity and completeness without contacting a live source unnecessarily.

Generate into the designated local output location outside the knowledge graph. Apply the report’s checks for grain, totals, exclusions, completeness and required presentation. Preserve existing artifacts; distinguish incomplete outputs from verified deliverables.

Validation of existing artifacts does not trigger extraction or regeneration. Report failed checks and leave the original artifacts intact.

Complete when the requested artifacts have passed the report-specific checks, or failures and incomplete outputs are explicit.

## 4. Return artifacts

Return the artifact paths, report identity, actual input parameters, source revision or observation time, checks observed and limitations. Preserve sensitive source data according to the execution procedure’s retention rules.

External publication, scheduling or sending requires `manage-operational-workflow`. If that workflow is already the caller, return the package to its current run rather than opening another run.

Completion means verified local generation or validation. It does not establish external delivery.
