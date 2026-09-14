"""Verify the pinned icon subset and portable symbol reference checks."""
import hashlib
import importlib.util
import json
from pathlib import Path
import unittest
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[2]
SKILL = ROOT / 'kernel/.agents/skills/explain-visually'
SPEC = importlib.util.spec_from_file_location('check_visual', SKILL / 'scripts/check_visual.py')
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)


class IconChecks(unittest.TestCase):
    def test_pinned_subset_and_license(self):
        folder = SKILL / 'assets/icons'
        manifest = json.loads((folder / 'manifest.json').read_text())
        self.assertEqual(manifest['version'], '1.46.0')
        self.assertEqual({p.stem for p in folder.glob('*.svg')}, set(manifest['icons']))
        for name, digest in manifest['icons'].items():
            with self.subTest(icon=name):
                data = (folder / (name + '.svg')).read_bytes()
                self.assertEqual(hashlib.sha256(data).hexdigest(), digest)
                root = ET.fromstring(data)
                self.assertEqual(root.attrib['viewBox'], '0 0 24 24')
                self.assertFalse(any(e.tag.endswith('script') for e in root.iter()))
        notice = (folder / 'LICENSE').read_text()
        self.assertIn('ISC License', notice)
        self.assertIn('The MIT License', notice)
        self.assertIn('Cole Bemis', notice)

    def test_local_symbol_required(self):
        svg = '<svg aria-label="Diagram"><defs><symbol id="db"/></defs><use href="#db"/></svg>'
        self.assertTrue(CHECK.check(svg)['structural_pass'])
        self.assertFalse(CHECK.check(svg.replace('#db', '#missing'))['structural_pass'])
        self.assertFalse(CHECK.check(svg.replace('#db', 'https://example.org/db.svg'))['structural_pass'])
        icon = svg.replace('<use ', '<use data-icon="database" aria-hidden="true" focusable="false" ')
        self.assertTrue(CHECK.check(icon)['structural_pass'])
        self.assertFalse(CHECK.check(icon.replace('aria-hidden="true"', 'aria-hidden="false"'))['structural_pass'])


if __name__ == '__main__':
    unittest.main()
