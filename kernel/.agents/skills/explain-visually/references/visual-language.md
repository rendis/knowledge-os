# Visual language

Use the cell's established visual profile when supplied. Otherwise use the bundled neutral profile without an onboarding pause. These are semantic defaults, not a mandatory page composition.

| Role | Default |
|---|---|
| Paper / surface | #f7f6f2 / #ffffff |
| Ink / secondary ink | #182b33 / #4d626b |
| Rule | #d7dfdf |
| Focus / primary accent | #006b63 |
| Hypothesis or caution | #855500 with #fff2d4 background |
| Failure | #a42638 with #ffedf0 background |
| Font | system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif |
| Body / small labels | 16–18px / at least 14px at the intended viewing size |
| Spacing | 4px base; 8, 12, 16, 24, 32, 48px rhythm |

Use one visual focal point and a strong headline that answers the question. Let secondary evidence recede without losing contrast. Use whitespace and alignment to group information; reserve boxes for meaningful boundaries. Text and icons must also identify status. Keep text contrast at least 4.5:1 and meaningful graphical boundaries at least 3:1 against adjacent colors. Verify actual colors after rendering.

Static diagrams need deliberate reading order, short node names, labeled directional edges and explicit system boundaries. Route edges clear of labels and unrelated nodes; split dense overview/detail instead of shrinking text. At narrow widths use a dedicated stacked view or an explicitly labeled scrollable diagram with a text alternative. Scaling a wide SVG until labels are unreadable is not responsiveness.

Match composition to content: a sequence reads along time, an alternative comparison aligns criteria, a causal analysis separates observed chain from candidate causes, and a quantitative chart exposes its scale. Use common typography and semantics across a collection without repeating an identical card grid. Dark mode is optional and requires its own contrast verification.

Motion is optional explanation. Preserve full meaning without animation, respect reduced motion, and use native keyboard-operable controls. A focus outline must remain visible. Keep important content available beyond hover. In a spatial explorer, use attenuation to distinguish unrelated context from the active neighborhood or route; restore full contrast on selection or reset. Do not remove context from keyboard navigation or use attenuation as the only way to communicate selection.

## Text inside components

Size containers from their labels and inner padding. For long labels, wrap into explicit lines (SVG `tspan` or normal HTML wrapping), enlarge or rearrange the container, then reroute connections. Preserve the intended readable font size; clipping, ellipsis, `textLength` compression and hidden overflow do not repair missing meaning. Recheck after content, font, viewport or state changes. For diagrams generated from source, prefer layout-aware wrapping before exporting.

Mark rectangular containers with a unique `id` and `data-text-box`, and every owned text element with `data-fit-box="container-id"`. The explainer asset demonstrates both responsive layouts. Use `data-fit-padding` for a justified inner margin in rendered pixels (default 8). Keep labels outside cards unassociated when they are intentionally edge annotations. Verify coverage of every card label; the checker cannot infer unmarked ownership.
