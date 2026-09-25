"""Verify the pinned icon subset; symbol reference checks are Go tests of `check visual`."""
import hashlib
import json
from pathlib import Path
import unittest
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[2]
SKILL = ROOT / 'kernel/.agents/skills/explain-visually'


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


if __name__ == '__main__':
    unittest.main()
