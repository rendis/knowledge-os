---
name: explain-visually
description: Explain relationships, proposals, problems or quantitative scenarios through clear static or interactive visuals, when requested or when exploration materially improves understanding. Produce temporary aids or source-linked investigation artifacts.
---
# Explain visually

Keep the current investigation, analysis or mapping skill as owner. This auxiliary skill produces a visual and returns its sources, assumptions, checks and retention decision to that owner. Use prose or a table when it answers the question equally well.

## Scope and select

Identify the reader's question, sources and evidence boundary. Establish retention from the request; occasional explanations default to temporary even inside an investigation. Read [retention.md](references/retention.md) before selecting an output path or preserving a result.

Decide presentation separately from retention. Offer a visual when explicitly requested or when it resolves a concrete comprehension gap: relationships, a sequence, alternatives, quantities, or the effect of changing an input. Complexity alone is not a trigger. Prefer one focused aid initially and reuse or refine a sufficient existing visual before adding another. Respect requests for text only.

When the purpose is unclear and different interpretations would change the answer, ask about the reader's goal with two or three contextual alternatives (for example, following a request versus comparing designs). Choose the format yourself when the purpose is clear; missing evidence calls for a source question or an explicitly hypothetical example, not invented facts.

Choose the smallest useful format:

| Reader's question | Starting form |
|---|---|
| What connects, or in what order? | Mermaid in Markdown for a small editable diagram; SVG for controlled layout |
| How do alternatives differ? | Aligned comparison with explicit criteria, assumptions and tradeoffs |
| Where does a process fail? | Sequence or causal diagram distinguishing observation from hypothesis |
| How much, when, or what changes? | Quantitative chart with units, source data and inspectable calculations |
| What happens if I change this input? | Self-contained HTML with working controls and stated model limits |

Select the scope before selecting a template:

- **Atomic visual:** one relationship, state machine, quadrant or layered explanation. The diagram itself is the deliverable, with a short caption and source boundary. Use Mermaid or `assets/atomic.svg`; do not wrap a simple request in a report, hero or dashboard.
- **Spatial explorer:** the reader follows alternative paths or inspects components. Use `assets/path-explorer.html`; keep the map stable while routes, steps and details change. Apply the node/route focus and reset contract in `references/patterns.md`.
- **Document or scenario tool:** use `assets/explainer.html` for a multi-section argument and `assets/explorer.html` for an input-driven calculation. Choose these only when the question needs that structure.

Read [visual-language.md](references/visual-language.md) for every generated artifact. Adapt the nearest asset to the chosen scope, replacing sample content; templates are starting points, not a required house layout. Read [patterns.md](references/patterns.md) only for the chosen form. Native Markdown/Mermaid is a valid complete output; HTML is not mandatory.

For component-oriented visuals, read [icons.md](references/icons.md) and use semantic icons when they improve recognition. Start with the bundled set and extend from verified official catalogs when the component needs a different icon. Keep visible names and choose icons from supported meaning, not appearance alone.

External design skills are optional references, never required installations. Read [provenance.md](references/provenance.md) before importing or updating third-party assets. The bundled originals are sufficient for basic operation.

## Generate and verify

Keep facts, proposals, hypotheses and synthetic fixtures visibly distinct. Every meaningful relationship and quantity must trace to the supplied sources or an explicit assumption. A chart is a derived view, not new runtime evidence. Preserve the data and calculation needed to check quantitative claims.

Use essential inline CSS/JavaScript and local or system fonts for HTML; default to offline output. Read [verification.md](references/verification.md), select its widget-specific acceptance criteria before generation, and run the applicable checker plus rendered/manual checks before delivery. A failing criterion requires repair or an explicit unverified/failed result; do not equate a structural pass with visual approval. Inspect the actual artifacts and available render evidence before reporting completion. Repair concrete defects and rerun the affected checks; if access or rendering is unavailable, report that boundary without claiming a pass.

Return the artifact paths, question answered, source boundary, material simplifications, retention/cleanup status and observed checks. For retained output, deliver and validate the visual together with its same-stem Markdown context using the retention contract; embedded Markdown diagrams carry their context in the same file. Route case registration to `manage-investigation`; route stable knowledge publication to `map-ecosystem`. Generating a visual authorizes neither a new case nor commit, push or external hosting.
