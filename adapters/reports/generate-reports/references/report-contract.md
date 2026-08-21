# Report execution contract

Resolve each report through `90-Meta/operational-catalog.py`. Require exactly one `clase: reporte` note and one recipe directory named with the same `report-id`.

The note owns purpose, audience, metric, grain, scope, period, exclusions, output, validation, maintenance, and limitations. The recipe owns only executable details: fixed source, query, byte cap, normalized schema, renderer, and implementation version.

The bundled `monthly-event-excel` renderer is one concrete report shape. It requires the exact normalized schema `event_month`, `country`, `event_name`, `event_count`; the first field is a calendar month and the final field is a non-negative integer. A different schema requires a new renderer implementation and tests behind the same recipe interface. The generic skill does not reinterpret another shape as monthly events.

Execution stages are fixed: resolve → confirm inclusive period → dry-run → extract → normalize → render → inject PivotTables → validate → manifest → atomic publish. Both months are mandatory; only the runner derives the exclusive query bound. A failure leaves no final artifact package.

The manifest identifies the data snapshot, recipe, query, implementation and workbook with SHA-256 values and records the UTC extraction time. The workbook embeds its report, inclusive period and dataset hash as custom properties. An identical normalized input under one implementation produces a byte-identical workbook; the manifest remains an execution record and therefore carries its actual extraction time.

Stable errors: `unsupported-report-id`, `invalid-period`, `query-cap-exceeded`, `source-schema-invalid`, `dataset-invalid`, `renderer-unavailable`, and `artifact-invalid`.

Adding a report with an already-supported renderer requires one report note, one recipe directory, and tests. Add an adapter or renderer when a concrete report introduces a new source or normalized shape; never create a separate skill or central semantic registry.
