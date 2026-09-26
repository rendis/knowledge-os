#!/usr/bin/env python3
"""Render the README's images, light and dark, into docs/assets/, from the flow guide itself.

The guide (docs/flows) draws every frame on a 224x126 pixel canvas. With ?export=FLOW@LABEL,... it
renders those frames in both themes and leaves them as JSON; this script reads that JSON through a
headless Chrome (CHROME, else the usual install paths) and writes pixel-exact SVGs: the hero, one
card per flow and the evidence grades. Needs network access for the guide's animation library.
"""
from __future__ import annotations

import html
import json
import os
import re
import shutil
import subprocess
import urllib.parse
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "assets"
GUIDE = ROOT / "docs" / "flows" / "index.html"

SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif"

HERO = "ask@e4+clear+big+say:SOURCED AND GRADED"
FLOWS = {
    "onboard": "onboard@e4+clear",
    "discover": "discover@e4+clear",
    "ask": "ask@e6+clear",
    "investigate": "investigate@e5+clear",
    "handoff": "handoff@e5+clear",
    "sync": "sync@e7+clear",
}
GRADES = [
    ("DEMONSTRATED", "B", ["The inspected source shows it:", "file and lines, snapshot, query."]),
    ("OBSERVED", "b", ["Within stated limits: sample,", "environment or time window."]),
    ("INFERRED", "Y", ["The step from demonstrated", "facts is made explicit."]),
    ("UNRESOLVED", "R", ["The missing source is named,", "with how to obtain it."]),
]
# the 3x3 marks the guide's answer card gives each grade (K ink, W paper)
GRADE_MARKS = [
    ["KKK", "KKK", "KKK"],
    ["KKK", "KWW", "KWK"],
    ["K.K", "...", "K.K"],
    ["KKK", "KWK", "KKK"],
]


def chrome() -> str:
    for c in [os.environ.get("CHROME", ""), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
              "google-chrome", "google-chrome-stable", "chromium", "chromium-browser"]:
        if c and (Path(c).exists() or shutil.which(c)):
            return c
    raise SystemExit("Chrome or Chromium is needed: set CHROME to its executable")


def export(frames: list[str]) -> dict:
    url = f"{GUIDE.as_uri()}?export={urllib.parse.quote(','.join(frames), safe='@,:')}"
    dom = subprocess.run([chrome(), "--headless=new", "--disable-gpu", "--virtual-time-budget=20000", "--dump-dom", url],
                         capture_output=True, text=True, timeout=180, check=True).stdout
    m = re.search(r'<script type="application/json" id="export">(.*?)</script>', dom, re.S)
    if not m:
        raise SystemExit("the guide produced no export (is the network reachable for its scripts?)")
    return json.loads(html.unescape(m.group(1)))


def frame(f: dict, x: float, y: float, scale: float, crop: tuple[int, int] | None = None) -> str:
    """The frame's pixels as one path per colour, placed at x, y and scaled."""
    y0, y1 = crop or (0, f["h"])
    clip = f'<clipPath id="c{id(f)}"><rect x="0" y="{y0}" width="{f["w"]}" height="{y1 - y0}"/></clipPath>'
    body = "".join(f'<path fill="{c}" d="{d}"/>' for c, d in f["paths"].items())
    return (f'<g transform="translate({x} {y - y0 * scale}) scale({scale})" shape-rendering="crispEdges">'
            f'<defs>{clip}</defs><g clip-path="url(#c{id(f)})">{body}</g></g>')


def text(font: dict, s: str, x: float, y: float, scale: float, fill: str) -> str:
    """Pixel text in the guide's 3x5 font."""
    d, cx = [], x
    for ch in s:
        g = font.get(ch, font[" "])
        for r, row in enumerate(g):
            for c, v in enumerate(row):
                if v == "#":
                    d.append(f"M{cx + c * scale} {y + r * scale}h{scale}v{scale}h{-scale}z")
        cx += (len(g[0]) + 1) * scale
    return f'<path fill="{fill}" d="{"".join(d)}" shape-rendering="crispEdges"/>'


def text_width(font: dict, s: str, scale: float) -> float:
    return sum((len(font.get(ch, font[" "])[0]) + 1) * scale for ch in s) - scale


def svg(w: int, h: int, title: str, desc: str, body: str) -> str:
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}" role="img" '
            f'aria-labelledby="title desc"><title id="title">{title}</title><desc id="desc">{desc}</desc>{body}</svg>\n')


def hero(data: dict, mode: str) -> str:
    P, font, f = data["palette"][mode], data["font"], data["frames"][f"{HERO}:{mode}"]
    s, pad, top = 4, 32, 118
    w, h = f["w"] * s + pad * 2, top + f["h"] * s + pad
    body = (f'<rect x="1" y="1" width="{w - 2}" height="{h - 2}" rx="20" fill="{P["bg"]}" stroke="{P["g"]}" stroke-width="2"/>'
            + text(font, "KNOWLEDGE-OS", pad, 30, 7, P["K"])
            + text(font, "YOUR SYSTEMS, MAPPED. EVERY ANSWER, SOURCED.", pad, 82, 3, P["G"])
            + frame(f, pad, top, s))
    return svg(w, h, "knowledge-os",
               "The agent follows the trail from the vault to the code, a cloud snapshot, a database and a tracker, "
               "and each claim of its answer comes back with its source.", body)


def flow_card(data: dict, name: str, mode: str) -> str:
    P, f = data["palette"][mode], data["frames"][f"{FLOWS[name]}:{mode}"]
    s, pad = 2, 0
    w, h = f["w"] * s, f["h"] * s
    body = (f'<clipPath id="r"><rect width="{w}" height="{h}" rx="14"/></clipPath>'
            f'<g clip-path="url(#r)"><rect width="{w}" height="{h}" fill="{P["bg"]}"/>{frame(f, pad, pad, s)}</g>'
            f'<rect x="1" y="1" width="{w - 2}" height="{h - 2}" rx="13" fill="none" stroke="{P["g"]}" stroke-width="2"/>')
    return svg(w, h, f"{name} flow", f"A frame of the {name} flow from the animated guide.", body)


def grades(data: dict, mode: str) -> str:
    P, font = data["palette"][mode], data["font"]
    cards = []
    for i, (name, tone, lines) in enumerate(GRADES):
        x, acc = 1 + i * 240, P[tone]
        mark = "".join(f'<rect x="{x + 20 + c * 5}" y="{22 + r * 5}" width="5" height="5" fill="{P[v]}"/>'
                       for r, row in enumerate(GRADE_MARKS[i]) for c, v in enumerate(row) if v != ".")
        body = "".join(f'<text x="{x + 20}" y="{80 + j * 20}" font-size="13.5" fill="{P["G"]}" font-family="{SANS}">{line}</text>'
                       for j, line in enumerate(lines))
        cards.append(f'<rect x="{x}" y="1" width="226" height="118" rx="14" fill="{P["W"]}" stroke="{P["g"]}" stroke-width="2"/>'
                     f'<rect x="{x}" y="1" width="226" height="6" rx="3" fill="{acc}"/>'
                     f'<g shape-rendering="crispEdges">{mark}</g>'
                     + text(font, name, x + 44, 24, 2.6, P["K"]) + body)
    return svg(960, 120, "Evidence grades",
               "Demonstrated: the inspected source shows it. Observed within limits: sample, environment or time window "
               "stated. Inferred: the step from demonstrated facts is explicit. Unresolved: the missing source is named "
               "with how to obtain it.", "".join(cards))


def main() -> None:
    data = export([HERO, *FLOWS.values()])
    OUT.mkdir(parents=True, exist_ok=True)
    for mode in ("light", "dark"):
        (OUT / f"hero-{mode}.svg").write_text(hero(data, mode), encoding="utf-8")
        (OUT / f"grades-{mode}.svg").write_text(grades(data, mode), encoding="utf-8")
        for name in FLOWS:
            (OUT / f"flow-{name}-{mode}.svg").write_text(flow_card(data, name, mode), encoding="utf-8")


if __name__ == "__main__":
    main()
