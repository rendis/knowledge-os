# Patterns

Read only the matching section.

## Architecture and sequence

Choose a bounded flow and name the boundary, direction, connector and effect. Prefer 5–9 nodes for a single path, but preserve necessary context in a larger explorer. Put synchronous and asynchronous interactions in separate visual grammars with a legend if needed. A proposed component gets an explicit proposed label, not just a color. Mermaid is preferred for compact diagrams maintained alongside Markdown. For SVG supply `role="img"`, a unique title/description and a text equivalent. Keep an overview plus detail when a faithful map exceeds the visual budget.

## Atomic diagrams

Use the subject's grammar: states with labeled transitions and revision loops; quadrants with named axes and explicitly qualitative or measured positions; layers with responsibility boundaries; trees for hierarchy. Do not turn every subject into left-to-right boxes. Keep the diagram dominant with one short title/caption; omit navigation, summaries and metric cards unless they answer the question. At narrow widths reflow or provide a labeled keyboard-scrollable region rather than shrinking labels into illegibility.

## Spatial explorers

Keep component positions and the surrounding architecture stable across selection. Selection must change the map, not only its inspector. For a selected node, emphasize that node, its direct incoming/outgoing neighbors, and only the edges incident to that node with their labels. For a selected route, emphasize its declared nodes, edges and labels. Visibly attenuate unrelated context while keeping it recognizable and selectable; active content stays at full contrast. Node and route selections replace one another unless the brief explicitly requires combined filtering. Clear stale selection indicators and explanatory text when switching modes. All/Reset restores the complete map and clears the selection. Provide selectable routes, an ordered causal explanation, and a node inspector with meaningful details. Use buttons with keyboard support, visible focus and selection state. Every highlighted edge must match the route explanation. Animation is optional; honor reduced motion and provide a complete static equivalent. Compact controls and the diagram should occupy the initial view, not a document hero.

## Proposals and alternatives

Compare the same criteria in the same order. Show tradeoffs and decision conditions, not invented scores. Distinguish recommendation from approval. Use a side-by-side matrix or differently structured panels according to the options; preserve the full textual comparison at narrow widths. If requested to offer multiple designs, vary composition or interaction materially rather than only recoloring.

## Problems and diagnosis

Show the observed event sequence first, then hypotheses and the next discriminating check. A causal edge needs source support; label plausible explanations as hypotheses. Separate configured, observed and unverified behavior. An interactive trace may reveal steps but must leave a complete static explanation available.

## Quantitative charts and scenario tools

Keep source rows and units inspectable. Use zero baseline for magnitude bars; disclose any truncated axis. State denominator, period, missingness and aggregation. Compute geometry from the same data as visible values and tables. Scenario sliders manipulate a declared model, not production measurements. Label synthetic examples prominently. Include reset, boundary values and a readable result; use a table or textual calculation as the non-graphical equivalent.

## HTML explainers

Choose a document composition from the subject: argument and evidence, comparison, timeline, or guided exploration. Bundle essential assets so opening the file requires no server or network. Browser display is a presentation capability, not a promise that the same HTML executes inside an Obsidian note. For durable vault use include a Markdown summary and, when useful, a static SVG or PNG preview.
