# Report contract

Resolve one canonical `clase: reporte` note with a unique `report-id` through `90-Meta/operational-catalog.py`. The note defines purpose, audience, metrics, grain, scope, input parameters, exclusions, output, validation, maintenance and limitations.

Its source or maintenance section links the destination-owned execution procedure or implementation. That procedure identifies the executor and version, exact source, authentication references, read-only checks, parameter semantics, resource limits, output destination and validation commands. Reuse installed tools; this contract requires no plugin interface, directory layout, query language or renderer.

For generation, resolve → bind inputs and access → execute → validate → deliver locally. For listing or validation, perform only that mode’s required steps.

## Evidence and completion

The returned package records report identity, actual parameters, input identity or observation time, artifact paths and checks observed. Record hashes, reconciliation totals or other reproducibility evidence when required by the report. Distinguish observed completeness from a sampled or truncated result. Output-specific properties belong to the report contract, not this shared method.

A missing implementation is a capability gap, not permission to invent queries or publish an unverified result. The executor owns cleanup and atomic delivery when the report requires them. Failed or partial generation must not be presented as a complete artifact.

Existing cell-owned recipes may remain where their developer installed them. The vault auditor validates report-note identity and uniqueness; the execution procedure validates implementation and output.
