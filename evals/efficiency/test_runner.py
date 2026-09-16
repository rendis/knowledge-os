"""Protect incomplete telemetry and review gates in the comparison runner."""
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location('efficiency_run', Path(__file__).with_name('run.py'))
RUN = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUN)


class RunnerTests(unittest.TestCase):
    def test_failed_outcome_cost_is_retained_but_pair_excluded(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rows = []
            for arm in ('baseline', 'alternative'):
                rows.append({'id': 'case/r01-' + arm, 'case': 'case', 'repetition': 1,
                             'variant': arm, 'ok': True, 'wall_seconds': 2,
                             'usage': {'input_tokens': 10, 'cached_input_tokens': 4,
                                       'uncached_input_tokens': 6, 'output_tokens': 2}})
            (root / 'ledger.json').write_text(json.dumps({'runs': rows}))
            (root / 'scores.json').write_text(json.dumps([
                {'id': r['id'], 'accepted': r['variant'] == 'alternative', 'reason': 'fixture'} for r in rows]))
            subprocess.run([sys.executable, '-B', str(Path(__file__).with_name('compare.py')),
                            '--ledger', str(root / 'ledger.json'), '--scores', str(root / 'scores.json'),
                            '--output', str(root / 'result.json')], check=True)
            result = json.loads((root / 'result.json').read_text())['comparisons'][0]
            self.assertIsNone(result['median_paired_delta'])
            self.assertEqual(result['all_attempts']['baseline']['input_tokens']['total'], 10)
            self.assertIsNone(result['all_attempts']['baseline']['input_tokens']['per_accepted'])
            scores = [{'id': r['id'], 'accepted': True, 'reason': 'fixture',
                       'trace_verified': None if r['variant'] == 'alternative' else True} for r in rows]
            (root / 'scores.json').write_text(json.dumps(scores))
            subprocess.run([sys.executable, '-B', str(Path(__file__).with_name('compare.py')),
                            '--ledger', str(root / 'ledger.json'), '--scores', str(root / 'scores.json'),
                            '--output', str(root / 'result.json')], check=True)
            result = json.loads((root / 'result.json').read_text())['comparisons'][0]
            self.assertIsNone(result['median_paired_delta'])
            self.assertEqual(result['all_attempts']['alternative']['accepted'], 0)

    def test_missing_usage_stays_unknown(self):
        self.assertTrue(all(v is None for v in RUN.aggregate_usage([]).values()))
        total = RUN.aggregate_usage([{'input_tokens': 10, 'cached_input_tokens': 4, 'output_tokens': 2}, {}])
        self.assertTrue(all(v is None for v in total.values()))

    def test_cached_is_not_added_to_input(self):
        total = RUN.aggregate_usage([{'input_tokens': 10, 'cached_input_tokens': 4, 'output_tokens': 2}] * 2)
        self.assertEqual(total, {'input_tokens': 20, 'cached_input_tokens': 8, 'output_tokens': 4})

    def test_reviews_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            review = root / 'review'
            review.mkdir()
            for value in ('looks fine', '{"accepted":"true","issues":[]}', '{"accepted":true}'):
                (review / 'answer.md').write_text(value)
                self.assertFalse(RUN.review_result(root, 'review')['accepted'])
            (review / 'answer.md').write_text('{"accepted":true,"issues":[]}')
            self.assertTrue(RUN.review_result(root, 'review')['accepted'])

    def test_failed_turn_without_usage_is_unknown(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'events.jsonl'
            path.write_text(json.dumps({'type': 'turn.failed', 'error': 'unavailable'}) + '\n')
            self.assertTrue(all(v is None for v in RUN.event_usage(path).values()))


if __name__ == '__main__':
    unittest.main()
