#!/usr/bin/env python3
"""Dependency-free structural checks; never substitutes for rendered inspection."""
import argparse
from datetime import datetime
from html.parser import HTMLParser
import json
from pathlib import Path
import re


class Visual(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.elements = []
        self.groups = []
        self.boxes = {}
        self.geometry_stack = []
        self.transformed_elements = set()
        self.transformed_boxes = set()

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        self.elements.append((tag, attrs))
        transformed = bool(attrs.get('transform')) or any(item[1] for item in self.geometry_stack)
        if transformed:
            self.transformed_elements.add(id(attrs))
        if tag in ('svg', 'g', 'defs', 'symbol'):
            self.geometry_stack.append((tag, transformed))
        if tag == 'g':
            self.groups.append(attrs.get('data-node'))
        if tag == 'rect' and self.groups and self.groups[-1]:
            if transformed:
                self.transformed_boxes.add(self.groups[-1])
            try:
                self.boxes[self.groups[-1]] = tuple(float(attrs.get(k, 0)) for k in ('x', 'y', 'width', 'height'))
            except ValueError:
                pass

    def handle_endtag(self, tag):
        if self.geometry_stack and self.geometry_stack[-1][0] == tag:
            self.geometry_stack.pop()
        if tag == 'g' and self.groups:
            self.groups.pop()

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        self.handle_endtag(tag)


def endpoints(path):
    """Absolute M/L/C paths without transforms; unsupported geometry fails closed."""
    tokens = re.findall(r'[A-Za-z]|[-+]?(?:\d*\.\d+|\d+\.?\d*)(?:[eE][-+]?\d+)?', path)
    if not tokens or tokens[0] != 'M' or any(t.isalpha() and t not in ('M', 'L', 'C') for t in tokens):
        return None
    try:
        command, count = None, 0
        for token in tokens + ['M']:
            if token in ('M', 'L', 'C'):
                if command and (count == 0 or count % (6 if command == 'C' else 2)):
                    return None
                command, count = token, 0
            else:
                float(token)
                count += 1
        start = tuple(map(float, tokens[1:3]))
        end = tuple(map(float, tokens[-2:]))
        if len(start) != 2 or len(end) != 2:
            return None
        return start, end
    except ValueError:
        return None


def on_boundary(point, box):
    x, y = point
    left, top, width, height = box
    return ((abs(x-left) <= 1 or abs(x-left-width) <= 1) and top <= y <= top+height or
            (abs(y-top) <= 1 or abs(y-top-height) <= 1) and left <= x <= left+width)


def check(text, kind='diagram', temporal=False):
    parser = Visual()
    parser.feed(text)
    errors = []
    ids = [a['id'] for _, a in parser.elements if a.get('id')]
    for value in sorted(set(ids)):
        if ids.count(value) > 1:
            errors.append(f'Duplicate id: {value}')
    for _, a in parser.elements:
        for ref in a.get('aria-labelledby', '').split() + a.get('aria-describedby', '').split():
            if ref not in ids:
                errors.append(f'Missing accessibility reference: {ref}')
    for tag, a in parser.elements:
        if tag == 'use':
            ref = a.get('href', a.get('xlink:href', ''))
            if not ref.startswith('#') or ref[1:] not in ids:
                errors.append(f'Missing or non-local SVG symbol: {ref}')
            if a.get('data-icon') and (a.get('aria-hidden') != 'true' or a.get('focusable') != 'false'):
                errors.append('Supplementary component icons need aria-hidden=true and focusable=false; keep the visible component label.')
    svgs = [a for tag, a in parser.elements if tag == 'svg']
    if not svgs:
        errors.append('Expected an SVG diagram in this checker; other renderers need their own checks.')
    for a in svgs:
        if a.get('aria-hidden') == 'true':
            continue  # Decorative icons supplement visible labels; they need no duplicate name.
        if not (a.get('aria-label') or a.get('aria-labelledby')):
            errors.append('SVG needs an accessible name.')
    if kind == 'spatial':
        nodes = [a for _, a in parser.elements if a.get('data-node')]
        names = [a['data-node'] for a in nodes]
        if not nodes:
            errors.append('Spatial explorer needs data-node identifiers.')
        if len(names) != len(set(names)):
            errors.append('Duplicate data-node identifier.')
        for a in nodes:
            if a.get('role') != 'button' or a.get('tabindex') != '0':
                errors.append(f"Node {a['data-node']} needs role=button and tabindex=0.")
            if a.get('aria-pressed') not in ('true', 'false'):
                errors.append(f"Node {a['data-node']} needs aria-pressed selection state.")
        edges = [a for tag, a in parser.elements if tag == 'path' and 'edge' in a.get('class', '').split()]
        if not edges:
            errors.append('Spatial explorer needs explicit edge paths.')
        for a in edges:
            for endpoint in ('data-from', 'data-to'):
                if a.get(endpoint) not in names:
                    errors.append(f"Edge {a.get('id', '(unnamed)')} has missing/unknown {endpoint}.")
            if not a.get('id') or not a.get('d'):
                errors.append('Every edge needs an id and path geometry.')
            points = endpoints(a.get('d', ''))
            if not points or id(a) in parser.transformed_elements or any(a.get(key) in parser.transformed_boxes for key in ('data-from', 'data-to')):
                errors.append(f"Edge {a.get('id')} needs manual geometry verification: checker supports untransformed absolute M/L/C paths only.")
            else:
                for endpoint, point in zip(('data-from', 'data-to'), points):
                    box = parser.boxes.get(a.get(endpoint))
                    if not box or not on_boundary(point, box):
                        errors.append(f"Edge {a.get('id')} {endpoint} point {point} is not on its node rectangle boundary.")
        edge_ids = {a.get('id') for a in edges}
        labels = {a.get('data-edge') for _, a in parser.elements if a.get('data-edge')}
        for value in sorted(edge_ids - labels, key=str):
            errors.append(f'Edge has no linked label: {value}')
        for value in sorted(labels - edge_ids):
            errors.append(f'Label refers to unknown edge: {value}')
    if temporal:
        dates = [a['datetime'] for tag, a in parser.elements if tag == 'time' and a.get('datetime')]
        dates += [a['data-as-of'] for _, a in parser.elements if a.get('data-as-of')]
        if not dates:
            errors.append('Temporal visual needs source-backed time[datetime] or data-as-of (ISO date/time).')
        for value in dates:
            try:
                datetime.fromisoformat(value.replace('Z', '+00:00'))
            except ValueError:
                errors.append(f'Invalid ISO date/time: {value}')
    return {'structural_pass': not errors, 'errors': errors,
            'requires_manual_review': ['source fidelity and date/period meaning', 'actual arrow endpoints and route causality', 'rendered labels, contrast and overlap', 'keyboard and control behavior'],
            'browser_verified': False}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('artifact', type=Path)
    p.add_argument('--kind', choices=('diagram', 'spatial'), default='diagram')
    p.add_argument('--temporal', action='store_true', help='Require a machine-readable source-backed date; inspect visible period/timezone separately.')
    args = p.parse_args()
    result = check(args.artifact.read_text(), args.kind, args.temporal)
    print(json.dumps(result, indent=2))
    raise SystemExit(0 if result['structural_pass'] else 1)


if __name__ == '__main__':
    main()
