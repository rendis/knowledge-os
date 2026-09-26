#!/usr/bin/env python3
"""Render the README's hero and evidence-grade images, light and dark, into docs/assets/."""
from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "assets"

SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace"

# GitHub's light and dark palettes, plus one accent family per role.
THEMES = {
    "light": {
        "panel": "#f6f8fa", "edge": "#d0d7de", "text": "#1f2328", "muted": "#59636e", "line": "#afb8c1",
        "vault": "#eeedfe", "vault_edge": "#534ab7", "vault_text": "#3c3489",
        "source": "#e1f5ee", "source_edge": "#0f6e56", "source_text": "#085041",
        "node": "#ffffff", "pulse": "#534ab7",
        "demonstrated": ("#dafbe1", "#1a7f37"), "observed": ("#ddf4ff", "#0969da"),
        "inferred": ("#fff8c5", "#9a6700"), "unresolved": ("#eaeef2", "#59636e"),
    },
    "dark": {
        "panel": "#161b22", "edge": "#30363d", "text": "#e6edf3", "muted": "#9198a1", "line": "#484f58",
        "vault": "#26215c", "vault_edge": "#afa9ec", "vault_text": "#eeedfe",
        "source": "#04342c", "source_edge": "#5dcaa5", "source_text": "#e1f5ee",
        "node": "#0d1117", "pulse": "#afa9ec",
        "demonstrated": ("#12261e", "#3fb950"), "observed": ("#0c2d4a", "#58a6ff"),
        "inferred": ("#2e2410", "#d29922"), "unresolved": ("#21262d", "#9198a1"),
    },
}

SOURCES = ["Repositories", "Cloud snapshots", "Databases", "Trackers"]
SOURCE_Y = [160, 213, 266, 319]  # 42 px tall; centred on the vault's axis
AXIS = 260


def hero(t: dict) -> str:
    fan_out = "".join(
        f'<path class="edge" d="M460 {AXIS}C495 {AXIS} 495 {y + 21} 526 {y + 21}" marker-end="url(#arrow)"/>'
        for y in SOURCE_Y)
    fan_in = "".join(
        f'<path class="edge" d="M700 {y + 21}C735 {y + 21} 735 {AXIS} 766 {AXIS}" marker-end="url(#arrow)"/>'
        for y in SOURCE_Y)
    pulses_out = "".join(
        f'<path class="pulse p2" pathLength="100" d="M460 {AXIS}C495 {AXIS} 495 {y + 21} 526 {y + 21}"/>'
        for y in SOURCE_Y)
    pulses_in = "".join(
        f'<path class="pulse p3" pathLength="100" d="M700 {y + 21}C735 {y + 21} 735 {AXIS} 766 {AXIS}"/>'
        for y in SOURCE_Y)
    sources = "".join(
        f'<rect class="source" x="530" y="{y}" width="170" height="42" rx="10"/>'
        f'<rect class="flash" x="530" y="{y}" width="170" height="42" rx="10"/>'
        f'<text class="source-label" x="615" y="{y + 26}" text-anchor="middle">{name}</text>'
        for name, y in zip(SOURCES, SOURCE_Y))
    notes = "".join(
        f'<rect x="{318 + i * 40}" y="264" width="32" height="30" rx="4" class="note"/>'
        f'<path d="M{324 + i * 40} 273h20M{324 + i * 40} 280h14M{324 + i * 40} 287h17" class="note-line"/>'
        for i in range(3))
    demonstrated = t["demonstrated"]
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 444" width="960" height="444" role="img" aria-labelledby="title desc">
<title id="title">knowledge-os</title>
<desc id="desc">A question enters the vault, which follows the trail to repositories, cloud snapshots, databases and trackers; the answer comes back sourced and graded, and what was learned returns to the vault through a gated sync.</desc>
<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10z" fill="{t['line']}"/></marker></defs>
<style>
text{{font-family:{SANS};fill:{t['text']}}}
.mono{{font-family:{MONO}}}
.muted{{fill:{t['muted']}}}
.edge{{fill:none;stroke:{t['line']};stroke-width:1.5}}
.node{{fill:{t['node']};stroke:{t['edge']};stroke-width:1.5}}
.vault{{fill:{t['vault']};stroke:{t['vault_edge']};stroke-width:1.5}}
.vault-label{{fill:{t['vault_text']};font-weight:600;font-size:17px}}
.note{{fill:{t['node']};stroke:{t['vault_edge']};stroke-width:1}}
.note-line{{stroke:{t['vault_edge']};stroke-width:1.5;stroke-linecap:round;opacity:.6}}
.source{{fill:{t['source']};stroke:{t['source_edge']};stroke-width:1.5}}
.source-label{{fill:{t['source_text']};font-size:14px;font-weight:500}}
.pulse{{fill:none;stroke:{t['pulse']};stroke-width:3.5;stroke-linecap:round;stroke-dasharray:14 200;stroke-dashoffset:14}}
.p1{{animation:p1 8s linear infinite}}.p2{{animation:p2 8s linear infinite}}.p3{{animation:p3 8s linear infinite}}.p4{{animation:p4 8s linear infinite}}
@keyframes p1{{0%{{stroke-dashoffset:14}}10%,100%{{stroke-dashoffset:-100}}}}
@keyframes p2{{0%,10%{{stroke-dashoffset:14}}22%,100%{{stroke-dashoffset:-100}}}}
@keyframes p3{{0%,26%{{stroke-dashoffset:14}}38%,100%{{stroke-dashoffset:-100}}}}
@keyframes p4{{0%,46%{{stroke-dashoffset:14}}66%,100%{{stroke-dashoffset:-100}}}}
.flash{{fill:none;stroke:{t['source_edge']};stroke-width:3;opacity:0;animation:flash 8s ease-out infinite}}
@keyframes flash{{0%,20%{{opacity:0}}23%{{opacity:1}}32%,100%{{opacity:0}}}}
.glow{{fill:none;stroke:{t['vault_edge']};stroke-width:3;opacity:0;animation:glow 8s ease-out infinite}}
@keyframes glow{{0%,64%{{opacity:0}}67%{{opacity:1}}80%,100%{{opacity:0}}}}
.grade{{opacity:0;animation:grade 8s ease-out infinite}}
@keyframes grade{{0%,37%{{opacity:0}}41%,92%{{opacity:1}}97%,100%{{opacity:0}}}}
@media (prefers-reduced-motion:reduce){{.pulse,.flash,.glow{{display:none}}.grade{{animation:none;opacity:1}}}}
</style>
<rect x="1" y="1" width="958" height="442" rx="16" fill="{t['panel']}" stroke="{t['edge']}"/>
<g transform="translate(40 44)">
<circle cx="8" cy="10" r="6" fill="{t['vault_edge']}"/><circle cx="30" cy="4" r="4" fill="{t['source_edge']}"/><circle cx="28" cy="26" r="5" fill="{t['source_edge']}"/>
<path d="M8 10L30 4M8 10L28 26" stroke="{t['line']}" stroke-width="1.5"/>
</g>
<text x="86" y="74" font-size="36" font-weight="600">knowledge-os</text>
<text x="40" y="112" font-size="19" class="muted">Your team's systems, mapped. Every answer, sourced.</text>
<path class="edge" d="M230 {AXIS}H286" marker-end="url(#arrow)"/>
{fan_out}{fan_in}
<path class="edge" d="M845 295C845 422 375 422 375 314" stroke-dasharray="5 5" marker-end="url(#arrow)"/>
<text x="610" y="430" font-size="13" text-anchor="middle" class="muted">retained through a gated, reviewed sync</text>
<rect class="node" x="40" y="225" width="190" height="70" rx="12"/>
<text x="135" y="252" font-size="15" font-weight="600" text-anchor="middle">Question</text>
<text x="135" y="275" font-size="12.5" text-anchor="middle" class="mono muted">who consumes order.paid?</text>
<rect class="vault" x="290" y="210" width="170" height="100" rx="12"/>
<rect class="glow" x="290" y="210" width="170" height="100" rx="12"/>
<text class="vault-label" x="375" y="245" text-anchor="middle">Vault</text>
{notes}
{sources}
<rect class="node" x="770" y="225" width="150" height="70" rx="12"/>
<text x="845" y="252" font-size="15" font-weight="600" text-anchor="middle">Answer</text>
<text x="845" y="275" font-size="12.5" text-anchor="middle" class="muted">sourced, checked</text>
<g class="grade"><rect x="785" y="182" width="120" height="28" rx="14" fill="{demonstrated[0]}" stroke="{demonstrated[1]}"/>
<text x="845" y="201" font-size="13" font-weight="600" text-anchor="middle" fill="{demonstrated[1]}" style="fill:{demonstrated[1]}">demonstrated</text></g>
<text x="495" y="150" font-size="12" text-anchor="middle" class="muted">the map, not the boundary</text>
<path class="pulse p1" pathLength="100" d="M230 {AXIS}H286"/>
{pulses_out}{pulses_in}
<path class="pulse p4" pathLength="100" d="M845 295C845 422 375 422 375 314"/>
</svg>
"""


GRADES = [
    ("demonstrated", ["The inspected source shows it:", "file and lines, snapshot, query."]),
    ("observed", ["Within stated limits: sample,", "environment or time window."]),
    ("inferred", ["The step from demonstrated", "facts is made explicit."]),
    ("unresolved", ["The missing source is named,", "with how to obtain it."]),
]


def grades(t: dict) -> str:
    cards = []
    for i, (name, lines) in enumerate(GRADES):
        fill, accent = t[name]
        x = 1 + i * 240
        body = "".join(
            f'<text x="{x + 20}" y="{78 + j * 20}" font-size="13.5" class="muted">{line}</text>'
            for j, line in enumerate(lines))
        cards.append(
            f'<rect x="{x}" y="1" width="226" height="118" rx="14" fill="{fill}" stroke="{accent}" stroke-opacity=".45"/>'
            f'<circle cx="{x + 26}" cy="36" r="6" fill="{accent}"/>'
            f'<text x="{x + 40}" y="42" font-size="17" font-weight="600" fill="{accent}" style="fill:{accent}">{name}</text>'
            + body)
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 120" width="960" height="120" role="img" aria-labelledby="title desc">
<title id="title">Evidence grades</title>
<desc id="desc">Demonstrated: the inspected source shows it. Observed within limits: sample, environment or time window stated. Inferred: the step from demonstrated facts is explicit. Unresolved: the missing source is named with how to obtain it.</desc>
<style>text{{font-family:{SANS};fill:{t['text']}}}.muted{{fill:{t['muted']}}}</style>
{''.join(cards)}
</svg>
"""


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for mode, theme in THEMES.items():
        (OUT / f"hero-{mode}.svg").write_text(hero(theme), encoding="utf-8")
        (OUT / f"grades-{mode}.svg").write_text(grades(theme), encoding="utf-8")


if __name__ == "__main__":
    main()
