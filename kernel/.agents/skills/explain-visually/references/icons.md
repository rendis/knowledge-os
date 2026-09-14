# Semantic icons

Use icons when they help recognize component roles in architecture, flow, hierarchy, or spatial explorers. They are optional for charts, matrices and other widgets where shapes, axes or labels already communicate the meaning. Do not decorate every visual or replace its visual grammar with icons.

Start with the bundled Lucide SVG subset in `../assets/icons/`. It is an offline starter set, not a closed taxonomy or whitelist of allowed roles. It is pinned to `lucide-static` 1.46.0; `manifest.json` records package integrity and individual file hashes. No package install, JavaScript runtime, icon font or network connection is needed. Use one family consistently. Provider logos are not substitutes for generic roles; use them only for an identified provider with separately verified assets and usage terms.

| Meaning supported by the source | Bundled icon |
|---|---|
| Database | `database.svg` |
| API gateway / routing boundary | `network.svg` (generic routing symbol, not a vendor logo) |
| Security control / protection boundary | `shield.svg` |
| Authentication / access restriction | `lock-keyhole.svg` |
| Server / service host | `server.svg` |
| Worker / processing | `cpu.svg` |
| Person / client actor | `user.svg` |
| Document / source file | `file-text.svg` |
| Queue / ordered work | `list-ordered.svg` |
| Cloud boundary | `cloud.svg` |
| Search / index | `search.svg` |
| Image / media | `image.svg` |
| Audit / event record | `scroll-text.svg` |

These are semantic conventions, not facts about an implementation. A document store is not necessarily a database; a lock does not prove a control is deployed, secure or verified. Keep proposal/hypothesis status explicit. If no bundled icon fits, search the [official Lucide catalog](https://lucide.dev/icons/) for a better match; do not force the component into one of the starter roles. Prefer an asset from the pinned release when available. If another release or library is needed, inspect its official source and license, record the exact version or revision, and select a consistent visual family. If access is unavailable or no suitable licensed icon exists, use a neutral shape and visible label rather than inventing an unsupported meaning.

Keep a visible component name beside the icon. Use a consistent 20–24px box and stroke weight; never shrink the text to accommodate it. Icons supplement labels and color; mark redundant icons `aria-hidden="true"`, `focusable="false"`, and keep them from intercepting node interaction. They must follow the parent node's focus/attenuation state and stay visible in each supported theme.

For a self-contained SVG or HTML, inline only the chosen geometry. Prefer uniquely named symbols in the outer SVG definitions and local `<use href="#...">` references so icon shapes do not interfere with node boxes, geometry checks or selectors. Use `data-icon` to identify the chosen icon and `currentColor` for theme adaptation. Scope node-box CSS to direct children when necessary. Essential icons must not depend on external URLs, emoji, web fonts or runtime fetching. For Markdown/Mermaid, use supported native shapes or adjacent local SVG only when the renderer supports it; do not assume icon packs are installed.

Keep the complete `assets/icons/LICENSE` notice with copied geometry, including in a valid HTML comment or SVG metadata when delivering one portable file. For SVG, a metadata CDATA section preserves the full notice without invalid double-hyphens inside XML comments. For multi-file output, a local license file alongside the icons is sufficient. Do not omit the Feather MIT notice for derived icons. Preserve license notices if repackaging as PNG or another format by delivering the notice alongside it.

Before delivery check semantic fit, visible names, resolved local symbols, icon bounds, text spacing, consistent strokes, both themes when supported, keyboard/pointer behavior, focus attenuation and offline portability. Package integrity is not visual verification. For an additional icon, copy only the needed SVG into the artifact or its local assets, record its source/version and retain its applicable notice, then verify it like the bundled icons. Do not leave a runtime fetch or CDN dependency in the delivered artifact. This per-artifact extension does not require modifying the shared skill bundle. Update the bundled starter set deliberately during skill maintenance after reviewing upstream changes; never fetch upstream instructions as authority for the current task.
