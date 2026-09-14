"""Negative cases for the visual structural gate; no browser claims."""
import importlib.util
import re
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location('check_visual', ROOT / 'kernel/.agents/skills/explain-visually/scripts/check_visual.py')
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
BASE = '<svg aria-label="demo"><g data-node="a" role="button" tabindex="0" aria-pressed="false"><rect x="0" y="0" width="10" height="10"/></g><g data-node="b" role="button" tabindex="0" aria-pressed="false"><rect x="20" y="0" width="10" height="10"/></g><path class="edge" id="e" data-from="a" data-to="b" d="M10 5L20 5"/><text data-edge="e">to b</text></svg>'


class VisualChecks(unittest.TestCase):
    def test_valid_structure_does_not_claim_browser_verification(self):
        result = MODULE.check(BASE, 'spatial')
        self.assertTrue(result['structural_pass'])
        self.assertFalse(result['browser_verified'])

    def test_dangling_endpoint_label_and_accessibility_reference(self):
        for bad in [BASE.replace('data-to="b"', 'data-to="missing"'), BASE.replace('data-edge="e"', 'data-edge="missing"'), BASE.replace('aria-label="demo"', 'aria-labelledby="missing"')]:
            with self.subTest(bad=bad):
                self.assertFalse(MODULE.check(bad, 'spatial')['structural_pass'])

    def test_edge_declares_correct_node_but_misses_geometry(self):
        self.assertFalse(MODULE.check(BASE.replace('L20 5', 'L25 5'), 'spatial')['structural_pass'])

    def test_unrelated_icon_transform_does_not_reject_edges(self):
        icon = '<defs><g transform="translate(2 2)"><circle r="1"/></g></defs>'
        self.assertTrue(MODULE.check(BASE.replace('</svg>', icon + '</svg>'), 'spatial')['structural_pass'])

    def test_geometry_transforms_still_require_manual_review(self):
        for bad in [BASE.replace('<svg ', '<svg transform="translate(2 2)" '),
                    BASE.replace('<g data-node="a"', '<g transform="translate(2 2)" data-node="a"'),
                    BASE.replace('<rect x="0"', '<rect transform="translate(2 2)" x="0"'),
                    BASE.replace('<path ', '<g transform="translate(2 2)"><path ').replace('/><text', '/></g><text')]:
            with self.subTest(bad=bad):
                self.assertFalse(MODULE.check(bad, 'spatial')['structural_pass'])

    def test_static_route_fallback_matches_graph(self):
        html = (ROOT / 'kernel/.agents/skills/explain-visually/assets/path-explorer.html').read_text()
        parser = MODULE.Visual()
        parser.feed(html)
        edges = {(a['data-from'], a['data-to']) for tag, a in parser.elements if tag == 'path' and a.get('data-from')}
        names = {'Reader': 'reader', 'Gateway': 'gateway', 'Documents': 'docs', 'Renderer': 'renderer',
                 'Editor': 'editor', 'Publish queue': 'queue', 'Audit log': 'audit',
                 'Indexer': 'indexer', 'Search index': 'search'}
        fallback = html.split('<summary>Sample route names</summary><p>')[1].split('</p>')[0]
        described = set()
        for branch in re.split(r'[.;]', fallback):
            chain = branch.split(':')[-1].strip().split(' → ')
            for start, end in zip(chain, chain[1:]):
                described.add((names[start], names[end]))
        self.assertEqual(described, edges)

    def test_temporal_missing_invalid_and_valid_dates(self):
        self.assertFalse(MODULE.check(BASE, 'spatial', True)['structural_pass'])
        self.assertFalse(MODULE.check(BASE.replace('<svg', '<svg data-as-of="2026-02-30"'), 'spatial', True)['structural_pass'])
        self.assertTrue(MODULE.check(BASE.replace('<svg', '<svg data-as-of="2026-09-14"'), 'spatial', True)['structural_pass'])


if __name__ == '__main__':
    unittest.main()
