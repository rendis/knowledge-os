---
name: generate-reports
description: "Trigger: list, generate, or validate a registered report from a versioned recipe. Hand publication to manage-operational-workflow."
---

# Generate registered reports

Resolve report semantics from the vault and execute only the matching bundled recipe. Do not accept arbitrary SQL or targets.

## 1. Resolve

1. Resolve the canonical vault with `map-ecosystem`.
2. Run `python3 90-Meta/operational-catalog.py list-reports` or `resolve --report-id <id>`.
3. Read the resolved report note and [references/report-contract.md](references/report-contract.md).
4. If the matching recipe directory contains an extra note, read it; do not assume a product-specific report-id.

Require the user-facing start and end months as inclusive `YYYY-MM` values. Ask for either missing value and carry both unchanged into the command; the runner alone derives the exclusive BigQuery boundary. Complete when one documented `report-id`, its recipe, both months, output directory, and effect boundary are exact. Return `unsupported-report-id` for no match.

## 2. Select the execution branch

- **List**: run `scripts/run_report.py list`; remain read-only.
- **Fixture generation**: pass `--input-csv`; do not contact BigQuery.
- **Live generation**: use the bundled query and the already-active `bq` identity; require user authorization before the productive query.
- **Validate**: run `scripts/run_report.py validate --workbook <path> --manifest <path>`.
- **Publish or schedule**: hand the generated artifact package to `manage-operational-workflow`; this skill stops before the external effect.

Complete when exactly one branch owns the requested outcome.

## 3. Generate

Use Python 3.9 or newer with the interpreter prepared from `scripts/requirements.txt`:

```text
<python> scripts/run_report.py generate --report-id <id> --output-dir <dir> --start-month YYYY-MM --end-month YYYY-MM [--input-csv <fixture>]
```

The live branch always performs a dry-run, enforces the recipe caps, executes the fixed query with parameters, normalizes the four-column aggregate, creates the workbook, injects native PivotTables, validates OOXML, provenance and totals, then atomically publishes the workbook and non-sensitive manifest. Do not accept arbitrary SQL, project, dataset, table, exclusions, or renderer overrides.

Complete when the `.xlsx` and manifest exist, snapshot/query/recipe/implementation/workbook hashes and totals reconcile, all expected year sheets and PivotTables validate, and temporary extraction files were removed. A failed run leaves neither final file. For the same normalized input and implementation, the workbook SHA-256 must repeat; execution timestamps may differ in the manifest.

## Guardrails

- Consume the active identity; preserve authentication and gcloud configuration.
- Keep queries bounded, aggregated, parameterized, dry-run first, and below the versioned byte cap.
- Keep data, generated workbooks, manifests, and temporaries outside the vault.
- Treat the report note as semantic authority and the recipe as its one implementation.
