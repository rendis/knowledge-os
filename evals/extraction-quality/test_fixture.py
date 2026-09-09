"""Validate fixture truth, not extractor prose. Run with unittest discover."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('prepare', HERE/'prepare.py')
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class FixtureTruth(unittest.TestCase):
    def test_frozen_package_and_behavior(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)/'case'
            prepare.prepare(HERE.parents[1], root)
            card = json.loads((root/'package/worker-card.json').read_text())
            repo = root/'sources/availability-service'
            old, new = card['old_oid'], card['new_oid']
            def blob(oid, path):
                return prepare.run('git', 'show', f'{oid}:{path}', cwd=repo)
            before, after = {}, {}
            exec(blob(old, 'availability.py'), before)
            exec(blob(new, 'availability.py'), after)
            self.assertEqual(before['available'](10, 1400)['available'], 9)
            self.assertEqual(after['available'](10, 1400)['available'], 8.6)
            for ns in (before, after):
                self.assertEqual(ns['available'](None, 1400), {'available': None, 'observed': False})
                self.assertEqual(ns['available'](0, 1400), {'available': 0, 'observed': True})
            consumer = {}
            exec((root/'sources/dispatch-service/dispatch.py').read_text(), consumer)
            self.assertTrue(consumer['can_allocate'](before['available'](10, 1400), 9))
            self.assertFalse(consumer['can_allocate'](after['available'](10, 1400), 9))
            for path in ('contracts/availability.md', 'contracts/dispatch-consumer.md'):
                self.assertEqual(blob(old, path), blob(new, path))
            self.assertIn('"legacy-audit"', blob(old, 'routes.py'))
            self.assertIn('EVENT_SINKS = []', blob(new, 'routes.py'))
            self.assertEqual(json.loads(blob(new, 'config/production.json'))['reservation_authority'], 'ledger-secondary')
            self.assertEqual(json.loads(blob(new, 'config/preview.json'))['reservation_authority'], 'ledger-experiment')
            changed = prepare.run('git', 'diff', '--name-only', old, new, cwd=repo).splitlines()
            self.assertEqual(len(changed), 4)
            self.assertFalse(prepare.run('git', 'status', '--porcelain', cwd=repo))
            self.assertEqual(prepare.run('git', 'remote', 'get-url', 'origin', cwd=repo).strip(), 'https://example.invalid/availability-service.git')
            self.assertFalse((root/'package/analysis.json').exists())
            self.assertFalse(list(root.rglob('*oracle*')))
            self.assertEqual(card['analysis'], card['allowed_write'])
            self.assertTrue(json.loads((root/'package/scaffold.json').read_text()))


if __name__ == '__main__':
    unittest.main()
