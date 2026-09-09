"""Public investigation transaction regressions; no external worktree effects."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

DIST = Path(__file__).resolve().parents[2]
HELPER = DIST / 'kernel/.agents/skills/manage-investigation/scripts/investigation-case.py'

class Transactions(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / '.investigations'
        self.root.mkdir()

        self.case_id = '20260908-120000-binding'
        self.run_cli('open', '--id', self.case_id, '--title', 'Binding', '--objective', 'Verify binding', '--dedupe-key', 'binding', '--purpose', 'knowledge', '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated')
        self.case = self.root / self.case_id / 'investigation.md'

    def run_cli(self, *args, ok=True):
        result = subprocess.run([sys.executable, '-B', str(HELPER), '--root', str(self.root), *args], capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, ok, result.stdout + result.stderr)
        return result

    def test_bind_retry_advance_and_rollback_preserve_freshness(self):
        (self.case.parent / 'exports' / 'S-001-story.md').write_text(f'---\nstory-id: S-001\nsource-investigation: {self.case_id}\n---\n# Story\n')
        original = self.case.read_text()
        digest = hashlib.sha256(b'delivery|ABC-123|example.invalid/team/repository').hexdigest()
        observation = {'story-id': 'S-001', 'tracker-id': 'delivery', 'provider': 'example', 'tracker-url': 'https://tracker.example.com', 'work-item-reference': 'ABC-123', 'repository-remote': 'example.invalid/team/repository', 'worktree-path': '/tmp/unused-worktree', 'handoff-id': digest, 'family': f'delivery-abc-123-{digest[:10]}--repository', 'revision': 'v0001', 'materialized-at': '2026-09-09T12:00:00+00:00'}
        path = Path(self.tmp.name) / 'observation.json'
        def bind(ok=True):
            path.write_text(json.dumps(observation))
            return self.run_cli('bind', '--id', self.case_id, '--observation', str(path), ok=ok)
        bind()
        bound = self.case.read_bytes()
        self.assertIn(next(x for x in original.splitlines() if x.startswith('updated-at:')), self.case.read_text())
        self.assertIn('unchanged', bind().stdout)
        self.assertEqual(bound, self.case.read_bytes())
        observation['revision'] = 'v0003'
        bind(False)
        self.assertEqual(bound, self.case.read_bytes())
        observation['revision'] = 'v0002'
        observation['materialized-at'] = 'invalid'
        bind(False)
        self.assertEqual(bound, self.case.read_bytes())
        observation['materialized-at'] = '2026-09-10T12:00:00+00:00'
        bind()
        self.run_cli('validate')
        self.assertFalse((self.root / '.open.lock').exists())

    def test_knowledge_close_without_story_or_export(self):
        self.case.write_text(self.case.read_text().replace('status: intake', 'status: validating'))
        self.run_cli('close', '--id', self.case_id, '--decision', 'complete', '--reason', 'Question answered', '--limitations', 'No production access')
        self.assertIn('status: closed', self.case.read_text())
        self.assertIn('No production access', self.case.read_text())
        self.run_cli('validate')

    def test_abandoned_intake_needs_no_export_or_assessment(self):
        self.case.write_text(self.case.read_text().replace('vault-outcome: none', 'vault-outcome: not-evaluated'))
        self.run_cli('close', '--id', self.case_id, '--decision', 'abandoned', '--reason', 'User discarded question', '--limitations', 'Not investigated')
        self.assertIn('vault-outcome: none', self.case.read_text())
        self.run_cli('validate')

    def test_context_template_is_exported_with_bounded_source_anchor(self):
        contract = (HELPER.parents[1] / 'references/export-contract.md').read_text()
        self.assertIn('development-context-template.md', contract)
        template = (HELPER.parents[1] / 'assets/development-context-template.md').read_text()
        for field in ('Vault remote:', 'Investigation ID:', 'Relevant investigation sections:', 'Relevant linked vault notes:'):
            self.assertIn(field, template)

if __name__ == '__main__':
    unittest.main()
