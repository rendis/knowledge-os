# Report contract

Resolve one canonical `clase: reporte` note with a unique `report-id` through `<CLI> config operation --vault "<VAULT_ROOT>" --report-id "<id>"`. The [Report section of the operational contract](../../manage-operational-workflow/references/procedure-contract.md#report) is the normative definition of required inputs, execution and acceptance criteria. Apply that contract's execution eligibility rules to the report and linked procedure before execution; listing remains a read-only catalog operation.

Follow the destination-owned procedure or implementation linked from the report. Reuse installed tools and resolve the requirements for the selected mode through that contract.

For generation, resolve → bind inputs and access → execute → validate → deliver locally. For listing or validation, perform only that mode’s required steps.

## Evidence and completion

The returned package records report identity, actual parameters, input identity or observation time, artifact paths and checks observed. Record hashes, reconciliation totals or other reproducibility evidence when required by the report. Distinguish observed completeness from a sampled or truncated result. Output-specific properties belong to the report contract, not this shared method.

A missing implementation is a capability gap, not permission to invent queries or publish an unverified result. The executor owns cleanup and atomic delivery when the report requires them. Failed or partial generation must not be presented as a complete artifact.

Existing cell-owned recipes may remain where their developer installed them. The vault auditor validates report-note identity and uniqueness; the execution procedure validates implementation and output.
