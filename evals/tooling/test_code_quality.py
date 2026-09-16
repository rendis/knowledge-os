"""Quality gates reject incomplete analysis even when a scanner exits zero."""
import importlib.util
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location('quality', ROOT / 'kernel/90-Meta/check-code-quality.py')
quality = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(quality)


class BanditCompletionTests(unittest.TestCase):
    def run_report(self, payload, code=0):
        result = subprocess.CompletedProcess([], code, payload, '')
        with patch.object(quality.subprocess, 'run', return_value=result), patch('builtins.print'):
            return quality.run_bandit(['python', '-m', 'bandit'], ROOT)

    def test_complete_clean_report(self):
        self.assertEqual(0, self.run_report(json.dumps({'errors': [], 'results': []})))

    def test_internal_error_with_success_exit_is_rejected(self):
        self.assertEqual(2, self.run_report(json.dumps({
            'errors': [{'filename': 'source.py', 'reason': 'exception while scanning file'}],
            'results': [],
        })))

    def test_missing_or_malformed_report_is_rejected(self):
        for value in ['', 'not json', 'null', '[]', '{}', '{"errors": [], "results": null}']:
            with self.subTest(value=value):
                self.assertEqual(2, self.run_report(value))

    def test_findings_and_scanner_failures_are_preserved(self):
        self.assertEqual(1, self.run_report(json.dumps({'errors': [], 'results': [{'issue_text': 'finding'}]})))
        self.assertEqual(1, self.run_report(json.dumps({'errors': [], 'results': []}), 1))


if __name__ == '__main__':
    unittest.main()
