import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import test_run_eval
from test_run_eval import load_module, git

class EvalIntegrity(unittest.TestCase):
    def setUp(self):
        self.module = load_module()
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'repo'
        (self.root / 'evals/sync').mkdir(parents=True)
        (self.root / 'evals/sync/criteria.md').write_text('criteria')
        (self.root / 'candidate').write_text('before')
        git(self.root, 'init', '-b', 'main')
        git(self.root, 'add', '.')
        git(self.root, '-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-m', 'base')
        self.args = SimpleNamespace(repo_root=self.root, eval_root=Path(self.temp.name) / 'eval')
        self.receipt = {'version': 1, 'status': 'pass', 'tests': [{'name':'fixture', 'command':['true'], 'returncode':0, 'status':'pass', 'test_count':1}]}

    def prepare(self):
        with patch.object(self.module, 'run_tests', return_value=(self.receipt, {})):
            return self.module.prepare(self.args)

    def finalize(self, result):
        bundle = Path(result['bundle_root'])
        paths = []
        for role in ('functional-review', 'recovery-review'):
            path = bundle / (role + '.json')
            path.write_text(json.dumps(test_run_eval.RunEvalTests.review(self, role, result['input_digest'])))
            paths.append(path)
        return self.module.finalize(SimpleNamespace(bundle_root=bundle, functional_review=paths[0], recovery_review=paths[1]))

    def test_reject_test_receipt_tamper(self):
        result = self.prepare()
        (Path(result['bundle_root']) / 'test-receipt.json').write_text('{}')
        with self.assertRaises(self.module.EvalError):
            self.finalize(result)

    def test_failed_tests_cannot_approve(self):
        self.receipt['status'] = 'fail'
        self.receipt['tests'][0].update(returncode=1, status='fail')
        result = self.prepare()
        with self.assertRaises(self.module.EvalError):
            self.finalize(result)

    def test_reject_tree_changed_during_tests(self):
        def mutate(root):
            (root / 'candidate').write_text('changed')
            return self.receipt, {}
        with patch.object(self.module, 'run_tests', side_effect=mutate):
            with self.assertRaises(self.module.EvalError):
                self.module.prepare(self.args)

    def test_reject_snapshot_tamper(self):
        result = self.prepare()
        (Path(result['bundle_root']) / 'snapshot/candidate').write_text('tamper')
        with self.assertRaises(self.module.EvalError):
            self.finalize(result)

if __name__ == '__main__':
    unittest.main()
