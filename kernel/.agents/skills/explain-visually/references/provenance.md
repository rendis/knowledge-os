# Design provenance

The bundled templates and visual profile are original project assets. The workflow was informed by these MIT-licensed references inspected on 2026-09-14:

- Cathryn Lavery, Diagram Design, revision `8d8b2993ee2256ee7dfc0eeb3b5713aba3b60792`: https://github.com/cathrynlavery/diagram-design/tree/8d8b2993ee2256ee7dfc0eeb3b5713aba3b60792 — diagram selection, semantic hierarchy, complexity budgets and deliberate connectors.
- Plannotator, Effective HTML, revision `2ac1dfecb0f2474e75260cb6d3c9b9d6d9b5062e`: https://github.com/plannotator/effective-html/tree/2ac1dfecb0f2474e75260cb6d3c9b9d6d9b5062e — self-contained HTML, content-specific composition, useful interaction and browser verification.

No upstream design templates, scripts or fonts are vendored in this version. A selected icon subset is vendored as documented below. Reuse the bundled assets without network access. Optional installed design skills may contribute specialized guidance while the vault retains evidence and retention ownership.

Additional visual review on 2026-09-14 covered Diagram Design's `docs/screenshots/state.png`, `quadrant.png` and `architecture.png`: compact diagrams are complete outputs, not necessarily documents. Effective HTML's [design guide](https://www.effectivehtml.com/docs/designing-artifacts), [diagram guide](https://www.effectivehtml.com/docs/diagrams), and [live architecture example](https://www.effectivehtml.com/catalog/featured/workspaces-architecture.html) informed the distinction between documents and spatial interfaces. Route selection and node inspection were exercised in the live example linked from its catalog and README recording. Website references are dated observations, not pinned or bundled dependencies; the new assets use original layouts and synthetic content.

Before copying upstream implementation or substantial reference text, inspect the exact revision and license, retain its copyright/license notice with the copied material, and record source paths and local modifications here. Update deliberately after reviewing changes and rerunning representative visual checks; do not fetch mutable upstream instructions during ordinary generation.

## Bundled component icons

Lucide Static 1.46.0 was inspected on 2026-09-14 using the [official static-assets guide](https://lucide.dev/guide/static), [license](https://lucide.dev/license), and versioned npm package. Thirteen SVG files are copied verbatim under `assets/icons/`; `manifest.json` records the exact package URL, verified SHA-512 integrity and individual SHA-256 hashes. `assets/icons/LICENSE` preserves the full ISC and Feather-derived MIT notices. Only selected SVG geometry is needed at generation time; no runtime dependency or automatic upstream updates. Semantic mappings and portable-output notice requirements are in [icons.md](icons.md).
