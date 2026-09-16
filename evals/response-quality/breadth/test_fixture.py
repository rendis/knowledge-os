"""Verify fixture mechanics and expected deterministic facts, not agent behavior."""
import csv
from decimal import Decimal
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('breadth_prepare', Path(__file__).with_name('prepare.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class FixtureTests(unittest.TestCase):
    def test_matrix(self):
        self.assertEqual(len(m.CASES), 13)
        self.assertEqual(len({c[0] for c in m.CASES}), 13)
        for domain in m.SOURCES:
            self.assertEqual(sum(c[1] == domain for c in m.CASES), 5 if domain == 'support' else 4)

    def test_supported_cause_reproduction(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'reproduce.py'
            path.write_text(m.S5_SOURCES['reproduce.py'])
            result = subprocess.run([sys.executable, '-B', str(path)], capture_output=True, text=True, check=True)
            runs = [json.loads(line) for line in result.stdout.splitlines()]
            self.assertEqual([run['status'] for run in runs], ['timeout', 'success', 'timeout'])
            for actual, recorded in zip(runs, m.S5_SOURCES['experiment.json']['runs']):
                self.assertEqual(actual['request_timeout_ms'], recorded['request_timeout_ms'])
                self.assertEqual(actual['status'], recorded['status'])
                self.assertEqual(actual['db_calls'], 0)
                self.assertEqual(actual['events'][-1]['event'], recorded['last_event'])

    def test_supported_cause_scope_is_separate(self):
        self.assertNotIn('incident.md', m.case_sources('S5', 'support'))
        self.assertIn('No request trace', m.case_sources('S4', 'support')['incident.md'])
        self.assertIn('complete synthetic per-request trace', m.case_scope('S5'))
        self.assertNotIn('or per-request trace', m.case_scope('S5'))
        self.assertIn('or per-request trace', m.case_scope('S4'))

    def test_selector(self):
        namespace = {}
        exec(m.SOURCES['logistics']['dispatch.py'], namespace)
        self.assertEqual(namespace['eligible'](m.SOURCES['logistics']['shipments.json']['rows']), ['P1'])

    def test_total(self):
        unique = {}
        for row in csv.DictReader(io.StringIO(m.SOURCES['billing']['charges.csv'])):
            unique[row['line_id']] = row
        total = sum(Decimal(r['amount']) / (100 if r['unit'] == 'cents' else 1) for r in unique.values() if r['status'] == 'posted')
        self.assertEqual(total, Decimal('17.50'))

    def test_failure_vs_empty(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'query.py'
            path.write_text(m.SOURCES['billing']['query.py'])
            failed = subprocess.run([sys.executable, '-B', str(path), 'today'], capture_output=True, text=True)
            empty = subprocess.run([sys.executable, '-B', str(path), 'yesterday'], capture_output=True, text=True)
            self.assertEqual(failed.returncode, 2)
            self.assertIn('timeout', failed.stdout)
            self.assertEqual(empty.returncode, 0)
            self.assertIn('"rows": []', empty.stdout)

    def test_changed_source(self):
        old = '{"revision":"B1","environment":"lab","auto_refund_limit_usd":50}\n'
        self.assertNotEqual(m.sha(old), m.sha(m.SOURCES['billing']['policy.json']))

    def test_existing_root_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(ValueError):
                m.prepare(tmp)


if __name__ == '__main__':
    unittest.main()
